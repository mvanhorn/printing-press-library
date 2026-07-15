package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyticsDoesNotCreateMissingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	_, _, err := runRootArgs(t, "analytics", "--db", dbPath, "--agent")
	if err == nil {
		t.Fatal("analytics must fail when its read-only database does not exist")
	}
	if _, statErr := os.Stat(dbPath); !os.IsNotExist(statErr) {
		t.Fatalf("analytics created or touched missing database: %v", statErr)
	}
}
