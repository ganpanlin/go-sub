package auth

import (
	"go-sub/internal/appconfig"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestMain initializes appconfig with a temp data dir so auth.json writes are isolated.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-sub-auth-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	appconfig.Init("0", "config.json", dir, "frontend", 10, 60, 10)
	os.Exit(m.Run())
}

func TestInitDefaults(t *testing.T) {
	Init()
	c := GetConfig()
	if c.Username != "admin" {
		t.Fatalf("expected default username admin, got %q", c.Username)
	}
	if c.JWTSecret == "" {
		t.Fatal("JWT secret should be generated on first init")
	}
	if !NeedsSetup() {
		t.Fatal("fresh instance should need setup")
	}
}

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("s3cret-密码")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" || strings.Contains(hash, "s3cret") {
		t.Fatal("hash should not be empty or contain plaintext")
	}

	// Save and verify through the config path
	if err := SaveConfigSafe(Config{Enabled: true, Username: "admin", PasswordHash: hash, JWTSecret: GetConfig().JWTSecret}); err != nil {
		t.Fatalf("SaveConfigSafe: %v", err)
	}
	if !CheckPassword("s3cret-密码") {
		t.Fatal("correct password should validate")
	}
	if CheckPassword("wrong") {
		t.Fatal("wrong password should not validate")
	}
	if CheckPassword("") {
		t.Fatal("empty password should not validate")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	token, err := CreateJWT()
	if err != nil {
		t.Fatalf("CreateJWT: %v", err)
	}
	if !ValidateJWT(token) {
		t.Fatal("freshly created token should validate")
	}
	if ValidateJWT("") {
		t.Fatal("empty token should not validate")
	}
	if ValidateJWT(token + "x") {
		t.Fatal("tampered token should not validate")
	}
	if ValidateJWT("not-a-jwt") {
		t.Fatal("garbage token should not validate")
	}
}

func TestIsAuthenticated(t *testing.T) {
	token, _ := CreateJWT()

	// Disabled auth lets everything through
	mu.Lock()
	cfg.Enabled = false
	mu.Unlock()
	if !IsAuthenticated(httptest.NewRequest("GET", "/api/profiles", nil)) {
		t.Fatal("disabled auth should allow all requests")
	}

	mu.Lock()
	cfg.Enabled = true
	mu.Unlock()

	// Valid bearer token
	r := httptest.NewRequest("GET", "/api/profiles", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	if !IsAuthenticated(r) {
		t.Fatal("valid bearer token should authenticate")
	}

	// Valid cookie token
	r2 := httptest.NewRequest("GET", "/api/profiles", nil)
	r2.AddCookie(&http.Cookie{Name: "pf_token", Value: token})
	if !IsAuthenticated(r2) {
		t.Fatal("valid cookie token should authenticate")
	}

	// Missing token
	if IsAuthenticated(httptest.NewRequest("GET", "/api/profiles", nil)) {
		t.Fatal("missing token should not authenticate")
	}

	// Regression: a very short invalid token must be rejected without panicking
	// (the old code sliced token[:20] unconditionally).
	r3 := httptest.NewRequest("GET", "/api/profiles", nil)
	r3.Header.Set("Authorization", "Bearer x")
	if IsAuthenticated(r3) {
		t.Fatal("short invalid token should not authenticate")
	}
}

func TestMiddleware(t *testing.T) {
	mu.Lock()
	cfg.Enabled = true
	mu.Unlock()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	// Whitelisted path passes without a token
	rec := httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, httptest.NewRequest("GET", "/api/health", nil))
	if !called {
		t.Fatal("whitelisted path should reach the handler")
	}

	// Missing token → 401
	called = false
	rec = httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, httptest.NewRequest("GET", "/api/profiles", nil))
	if called || rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got code=%d called=%v", rec.Code, called)
	}

	// Valid token → passes through
	token, _ := CreateJWT()
	req := httptest.NewRequest("GET", "/api/profiles", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	called = false
	rec = httptest.NewRecorder()
	Middleware(next).ServeHTTP(rec, req)
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("valid token should pass middleware, got code=%d called=%v", rec.Code, called)
	}
}

func TestValidateJWTRejectsNoneAlgorithm(t *testing.T) {
	// alg=none tokens must be rejected (parser is pinned to HMAC).
	malformed := "eyJhbGciOiJub25lIiwidHlwIjoiSldUIn0.eyJzdWIiOiJhZG1pbiJ9."
	if ValidateJWT(malformed) {
		t.Fatal("alg=none token must be rejected")
	}
}
