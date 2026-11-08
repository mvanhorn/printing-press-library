package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCancelTokenPersistsAcrossProcesses(t *testing.T) {
	home := t.TempDir()
	token, err := IssueCancelToken(home, "100000001")
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("empty token")
	}
	if err := ValidateCancelToken(home, token, "100000001"); err != nil {
		t.Fatal(err)
	}
	// Simulate a new process reading the same config dir.
	if err := ConsumeCancelToken(home, token, "100000001"); err != nil {
		t.Fatal(err)
	}
	if err := ConsumeCancelToken(home, token, "100000001"); err == nil {
		t.Fatal("expected single-use failure")
	}
	path := filepath.Join(home, "confirm-tokens.json")
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("token file missing: %v", err)
	}
}

func TestConfirmTokensTightenExistingPermissions(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "confirm-tokens.json")
	if err := os.WriteFile(path, []byte(`{"cancel":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := IssueCancelToken(home, "100000001"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o", info.Mode().Perm())
	}
}
