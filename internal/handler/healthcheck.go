package handler

import (
	"encoding/json"
	"go-sub/internal/healthcheck"
	"go-sub/internal/latency"
	"go-sub/internal/pipeline"
	"go-sub/internal/rule"
	"net/http"
	"time"
)

// HealthCheckRequest is the request body for /api/health-check.
type HealthCheckRequest struct {
	ProfileID string `json:"profile_id"`
	SourceID  string `json:"source_id"`
	Mode      string `json:"mode"`     // "tcp" (default) | "url" (real proxy round trip via embedded mihomo)
	URL       string `json:"url"`      // url mode: test URL (default http://www.gstatic.com/generate_204)
	Timeout   int    `json:"timeout"`  // per-node timeout in seconds (default 3 tcp / 5 url)
	MaxConc   int    `json:"max_conc"` // max concurrent checks (default 20 tcp / 8 url)
}

// HealthCheckResponse is returned by the health check endpoint.
type HealthCheckResponse struct {
	Total    int                      `json:"total"`
	Alive    int                      `json:"alive"`
	Dead     int                      `json:"dead"`
	Mode     string                   `json:"mode"`
	URL      string                   `json:"url,omitempty"`
	Results  []healthcheck.NodeResult `json:"results"`
	Duration int64                    `json:"duration_ms"`
}

// HealthCheckHandler checks node health.
//
// POST /api/health-check
//
// Request body:
//
//	{ "profile_id": "xxx", "mode": "tcp"|"url", "timeout": 3, "max_conc": 20 }
//
// mode=tcp: TCP handshake against server:port (fast reachability probe).
// mode=url: real latency — dial through the proxy node itself and request
// the test URL (same as Clash url-test), requires network egress.
//
// Either profile_id or source_id is required. If both are given, profile_id takes priority.
// The profile's own latency filter (max_latency/drop_dead) is bypassed so every
// node — including previously filtered ones — gets re-tested.
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

	// Collect proxies to check
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

	// Default parameters
	defaultTimeout := 3
	defaultConc := 20
	if req.Mode == "url" {
		defaultTimeout = 5
		defaultConc = 8
	}
	if req.Timeout <= 0 {
		req.Timeout = defaultTimeout
	}
	timeout := time.Duration(req.Timeout) * time.Second
	if req.MaxConc <= 0 {
		req.MaxConc = defaultConc
	}

	// Run health checks
	start := time.Now()
	var results []healthcheck.NodeResult
	switch req.Mode {
	case "url":
		testURL := req.URL
		if testURL == "" {
			testURL = latency.DefaultTestURL
		}
		results = latency.TestNodes(proxies, testURL, timeout, req.MaxConc)
		req.URL = testURL
	default:
		results = healthcheck.CheckNodes(proxies, req.MaxConc, timeout)
	}
	duration := time.Since(start).Milliseconds()

	// Persist results so profiles can filter by latency/alive (max_latency / drop_dead)
	healthcheck.UpdateResults(results)

	// Count alive/dead
	alive := 0
	dead := 0
	for _, res := range results {
		if res.Alive {
			alive++
		} else {
			dead++
		}
	}

	resp := HealthCheckResponse{
		Total:    len(results),
		Alive:    alive,
		Dead:     dead,
		Mode:     req.Mode,
		URL:      req.URL,
		Results:  results,
		Duration: duration,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
