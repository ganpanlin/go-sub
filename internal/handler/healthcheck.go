package handler

import (
	"encoding/json"
	"go-sub/internal/healthcheck"
	"go-sub/internal/pipeline"
	"go-sub/internal/rule"
	"net/http"
	"time"
)

// HealthCheckRequest is the request body for /api/health-check.
type HealthCheckRequest struct {
	ProfileID string `json:"profile_id"`
	SourceID  string `json:"source_id"`
	Timeout   int    `json:"timeout"`   // per-node timeout in seconds (default 3)
	MaxConc   int    `json:"max_conc"`  // max concurrent checks (default 20)
}

// HealthCheckResponse is returned by the health check endpoint.
type HealthCheckResponse struct {
	Total    int                  `json:"total"`
	Alive    int                  `json:"alive"`
	Dead     int                  `json:"dead"`
	Results  []healthcheck.NodeResult `json:"results"`
	Duration int64                `json:"duration_ms"`
}

// HealthCheckHandler performs TCP ping health checks on nodes.
//
// POST /api/health-check
//
// Request body:
//
//	{ "profile_id": "xxx", "timeout": 3, "max_conc": 20 }
//
// Either profile_id or source_id is required. If both are given, profile_id takes priority.
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	var req HealthCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
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
		result, err := pipeline.Run(profile)
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
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	maxConc := req.MaxConc
	if maxConc <= 0 {
		maxConc = 20
	}

	// Run health checks
	start := time.Now()
	results := healthcheck.CheckNodes(proxies, maxConc, timeout)
	duration := time.Since(start).Milliseconds()

	// Count alive/dead
	alive := 0
	dead := 0
	for _, r := range results {
		if r.Alive {
			alive++
		} else {
			dead++
		}
	}

	resp := HealthCheckResponse{
		Total:    len(results),
		Alive:    alive,
		Dead:     dead,
		Results:  results,
		Duration: duration,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
