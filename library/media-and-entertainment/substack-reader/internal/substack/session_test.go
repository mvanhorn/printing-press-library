// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package substack

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSessionEnv(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare sid", "s%3Aabc.def", "s%3Aabc.def"},
		{"named pair", "substack.sid=s%3Aabc.def", "s%3Aabc.def"},
		{"sid alias", "sid=s%3Aabc.def", "s%3Aabc.def"},
		{"fragment picks substack.sid", "other=1; substack.sid=s%3Aabc; x=2", "s%3Aabc"},
		{"quoted value", "\"s%3Aabc\"", "s%3Aabc"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseSessionEnv(tc.raw).SID; got != tc.want {
				t.Errorf("SID = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSessionCookieHeaderAndZero(t *testing.T) {
	if !(Session{}).IsZero() {
		t.Error("empty Session should be zero")
	}
	if (Session{}).CookieHeader() != "" {
		t.Error("empty Session should render no cookie header")
	}
	s := Session{SID: "s%3Aabc"}
	if s.IsZero() {
		t.Error("populated Session should not be zero")
	}
	if got, want := s.CookieHeader(), "substack.sid=s%3Aabc"; got != want {
		t.Errorf("CookieHeader = %q, want %q", got, want)
	}
}

func TestMaskTokenNeverLeaksFullValue(t *testing.T) {
	secret := "s%3Ao1LZDurTP5yR9cDIzxf-DIDplzuYZeXd.qycfLN6fzBse"
	masked := MaskToken(secret)
	if strings.Contains(masked, secret) {
		t.Fatalf("masked value must not contain the full secret: %q", masked)
	}
	// The unique tail must not appear.
	if strings.Contains(masked, "qycfLN6fzBse") {
		t.Fatalf("masked value leaked the secret tail: %q", masked)
	}
	if MaskToken("") != "<none>" {
		t.Errorf("empty token should mask to <none>")
	}
}

func TestCookieFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.json")
	want := Session{SID: "s%3Aroundtrip.value"}
	if err := WriteCookieFile(path, want); err != nil {
		t.Fatalf("WriteCookieFile: %v", err)
	}
	// File must be 0600 (holds a live session).
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cookie file mode = %#o, want 0600", perm)
	}
	got, err := ParseCookieFile(path)
	if err != nil {
		t.Fatalf("ParseCookieFile: %v", err)
	}
	if got.SID != want.SID {
		t.Errorf("round-trip SID = %q, want %q", got.SID, want.SID)
	}
}

func TestLoadSessionExplicitCookieFileBeatsDefault(t *testing.T) {
	// SUBSTACK_COOKIE_FILE (an explicit override) must win over a stale
	// default config-dir cookie.json. Regression for the precedence bug where
	// the default file was tried first, so a stale default silently beat an
	// explicit override and paid posts read as preview-only (Greptile P1).
	home := t.TempDir()
	t.Setenv("SUBSTACK_READER_HOME", home) // steers ConfigDir -> <home>/config
	t.Setenv(EnvSession, "")      // no env session; exercise the file path
	t.Setenv(EnvCookieFile, "")   // set below, after writing the default

	defaultPath, err := DefaultCookieFilePath()
	if err != nil {
		t.Fatalf("DefaultCookieFilePath: %v", err)
	}
	if err := WriteCookieFile(defaultPath, Session{SID: "s%3Astale-default"}); err != nil {
		t.Fatalf("write default cookie: %v", err)
	}
	explicit := filepath.Join(t.TempDir(), "explicit.json")
	if err := WriteCookieFile(explicit, Session{SID: "s%3Afresh-explicit"}); err != nil {
		t.Fatalf("write explicit cookie: %v", err)
	}
	t.Setenv(EnvCookieFile, explicit)

	got, err := LoadSession()
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if got.SID != "s%3Afresh-explicit" {
		t.Errorf("SID = %q, want the explicit SUBSTACK_COOKIE_FILE to win over the stale default", got.SID)
	}
}

func TestLoadSessionExplicitMissingCookieFileIsHardError(t *testing.T) {
	// An explicit SUBSTACK_COOKIE_FILE that is missing/mistyped must be a hard
	// error, not a silent fall-through to a stale default cookie (Greptile P1,
	// follow-up to the precedence fix). The default cookie below must NOT win.
	home := t.TempDir()
	t.Setenv("SUBSTACK_READER_HOME", home)
	t.Setenv(EnvSession, "")
	defaultPath, err := DefaultCookieFilePath()
	if err != nil {
		t.Fatalf("DefaultCookieFilePath: %v", err)
	}
	if err := WriteCookieFile(defaultPath, Session{SID: "s%3Astale-default"}); err != nil {
		t.Fatalf("write default cookie: %v", err)
	}
	t.Setenv(EnvCookieFile, filepath.Join(t.TempDir(), "does-not-exist.json"))

	got, err := LoadSession()
	if err == nil {
		t.Fatalf("expected a hard error for a missing explicit cookie file, got session %q", got.SID)
	}
	if got.SID == "s%3Astale-default" {
		t.Error("a missing explicit cookie file must not silently fall back to the default")
	}
}

func TestLoadSessionEmptyExplicitCookieFileIsHardError(t *testing.T) {
	// An explicit SUBSTACK_COOKIE_FILE that exists but carries no SID (e.g. `{}`)
	// must be a hard error, not a silent fall-through to the default — the final
	// edge of the explicit-is-authoritative class (Greptile P1).
	home := t.TempDir()
	t.Setenv("SUBSTACK_READER_HOME", home)
	t.Setenv(EnvSession, "")
	defaultPath, err := DefaultCookieFilePath()
	if err != nil {
		t.Fatalf("DefaultCookieFilePath: %v", err)
	}
	if err := WriteCookieFile(defaultPath, Session{SID: "s%3Astale-default"}); err != nil {
		t.Fatalf("write default cookie: %v", err)
	}
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvCookieFile, empty)

	got, err := LoadSession()
	if err == nil {
		t.Fatalf("expected a hard error for a SID-less explicit cookie file, got session %q", got.SID)
	}
	if got.SID == "s%3Astale-default" {
		t.Error("a SID-less explicit cookie file must not fall back to the default")
	}
}

func TestLoadSessionCorruptDefaultDegradesToAnonymous(t *testing.T) {
	// A corrupt DEFAULT cookie file must NOT error — only an explicit
	// SUBSTACK_COOKIE_FILE is authoritative. This keeps a plain free read working
	// (and lets read.go propagate LoadSession errors unconditionally, knowing an
	// error always means an explicit cookie failed).
	home := t.TempDir()
	t.Setenv("SUBSTACK_READER_HOME", home)
	t.Setenv(EnvSession, "")
	t.Setenv(EnvCookieFile, "")
	defaultPath, err := DefaultCookieFilePath()
	if err != nil {
		t.Fatalf("DefaultCookieFilePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(defaultPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(defaultPath, []byte("not json at all"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := LoadSession()
	if err != nil {
		t.Fatalf("a corrupt default cookie must degrade to anonymous, not error: %v", err)
	}
	if !got.IsZero() {
		t.Errorf("corrupt default must yield an anonymous session, got %q", got.SID)
	}
}

func TestLoadSessionUnparseableEnvWarns(t *testing.T) {
	// A non-empty SUBSTACK_SESSION that carries no recognizable substack.sid
	// (e.g. "garbage=value" — has an '=' but no known cookie key) must WARN on
	// stderr rather than silently fall through to the cookie files as if unset
	// (Greptile P2). With no cookie file configured it then degrades to anonymous.
	home := t.TempDir()
	t.Setenv("SUBSTACK_READER_HOME", home) // steer ConfigDir -> <home>/config (no cookie there)
	t.Setenv(EnvCookieFile, "")
	t.Setenv(EnvSession, "garbage=value")

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w

	got, loadErr := LoadSession()

	_ = w.Close()
	os.Stderr = orig
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	if loadErr != nil {
		t.Fatalf("LoadSession should degrade to anonymous, not error: %v", loadErr)
	}
	if !got.IsZero() {
		t.Errorf("unparseable SUBSTACK_SESSION should fall through to anonymous, got SID %q", got.SID)
	}
	if !strings.Contains(buf.String(), "SUBSTACK_SESSION") {
		t.Errorf("expected a stderr warning mentioning SUBSTACK_SESSION, got: %q", buf.String())
	}
}

func TestParseCookieFileSidAlias(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cookie.json")
	if err := os.WriteFile(path, []byte(`{"sid":"s%3Aalias"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := ParseCookieFile(path)
	if err != nil {
		t.Fatalf("ParseCookieFile: %v", err)
	}
	if got.SID != "s%3Aalias" {
		t.Errorf("SID = %q, want alias value", got.SID)
	}
}
