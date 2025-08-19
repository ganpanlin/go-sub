package scheduler

import (
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/provider"
	"go-sub/internal/source"
	"log"
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
		log.Printf("Could not read config.json, scheduled refresh is disabled.")
		return
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		log.Printf("Error parsing config.json for scheduler settings: %v", err)
		return
	}

	if config.RefreshIntervalMinutes <= 0 {
		log.Println("Scheduled refresh is disabled as per config.")
		return
	}

	interval := time.Duration(config.RefreshIntervalMinutes) * time.Minute
	ticker := time.NewTicker(interval)

	log.Printf("Scheduled refresh enabled. Will run every %d minutes.", config.RefreshIntervalMinutes)

	go func() {
		for {
			<-ticker.C
			log.Println("Running scheduled refresh of all sources...")
			refreshAllSources()
		}
	}()
}

func refreshAllSources() {
	sources, err := source.LoadAll()
	if err != nil {
		log.Printf("Could not read sources.json for scheduled refresh: %v", err)
		return
	}
	log.Printf("Found %d sources to refresh.", len(sources))

	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		go func(s source.Config) {
			runtimeURL := source.RuntimeURL(s)
			config, status, latency, isCached, err := provider.FetchAndParseYAML(runtimeURL)
			_ = config
			_ = err
			log.Printf("Refreshed source %s, status: %d, latency: %dms, cached: %t", runtimeURL, status, latency, isCached)
		}(src)
	}
}
