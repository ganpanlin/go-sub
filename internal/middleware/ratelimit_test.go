package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterStore_Allow(t *testing.T) {
	store := NewLimiterStore(2.0, 2) // 2 req/s, burst 2

	// Should allow first 2 (burst)
	if !store.Allow("1.1.1.1") {
		t.Fatal("first request should be allowed")
	}
	if !store.Allow("1.1.1.1") {
		t.Fatal("second request should be allowed")
	}
	// Third should be rejected (burst exhausted)
	if store.Allow("1.1.1.1") {
		t.Fatal("third request should be rejected")
	}

	// Different key should be independent
	if !store.Allow("2.2.2.2") {
		t.Fatal("different IP should be allowed")
	}
}

func TestLimiterStore_Refill(t *testing.T) {
	store := NewLimiterStore(1000.0, 1) // very high rate, burst 1

	if !store.Allow("1.1.1.1") {
		t.Fatal("first should be allowed")
	}
	if store.Allow("1.1.1.1") {
		t.Fatal("second should be rejected")
	}

	// Wait a bit for refill
	time.Sleep(10 * time.Millisecond)

	if !store.Allow("1.1.1.1") {
		t.Fatal("should be allowed after refill")
	}
}

func TestExtractIP_RemoteAddr(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "1.2.3.4:12345"

	ip := ExtractIP(r)
	if ip != "1.2.3.4" {
		t.Fatalf("expected 1.2.3.4, got %s", ip)
	}
}

func TestExtractIP_XForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "5.6.7.8, 9.10.11.12")

	ip := ExtractIP(r)
	if ip != "5.6.7.8" {
		t.Fatalf("expected 5.6.7.8, got %s", ip)
	}
}

func TestExtractIP_XRealIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Real-IP", "3.4.5.6")

	ip := ExtractIP(r)
	if ip != "3.4.5.6" {
		t.Fatalf("expected 3.4.5.6, got %s", ip)
	}
}

func TestExtractIP_XForwardedForPriority(t *testing.T) {
	// X-Forwarded-For should take priority over X-Real-IP
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	r.Header.Set("X-Real-IP", "2.2.2.2")

	ip := ExtractIP(r)
	if ip != "1.1.1.1" {
		t.Fatalf("expected XFF to take priority, got %s", ip)
	}
}

func TestMiddleware_Blocked(t *testing.T) {
	store := NewLimiterStore(1.0, 1)

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	// First request passes
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.1.1.1:12345"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("first request should pass, got %d", w.Code)
	}

	// Second request blocked
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request should be 429, got %d", w.Code)
	}
}

func TestMiddleware_DifferentIPs(t *testing.T) {
	store := NewLimiterStore(1.0, 1)

	handler := Middleware(store)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Both IPs should get one request each
	for _, ip := range []string{"1.1.1.1:1234", "2.2.2.2:5678"} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request from %s should pass, got %d", ip, w.Code)
		}
	}
}
