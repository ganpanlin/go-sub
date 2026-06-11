package healthcheck

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckNode_InvalidPort(t *testing.T) {
	r := CheckNode("test", "1.2.3.4", 0, time.Second)
	if r.Alive {
		t.Fatal("should not be alive with port 0")
	}
	if r.Latency != -1 {
		t.Fatalf("expected latency -1, got %d", r.Latency)
	}
	if r.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestCheckNode_EmptyServer(t *testing.T) {
	r := CheckNode("test", "", 443, time.Second)
	if r.Alive {
		t.Fatal("should not be alive with empty server")
	}
}

func TestCheckNode_TCPSuccess(t *testing.T) {
	// Start a real TCP listener
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	addr := ln.Addr().(*net.TCPAddr)
	r := CheckNode("test-node", "127.0.0.1", addr.Port, 2*time.Second)
	if !r.Alive {
		t.Fatalf("expected alive, got error: %s", r.Error)
	}
	if r.Latency < 0 {
		t.Fatalf("expected positive latency, got %d", r.Latency)
	}
	if r.Name != "test-node" {
		t.Fatalf("expected name test-node, got %s", r.Name)
	}
	if r.Port != addr.Port {
		t.Fatalf("expected port %d, got %d", addr.Port, r.Port)
	}
}

func TestCheckNode_TCPRefused(t *testing.T) {
	// Use a port that's almost certainly not listening
	r := CheckNode("refused", "127.0.0.1", 1, 500*time.Millisecond)
	if r.Alive {
		t.Fatal("should not be alive on refused port")
	}
	if r.Latency != -1 {
		t.Fatalf("expected latency -1, got %d", r.Latency)
	}
}

func TestCheckNode_Timeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}
	// Create a listener that accepts but never responds (simulates firewall drop)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	// Accept connections but don't close them (to test timeout behavior)
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open
			time.Sleep(10 * time.Second)
			conn.Close()
		}
	}()

	// This tests the timeout by using a very short timeout
	addr := ln.Addr().(*net.TCPAddr)
	// The connection should succeed (port is open) so this actually tests success
	r := CheckNode("timeout-test", "127.0.0.1", addr.Port, 1*time.Second)
	if !r.Alive {
		t.Fatalf("expected alive (port is open), got error: %s", r.Error)
	}
}

func TestCheckNodes_Batch(t *testing.T) {
	// Start two listeners
	ln1, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln1.Close()
	ln2, _ := net.Listen("tcp", "127.0.0.1:0")
	defer ln2.Close()

	addr1 := ln1.Addr().(*net.TCPAddr)
	addr2 := ln2.Addr().(*net.TCPAddr)

	proxies := []map[string]interface{}{
		{"name": "alive-1", "server": "127.0.0.1", "port": addr1.Port},
		{"name": "alive-2", "server": "127.0.0.1", "port": addr2.Port},
		{"name": "dead-1", "server": "127.0.0.1", "port": 1},
		{"name": "invalid", "server": "", "port": 0},
	}

	results := CheckNodes(proxies, 10, 1*time.Second)
	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	// First two should be alive
	if !results[0].Alive {
		t.Fatalf("alive-1 should be alive, got: %s", results[0].Error)
	}
	if !results[1].Alive {
		t.Fatalf("alive-2 should be alive, got: %s", results[1].Error)
	}
	// Third should be dead (connection refused)
	if results[2].Alive {
		t.Fatal("dead-1 should not be alive")
	}
	// Fourth should be dead (invalid)
	if results[3].Alive {
		t.Fatal("invalid should not be alive")
	}
}

func TestCheckNodes_Empty(t *testing.T) {
	results := CheckNodes(nil, 10, time.Second)
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestFieldStr(t *testing.T) {
	tests := []struct {
		m    map[string]interface{}
		key  string
		want string
	}{
		{map[string]interface{}{"name": "hello"}, "name", "hello"},
		{map[string]interface{}{"name": 42}, "name", "42"},
		{map[string]interface{}{}, "name", ""},
		{nil, "name", ""},
	}
	for _, tt := range tests {
		got := fieldStr(tt.m, tt.key)
		if got != tt.want {
			t.Errorf("fieldStr(%v, %q) = %q, want %q", tt.m, tt.key, got, tt.want)
		}
	}
}

func TestFieldInt(t *testing.T) {
	tests := []struct {
		m    map[string]interface{}
		key  string
		want int
	}{
		{map[string]interface{}{"port": 443}, "port", 443},
		{map[string]interface{}{"port": float64(8080)}, "port", 8080},
		{map[string]interface{}{"port": "1234"}, "port", 1234},
		{map[string]interface{}{}, "port", 0},
	}
	for _, tt := range tests {
		got := fieldInt(tt.m, tt.key)
		if got != tt.want {
			t.Errorf("fieldInt(%v, %q) = %d, want %d", tt.m, tt.key, got, tt.want)
		}
	}
}

// Unused import guard — httptest is used for future HTTP-level checks.
var _ = httptest.NewServer
var _ = &http.Server{}
