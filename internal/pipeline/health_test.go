package pipeline

import (
	"go-sub/internal/healthcheck"
	"go-sub/internal/rule"
	"testing"
)

// seedHealth seeds the health store and returns a restore func.
func seedHealth(t *testing.T, results []healthcheck.NodeResult) {
	t.Helper()
	healthcheck.ResetStoreForTesting()
	healthcheck.UpdateResults(results)
	t.Cleanup(healthcheck.ResetStoreForTesting)
}

func proxy(name, server string, port int) map[string]interface{} {
	return map[string]interface{}{"name": name, "type": "ss", "server": server, "port": port}
}

func TestApplyHealthFilterDisabled(t *testing.T) {
	seedHealth(t, []healthcheck.NodeResult{
		{Server: "1.1.1.1", Port: 443, Alive: false, Latency: -1},
	})
	p := &rule.Profile{}
	proxies := []interface{}{proxy("A", "1.1.1.1", 443)}

	filtered, dropped := applyHealthFilter(proxies, p)
	if len(filtered) != 1 || dropped != 0 {
		t.Fatalf("filter disabled should keep all, got %d kept %d dropped", len(filtered), dropped)
	}
}

func TestApplyHealthFilterMaxLatency(t *testing.T) {
	seedHealth(t, []healthcheck.NodeResult{
		{Server: "fast", Port: 443, Alive: true, Latency: 80},
		{Server: "slow", Port: 443, Alive: true, Latency: 900},
		{Server: "dead", Port: 443, Alive: false, Latency: -1},
	})
	p := &rule.Profile{MaxLatency: 500}
	proxies := []interface{}{
		proxy("fast", "fast", 443),
		proxy("slow", "slow", 443),
		proxy("dead", "dead", 443),
		proxy("unknown", "unknown", 443), // never checked → kept
	}

	filtered, dropped := applyHealthFilter(proxies, p)
	if dropped != 2 {
		t.Fatalf("expected 2 dropped (slow+dead), got %d", dropped)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 kept (fast+unknown), got %d", len(filtered))
	}
	if nameOf(filtered[0]) != "fast" || nameOf(filtered[1]) != "unknown" {
		t.Fatalf("unexpected keep set: %v, %v", nameOf(filtered[0]), nameOf(filtered[1]))
	}
}

func TestApplyHealthFilterDropDeadOnly(t *testing.T) {
	seedHealth(t, []healthcheck.NodeResult{
		{Server: "dead", Port: 443, Alive: false, Latency: -1},
		{Server: "slow", Port: 443, Alive: true, Latency: 5000},
	})
	p := &rule.Profile{DropDead: true}
	proxies := []interface{}{
		proxy("dead", "dead", 443),
		proxy("slow", "slow", 443),
	}

	filtered, dropped := applyHealthFilter(proxies, p)
	if dropped != 1 || len(filtered) != 1 || nameOf(filtered[0]) != "slow" {
		t.Fatalf("drop_dead should drop only dead: kept=%d dropped=%d", len(filtered), dropped)
	}
}

func TestSortByLatency(t *testing.T) {
	seedHealth(t, []healthcheck.NodeResult{
		{Server: "mid", Port: 443, Alive: true, Latency: 300},
		{Server: "fast", Port: 443, Alive: true, Latency: 50},
		{Server: "dead", Port: 443, Alive: false, Latency: -1},
	})
	proxies := []interface{}{
		proxy("dead", "dead", 443),
		proxy("unknown", "unknown", 443),
		proxy("mid", "mid", 443),
		proxy("fast", "fast", 443),
	}

	sortByLatency(proxies)
	want := []string{"fast", "mid", "unknown", "dead"}
	for i, w := range want {
		if nameOf(proxies[i]) != w {
			t.Fatalf("position %d: want %s got %s (full order: %v %v %v %v)",
				i, w, nameOf(proxies[i]), nameOf(proxies[0]), nameOf(proxies[1]), nameOf(proxies[2]), nameOf(proxies[3]))
		}
	}
}

func TestToInt(t *testing.T) {
	cases := []struct {
		in   interface{}
		want int
	}{
		{443, 443},
		{443.0, 443},
		{"443", 443},
		{nil, 0},
		{true, 0},
	}
	for _, c := range cases {
		if got := toInt(c.in); got != c.want {
			t.Fatalf("toInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func nameOf(p interface{}) string {
	if m, ok := p.(map[string]interface{}); ok {
		if v, ok := m["name"].(string); ok {
			return v
		}
	}
	return ""
}
