// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withFakeHome redirects the resolver's home accessor to dir for the duration
// of the test, restoring the previous accessor (and unsetting DEEPLINE_API_KEY)
// on cleanup. Tests use this to sandbox file discovery in t.TempDir() so the
// real $HOME is never touched.
func withFakeHome(t *testing.T, dir string) {
	t.Helper()
	prev := deeplineHomeFunc
	deeplineHomeFunc = func() (string, error) { return dir, nil }
	t.Cleanup(func() { deeplineHomeFunc = prev })
	t.Setenv("DEEPLINE_API_KEY", "")
}

// writeKeyFile writes a sibling-CLI .env file under fakeHome at the given
// host slug with mode `mode`. We chmod after WriteFile because the OS umask
// would otherwise strip group/world bits from the requested perms — without
// this, asking for 0664 silently yields 0644 on a default-umask macOS box,
// which would mask the very test cases that exist to catch loose modes.
func writeKeyFile(t *testing.T, fakeHome, slug, body string, mode os.FileMode) string {
	t.Helper()
	dir := filepath.Join(fakeHome, ".local", "deepline", slug)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte(body), mode); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Chmod(envPath, mode); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	return envPath
}

func TestResolveDeeplineKey_FlagWins(t *testing.T) {
	withFakeHome(t, t.TempDir())
	t.Setenv("DEEPLINE_API_KEY", "dlp_ENV")
	key, source := resolveDeeplineKey("dlp_FLAG")
	if key != "dlp_FLAG" || source != "flag" {
		t.Fatalf("got (%q,%q); want (dlp_FLAG, flag)", key, source)
	}
}

func TestResolveDeeplineKey_EnvWins(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_FILE\n", 0o600)
	t.Setenv("DEEPLINE_API_KEY", "dlp_ENV")
	key, source := resolveDeeplineKey("")
	if key != "dlp_ENV" || source != "env" {
		t.Fatalf("got (%q,%q); want (dlp_ENV, env)", key, source)
	}
}

func TestResolveDeeplineKey_FileFallback(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	envPath := writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_FILE\n", 0o600)
	key, source := resolveDeeplineKey("")
	if key != "dlp_FILE" {
		t.Fatalf("key=%q; want dlp_FILE", key)
	}
	if source != "file:"+envPath {
		t.Fatalf("source=%q; want file:%s", source, envPath)
	}
}

func TestResolveDeeplineKey_PreferredHostSlugWinsOverOthers(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	// Write the non-preferred slug first (alphabetically earlier), then the
	// preferred one. The resolver must still pick code-deepline-com.
	writeKeyFile(t, fakeHome, "alpha-deepline", "DEEPLINE_API_KEY=dlp_OTHER\n", 0o600)
	preferredPath := writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_PREFERRED\n", 0o600)
	key, source := resolveDeeplineKey("")
	if key != "dlp_PREFERRED" {
		t.Fatalf("key=%q; want dlp_PREFERRED (preferred slug should win)", key)
	}
	if source != "file:"+preferredPath {
		t.Fatalf("source=%q; want file:%s", source, preferredPath)
	}
}

func TestResolveDeeplineKey_LexicalFallbackWhenNoPreferredSlug(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	// Two non-preferred slugs. Lexical order picks "alpha" before "beta".
	alphaPath := writeKeyFile(t, fakeHome, "alpha-deepline", "DEEPLINE_API_KEY=dlp_ALPHA\n", 0o600)
	writeKeyFile(t, fakeHome, "beta-deepline", "DEEPLINE_API_KEY=dlp_BETA\n", 0o600)
	key, source := resolveDeeplineKey("")
	if key != "dlp_ALPHA" {
		t.Fatalf("key=%q; want dlp_ALPHA (lexical first wins absent preferred slug)", key)
	}
	if source != "file:"+alphaPath {
		t.Fatalf("source=%q; want file:%s", source, alphaPath)
	}
}

func TestResolveDeeplineKey_EmptyValueIsSkipped(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=\n", 0o600)
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty", key, source)
	}
	if len(skips) == 0 || !strings.Contains(skips[0], "empty value") {
		t.Fatalf("expected 'empty value' skip reason; got %v", skips)
	}
}

func TestResolveDeeplineKey_DoubleQuotedValue(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", `DEEPLINE_API_KEY="dlp_QUOTED"`+"\n", 0o600)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_QUOTED" {
		t.Fatalf("key=%q; want dlp_QUOTED (quotes should strip)", key)
	}
}

func TestResolveDeeplineKey_SingleQuotedValue(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", `DEEPLINE_API_KEY='dlp_SINGLE'`+"\n", 0o600)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_SINGLE" {
		t.Fatalf("key=%q; want dlp_SINGLE", key)
	}
}

func TestResolveDeeplineKey_CommentsAndBlanksIgnored(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	body := "# Deepline auth status\n\n# saved 2026-04-28\nDEEPLINE_API_KEY=dlp_AFTER_COMMENTS\n"
	writeKeyFile(t, fakeHome, "code-deepline-com", body, 0o600)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_AFTER_COMMENTS" {
		t.Fatalf("key=%q; want dlp_AFTER_COMMENTS", key)
	}
}

func TestResolveDeeplineKey_ExportPrefixHandled(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "export DEEPLINE_API_KEY=dlp_EXPORTED\n", 0o600)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_EXPORTED" {
		t.Fatalf("key=%q; want dlp_EXPORTED", key)
	}
}

func TestResolveDeeplineKey_Mode0644Accepted(t *testing.T) {
	// The official Deepline CLI writes the .env file at mode 0644. Auto-
	// discovery must accept that mode — rejecting it would defeat the
	// feature. The security boundary is group/world WRITE, not READ. (See
	// TestResolveDeeplineKey_GroupWritableRejected for the negative case.)
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_UPSTREAM_DEFAULT\n", 0o644)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_UPSTREAM_DEFAULT" {
		t.Fatalf("key=%q; want dlp_UPSTREAM_DEFAULT (mode 0644 must be accepted — Deepline CLI writes this mode)", key)
	}
}

func TestResolveDeeplineKey_GroupWritableRejected(t *testing.T) {
	// Group-writable means a non-owner principal could substitute the key
	// before we read it. Refuse.
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_TAMPERABLE\n", 0o664)
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty (group-writable mode 0664 must skip)", key, source)
	}
	if len(skips) == 0 || !strings.Contains(skips[0], "0664") {
		t.Fatalf("expected mode 0664 in skip reason; got %v", skips)
	}
}

func TestResolveDeeplineKey_WorldWritableRejected(t *testing.T) {
	// World-writable is the canonical credential-substitution risk.
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_TAMPERABLE\n", 0o666)
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty (world-writable mode 0666 must skip)", key, source)
	}
	if len(skips) == 0 || !strings.Contains(skips[0], "0666") {
		t.Fatalf("expected mode 0666 in skip reason; got %v", skips)
	}
}

func TestResolveDeeplineKey_Mode0400Accepted(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dlp_READONLY\n", 0o400)
	key, _ := resolveDeeplineKey("")
	if key != "dlp_READONLY" {
		t.Fatalf("key=%q; want dlp_READONLY (mode 0400 should be accepted)", key)
	}
}

func TestResolveDeeplineKey_SymlinkOutsideHomeRejected(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	// Build a real key file outside the fake home.
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "stolen.env")
	if err := os.WriteFile(outsideFile, []byte("DEEPLINE_API_KEY=dlp_ESCAPED\n"), 0o600); err != nil {
		t.Fatalf("write outside: %v", err)
	}
	// Create a symlink from inside the fake home that points at it.
	dir := filepath.Join(fakeHome, ".local", "deepline", "code-deepline-com")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	link := filepath.Join(dir, ".env")
	if err := os.Symlink(outsideFile, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty (symlink escape should skip)", key, source)
	}
	if len(skips) == 0 || !strings.Contains(skips[0], "symlink escapes home directory") {
		t.Fatalf("expected escape skip reason; got %v", skips)
	}
}

func TestResolveDeeplineKey_NoFlagNoEnvNoFile(t *testing.T) {
	withFakeHome(t, t.TempDir())
	key, source := resolveDeeplineKey("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty empty", key, source)
	}
}

func TestResolveDeeplineKey_BadPrefixSkipped(t *testing.T) {
	fakeHome := t.TempDir()
	withFakeHome(t, fakeHome)
	writeKeyFile(t, fakeHome, "code-deepline-com", "DEEPLINE_API_KEY=dpl_VERCEL_PREFIX\n", 0o600)
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty (bad prefix should skip)", key, source)
	}
	if len(skips) == 0 || !strings.Contains(skips[0], "missing dlp_ prefix") {
		t.Fatalf("expected prefix skip reason; got %v", skips)
	}
}

func TestResolveDeeplineKey_FileNotInDirectorySilent(t *testing.T) {
	// $HOME exists but no ~/.local/deepline directory at all — the common
	// case for users without the sibling CLI installed. Should be a clean
	// empty return with NO skip reasons (not a security skip, just absent).
	withFakeHome(t, t.TempDir())
	key, source, skips := resolveDeeplineKeyWithSkips("")
	if key != "" || source != "" {
		t.Fatalf("got (%q,%q); want empty", key, source)
	}
	if len(skips) != 0 {
		t.Fatalf("expected no skip reasons when sibling CLI absent; got %v", skips)
	}
}

func TestResolveDeeplineKey_FlagDoesNotTriggerFileScan(t *testing.T) {
	// When the flag is provided, the resolver must short-circuit and never
	// touch the filesystem. We assert this by setting deeplineHomeFunc to a
	// function that fails the test if called.
	fakeHome := t.TempDir()
	called := false
	prev := deeplineHomeFunc
	deeplineHomeFunc = func() (string, error) {
		called = true
		return fakeHome, nil
	}
	t.Cleanup(func() { deeplineHomeFunc = prev })
	t.Setenv("DEEPLINE_API_KEY", "")
	key, source := resolveDeeplineKey("dlp_FLAG")
	if key != "dlp_FLAG" || source != "flag" {
		t.Fatalf("got (%q,%q); want (dlp_FLAG, flag)", key, source)
	}
	if called {
		t.Fatal("resolver touched the filesystem when flag was set")
	}
}

func TestModeOctalFormatting(t *testing.T) {
	// Spot-check the small helper so a regex-based skip-reason assertion
	// elsewhere can rely on the exact rendering.
	cases := []struct {
		mode os.FileMode
		want string
	}{
		{0o600, "0600"},
		{0o644, "0644"},
		{0o400, "0400"},
		{0o777, "0777"},
		{0o000, "0000"},
	}
	for _, c := range cases {
		got := modeOctal(c.mode)
		if got != c.want {
			t.Errorf("modeOctal(%o)=%q; want %q", c.mode, got, c.want)
		}
	}
}
