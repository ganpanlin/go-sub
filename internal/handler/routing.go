package handler

import (
	"encoding/json"
	"go-sub/internal/cache"
	"go-sub/internal/routing"
	"net/http"
)

// RoutingCatalogHandler returns the rule catalog for checkbox selection.
func RoutingCatalogHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routing.GetCatalog())
}

// RoutingListHandler returns all routing profiles.
func RoutingListHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routing.GetManager().List())
}

// RoutingAddHandler creates a new routing profile.
func RoutingAddHandler(w http.ResponseWriter, r *http.Request) {
	var p routing.RoutingProfile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if p.Name == "" {
		http.Error(w, "Name is required", http.StatusBadRequest)
		return
	}
	if err := routing.GetManager().Add(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

// RoutingUpdateHandler updates a routing profile. Uses ?id= query param.
func RoutingUpdateHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}
	var p routing.RoutingProfile
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	p.ID = id
	if err := routing.GetManager().Update(&p); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cache.DeleteByPrefix("sub:")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

// RoutingDeleteHandler deletes a routing profile. Uses ?id= query param.
func RoutingDeleteHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing id", http.StatusBadRequest)
		return
	}
	if id == routing.DefaultRoutingID {
		http.Error(w, "Default routing cannot be deleted", http.StatusBadRequest)
		return
	}
	if err := routing.GetManager().Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	cache.DeleteByPrefix("sub:")
	w.WriteHeader(http.StatusNoContent)
}
