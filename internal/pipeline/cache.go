package pipeline

import (
	"context"
	"fmt"
	"go-sub/internal/appconfig"
	"go-sub/internal/cache"
	"go-sub/internal/healthcheck"
	"go-sub/internal/rule"
	"sync"
	"time"
)

// 单飞：同一 key 的并发 pipeline 请求只跑一次，其余等待共享结果
var (
	inflightMu sync.Mutex
	inflight   = map[string]*inflightCall{}
)

type inflightCall struct {
	wg  sync.WaitGroup
	res *Result
	err error
}

// RunCached wraps RunCtx with a process-level result cache so repeated
// requests (and different output formats) reuse the fetched/filtered/routed
// data instead of rebuilding it every time. Cached entries are keyed by
// profile ID + health-data generation + latency-filter overrides, so:
//   - editing a profile / routing / sources must call Invalidate()
//   - a completed health check bumps the generation automatically
//
// Results are shared read-only: converters and buildOutputConfig never mutate
// the proxies/groups/rules they receive.
func RunCached(ctx context.Context, profile *rule.Profile) (*Result, error) {
	key := ResultCacheKey(profile)
	if v, ok := cache.Get(key); ok {
		if r, ok := v.(*Result); ok {
			return r, nil
		}
	}

	// 单飞：已有同 key 的构建在跑就等待共享，避免链路预热与客户端请求重复拉取
	inflightMu.Lock()
	if c, ok := inflight[key]; ok {
		inflightMu.Unlock()
		c.wg.Wait()
		if c.err != nil {
			return nil, c.err
		}
		return c.res, nil
	}
	c := &inflightCall{}
	c.wg.Add(1)
	inflight[key] = c
	inflightMu.Unlock()

	res, err := RunCtx(ctx, profile)

	c.res, c.err = res, err
	inflightMu.Lock()
	delete(inflight, key)
	inflightMu.Unlock()
	c.wg.Done()

	if err != nil {
		return nil, err
	}
	cache.Set(key, res, time.Duration(appconfig.Get().FilterCacheTTLMinutes)*time.Minute)
	return res, nil
}

// ResultCacheKey identifies a pipeline result: profile ID + health generation
// + the latency-filter overrides (health checks bypass them via a copy, so
// they get a distinct key and never pollute the normal result).
func ResultCacheKey(p *rule.Profile) string {
	return fmt.Sprintf("pp:%s:g%d:l%d:d%d", p.ID, healthcheck.Generation(), p.MaxLatency, boolFlag(p.DropDead))
}

func boolFlag(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Invalidate drops all pipeline results and generated subscription outputs.
// Call after profile / routing / source changes.
func Invalidate() {
	cache.DeleteByPrefix("pp:")
	cache.DeleteByPrefix("sub:")
}
