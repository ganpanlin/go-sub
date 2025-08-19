package pipeline

import (
	"fmt"
	"go-sub/internal/provider"
	"go-sub/internal/routing"
	"go-sub/internal/rule"
	"go-sub/internal/source"
	"strings"
	"sync"
)

// SourceInfo records the result of fetching a single subscription source.
type SourceInfo struct {
	URL       string `json:"url"`
	Name      string `json:"name"`
	NodeCount int    `json:"node_count"`
	Error     string `json:"error,omitempty"`
}

// Result holds the fully processed subscription data ready for output.
type Result struct {
	Proxies       []map[string]interface{}
	Groups        []interface{}
	RuleProviders map[string]interface{}
	Rules         []string
	Extra         map[string]interface{} // base config from first source (dns, geox-url, etc.)
	Sources       []SourceInfo
	TotalBefore   int // nodes after dedup, before filter
	TotalAfter    int // nodes after filter
}

// Run fetches all sources for a profile, deduplicates, filters, and builds routing config.
// This is the shared data preparation pipeline used by /sub, /simulate, /preview.
func Run(profile *rule.Profile) (*Result, error) {
	// 1. Resolve source URLs
	sourceURLs := source.EnabledRuntimeURLs(profile.Sources)
	sourceNames := source.NameMap()

	var mergedProxies []interface{}
	var firstConfig map[string]interface{}
	var sources []SourceInfo

	// Track source per proxy for prefix tagging
	type proxyWithSource struct {
		proxy     interface{}
		sourceURL string
	}
	var taggedProxies []proxyWithSource

	// 2. Fetch and parse each source (parallel, max 8 concurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, u := range sourceURLs {
		url := u
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			config, status, latency, isCached, err := provider.FetchAndParseYAML(url)
			_ = status
			_ = latency
			_ = isCached

			si := SourceInfo{
				URL:       displayURL(url),
				Name:      sourceNames[url],
				NodeCount: provider.ProxyCount(config),
			}
			if err != nil {
				si.Error = err.Error()
			}

			mu.Lock()
			sources = append(sources, si)
			if err == nil {
				if firstConfig == nil && config != nil {
					firstConfig = config
				}
				if config != nil && config["proxies"] != nil {
					if proxies, ok := config["proxies"].([]interface{}); ok {
						for _, p := range proxies {
							taggedProxies = append(taggedProxies, proxyWithSource{proxy: p, sourceURL: url})
						}
						mergedProxies = append(mergedProxies, proxies...)
					}
				}
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	if firstConfig == nil {
		return nil, fmt.Errorf("no valid configuration found")
	}
	if len(mergedProxies) == 0 {
		return nil, fmt.Errorf("no proxies found")
	}

	// 3. Deduplicate + validate
	mergedProxies = dedup(mergedProxies)
	mergedProxies = validate(mergedProxies)
	totalBefore := len(mergedProxies)

	// 4. Apply profile rule engine (filter, rename, sort, etc.)
	engine, err := rule.NewEngine(profile)
	if err != nil {
		return nil, fmt.Errorf("rule engine error: %w", err)
	}
	filteredProxies := engine.Process(mergedProxies)

	// 5. Apply source prefix
	if profile.SourcePrefix != "" && profile.SourcePrefix != "off" {
		proxySourceMap := map[string]string{}
		for _, tp := range taggedProxies {
			if pm, ok := tp.proxy.(map[string]interface{}); ok {
				key := fmt.Sprintf("%v:%v:%v", pm["type"], pm["server"], pm["port"])
				proxySourceMap[key] = tp.sourceURL
			}
		}
		for _, p := range filteredProxies {
			if pm, ok := p.(map[string]interface{}); ok {
				key := fmt.Sprintf("%v:%v:%v", pm["type"], pm["server"], pm["port"])
				if srcURL, found := proxySourceMap[key]; found {
					prefix := getSourcePrefix(srcURL, sourceNames[srcURL], profile.SourcePrefix)
					name := fmt.Sprintf("%v", pm["name"])
					pm["name"] = prefix + name
				}
			}
		}
	}

	// 6. Convert to []map[string]interface{}
	proxyMaps := make([]map[string]interface{}, len(filteredProxies))
	for i, p := range filteredProxies {
		if pm, ok := p.(map[string]interface{}); ok {
			proxyMaps[i] = pm
		}
	}

	// 7. Build routing config (proxy-groups, rule-providers, rules)
	rp := routing.GetManager().GetEffective(profile.RoutingID)
	proxyNames := getNames(filteredProxies)
	groups, ruleProviders, rules := routing.BuildConfig(rp, proxyNames)

	// 8. Extra config (base config from first source)
	extra := make(map[string]interface{})
	if firstConfig != nil {
		for k, v := range firstConfig {
			extra[k] = v
		}
	}

	return &Result{
		Proxies:       proxyMaps,
		Groups:        groups,
		RuleProviders: ruleProviders,
		Rules:         rules,
		Extra:         extra,
		Sources:       sources,
		TotalBefore:   totalBefore,
		TotalAfter:    len(proxyMaps),
	}, nil
}

// --- internal helpers ---

func displayURL(url string) string {
	if strings.HasPrefix(url, "data:") {
		return "local://subscription"
	}
	return url
}

func getSourcePrefix(url, name, mode string) string {
	switch mode {
	case "name":
		if name != "" && name != url {
			return name + "-"
		}
		return extractDomain(url) + "-"
	case "domain":
		return extractDomain(url) + "-"
	default:
		return ""
	}
}

func extractDomain(rawURL string) string {
	u := rawURL
	if strings.HasPrefix(u, "https://") {
		u = u[8:]
	} else if strings.HasPrefix(u, "http://") {
		u = u[7:]
	}
	for i := 0; i < len(u); i++ {
		if u[i] == '/' {
			u = u[:i]
			break
		}
	}
	for i := 0; i < len(u); i++ {
		if u[i] == ':' {
			u = u[:i]
			break
		}
	}
	return u
}

func getNames(proxies []interface{}) []string {
	names := make([]string, 0, len(proxies))
	for _, p := range proxies {
		if m, ok := p.(map[string]interface{}); ok {
			names = append(names, fmt.Sprintf("%v", m["name"]))
		}
	}
	return names
}

func dedup(proxies []interface{}) []interface{} {
	seen := make(map[string]bool)
	var result []interface{}
	for _, p := range proxies {
		if m, ok := p.(map[string]interface{}); ok {
			key := fmt.Sprintf("%v:%v:%v", m["type"], m["server"], m["port"])
			if !seen[key] {
				seen[key] = true
				result = append(result, p)
			}
		}
	}
	return result
}

func validate(proxies []interface{}) []interface{} {
	var result []interface{}
	for _, p := range proxies {
		if m, ok := p.(map[string]interface{}); ok {
			if m["name"] != nil && m["server"] != nil && m["type"] != nil && m["port"] != nil {
				result = append(result, p)
			}
		}
	}
	return result
}
