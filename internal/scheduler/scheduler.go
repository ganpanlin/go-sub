package scheduler

import (
	"context"
	"encoding/json"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/datastore"
	"go-sub/internal/healthcheck"
	"go-sub/internal/latency"
	"go-sub/internal/pipeline"
	"go-sub/internal/provider"
	"go-sub/internal/rule"
	"go-sub/internal/source"
	"log/slog"
	"os"
	"sync"
	"time"
)

// Config represents the structure of the config.json file for the scheduler.
type Config struct {
	RefreshIntervalMinutes int `json:"refresh_interval_minutes"` // extra standalone refresh cycle; 0 = off
	// HealthCheckIntervalMinutes drives the periodic chain
	// (refresh sources → health check → pre-warm subscription caches).
	// nil (absent) = default 60 minutes; 0 = disabled.
	HealthCheckIntervalMinutes *int   `json:"health_check_interval_minutes"`
	HealthCheckMode            string `json:"health_check_mode"`        // "tcp" (default) | "url" (real proxy round trip)
	HealthCheckTimeoutSec      int    `json:"health_check_timeout_sec"` // per-node timeout, default 3 (tcp) / 5 (url)
}

const defaultHealthCheckIntervalMinutes = 60

// CheckProgress 当前订阅的节点级检测进度
type CheckProgress struct {
	Name  string `json:"name"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// CheckSummary 一个订阅最近一次检测的结果摘要（持久化到 check-summaries.json）
type CheckSummary struct {
	Total int       `json:"total"`
	Alive int       `json:"alive"`
	Mode  string    `json:"mode,omitempty"` // tcp | url
	At    time.Time `json:"at"`
}

// ChainStatus 是调度链路的实时状态，供前端展示（拉取中/检测中/预热中）。
type ChainStatus struct {
	Phase       string                  `json:"phase"`                  // idle | refreshing | checking | prewarming
	Detail      string                  `json:"detail,omitempty"`       // 当前动作描述
	CheckDone   int                     `json:"check_done,omitempty"`   // 已完成检测的订阅数
	CheckTotal  int                     `json:"check_total,omitempty"`  // 待检测订阅总数
	Checking    *CheckProgress          `json:"checking,omitempty"`     // 检测中（节点级进度）
	Pending     []string                `json:"pending,omitempty"`      // 待检测订阅名
	Checked     []string                `json:"checked,omitempty"`      // 本轮已完成订阅名
	LastResults map[string]CheckSummary `json:"last_results,omitempty"` // 每订阅上次检测摘要
	LastRun     time.Time               `json:"last_run,omitempty"`
	LastError   string                  `json:"last_error,omitempty"`
}

var (
	chainMu     sync.Mutex
	chainStatus = ChainStatus{Phase: "idle"}
)

func setChain(phase, detail string) {
	chainMu.Lock()
	chainStatus.Phase = phase
	chainStatus.Detail = detail
	chainMu.Unlock()
}

func setChainCheck(done, total int, name string, nodeDone, nodeTotal int) {
	chainMu.Lock()
	chainStatus.Phase = "checking"
	chainStatus.CheckDone = done
	chainStatus.CheckTotal = total
	if nodeTotal > 0 {
		chainStatus.Checking = &CheckProgress{Name: name, Done: nodeDone, Total: nodeTotal}
	} else {
		chainStatus.Checking = nil // 订阅完成/切换时清除旧进度，避免行内残留"检测中"
	}
	chainMu.Unlock()
}

// setChainQueues 更新待检测/已检测队列
func setChainQueues(pending, checked []string) {
	chainMu.Lock()
	chainStatus.Pending = pending
	chainStatus.Checked = checked
	chainMu.Unlock()
}

// recordCheckSummary 记录并持久化单订阅检测摘要
func recordCheckSummary(name string, total, alive int, mode string) {
	chainMu.Lock()
	if chainStatus.LastResults == nil {
		chainStatus.LastResults = map[string]CheckSummary{}
	}
	chainStatus.LastResults[name] = CheckSummary{Total: total, Alive: alive, Mode: mode, At: time.Now()}
	snapshot := chainStatus.LastResults
	chainMu.Unlock()
	_ = datastore.WriteJSON("check-summaries.json", snapshot)
}

// GetChainStatus returns a snapshot of the scheduler chain state.
func GetChainStatus() ChainStatus {
	chainMu.Lock()
	defer chainMu.Unlock()
	out := chainStatus
	if chainStatus.Phase == "idle" {
		out.Detail = ""
	}
	return out
}

// HealthIntervalOrDefault resolves the health-check cycle interval.
func (c Config) HealthIntervalOrDefault() int {
	if c.HealthCheckIntervalMinutes == nil {
		return defaultHealthCheckIntervalMinutes
	}
	return *c.HealthCheckIntervalMinutes
}

// LoadConfig reads scheduler settings from config.json. Missing file or fields
// fall back to defaults (health-check chain every 60 minutes).
func LoadConfig() Config {
	var config Config
	file, err := os.ReadFile(appconfig.Get().ConfigPath)
	if err != nil {
		return config
	}
	if err := json.Unmarshal(file, &config); err != nil {
		slog.Error("scheduler config parse error", "error", err)
	}
	return config
}

// Start launches the scheduled jobs:
//   - immediately after startup: refresh sources → health check → pre-warm
//   - every HealthCheckIntervalMinutes (default 60): the same chain
//   - optional extra standalone source refresh (refresh_interval_minutes)
func Start() {
	config := LoadConfig()

	// 载入上次的检测摘要（重启后行内仍可展示最近结果）
	var summaries map[string]CheckSummary
	if err := datastore.ReadJSON("check-summaries.json", &summaries); err == nil && len(summaries) > 0 {
		chainMu.Lock()
		chainStatus.LastResults = summaries
		chainMu.Unlock()
	}
	healthInterval := config.HealthIntervalOrDefault()
	// 默认 url 真实测速——TCP 握手延迟对延迟过滤几乎没有区分度
	healthMode := config.HealthCheckMode
	if healthMode != "tcp" {
		healthMode = "url"
	}

	chain := func() {
		chainMu.Lock()
		chainStatus.Phase = "refreshing"
		chainStatus.Detail = "拉取订阅源中"
		chainMu.Unlock()
		refreshAllSourcesSync()

		chainMu.Lock()
		chainStatus.Phase = "checking"
		chainStatus.CheckDone = 0
		chainMu.Unlock()
		runHealthChecks(healthMode, healthCheckTimeout(config))

		chainMu.Lock()
		chainStatus.Phase = "prewarming"
		chainStatus.Detail = "预热订阅缓存中"
		chainMu.Unlock()
		prewarmSubscriptionCaches()

		chainMu.Lock()
		chainStatus.Phase = "idle"
		chainStatus.Detail = ""
		chainStatus.Pending = nil
		chainStatus.Checked = nil
		chainStatus.Checking = nil
		chainStatus.LastRun = time.Now()
		chainMu.Unlock()
	}

	// Startup: fetch subscriptions, then immediately latency-test and pre-warm.
	go chain()

	if healthInterval > 0 {
		ticker := time.NewTicker(time.Duration(healthInterval) * time.Minute)
		slog.Info("scheduled chain enabled (refresh → health check → pre-warm)",
			"interval_minutes", healthInterval, "mode", healthMode)
		go func() {
			for {
				<-ticker.C
				slog.Info("running scheduled chain", "mode", healthMode)
				chain()
			}
		}()
	} else {
		slog.Info("periodic health check disabled by config")
	}

	if config.RefreshIntervalMinutes > 0 {
		ticker := time.NewTicker(time.Duration(config.RefreshIntervalMinutes) * time.Minute)
		slog.Info("standalone scheduled refresh enabled", "interval_minutes", config.RefreshIntervalMinutes)
		go func() {
			for {
				<-ticker.C
				slog.Info("running scheduled refresh")
				refreshAllSourcesSync()
			}
		}()
	}
}

func healthCheckTimeout(config Config) time.Duration {
	if config.HealthCheckTimeoutSec > 0 {
		return time.Duration(config.HealthCheckTimeoutSec) * time.Second
	}
	if config.HealthCheckMode == "url" {
		return 5 * time.Second
	}
	return 3 * time.Second
}

// refreshAllSourcesSync refreshes all enabled sources and waits for completion
// so the health check always runs against freshly pulled subscriptions.
func refreshAllSourcesSync() {
	sources, err := source.LoadAll()
	if err != nil {
		slog.Error("scheduler failed to read sources", "error", err)
		return
	}
	slog.Info("scheduled refresh start", "sources", len(sources))
	start := time.Now()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, src := range sources {
		if !src.Enabled {
			continue
		}
		wg.Add(1)
		go func(s source.Config) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			runtimeURL := source.RuntimeURL(s)
			config, status, lat, isCached, err := provider.FetchAndParseYAML(runtimeURL)
			if err != nil {
				slog.Warn("scheduled refresh failed", "url", runtimeURL, "error", err)
				return
			}
			slog.Info("scheduled refresh done", "url", runtimeURL, "status", status,
				"nodes", provider.ProxyCount(config), "latency_ms", lat, "cached", isCached)
		}(src)
	}
	wg.Wait()
	slog.Info("scheduled refresh complete", "elapsed_ms", time.Since(start).Milliseconds())
}

// runHealthChecks re-checks every enabled profile and persists the latency
// values to data/health-status.json. The profile's own latency filter is
// bypassed so previously filtered nodes get re-tested too (recovery possible).
func runHealthChecks(mode string, timeout time.Duration) {
	mgr := rule.GetManager()
	if mgr == nil {
		return
	}
	profiles := mgr.GetAllProfiles()
	var enabled []*rule.Profile
	for _, p := range profiles {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	total := len(enabled)
	done := 0
	var pendingNames, checkedNames []string
	for _, p := range enabled {
		pendingNames = append(pendingNames, p.Name)
	}
	setChainQueues(pendingNames, checkedNames)
	for _, p := range enabled {
		setChainCheck(done, total, p.Name, 0, 0)
		testProfile := *p
		testProfile.MaxLatency = 0
		testProfile.DropDead = false

		result, err := pipeline.RunCached(context.Background(), &testProfile)
		if err != nil {
			slog.Warn("scheduled health check skipped", "profile", p.Name, "error", err)
			chainMu.Lock()
			chainStatus.LastError = p.Name + ": " + err.Error()
			chainMu.Unlock()
			recordCheckSummary(p.Name, 0, 0, mode)
			done++
			for i, n := range pendingNames {
				if n == p.Name {
					pendingNames = append(pendingNames[:i], pendingNames[i+1:]...)
					break
				}
			}
			continue
		}

		onProgress := func(nodeDone, nodeTotal int) {
			setChainCheck(done, total, p.Name, nodeDone, nodeTotal)
		}
		var results []healthcheck.NodeResult
		if mode == "url" {
			results = latency.TestNodesProgress(result.Proxies, "", timeout, 16, onProgress)
		} else {
			results = healthcheck.CheckNodesProgress(result.Proxies, 100, timeout, onProgress)
		}
		healthcheck.UpdateResults(results)

		alive := 0
		for _, r := range results {
			if r.Alive {
				alive++
			}
		}
		slog.Info("scheduled health check done",
			"profile", p.Name, "total", len(results), "alive", alive, "mode", mode)
		recordCheckSummary(p.Name, len(results), alive, mode)
		done++
		setChainCheck(done, total, p.Name, 0, 0)
		for i, n := range pendingNames {
			if n == p.Name {
				pendingNames = append(pendingNames[:i], pendingNames[i+1:]...)
				break
			}
		}
		checkedNames = append(checkedNames, p.Name)
		setChainQueues(pendingNames, checkedNames)
	}
	// Fresh health data changes filter outcomes — drop cached subscription
	// output so the next request applies the fresh data.
	cache.DeleteByPrefix("sub:")
}

// prewarmSubscriptionCaches rebuilds the pipeline result for every enabled
// profile right after the health check, so client requests hit warm caches
// with the freshest latency filtering already applied.
func prewarmSubscriptionCaches() {
	mgr := rule.GetManager()
	if mgr == nil {
		return
	}
	for _, p := range mgr.GetAllProfiles() {
		if !p.Enabled {
			continue
		}
		chainMu.Lock()
		chainStatus.Phase = "prewarming"
		chainStatus.Detail = "预热订阅缓存：" + p.Name
		chainStatus.Checking = nil
		chainMu.Unlock()
		if _, err := pipeline.RunCached(context.Background(), p); err != nil {
			slog.Warn("pre-warm failed", "profile", p.Name, "error", err)
			continue
		}
		slog.Info("subscription cache pre-warmed", "profile", p.Name)
	}
}
