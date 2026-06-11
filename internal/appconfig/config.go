package appconfig

import (
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Config holds the runtime configuration of the application.
type Config struct {
	Port                  string
	ConfigPath            string // path to base config.json
	DataDir               string // directory for all persistent data
	FrontendPath          string
	HTTPTimeoutSec        int
	CacheTTLMinutes       int
	FilterCacheTTLMinutes int
	RequestUA             string
}

var (
	instance *Config
	once     sync.Once
)

// getBaseDir returns the directory to resolve relative paths against.
// Priority: 1) working directory, 2) executable directory.
func getBaseDir() string {
	// Try working directory first
	if wd, err := os.Getwd(); err == nil && wd != "" {
		return wd
	}
	// Fallback to executable directory
	if ex, err := os.Executable(); err == nil {
		return filepath.Dir(ex)
	}
	return "."
}

func Init(port, configPath, dataDir, frontendPath string, httpTimeoutSec, cacheTTLMinutes, filterCacheTTLMinutes int) {
	once.Do(func() {
		baseDir := getBaseDir()

		// Resolve paths relative to baseDir
		absConfig := resolvePath(baseDir, configPath)
		absDataDir := resolvePath(baseDir, dataDir)
		absFrontendPath := resolvePath(baseDir, frontendPath)

		instance = &Config{
			Port:                  port,
			ConfigPath:            absConfig,
			DataDir:               absDataDir,
			FrontendPath:          absFrontendPath,
			HTTPTimeoutSec:        httpTimeoutSec,
			CacheTTLMinutes:       cacheTTLMinutes,
			FilterCacheTTLMinutes: filterCacheTTLMinutes,
		}

		// Ensure data dirs exist
		os.MkdirAll(absDataDir, 0755)
		os.MkdirAll(filepath.Join(absDataDir, "cache"), 0755)

		slog.Info("config initialized", "base_dir", baseDir)
		slog.Info("config initialized", "data_dir", absDataDir)
		slog.Info("config initialized", "frontend", absFrontendPath)
	})
}

func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Join(baseDir, path)
}

func Get() *Config { return instance }

// DataFile returns the full path for a named data file under data dir.
func (c *Config) DataFile(name string) string {
	return filepath.Join(c.DataDir, name)
}

// CacheDir returns the cache directory path.
func (c *Config) CacheDir() string {
	return filepath.Join(c.DataDir, "cache")
}
