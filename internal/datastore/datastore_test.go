package datastore

import (
	"fmt"
	"go-sub/internal/appconfig"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestMain initializes appconfig with an isolated temp data dir.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-sub-datastore-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	appconfig.Init("0", "config.json", dir, "frontend", 10, 60, 10)
	os.Exit(m.Run())
}

type sample struct {
	Name  string   `json:"name"`
	Items []string `json:"items"`
}

func TestWriteReadJSONRoundTrip(t *testing.T) {
	in := sample{Name: "测试", Items: []string{"a", "b"}}
	if err := WriteJSON("roundtrip.json", in); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}

	var out sample
	if err := ReadJSON("roundtrip.json", &out); err != nil {
		t.Fatalf("ReadJSON: %v", err)
	}
	if out.Name != in.Name || len(out.Items) != 2 {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
	if !Exists("roundtrip.json") {
		t.Fatal("Exists should be true after write")
	}
	if Exists("missing.json") {
		t.Fatal("Exists should be false for missing file")
	}
}

func TestReadJSONMissingFile(t *testing.T) {
	var out sample
	if err := ReadJSON("definitely-missing.json", &out); err == nil {
		t.Fatal("expected error reading missing file")
	}
}

func TestSaveOverwrites(t *testing.T) {
	if err := Save("overwrite.json", sample{Name: "v1"}); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if err := Save("overwrite.json", sample{Name: "v2"}); err != nil {
		t.Fatalf("second save: %v", err)
	}
	var out sample
	if err := ReadJSON("overwrite.json", &out); err != nil {
		t.Fatalf("read: %v", err)
	}
	if out.Name != "v2" {
		t.Fatalf("expected v2, got %q", out.Name)
	}
}

func TestSaveConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := Save("concurrent.json", sample{Name: fmt.Sprintf("w%d", i)}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent save failed: %v", err)
	}
	if !Exists("concurrent.json") {
		t.Fatal("file should exist after concurrent saves")
	}
}

func TestInitDefaults(t *testing.T) {
	defaultDir := t.TempDir()
	// A default template that should be copied over
	if err := os.WriteFile(filepath.Join(defaultDir, "profiles.json"), []byte(`[{"id":"seed"}]`), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	InitDefaults(defaultDir)

	var profiles []map[string]interface{}
	if err := ReadJSON("profiles.json", &profiles); err != nil {
		t.Fatalf("default not copied: %v", err)
	}
	if len(profiles) != 1 || profiles[0]["id"] != "seed" {
		t.Fatalf("unexpected default content: %+v", profiles)
	}

	// Second run must not overwrite existing data
	if err := Save("profiles.json", []map[string]interface{}{{"id": "user-data"}}); err != nil {
		t.Fatalf("save user data: %v", err)
	}
	InitDefaults(defaultDir)
	if err := ReadJSON("profiles.json", &profiles); err != nil {
		t.Fatalf("read after re-init: %v", err)
	}
	if len(profiles) != 1 || profiles[0]["id"] != "user-data" {
		t.Fatalf("InitDefaults overwrote existing data: %+v", profiles)
	}
}

func TestDataFilePath(t *testing.T) {
	p := DataFilePath("x.json")
	if p == "" || filepath.Base(p) != "x.json" {
		t.Fatalf("unexpected DataFilePath: %q", p)
	}
}
