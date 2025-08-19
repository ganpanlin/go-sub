package converter

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	Register("surge", &SurgeConverter{})
}

// SurgeConverter outputs Surge proxy list format.
type SurgeConverter struct{}

func (c *SurgeConverter) Name() string { return "surge" }

func (c *SurgeConverter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("#!MANAGED-CONFIG\n"))
	sb.WriteString(fmt.Sprintf("# Generated: %s\n# Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(proxies)))

	// [General]
	sb.WriteString("[General]\n")
	sb.WriteString("loglevel = notify\n")
	sb.WriteString("skip-proxy = 127.0.0.1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12, 100.64.0.0/10, localhost, *.local\n")
	sb.WriteString("dns-server = 223.5.5.5, 114.114.114.114\n\n")

	// [Replica]
	sb.WriteString("[Replica]\n")
	sb.WriteString("hide-error-tls-ticket = true\n\n")

	// [Proxy]
	sb.WriteString("[Proxy]\n")
	sb.WriteString("DIRECT = direct\n\n")

	var proxyNames []string
	for _, p := range proxies {
		line := toSurgeProxy(p)
		if line != "" {
			name := strField(p, "name")
			sb.WriteString(fmt.Sprintf("%s = %s\n", surgeEscapeName(name), line))
			proxyNames = append(proxyNames, name)
		}
	}

	// [Proxy Group]
	sb.WriteString("\n[Proxy Group]\n")
	for _, g := range groups {
		if gm, ok := g.(map[string]interface{}); ok {
			groupName := fmt.Sprintf("%v", gm["name"])
			groupType := fmt.Sprintf("%v", gm["type"])
			proxies, _ := gm["proxies"].([]string)
			if proxies == nil {
				if pi, ok := gm["proxies"].([]interface{}); ok {
					for _, pp := range pi {
						proxies = append(proxies, fmt.Sprintf("%v", pp))
					}
				}
			}

			surgeType := "select"
			switch groupType {
			case "url-test":
				surgeType = "url-test"
			case "load-balance":
				surgeType = "url-test" // Surge doesn't have exact load-balance, fallback to url-test
			case "fallback":
				surgeType = "fallback"
			}

			sb.WriteString(fmt.Sprintf("%s = %s", surgeEscapeName(groupName), surgeType))
			for _, pp := range proxies {
				sb.WriteString(fmt.Sprintf(", %s", surgeEscapeName(pp)))
			}
			if url, ok := gm["url"].(string); ok && url != "" {
				sb.WriteString(fmt.Sprintf(", url=%s, interval=300", url))
			}
			sb.WriteString("\n")
		}
	}

	// [Rule]
	sb.WriteString("\n[Rule]\n")
	for _, rule := range rules {
		surgeRule := toSurgeRule(rule)
		if surgeRule != "" {
			sb.WriteString(surgeRule + "\n")
		}
	}
	sb.WriteString("FINAL,DIRECT\n")

	return []byte(sb.String()), "text/plain; charset=utf-8", nil
}

func toSurgeProxy(p map[string]interface{}) string {
	proxyType := strField(p, "type")
	server := strField(p, "server")
	port := intField(p, "port")

	switch proxyType {
	case "ss":
		cipher := strField(p, "cipher")
		password := strField(p, "password")
		return fmt.Sprintf("ss, %s, %d, encrypt-method=%s, password=%s", server, port, cipher, password)

	case "vmess":
		uuid := strField(p, "uuid")
		alterId := intField(p, "alterId")
		network := strField(p, "network")
		tls := boolField(p, "tls")
		sni := strField(p, "sni")

		parts := []string{fmt.Sprintf("vmess, %s, %d, username=%s", server, port, uuid)}
		if alterId > 0 {
			parts = append(parts, fmt.Sprintf("vmess-aead=false"))
		}
		if network == "ws" {
			wsPath := strField(p, "ws-path")
			wsHost := ""
			if headers := mapField(p, "ws-headers"); headers != nil {
				wsHost = fmt.Sprintf("%v", headers["Host"])
			}
			parts = append(parts, "ws=true")
			if wsPath != "" {
				parts = append(parts, fmt.Sprintf("ws-path=%s", wsPath))
			}
			if wsHost != "" {
				parts = append(parts, fmt.Sprintf("ws-headers=host:%s", wsHost))
			}
		}
		if tls {
			parts = append(parts, "tls=true")
			if sni != "" {
				parts = append(parts, fmt.Sprintf("sni=%s", sni))
			}
			if skipCert := boolField(p, "skip-cert-verify"); skipCert {
				parts = append(parts, "skip-cert-verify=true")
			}
		}
		return strings.Join(parts, ", ")

	case "trojan":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := []string{fmt.Sprintf("trojan, %s, %d, password=%s", server, port, password)}
		if sni != "" {
			parts = append(parts, fmt.Sprintf("sni=%s", sni))
		}
		if boolField(p, "skip-cert-verify") {
			parts = append(parts, "skip-cert-verify=true")
		}
		return strings.Join(parts, ", ")

	case "vless":
		// Surge 5+ supports VLESS
		uuid := strField(p, "uuid")
		network := strField(p, "network")
		tls := boolField(p, "tls")
		sni := strField(p, "sni")

		parts := []string{fmt.Sprintf("vless, %s, %d, uuid=%s", server, port, uuid)}
		if network == "ws" {
			wsPath := strField(p, "ws-path")
			wsHost := ""
			if headers := mapField(p, "ws-headers"); headers != nil {
				wsHost = fmt.Sprintf("%v", headers["Host"])
			}
			parts = append(parts, "transport=ws")
			if wsPath != "" {
				parts = append(parts, fmt.Sprintf("path=%s", wsPath))
			}
			if wsHost != "" {
				parts = append(parts, fmt.Sprintf("host=%s", wsHost))
			}
		} else if network == "grpc" {
			sn := strField(p, "grpc-service-name")
			parts = append(parts, "transport=grpc")
			if sn != "" {
				parts = append(parts, fmt.Sprintf("service-name=%s", sn))
			}
		}
		if tls {
			parts = append(parts, "tls=true")
			if sni != "" {
				parts = append(parts, fmt.Sprintf("sni=%s", sni))
			}
		}
		if flow := strField(p, "flow"); flow != "" {
			parts = append(parts, fmt.Sprintf("flow=%s", flow))
		}
		if realityOpts := mapField(p, "reality-opts"); realityOpts != nil {
			parts = append(parts, "tls=true")
			if pbk := fmt.Sprintf("%v", realityOpts["public-key"]); pbk != "" {
				parts = append(parts, fmt.Sprintf("public-key=%s", pbk))
			}
			if sid := fmt.Sprintf("%v", realityOpts["short-id"]); sid != "" {
				parts = append(parts, fmt.Sprintf("short-id=%s", sid))
			}
		}
		return strings.Join(parts, ", ")

	case "hysteria2":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := []string{fmt.Sprintf("hysteria2, %s, %d, password=%s", server, port, password)}
		if sni != "" {
			parts = append(parts, fmt.Sprintf("sni=%s", sni))
		}
		if boolField(p, "skip-cert-verify") {
			parts = append(parts, "skip-cert-verify=true")
		}
		return strings.Join(parts, ", ")

	default:
		return ""
	}
}

func toSurgeRule(rule string) string {
	// Convert Clash rule format to Surge rule format
	// RULE-SET is not directly supported in Surge the same way, convert to DOMAIN-SUFFIX etc.
	// For simplicity, keep compatible rules and skip unsupported ones
	if strings.HasPrefix(rule, "RULE-SET,") {
		// Surge uses RULE-SET differently, skip for now
		return ""
	}
	if strings.HasPrefix(rule, "GEOSITE,") {
		// Surge uses DOMAIN-SET or RULE-SET with geosite
		parts := strings.SplitN(rule, ",", 2)
		if len(parts) == 2 {
			return "" // Skip GEOSITE for Surge compatibility
		}
	}
	if strings.HasPrefix(rule, "GEOIP,") {
		parts := strings.SplitN(rule, ",", 3)
		if len(parts) >= 2 {
			return fmt.Sprintf("GEOIP,%s", parts[1])
		}
	}
	if strings.HasPrefix(rule, "MATCH,") {
		parts := strings.SplitN(rule, ",", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("FINAL,%s", parts[1])
		}
	}
	// IP-CIDR, DOMAIN-SUFFIX etc. are mostly compatible
	return rule
}

func surgeEscapeName(name string) string {
	// Surge proxy names can't have certain chars
	name = strings.ReplaceAll(name, ",", " ")
	return name
}
