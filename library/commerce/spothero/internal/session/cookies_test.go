package session_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

func TestParseCookieInput(t *testing.T) {
	s, err := session.ParseCookieInput("sessionid=abc; csrftoken=xyz", "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.CookieHeader() != "sessionid=abc; csrftoken=xyz" {
		t.Fatalf("header=%q", s.CookieHeader())
	}
}

func TestParsePlaywrightStorageState(t *testing.T) {
	raw := `{"cookies":[{"name":"sessionid","value":"abc","domain":".spothero.com"}]}`
	s, err := session.ParseCookiesFile([]byte(raw), "test")
	if err != nil {
		t.Fatal(err)
	}
	if s.Cookies["sessionid"] != "abc" {
		t.Fatalf("cookies=%v", s.Cookies)
	}
}

func TestSaveSessionPermissions(t *testing.T) {
	dir := t.TempDir()
	s := &session.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := session.SaveSession(dir, s); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(dir, "cookies.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("perm=%o", info.Mode().Perm())
	}
}

func TestSaveSessionTightensExistingPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &session.Session{Cookies: map[string]string{"a": "b"}, Source: "test"}
	if err := session.SaveSession(dir, s); err != nil {
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
