// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
package gauth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func writeProfiles(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "profiles.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Every role — declared, weird, or absent — maps to exactly the single
// gmail.modify scope. That IS the scope model; a regression here would mint
// tokens with a different authority than the binary was reviewed for.
func TestScopesFor(t *testing.T) {
	cases := []struct {
		name string
		role string
	}{
		{"readonly role", "readonly"},
		{"writable role", "writable"},
		{"cleanup role", "cleanup"},
		{"empty role", ""},
		{"unknown role", "superuser"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s, err := ScopesFor(c.role)
			if err != nil {
				t.Fatalf("ScopesFor(%q) errored: %v", c.role, err)
			}
			if len(s) != 1 || s[0] != ScopeModify {
				t.Fatalf("ScopesFor(%q) = %v, want exactly [%s]", c.role, s, ScopeModify)
			}
			if !strings.HasSuffix(s[0], "gmail.modify") {
				t.Fatalf("scope is not gmail.modify: %s", s[0])
			}
		})
	}
}

func TestConfigDir(t *testing.T) {
	if got := ConfigDir("/x/override"); got != "/x/override" {
		t.Fatalf("override ignored: %s", got)
	}
	t.Setenv("GMAIL_CONFIG_DIR", "/x/env")
	if got := ConfigDir(""); got != "/x/env" {
		t.Fatalf("env ignored: %s", got)
	}
	t.Setenv("GMAIL_CONFIG_DIR", "")
	if got := ConfigDir(""); !strings.Contains(got, filepath.Join(".config", "gmail-pp-cli", "gauth")) {
		t.Fatalf("default path wrong: %s", got)
	}
	// Override wins over env when both are set.
	t.Setenv("GMAIL_CONFIG_DIR", "/x/env")
	if got := ConfigDir("/x/flag"); got != "/x/flag" {
		t.Fatalf("flag should beat env: %s", got)
	}
}

func TestLoadProfiles(t *testing.T) {
	dir := t.TempDir()

	// Missing file: explicit, actionable error.
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "profiles not found") {
		t.Fatalf("missing file error wrong: %v", err)
	}

	// Valid file, role normalized from mixed case/whitespace, arbitrary
	// role strings accepted (forward-compat; scope is pinned regardless).
	writeProfiles(t, dir, `accounts:
  - {name: ads, email: cleanup-a@example.com, role: " Cleanup "}
  - {name: personal, email: cleanup-b@example.com, role: readonly}
`)
	ps, err := LoadProfiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].Role != "cleanup" || ps[1].Role != "readonly" {
		t.Fatalf("parsed wrong: %+v", ps)
	}

	// Duplicate names rejected.
	writeProfiles(t, dir, `accounts:
  - {name: dup, email: a@example.com, role: cleanup}
  - {name: dup, email: b@example.com, role: cleanup}
`)
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate not rejected: %v", err)
	}

	// Missing name/email rejected.
	writeProfiles(t, dir, `accounts:
  - {name: "", email: a@example.com, role: cleanup}
`)
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "missing name or email") {
		t.Fatalf("missing name not rejected: %v", err)
	}

	// Empty accounts rejected.
	writeProfiles(t, dir, "accounts: []\n")
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "no accounts") {
		t.Fatalf("empty accounts not rejected: %v", err)
	}
}

func TestGetAndDefaultAccount(t *testing.T) {
	dir := t.TempDir()
	writeProfiles(t, dir, `accounts:
  - {name: ads, email: cleanup-a@example.com, role: cleanup}
  - {name: personal, email: cleanup-b@example.com, role: cleanup}
`)
	p, err := Get(dir, "personal")
	if err != nil || p.Email != "cleanup-b@example.com" {
		t.Fatalf("Get(personal): %+v, %v", p, err)
	}
	if _, err := Get(dir, "nope"); err == nil || !strings.Contains(err.Error(), "have: ads, personal") {
		t.Fatalf("Get(nope) should list profiles: %v", err)
	}

	// Two profiles: no implicit default; error must list the choices.
	if _, err := DefaultAccount(dir); err == nil || !strings.Contains(err.Error(), "pass --account") || !strings.Contains(err.Error(), "ads, personal") {
		t.Fatalf("DefaultAccount with 2 profiles should demand --account: %v", err)
	}

	// Exactly one profile: it is the default.
	writeProfiles(t, dir, `accounts:
  - {name: solo, email: solo@example.com, role: cleanup}
`)
	dp, err := DefaultAccount(dir)
	if err != nil || dp.Name != "solo" {
		t.Fatalf("DefaultAccount with 1 profile: %+v, %v", dp, err)
	}
}

func TestTokenRoundTrip(t *testing.T) {
	dir := t.TempDir()
	tok := &oauth2.Token{AccessToken: "at", RefreshToken: "rt", Expiry: time.Now().Add(time.Hour).UTC()}
	if err := saveToken(dir, "personal", tok); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(tokenPath(dir, "personal"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("token file mode = %v, want 0600", fi.Mode().Perm())
	}
	got, err := loadToken(dir, "personal")
	if err != nil || got.AccessToken != "at" || got.RefreshToken != "rt" {
		t.Fatalf("round trip: %+v, %v", got, err)
	}
	// Statuses sees the token.
	writeProfiles(t, dir, `accounts:
  - {name: personal, email: p@example.com, role: cleanup}
  - {name: bare, email: b@example.com, role: cleanup}
`)
	sts, err := Statuses(dir)
	if err != nil || len(sts) != 2 {
		t.Fatalf("Statuses: %+v, %v", sts, err)
	}
	if !sts[0].HasTok || sts[1].HasTok {
		t.Fatalf("token presence wrong: %+v", sts)
	}
}

func TestRetryAfter(t *testing.T) {
	def, max := 2*time.Second, 5*time.Second
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", def}, // absent -> default
		{"3", 3 * time.Second},
		{" 4 ", 4 * time.Second}, // whitespace tolerated
		{"9", max},               // capped
		{"-1", def},              // negative -> default
		{"soon", def},            // HTTP-date / junk -> default
	}
	for _, c := range cases {
		if got := retryAfter(c.in, def, max); got != c.want {
			t.Errorf("retryAfter(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
