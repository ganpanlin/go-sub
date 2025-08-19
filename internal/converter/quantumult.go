package converter

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	Register("quantumult", &QuantumultConverter{})
	Register("quanx", &QuantumultConverter{}) // alias
}

// QuantumultConverter outputs Quantumult X format.
type QuantumultConverter struct{}

func (c *QuantumultConverter) Name() string { return "quantumult" }

func (c *QuantumultConverter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	var serverLines, proxyLines []string

	for _, p := range proxies {
		line := toQuantumultProxy(p)
		if line != "" {
			proxyLines = append(proxyLines, line)
		}
	}

	// Build [SERVER] and [PROXY] sections
	serverLines = proxyLines // Quantumult X uses server_local lines

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("; Generated: %s\n; Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(proxies)))

	// [SERVER]
	sb.WriteString("[SERVER]\n")
	for _, line := range serverLines {
		sb.WriteString(line + "\n")
	}

	// [PROXY]
	sb.WriteString("\n[PROXY]\n")
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

			switch groupType {
			case "select":
				sb.WriteString(fmt.Sprintf("%s = select, %s, %s\n", groupName, "direct", strings.Join(proxies, ", ")))
			case "url-test":
				sb.WriteString(fmt.Sprintf("%s = url-test, %s, server_check_url=http://www.gstatic.com/generate_204\n", groupName, strings.Join(proxies, ", ")))
			case "load-balance":
				sb.WriteString(fmt.Sprintf("%s = round-robin, %s, server_check_url=http://www.gstatic.com/generate_204\n", groupName, strings.Join(proxies, ", ")))
			}
		}
	}

	// [RULE]
	sb.WriteString("\n[RULE]\n")
	for _, rule := range rules {
		qxRule := toQuantumultRule(rule)
		if qxRule != "" {
			sb.WriteString(qxRule + "\n")
		}
	}
	sb.WriteString("FINAL,DIRECT\n")

	return []byte(sb.String()), "text/plain; charset=utf-8", nil
}

func toQuantumultProxy(p map[string]interface{}) string {
	proxyType := strField(p, "type")
	server := strField(p, "server")
	port := intField(p, "port")
	name := strField(p, "name")

	switch proxyType {
	case "ss":
		cipher := strField(p, "cipher")
		password := strField(p, "password")
		return fmt.Sprintf("%s = shadowsocks, %s, %d, encrypt-method=%s, password=%s, obfs=off, obfs-host=", name, server, port, cipher, password)

	case "vmess":
		uuid := strField(p, "uuid")
		alterId := intField(p, "alterId")
		network := strField(p, "network")
		tls := boolField(p, "tls")

		header := fmt.Sprintf("%s = vmess, %s, %d, %s, aes-128-gcm, %s:%d, over-tls=%v", name, server, port, uuid, server, port, tls)

		if network == "ws" {
			wsPath := strField(p, "ws-path")
			wsHost := ""
			if headers := mapField(p, "ws-headers"); headers != nil {
				wsHost = fmt.Sprintf("%v", headers["Host"])
			}
			obfs := "off"
			if wsPath != "" || wsHost != "" {
				obfs = "ws"
			}
			header += fmt.Sprintf(", obfs=%s", obfs)
			if wsPath != "" {
				header += fmt.Sprintf(", obfs-path=%s", wsPath)
			}
			if wsHost != "" {
				header += fmt.Sprintf(", obfs-header=Host: %s", wsHost)
			}
		}

		_ = alterId
		return header

	case "trojan":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := fmt.Sprintf("%s = trojan, %s, %d, password=%s", name, server, port, password)
		if sni != "" {
			parts += fmt.Sprintf(", tls13=true, sni=%s", sni)
		}
		return parts

	case "vless":
		uuid := strField(p, "uuid")
		tls := boolField(p, "tls")
		sni := strField(p, "sni")
		parts := fmt.Sprintf("%s = vless, %s, %d, %s, over-tls=%v", name, server, port, uuid, tls)
		if sni != "" {
			parts += fmt.Sprintf(", sni=%s", sni)
		}
		return parts

	case "hysteria2":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := fmt.Sprintf("%s = hysteria2, %s, %d, password=%s", name, server, port, password)
		if sni != "" {
			parts += fmt.Sprintf(", sni=%s", sni)
		}
		return parts

	default:
		return ""
	}
}

func toQuantumultRule(rule string) string {
	// Skip RULE-SET (not supported in QX)
	if strings.HasPrefix(rule, "RULE-SET,") {
		return ""
	}
	if strings.HasPrefix(rule, "GEOSITE,") {
		return ""
	}
	if strings.HasPrefix(rule, "GEOIP,") {
		return ""
	}
	if strings.HasPrefix(rule, "MATCH,") {
		parts := strings.SplitN(rule, ",", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("FINAL,%s", parts[1])
		}
	}
	return rule
}
