package healthcheck

import (
	"path/filepath"
	"testing"
	"time"
)

func TestUpdateResultsAndGetStatus(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" } // in-memory only
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	UpdateResults([]NodeResult{
		{Name: "fast", Server: "1.2.3.4", Port: 443, Alive: true, Latency: 87},
		{Name: "slow", Server: "5.6.7.8", Port: 8443, Alive: true, Latency: 950},
		{Name: "dead", Server: "dead.example.com", Port: 80, Alive: false, Latency: -1, Error: "timeout"},
	})

	if e, ok := GetStatus("1.2.3.4", 443); !ok || !e.Alive || e.Latency != 87 {
		t.Fatalf("expected fast entry, got %+v found=%v", e, ok)
	}
	// Key is case-insensitive on server
	if _, ok := GetStatus("DEAD.EXAMPLE.COM", 80); !ok {
		t.Fatal("expected case-insensitive server lookup")
	}
	if e, ok := GetStatus("dead.example.com", 80); !ok || e.Alive || e.Latency != -1 {
		t.Fatalf("expected dead entry, got %+v found=%v", e, ok)
	}
	if _, ok := GetStatus("9.9.9.9", 443); ok {
		t.Fatal("unknown endpoint should not be found")
	}
	// CheckedAt should be set to now
	if e, _ := GetStatus("1.2.3.4", 443); time.Now().Unix()-e.CheckedAt > 5 {
		t.Fatalf("CheckedAt not recent: %d", e.CheckedAt)
	}
}

func TestStorePersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "health-status.json")

	origPath := statusFilePath
	statusFilePath = func() string { return path }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	UpdateResults([]NodeResult{
		{Name: "n1", Server: "1.1.1.1", Port: 443, Alive: true, Latency: 50},
	})

	// LoadFromPath simulates a fresh process reading the persisted file.
	if err := LoadFromPath(path); err != nil {
		t.Fatalf("LoadFromPath: %v", err)
	}
	e, ok := GetStatus("1.1.1.1", 443)
	if !ok || e.Latency != 50 || !e.Alive || e.Name != "n1" {
		t.Fatalf("persistence round-trip failed: %+v found=%v", e, ok)
	}
}

func TestStoreDisabledPersistence(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	// Must not panic and must still record in memory.
	UpdateResults([]NodeResult{{Name: "x", Server: "2.2.2.2", Port: 80, Alive: true, Latency: 10}})
	if _, ok := GetStatus("2.2.2.2", 80); !ok {
		t.Fatal("expected in-memory entry when persistence disabled")
	}
}

func TestUpdateResultsEmpty(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	UpdateResults(nil)
	UpdateResults([]NodeResult{})
	if _, ok := GetStatus("1.1.1.1", 443); ok {
		t.Fatal("store should stay empty")
	}
}

func TestStoreTestTypeOverwrite(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	// TCP check first
	UpdateResults([]NodeResult{{Name: "n", Server: "3.3.3.3", Port: 443, Alive: true, Latency: 40}})
	if e, _ := GetStatus("3.3.3.3", 443); e.TestType != "tcp" {
		t.Fatalf("default test type should be tcp, got %q", e.TestType)
	}

	// URL test overwrites with its type
	UpdateResults([]NodeResult{{Name: "n", Server: "3.3.3.3", Port: 443, Alive: true, Latency: 350, TestType: "url"}})
	e, ok := GetStatus("3.3.3.3", 443)
	if !ok || e.TestType != "url" || e.Latency != 350 {
		t.Fatalf("url result should overwrite: %+v found=%v", e, ok)
	}
}
