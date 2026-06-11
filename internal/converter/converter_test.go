package converter

import (
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// testProxies is a shared set of proxy nodes for all converter tests.
var testProxies = []map[string]interface{}{
	{
		"name":     "HK_01",
		"type":     "ss",
		"server":   "1.2.3.4",
		"port":     8388,
		"cipher":   "aes-256-gcm",
		"password": "testpass",
		"udp":      true,
	},
	{
		"name":     "JP_02",
		"type":     "vmess",
		"server":   "5.6.7.8",
		"port":     443,
		"uuid":     "12345678-1234-1234-1234-123456789abc",
		"alterId":  0,
		"cipher":   "auto",
		"network":  "ws",
		"tls":      true,
		"sni":      "example.com",
		"ws-opts":  map[string]interface{}{"path": "/ws", "headers": map[string]interface{}{"Host": "example.com"}},
	},
	{
		"name":     "US_03",
		"type":     "trojan",
		"server":   "9.10.11.12",
		"port":     443,
		"password": "trojanpass",
		"tls":      true,
		"sni":      "trojan.example.com",
		"udp":      true,
	},
}

var testGroups = []interface{}{
	map[string]interface{}{
		"name":        "自动选择",
		"type":        "url-test",
		"proxies":     []string{"HK_01", "JP_02", "US_03"},
		"include-all": true,
		"url":         "http://www.gstatic.com/generate_204",
	},
}

var testRules = []string{
	"GEOIP,CN,DIRECT",
	"MATCH,Proxy",
}

// === Converter Registry ===

func TestGet_Default(t *testing.T) {
	c := Get("nonexistent")
	if c == nil {
		t.Fatal("expected default clash converter")
	}
	if c.Name() != "clash" {
		t.Fatalf("expected clash, got %s", c.Name())
	}
}

func TestGet_AllTypes(t *testing.T) {
	types := []string{"clash", "base64", "uri", "surge", "loon", "singbox", "sing-box", "quantumult", "quanx"}
	for _, typ := range types {
		c := Get(typ)
		if c == nil {
			t.Fatalf("converter %q not found", typ)
		}
	}
}

func TestAvailableTypes(t *testing.T) {
	types := AvailableTypes()
	if len(types) < 6 {
		t.Fatalf("expected at least 6 types, got %d: %v", len(types), types)
	}
}

// === Clash ===

func TestClashConvert(t *testing.T) {
	c := &ClashConverter{}
	out, ct, err := c.Convert(testProxies, testGroups, nil, testRules, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ct != "text/yaml; charset=utf-8" {
		t.Fatalf("unexpected content type: %s", ct)
	}

	// Verify it's valid YAML with proxies
	var config map[string]interface{}
	if err := yaml.Unmarshal(out, &config); err != nil {
		t.Fatalf("invalid YAML output: %v", err)
	}
	proxies, ok := config["proxies"].([]interface{})
	if !ok {
		t.Fatal("proxies not found in output")
	}
	if len(proxies) != 3 {
		t.Fatalf("expected 3 proxies, got %d", len(proxies))
	}

	// Verify rules
	rules, ok := config["rules"].([]interface{})
	if !ok {
		t.Fatal("rules not found")
	}
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}
}

func TestClashConvert_Empty(t *testing.T) {
	c := &ClashConverter{}
	out, _, err := c.Convert(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

// === Base64 URI ===

func TestBase64Convert(t *testing.T) {
	c := &Base64Converter{}
	out, ct, err := c.Convert(testProxies, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected content type: %s", ct)
	}

	output := string(out)
	// Output has a header comment then base64 block
	if !strings.Contains(output, "# Generated:") {
		t.Fatal("expected header comment")
	}
	// Find the base64 part (after blank line)
	parts := strings.SplitN(output, "\n\n", 2)
	if len(parts) < 2 {
		t.Fatal("expected header + base64 separated by blank line")
	}
	encoded := strings.TrimSpace(parts[1])
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("invalid base64: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(decoded)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if !strings.HasPrefix(lines[0], "ss://") {
		t.Fatalf("expected ss:// prefix, got %s", lines[0][:10])
	}
}

// === Surge ===

func TestSurgeConvert(t *testing.T) {
	c := &SurgeConverter{}
	out, ct, err := c.Convert(testProxies, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected content type: %s", ct)
	}

	output := string(out)
	if !strings.Contains(output, "HK_01") {
		t.Fatal("expected HK_01 in output")
	}
	if !strings.Contains(output, "JP_02") {
		t.Fatal("expected JP_02 in output")
	}
}

// === Sing-box ===

func TestSingboxConvert(t *testing.T) {
	c := &SingboxConverter{}
	out, ct, err := c.Convert(testProxies, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("unexpected content type: %s", ct)
	}

	// Skip header comments (lines starting with //)
	output := string(out)
	lines := strings.Split(output, "\n")
	var jsonStart int
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "{") {
			jsonStart = i
			break
		}
	}
	jsonStr := strings.Join(lines[jsonStart:], "\n")

	var config map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	outbounds, ok := config["outbounds"].([]interface{})
	if !ok {
		t.Fatal("outbounds not found in output")
	}
	// 3 proxy + DIRECT + REJECT + dns-out = 6
	if len(outbounds) < 3 {
		t.Fatalf("expected at least 3 outbounds, got %d", len(outbounds))
	}
}

// === Loon ===

func TestLoonConvert(t *testing.T) {
	c := &LoonConverter{}
	out, ct, err := c.Convert(testProxies, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected content type: %s", ct)
	}

	output := string(out)
	if !strings.Contains(output, "HK_01") {
		t.Fatal("expected HK_01 in output")
	}
}

// === Quantumult X ===

func TestQuantumultConvert(t *testing.T) {
	c := &QuantumultConverter{}
	out, ct, err := c.Convert(testProxies, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ct, "text/plain") {
		t.Fatalf("unexpected content type: %s", ct)
	}

	output := string(out)
	if !strings.Contains(output, "HK_01") {
		t.Fatal("expected HK_01 in output")
	}
}

// === Helper functions ===

func TestStrField(t *testing.T) {
	m := map[string]interface{}{"name": "test", "port": 443}
	if strField(m, "name") != "test" {
		t.Fatal("expected 'test'")
	}
	if strField(m, "missing") != "" {
		t.Fatal("expected empty string")
	}
}

func TestIntField(t *testing.T) {
	m := map[string]interface{}{"port": 443, "name": "hello"}
	if intField(m, "port") != 443 {
		t.Fatalf("expected 443, got %d", intField(m, "port"))
	}
	if intField(m, "missing") != 0 {
		t.Fatal("expected 0")
	}
}

func TestBoolField(t *testing.T) {
	m := map[string]interface{}{"tls": true, "udp": "true", "name": "x"}
	if !boolField(m, "tls") {
		t.Fatal("expected true")
	}
	if !boolField(m, "udp") {
		t.Fatal("expected true for string 'true'")
	}
	if boolField(m, "name") {
		t.Fatal("expected false")
	}
}

func TestMapField(t *testing.T) {
	m := map[string]interface{}{
		"ws-opts": map[string]interface{}{"path": "/ws"},
	}
	result := mapField(m, "ws-opts")
	if result == nil || result["path"] != "/ws" {
		t.Fatal("expected ws-opts map")
	}
	if mapField(m, "missing") != nil {
		t.Fatal("expected nil for missing key")
	}
}

func TestStrSliceField(t *testing.T) {
	m := map[string]interface{}{
		"proxies": []string{"a", "b"},
	}
	result := strSliceField(m, "proxies")
	if len(result) != 2 || result[0] != "a" {
		t.Fatalf("unexpected result: %v", result)
	}
}

func TestWriteOutput(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteOutput(rec, []byte("test"), "text/plain")
	if rec.Code != 200 {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/plain" {
		t.Fatal("expected content-type header")
	}
}

