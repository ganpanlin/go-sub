package healthcheck

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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

// Key normalizes a server:port pair into the store lookup key.
// Exported so the pipeline can build identical keys when filtering.
func Key(server string, port int) string {
	return strings.ToLower(server) + "|" + strconv.Itoa(port)
}

// statusStore is a copy-on-write store: readers operate on an immutable
// snapshot (lock-free via atomic.Pointer), writers swap in a fresh map under
// a mutex. Filtering thousands of nodes costs zero lock acquisitions.
type statusStore struct {
	mu       sync.Mutex // serializes load-from-disk + updates + snapshot swaps
	entries  map[string]StatusEntry
	snapshot atomic.Pointer[map[string]StatusEntry]
	loaded   bool
	gen      atomic.Uint64 // bumped on every UpdateResults; used for cache keys
}

func newStatusStore() *statusStore {
	s := &statusStore{entries: make(map[string]StatusEntry)}
	s.snapshot.Store(&s.entries)
	return s
}

var globalStore = newStatusStore()

// statusFilePath returns the persistence path. Overridden in tests;
// empty disables persistence (in-memory only).
var statusFilePath = func() string {
	if appconfig.Get() == nil {
		return "" // app not initialized: persistence disabled
	}
	return appconfig.Get().DataFile(statusFileName)
}

func statusKey(server string, port int) string {
	return Key(server, port)
}

// UpdateResults merges health check results into the global store, bumps the
// generation counter and persists them. The pipeline reads these via Snapshot
// to implement latency-based filtering.
func UpdateResults(results []NodeResult) {
	globalStore.update(results)
}

// GetStatus returns the last known health status for a server:port endpoint.
func GetStatus(server string, port int) (StatusEntry, bool) {
	return globalStore.get(server, port)
}

// Snapshot returns the current read-only status map. Callers must not mutate
// it. Much faster than per-node GetStatus when filtering thousands of nodes.
func Snapshot() map[string]StatusEntry {
	return globalStore.current()
}

// Generation returns the number of UpdateResults calls so far. The pipeline
// embeds it in cache keys so health changes invalidate cached results.
func Generation() uint64 {
	return globalStore.gen.Load()
}

// ResetStoreForTesting clears the in-memory store (test hook).
func ResetStoreForTesting() {
	globalStore.mu.Lock()
	defer globalStore.mu.Unlock()
	globalStore.entries = make(map[string]StatusEntry)
	globalStore.snapshot.Store(&globalStore.entries)
	globalStore.loaded = true
	globalStore.gen.Store(0)
}

func (s *statusStore) ensureLoaded() {
	if s.loaded {
		return
	}
	s.mu.Lock()
	if s.loaded {
		s.mu.Unlock()
		return
	}
	s.loaded = true
	path := statusFilePath()
	if path == "" {
		s.mu.Unlock()
		return
	}
	data, err := os.ReadFile(path)
	if err == nil {
		var entries []StatusEntry
		if json.Unmarshal(data, &entries) == nil {
			s.entries = make(map[string]StatusEntry, len(entries))
			for _, e := range entries {
				s.entries[statusKey(e.Server, e.Port)] = e
			}
			s.snapshot.Store(&s.entries)
		}
	}
	s.mu.Unlock()
}

func (s *statusStore) update(results []NodeResult) {
	if len(results) == 0 {
		return
	}
	now := time.Now().Unix()
	s.mu.Lock()
	next := make(map[string]StatusEntry, len(s.entries)+len(results))
	for k, v := range s.entries {
		next[k] = v
	}
	for _, r := range results {
		testType := r.TestType
		if testType == "" {
			testType = "tcp"
		}
		next[statusKey(r.Server, r.Port)] = StatusEntry{
			Name:      r.Name,
			Server:    r.Server,
			Port:      r.Port,
			Alive:     r.Alive,
			Latency:   r.Latency,
			CheckedAt: now,
			TestType:  testType,
		}
	}
	s.entries = next
	s.snapshot.Store(&next)
	s.gen.Add(1)
	s.mu.Unlock()
	go persist(statusFilePath(), next)
}

func (s *statusStore) get(server string, port int) (StatusEntry, bool) {
	s.ensureLoaded()
	m := s.snapshot.Load()
	e, ok := (*m)[statusKey(server, port)]
	return e, ok
}

func (s *statusStore) current() map[string]StatusEntry {
	s.ensureLoaded()
	return *s.snapshot.Load()
}

// persist writes the store to disk (best effort, off the hot path).
func persist(path string, entries map[string]StatusEntry) {
	if path == "" {
		return
	}
	list := make([]StatusEntry, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	// Prune stale entries and enforce the cap to keep the file bounded.
	cutoff := time.Now().Add(-maxStatusAge).Unix()
	kept := list[:0]
	for _, e := range list {
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
	globalStore.snapshot.Store(&globalStore.entries)
	globalStore.loaded = true
	return nil
}
