package latency

import (
	"go-sub/internal/healthcheck"
	"net"
	"testing"
	"time"
)

func TestTestNodeInvalidProxyType(t *testing.T) {
	res := TestNode(map[string]interface{}{
		"name": "bad", "type": "no-such-protocol", "server": "127.0.0.1", "port": 1,
	}, "", time.Second)
	if res.Alive || res.Latency != -1 {
		t.Fatalf("invalid proxy should fail, got %+v", res)
	}
	if res.Error == "" {
		t.Fatal("expected error message for unsupported proxy type")
	}
	if res.TestType != "url" {
		t.Fatalf("TestType should be url, got %q", res.TestType)
	}
	if res.Name != "bad" || res.Server != "127.0.0.1" || res.Port != 1 {
		t.Fatalf("identity fields not preserved: %+v", res)
	}
}

func TestTestNodeMissingFields(t *testing.T) {
	res := TestNode(map[string]interface{}{"name": "x", "type": "ss"}, "", time.Second)
	if res.Alive || res.Error == "" {
		t.Fatalf("proxy without server/port should fail, got %+v", res)
	}
}

// A TCP listener that never completes the proxy handshake: dial succeeds but
// the url-test must fail — proving we measure THROUGH the protocol, not raw TCP.
func TestTestNodeThroughDummyListener(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind local listener")
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c // accept and stay silent
		}
	}()

	port := l.Addr().(*net.TCPAddr).Port
	res := TestNode(map[string]interface{}{
		"name": "silent-socks", "type": "socks5", "server": "127.0.0.1", "port": port,
	}, "", 2*time.Second)
	if res.Alive {
		t.Fatal("silent listener must not pass a real url-test")
	}
	if res.Latency != -1 || res.Error == "" {
		t.Fatalf("expected failure with error, got %+v", res)
	}
}

func TestTestNodesPreservesOrderAndCompletes(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("cannot bind local listener")
	}
	defer l.Close()
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c
		}
	}()
	port := l.Addr().(*net.TCPAddr).Port

	proxies := []map[string]interface{}{
		{"name": "a", "type": "socks5", "server": "127.0.0.1", "port": port},
		{"name": "b", "type": "broken", "server": "127.0.0.1", "port": port},
		{"name": "c", "type": "socks5", "server": "127.0.0.1", "port": port},
	}
	results := TestNodes(proxies, "", 2*time.Second, 3)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, want := range []string{"a", "b", "c"} {
		if results[i].Name != want {
			t.Fatalf("results[%d].Name = %q, want %q (order must match input)", i, results[i].Name, want)
		}
		if results[i].Alive {
			t.Fatalf("results[%d] should fail against silent listener", i)
		}
	}
	var _ = healthcheck.NodeResult{}
}
