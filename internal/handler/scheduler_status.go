package handler

import (
	"encoding/json"
	"go-sub/internal/scheduler"
	"net/http"
)

// SchedulerStatusHandler exposes the live scheduler chain state
// (refreshing / checking / prewarming / idle + progress) for the admin UI.
func SchedulerStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scheduler.GetChainStatus())
}
