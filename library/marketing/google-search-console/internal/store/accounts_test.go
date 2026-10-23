package store

import (
	"context"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-search-console/internal/config"
	"testing"
)

func TestAccountStoreIsolation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	var paths []string
	for _, name := range []string{"bestself", "littlemight"} {
		s, err := Open(config.WithAccount(context.Background(), name), "")
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, s.Path())
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if paths[0] == paths[1] || paths[0] == DefaultDBPath() {
		t.Fatal("accounts share a store")
	}
}

func TestLegacyStoreFallback(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")
	if config.DataPath(context.Background()) != DefaultDBPath() {
		t.Fatal("legacy fallback changed")
	}
}
