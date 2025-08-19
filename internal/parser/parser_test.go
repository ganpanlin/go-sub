package parser

import (
	"encoding/base64"
	"testing"
)

func TestIsIPAddress(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1.2.3.4", true},
		{"255.255.255.255", true},
		{"0.0.0.0", true},
		{"example.com", false},
		{"sub.example.com", false},
		{"", false},
		{"not-an-ip", false},
		{"192.168.1.1", true},
	}

	for _, tt := range tests {
		result := IsIPAddress(tt.input)
		if result != tt.expected {
			t.Errorf("IsIPAddress(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestIsDomainName(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"example.com", true},
		{"sub.example.com", true},
		{"a-b.c-d.com", true},
		{"1.2.3.4", false}, // IP is not a domain
		{"", false},
		{"-invalid.com", false},
	}

	for _, tt := range tests {
		result := IsDomainName(tt.input)
		if result != tt.expected {
			t.Errorf("IsDomainName(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestParseVmessURI(t *testing.T) {
	// vmess://eyJhZGQiOiIxLjIuMy40IiwgImFpZCI6IjAiLCAiaG9zdCI6IiIsICJpZCI6InV1aWQtdGVzdCIsICJuZXQiOiJ3cyIsICJwYXRoIjoiLyIsICJwb3J0IjoiNDQzIiwgInBzIjoiVGVzdC1Ob2RlIiwgInRscyI6IiJ9
	uri := "vmess://eyJhZGQiOiIxLjIuMy40IiwgImFpZCI6IjAiLCAiaG9zdCI6IiIsICJpZCI6InV1aWQtdGVzdCIsICJuZXQiOiJ3cyIsICJwYXRoIjoiLyIsICJwb3J0IjoiNDQzIiwgInBzIjoiVGVzdC1Ob2RlIiwgInRscyI6IiJ9"
	proxy := parseVmessURI(uri)
	if proxy == nil {
		t.Fatal("expected valid proxy from vmess URI")
	}
	if proxy["type"] != "vmess" {
		t.Errorf("expected type 'vmess', got %v", proxy["type"])
	}
	if proxy["name"] != "Test-Node" {
		t.Errorf("expected name 'Test-Node', got %v", proxy["name"])
	}
	if proxy["server"] != "1.2.3.4" {
		t.Errorf("expected server '1.2.3.4', got %v", proxy["server"])
	}
}

func TestParseSsURI(t *testing.T) {
	// ss://base64(method:password)@host:port#name
	uri := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@1.2.3.4:8388#TestNode"
	proxy := parseSsURI(uri)
	if proxy == nil {
		t.Fatal("expected valid proxy from ss URI")
	}
	if proxy["type"] != "ss" {
		t.Errorf("expected type 'ss', got %v", proxy["type"])
	}
	if proxy["server"] != "1.2.3.4" {
		t.Errorf("expected server '1.2.3.4', got %v", proxy["server"])
	}
	if proxy["name"] != "TestNode" {
		t.Errorf("expected name 'TestNode', got %v", proxy["name"])
	}
}

func TestParseTrojanURI(t *testing.T) {
	uri := "trojan://password123@1.2.3.4:443?sni=example.com#TrojanNode"
	proxy := parseTrojanURI(uri)
	if proxy == nil {
		t.Fatal("expected valid proxy from trojan URI")
	}
	if proxy["type"] != "trojan" {
		t.Errorf("expected type 'trojan', got %v", proxy["type"])
	}
	if proxy["sni"] != "example.com" {
		t.Errorf("expected sni 'example.com', got %v", proxy["sni"])
	}
}

func TestParseVlessURI(t *testing.T) {
	uri := "vless://uuid-test@1.2.3.4:443?type=ws&security=reality&sni=example.com&pbk=pubkey123&sid=abc#VlessNode"
	proxy := parseVlessURI(uri)
	if proxy == nil {
		t.Fatal("expected valid proxy from vless URI")
	}
	if proxy["type"] != "vless" {
		t.Errorf("expected type 'vless', got %v", proxy["type"])
	}
	if proxy["server"] != "1.2.3.4" {
		t.Errorf("expected server '1.2.3.4', got %v", proxy["server"])
	}
	if proxy["ws-path"] != "" {
		t.Errorf("expected empty ws-path for vless, got %v", proxy["ws-path"])
	}
}

func TestParseHysteria2URI(t *testing.T) {
	uri := "hysteria2://password@1.2.3.4:443?sni=example.com&insecure=1#HysNode"
	proxy := parseHysteria2URI(uri)
	if proxy == nil {
		t.Fatal("expected valid proxy from hysteria2 URI")
	}
	if proxy["type"] != "hysteria2" {
		t.Errorf("expected type 'hysteria2', got %v", proxy["type"])
	}
	if proxy["sni"] != "example.com" {
		t.Errorf("expected sni 'example.com', got %v", proxy["sni"])
	}
}

func TestParseSsrURI(t *testing.T) {
	// ssr://base64-encoded string
	// Format: host:port:protocol:method:obfs:base64password
	// 1.2.3.4:443:origin:auto:none:aHR0cHM6Ly9leGFtcGxlLmNvbQ==
	encoded := "ssr://" + base64Encode("1.2.3.4:443:origin:auto:none:aHR0cHM6Ly9leGFtcGxlLmNvbQ==")
	proxy := parseSsrURI(encoded)
	if proxy == nil {
		t.Fatal("expected valid proxy from ssr URI")
	}
	if proxy["type"] != "ssr" {
		t.Errorf("expected type 'ssr', got %v", proxy["type"])
	}
	if proxy["server"] != "1.2.3.4" {
		t.Errorf("expected server '1.2.3.4', got %v", proxy["server"])
	}
}

func base64Encode(s string) string {
	return base64.URLEncoding.EncodeToString([]byte(s))
}
