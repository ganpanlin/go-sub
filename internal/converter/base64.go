package converter

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"
	"time"
)

func init() {
	Register("base64", &Base64Converter{})
	Register("uri", &Base64Converter{}) // alias
}

// Base64Converter outputs a base64-encoded URI list (ss://, vmess://, trojan://, vless://, hysteria2://, ssr://).
type Base64Converter struct{}

func (c *Base64Converter) Name() string { return "base64" }

func (c *Base64Converter) Convert(proxies []map[string]interface{}, groups []interface{}, ruleProviders map[string]interface{}, rules []string, extra map[string]interface{}) ([]byte, string, error) {
	var lines []string
	for _, p := range proxies {
		uri := proxyToURI(p)
		if uri != "" {
			lines = append(lines, uri)
		}
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n")))
	header := fmt.Sprintf("# Generated: %s\n# Nodes: %d\n\n", time.Now().Format(time.RFC3339), len(lines))

	return []byte(header + encoded), "text/plain; charset=utf-8", nil
}

func proxyToURI(p map[string]interface{}) string {
	proxyType := strField(p, "type")
	name := strField(p, "name")
	server := strField(p, "server")
	port := intField(p, "port")

	switch proxyType {
	case "vmess":
		return toVmessURI(p, name, server, port)
	case "ss":
		return toSSURI(p, name, server, port)
	case "trojan":
		return toTrojanURI(p, name, server, port)
	case "vless":
		return toVlessURI(p, name, server, port)
	case "hysteria2":
		return toHysteria2URI(p, name, server, port)
	case "ssr":
		return toSSRURI(p, name, server, port)
	default:
		return ""
	}
}

func toVmessURI(p map[string]interface{}, name, server string, port int) string {
	uuid := strField(p, "uuid")
	alterId := intField(p, "alterId")
	cipher := strField(p, "cipher")
	if cipher == "" {
		cipher = "auto"
	}
	network := strField(p, "network")
	if network == "" {
		network = "ws"
	}

	tls := "false"
	if boolField(p, "tls") {
		tls = "tls"
	}

	wsPath := strField(p, "ws-path")
	wsHost := ""
	if headers := mapField(p, "ws-headers"); headers != nil {
		wsHost = fmt.Sprintf("%v", headers["Host"])
	}

	vmessObj := map[string]interface{}{
		"v":    "2",
		"ps":   name,
		"add":  server,
		"port": port,
		"id":   uuid,
		"aid":  alterId,
		"scy":  cipher,
		"net":  network,
		"type": "none",
		"tls":  tls,
	}
	if wsPath != "" {
		vmessObj["path"] = wsPath
	}
	if wsHost != "" {
		vmessObj["host"] = wsHost
	}

	// Manual JSON encoding for vmess
	json := fmt.Sprintf(`{"v":"2","ps":"%s","add":"%s","port":%d,"id":"%s","aid":%d,"scy":"%s","net":"%s","type":"none","tls":"%s"`,
		escapeJSON(name), server, port, uuid, alterId, cipher, network, tls)
	if wsPath != "" {
		json += fmt.Sprintf(`,"path":"%s"`, escapeJSON(wsPath))
	}
	if wsHost != "" {
		json += fmt.Sprintf(`,"host":"%s"`, escapeJSON(wsHost))
	}
	if sni := strField(p, "sni"); sni != "" {
		json += fmt.Sprintf(`,"sni":"%s"`, sni)
	}
	if fp := strField(p, "client-fingerprint"); fp != "" {
		json += fmt.Sprintf(`,"fp":"%s"`, fp)
	}
	json += "}"

	return "vmess://" + base64.StdEncoding.EncodeToString([]byte(json))
}

func toSSURI(p map[string]interface{}, name, server string, port int) string {
	cipher := strField(p, "cipher")
	password := strField(p, "password")
	userInfo := base64.StdEncoding.EncodeToString([]byte(cipher + ":" + password))
	return fmt.Sprintf("ss://%s@%s:%d#%s", userInfo, server, port, url.QueryEscape(name))
}

func toTrojanURI(p map[string]interface{}, name, server string, port int) string {
	password := strField(p, "password")
	params := url.Values{}
	if sni := strField(p, "sni"); sni != "" {
		params.Set("sni", sni)
	}
	if boolField(p, "skip-cert-verify") {
		params.Set("allowInsecure", "1")
	}
	network := strField(p, "network")
	if network == "ws" {
		params.Set("type", "ws")
		if wsPath := strField(p, "ws-path"); wsPath != "" {
			params.Set("path", wsPath)
		}
		if headers := mapField(p, "ws-headers"); headers != nil {
			if host := fmt.Sprintf("%v", headers["Host"]); host != "" {
				params.Set("host", host)
			}
		}
	}
	return fmt.Sprintf("trojan://%s@%s:%d?%s#%s", password, server, port, params.Encode(), url.QueryEscape(name))
}

func toVlessURI(p map[string]interface{}, name, server string, port int) string {
	uuid := strField(p, "uuid")
	params := url.Values{}

	network := strField(p, "network")
	if network != "" {
		params.Set("type", network)
	}
	if network == "ws" {
		if wsPath := strField(p, "ws-path"); wsPath != "" {
			params.Set("path", wsPath)
		}
		if headers := mapField(p, "ws-headers"); headers != nil {
			if host := fmt.Sprintf("%v", headers["Host"]); host != "" {
				params.Set("host", host)
			}
		}
	} else if network == "grpc" {
		if sn := strField(p, "grpc-service-name"); sn != "" {
			params.Set("serviceName", sn)
		}
	}

	if boolField(p, "tls") {
		params.Set("security", "tls")
		if sni := strField(p, "sni"); sni != "" {
			params.Set("sni", sni)
		}
		if fp := strField(p, "client-fingerprint"); fp != "" {
			params.Set("fp", fp)
		}
	}
	if realityOpts := mapField(p, "reality-opts"); realityOpts != nil {
		params.Set("security", "reality")
		if pbk := fmt.Sprintf("%v", realityOpts["public-key"]); pbk != "" {
			params.Set("pbk", pbk)
		}
		if sid := fmt.Sprintf("%v", realityOpts["short-id"]); sid != "" {
			params.Set("sid", sid)
		}
		if sni := strField(p, "servername"); sni != "" {
			params.Set("sni", sni)
		}
		if fp := strField(p, "client-fingerprint"); fp != "" {
			params.Set("fp", fp)
		}
	}
	if flow := strField(p, "flow"); flow != "" {
		params.Set("flow", flow)
	}
	if alpn := strSliceField(p, "alpn"); alpn != nil {
		params.Set("alpn", strings.Join(alpn, ","))
	}

	return fmt.Sprintf("vless://%s@%s:%d?%s#%s", uuid, server, port, params.Encode(), url.QueryEscape(name))
}

func toHysteria2URI(p map[string]interface{}, name, server string, port int) string {
	password := strField(p, "password")
	params := url.Values{}
	if sni := strField(p, "sni"); sni != "" {
		params.Set("sni", sni)
	}
	if boolField(p, "skip-cert-verify") {
		params.Set("insecure", "1")
	}
	if obfs := strField(p, "obfs"); obfs != "" {
		params.Set("obfs", obfs)
		if obfsPwd := strField(p, "obfs-password"); obfsPwd != "" {
			params.Set("obfs-password", obfsPwd)
		}
	}
	return fmt.Sprintf("hysteria2://%s@%s:%d?%s#%s", password, server, port, params.Encode(), url.QueryEscape(name))
}

func toSSRURI(p map[string]interface{}, name, server string, port int) string {
	password := strField(p, "password")
	cipher := strField(p, "cipher")
	protocol := strField(p, "protocol")
	obfs := strField(p, "obfs")

	encPass := ssrEncode(password)
	obfsParam := strField(p, "obfs-param")
	protoParam := strField(p, "protocol-param")

	body := fmt.Sprintf("%s:%d:%s:%s:%s:%s", server, port, protocol, cipher, obfs, encPass)
	params := fmt.Sprintf("obfsparam=%s&protoparam=%s&remarks=%s", ssrEncode(obfsParam), ssrEncode(protoParam), ssrEncode(name))

	full := body + "/?" + params
	encoded := ssrSubstituteEncode(full)
	return "ssr://" + encoded
}

func ssrEncode(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	return strings.TrimRight(encoded, "=")
}

func ssrSubstituteEncode(s string) string {
	encoded := base64.StdEncoding.EncodeToString([]byte(s))
	encoded = strings.ReplaceAll(encoded, "+", "-")
	encoded = strings.ReplaceAll(encoded, "/", "_")
	return strings.TrimRight(encoded, "=")
}

func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
