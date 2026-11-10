package healthcheck

import (
	"encoding/json"
	"fmt"
	"go-sub/internal/appconfig"
	"os"
	"strings"
	"sync"
	"time"
)

// StatusEntry is the persisted result of the last health check for a node endpoint.
type StatusEntry struct {
	Name      string `json:"name"`
	Server    string `json:"server"`
	Port      int    `json:"port"`
	Alive     bool   `json:"alive"`
	Latency   int64  `json:"latency"`             // ms, -1 = timeout/error
	CheckedAt int64  `json:"checked_at"`          // unix seconds
	TestType  string `json:"test_type,omitempty"` // "tcp" | "url"; latest check wins
}

const statusFileName = "health-status.json"

// maxStatusEntries caps the store so churned nodes cannot grow it unbounded.
const maxStatusEntries = 20000

// maxStatusAge prunes entries not re-checked within this window.
const maxStatusAge = 30 * 24 * time.Hour

type statusStore struct {
	mu      sync.RWMutex
	entries map[string]StatusEntry
	loaded  bool
}

var globalStore = &statusStore{entries: make(map[string]StatusEntry)}

// statusFilePath returns the persistence path. Overridden in tests;
// empty disables persistence (in-memory only).
var statusFilePath = func() string {
	if appconfig.Get() == nil {
		return "" // app not initialized: persistence disabled
	}
	return appconfig.Get().DataFile(statusFileName)
}

func statusKey(server string, port int) string {
	return fmt.Sprintf("%s|%d", strings.ToLower(server), port)
}

// UpdateResults merges health check results into the global store and persists them.
// The pipeline reads these via GetStatus to implement latency-based filtering.
func UpdateResults(results []NodeResult) {
	globalStore.update(results)
}

// GetStatus returns the last known health status for a server:port endpoint.
func GetStatus(server string, port int) (StatusEntry, bool) {
	return globalStore.get(server, port)
}

// ResetStoreForTesting clears the in-memory store (test hook).
func ResetStoreForTesting() {
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	globalStore.entries = make(map[string]StatusEntry)
	globalStore.loaded = true
}

func (s *statusStore) update(results []NodeResult) {
	if len(results) == 0 {
		return
	}
	now := time.Now().Unix()
	s.mu.Lock()
	for _, r := range results {
		testType := r.TestType
		if testType == "" {
			testType = "tcp"
		}
		s.entries[statusKey(r.Server, r.Port)] = StatusEntry{
			Name:      r.Name,
			Server:    r.Server,
			Port:      r.Port,
			Alive:     r.Alive,
			Latency:   r.Latency,
			CheckedAt: now,
			TestType:  testType,
		}
	}
	s.mu.Unlock()
	s.persist()
}

func (s *statusStore) get(server string, port int) (StatusEntry, bool) {
	s.loadOnce()
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[statusKey(server, port)]
	return e, ok
}

// loadOnce lazily loads persisted status from disk on first access.
func (s *statusStore) loadOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded {
		return
	}
	s.loaded = true
	path := statusFilePath()
	if path == "" {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return // first run: no file yet
	}
	var entries []StatusEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return
	}
	for _, e := range entries {
		s.entries[statusKey(e.Server, e.Port)] = e
	}
}

// persist writes the store to disk (best effort).
func (s *statusStore) persist() {
	path := statusFilePath()
	if path == "" {
		return
	}
	s.mu.RLock()
	entries := make([]StatusEntry, 0, len(s.entries))
	for _, e := range s.entries {
		entries = append(entries, e)
	}
	s.mu.RUnlock()

	// Prune stale entries and enforce the cap to keep the file bounded.
	cutoff := time.Now().Add(-maxStatusAge).Unix()
	kept := entries[:0]
	for _, e := range entries {
		if e.CheckedAt >= cutoff {
			kept = append(kept, e)
		}
	}
	if len(kept) > maxStatusEntries {
		kept = kept[len(kept)-maxStatusEntries:]
	}

	data, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0644)
}

// LoadFromPath is a test helper that loads status entries from an explicit file.
func LoadFromPath(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var entries []StatusEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	globalStore.entries = make(map[string]StatusEntry, len(entries))
	for _, e := range entries {
		globalStore.entries[statusKey(e.Server, e.Port)] = e
	}
	globalStore.loaded = true
	return nil
}
