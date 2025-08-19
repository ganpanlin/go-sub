package routing

import (
	"fmt"
	"strings"
)

// BuildConfig takes a RoutingProfile and a list of proxy names,
// returns generated proxy-groups, rule-providers, and rules ready to merge into the output config.
func BuildConfig(rp *RoutingProfile, proxyNames []string) (proxyGroups []interface{}, ruleProviders map[string]interface{}, rules []string) {
	if rp == nil {
		rp = DefaultRouting()
	}

	ruleProviders = make(map[string]interface{})
	var generatedRules []string

	var gfwGroups []interface{}
	var nonGfwGroups []interface{}

	for _, rule := range rp.Rules {
		group := buildProxyGroup(rule, proxyNames)
		if rule.GFW {
			gfwGroups = append(gfwGroups, group)
		} else {
			nonGfwGroups = append(nonGfwGroups, group)
		}

		if len(rule.Payload) > 0 {
			for _, p := range rule.Payload {
				generatedRules = append(generatedRules, createPayloadRules(p, rule.Name)...)
			}
		} else if len(rule.Urls) > 0 {
			for i, rawURL := range rule.Urls {
				providerName := fmt.Sprintf("%s-rule", rule.Name)
				if i > 0 {
					providerName = fmt.Sprintf("%s-rule-%d", rule.Name, i)
				}
				ruleProviders[sanitizeProviderName(providerName)] = buildRuleProvider(rawURL)
				generatedRules = append(generatedRules, fmt.Sprintf("RULE-SET,%s,%s", sanitizeProviderName(providerName), rule.Name))
			}
		}
	}

	// Assemble final proxy-groups in order
	proxyGroups = []interface{}{}

	// 🇨🇳 国内直连
	proxyGroups = append(proxyGroups, map[string]interface{}{
		"name": "国内网站", "type": "select",
		"proxies":     append([]string{"DIRECT", "自动选择(最低延迟)", "负载均衡"}, allProxyNamesWithAuto(proxyNames)...),
		"include-all": true,
		"url":         "https://www.baidu.com/favicon.ico",
	})

	// Non-GFW groups
	proxyGroups = append(proxyGroups, nonGfwGroups...)

	// 🌍 国外网站
	proxyGroups = append(proxyGroups, map[string]interface{}{
		"name": "国外网站", "type": "select",
		"proxies":     append([]string{"DIRECT", "自动选择(最低延迟)", "负载均衡"}, allProxyNamesWithAuto(proxyNames)...),
		"include-all": true,
		"url":         "https://www.bing.com/favicon.ico",
	})

	// GFW groups
	proxyGroups = append(proxyGroups, gfwGroups...)

	// 🔒 被墙网站
	proxyGroups = append(proxyGroups, map[string]interface{}{
		"name": "被墙网站", "type": "select",
		"proxies":     append([]string{"自动选择(最低延迟)", "负载均衡", "DIRECT"}, allProxyNamesWithAuto(proxyNames)...),
		"include-all": true,
	})

	// Utility groups
	proxyGroups = append(proxyGroups, map[string]interface{}{
		"name": "自动选择(最低延迟)", "type": "url-test",
		"include-all": true, "tolerance": 20,
		"url": "https://play-lh.googleusercontent.com/1UF2WCBNl4918bNk8JsILadL9-agIjRtMpdjuPgx2ohsxnQyspdWDwYMquW1-r8mSQOSjSLOY4g=w720-rw",
	})
	proxyGroups = append(proxyGroups, map[string]interface{}{
		"name": "负载均衡", "type": "load-balance",
		"include-all": true, "hidden": true, "strategy": "sticky-sessions",
		"url": "https://play-lh.googleusercontent.com/1UF2WCBNl4918bNk8JsILadL9-agIjRtMpdjuPgx2ohsxnQyspdWDwYMquW1-r8mSQOSjSLOY4g=w720-rw",
	})

	// Final catch-all rules
	rules = []string{
		"IP-CIDR,127.0.0.0/8,DIRECT,no-resolve",
		"IP-CIDR6,::1/128,DIRECT,no-resolve",
	}
	rules = append(rules, generatedRules...)
	rules = append(rules, "GEOSITE,gfw,被墙网站")
	rules = append(rules, "GEOIP,CN,国内网站")
	rules = append(rules, "MATCH,国外网站")

	return
}

func buildProxyGroup(rule RuleItem, _ []string) map[string]interface{} {
	extra := rule.ExtraProxies
	if extra == nil {
		extra = []string{}
	}
	if rule.GFW {
		return map[string]interface{}{
			"name": rule.Name, "type": "select",
			"proxies":     append(append([]string{}, extra...), "自动选择(最低延迟)", "负载均衡", "DIRECT"),
			"include-all": true,
		}
	}
	return map[string]interface{}{
		"name": rule.Name, "type": "select",
		"proxies":     append(append([]string{}, extra...), "DIRECT", "自动选择(最低延迟)", "负载均衡"),
		"include-all": true,
	}
}

func buildRuleProvider(url string) map[string]interface{} {
	return map[string]interface{}{
		"type":     "http",
		"interval": 86400,
		"behavior": "classical",
		"format":   "yaml",
		"url":      url,
	}
}

func allProxyNamesWithAuto(names []string) []string {
	return nil
}

func sanitizeProviderName(name string) string {
	return strings.ReplaceAll(name, ",", "-")
}

func createPayloadRules(raw, name string) []string {
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	pushIndex := len(parts)
	if len(parts) > 0 && strings.EqualFold(parts[len(parts)-1], "no-resolve") {
		pushIndex--
	}
	parts = append(parts[:pushIndex], append([]string{sanitizeProviderName(name)}, parts[pushIndex:]...)...)
	return []string{strings.Join(parts, ",")}
}
