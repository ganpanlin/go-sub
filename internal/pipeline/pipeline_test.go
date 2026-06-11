package pipeline

import (
	"fmt"
	"testing"
)

// TestDedup verifies that duplicate proxies are removed by type:server:port key.
func TestDedup(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"name": "B", "type": "ss", "server": "1.1.1.1", "port": 443}, // dup of A
		map[string]interface{}{"name": "C", "type": "vmess", "server": "2.2.2.2", "port": 8080},
		map[string]interface{}{"name": "D", "type": "ss", "server": "1.1.1.1", "port": 8443}, // different port
	}

	result := dedup(proxies)
	if len(result) != 3 {
		t.Fatalf("expected 3 after dedup, got %d", len(result))
	}

	// Should keep first occurrence
	names := make(map[string]bool)
	for _, p := range result {
		m := p.(map[string]interface{})
		names[m["name"].(string)] = true
	}
	if names["A"] && names["B"] {
		t.Fatal("should not keep both A and B")
	}
	if !names["A"] {
		t.Fatal("should keep first occurrence A")
	}
	if !names["C"] {
		t.Fatal("should keep C")
	}
	if !names["D"] {
		t.Fatal("should keep D")
	}
}

func TestDedup_Empty(t *testing.T) {
	result := dedup(nil)
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}

// TestValidate verifies that proxies without required fields are removed.
func TestValidate(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "good", "type": "ss", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"type": "ss", "server": "2.2.2.2", "port": 443},            // missing name
		map[string]interface{}{"name": "no-server", "type": "ss", "port": 443},             // missing server
		map[string]interface{}{"name": "no-type", "server": "3.3.3.3", "port": 443},        // missing type
		map[string]interface{}{"name": "no-port", "type": "ss", "server": "4.4.4.4"},       // missing port
		"not a map", // not a map
	}

	result := validate(proxies)
	if len(result) != 1 {
		t.Fatalf("expected 1 valid proxy, got %d", len(result))
	}
	m := result[0].(map[string]interface{})
	if m["name"] != "good" {
		t.Fatalf("expected 'good', got %v", m["name"])
	}
}

func TestValidate_AllValid(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "A", "type": "ss", "server": "1.1.1.1", "port": 443},
		map[string]interface{}{"name": "B", "type": "vmess", "server": "2.2.2.2", "port": 8080},
	}
	result := validate(proxies)
	if len(result) != 2 {
		t.Fatalf("expected 2, got %d", len(result))
	}
}

// TestExtractDomain verifies URL domain extraction.
func TestExtractDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/path", "example.com"},
		{"http://test.org:8080/sub", "test.org"},
		{"https://a.b.c/d/e/f", "a.b.c"},
		{"example.com/path", "example.com"},
		{"http://127.0.0.1:9090/api", "127.0.0.1"},
	}

	for _, tt := range tests {
		got := extractDomain(tt.input)
		if got != tt.want {
			t.Errorf("extractDomain(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestGetNames verifies proxy name extraction.
func TestGetNames(t *testing.T) {
	proxies := []interface{}{
		map[string]interface{}{"name": "HK_01"},
		map[string]interface{}{"name": "JP_02"},
		map[string]interface{}{"name": "US_03"},
	}
	names := getNames(proxies)
	if len(names) != 3 {
		t.Fatalf("expected 3, got %d", len(names))
	}
	if names[0] != "HK_01" || names[1] != "JP_02" || names[2] != "US_03" {
		t.Fatalf("unexpected names: %v", names)
	}
}

// TestGetSourcePrefix verifies source prefix generation.
func TestGetSourcePrefix(t *testing.T) {
	tests := []struct {
		url  string
		name string
		mode string
		want string
	}{
		{"https://example.com/sub", "MySource", "name", "MySource-"},
		{"https://example.com/sub", "MySource", "domain", "example.com-"},
		{"https://example.com/sub", "https://example.com/sub", "name", "example.com-"}, // name == url
		{"https://example.com/sub", "", "domain", "example.com-"},
		{"", "", "off", ""},
	}

	for _, tt := range tests {
		got := getSourcePrefix(tt.url, tt.name, tt.mode)
		if got != tt.want {
			t.Errorf("getSourcePrefix(%q,%q,%q) = %q, want %q", tt.url, tt.name, tt.mode, got, tt.want)
		}
	}
}

// TestDisplayURL verifies that data: URLs are displayed as local://.
func TestDisplayURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://example.com/sub", "https://example.com/sub"},
		{"data:text/plain;base64,abc123", "local://subscription"},
		{"http://test.org/api", "http://test.org/api"},
	}

	for _, tt := range tests {
		got := displayURL(tt.input)
		if got != tt.want {
			t.Errorf("displayURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// Make sure fmt is imported.
var _ = fmt.Sprintf
