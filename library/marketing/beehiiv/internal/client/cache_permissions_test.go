package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCacheUsesPrivatePermissions(t *testing.T) {
	cacheDir := filepath.Join(t.TempDir(), "cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	c := &Client{BaseURL: "https://api.example.test", cacheDir: cacheDir}
	cacheFile := filepath.Join(cacheDir, c.cacheKey("/subscriptions", nil)+".json")
	if err := os.WriteFile(cacheFile, []byte(`{"old":true}`), 0o644); err != nil {
		t.Fatal(err)
	}

	c.writeCache("/subscriptions", nil, json.RawMessage(`{"ok":true}`))

	dirInfo, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("cache dir permissions = %o, want 700", got)
	}

	fileInfo, err := os.Stat(cacheFile)
	if err != nil {
		t.Fatal(err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache file permissions = %o, want 600", got)
	}
}
