package main

import (
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
	"log"
	"net/http"
	"os"
	"path/filepath"
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

	r := router.NewRouter()
	log.Printf("Server is running on http://localhost:%s", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", *port), r))
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
