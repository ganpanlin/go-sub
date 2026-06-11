package handler

import (
	"encoding/json"
	"go-sub/internal/rule"
	"go-sub/internal/stats"
	"net/http"
	"time"
)

// ProfileStatsHandler returns access statistics for profiles.
//
// GET /api/profiles/stats
// GET /api/profiles/stats?id=xxx
func ProfileStatsHandler(w http.ResponseWriter, r *http.Request) {
	profileID := r.URL.Query().Get("id")
	tracker := stats.GetTracker()

	if profileID != "" {
		// Return stats for a single profile (with recent logs)
		s := tracker.GetStats(profileID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)
		return
	}

	// Return summary for all profiles
	allStats := tracker.GetAllStats()

	type profileStatEntry struct {
		ID          string     `json:"id"`
		Name        string     `json:"name"`
		AccessCount int        `json:"access_count"`
		LastAccess  *jsonTime  `json:"last_access,omitempty"`
	}

	profiles := rule.GetManager().GetAllProfiles()
	profileNames := make(map[string]string)
	for _, p := range profiles {
		profileNames[p.ID] = p.Name
	}

	result := make([]profileStatEntry, 0, len(allStats))
	for id, s := range allStats {
		name := profileNames[id]
		if name == "" {
			name = "(unknown)"
		}
		result = append(result, profileStatEntry{
			ID:          id,
			Name:        name,
			AccessCount: s.AccessCount,
			LastAccess:  wrapJSONTime(s.LastAccess),
		})
	}

	// Also include profiles with zero accesses
	for _, p := range profiles {
		if _, ok := allStats[p.ID]; !ok {
			result = append(result, profileStatEntry{
				ID:          p.ID,
				Name:        p.Name,
				AccessCount: 0,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// jsonTime wraps time.Time for clean JSON output.
type jsonTime struct {
	time.Time
}

func wrapJSONTime(t *time.Time) *jsonTime {
	if t == nil {
		return nil
	}
	return &jsonTime{Time: *t}
}

func (t jsonTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + t.Format(time.RFC3339) + `"`), nil
}
