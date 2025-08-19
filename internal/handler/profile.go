package handler

import (
	"encoding/json"
	"go-sub/internal/cache"
	"go-sub/internal/rule"
	"net/http"
)

// GetProfilesHandler returns all profiles, or a single profile when ?id= is provided.
func GetProfilesHandler(w http.ResponseWriter, r *http.Request) {
	if id := r.URL.Query().Get("id"); id != "" {
		p := rule.GetManager().GetProfile(id)
		if p == nil {
			http.Error(w, "Profile not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}
	profiles := rule.GetManager().GetAllProfiles()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(profiles)
}

// CreateProfileHandler creates a new profile.
func CreateProfileHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Name cannot be empty", http.StatusBadRequest)
		return
	}

	p := rule.GetManager().NewProfile(req.Name)
	cache.DeleteByPrefix("sub:")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// UpdateProfileHandler updates a profile.
func UpdateProfileHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	p, err := rule.GetManager().UpdateProfile(id, updates)
	if err != nil {
		if err == rule.ErrProfileNotFound {
			http.Error(w, "Profile not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	cache.DeleteByPrefix("sub:")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// DeleteProfileHandler deletes a profile.
func DeleteProfileHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	if err := rule.GetManager().DeleteProfile(id); err != nil {
		if err == rule.ErrProfileNotFound {
			http.Error(w, "Profile not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	cache.DeleteByPrefix("sub:")
	w.WriteHeader(http.StatusNoContent)
}

// TestScriptHandler validates and tests a JS script against sample data.
func TestScriptHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Script string `json:"script"`
		Nodes  []struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Server string `json:"server"`
			Port   int    `json:"port"`
		} `json:"nodes"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Script == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"ok":    true,
			"error": "",
		})
		return
	}

	results := make([]map[string]interface{}, 0, len(req.Nodes))

	for _, node := range req.Nodes {
		pMap := map[string]interface{}{
			"name":   node.Name,
			"type":   node.Type,
			"server": node.Server,
			"port":   node.Port,
		}

		p := &rule.Profile{Script: req.Script}
		engine, err := rule.NewEngine(p)
		if err != nil {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"ok":    false,
				"error": err.Error(),
			})
			return
		}

		excluded := false
		if engine.Process([]interface{}{pMap}); len(engine.Process([]interface{}{pMap})) == 0 {
			excluded = true
		}

		results = append(results, map[string]interface{}{
			"input":    node,
			"output":   pMap,
			"excluded": excluded,
		})
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"ok":      true,
		"results": results,
	})
}
