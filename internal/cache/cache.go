package cache

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	memCache = make(map[string]*cacheItem)
	mutex    = &sync.Mutex{}
	diskDir  string
)

type cacheItem struct {
	Value      interface{}
	Expiration int64
}

// diskEntry is the on-disk format: original key + value + expiration.
type diskEntry struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
	Exp   int64       `json:"exp"`
}

// InitDiskCache sets the directory for disk-persisted cache and loads existing items.
func InitDiskCache(dir string) {
	diskDir = dir
	os.MkdirAll(dir, 0755)
	loadFromDisk()
}

// Set stores a value in memory cache only (not persisted to disk).
func Set(key string, value interface{}, ttl time.Duration) {
	mutex.Lock()
	defer mutex.Unlock()
	memCache[key] = &cacheItem{
		Value:      value,
		Expiration: time.Now().Add(ttl).UnixNano(),
	}
}

// SetWithDisk stores a value in memory + persists to disk.
func SetWithDisk(key string, value interface{}, ttl time.Duration) {
	mutex.Lock()
	expiration := time.Now().Add(ttl).UnixNano()
	memCache[key] = &cacheItem{
		Value:      value,
		Expiration: expiration,
	}
	mutex.Unlock()

	if diskDir != "" {
		go persistToDisk(key, value, expiration)
	}
}

// Get returns a value from memory cache.
func Get(key string) (interface{}, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	item, found := memCache[key]
	if !found {
		return nil, false
	}
	if time.Now().UnixNano() > item.Expiration {
		delete(memCache, key)
		if diskDir != "" {
			go removeFromDisk(key)
		}
		return nil, false
	}
	return item.Value, true
}

// GetStale returns a value even if expired (but not deleted). For serving stale while refreshing.
func GetStale(key string) (interface{}, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	item, found := memCache[key]
	if !found {
		return nil, false
	}
	// Don't delete, just return stale value
	return item.Value, true
}

// Delete removes a single key.
func Delete(key string) {
	mutex.Lock()
	defer mutex.Unlock()
	delete(memCache, key)
	if diskDir != "" {
		go removeFromDisk(key)
	}
}

// DeleteByPrefix removes all keys starting with the given prefix from memory and disk.
func DeleteByPrefix(prefix string) {
	mutex.Lock()
	defer mutex.Unlock()
	for key := range memCache {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(memCache, key)
			if diskDir != "" {
				go removeFromDisk(key)
			}
		}
	}
}

// --- Disk persistence ---

func diskPath(key string) string {
	if diskDir == "" {
		return ""
	}
	h := fnvHash(key)
	return filepath.Join(diskDir, fmt.Sprintf("%016x.cache", h))
}

func fnvHash(s string) uint64 {
	var h uint64 = 14695981039346656037
	for _, b := range []byte(s) {
		h ^= uint64(b)
		h *= 1099511628211
	}
	return h
}

func persistToDisk(key string, value interface{}, expiration int64) {
	path := diskPath(key)
	if path == "" {
		return
	}
	entry := diskEntry{Key: key, Value: value, Exp: expiration}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	os.Rename(tmp, path)
}

func removeFromDisk(key string) {
	path := diskPath(key)
	if path != "" {
		os.Remove(path)
	}
}

func loadFromDisk() {
	if diskDir == "" {
		return
	}
	entries, err := os.ReadDir(diskDir)
	if err != nil {
		return
	}
	now := time.Now().UnixNano()
	loaded := 0
	expired := 0

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".cache" {
			continue
		}
		path := filepath.Join(diskDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var de diskEntry
		if err := json.Unmarshal(data, &de); err != nil {
			os.Remove(path)
			continue
		}
		if now > de.Exp {
			os.Remove(path)
			expired++
			continue
		}
		memCache[de.Key] = &cacheItem{Value: de.Value, Expiration: de.Exp}
		loaded++
	}
	if loaded > 0 || expired > 0 {
		slog.Info("cache loaded from disk", "loaded", loaded, "expired", expired)
	}
}
