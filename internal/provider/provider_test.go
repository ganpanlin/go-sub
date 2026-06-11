package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-sub/internal/appconfig"
)

// initTestConfig sets up a minimal appconfig for tests.
func initTestConfig() {
	appconfig.Init("0", "config.json", "testdata", "testdata", 5, 60, 10)
}

func TestGetRequestUA_Default(t *testing.T) {
	initTestConfig()
	ua := GetRequestUA()
	if ua == "" {
		t.Fatal("expected non-empty default UA")
	}
	if ua != UAPresets["clash"] {
		t.Fatalf("expected default clash UA, got %s", ua)
	}
}

func TestProxyCount_Nil(t *testing.T) {
	if ProxyCount(nil) != 0 {
		t.Fatal("expected 0 for nil config")
	}
}

func TestProxyCount_NoProxies(t *testing.T) {
	if ProxyCount(map[string]interface{}{}) != 0 {
		t.Fatal("expected 0 for config without proxies")
	}
}

func TestProxyCount_Valid(t *testing.T) {
	cfg := map[string]interface{}{
		"proxies": []interface{}{
			map[string]interface{}{"name": "A"},
			map[string]interface{}{"name": "B"},
			map[string]interface{}{"name": "C"},
		},
	}
	if ProxyCount(cfg) != 3 {
		t.Fatalf("expected 3, got %d", ProxyCount(cfg))
	}
}

func TestProxyCount_WrongType(t *testing.T) {
	cfg := map[string]interface{}{
		"proxies": "not a list",
	}
	if ProxyCount(cfg) != 0 {
		t.Fatal("expected 0 for wrong type")
	}
}

func TestParseDataURL_Invalid(t *testing.T) {
	initTestConfig()
	_, status, _, _, err := parseDataURL(context.Background(), "data:invalid")
	if err == nil {
		t.Fatal("expected error for invalid data URL")
	}
	_ = status
}

func TestParseDataURL_PlainText(t *testing.T) {
	initTestConfig()
	yamlContent := "proxies:\n  - name: test-node\n    type: ss\n    server: 1.2.3.4\n    port: 8388\n    cipher: aes-256-gcm\n    password: pass\n"
	url := "data:text/plain," + yamlContent

	config, status, _, _, err := parseDataURL(context.Background(), url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if ProxyCount(config) != 1 {
		t.Fatalf("expected 1 proxy, got %d", ProxyCount(config))
	}
}

func TestParseDataURL_Base64(t *testing.T) {
	initTestConfig()
	yamlContent := "proxies:\n  - name: b64-node\n    type: ss\n    server: 5.6.7.8\n    port: 443\n    cipher: chacha20-ietf-poly1305\n    password: test\n"
	url := "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(yamlContent))

	config, _, _, _, err := parseDataURL(context.Background(), url)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ProxyCount(config) != 1 {
		t.Fatalf("expected 1 proxy, got %d", ProxyCount(config))
	}
	proxies := config["proxies"].([]interface{})
	node := proxies[0].(map[string]interface{})
	if node["name"] != "b64-node" {
		t.Fatalf("expected b64-node, got %v", node["name"])
	}
}

func TestDoHTTPRequest_Success(t *testing.T) {
	initTestConfig()
	yamlContent := "proxies:\n  - name: http-node\n    type: trojan\n    server: 9.8.7.6\n    port: 443\n    password: pass\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(yamlContent))
	}))
	defer srv.Close()

	config, status, latency, cached, err := doHTTPRequest(context.Background(), srv.URL, "test-agent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if cached {
		t.Fatal("should not be cached on first fetch")
	}
	if latency < 0 {
		t.Fatalf("latency should be >= 0, got %d", latency)
	}
	if ProxyCount(config) != 1 {
		t.Fatalf("expected 1 proxy, got %d", ProxyCount(config))
	}
}

func TestDoHTTPRequest_Non200(t *testing.T) {
	initTestConfig()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	_, status, _, _, err := doHTTPRequest(context.Background(), srv.URL, "test-agent")
	if err == nil {
		t.Fatal("expected error for 403")
	}
	if status != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", status)
	}
}

func TestDoHTTPRequest_Cancelled(t *testing.T) {
	// Skip this test in short mode since it tests cancellation timing
	if testing.Short() {
		t.Skip("skipping cancellation test in short mode")
	}

	appconfig.Init("0", "config.json", "testdata", "testdata", 1, 60, 10)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, _, _, _, err := doHTTPRequest(ctx, srv.URL, "test-agent")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestFetchAndParseYAML_RetryOnServerError(t *testing.T) {
	initTestConfig()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write([]byte("proxies:\n  - name: retry-node\n    type: ss\n    server: 1.1.1.1\n    port: 443\n    cipher: aes-256-gcm\n    password: p\n"))
	}))
	defer srv.Close()

	// Use 0 backoff so retries are instant in tests
	origBackoff := DefaultRetryBackoff
	DefaultRetryBackoff = 1 * time.Millisecond
	defer func() { DefaultRetryBackoff = origBackoff }()

	config, status, _, _, err := fetchAndParseYAML(context.Background(), srv.URL, true, "", 3)
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if ProxyCount(config) != 1 {
		t.Fatalf("expected 1 proxy, got %d", ProxyCount(config))
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (2 failures + 1 success), got %d", callCount)
	}
}

func TestFetchAndParseYAML_NoRetryOn404(t *testing.T) {
	initTestConfig()
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, status, _, _, err := fetchAndParseYAML(context.Background(), srv.URL, true, "", 3)
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if status != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", status)
	}
	if callCount != 1 {
		t.Fatalf("expected 1 call (no retry on 404), got %d", callCount)
	}
}

func TestIsTLSError(t *testing.T) {
	tests := []struct {
		errMsg string
		want   bool
	}{
		{"tls: failed to verify certificate: x509: certificate has expired", true},
		{"x509: certificate signed by unknown authority", true},
		{"TLS handshake failure: remote error", true},
		{"connection refused", false},
		{"timeout awaiting response headers", false},
		{"", false},
	}
	for _, tt := range tests {
		var err error
		if tt.errMsg != "" {
			err = fmt.Errorf("%s", tt.errMsg)
		}
		got := isTLSError(err)
		if got != tt.want {
			t.Errorf("isTLSError(%q) = %v, want %v", tt.errMsg, got, tt.want)
		}
	}
}

func TestDoHTTPRequest_TLSRetryOnCertError(t *testing.T) {
	initTestConfig()

	// Create an HTTPS server with a self-signed cert that will fail verification
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("proxies:\n  - name: tls-node\n    type: ss\n    server: 1.2.3.4\n    port: 443\n    cipher: aes-256-gcm\n    password: p\n"))
	}))
	defer srv.Close()

	// httptest.NewTLSServer creates a server with a test certificate.
	// Without adding its cert to the client's CA pool, the request will fail
	// with a certificate error → triggers TLS retry → succeeds.
	config, status, _, _, err := doHTTPRequest(context.Background(), srv.URL, "test-agent")
	if err != nil {
		t.Fatalf("expected TLS retry to succeed, got error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d", status)
	}
	if ProxyCount(config) != 1 {
		t.Fatalf("expected 1 proxy, got %d", ProxyCount(config))
	}
}
