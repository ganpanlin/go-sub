package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-sub/internal/healthcheck"
	"go-sub/internal/latency"
	"go-sub/internal/pipeline"
	"go-sub/internal/rule"
	"net/http"
	"sync"
	"time"
)

// HealthCheckRequest is the request body for /api/health-check.
type HealthCheckRequest struct {
	ProfileID string `json:"profile_id"`
	SourceID  string `json:"source_id"`
	Mode      string `json:"mode"`     // "tcp" (default) | "url" (real proxy round trip via embedded mihomo)
	URL       string `json:"url"`      // url mode: test URL (default http://www.gstatic.com/generate_204)
	Timeout   int    `json:"timeout"`  // per-node timeout in seconds (default 3 tcp / 5 url)
	MaxConc   int    `json:"max_conc"` // max concurrent checks (default 20 tcp / 16 url)
}

// hcFinished is one completed node check, carrying its input index so the
// frontend can update the right table row as results stream in.
type hcFinished struct {
	Index  int                    `json:"index"`
	Result healthcheck.NodeResult `json:"result"`
}

// hcJob tracks a running health check so clients can poll progressive results.
type hcJob struct {
	mu        sync.Mutex
	mode      string
	testURL   string
	total     int
	proxies   []map[string]interface{} // full proxy configs (url mode needs them)
	nodes     []healthcheck.NodeResult // identity of every node, in input order
	finished  []hcFinished             // completion order
	completed int
	done      bool
	cancelled bool
	createdAt time.Time
}

func (j *hcJob) finish(idx int, r healthcheck.NodeResult) {
	j.mu.Lock()
	j.finished = append(j.finished, hcFinished{Index: idx, Result: r})
	j.completed++
	j.mu.Unlock()
}

func (j *hcJob) isCancelled() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.cancelled
}

var (
	hcJobsMu sync.Mutex
	hcJobs   = map[string]*hcJob{}
)

func newJobID() string {
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getJob(id string) *hcJob {
	hcJobsMu.Lock()
	defer hcJobsMu.Unlock()
	return hcJobs[id]
}

// HealthCheckHandler starts an async health check job.
//
// POST /api/health-check
//
//	{ "profile_id": "xxx", "mode": "tcp"|"url", "timeout": 3, "max_conc": 20 }
//
// Returns { job_id, total, mode, url, nodes } immediately; poll
// GET /api/health-check/status?job_id=..&since=N for progressive results and
// POST /api/health-check/cancel to abort. The profile's own latency filter is
// bypassed so every node — including previously filtered ones — gets re-tested.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	var req HealthCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Mode == "" {
		req.Mode = "tcp"
	}
	if req.Mode != "tcp" && req.Mode != "url" {
		http.Error(w, "mode must be tcp or url", http.StatusBadRequest)
		return
	}

	var proxies []map[string]interface{}
	if req.ProfileID != "" {
		mgr := rule.GetManager()
		if mgr == nil {
			http.Error(w, "Service not initialized", http.StatusInternalServerError)
			return
		}
		profile := mgr.GetProfile(req.ProfileID)
		if profile == nil {
			profile = mgr.GetProfileByToken(req.ProfileID)
		}
		if profile == nil {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		}
		// Test the full node set: disable the profile's health filter for this run.
		testProfile := *profile
		testProfile.MaxLatency = 0
		testProfile.DropDead = false
		result, err := pipeline.RunCached(r.Context(), &testProfile)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		proxies = result.Proxies
	} else if req.SourceID != "" {
		http.Error(w, "Source-level health check not yet implemented, use profile_id", http.StatusBadRequest)
		return
	} else {
		http.Error(w, "profile_id is required", http.StatusBadRequest)
		return
	}
	if len(proxies) == 0 {
		http.Error(w, "No proxies to check", http.StatusBadRequest)
		return
	}

	defaultTimeout, defaultConc := 3, 20
	if req.Mode == "url" {
		defaultTimeout, defaultConc = 5, 16
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultTimeout
	}
	if req.MaxConc <= 0 {
		req.MaxConc = defaultConc
	}
	testURL := req.URL
	if testURL == "" {
		testURL = latency.DefaultTestURL
	}

	nodes := make([]healthcheck.NodeResult, len(proxies))
	for i, p := range proxies {
		nodes[i] = healthcheck.NodeResult{
			Name:    fmt.Sprintf("%v", p["name"]),
			Server:  fmt.Sprintf("%v", p["server"]),
			Port:    toInt(p["port"]),
			Latency: -1,
		}
	}

	job := &hcJob{
		mode:      req.Mode,
		testURL:   testURL,
		total:     len(nodes),
		proxies:   proxies,
		nodes:     nodes,
		createdAt: time.Now(),
	}
	jobID := newJobID()
	hcJobsMu.Lock()
	hcJobs[jobID] = job
	hcJobsMu.Unlock()
	go job.run(time.Duration(req.Timeout)*time.Second, req.MaxConc)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"job_id": jobID,
		"total":  job.total,
		"mode":   req.Mode,
		"url":    testURL,
		"nodes":  nodes,
	})
}

// HealthCheckStatusHandler returns progressive results of a health check job.
//
// GET /api/health-check/status?job_id=xxx&since=N
//
// items contains every finished node check with input position >= N, letting
// the client merge deltas instead of re-downloading everything.
func HealthCheckStatusHandler(w http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Query().Get("job_id")
	if jobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	job := getJob(jobID)
	if job == nil {
		http.Error(w, "Job not found or expired", http.StatusNotFound)
		return
	}
	since := toIntQuery(r.URL.Query().Get("since"))

	job.mu.Lock()
	items := job.finished
	if since > 0 {
		if since >= len(items) {
			items = nil
		} else {
			items = items[since:]
		}
	}
	out := struct {
		JobID     string       `json:"job_id"`
		Done      bool         `json:"done"`
		Cancelled bool         `json:"cancelled"`
		Total     int          `json:"total"`
		Completed int          `json:"completed"`
		Items     []hcFinished `json:"items"`
	}{
		JobID:     jobID,
		Done:      job.done,
		Cancelled: job.cancelled,
		Total:     job.total,
		Completed: job.completed,
		Items:     items,
	}
	job.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// HealthCheckCancelHandler aborts a running health check job. Nodes already
// tested are still persisted.
func HealthCheckCancelHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		JobID string `json:"job_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JobID == "" {
		http.Error(w, "job_id is required", http.StatusBadRequest)
		return
	}
	job := getJob(req.JobID)
	if job == nil {
		http.Error(w, "Job not found or expired", http.StatusNotFound)
		return
	}
	job.mu.Lock()
	job.cancelled = true
	job.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// run executes the checks with a worker pool, streaming each result into the
// job as it completes. Cancelled jobs skip the remaining nodes but still
// persist whatever finished.
func (j *hcJob) run(timeout time.Duration, maxConc int) {
	defer func() {
		hcJobsMu.Lock()
		jobID := ""
		for id, jj := range hcJobs {
			if jj == j {
				jobID = id
			}
		}
		hcJobsMu.Unlock()
		if jobID != "" {
			// Keep the final status around for late pollers, then clean up.
			time.AfterFunc(10*time.Minute, func() {
				hcJobsMu.Lock()
				delete(hcJobs, jobID)
				hcJobsMu.Unlock()
			})
		}
	}()

	sem := make(chan struct{}, maxConc)
	var wg sync.WaitGroup
	for i := range j.nodes {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if j.isCancelled() {
				return
			}
			n := j.nodes[idx]
			var res healthcheck.NodeResult
			if j.mode == "url" {
				// Full proxy config required — mihomo parses type/cipher/uuid etc.
				res = latency.TestNode(j.proxies[idx], j.testURL, timeout)
			} else {
				res = healthcheck.CheckNode(n.Name, n.Server, n.Port, timeout)
			}
			res.TestType = j.mode
			j.finish(idx, res)
		}(i)
	}
	wg.Wait()

	j.mu.Lock()
	j.done = true
	results := make([]healthcheck.NodeResult, 0, len(j.finished))
	for _, f := range j.finished {
		results = append(results, f.Result)
	}
	j.mu.Unlock()

	// Persist whatever finished so latency filtering stays fresh.
	healthcheck.UpdateResults(results)
}

func toIntQuery(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	case string:
		var out int
		fmt.Sscanf(n, "%d", &out)
		return out
	}
	return 0
}
