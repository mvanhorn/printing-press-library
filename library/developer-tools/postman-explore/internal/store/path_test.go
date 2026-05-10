package store

import (
	"path/filepath"
	"testing"
)

func TestResolvePathExplicitWins(t *testing.T) {
	t.Setenv("POSTMAN_EXPLORE_DB", filepath.Join(t.TempDir(), "env.db"))
	explicit := filepath.Join(t.TempDir(), "explicit.db")
	if got := ResolvePath("postman-explore-pp-cli", explicit); got != explicit {
		t.Fatalf("ResolvePath explicit = %q, want %q", got, explicit)
	}
}

func TestDefaultPathHonorsEnv(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("POSTMAN_EXPLORE_DB", envPath)
	if got := DefaultPath("postman-explore-pp-cli"); got != envPath {
		t.Fatalf("DefaultPath = %q, want env override %q", got, envPath)
	}
}
