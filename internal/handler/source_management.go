package handler

import (
	"encoding/json"
	"go-sub/internal/source"
	"net/http"
	"sync"
)

type BodyRequest struct {
	Name    string `json:"name"`
	ID      string `json:"id"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Content string `json:"content"`
	UA      string `json:"ua"`
	Enabled *bool  `json:"enabled"`
}

var configMutex = &sync.Mutex{}

func sourceRef(req BodyRequest) string {
	return req.ID
}

// --- Handlers ---

// AddSourceHandler handles adding a new source URL.
func AddSourceHandler(w http.ResponseWriter, r *http.Request) {
	var req BodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" && req.Content == "" {
		http.Error(w, "URL or content cannot be empty", http.StatusBadRequest)
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	sources, err := source.LoadAll()
	if err != nil {
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}

	cfg := source.Config{
		Name:    req.Name,
		Type:    req.Type,
		URL:     req.URL,
		Content: req.Content,
		UA:      req.UA,
		Enabled: true,
	}
	if req.Enabled != nil {
		cfg.Enabled = *req.Enabled
	}
	source.Normalize(&cfg)
	runtimeURL := source.RuntimeURL(cfg)

	// Check for duplicates
	for _, s := range sources {
		if s.ID == cfg.ID || source.RuntimeURL(s) == runtimeURL {
			w.WriteHeader(http.StatusConflict)
			return
		}
	}

	sources = append(sources, cfg)

	if err := source.SaveAll(sources); err != nil {
		http.Error(w, "Failed to write sources file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(sourceResponseFromConfig(cfg))
}

// DeleteSourceHandler handles deleting a source URL.
func DeleteSourceHandler(w http.ResponseWriter, r *http.Request) {
	var req BodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ref := sourceRef(req)
	if ref == "" {
		http.Error(w, "ID cannot be empty", http.StatusBadRequest)
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	sources, err := source.LoadAll()
	if err != nil {
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}

	var newSources []source.Config
	found := false
	for _, s := range sources {
		if s.ID == ref {
			found = true
			continue
		}
		newSources = append(newSources, s)
	}

	if !found {
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}

	if err := source.SaveAll(newSources); err != nil {
		http.Error(w, "Failed to write sources file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// UpdateSourceHandler handles updating source name/UA.
func UpdateSourceHandler(w http.ResponseWriter, r *http.Request) {
	var req BodyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	ref := sourceRef(req)
	if ref == "" {
		http.Error(w, "ID cannot be empty", http.StatusBadRequest)
		return
	}

	configMutex.Lock()
	defer configMutex.Unlock()

	sources, err := source.LoadAll()
	if err != nil {
		http.Error(w, "Failed to read sources file", http.StatusInternalServerError)
		return
	}

	found := false
	var cfg source.Config
	for i, s := range sources {
		if s.ID == ref {
			if req.Name != "" {
				sources[i].Name = req.Name
			}
			if req.Type != "" {
				sources[i].Type = req.Type
			}
			if req.URL != "" {
				sources[i].URL = req.URL
			}
			if req.Content != "" {
				sources[i].Content = req.Content
			}
			sources[i].UA = req.UA
			if req.Enabled != nil {
				sources[i].Enabled = *req.Enabled
			}
			source.Normalize(&sources[i])
			cfg = sources[i]
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "Source not found", http.StatusNotFound)
		return
	}

	if err := source.SaveAll(sources); err != nil {
		http.Error(w, "Failed to write sources file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(sourceResponseFromConfig(cfg))
}
