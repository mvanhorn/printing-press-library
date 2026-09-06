package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFirstExistingReturnsOnlyExistingPath(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "homeclaw-cli")
	if err := os.WriteFile(existing, []byte("stub"), 0o700); err != nil {
		t.Fatal(err)
	}
	if got := firstExisting([]string{filepath.Join(dir, "missing"), existing}); got != existing {
		t.Fatalf("firstExisting() = %q, want %q", got, existing)
	}
}

func TestWrapperHasNoInstallOrPrivilegeCommand(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"sudo", "ln -s", "openclaw gateway"} {
		if contains(string(source), forbidden) {
			t.Fatalf("wrapper contains forbidden side effect %q", forbidden)
		}
	}
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
