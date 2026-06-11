package provider

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/parser"
	"go-sub/internal/source"
	"io"
	"net/http"
	"strings"
	"time"
)

// Predefined User-Agent presets for subscription fetching.
var UAPresets = map[string]string{
	"clash":        "clash-verge/v2.2.0 Mihomo/1.18.1",
	"mihomo":       "Mihomo/1.18.1",
	"surge":        "Surge/5.0",
	"shadowrocket": "Shadowrocket/1908 CFNetwork/1404.0.5",
	"quantumult":   "Quantumult%20X/1.3.0",
	"loon":         "Loon/3.2.4",
	"browser":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
}

// DefaultRetryCount is the number of retries for transient network errors.
const DefaultRetryCount = 2

// DefaultRetryBackoff is the base delay between retries (doubles each attempt).
var DefaultRetryBackoff = time.Second

// GetRequestUA returns the User-Agent to use, from config or default.
func GetRequestUA() string {
	cfg := appconfig.Get()
	if cfg != nil && cfg.RequestUA != "" {
		if ua, ok := UAPresets[cfg.RequestUA]; ok {
			return ua
		}
		return cfg.RequestUA
	}
	return UAPresets["clash"]
}

// CachedItem stores the raw HTTP response for disk caching.
type CachedItem struct {
	Body   []byte
	Status int
}

// ProxyCount returns the number of proxies in a parsed config map.
func ProxyCount(config map[string]interface{}) int {
	if config == nil || config["proxies"] == nil {
		return 0
	}
	proxies, ok := config["proxies"].([]interface{})
	if !ok {
		return 0
	}
	return len(proxies)
}

// FetchAndParseYAML fetches and parses a source URL with caching and retry.
// Uses context.Background() and DefaultRetryCount retries.
func FetchAndParseYAML(yamlURL string) (map[string]interface{}, int, int64, bool, error) {
	ua := source.UAForRuntimeURL(yamlURL)
	return fetchAndParseYAML(context.Background(), yamlURL, false, ua, DefaultRetryCount)
}

// FetchAndParseYAMLForce forces a fresh fetch (skips cache) with retry.
func FetchAndParseYAMLForce(yamlURL string) (map[string]interface{}, int, int64, bool, error) {
	ua := source.UAForRuntimeURL(yamlURL)
	return fetchAndParseYAML(context.Background(), yamlURL, true, ua, DefaultRetryCount)
}

// FetchAndParseYAMLCtx fetches with a caller-provided context and retry count.
func FetchAndParseYAMLCtx(ctx context.Context, yamlURL string, maxRetries int) (map[string]interface{}, int, int64, bool, error) {
	ua := source.UAForRuntimeURL(yamlURL)
	return fetchAndParseYAML(ctx, yamlURL, false, ua, maxRetries)
}

// fetchAndParseYAML is the core implementation with context, caching, and retry.
func fetchAndParseYAML(ctx context.Context, yamlURL string, skipCache bool, overrideUA string, maxRetries int) (map[string]interface{}, int, int64, bool, error) {
	// --- data: URL (local content) ---
	if strings.HasPrefix(yamlURL, "data:") {
		return parseDataURL(ctx, yamlURL)
	}

	// --- Cache lookup ---
	if !skipCache {
		if cached, found := cache.Get(yamlURL); found {
			if item, ok := cached.(CachedItem); ok {
				start := time.Now()
				config, err := parser.ParseYAML(item.Body)
				latency := time.Since(start).Milliseconds()
				if err != nil {
					return nil, item.Status, latency, true, err
				}
				return config, item.Status, latency, true, nil
			}
		}
	}

	// --- HTTP fetch with retry ---
	ua := GetRequestUA()
	if overrideUA != "" {
		if preset, ok := UAPresets[overrideUA]; ok {
			ua = preset
		} else {
			ua = overrideUA
		}
	}

	var (
		lastErr  error
		lastStat int
		body     []byte
		latency  int64
	)

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := DefaultRetryBackoff * (1 << (attempt - 1)) // 1s, 2s, 4s...
			select {
			case <-ctx.Done():
				return nil, 0, 0, false, ctx.Err()
			case <-time.After(backoff):
			}
		}

		config, status, lat, cached, err := doHTTPRequest(ctx, yamlURL, ua)
		if err == nil {
			return config, status, lat, cached, nil
		}

		lastErr = err
		lastStat = status
		if lat > 0 {
			latency = lat
		}

		// Don't retry on client errors (4xx) — the server won't give a different answer.
		if status >= 400 && status < 500 {
			break
		}
		// Don't retry if context was cancelled.
		if ctx.Err() != nil {
			return nil, lastStat, latency, false, ctx.Err()
		}
	}

	if body == nil {
		return nil, lastStat, latency, false, lastErr
	}
	return nil, lastStat, latency, false, lastErr
}

// doHTTPRequest performs a single HTTP GET, parses and caches the result.
// On TLS certificate errors, automatically retries once with InsecureSkipVerify.
func doHTTPRequest(ctx context.Context, yamlURL, ua string) (map[string]interface{}, int, int64, bool, error) {
	config, status, latency, cached, err := doHTTPRequestOnce(ctx, yamlURL, ua, false)
	if err != nil && isTLSError(err) {
		// Retry with TLS verification disabled
		config, status, latency, cached, err = doHTTPRequestOnce(ctx, yamlURL, ua, true)
	}
	return config, status, latency, cached, err
}

// doHTTPRequestOnce performs a single HTTP GET attempt.
func doHTTPRequestOnce(ctx context.Context, yamlURL, ua string, skipTLSVerify bool) (map[string]interface{}, int, int64, bool, error) {
	timeout := time.Duration(appconfig.Get().HTTPTimeoutSec) * time.Second
	client := &http.Client{
		Timeout: timeout,
	}
	if skipTLSVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", yamlURL, nil)
	if err != nil {
		return nil, 0, 0, false, err
	}
	req.Header.Set("User-Agent", ua)

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, 0, latency, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, latency, false, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, latency, false, fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	// Cache the successful response
	cache.SetWithDisk(yamlURL, CachedItem{Body: body, Status: resp.StatusCode}, time.Duration(appconfig.Get().CacheTTLMinutes)*time.Minute)

	// Invalidate filter caches because the source data has changed
	cache.DeleteByPrefix("filter:")

	config, err := parser.ParseYAML(body)
	if err != nil {
		return nil, resp.StatusCode, latency, false, err
	}

	return config, resp.StatusCode, latency, false, nil
}

// isTLSError checks if an error is caused by TLS/certificate verification failure.
func isTLSError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "certificate") ||
		strings.Contains(msg, "x509:") ||
		strings.Contains(msg, "TLS handshake")
}

// parseDataURL handles data:text/plain;base64,<content> URLs (local subscriptions).
func parseDataURL(_ context.Context, yamlURL string) (map[string]interface{}, int, int64, bool, error) {
	start := time.Now()
	idx := strings.Index(yamlURL, ",")
	if idx < 0 {
		return nil, 0, 0, false, fmt.Errorf("invalid data url")
	}
	payload := yamlURL[idx+1:]
	var body []byte
	var err error
	if strings.Contains(yamlURL[:idx], ";base64") {
		body, err = base64.StdEncoding.DecodeString(payload)
	} else {
		body = []byte(payload)
	}
	if err != nil {
		return nil, 0, time.Since(start).Milliseconds(), false, err
	}
	cache.SetWithDisk(yamlURL, CachedItem{Body: body, Status: http.StatusOK}, time.Duration(appconfig.Get().CacheTTLMinutes)*time.Minute)
	config, err := parser.ParseYAML(body)
	return config, http.StatusOK, time.Since(start).Milliseconds(), false, err
}
