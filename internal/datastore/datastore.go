package datastore

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var mu sync.Mutex

func dataPath(name string) string {
	return appconfig.Get().DataFile(name)
}

// InitDefaults copies default data files from defaultDataDir if they don't exist in data dir.
// Called on startup to ensure first-run has data.
func InitDefaults(defaultDataDir string) {
	if defaultDataDir == "" {
		return
	}
	files := []string{"sources.json", "profiles.json", "routing.json", "rulesets.json"}
	for _, name := range files {
		target := dataPath(name)
		if _, err := os.Stat(target); err == nil {
			continue // already exists
		}
		src := filepath.Join(defaultDataDir, name)
		if _, err := os.Stat(src); err != nil {
			continue // no default for this file
		}
		data, err := os.ReadFile(src)
		if err != nil {
			log.Printf("[INIT] Failed to read default %s: %v", name, err)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		if err := os.WriteFile(target, data, 0644); err != nil {
			log.Printf("[INIT] Failed to write %s: %v", name, err)
			continue
		}
		log.Printf("[INIT] Created %s from defaults", name)
	}
}

// ReadJSON reads a JSON file into dst.
func ReadJSON(name string, dst interface{}) error {
	data, err := os.ReadFile(dataPath(name))
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// WriteJSON writes src as pretty JSON to a file.
func WriteJSON(name string, src interface{}) error {
	data, err := json.MarshalIndent(src, "", "  ")
	if err != nil {
		return err
	}
	path := dataPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Save safely with lock.
func Save(name string, src interface{}) error {
	mu.Lock()
	defer mu.Unlock()
	return WriteJSON(name, src)
}

// Exists checks if a data file exists.
func Exists(name string) bool {
	_, err := os.Stat(dataPath(name))
	return err == nil
}
