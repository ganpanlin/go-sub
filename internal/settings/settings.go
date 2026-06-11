package settings

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/source"
	"log/slog"
	"os"
)

// LoadAppSettings loads app-level settings from config.json into appconfig.
func LoadAppSettings() {
	configPath := appconfig.Get().ConfigPath
	data, err := os.ReadFile(configPath)
	if err != nil {
		return
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return
	}
	if ua, ok := raw["request_ua"].(string); ok && ua != "" {
		appconfig.Get().RequestUA = ua
	}
}

type legacySourceConfig struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	URL     string `json:"url"`
	Content string `json:"content"`
	UA      string `json:"ua"`
	Enabled *bool  `json:"enabled"`
}

type legacyConfig struct {
	DefaultURLs  []string             `json:"default_urls"`
	SourceConfig []legacySourceConfig `json:"source_configs"`
	SourceNames  map[string]string    `json:"source_names"`
}

// MigrateLegacySources copies old config.json source fields into data/sources.json
// when the new source store is missing or empty.
func MigrateLegacySources() {
	existing, err := source.LoadAll()
	if err == nil && len(existing) > 0 {
		return
	}

	data, readErr := os.ReadFile(appconfig.Get().ConfigPath)
	if readErr != nil {
		return
	}

	var legacy legacyConfig
	if err := json.Unmarshal(data, &legacy); err != nil {
		return
	}

	seen := map[string]bool{}
	var migrated []source.Config
	add := func(cfg source.Config) {
		source.Normalize(&cfg)
		key := source.RuntimeURL(cfg)
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		migrated = append(migrated, cfg)
	}

	for _, item := range legacy.SourceConfig {
		enabled := true
		if item.Enabled != nil {
			enabled = *item.Enabled
		}
		add(source.Config{
			Name:    item.Name,
			Type:    item.Type,
			URL:     item.URL,
			Content: item.Content,
			UA:      item.UA,
			Enabled: enabled,
		})
	}

	for _, rawURL := range legacy.DefaultURLs {
		add(source.Config{
			Name:    legacy.SourceNames[rawURL],
			URL:     rawURL,
			Enabled: true,
		})
	}

	if len(migrated) == 0 {
		return
	}
	if err := source.SaveAll(migrated); err != nil {
		slog.Error("legacy source migration failed", "error", err)
		return
	}
	slog.Info("migrated legacy sources", "count", len(migrated))
}
