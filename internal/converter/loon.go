package converter

import (
	"fmt"
	"strings"
	"time"
)

func init() {
	Register("loon", &LoonConverter{})
}

// LoonConverter outputs Loon proxy format.
type LoonConverter struct{}

func (c *LoonConverter) Name() string { return "loon" }

func (c *LoonConverter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Generated: %s\n# Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(proxies)))

	// [General]
	sb.WriteString("[General]\n")
	sb.WriteString("ipv6 = true\n")
	sb.WriteString("dns-server = 223.5.5.5, 114.114.114.114\n")
	sb.WriteString("skip-proxy = 127.0.0.1, 192.168.0.0/16, 10.0.0.0/8, 172.16.0.0/12, localhost, *.local\n\n")

	// [Proxy]
	sb.WriteString("[Proxy]\n")
	sb.WriteString("DIRECT = direct\n\n")

	for _, p := range proxies {
		line := toLoonProxy(p)
		if line != "" {
			sb.WriteString(line + "\n")
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

			switch groupType {
			case "select":
				sb.WriteString(fmt.Sprintf("%s = select, %s\n", groupName, strings.Join(proxies, ", ")))
			case "url-test":
				url := "http://www.gstatic.com/generate_204"
				if u, ok := gm["url"].(string); ok && u != "" {
					url = u
				}
				sb.WriteString(fmt.Sprintf("%s = url-test, %s, url=%s, interval=300\n", groupName, strings.Join(proxies, ", "), url))
			case "load-balance":
				sb.WriteString(fmt.Sprintf("%s = load-balance, %s, url=http://www.gstatic.com/generate_204\n", groupName, strings.Join(proxies, ", ")))
			}
		}
	}

	// [Rule]
	sb.WriteString("\n[Rule]\n")
	for _, rule := range rules {
		loonRule := toLoonRule(rule)
		if loonRule != "" {
			sb.WriteString(loonRule + "\n")
		}
	}
	sb.WriteString("FINAL,DIRECT\n")

	return []byte(sb.String()), "text/plain; charset=utf-8", nil
}

func toLoonProxy(p map[string]interface{}) string {
	proxyType := strField(p, "type")
	server := strField(p, "server")
	port := intField(p, "port")
	name := strField(p, "name")

	switch proxyType {
	case "ss":
		cipher := strField(p, "cipher")
		password := strField(p, "password")
		return fmt.Sprintf("%s = Shadowsocks, %s, %d, %s, %s", name, server, port, cipher, password)

	case "vmess":
		uuid := strField(p, "uuid")
		alterId := intField(p, "alterId")
		network := strField(p, "network")
		tls := boolField(p, "tls")
		sni := strField(p, "sni")

		parts := fmt.Sprintf("%s = vmess, %s, %d, %s", name, server, port, uuid)

		if alterId > 0 {
			parts += fmt.Sprintf(", alterId=%d", alterId)
		}
		if network == "ws" {
			wsPath := strField(p, "ws-path")
			wsHost := ""
			if headers := mapField(p, "ws-headers"); headers != nil {
				wsHost = fmt.Sprintf("%v", headers["Host"])
			}
			parts += ", transport=ws"
			if wsPath != "" {
				parts += fmt.Sprintf(", path=%s", wsPath)
			}
			if wsHost != "" {
				parts += fmt.Sprintf(", host=%s", wsHost)
			}
		}
		if tls {
			parts += ", over-tls=true, tls=true"
			if sni != "" {
				parts += fmt.Sprintf(", sni=%s", sni)
			}
		}
		return parts

	case "trojan":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := fmt.Sprintf("%s = trojan, %s, %d, %s", name, server, port, password)
		if sni != "" {
			parts += fmt.Sprintf(", tls=true, sni=%s", sni)
		}
		return parts

	case "vless":
		uuid := strField(p, "uuid")
		tls := boolField(p, "tls")
		sni := strField(p, "sni")
		network := strField(p, "network")
		parts := fmt.Sprintf("%s = vless, %s, %d, %s", name, server, port, uuid)
		if network == "ws" {
			wsPath := strField(p, "ws-path")
			wsHost := ""
			if headers := mapField(p, "ws-headers"); headers != nil {
				wsHost = fmt.Sprintf("%v", headers["Host"])
			}
			parts += ", transport=ws"
			if wsPath != "" {
				parts += fmt.Sprintf(", path=%s", wsPath)
			}
			if wsHost != "" {
				parts += fmt.Sprintf(", host=%s", wsHost)
			}
		}
		if tls {
			parts += ", over-tls=true, tls=true"
			if sni != "" {
				parts += fmt.Sprintf(", sni=%s", sni)
			}
		}
		return parts

	case "hysteria2":
		password := strField(p, "password")
		sni := strField(p, "sni")
		parts := fmt.Sprintf("%s = hysteria2, %s, %d, %s", name, server, port, password)
		if sni != "" {
			parts += fmt.Sprintf(", sni=%s", sni)
		}
		return parts

	case "ssr":
		password := strField(p, "password")
		cipher := strField(p, "cipher")
		protocol := strField(p, "protocol")
		obfs := strField(p, "obfs")
		return fmt.Sprintf("%s = ShadowsocksR, %s, %d, %s, %s, %s, %s", name, server, port, password, cipher, protocol, obfs)

	default:
		return ""
	}
}

func toLoonRule(rule string) string {
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
