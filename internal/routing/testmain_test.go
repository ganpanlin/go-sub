package routing

import (
	"go-sub/internal/appconfig"
	"os"
	"testing"
)

// TestMain initializes appconfig with a temp data dir so datastore/catalog
// file operations stay isolated from the developer's real data directory.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "go-sub-routing-test")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	appconfig.Init("0", "config.json", dir, "frontend", 10, 60, 10)
	os.Exit(m.Run())
}
