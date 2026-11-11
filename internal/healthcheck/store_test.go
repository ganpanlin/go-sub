package healthcheck

import (
	"fmt"
	"os"
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

	// Persistence is asynchronous — wait for the file to appear.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("health-status.json was not written in time")
		}
		time.Sleep(10 * time.Millisecond)
	}

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

func TestSnapshotAndGeneration(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	before := Generation()
	UpdateResults([]NodeResult{{Name: "s", Server: "5.5.5.5", Port: 80, Alive: true, Latency: 10}})
	if Generation() != before+1 {
		t.Fatalf("generation should bump by 1, got %d -> %d", before, Generation())
	}

	snap := Snapshot()
	if e, ok := snap[Key("5.5.5.5", 80)]; !ok || !e.Alive || e.Latency != 10 {
		t.Fatalf("snapshot missing entry: %+v found=%v", e, ok)
	}
	if _, ok := snap[Key("5.5.5.5", 81)]; ok {
		t.Fatal("snapshot should not contain unrelated endpoints")
	}
}

func TestKeyNormalization(t *testing.T) {
	if Key("ABC.com", 443) != "abc.com|443" {
		t.Fatalf("Key should lowercase server: %q", Key("ABC.com", 443))
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	origPath := statusFilePath
	statusFilePath = func() string { return "" }
	defer func() { statusFilePath = origPath }()
	ResetStoreForTesting()

	done := make(chan struct{})
	for w := 0; w < 4; w++ {
		go func(w int) {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 200; i++ {
				UpdateResults([]NodeResult{{Server: fmt.Sprintf("10.0.%d.%d", w, i), Port: 443, Alive: true, Latency: int64(i)}})
			}
		}(w)
	}
	for r := 0; r < 4; r++ {
		go func() {
			defer func() { done <- struct{}{} }()
			for i := 0; i < 400; i++ {
				_ = Snapshot()
				_, _ = GetStatus("10.0.0.1", 443)
			}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}
	if len(Snapshot()) < 800 {
		t.Fatalf("expected at least 800 entries after concurrent writes, got %d", len(Snapshot()))
	}
}
