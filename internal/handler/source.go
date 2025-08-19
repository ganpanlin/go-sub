package handler

import (
	"encoding/json"
	"go-sub/internal/cache"
	"go-sub/internal/parser"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"log"
	"net/http"
	"sync"
)

// Track in-flight async fetches to avoid duplicates
var asyncFetchInFlight = sync.Map{}

type SourceResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	URL       string `json:"url"`
	UA        string `json:"ua,omitempty"`
	Enabled   bool   `json:"enabled"`
	Status    int    `json:"status"`
	Latency   int64  `json:"latency"`
	IsCached  bool   `json:"is_cached"`
	NodeCount int    `json:"node_count"`
	Error     string `json:"error,omitempty"`
}

// GetSourcesHandler returns persisted sources with status calculated from the node file cache.
// If cache miss, triggers async background fetch so next request will have data.
func GetSourcesHandler(w http.ResponseWriter, r *http.Request) {
	configs, err := source.LoadAll()
	if err != nil {
		log.Printf("[SOURCES] LoadAll failed: %v", err)
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}
	log.Printf("[SOURCES] Loaded %d configs from disk", len(configs))

	cacheHits := 0
	asyncFetches := 0

	sources := make([]SourceResponse, len(configs))
	for i, cfg := range configs {
		source.Normalize(&cfg)
		sources[i] = sourceResponseFromConfig(cfg)
		if !cfg.Enabled {
			continue
		}
		runtimeURL := source.RuntimeURL(cfg)
		// Try memory cache first
		if cached, found := cache.Get(runtimeURL); found {
			if item, ok := cached.(provider.CachedItem); ok {
				sources[i].Status = item.Status
				sources[i].IsCached = true
				config, parseErr := parser.ParseYAML(item.Body)
				if parseErr == nil {
					sources[i].NodeCount = provider.ProxyCount(config)
				}
				cacheHits++
			}
		} else if ss, ok := getSourceStatus(runtimeURL); ok {
			// Fallback to status from last fetch
			sources[i].Status = ss.Status
			sources[i].Latency = ss.Latency
			sources[i].IsCached = ss.IsCached
			sources[i].NodeCount = ss.NodeCount
			cacheHits++
		} else {
			// Cache miss: async fetch in background, don't block response
			// Skip if already fetching this URL
			if _, loaded := asyncFetchInFlight.LoadOrStore(runtimeURL, true); !loaded {
				go func(idx int, cfg2 source.Config, url string) {
					defer asyncFetchInFlight.Delete(url)
					log.Printf("[SOURCES] Async fetch: %s", url)
					provider.FetchAndParseYAML(url)
				}(i, cfg, runtimeURL)
			}
			asyncFetches++
		}
	}

	log.Printf("[SOURCES] Response: %d sources, %d cache hits, %d async fetches", len(sources), cacheHits, asyncFetches)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(sources); err != nil {
		log.Printf("[SOURCES] Encode error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func sourceResponseFromConfig(cfg source.Config) SourceResponse {
	return SourceResponse{
		ID:      cfg.ID,
		Name:    cfg.Name,
		Type:    cfg.Type,
		URL:     source.RuntimeURL(cfg),
		UA:      cfg.UA,
		Enabled: cfg.Enabled,
	}
}
