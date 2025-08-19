package converter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func init() {
	Register("singbox", &SingboxConverter{})
	Register("sing-box", &SingboxConverter{}) // alias
}

// SingboxConverter outputs sing-box JSON format.
type SingboxConverter struct{}

func (c *SingboxConverter) Name() string { return "singbox" }

func (c *SingboxConverter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	// Build sing-box config
	config := map[string]interface{}{
		"log": map[string]interface{}{
			"level":  "info",
			"timestamp": true,
		},
		"dns": map[string]interface{}{
			"servers": []interface{}{
				map[string]interface{}{"tag": "dns-ali", "address": "223.5.5.5"},
				map[string]interface{}{"tag": "dns-114", "address": "114.114.114.114"},
			},
		},
	}

	// Build outbounds
	var outbounds []interface{}

	// DIRECT outbound
	outbounds = append(outbounds, map[string]interface{}{
		"type": "direct", "tag": "DIRECT",
	})

	// REJECT outbound
	outbounds = append(outbounds, map[string]interface{}{
		"type": "block", "tag": "REJECT",
	})

	// dns outbound
	outbounds = append(outbounds, map[string]interface{}{
		"type": "dns", "tag": "dns-out",
	})

	// Proxy outbounds
	var proxyTags []string
	for _, p := range proxies {
		outbound := toSingboxOutbound(p)
		if outbound != nil {
			outbounds = append(outbounds, outbound)
			proxyTags = append(proxyTags, strField(p, "name"))
		}
	}

	// Build group outbounds
	for _, g := range groups {
		if gm, ok := g.(map[string]interface{}); ok {
			groupOut := toSingboxGroup(gm, proxyTags)
			if groupOut != nil {
				outbounds = append(outbounds, groupOut)
			}
		}
	}

	config["outbounds"] = outbounds

	// Build route rules
	var routeRules []interface{}
	for _, rule := range rules {
		singRule := toSingboxRule(rule)
		if singRule != nil {
			routeRules = append(routeRules, singRule)
		}
	}
	// Final rule
	routeRules = append(routeRules, map[string]interface{}{
		"outbound": "国外网站",
	})

	// Collect all rule-set tags for route
	route := map[string]interface{}{
		"rules": routeRules,
		"final": "国外网站",
	}

	// Auto detect geoip/geosite
	hasGeoIP := false
	hasGeoSite := false
	for _, rule := range rules {
		if strings.HasPrefix(rule, "GEOIP,") {
			hasGeoIP = true
		}
		if strings.HasPrefix(rule, "GEOSITE,") {
			hasGeoSite = true
		}
	}
	geo := map[string]interface{}{}
	if hasGeoIP {
		geo["geoip"] = "geoip.db"
	}
	if hasGeoSite {
		geo["geosite"] = "geosite.db"
	}
	if len(geo) > 0 {
		route["geoip"] = geo["geoip"]
		route["geosite"] = geo["geosite"]
	}

	config["route"] = route

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, "", err
	}

	header := fmt.Sprintf("// Generated: %s\n// Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(proxies))
	return append([]byte(header), data...), "application/json; charset=utf-8", nil
}

func toSingboxOutbound(p map[string]interface{}) map[string]interface{} {
	proxyType := strField(p, "type")
	name := strField(p, "name")
	server := strField(p, "server")
	port := intField(p, "port")

	base := map[string]interface{}{
		"type":     proxyType,
		"tag":      name,
		"server":   server,
		"port":     port,
	}

	switch proxyType {
	case "shadowsocks":
		base["method"] = strField(p, "cipher")
		base["password"] = strField(p, "password")

	case "vmess":
		base["uuid"] = strField(p, "uuid")
		alterId := intField(p, "alterId")
		if alterId > 0 {
			base["alter_id"] = alterId
		}
		base["security"] = "auto"
		addSingboxTransport(base, p)
		addSingboxTLS(base, p)

	case "vless":
		base["uuid"] = strField(p, "uuid")
		if flow := strField(p, "flow"); flow != "" {
			base["flow"] = flow
		}
		addSingboxTransport(base, p)
		addSingboxTLS(base, p)
		// Reality
		if realityOpts := mapField(p, "reality-opts"); realityOpts != nil {
			tls := map[string]interface{}{
				"enabled":    true,
				"server_name": strField(p, "servername"),
				"reality": map[string]interface{}{
					"enabled":   true,
					"public_key": fmt.Sprintf("%v", realityOpts["public-key"]),
					"short_id":   fmt.Sprintf("%v", realityOpts["short-id"]),
				},
			}
			if fp := strField(p, "client-fingerprint"); fp != "" {
				tls["utls"] = map[string]interface{}{
					"enabled": true,
					"fingerprint": fp,
				}
			}
			base["tls"] = tls
		}

	case "trojan":
		base["password"] = strField(p, "password")
		addSingboxTLS(base, p)
		addSingboxTransport(base, p)

	case "hysteria2":
		base["password"] = strField(p, "password")
		if sni := strField(p, "sni"); sni != "" {
			base["tls"] = map[string]interface{}{
				"enabled":    true,
				"server_name": sni,
				"insecure":    boolField(p, "skip-cert-verify"),
			}
		}
		if up := intField(p, "up"); up > 0 {
			base["up_mbps"] = up
		}
		if down := intField(p, "down"); down > 0 {
			base["down_mbps"] = down
		}
		if obfs := strField(p, "obfs"); obfs != "" {
			base["obfs"] = map[string]interface{}{
				"type":     obfs,
				"password": strField(p, "obfs-password"),
			}
		}

	case "shadowsocksr":
		base["method"] = strField(p, "cipher")
		base["password"] = strField(p, "password")
		base["protocol"] = strField(p, "protocol")
		base["obfs"] = strField(p, "obfs")
		if pp := strField(p, "protocol-param"); pp != "" {
			base["protocol_param"] = pp
		}
		if op := strField(p, "obfs-param"); op != "" {
			base["obfs_param"] = op
		}

	default:
		return nil
	}

	return base
}

func addSingboxTransport(base, p map[string]interface{}) {
	network := strField(p, "network")
	switch network {
	case "ws":
		wsOpts := map[string]interface{}{
			"enabled": true,
		}
		if wsPath := strField(p, "ws-path"); wsPath != "" {
			wsOpts["path"] = wsPath
		}
		if headers := mapField(p, "ws-headers"); headers != nil {
			if host := fmt.Sprintf("%v", headers["Host"]); host != "" {
				wsOpts["headers"] = map[string]interface{}{"Host": host}
			}
		}
		base["transport"] = map[string]interface{}{
			"type": "ws",
			"ws_options": wsOpts,
		}
	case "grpc":
		grpcOpts := map[string]interface{}{
			"enabled": true,
		}
		if sn := strField(p, "grpc-service-name"); sn != "" {
			grpcOpts["service_name"] = sn
		}
		base["transport"] = map[string]interface{}{
			"type": "grpc",
			"grpc_options": grpcOpts,
		}
	}
}

func addSingboxTLS(base, p map[string]interface{}) {
	if boolField(p, "tls") {
		tlsOpts := map[string]interface{}{
			"enabled": true,
		}
		if sni := strField(p, "sni"); sni != "" {
			tlsOpts["server_name"] = sni
		}
		if boolField(p, "skip-cert-verify") {
			tlsOpts["insecure"] = true
		}
		base["tls"] = tlsOpts
	}
}

func toSingboxGroup(gm map[string]interface{}, proxyTags []string) map[string]interface{} {
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

	// Filter to only include valid outbound tags
	var validProxies []string
	for _, p := range proxies {
		if p == "DIRECT" || p == "REJECT" {
			continue // Already added as separate outbounds
		}
		validProxies = append(validProxies, p)
	}

	switch groupType {
	case "select":
		return map[string]interface{}{
			"type":      "selector",
			"tag":       groupName,
			"outbounds": validProxies,
			"default":   validProxies[0],
		}
	case "url-test":
		url := "http://www.gstatic.com/generate_204"
		if u, ok := gm["url"].(string); ok && u != "" {
			url = u
		}
		return map[string]interface{}{
			"type":      "urltest",
			"tag":       groupName,
			"outbounds": validProxies,
			"url":       url,
			"interval":  "5m",
		}
	case "load-balance":
		return map[string]interface{}{
			"type":      "selector",
			"tag":       groupName,
			"outbounds": validProxies,
		}
	default:
		return nil
	}
}

func toSingboxRule(rule string) map[string]interface{} {
	// Parse Clash rule format: TYPE,VALUE[,POLICY]
	parts := strings.Split(rule, ",")
	if len(parts) < 2 {
		return nil
	}

	ruleType := parts[0]

	switch {
	case strings.HasPrefix(rule, "DOMAIN-SUFFIX,"):
		if len(parts) >= 3 {
			return map[string]interface{}{
				"domain_suffix": []string{parts[1]},
				"outbound":      parts[2],
			}
		}
	case strings.HasPrefix(rule, "DOMAIN,"):
		if len(parts) >= 3 {
			return map[string]interface{}{
				"domain":   []string{parts[1]},
				"outbound": parts[2],
			}
		}
	case strings.HasPrefix(rule, "DOMAIN-KEYWORD,"):
		if len(parts) >= 3 {
			return map[string]interface{}{
				"domain_keyword": []string{parts[1]},
				"outbound":       parts[2],
			}
		}
	case strings.HasPrefix(rule, "IP-CIDR,"):
		if len(parts) >= 3 {
			return map[string]interface{}{
				"ip_cidr":  []string{parts[1]},
				"outbound": parts[2],
			}
		}
	case strings.HasPrefix(rule, "IP-CIDR6,"):
		if len(parts) >= 3 {
			return map[string]interface{}{
				"ip_cidr":  []string{parts[1]},
				"outbound": parts[2],
			}
		}
	case strings.HasPrefix(rule, "GEOIP,"):
		if len(parts) >= 2 {
			return map[string]interface{}{
				"geoip":    []string{parts[1]},
				"outbound": parts[1],
			}
		}
	case strings.HasPrefix(rule, "GEOSITE,"):
		if len(parts) >= 2 {
			return map[string]interface{}{
				"geosite":  []string{parts[1]},
				"outbound": parts[1],
			}
		}
	}

	_ = ruleType
	return nil
}
