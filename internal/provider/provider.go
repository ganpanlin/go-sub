package provider

import (
	"encoding/base64"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/parser"
	"go-sub/internal/source"
	"io/ioutil"
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

// GetRequestUA returns the User-Agent to use, from config or default.
func GetRequestUA() string {
	cfg := appconfig.Get()
	if cfg != nil && cfg.RequestUA != "" {
		// Check if it's a preset name
		if ua, ok := UAPresets[cfg.RequestUA]; ok {
			return ua
		}
		return cfg.RequestUA // custom UA string
	}
	return UAPresets["clash"] // default to clash
}

type CachedItem struct {
	Body   []byte
	Status int
}

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

func FetchAndParseYAML(yamlURL string) (map[string]interface{}, int, int64, bool, error) {
	ua := source.UAForRuntimeURL(yamlURL)
	return fetchAndParseYAML(yamlURL, false, ua)
}

func FetchAndParseYAMLForce(yamlURL string) (map[string]interface{}, int, int64, bool, error) {
	ua := source.UAForRuntimeURL(yamlURL)
	return fetchAndParseYAML(yamlURL, true, ua)
}

func fetchAndParseYAML(yamlURL string, skipCache bool, overrideUA string) (map[string]interface{}, int, int64, bool, error) {
	// Support local data URL: data:text/plain;base64,<content>
	if strings.HasPrefix(yamlURL, "data:") {
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

	// Try fetching from cache first
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

	client := &http.Client{Timeout: time.Duration(appconfig.Get().HTTPTimeoutSec) * time.Second}
	req, err := http.NewRequest("GET", yamlURL, nil)
	if err != nil {
		return nil, 0, 0, false, err
	}
	ua := GetRequestUA()
	if overrideUA != "" {
		if preset, ok := UAPresets[overrideUA]; ok {
			ua = preset
		} else {
			ua = overrideUA
		}
	}
	req.Header.Set("User-Agent", ua)

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return nil, 0, latency, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, latency, false, fmt.Errorf("HTTP request failed with status code: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, latency, false, err
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
