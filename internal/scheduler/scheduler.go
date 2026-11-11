package scheduler

import (
	"context"
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/healthcheck"
	"go-sub/internal/latency"
	"go-sub/internal/pipeline"
	"go-sub/internal/provider"
	"go-sub/internal/rule"
	"go-sub/internal/source"
	"log/slog"
	"os"
	"time"
)

// Config represents the structure of the config.json file for the scheduler.
type Config struct {
	RefreshIntervalMinutes int `json:"refresh_interval_minutes"`
	// HealthCheckIntervalMinutes: 0 disables scheduled health checks.
	// When enabled, every enabled profile is re-checked and results are
	// persisted, keeping max_latency/drop_dead filtering fresh.
	HealthCheckIntervalMinutes int    `json:"health_check_interval_minutes"`
	HealthCheckMode            string `json:"health_check_mode"`        // "tcp" (default) | "url" (real proxy round trip)
	HealthCheckTimeoutSec      int    `json:"health_check_timeout_sec"` // per-node timeout, default 3 (tcp) / 5 (url)
}

// LoadConfig reads scheduler settings from config.json. Missing file or fields
// yield zero values (all scheduled jobs disabled).
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

// Start enables the scheduled jobs (source refresh, health checks) according
// to config.json. Jobs with a non-positive interval are disabled.
func Start() {
	config := LoadConfig()

	if _, err := os.Stat(appconfig.Get().ConfigPath); err != nil {
		slog.Info("scheduler disabled, config.json not found")
	} else {
		if config.RefreshIntervalMinutes > 0 {
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
		} else {
			slog.Info("scheduled refresh disabled by config")
		}

		if config.HealthCheckIntervalMinutes > 0 {
			mode := config.HealthCheckMode
			if mode != "url" {
				mode = "tcp"
			}
			interval := time.Duration(config.HealthCheckIntervalMinutes) * time.Minute
			ticker := time.NewTicker(interval)
			slog.Info("scheduled health check enabled",
				"interval_minutes", config.HealthCheckIntervalMinutes, "mode", mode)
			go func() {
				for {
					<-ticker.C
					slog.Info("running scheduled health check", "mode", mode)
					runHealthChecks(mode, healthCheckTimeout(config))
				}
			}()
		} else {
			slog.Info("scheduled health check disabled by config")
		}
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

// runHealthChecks re-checks every enabled profile and persists the results.
// The profile's own latency filter is bypassed so previously filtered nodes
// get re-tested too (recovery possible). Subscription output caches are
// invalidated afterwards so the next request applies the fresh data.
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
			results = latency.TestNodes(result.Proxies, "", timeout, 8)
		} else {
			results = healthcheck.CheckNodes(result.Proxies, 20, timeout)
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
	// output so the next request regenerates with the latest results.
	pipeline.Invalidate()
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
