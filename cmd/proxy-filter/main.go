package main

import (
	"context"
	"flag"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/auth"
	"go-sub/internal/cache"
	"go-sub/internal/datastore"
	"go-sub/internal/router"
	"go-sub/internal/routing"
	"go-sub/internal/rule"
	"go-sub/internal/scheduler"
	"go-sub/internal/settings"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func main() {
	// Define command-line flags
	port := flag.String("port", "8080", "Port to run the server on")
	configPath := flag.String("config", "config.json", "Path to the configuration file")
	dataDir := flag.String("data-dir", "data", "Directory for persistent data files; relative paths are resolved from the config file directory")
	frontendPath := flag.String("frontend-dir", "frontend", "Path to the frontend directory; relative paths are resolved from the config file directory")
	httpTimeout := flag.Int("http-timeout", 10, "HTTP client timeout in seconds")
	cacheTTL := flag.Int("cache-ttl", 60, "Source cache TTL in minutes")
	filterCacheTTL := flag.Int("filter-cache-ttl", 10, "Filter result cache TTL in minutes")
	flag.Parse()

	// Initialize the application configuration
	appconfig.Init(*port, *configPath, *dataDir, *frontendPath, *httpTimeout, *cacheTTL, *filterCacheTTL)

	// Initialize default data if first run (sources.json missing)
	defaultDataDir := findDefaultDataDir()
	if defaultDataDir != "" {
		datastore.InitDefaults(defaultDataDir)
	}

	// Ensure config.json exists in working directory (for scheduler etc.)
	configFile := appconfig.Get().ConfigPath
	if _, err := os.Stat(configFile); err != nil {
		// Try default-data/config.json first, then config.example.json
		for _, src := range []string{
			filepath.Join(defaultDataDir, "config.json"),
			filepath.Join(filepath.Dir(os.Args[0]), "config.example.json"),
			"config.example.json",
		} {
			if data, err := os.ReadFile(src); err == nil {
				os.WriteFile(configFile, data, 0644)
				slog.Info("created config from template", "source", src)
				break
			}
		}
	}

	// Initialize disk cache for source bodies
	cache.InitDiskCache(appconfig.Get().CacheDir())

	// Load app settings from config.json
	settings.LoadAppSettings()

	// Initialize auth
	auth.Init()

	// Initialize rule engine
	rule.InitManager()

	// Initialize routing profiles
	routing.LoadFromConfig()
	routing.EnsureDefault()

	// Ensure rule catalog exists
	routing.EnsureCatalogDefault()

	// Migrate legacy source fields into data/sources.json, then start scheduler.
	settings.MigrateLegacySources()
	go scheduler.Start()

	// Create HTTP server
	r := router.NewRouter()
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", *port),
		Handler: r,
	}

	// Start server in a goroutine
	go func() {
		slog.Info("server started", "addr", "http://localhost:"+*port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("shutting down", "signal", sig)

	// Give outstanding requests 10 seconds to complete
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
	}

	slog.Info("server stopped")
}

// findDefaultDataDir locates the default-data directory for first-run initialization.
func findDefaultDataDir() string {
	// Try relative to executable
	if ex, err := os.Executable(); err == nil {
		d := filepath.Join(filepath.Dir(ex), "default-data")
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	// Try relative to working directory
	if wd, err := os.Getwd(); err == nil {
		d := filepath.Join(wd, "default-data")
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			return d
		}
	}
	// Docker path
	if info, err := os.Stat("/app/default-data"); err == nil && info.IsDir() {
		return "/app/default-data"
	}
	return ""
}
