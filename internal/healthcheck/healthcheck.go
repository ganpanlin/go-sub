package healthcheck

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// NodeResult holds the health check result for a single node.
type NodeResult struct {
	Name    string `json:"name"`
	Server  string `json:"server"`
	Port    int    `json:"port"`
	Alive   bool   `json:"alive"`
	Latency int64  `json:"latency"` // ms, -1 = timeout/error
	Error   string `json:"error,omitempty"`
}

// CheckNode performs a TCP connect test against server:port.
// Returns a result indicating whether the port is reachable and the connect latency.
func CheckNode(name, server string, port int, timeout time.Duration) NodeResult {
	if port <= 0 || port > 65535 {
		return NodeResult{
			Name:   name,
			Server: server,
			Port:   port,
			Alive:  false,
			Latency: -1,
			Error:  fmt.Sprintf("invalid port: %d", port),
		}
	}
	if server == "" {
		return NodeResult{
			Name:   name,
			Server: server,
			Port:   port,
			Alive:  false,
			Latency: -1,
			Error:  "empty server",
		}
	}

	addr := net.JoinHostPort(server, fmt.Sprintf("%d", port))
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return NodeResult{
			Name:    name,
			Server:  server,
			Port:    port,
			Alive:   false,
			Latency: -1,
			Error:   err.Error(),
		}
	}
	conn.Close()
	return NodeResult{
		Name:    name,
		Server:  server,
		Port:    port,
		Alive:   true,
		Latency: latency,
	}
}

// CheckNodes performs batch health checks on proxy nodes.
// Each proxy map must have "name", "server", and "port" fields.
// maxConcurrent controls parallelism (0 = unlimited).
// timeout is per-node TCP connect timeout.
func CheckNodes(proxies []map[string]interface{}, maxConcurrent int, timeout time.Duration) []NodeResult {
	if maxConcurrent <= 0 {
		maxConcurrent = 50
	}
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	results := make([]NodeResult, len(proxies))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrent)

	for i, p := range proxies {
		wg.Add(1)
		go func(idx int, proxy map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name := fieldStr(proxy, "name")
			server := fieldStr(proxy, "server")
			port := fieldInt(proxy, "port")

			results[idx] = CheckNode(name, server, port, timeout)
		}(i, p)
	}
	wg.Wait()
	return results
}

// fieldStr safely extracts a string field from a proxy map.
func fieldStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

// fieldInt safely extracts an int field from a proxy map.
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
