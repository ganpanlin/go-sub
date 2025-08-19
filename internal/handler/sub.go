package handler

import (
	"encoding/json"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/converter"
	"go-sub/internal/pipeline"
	"go-sub/internal/rule"
	"net/http"
	"strings"
	"time"
)

// SubHandler handles /sub/{token} - generates filtered config using a saved profile.
// Supports ?type= parameter for client-specific output formats:
//   - type=clash (default)      - Clash/Mihomo YAML
//   - type=base64 / type=uri    - Base64 encoded URI list
//   - type=surge                - Surge config
//   - type=quantumult / type=quanx - Quantumult X config
//   - type=loon                 - Loon config
//   - type=singbox / type=sing-box - sing-box JSON
func SubHandler(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/sub/")
	token = strings.SplitN(token, "?", 2)[0]
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	profile := rule.GetManager().GetProfileByToken(token)
	if profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}
	if !profile.Enabled {
		http.Error(w, "Profile disabled", http.StatusForbidden)
		return
	}

	clientType := r.URL.Query().Get("type")
	if clientType == "" {
		clientType = "clash"
	}

	// Check cache (serve stale while refresh in background)
	cacheKey := fmt.Sprintf("sub:%s:%s", clientType, token)
	if cached, found := cache.Get(cacheKey); found {
		if data, ok := cached.([]byte); ok {
			w.Header().Set("X-Cache", "HIT")
			w.Write(data)
			return
		}
	}

	// Check stale cache - serve if exists, refresh async
	if staleCache, staleFound := cache.GetStale(cacheKey); staleFound {
		if data, ok := staleCache.([]byte); ok {
			// Serve stale immediately
			w.Header().Set("X-Cache", "STALE")
			w.Write(data)
			// Refresh in background
			go func() {
				pipeline.Run(profile)
			}()
			return
		}
	}

	// Pipeline: fetch → dedup → filter → routing
	result, err := pipeline.Run(profile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Convert to target format
	c := converter.Get(clientType)
	output, contentType, err := c.Convert(result.Proxies, result.Groups, result.RuleProviders, result.Rules, result.Extra)
	if err != nil {
		http.Error(w, fmt.Sprintf("Conversion error (%s): %s", clientType, err.Error()), http.StatusInternalServerError)
		return
	}

	// Cache
	cache.Set(cacheKey, output, time.Duration(appconfig.Get().FilterCacheTTLMinutes)*time.Minute)

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Profile-Name", profile.Name)
	w.Header().Set("Node-Count", fmt.Sprintf("%d", len(result.Proxies)))
	w.Write(output)
}

// SimulateHandler generates the subscription output for a profile and validates it.
func SimulateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID    string `json:"id"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var profile *rule.Profile
	if req.ID != "" {
		profile = rule.GetManager().GetProfile(req.ID)
	} else if req.Token != "" {
		profile = rule.GetManager().GetProfileByToken(req.Token)
	}
	if profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	// Pipeline
	result, err := pipeline.Run(profile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Validate
	warnings := []string{}
	errors := []string{}

	if len(result.Proxies) == 0 {
		errors = append(errors, "过滤后无可用节点")
	}

	for i, p := range result.Proxies {
		name := fmt.Sprintf("%v", p["name"])
		if p["server"] == nil || fmt.Sprintf("%v", p["server"]) == "" {
			errors = append(errors, fmt.Sprintf("节点[%d] %q 缺少 server", i, name))
		}
		if p["port"] == nil {
			errors = append(errors, fmt.Sprintf("节点[%d] %q 缺少 port", i, name))
		}
		if p["type"] == nil || fmt.Sprintf("%v", p["type"]) == "" {
			errors = append(errors, fmt.Sprintf("节点[%d] %q 缺少 type", i, name))
		}
	}

	groupNames := map[string]bool{}
	for _, g := range result.Groups {
		if gm, ok := g.(map[string]interface{}); ok {
			name := fmt.Sprintf("%v", gm["name"])
			if groupNames[name] {
				warnings = append(warnings, fmt.Sprintf("proxy-group 名称重复: %s", name))
			}
			groupNames[name] = true
		}
	}

	for _, r := range result.Rules {
		parts := strings.Split(fmt.Sprintf("%v", r), ",")
		if len(parts) >= 2 {
			target := parts[len(parts)-1]
			if target != "DIRECT" && target != "REJECT" && target != "PASS" && !groupNames[target] {
				// skip no-resolve etc.
			}
		}
	}

	if len(result.Rules) == 0 {
		warnings = append(warnings, "未生成任何分流规则")
	}

	hasMatch := false
	for _, r := range result.Rules {
		if strings.HasPrefix(fmt.Sprintf("%v", r), "MATCH") {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		warnings = append(warnings, "缺少 MATCH 兜底规则")
	}

	// YAML preview
	out := buildOutputConfig(result.Extra, toInterfaceProxies(result.Proxies), result.Groups, result.RuleProviders, result.Rules)
	yamlPreview, _ := marshalYAML(out)

	yamlStr := ""
	if len(yamlPreview) > 0 {
		lines := strings.Split(string(yamlPreview), "\n")
		if len(lines) > 2000 {
			yamlStr = strings.Join(lines[:2000], "\n") + "\n... (truncated)"
		} else {
			yamlStr = string(yamlPreview)
		}
	}

	resp := map[string]interface{}{
		"ok":      len(errors) == 0,
		"profile": profile.Name,
		"sources": result.Sources,
		"summary": map[string]interface{}{
			"total_nodes":    result.TotalBefore,
			"filtered_nodes": result.TotalAfter,
			"proxy_groups":   len(result.Groups),
			"rule_providers": len(result.RuleProviders),
			"rules":          len(result.Rules),
		},
		"proxy_groups":   groupNamesList(result.Groups),
		"rule_providers": ruleProviderNames(result.RuleProviders),
		"rules_sample":   result.Rules,
		"warnings":       warnings,
		"errors":         errors,
		"yaml_preview":   yamlStr,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// GeneratePreviewHandler generates and returns the full YAML for preview.
func GeneratePreviewHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var profile *rule.Profile
	if id != "" {
		profile = rule.GetManager().GetProfile(id)
	}
	if profile == nil {
		http.Error(w, "Profile not found", http.StatusNotFound)
		return
	}

	cache.DeleteByPrefix("sub:")

	result, err := pipeline.Run(profile)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := converter.Get("clash")
	output, _, err := c.Convert(result.Proxies, result.Groups, result.RuleProviders, result.Rules, result.Extra)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.Write(output)
}

// --- helpers ---

func toInterfaceProxies(maps []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(maps))
	for i, m := range maps {
		out[i] = m
	}
	return out
}

func groupNamesList(groups []interface{}) []string {
	var names []string
	for _, g := range groups {
		if gm, ok := g.(map[string]interface{}); ok {
			names = append(names, fmt.Sprintf("%v", gm["name"]))
		}
	}
	return names
}

func ruleProviderNames(rp map[string]interface{}) []string {
	var names []string
	for k := range rp {
		names = append(names, k)
	}
	return names
}

func buildOutputConfig(base map[string]interface{}, proxies []interface{}, groups []interface{}, rp map[string]interface{}, rules []string) map[string]interface{} {
	out := make(map[string]interface{})
	if base != nil {
		for k, v := range base {
			out[k] = v
		}
	}
	out["proxies"] = proxies
	out["proxy-groups"] = groups
	if len(rp) > 0 {
		out["rule-providers"] = rp
	}
	out["rules"] = rules
	out["mode"] = "rule"
	return out
}
