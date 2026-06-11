package handler

import (
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/parser"
	"go-sub/internal/provider"
	"go-sub/internal/proxy"
	"go-sub/internal/source"
	"gopkg.in/yaml.v3"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

func FilterHandler(w http.ResponseWriter, r *http.Request) {
	queryParams := r.URL.Query()
	yamlURLParam := queryParams.Get("url")
	nameFilter := queryParams.Get("name")
	typeFilter := queryParams.Get("type")
	serverFilter := queryParams.Get("server")

	// Create a cache key based on the filter parameters.
	// If a URL is provided, it becomes part of the key.
	filterCacheKey := fmt.Sprintf("filter:url=%s,name=%s,type=%s,server=%s", yamlURLParam, nameFilter, typeFilter, serverFilter)

	// Check if the final result is already cached.
	if cached, found := cache.Get(filterCacheKey); found {
		if yamlBytes, ok := cached.([]byte); ok {
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.Header().Set("X-Cache", "HIT") // Custom header to indicate cache hit
			w.Write(yamlBytes)
			return
		}
	}

	var mergedProxies []interface{}
	var firstConfig map[string]interface{}
	sourceURLInfo := []string{}

	if yamlURLParam != "" {
		// Logic for handling specific URLs passed in the query parameter
		yamlURLs := strings.Split(yamlURLParam, ",")
		for _, yamlURL := range yamlURLs {
			config, status, latency, isCached, err := provider.FetchAndParseYAML(yamlURL)
			updateSourceStatusFromConfig(yamlURL, config, status, latency, isCached, err)

			if err != nil {
				sourceURLInfo = append(sourceURLInfo, fmt.Sprintf("%s (Error: %s)", yamlURL, err.Error()))
				continue
			}

			if firstConfig == nil && config != nil {
				firstConfig = config
			}

			if config != nil && config["proxies"] != nil {
				if proxies, ok := config["proxies"].([]interface{}); ok {
					mergedProxies = append(mergedProxies, proxies...)
					sourceURLInfo = append(sourceURLInfo, fmt.Sprintf("%s (%d nodes)", yamlURL, len(proxies)))
				}
			}
		}
	} else {
		// Logic for handling request without specific URLs, using persisted enabled sources.
		sources, err := source.LoadAll()
		if err != nil {
			http.Error(w, "Error: Failed to read sources file", http.StatusInternalServerError)
			return
		}
		sourceURLInfo = append(sourceURLInfo, "Using all enabled sources")

		for _, src := range sources {
			if !src.Enabled {
				continue
			}
			runtimeURL := source.RuntimeURL(src)

			if cached, found := cache.Get(runtimeURL); found {
				if item, ok := cached.(provider.CachedItem); ok {
					config, err := parser.ParseYAML(item.Body)
					if err != nil {
						slog.Error("error parsing cached content", "url", runtimeURL, "error", err)
						continue
					}

					if firstConfig == nil && config != nil {
						firstConfig = config
					}

					if config != nil && config["proxies"] != nil {
						if proxies, ok := config["proxies"].([]interface{}); ok {
							mergedProxies = append(mergedProxies, proxies...)
						}
					}
				}
				continue
			}

			config, _, _, _, err := provider.FetchAndParseYAML(runtimeURL)
			if err != nil {
				slog.Error("error fetching source", "url", runtimeURL, "error", err)
				continue
			}
			if firstConfig == nil && config != nil {
				firstConfig = config
			}
			if config != nil && config["proxies"] != nil {
				if proxies, ok := config["proxies"].([]interface{}); ok {
					mergedProxies = append(mergedProxies, proxies...)
				}
			}
		}
	}

	if firstConfig == nil {
		http.Error(w, "Error: No valid configuration found from the provided URLs or cache", http.StatusBadRequest)
		return
	}

	if len(mergedProxies) == 0 {
		http.Error(w, "Error: No proxies found in the configurations", http.StatusBadRequest)
		return
	}

	// The rest of the logic remains the same (deduplication, filtering, etc.)
	beforeDedupeCount := len(mergedProxies)
	mergedProxies = proxy.DeduplicateProxies(mergedProxies)
	afterDedupeCount := len(mergedProxies)
	duplicateCount := beforeDedupeCount - afterDedupeCount

	beforeValidateCount := len(mergedProxies)
	mergedProxies = proxy.ValidateProxies(mergedProxies)
	afterValidateCount := len(mergedProxies)
	invalidCount := beforeValidateCount - afterValidateCount

	filteredProxies := filterProxies(mergedProxies, nameFilter, typeFilter, serverFilter)

	filteredProxies = proxy.RenameProxies(filteredProxies, nameFilter)

	filteredCount := len(filteredProxies)

	filteredConfig := make(map[string]interface{})
	for k, v := range firstConfig {
		filteredConfig[k] = v
	}
	filteredConfig["proxies"] = filteredProxies

	if firstConfig["proxy-groups"] != nil {
		if proxyGroups, ok := firstConfig["proxy-groups"].([]interface{}); ok {
			filteredConfig["proxy-groups"] = proxy.UpdateProxyGroups(proxyGroups, getProxyNames(filteredProxies))
		}
	}

	filterInfo := fmt.Sprintf("# Original nodes: %d, After deduplication: %d (removed %d duplicates), Invalid nodes: %d, Filtered nodes: %d\n", beforeDedupeCount, afterDedupeCount, duplicateCount, invalidCount, filteredCount)
	filterInfo += fmt.Sprintf("# Name filter: %s\n", nameFilter)
	filterInfo += fmt.Sprintf("# Type filter: %s\n", typeFilter)
	filterInfo += fmt.Sprintf("# Server filter: %s\n", serverFilter)
	filterInfo += fmt.Sprintf("# Generation time: %s\n", time.Now().Format(time.RFC3339))
	filterInfo += fmt.Sprintf("# Configuration sources:\n# %s\n", strings.Join(sourceURLInfo, "\n# "))

	yamlBytes, err := yaml.Marshal(filteredConfig)
	if err != nil {
		http.Error(w, "Error: Failed to generate YAML", http.StatusInternalServerError)
		return
	}

	// Combine the info header and the yaml content
	finalYAML := append([]byte(filterInfo), yamlBytes...)

	// Cache the final generated YAML for 10 minutes.
	cache.Set(filterCacheKey, finalYAML, time.Duration(appconfig.Get().FilterCacheTTLMinutes)*time.Minute)

	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(finalYAML)
}

func filterProxies(proxies []interface{}, nameFilter, typeFilter, serverFilter string) []interface{} {
	var filteredProxies []interface{}

	nameRegex, err := regexp.Compile(proxy.ExpandRegionNameFilter(nameFilter))
	if err != nil {
		slog.Error("invalid name regex", "error", err)
		nameRegex = regexp.MustCompile(".*")
	}

	typeRegex, err := regexp.Compile(typeFilter)
	if err != nil {
		slog.Error("invalid type regex", "error", err)
		typeRegex = regexp.MustCompile(".*")
	}

	serverRegex, err := regexp.Compile(serverFilter)
	if err != nil {
		slog.Error("invalid server regex", "error", err)
		serverRegex = regexp.MustCompile(".*")
	}

	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			nameMatch := nameRegex.MatchString(pMap["name"].(string))
			typeMatch := typeRegex.MatchString(pMap["type"].(string))

			serverMatch := false
			if serverFilter == "domain" {
				serverMatch = parser.IsDomainName(pMap["server"].(string))
			} else if serverFilter == "ip" {
				serverMatch = parser.IsIPAddress(pMap["server"].(string))
			} else {
				serverMatch = serverRegex.MatchString(pMap["server"].(string))
			}

			if nameMatch && typeMatch && serverMatch {
				filteredProxies = append(filteredProxies, p)
			}
		}
	}

	return filteredProxies
}

func getProxyNames(proxies []interface{}) []string {
	var names []string
	for _, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			names = append(names, pMap["name"].(string))
		}
	}
	return names
}
