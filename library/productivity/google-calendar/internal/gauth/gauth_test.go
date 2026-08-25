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

func TestScopesFor(t *testing.T) {
	if s, err := ScopesFor(RoleReadonly); err != nil || len(s) != 1 || !strings.HasSuffix(s[0], "calendar.readonly") {
		t.Fatalf("readonly: got %v, %v", s, err)
	}
	if s, err := ScopesFor(RoleWritable); err != nil || len(s) != 1 || !strings.HasSuffix(s[0], "/calendar") {
		t.Fatalf("writable: got %v, %v", s, err)
	}
	if _, err := ScopesFor("admin"); err == nil {
		t.Fatal("unknown role should error")
	}
}

func TestConfigDir(t *testing.T) {
	if got := ConfigDir("/x/override"); got != "/x/override" {
		t.Fatalf("override ignored: %s", got)
	}
	t.Setenv("GCAL_CONFIG_DIR", "/x/env")
	if got := ConfigDir(""); got != "/x/env" {
		t.Fatalf("env ignored: %s", got)
	}
	t.Setenv("GCAL_CONFIG_DIR", "")
	if got := ConfigDir(""); !strings.Contains(got, filepath.Join(".config", "google-calendar-pp-cli", "gauth")) {
		t.Fatalf("default path wrong: %s", got)
	}
}

func TestLoadProfiles(t *testing.T) {
	dir := t.TempDir()

	// Missing file: explicit, actionable error.
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "profiles not found") {
		t.Fatalf("missing file error wrong: %v", err)
	}

	// Valid file, role normalized from mixed case/whitespace.
	writeProfiles(t, dir, `accounts:
  - {name: personal, email: p@example.com, role: " Readonly "}
  - {name: ads, email: a@example.com, role: writable, default_write: true}
`)
	ps, err := LoadProfiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[0].Role != RoleReadonly || ps[1].Role != RoleWritable {
		t.Fatalf("parsed wrong: %+v", ps)
	}

	// Duplicate names rejected.
	writeProfiles(t, dir, `accounts:
  - {name: dup, email: a@example.com, role: readonly}
  - {name: dup, email: b@example.com, role: readonly}
`)
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate not rejected: %v", err)
	}

	// Unknown role rejected.
	writeProfiles(t, dir, `accounts:
  - {name: x, email: a@example.com, role: superuser}
`)
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "unknown role") {
		t.Fatalf("bad role not rejected: %v", err)
	}

	// Empty accounts rejected.
	writeProfiles(t, dir, "accounts: []\n")
	if _, err := LoadProfiles(dir); err == nil || !strings.Contains(err.Error(), "no accounts") {
		t.Fatalf("empty accounts not rejected: %v", err)
	}
}

func TestGetAndDefaultWrite(t *testing.T) {
	dir := t.TempDir()
	writeProfiles(t, dir, `accounts:
  - {name: personal, email: p@example.com, role: readonly}
  - {name: ads, email: a@example.com, role: writable, default_write: true}
`)
	p, err := Get(dir, "ads")
	if err != nil || p.Email != "a@example.com" {
		t.Fatalf("Get(ads): %+v, %v", p, err)
	}
	if _, err := Get(dir, "nope"); err == nil || !strings.Contains(err.Error(), "have: personal, ads") {
		t.Fatalf("Get(nope) should list profiles: %v", err)
	}
	dw, err := DefaultWrite(dir)
	if err != nil || dw.Name != "ads" {
		t.Fatalf("DefaultWrite: %+v, %v", dw, err)
	}

	// default_write on a readonly profile is a hard error.
	writeProfiles(t, dir, `accounts:
  - {name: personal, email: p@example.com, role: readonly, default_write: true}
`)
	if _, err := DefaultWrite(dir); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Fatalf("readonly default_write not rejected: %v", err)
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
  - {name: personal, email: p@example.com, role: readonly}
  - {name: bare, email: b@example.com, role: readonly}
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
