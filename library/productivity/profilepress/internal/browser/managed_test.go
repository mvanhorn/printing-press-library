package browser

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultManagedUserDataDirIsProfilePressOwned(t *testing.T) {
	dir := DefaultManagedUserDataDir()
	if !strings.Contains(dir, filepath.Join(".local", "share", "profilepress", "browser-profile")) {
		t.Fatalf("unexpected managed profile dir: %s", dir)
	}
}

func TestResolveBrowserBinaryHonorsExplicitPath(t *testing.T) {
	got, err := resolveBrowserBinary("/custom/chrome")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/chrome" {
		t.Fatalf("got %q", got)
	}
}
