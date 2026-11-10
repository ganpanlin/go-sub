package routing

import (
	"strings"
	"testing"
)

func TestEnsureCatalogDefault(t *testing.T) {
	// TestMain pointed the data dir at a temp folder: the first call should
	// write the built-in catalog, the second should be a no-op.
	EnsureCatalogDefault()
	EnsureCatalogDefault()

	cats := GetCatalog()
	if len(cats) == 0 {
		t.Fatal("catalog should not be empty after EnsureCatalogDefault")
	}

	ids := map[string]bool{}
	for _, cat := range cats {
		if cat.ID == "" || cat.Name == "" {
			t.Fatalf("category missing identity: %+v", cat)
		}
		if ids[cat.ID] {
			t.Fatalf("duplicate category id: %s", cat.ID)
		}
		ids[cat.ID] = true
		if len(cat.Rules) == 0 {
			t.Fatalf("category %s has no rules", cat.ID)
		}
		seen := map[string]bool{}
		for _, preset := range cat.Rules {
			if preset.ID == "" || preset.Name == "" {
				t.Fatalf("preset missing identity in %s: %+v", cat.ID, preset)
			}
			if seen[preset.ID] {
				t.Fatalf("duplicate preset id %s in category %s", preset.ID, cat.ID)
			}
			seen[preset.ID] = true
			// Each preset must have a usable source: remote URL(s) or inline payload.
			urls := presetURLs(preset)
			if len(urls) == 0 && len(preset.Payload) == 0 {
				t.Fatalf("preset %s/%s has neither url nor payload", cat.ID, preset.ID)
			}
			for _, u := range urls {
				if !strings.HasPrefix(u, "https://") {
					t.Fatalf("preset %s/%s has non-https url %q", cat.ID, preset.ID, u)
				}
			}
		}
	}

	if len(cats) < 12 {
		t.Fatalf("expected at least 12 built-in categories, got %d", len(cats))
	}
}

func TestSaveCatalogRoundTrip(t *testing.T) {
	cats := GetCatalog()
	if len(cats) == 0 {
		t.Fatal("catalog empty before round-trip test")
	}

	// Mutate a copy, save, reload from disk.
	modified := append([]RuleCategory{}, cats...)
	modified[0].Name = "修改后的分类"
	if err := SaveCatalog(modified); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	// In-memory cache must reflect the save...
	if GetCatalog()[0].Name != "修改后的分类" {
		t.Fatal("in-memory catalog not updated after save")
	}

	// ...and the on-disk file must contain it (fresh read).
	// Reset the cache by requesting through a new read of the file.
	catalogData = nil
	if GetCatalog()[0].Name != "修改后的分类" {
		t.Fatal("catalog file does not contain saved change")
	}

	// Restore the built-in default for other tests.
	if err := SaveCatalog(cats); err != nil {
		t.Fatalf("restore catalog: %v", err)
	}
	catalogData = nil
}
