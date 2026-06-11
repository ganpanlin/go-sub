package stats

import (
	"sync"
	"time"
)

// AccessEntry records a single subscription access.
type AccessEntry struct {
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	ClientType string    `json:"client_type"`
	NodeCount  int       `json:"node_count"`
	AccessedAt time.Time `json:"accessed_at"`
}

// ProfileStats tracks access statistics for a single profile.
type ProfileStats struct {
	AccessCount int          `json:"access_count"`
	LastAccess  *time.Time   `json:"last_access,omitempty"`
	RecentLogs  []AccessEntry `json:"recent_logs,omitempty"`
}

// Tracker tracks subscription access across all profiles.
type Tracker struct {
	mu       sync.RWMutex
	counts   map[string]*ProfileStats  // key = profile ID
	maxLogs  int
}

var global *Tracker

func init() {
	global = &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 100, // keep last 100 accesses per profile
	}
}

// GetTracker returns the global access tracker.
func GetTracker() *Tracker {
	return global
}

// Record records a subscription access event.
func (t *Tracker) Record(profileID, ip, userAgent, clientType string, nodeCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()

	stats, ok := t.counts[profileID]
	if !ok {
		stats = &ProfileStats{
			RecentLogs: make([]AccessEntry, 0, t.maxLogs),
		}
		t.counts[profileID] = stats
	}

	stats.AccessCount++
	stats.LastAccess = &now

	// Append to ring buffer (drop oldest if over capacity)
	entry := AccessEntry{
		IP:         ip,
		UserAgent:  userAgent,
		ClientType: clientType,
		NodeCount:  nodeCount,
		AccessedAt: now,
	}
	stats.RecentLogs = append(stats.RecentLogs, entry)
	if len(stats.RecentLogs) > t.maxLogs {
		stats.RecentLogs = stats.RecentLogs[len(stats.RecentLogs)-t.maxLogs:]
	}
}

// GetStats returns access statistics for a profile.
func (t *Tracker) GetStats(profileID string) *ProfileStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats, ok := t.counts[profileID]
	if !ok {
		return &ProfileStats{AccessCount: 0}
	}

	// Return a copy to avoid data races
	result := &ProfileStats{
		AccessCount: stats.AccessCount,
		LastAccess:  stats.LastAccess,
	}
	if len(stats.RecentLogs) > 0 {
		result.RecentLogs = make([]AccessEntry, len(stats.RecentLogs))
		copy(result.RecentLogs, stats.RecentLogs)
	}
	return result
}

// GetAllStats returns a summary of access counts for all profiles.
func (t *Tracker) GetAllStats() map[string]*ProfileStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]*ProfileStats, len(t.counts))
	for k, v := range t.counts {
		s := &ProfileStats{
			AccessCount: v.AccessCount,
			LastAccess:  v.LastAccess,
		}
		result[k] = s
	}
	return result
}
