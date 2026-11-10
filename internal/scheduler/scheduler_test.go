package scheduler

import (
	"go-sub/internal/appconfig"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupConfig(t *testing.T, content string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if content != "" {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	appconfig.Init("0", path, filepath.Join(dir, "data"), "frontend", 10, 60, 10)
	t.Cleanup(func() { os.RemoveAll(dir) })
}

func TestLoadConfigFull(t *testing.T) {
	setupConfig(t, `{
		"refresh_interval_minutes": 120,
		"health_check_interval_minutes": 360,
		"health_check_mode": "url",
		"health_check_timeout_sec": 6
	}`)
	c := LoadConfig()
	if c.RefreshIntervalMinutes != 120 {
		t.Fatalf("RefreshIntervalMinutes = %d, want 120", c.RefreshIntervalMinutes)
	}
	if c.HealthCheckIntervalMinutes != 360 {
		t.Fatalf("HealthCheckIntervalMinutes = %d, want 360", c.HealthCheckIntervalMinutes)
	}
	if c.HealthCheckMode != "url" {
		t.Fatalf("HealthCheckMode = %q, want url", c.HealthCheckMode)
	}
	if c.HealthCheckTimeoutSec != 6 {
		t.Fatalf("HealthCheckTimeoutSec = %d, want 6", c.HealthCheckTimeoutSec)
	}
	if got := healthCheckTimeout(c); got != 6*time.Second {
		t.Fatalf("healthCheckTimeout = %v, want 6s", got)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	setupConfig(t, "")
	c := LoadConfig()
	if c.RefreshIntervalMinutes != 0 || c.HealthCheckIntervalMinutes != 0 {
		t.Fatalf("missing config should yield zero values, got %+v", c)
	}
	// Jobs stay disabled on zero config; timeout helper falls back per mode.
	if got := healthCheckTimeout(Config{}); got.String() != "3s" {
		t.Fatalf("default tcp timeout = %v, want 3s", got)
	}
	if got := healthCheckTimeout(Config{HealthCheckMode: "url"}); got.String() != "5s" {
		t.Fatalf("default url timeout = %v, want 5s", got)
	}
}

func TestLoadConfigInvalidJSON(t *testing.T) {
	setupConfig(t, `{bad json`)
	c := LoadConfig()
	if c.RefreshIntervalMinutes != 0 || c.HealthCheckIntervalMinutes != 0 {
		t.Fatalf("invalid config should yield zero values, got %+v", c)
	}
}

func TestHealthCheckModeNormalization(t *testing.T) {
	// Mode normalization happens in Start(); the timeout helper treats
	// anything not "url" as tcp.
	if got := healthCheckTimeout(Config{HealthCheckMode: "weird", HealthCheckTimeoutSec: 2}); got.String() != "2s" {
		t.Fatalf("explicit timeout should win, got %v", got)
	}
}
