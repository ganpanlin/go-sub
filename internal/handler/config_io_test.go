package handler

import (
	"bytes"
	"encoding/json"
	"go-sub/internal/appconfig"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sharedDir is the data directory shared by all tests in this file.
// appconfig uses sync.Once, so we initialize it once in TestMain.
var sharedDir string

func TestMain(m *testing.M) {
	sharedDir, _ = os.MkdirTemp("", "handler_test_*")
	os.MkdirAll(sharedDir, 0755)
	appconfig.Init("0", filepath.Join(sharedDir, "config.json"), sharedDir, sharedDir, 5, 60, 10)
	code := m.Run()
	os.RemoveAll(sharedDir)
	os.Exit(code)
}

func writeTestFile(t *testing.T, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(sharedDir, name), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writeTestFile: %v", err)
	}
}

func TestExportConfigHandler_AllFiles(t *testing.T) {
	writeTestFile(t, "sources.json", `[{"id":"s1","name":"test","type":"remote_url","url":"https://example.com"}]`)
	writeTestFile(t, "profiles.json", `[{"id":"p1","token":"abc123","name":"my-profile","enabled":true}]`)
	writeTestFile(t, "routing.json", `{"id":"r1","name":"default"}`)
	writeTestFile(t, "rulesets.json", `[]`)

	req := httptest.NewRequest("GET", "/api/config/export", nil)
	w := httptest.NewRecorder()

	ExportConfigHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var export ExportData
	if err := json.Unmarshal(w.Body.Bytes(), &export); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if export.Version != "1.0" {
		t.Fatalf("expected version 1.0, got %s", export.Version)
	}
	if export.Sources == nil {
		t.Fatal("expected sources to be present")
	}
	if export.Profiles == nil {
		t.Fatal("expected profiles to be present")
	}
	if export.Routing == nil {
		t.Fatal("expected routing to be present")
	}
}

func TestExportConfigHandler_EmptyDir(t *testing.T) {
	// Remove all data files
	for _, name := range []string{"sources.json", "profiles.json", "routing.json", "rulesets.json"} {
		os.Remove(filepath.Join(sharedDir, name))
	}

	req := httptest.NewRequest("GET", "/api/config/export", nil)
	w := httptest.NewRecorder()

	ExportConfigHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var export ExportData
	json.Unmarshal(w.Body.Bytes(), &export)
	if export.Sources != nil {
		t.Fatal("expected nil sources when no file exists")
	}
}

func TestImportConfigHandler_Overwrite(t *testing.T) {
	// Write initial data
	writeTestFile(t, "sources.json", `[]`)

	exportData := map[string]interface{}{
		"version":     "1.0",
		"exported_at": "2026-01-01T00:00:00Z",
		"sources":     []interface{}{map[string]interface{}{"id": "new1", "name": "imported"}},
	}
	exportJSON, _ := json.Marshal(exportData)

	body := map[string]interface{}{
		"data": json.RawMessage(exportJSON),
		"mode": "overwrite",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/config/import", bytes.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	ImportConfigHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify the file was written
	data, err := os.ReadFile(filepath.Join(sharedDir, "sources.json"))
	if err != nil {
		t.Fatalf("failed to read sources.json: %v", err)
	}
	var sources []map[string]interface{}
	json.Unmarshal(data, &sources)
	if len(sources) != 1 || sources[0]["id"] != "new1" {
		t.Fatalf("expected imported source, got %s", string(data))
	}
}

func TestImportConfigHandler_InvalidBody(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/config/import", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	ImportConfigHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestImportConfigHandler_InvalidMode(t *testing.T) {
	body := map[string]interface{}{
		"data": json.RawMessage(`{}`),
		"mode": "invalid",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/config/import", bytes.NewReader(bodyJSON))
	w := httptest.NewRecorder()

	ImportConfigHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
