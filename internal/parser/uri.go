package parser

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go-sub/pkg/utils"
	"net/url"
	"strconv"
	"strings"
)

func parseURIListToConfig(content string) (map[string]interface{}, error) {
	lines := strings.Split(content, "\n")
	var proxies []interface{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		var proxy map[string]interface{}
		if strings.HasPrefix(line, "vmess://") {
			proxy = parseVmessURI(line)
		} else if strings.HasPrefix(line, "ss://") {
			proxy = parseSsURI(line)
		} else if strings.HasPrefix(line, "trojan://") {
			proxy = parseTrojanURI(line)
		} else if strings.HasPrefix(line, "vless://") {
			proxy = parseVlessURI(line)
		} else if strings.HasPrefix(line, "hysteria2://") {
			proxy = parseHysteria2URI(line)
		} else if strings.HasPrefix(line, "ssr://") {
			proxy = parseSsrURI(line)
		}

		if proxy != nil {
			proxies = append(proxies, proxy)
		}
	}

	if len(proxies) == 0 {
		return nil, fmt.Errorf("no valid proxy URIs found")
	}

	proxyNames := make([]string, len(proxies))
	for i, p := range proxies {
		if pMap, ok := p.(map[string]interface{}); ok {
			proxyNames[i] = pMap["name"].(string)
		}
	}

	config := map[string]interface{}{
		"proxies": proxies,
		"proxy-groups": []interface{}{
			map[string]interface{}{
				"name":    "PROXY",
				"type":    "select",
				"proxies": append([]string{"DIRECT", "REJECT"}, proxyNames...),
			},
		},
		"rules": []string{
			"MATCH,PROXY",
		},
	}

	return config, nil
}

func parseVmessURI(uri string) map[string]interface{} {
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(uri, "vmess://"))
	if err != nil {
		return nil
	}

	var vmessConfig map[string]interface{}
	if err := json.Unmarshal(decoded, &vmessConfig); err != nil {
		return nil
	}

	port, _ := strconv.Atoi(utils.SafeToString(vmessConfig["port"]))
	aid, _ := strconv.Atoi(utils.SafeToString(vmessConfig["aid"]))

	return map[string]interface{}{
		"name":             utils.SafeToString(vmessConfig["ps"]),
		"type":             "vmess",
		"server":           utils.SafeToString(vmessConfig["add"]),
		"port":             port,
		"uuid":             utils.SafeToString(vmessConfig["id"]),
		"alterId":          aid,
		"cipher":           "auto",
		"tls":              utils.SafeToString(vmessConfig["tls"]) == "tls",
		"skip-cert-verify": true,
		"network":          utils.SafeToString(vmessConfig["net"]),
		"ws-path":          utils.SafeToString(vmessConfig["path"]),
		"ws-headers":       map[string]string{"Host": utils.SafeToString(vmessConfig["host"])},
	}
}

func parseSsURI(uri string) map[string]interface{} {
	u, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	decoded, err := base64.StdEncoding.DecodeString(u.User.String())
	if err != nil {
		return nil
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil
	}

	port, _ := strconv.Atoi(u.Port())

	return map[string]interface{}{
		"name":     u.Fragment,
		"type":     "ss",
		"server":   u.Hostname(),
		"port":     port,
		"cipher":   parts[0],
		"password": parts[1],
	}
}

func parseTrojanURI(uri string) map[string]interface{} {
	u, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	password, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())

	return map[string]interface{}{
		"name":             u.Fragment,
		"type":             "trojan",
		"server":           u.Hostname(),
		"port":             port,
		"password":         password,
		"sni":              u.Query().Get("sni"),
		"skip-cert-verify": true,
	}
}

func parseVlessURI(uri string) map[string]interface{} {
	u, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	uuid := u.User.Username()
	port, _ := strconv.Atoi(u.Port())
	query := u.Query()

	network := query.Get("type")
	security := query.Get("security")

	proxy := map[string]interface{}{
		"name":             u.Fragment,
		"type":             "vless",
		"server":           u.Hostname(),
		"port":             port,
		"uuid":             uuid,
		"tls":              security == "tls" || security == "reality",
		"skip-cert-verify": true,
		"network":          network,
	}

	// Transport layer
	if network == "ws" {
		proxy["ws-path"] = query.Get("path")
		proxy["ws-headers"] = map[string]string{"Host": query.Get("host")}
	} else if network == "grpc" {
		proxy["grpc-service-name"] = query.Get("serviceName")
	}

	// TLS layer
	if sni := query.Get("sni"); sni != "" {
		proxy["sni"] = sni
	}
	if fp := query.Get("fp"); fp != "" {
		proxy["client-fingerprint"] = fp
	}
	if alpn := query.Get("alpn"); alpn != "" {
		proxy["alpn"] = strings.Split(alpn, ",")
	}

	// Reality
	if security == "reality" {
		proxy["reality-opts"] = map[string]interface{}{
			"public-key": query.Get("pbk"),
			"short-id":   query.Get("sid"),
		}
		if servername := query.Get("sni"); servername != "" {
			proxy["servername"] = servername
		}
	}

	// Flow (XTLS)
	if flow := query.Get("flow"); flow != "" {
		proxy["flow"] = flow
	}

	return proxy
}

func parseHysteria2URI(uri string) map[string]interface{} {
	u, err := url.Parse(uri)
	if err != nil {
		return nil
	}

	password, _ := u.User.Password()
	port, _ := strconv.Atoi(u.Port())
	query := u.Query()

	proxy := map[string]interface{}{
		"name":             u.Fragment,
		"type":             "hysteria2",
		"server":           u.Hostname(),
		"port":             port,
		"password":         password,
		"skip-cert-verify": query.Get("insecure") == "1",
	}

	if sni := query.Get("sni"); sni != "" {
		proxy["sni"] = sni
	}

	// Bandwidth / speed settings
	if up := query.Get("upmbps"); up != "" {
		if v, err := strconv.Atoi(up); err == nil {
			proxy["up"] = v
		}
	}
	if down := query.Get("downmbps"); down != "" {
		if v, err := strconv.Atoi(down); err == nil {
			proxy["down"] = v
		}
	}

	// Obfuscation
	if obfs := query.Get("obfs"); obfs != "" {
		proxy["obfs"] = obfs
		if obfsPassword := query.Get("obfs-password"); obfsPassword != "" {
			proxy["obfs-password"] = obfsPassword
		}
	}

	return proxy
}

func parseSsrURI(uri string) map[string]interface{} {
	// SSR URI format: ssr://base64(host:port:protocol:method:obfs:base64password/?params)
	encoded := strings.TrimPrefix(uri, "ssr://")
	// SSR uses custom Base64: replace - with + and _ with /
	encoded = strings.ReplaceAll(encoded, "-", "+")
	encoded = strings.ReplaceAll(encoded, "_", "/")
	// Pad if necessary
	if m := len(encoded) % 4; m != 0 {
		encoded += strings.Repeat("=", 4-m)
	}

	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil
	}

	// Split main body and params
	parts := strings.SplitN(string(decoded), "/?", 2)
	body := parts[0]

	// body: host:port:protocol:method:obfs:base64password
	fields := strings.Split(body, ":")
	if len(fields) < 6 {
		return nil
	}

	host := fields[0]
	port, _ := strconv.Atoi(fields[1])
	protocol := fields[2]
	method := fields[3]
	obfs := fields[4]

	// Decode password
	passEncoded := fields[5]
	passEncoded = strings.ReplaceAll(passEncoded, "-", "+")
	passEncoded = strings.ReplaceAll(passEncoded, "_", "/")
	if m := len(passEncoded) % 4; m != 0 {
		passEncoded += strings.Repeat("=", 4-m)
	}
	password, _ := base64.StdEncoding.DecodeString(passEncoded)

	// Parse optional params
	var obfsParam, protocolParam string
	name := host
	if len(parts) > 1 {
		paramStr := parts[1]
		if paramDecoded, err := base64.StdEncoding.DecodeString(paramStr); err == nil {
			paramStr = string(paramDecoded)
		}
		for _, p := range strings.Split(paramStr, "&") {
			kv := strings.SplitN(p, "=", 2)
			if len(kv) == 2 {
				val := kv[1]
				if val, err := base64.StdEncoding.DecodeString(val); err == nil {
					valStr := string(val)
					switch kv[0] {
					case "obfsparam":
						obfsParam = valStr
					case "protoparam":
						protocolParam = valStr
					case "remarks":
						name = valStr
					}
					continue
				}
				switch kv[0] {
				case "obfsparam":
					obfsParam = val
				case "protoparam":
					protocolParam = val
				case "remarks":
					name = val
				}
			}
		}
	}

	proxy := map[string]interface{}{
		"name":     name,
		"type":     "ssr",
		"server":   host,
		"port":     port,
		"password": string(password),
		"cipher":   method,
		"protocol": protocol,
		"obfs":     obfs,
	}

	if obfsParam != "" {
		proxy["obfs-param"] = obfsParam
	}
	if protocolParam != "" {
		proxy["protocol-param"] = protocolParam
	}

	return proxy
}
