package converter

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

func init() {
	Register("clash", &ClashConverter{})
}

// ClashConverter outputs standard Clash/Mihomo YAML.
type ClashConverter struct{}

func (c *ClashConverter) Name() string { return "clash" }

func (c *ClashConverter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	// Build output config
	out := make(map[string]interface{})
	for k, v := range extra {
		out[k] = v
	}

	// Convert proxies back to []interface{}
	proxyList := make([]interface{}, len(proxies))
	for i, p := range proxies {
		proxyList[i] = p
	}
	out["proxies"] = proxyList
	out["proxy-groups"] = groups
	if len(ruleProviders) > 0 {
		out["rule-providers"] = ruleProviders
	}
	out["rules"] = rules

	// Override mode / performance flags
	out["mode"] = "rule"
	out["unified-delay"] = true
	out["tcp-concurrent"] = true
	out["find-process-mode"] = "strict"
	out["global-client-fingerprint"] = "chrome"

	// Ensure DNS config
	if _, ok := out["dns"]; !ok {
		out["dns"] = map[string]interface{}{
			"enable": true, "ipv6": true, "enhanced-mode": "fake-ip",
			"nameserver":      []string{"223.5.5.5", "114.114.114.114", "https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"},
			"fallback":        []string{"https://1.0.0.1/dns-query", "https://dns.alidns.com/dns-query", "https://doh.pub/dns-query"},
			"fallback-filter": map[string]interface{}{"geoip": true, "geoip-code": "CN"},
		}
	}

	// Ensure geox-url
	if _, ok := out["geox-url"]; !ok {
		out["geox-url"] = map[string]interface{}{
			"geoip":   "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geoip-lite.dat",
			"geosite": "https://cdn.jsdelivr.net/gh/MetaCubeX/meta-rules-dat@release/geosite.dat",
		}
	}

	// Header
	header := fmt.Sprintf("# Generated: %s\n# Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(proxies))

	yamlBytes, err := MarshalClashYAML(out)
	if err != nil {
		return nil, "", err
	}

	return append([]byte(header), yamlBytes...), "text/yaml; charset=utf-8", nil
}

func MarshalClashYAML(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(4)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return unescapeYAMLUnicode([]byte(buf.String())), nil
}

func unescapeYAMLUnicode(data []byte) []byte {
	var out bytes.Buffer
	i := 0
	for i < len(data) {
		if i+10 <= len(data) && data[i] == '\\' && data[i+1] == 'U' {
			hex := string(data[i+2 : i+10])
			if code, err := strconv.ParseInt(hex, 16, 32); err == nil {
				out.WriteString(string(rune(code)))
				i += 10
				continue
			}
		}
		out.WriteByte(data[i])
		i++
	}
	return out.Bytes()
}
