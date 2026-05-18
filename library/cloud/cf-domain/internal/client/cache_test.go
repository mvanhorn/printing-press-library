package client

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteCacheUsesPrivatePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	c := &Client{BaseURL: "https://example.test", cacheDir: dir}
	c.writeCache("/accounts/test/registrar/domain-check", map[string]string{"domain": "example.dev"}, json.RawMessage(`{"ok":true}`))

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("cache dir stat failed: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("cache dir mode = %o, want 700", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cache dir read failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("cache entries = %d, want 1", len(entries))
	}
	fileInfo, err := entries[0].Info()
	if err != nil {
		t.Fatalf("cache file info failed: %v", err)
	}
	if got := fileInfo.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache file mode = %o, want 600", got)
	}
}
