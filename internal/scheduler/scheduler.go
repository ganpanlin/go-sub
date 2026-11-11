package scheduler

import (
	"context"
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/healthcheck"
	"go-sub/internal/latency"
	"go-sub/internal/pipeline"
	"go-sub/internal/provider"
	"go-sub/internal/rule"
	"go-sub/internal/source"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Config represents the structure of the config.json file for the scheduler.
type Config struct {
	RefreshIntervalMinutes int `json:"refresh_interval_minutes"` // extra standalone refresh cycle; 0 = off
	// HealthCheckIntervalMinutes drives the periodic chain
	// (refresh sources → health check → pre-warm subscription caches).
	// nil (absent) = default 60 minutes; 0 = disabled.
	HealthCheckIntervalMinutes *int   `json:"health_check_interval_minutes"`
	HealthCheckMode            string `json:"health_check_mode"`        // "tcp" (default) | "url" (real proxy round trip)
	HealthCheckTimeoutSec      int    `json:"health_check_timeout_sec"` // per-node timeout, default 3 (tcp) / 5 (url)
}

const defaultHealthCheckIntervalMinutes = 60

// HealthIntervalOrDefault resolves the health-check cycle interval.
func (c Config) HealthIntervalOrDefault() int {
	if c.HealthCheckIntervalMinutes == nil {
		return defaultHealthCheckIntervalMinutes
	}
	return *c.HealthCheckIntervalMinutes
}

// LoadConfig reads scheduler settings from config.json. Missing file or fields
// fall back to defaults (health-check chain every 60 minutes).
func LoadConfig() Config {
	var config Config
	file, err := os.ReadFile(appconfig.Get().ConfigPath)
	if err != nil {
		return config
	}
	if err := json.Unmarshal(file, &config); err != nil {
		slog.Error("scheduler config parse error", "error", err)
	}
	return config
}

// Start launches the scheduled jobs:
//   - immediately after startup: refresh sources → health check → pre-warm
//   - every HealthCheckIntervalMinutes (default 60): the same chain
//   - optional extra standalone source refresh (refresh_interval_minutes)
func Start() {
	config := LoadConfig()
	healthInterval := config.HealthIntervalOrDefault()
	healthMode := config.HealthCheckMode
	if healthMode != "url" {
		healthMode = "tcp"
	}

	chain := func() {
		refreshAllSourcesSync()
		runHealthChecks(healthMode, healthCheckTimeout(config))
		prewarmSubscriptionCaches()
	}

	// Startup: fetch subscriptions, then immediately latency-test and pre-warm.
	go chain()

	if healthInterval > 0 {
		ticker := time.NewTicker(time.Duration(healthInterval) * time.Minute)
		slog.Info("scheduled chain enabled (refresh → health check → pre-warm)",
			"interval_minutes", healthInterval, "mode", healthMode)
		go func() {
			for {
				<-ticker.C
				slog.Info("running scheduled chain", "mode", healthMode)
				chain()
			}
		}()
	} else {
		slog.Info("periodic health check disabled by config")
	}

	if config.RefreshIntervalMinutes > 0 {
		ticker := time.NewTicker(time.Duration(config.RefreshIntervalMinutes) * time.Minute)
		slog.Info("standalone scheduled refresh enabled", "interval_minutes", config.RefreshIntervalMinutes)
		go func() {
			for {
				<-ticker.C
				slog.Info("running scheduled refresh")
				refreshAllSourcesSync()
			}
		}()
	}
}

func healthCheckTimeout(config Config) time.Duration {
	if config.HealthCheckTimeoutSec > 0 {
		return time.Duration(config.HealthCheckTimeoutSec) * time.Second
	}
	if config.HealthCheckMode == "url" {
		return 5 * time.Second
	}
	return 3 * time.Second
}

// refreshAllSourcesSync refreshes all enabled sources and waits for completion
// so the health check always runs against freshly pulled subscriptions.
func refreshAllSourcesSync() {
	sources, err := source.LoadAll()
	if err != nil {
		slog.Error("scheduler failed to read sources", "error", err)
		return
	}
	slog.Info("scheduled refresh start", "sources", len(sources))
	start := time.Now()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		wg.Add(1)
		go func(s source.Config) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			runtimeURL := source.RuntimeURL(s)
			config, status, lat, isCached, err := provider.FetchAndParseYAML(runtimeURL)
			if err != nil {
				slog.Warn("scheduled refresh failed", "url", runtimeURL, "error", err)
				return
			}
			slog.Info("scheduled refresh done", "url", runtimeURL, "status", status,
				"nodes", provider.ProxyCount(config), "latency_ms", lat, "cached", isCached)
		}(src)
	}
	wg.Wait()
	slog.Info("scheduled refresh complete", "elapsed_ms", time.Since(start).Milliseconds())
}

// runHealthChecks re-checks every enabled profile and persists the latency
// values to data/health-status.json. The profile's own latency filter is
// bypassed so previously filtered nodes get re-tested too (recovery possible).
func runHealthChecks(mode string, timeout time.Duration) {
	mgr := rule.GetManager()
	if mgr == nil {
		return
	}
	profiles := mgr.GetAllProfiles()
	for _, p := range profiles {
		if !p.Enabled {
			continue
		}
		testProfile := *p
		testProfile.MaxLatency = 0
		testProfile.DropDead = false

		result, err := pipeline.RunCached(context.Background(), &testProfile)
		if err != nil {
			slog.Warn("scheduled health check skipped", "profile", p.Name, "error", err)
			continue
		}

		var results []healthcheck.NodeResult
		if mode == "url" {
			results = latency.TestNodes(result.Proxies, "", timeout, 16)
		} else {
			results = healthcheck.CheckNodes(result.Proxies, 100, timeout)
		}
		healthcheck.UpdateResults(results)

		alive := 0
		for _, r := range results {
			if r.Alive {
				alive++
			}
		}
		slog.Info("scheduled health check done",
			"profile", p.Name, "total", len(results), "alive", alive, "mode", mode)
	}
	// Fresh health data changes filter outcomes — drop cached subscription
	// output so the next request applies the fresh data.
	cache.DeleteByPrefix("sub:")
}

// prewarmSubscriptionCaches rebuilds the pipeline result for every enabled
// profile right after the health check, so client requests hit warm caches
// with the freshest latency filtering already applied.
func prewarmSubscriptionCaches() {
	mgr := rule.GetManager()
	if mgr == nil {
		return
	}
	for _, p := range mgr.GetAllProfiles() {
		if !p.Enabled {
			continue
		}
		if _, err := pipeline.RunCached(context.Background(), p); err != nil {
			slog.Warn("pre-warm failed", "profile", p.Name, "error", err)
			continue
		}
		slog.Info("subscription cache pre-warmed", "profile", p.Name)
	}
}
