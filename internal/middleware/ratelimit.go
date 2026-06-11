package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// LimiterStore stores per-key rate limiters with automatic cleanup.
type LimiterStore struct {
	mu       sync.Mutex
	limiters map[string]*limiterEntry
	rate     rate.Limit // tokens per second
	burst    int
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewLimiterStore creates a new rate limiter store.
// r = tokens per second, burst = maximum burst size.
func NewLimiterStore(r float64, burst int) *LimiterStore {
	store := &LimiterStore{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(r),
		burst:    burst,
	}
	// Background cleanup of stale entries every 5 minutes
	go store.cleanup()
	return store
}

// Allow checks if a request from the given key is allowed.
func (s *LimiterStore) Allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.limiters[key]
	if !ok {
		entry = &limiterEntry{
			limiter: rate.NewLimiter(s.rate, s.burst),
		}
		s.limiters[key] = entry
	}
	entry.lastSeen = time.Now()
	return entry.limiter.Allow()
}

// cleanup removes entries not seen in the last 10 minutes.
func (s *LimiterStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		s.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for key, entry := range s.limiters {
			if entry.lastSeen.Before(cutoff) {
				delete(s.limiters, key)
			}
		}
		s.mu.Unlock()
	}
}

// ExtractIP extracts the client IP from a request, stripping port.
// Respects X-Forwarded-For and X-Real-IP headers if present.
func ExtractIP(r *http.Request) string {
	// Check X-Forwarded-For (first IP in the list)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For: client, proxy1, proxy2
		// We want the first (leftmost) IP
		ips := strings.Split(xff, ",")
		ip := strings.TrimSpace(ips[0])
		host, _, err := net.SplitHostPort(ip)
		if err == nil {
			return host
		}
		return ip
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback to RemoteAddr
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Middleware returns an HTTP middleware that rate-limits requests by client IP.
func Middleware(store *LimiterStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := ExtractIP(r)
			if !store.Allow(ip) {
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
