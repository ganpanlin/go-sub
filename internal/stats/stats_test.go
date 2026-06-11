package stats

import (
	"strings"
	"sync"
	"testing"
)

func TestRecordAndGetStats(t *testing.T) {
	tr := &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 10,
	}

	// Initially empty
	s := tr.GetStats("p1")
	if s.AccessCount != 0 {
		t.Fatalf("expected 0, got %d", s.AccessCount)
	}

	// Record first access
	tr.Record("p1", "1.2.3.4", "Clash/1.0", "clash", 10)
	s = tr.GetStats("p1")
	if s.AccessCount != 1 {
		t.Fatalf("expected 1, got %d", s.AccessCount)
	}
	if s.LastAccess == nil {
		t.Fatal("expected last_access to be set")
	}
	if len(s.RecentLogs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(s.RecentLogs))
	}
	if s.RecentLogs[0].IP != "1.2.3.4" {
		t.Fatalf("expected IP 1.2.3.4, got %s", s.RecentLogs[0].IP)
	}
	if s.RecentLogs[0].ClientType != "clash" {
		t.Fatalf("expected client type clash, got %s", s.RecentLogs[0].ClientType)
	}

	// Record second access
	tr.Record("p1", "5.6.7.8", "Surge/5.0", "surge", 15)
	s = tr.GetStats("p1")
	if s.AccessCount != 2 {
		t.Fatalf("expected 2, got %d", s.AccessCount)
	}
	if len(s.RecentLogs) != 2 {
		t.Fatalf("expected 2 logs, got %d", len(s.RecentLogs))
	}
}

func TestRecord_RingBuffer(t *testing.T) {
	tr := &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 3,
	}

	// Record more than maxLogs
	for i := 0; i < 5; i++ {
		tr.Record("p1", "ip", "ua", "clash", i)
	}

	s := tr.GetStats("p1")
	if s.AccessCount != 5 {
		t.Fatalf("expected total count 5, got %d", s.AccessCount)
	}
	if len(s.RecentLogs) != 3 {
		t.Fatalf("expected 3 logs (ring buffer), got %d", len(s.RecentLogs))
	}
	// Should keep the last 3
	if s.RecentLogs[0].NodeCount != 2 {
		t.Fatalf("expected first kept log to have NodeCount 2, got %d", s.RecentLogs[0].NodeCount)
	}
	if s.RecentLogs[2].NodeCount != 4 {
		t.Fatalf("expected last kept log to have NodeCount 4, got %d", s.RecentLogs[2].NodeCount)
	}
}

func TestRecord_MultipleProfiles(t *testing.T) {
	tr := &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 10,
	}

	tr.Record("p1", "1.1.1.1", "ua", "clash", 10)
	tr.Record("p2", "2.2.2.2", "ua", "surge", 20)
	tr.Record("p1", "3.3.3.3", "ua", "clash", 15)

	all := tr.GetAllStats()
	if len(all) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(all))
	}
	if all["p1"].AccessCount != 2 {
		t.Fatalf("expected p1 count 2, got %d", all["p1"].AccessCount)
	}
	if all["p2"].AccessCount != 1 {
		t.Fatalf("expected p2 count 1, got %d", all["p2"].AccessCount)
	}
}

func TestRecord_ConcurrentSafety(t *testing.T) {
	tr := &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 100,
	}

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tr.Record("p1", "ip", "ua", "clash", 1)
		}()
	}
	wg.Wait()

	s := tr.GetStats("p1")
	if s.AccessCount != 100 {
		t.Fatalf("expected 100 concurrent records, got %d", s.AccessCount)
	}
}

func TestGetStats_NonExistent(t *testing.T) {
	tr := &Tracker{
		counts: make(map[string]*ProfileStats),
	}
	s := tr.GetStats("nonexistent")
	if s.AccessCount != 0 {
		t.Fatalf("expected 0 for nonexistent profile, got %d", s.AccessCount)
	}
	if s.LastAccess != nil {
		t.Fatal("expected nil LastAccess for nonexistent profile")
	}
}

func TestGetAllStats_Empty(t *testing.T) {
	tr := &Tracker{
		counts: make(map[string]*ProfileStats),
	}
	all := tr.GetAllStats()
	if len(all) != 0 {
		t.Fatalf("expected empty map, got %d entries", len(all))
	}
}

func TestGetTracker_Singleton(t *testing.T) {
	t1 := GetTracker()
	t2 := GetTracker()
	if t1 != t2 {
		t.Fatal("GetTracker should return the same instance")
	}
}

func TestRecord_ClientTypes(t *testing.T) {
	tr := &Tracker{
		counts:  make(map[string]*ProfileStats),
		maxLogs: 10,
	}

	types := []string{"clash", "surge", "loon", "singbox", "quantumult", "base64"}
	for _, ct := range types {
		tr.Record("p1", "1.1.1.1", "ua", ct, 10)
	}

	s := tr.GetStats("p1")
	if s.AccessCount != len(types) {
		t.Fatalf("expected %d, got %d", len(types), s.AccessCount)
	}

	// Verify all types are recorded
	for i, ct := range types {
		if s.RecentLogs[i].ClientType != ct {
			t.Errorf("log[%d]: expected %s, got %s", i, ct, s.RecentLogs[i].ClientType)
		}
	}
}

// Make sure strings import is used
var _ = strings.Contains
