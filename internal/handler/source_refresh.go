package handler

import (
	"encoding/json"
	"go-sub/internal/cache"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"net/http"
	"sync"
)

type RefreshSourceResult struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	URL       string `json:"url"`
	Status    int    `json:"status"`
	Latency   int64  `json:"latency"`
	IsCached  bool   `json:"is_cached"`
	NodeCount int    `json:"node_count"`
	Error     string `json:"error,omitempty"`
}

type RefreshSourcesResponse struct {
	Total   int                   `json:"total"`
	Success int                   `json:"success"`
	Failed  int                   `json:"failed"`
	Skipped int                   `json:"skipped"`
	Results []RefreshSourceResult `json:"results"`
}

// RefreshSourcesHandler force-refreshes all enabled source caches.
func RefreshSourcesHandler(w http.ResponseWriter, r *http.Request) {
	sources, err := source.LoadAll()
	if err != nil {
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}

	enabled := make([]source.Config, 0, len(sources))
	skipped := 0
	for _, cfg := range sources {
		source.Normalize(&cfg)
		if !cfg.Enabled {
			skipped++
			continue
		}
		enabled = append(enabled, cfg)
	}

	results := make([]RefreshSourceResult, len(enabled))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, cfg := range enabled {
		wg.Add(1)
		go func(i int, cfg source.Config) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			runtimeURL := source.RuntimeURL(cfg)
			config, status, latency, isCached, fetchErr := provider.FetchAndParseYAMLForce(runtimeURL)
			nodeCount := updateSourceStatusFromConfig(runtimeURL, config, status, latency, isCached, fetchErr)

			result := RefreshSourceResult{
				ID:        cfg.ID,
				Name:      cfg.Name,
				URL:       runtimeURL,
				Status:    status,
				Latency:   latency,
				IsCached:  isCached,
				NodeCount: nodeCount,
			}
			if fetchErr != nil {
				result.Error = fetchErr.Error()
			}
			results[i] = result
		}(i, cfg)
	}

	wg.Wait()

	success := 0
	failed := 0
	for _, result := range results {
		if result.Error == "" && result.Status >= 200 && result.Status < 300 {
			success++
			continue
		}
		failed++
	}

	cache.DeleteByPrefix("sub:")
	cache.DeleteByPrefix("filter:")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(RefreshSourcesResponse{
		Total:   len(enabled),
		Success: success,
		Failed:  failed,
		Skipped: skipped,
		Results: results,
	})
}
