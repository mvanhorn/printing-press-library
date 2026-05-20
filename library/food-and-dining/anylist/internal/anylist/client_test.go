package anylist

import (
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
)

func TestEnsureClientIdentifierPersistsGeneratedIdentifier(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), "config.toml")
	cfg := &config.Config{Path: configPath}

	if err := EnsureClientIdentifier(cfg); err != nil {
		t.Fatalf("EnsureClientIdentifier returned error: %v", err)
	}
	if cfg.ClientIdentifier == "" {
		t.Fatal("ClientIdentifier was not set")
	}

	reloaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if reloaded.ClientIdentifier != cfg.ClientIdentifier {
		t.Fatalf("persisted ClientIdentifier = %q, want %q", reloaded.ClientIdentifier, cfg.ClientIdentifier)
	}
}
