package auth

import (
	"crypto/rand"
	"encoding/hex"
	"go-sub/internal/datastore"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Config struct {
	Enabled   bool   `json:"enabled"`
	Username  string `json:"username"`
	PasswordHash string `json:"password_hash"`
	JWTSecret string `json:"jwt_secret"`
}

var (
	cfg Config
	mu  sync.RWMutex
)

func Init() {
	var loaded Config
	if err := datastore.ReadJSON("auth.json", &loaded); err == nil {
		mu.Lock()
		cfg = loaded
		mu.Unlock()
	}
	mu.Lock()
	if cfg.Username == "" {
		cfg.Username = "admin"
	}
	needSave := false
	if cfg.JWTSecret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		cfg.JWTSecret = hex.EncodeToString(b)
		needSave = true
	}
	mu.Unlock()
	if needSave {
		_ = SaveConfigSafe(cfg)
	}
}

func GetConfig() Config {
	mu.RLock()
	defer mu.RUnlock()
	return cfg
}
func Enabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return cfg.Enabled
}
func NeedsSetup() bool {
	mu.RLock()
	defer mu.RUnlock()
	return !cfg.Enabled || cfg.PasswordHash == ""
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(b), err
}

func CheckPassword(password string) bool {
	mu.RLock()
	hash := cfg.PasswordHash
	mu.RUnlock()
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateJWT generates a new JWT token valid for 24 hours.
func CreateJWT() (string, error) {
	mu.RLock()
	secret := cfg.JWTSecret
	mu.RUnlock()

	claims := jwt.MapClaims{
		"sub": "admin",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// extractToken pulls the JWT from Authorization header or pf_token cookie.
func extractToken(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	if c, err := r.Cookie("pf_token"); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

// ValidateJWT checks a token string and returns true if valid.
func ValidateJWT(tokenStr string) bool {
	if tokenStr == "" {
		return false
	}
	mu.RLock()
	secret := cfg.JWTSecret
	mu.RUnlock()

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	return err == nil && token.Valid
}

func IsAuthenticated(r *http.Request) bool {
	if !cfg.Enabled {
		return true
	}
	token := extractToken(r)
	if token == "" {
		return false
	}
	valid := ValidateJWT(token)
	if !valid {
		slog.Warn("JWT validation failed", "path", r.URL.Path, "token_prefix", token[:20])
	}
	return valid
}

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cfg.Enabled || r.URL.Path == "/api/health" || r.URL.Path == "/api/version" || r.URL.Path == "/api/auth/status" || r.URL.Path == "/api/auth/login" || r.URL.Path == "/api/auth/logout" || r.URL.Path == "/api/auth/setup" || r.URL.Path == "/api/auth/change-password" {
			next.ServeHTTP(w, r)
			return
		}
		if !IsAuthenticated(r) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
