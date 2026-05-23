package client

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentJarImportSaveAndReload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.json")
	jar, err := NewPersistentJar(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jar.ImportCookieHeader("Cookie: dd_session_id=secret; csrf_token=token123", "https://www.doordash.com"); err != nil {
		t.Fatal(err)
	}
	if got := jar.CookieValue("csrf_token"); got != "token123" {
		t.Fatalf("csrf cookie = %q", got)
	}
	if err := jar.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session mode = %v, want 0600", info.Mode().Perm())
	}
	reloaded, err := NewPersistentJar(path)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://www.doordash.com/graphql/listCarts")
	cookies := reloaded.Cookies(u)
	if len(cookies) != 2 {
		t.Fatalf("cookies = %d, want 2", len(cookies))
	}
}

func TestPersistentJarRejectsEmptyCookieHeader(t *testing.T) {
	jar, err := NewPersistentJar("")
	if err != nil {
		t.Fatal(err)
	}
	if err := jar.ImportCookieHeader("not-a-cookie", "https://www.doordash.com"); err == nil {
		t.Fatal("expected invalid cookie header error")
	}
}
