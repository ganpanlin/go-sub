package handler

import (
	"encoding/json"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"net/http"
)

// SourceDataHandler returns the parsed proxies from a source URL.
// GET /api/sources/data?id=...
// First checks cache, only fetches remote if cache miss.
func SourceDataHandler(w http.ResponseWriter, r *http.Request) {
	ref := r.URL.Query().Get("id")
	if ref == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
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
	url := source.RuntimeURL(cfg)

	// Try cache first, fetch only if missing
	config, status, latency, isCached, err := provider.FetchAndParseYAML(url)
	updateSourceStatusFromConfig(url, config, status, latency, isCached, err)

	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"url":     url,
			"status":  status,
			"error":   err.Error(),
			"proxies": []interface{}{},
			"total":   0,
		})
		return
	}

	proxies := []interface{}{}
	total := 0
	if config != nil && config["proxies"] != nil {
		if p, ok := config["proxies"].([]interface{}); ok {
			proxies = p
			total = len(p)
		}
	}

	groups := 0
	if config != nil && config["proxy-groups"] != nil {
		if g, ok := config["proxy-groups"].([]interface{}); ok {
			groups = len(g)
		}
	}

	rules := 0
	if config != nil && config["rules"] != nil {
		if r, ok := config["rules"].([]interface{}); ok {
			rules = len(r)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if isCached {
		w.Header().Set("X-Cache", "HIT")
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"url":          url,
		"status":       status,
		"latency":      latency,
		"is_cached":    isCached,
		"proxies":      proxies,
		"total":        total,
		"proxy_groups": groups,
		"rules":        rules,
	})
}
