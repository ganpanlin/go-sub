package auth

import (
	"go-sub/internal/datastore"
	"sync"
)

// SaveConfigSafe persists auth config to data/auth.json and reloads in-memory.
var saveMu sync.Mutex

func SaveConfigSafe(c Config) error {
	saveMu.Lock()
	defer saveMu.Unlock()

	if err := datastore.Save("auth.json", c); err != nil {
		return err
	}

	// Update in-memory config
	mu.Lock()
	cfg = c
	if cfg.Username == "" {
		cfg.Username = "admin"
	}
	mu.Unlock()
	return nil
}
