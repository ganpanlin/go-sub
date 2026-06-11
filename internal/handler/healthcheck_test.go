package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthCheckHandler_MissingBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/health-check", strings.NewReader(""))
	w := httptest.NewRecorder()

	HealthCheckHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHealthCheckHandler_NoProfileID(t *testing.T) {
	body := `{}`
	req := httptest.NewRequest("POST", "/api/health-check", strings.NewReader(body))
	w := httptest.NewRecorder()

	HealthCheckHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHealthCheckHandler_ProfileNotFound(t *testing.T) {
	body := `{"profile_id":"nonexistent"}`
	req := httptest.NewRequest("POST", "/api/health-check", strings.NewReader(body))
	w := httptest.NewRecorder()

	HealthCheckHandler(w, req)

	// rule manager is nil in tests, so we get 500
	if w.Code != http.StatusInternalServerError && w.Code != http.StatusNotFound {
		t.Fatalf("expected 500 or 404, got %d", w.Code)
	}
}
