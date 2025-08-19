package handler

import (
	"go-sub/internal/cache"
	"go-sub/internal/provider"
	"sync"
)

// sourceStatus tracks per-source runtime status (populated by fetch operations).
type sourceStatusEntry struct {
	Status    int   `json:"status"`
	Latency   int64 `json:"latency"`
	IsCached  bool  `json:"is_cached"`
	NodeCount int   `json:"node_count"`
}

var (
	sourceStatusMap = make(map[string]sourceStatusEntry)
	sourceStatusMu  sync.RWMutex
)

// updateSourceStatusFromConfig updates the runtime status for a source and returns node count.
func updateSourceStatusFromConfig(url string, config map[string]interface{}, status int, latency int64, isCached bool, err error) int {
	nc := provider.ProxyCount(config)
	sourceStatusMu.Lock()
	sourceStatusMap[url] = sourceStatusEntry{
		Status:    status,
		Latency:   latency,
		IsCached:  isCached,
		NodeCount: nc,
	}
	sourceStatusMu.Unlock()
	_ = cache.Get // keep import
	return nc
}

// getSourceStatus returns the cached status for a source, if available.
func getSourceStatus(url string) (sourceStatusEntry, bool) {
	sourceStatusMu.RLock()
	s, ok := sourceStatusMap[url]
	sourceStatusMu.RUnlock()
	return s, ok
}
