package handler

import (
	"encoding/json"
	"go-sub/internal/version"
	"net/http"
)

func VersionHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version":    version.Version,
		"commit":     version.Commit,
		"build_time": version.BuildTime,
	})
}
