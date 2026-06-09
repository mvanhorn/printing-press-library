package auth

import (
	"path/filepath"
	"testing"
)

func TestLoadFromPasteParsesHeader(t *testing.T) {
	header := "Cookie: session-id=123-456-789; at-main=abcd; ubid-main=ZZ"
	sess, err := LoadFromPaste(header, "")
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if len(sess.Cookies) != 3 {
		t.Fatalf("want 3 cookies, got %d", len(sess.Cookies))
	}
	if !sess.HasMarker() {
		t.Fatalf("expected marker (at-main + session-id), got false")
	}
	if sess.Cookies[0].Domain != ".amazon.com" {
		t.Fatalf("expected default domain to be .amazon.com, got %q", sess.Cookies[0].Domain)
	}
}

func TestLoadFromPasteRejectsEmpty(t *testing.T) {
	if _, err := LoadFromPaste("Cookie:", ""); err == nil {
		t.Fatal("expected error on empty cookie header")
	}
}

func TestSessionRoundTrip(t *testing.T) {
	sess, err := LoadFromPaste("session-id=1; at-main=A", "")
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	p := filepath.Join(t.TempDir(), "cookies.json")
	if err := sess.Save(p); err != nil {
		t.Fatalf("save: %v", err)
	}
	loaded, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Cookies) != 2 {
		t.Fatalf("round-trip expected 2 cookies, got %d", len(loaded.Cookies))
	}
}

func TestSessionHasMarkerNegative(t *testing.T) {
	s := &Session{Cookies: []Cookie{{Name: "session-id", Value: "x"}}}
	if s.HasMarker() {
		t.Fatal("session-id alone should not satisfy HasMarker")
	}
}

func TestCookieHeader(t *testing.T) {
	s := &Session{Cookies: []Cookie{{Name: "a", Value: "1"}, {Name: "b", Value: "2"}}}
	if got := s.CookieHeader(); got != "a=1; b=2" {
		t.Fatalf("CookieHeader = %q", got)
	}
}
