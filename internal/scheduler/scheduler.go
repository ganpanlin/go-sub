package scheduler

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"log/slog"
	"os"
	"time"
)

// Config represents the structure of the config.json file for the scheduler.
type Config struct {
	RefreshIntervalMinutes int `json:"refresh_interval_minutes"`
}

// Start enables the scheduled refresh of all sources.
func Start() {
	file, err := os.ReadFile(appconfig.Get().ConfigPath)
	if err != nil {
		slog.Info("scheduled refresh disabled, config.json not found")
		return
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		slog.Error("scheduler config parse error", "error", err)
		return
	}

	if config.RefreshIntervalMinutes <= 0 {
		slog.Info("scheduled refresh disabled by config")
		return
	}

	interval := time.Duration(config.RefreshIntervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)

	slog.Info("scheduled refresh enabled", "interval_minutes", config.RefreshIntervalMinutes)

	go func() {
		for {
			<-ticker.C
			slog.Info("running scheduled refresh")
			refreshAllSources()
		}
	}()
}

func refreshAllSources() {
	sources, err := source.LoadAll()
	if err != nil {
		slog.Error("scheduler failed to read sources", "error", err)
		return
	}
	slog.Info("sources to refresh", "count", len(sources))

	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		go func(s source.Config) {
			runtimeURL := source.RuntimeURL(s)
			config, status, latency, isCached, err := provider.FetchAndParseYAML(runtimeURL)
			_ = config
			_ = err
			slog.Info("refreshed source", "url", runtimeURL, "status", status, "latency_ms", latency, "cached", isCached)
		}(src)
	}
}
