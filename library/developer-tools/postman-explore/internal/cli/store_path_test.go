package cli

import (
	"path/filepath"
	"testing"
)

func TestLocalStorePathExplicitWinsOverEnv(t *testing.T) {
	t.Setenv("POSTMAN_EXPLORE_DB", filepath.Join(t.TempDir(), "env.db"))
	explicit := filepath.Join(t.TempDir(), "explicit.db")
	if got := localStorePath(&rootFlags{dbPath: explicit}); got != explicit {
		t.Fatalf("localStorePath = %q, want %q", got, explicit)
	}
}

func TestLocalStorePathUsesEnvWithoutExplicitFlag(t *testing.T) {
	envPath := filepath.Join(t.TempDir(), "env.db")
	t.Setenv("POSTMAN_EXPLORE_DB", envPath)
	if got := localStorePath(&rootFlags{}); got != envPath {
		t.Fatalf("localStorePath = %q, want %q", got, envPath)
	}
}
