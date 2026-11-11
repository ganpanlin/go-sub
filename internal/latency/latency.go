// Package latency performs real url-test style latency measurement:
// it dials THROUGH each proxy node (via the embedded mihomo core) and
// requests a test URL, measuring the end-to-end round trip exactly like
// Clash/Mihomo clients do — not just a TCP handshake to the entry server.
package latency

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	U "github.com/metacubex/mihomo/common/utils"
	"github.com/metacubex/mihomo/log"

	"go-sub/internal/healthcheck"
)

func init() {
	// mihomo logs to stdout on its own; keep the server log clean.
	log.SetLevel(log.SILENT)
}

// DefaultTestURL is the standard zero-body latency probe used by Clash clients.
const DefaultTestURL = "http://www.gstatic.com/generate_204"

// defaultExpectedStatus accepts any 2xx response (some nodes return 200 instead of 204).
const defaultExpectedStatus = "200-299"

// TestNode dials through a single proxy node and measures the HTTP round trip
// to testURL. The returned NodeResult carries TestType "url".
func TestNode(proxy map[string]interface{}, testURL string, timeout time.Duration) healthcheck.NodeResult {
	name := fieldStr(proxy, "name")
	server := fieldStr(proxy, "server")
	port := fieldInt(proxy, "port")

	res := healthcheck.NodeResult{
		Name:     name,
		Server:   server,
		Port:     port,
		Alive:    false,
		Latency:  -1,
		TestType: "url",
	}
	if testURL == "" {
		testURL = DefaultTestURL
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	p, err := adapter.ParseProxy(proxy)
	if err != nil {
		res.Error = fmt.Sprintf("unsupported proxy config: %v", err)
		return res
	}

	expected, err := U.NewUnsignedRanges[uint16](defaultExpectedStatus)
	if err != nil {
		res.Error = fmt.Sprintf("bad expected status: %v", err)
		return res
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	delay, err := p.URLTest(ctx, testURL, expected)
	if err != nil || delay == 0 {
		if err == nil {
			err = fmt.Errorf("no delay measured")
		}
		res.Error = err.Error()
		return res
	}

	res.Alive = true
	res.Latency = int64(delay)
	return res
}

// TestNodes batch-tests proxies through the proxy channel itself.
// maxConcurrent <= 0 defaults to 8 (url tests are heavier than TCP checks).
func TestNodes(proxies []map[string]interface{}, testURL string, timeout time.Duration, maxConcurrent int) []healthcheck.NodeResult {
	return TestNodesProgress(proxies, testURL, timeout, maxConcurrent, nil)
}

// TestNodesProgress is TestNodes with an optional (done, total) progress callback.
func TestNodesProgress(proxies []map[string]interface{}, testURL string, timeout time.Duration, maxConcurrent int, onProgress func(done, total int)) []healthcheck.NodeResult {
	if maxConcurrent <= 0 {
		maxConcurrent = 8
	}
	results := make([]healthcheck.NodeResult, len(proxies))
	var done atomic.Int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, p := range proxies {
		wg.Add(1)
		go func(idx int, proxy map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[idx] = TestNode(proxy, testURL, timeout)
			if onProgress != nil {
				n := done.Add(1)
				onProgress(int(n), len(proxies))
			}
		}(i, p)
	}
	wg.Wait()
	return results
}

func fieldStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func fieldInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case int:
			return val
		case float64:
			return int(val)
		case string:
			var n int
			fmt.Sscanf(val, "%d", &n)
			return n
		}
	}
	return 0
}
