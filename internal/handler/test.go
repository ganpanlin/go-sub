package handler

import (
	"encoding/json"
	"go-sub/internal/cache"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"log"
	"net/http"
)

// TestSourceRequest is the structure for the manual test request.
type TestSourceRequest struct {
	URL string `json:"url"`
	ID  string `json:"id"`
}

// TestSourceResponse is returned after a source test completes.
type TestSourceResponse struct {
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Latency   int64  `json:"latency"`
	IsCached  bool   `json:"is_cached"`
	NodeCount int    `json:"node_count"`
	Error     string `json:"error,omitempty"`
}

// TestSourceHandler tests a single source synchronously and returns the result.
func TestSourceHandler(w http.ResponseWriter, r *http.Request) {
	var req TestSourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	ref := req.ID
	if ref == "" {
		http.Error(w, "ID cannot be empty", http.StatusBadRequest)
		return
	}

	sources, loadErr := source.LoadAll()
	if loadErr != nil {
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}
	cfg, ok := source.FindByID(sources, ref)
	if !ok {
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}
	runtimeURL := source.RuntimeURL(cfg)

	config, status, latency, isCached, err := provider.FetchAndParseYAMLForce(runtimeURL)
	nodeCount := updateSourceStatusFromConfig(runtimeURL, config, status, latency, isCached, err)

	resp := TestSourceResponse{
		URL:       runtimeURL,
		Status:    status,
		Latency:   latency,
		IsCached:  isCached,
		NodeCount: nodeCount,
	}
	if err != nil {
		resp.Error = err.Error()
	}

	log.Printf("Manually tested source %s, status: %d, latency: %dms, cached: %t, error: %v", runtimeURL, status, latency, isCached, err)

	// Invalidate all sub caches since source data has changed
	cache.DeleteByPrefix("sub:")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
