package handler

import (
	"encoding/json"
	"go-sub/internal/geoip"
	"net/http"
)

// GeoIPHandler resolves an IP or domain to its region.
// GET /api/geoip?host=1.2.3.4
func GeoIPHandler(w http.ResponseWriter, r *http.Request) {
	host := r.URL.Query().Get("host")
	if host == "" {
		http.Error(w, "Missing 'host' parameter", http.StatusBadRequest)
		return
	}

	region := geoip.LookupRegion(host)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"host":   host,
		"region": region,
	})
}
