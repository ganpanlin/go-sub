package pipeline

import (
	"context"
	"encoding/base64"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/healthcheck"
	"go-sub/internal/rule"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain isolates datastore/healthcheck file IO in a temp dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-sub-pipeline-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	appconfig.Init("0", filepath.Join(dir, "config.json"), dir, "frontend", 10, 60, 10)
	os.Exit(m.Run())
}

// yamlSource builds a data: subscription URL with n distinct ss nodes.
func yamlSource(n int) string {
	var b strings.Builder
	b.WriteString("proxies:\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  - name: node-%d\n    type: ss\n    server: 10.%d.%d.%d\n    port: 443\n    cipher: aes-128-gcm\n    password: pw\n",
			i, i>>16&255, i>>8&255, i&255)
	}
	return "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte(b.String()))
}

func TestRunCachedReusesAndRefreshes(t *testing.T) {
	healthcheck.ResetStoreForTesting()
	profile := &rule.Profile{ID: "cachetest", Name: "t", Sources: []string{yamlSource(3)}}

	r1, err := RunCached(context.Background(), profile)
	if err != nil {
		t.Fatalf("RunCached: %v", err)
	}
	if r1.TotalAfter != 3 {
		t.Fatalf("expected 3 nodes, got %d", r1.TotalAfter)
	}

	r2, err := RunCached(context.Background(), profile)
	if err != nil {
		t.Fatalf("RunCached 2: %v", err)
	}
	if r1 != r2 {
		t.Fatal("second call must return the same cached *Result pointer")
	}

	// A health data update must bump the generation and invalidate the cache.
	before := healthcheck.Generation()
	healthcheck.UpdateResults([]healthcheck.NodeResult{
		{Server: "10.0.0.0", Port: 443, Alive: true, Latency: 42},
	})
	if healthcheck.Generation() != before+1 {
		t.Fatalf("generation should bump by 1, got %d -> %d", before, healthcheck.Generation())
	}
	r3, err := RunCached(context.Background(), profile)
	if err != nil {
		t.Fatalf("RunCached 3: %v", err)
	}
	if r3 == r2 {
		t.Fatal("health update must invalidate the cached pipeline result")
	}
}

func TestResultCacheKeyComponents(t *testing.T) {
	p := &rule.Profile{ID: "k1", MaxLatency: 0, DropDead: false}
	k1 := ResultCacheKey(p)
	p2 := &rule.Profile{ID: "k1", MaxLatency: 500, DropDead: true}
	if ResultCacheKey(p2) == k1 {
		t.Fatal("filter overrides must change the cache key")
	}
	p3 := &rule.Profile{ID: "k2", MaxLatency: 0, DropDead: false}
	if ResultCacheKey(p3) == k1 {
		t.Fatal("profile ID must be part of the cache key")
	}
}

func BenchmarkPipelineColdVsCached(b *testing.B) {
	healthcheck.ResetStoreForTesting()

	b.Run("cold", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Unique data URL per iteration: provider/parse work every time.
			p := &rule.Profile{ID: fmt.Sprintf("bench-cold-%d", i), Sources: []string{yamlSource(2000)}}
			if _, err := Run(p); err != nil {
				b.Fatal(err)
			}
		}
	})

	p := &rule.Profile{ID: "bench-hot", Sources: []string{yamlSource(2000)}}
	if _, err := RunCached(context.Background(), p); err != nil {
		b.Fatal(err)
	}
	b.Run("cached", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := RunCached(context.Background(), p); err != nil {
				b.Fatal(err)
			}
		}
	})
}
