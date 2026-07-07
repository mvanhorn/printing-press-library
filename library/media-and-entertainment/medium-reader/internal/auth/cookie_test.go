// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// All tests here are HERMETIC: they exercise the env + file parsing paths only,
// never the Chrome/Keychain auto-extract path (which would pop a macOS GUI
// authorization). The Keychain path is guarded so the binary builds and the
// env/file paths work without it; that is asserted indirectly by these tests
// passing on every platform with no Keychain access.

// ---- MEDIUM_SESSION env parsing -------------------------------------------

// TestParseSessionEnv_KeyValuePairs covers the documented "sid=..; uid=.." raw
// cookie-header form. Both fields must land on the right Cookies field, and
// whitespace/ordering must not matter.
func TestParseSessionEnv_KeyValuePairs(t *testing.T) {
	c := ParseSessionEnv("sid=SID123; uid=UID456")
	if c.Sid != "SID123" {
		t.Errorf("Sid = %q, want SID123", c.Sid)
	}
	if c.Uid != "UID456" {
		t.Errorf("Uid = %q, want UID456", c.Uid)
	}
	if c.IsZero() {
		t.Error("Cookies should not be zero after parsing a sid+uid pair")
	}

	// Reordered, extra spaces, and a cf_clearance pair.
	c2 := ParseSessionEnv("  uid=U ;  cf_clearance=CF ; sid=S  ")
	if c2.Sid != "S" || c2.Uid != "U" || c2.CfClearance != "CF" {
		t.Errorf("reordered parse = %+v, want Sid=S Uid=U CfClearance=CF", c2)
	}
}

// TestParseSessionEnv_BareSid covers the "just the sid value" convenience form:
// a string with no '=' is taken to be the sid token.
func TestParseSessionEnv_BareSid(t *testing.T) {
	c := ParseSessionEnv("RAWSIDVALUE")
	if c.Sid != "RAWSIDVALUE" {
		t.Errorf("bare-sid Sid = %q, want RAWSIDVALUE", c.Sid)
	}
	if c.Uid != "" || c.CfClearance != "" {
		t.Errorf("bare-sid set extra fields: %+v", c)
	}
}

// TestParseSessionEnv_Empty covers the "no env" case: empty/whitespace-only
// input must yield a zero Cookies and (downstream) no error.
func TestParseSessionEnv_Empty(t *testing.T) {
	if !ParseSessionEnv("").IsZero() {
		t.Error("empty env should yield zero Cookies")
	}
	if !ParseSessionEnv("   ").IsZero() {
		t.Error("whitespace-only env should yield zero Cookies")
	}
}

// TestParseSessionEnv_StripsSurroundingQuotes guards the common shell-paste
// mistake where the value is wrapped in quotes.
func TestParseSessionEnv_StripsSurroundingQuotes(t *testing.T) {
	c := ParseSessionEnv(`sid="QUOTED"; uid='UQ'`)
	if c.Sid != "QUOTED" {
		t.Errorf("Sid = %q, want QUOTED (quotes stripped)", c.Sid)
	}
	if c.Uid != "UQ" {
		t.Errorf("Uid = %q, want UQ (quotes stripped)", c.Uid)
	}
}

// ---- cookie-file parsing ---------------------------------------------------

// TestParseCookieFile_FlatJSON covers the documented flat {"sid":..,"uid":..}
// shape.
func TestParseCookieFile_FlatJSON(t *testing.T) {
	path := writeTemp(t, `{"sid":"FILESID","uid":"FILEUID","cf_clearance":"FILECF"}`)
	c, err := ParseCookieFile(path)
	if err != nil {
		t.Fatalf("ParseCookieFile: %v", err)
	}
	if c.Sid != "FILESID" || c.Uid != "FILEUID" || c.CfClearance != "FILECF" {
		t.Errorf("file parse = %+v", c)
	}
}

// TestParseCookieFile_Partial covers a file with only sid (uid optional).
func TestParseCookieFile_Partial(t *testing.T) {
	path := writeTemp(t, `{"sid":"ONLYSID"}`)
	c, err := ParseCookieFile(path)
	if err != nil {
		t.Fatalf("ParseCookieFile: %v", err)
	}
	if c.Sid != "ONLYSID" || c.Uid != "" {
		t.Errorf("partial parse = %+v, want only Sid=ONLYSID", c)
	}
}

// TestParseCookieFile_Missing: a missing path is NOT an error in the loader
// contract (the chain just falls through), but ParseCookieFile itself reports
// the read error so callers can distinguish "no file configured" from "file
// configured but unreadable". The empty-path case returns zero + nil.
func TestParseCookieFile_EmptyPath(t *testing.T) {
	c, err := ParseCookieFile("")
	if err != nil {
		t.Errorf("empty path err = %v, want nil", err)
	}
	if !c.IsZero() {
		t.Error("empty path should yield zero Cookies")
	}
}

func TestParseCookieFile_NonexistentPath(t *testing.T) {
	_, err := ParseCookieFile(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Error("nonexistent file should report a read error")
	}
}

func TestParseCookieFile_BadJSON(t *testing.T) {
	path := writeTemp(t, `not json at all`)
	_, err := ParseCookieFile(path)
	if err == nil {
		t.Error("bad JSON should report a parse error")
	}
}

// ---- fallback chain (Load) -------------------------------------------------

// TestLoad_EnvWinsOverFile asserts the first-hit-wins ordering: MEDIUM_SESSION
// env beats the cookie file.
func TestLoad_EnvWinsOverFile(t *testing.T) {
	t.Setenv("MEDIUM_SESSION", "sid=ENVSID")
	t.Setenv("MEDIUM_COOKIE_FILE", "")
	filePath := writeTemp(t, `{"sid":"FILESID"}`)

	c, err := Load(Options{CookieFile: filePath})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Sid != "ENVSID" {
		t.Errorf("Load Sid = %q, want ENVSID (env should win over file)", c.Sid)
	}
}

// TestLoad_FileWhenNoEnv asserts the file is used when no env is set, and that
// the explicit --cookie-file Option beats the MEDIUM_COOKIE_FILE env.
func TestLoad_FileWhenNoEnv(t *testing.T) {
	t.Setenv("MEDIUM_SESSION", "")
	envFile := writeTemp(t, `{"sid":"ENVFILESID"}`)
	flagFile := writeTemp(t, `{"sid":"FLAGFILESID"}`)
	t.Setenv("MEDIUM_COOKIE_FILE", envFile)

	// Explicit flag path wins over the env-provided path.
	c, err := Load(Options{CookieFile: flagFile})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Sid != "FLAGFILESID" {
		t.Errorf("Load Sid = %q, want FLAGFILESID (flag path beats env path)", c.Sid)
	}

	// With no flag path, the MEDIUM_COOKIE_FILE env path is used.
	c2, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load (env file): %v", err)
	}
	if c2.Sid != "ENVFILESID" {
		t.Errorf("Load Sid = %q, want ENVFILESID (env file path used)", c2.Sid)
	}
}

// TestLoad_NoneConfigured asserts the all-optional contract: with nothing set,
// Load returns a zero Cookies and NO error (anonymous Tier 0).
func TestLoad_NoneConfigured(t *testing.T) {
	t.Setenv("MEDIUM_SESSION", "")
	t.Setenv("MEDIUM_COOKIE_FILE", "")
	c, err := Load(Options{})
	if err != nil {
		t.Fatalf("Load with nothing configured returned err = %v, want nil", err)
	}
	if !c.IsZero() {
		t.Errorf("Load with nothing configured = %+v, want zero", c)
	}
}

// TestLoad_EmptyEnvFallsThroughToFile asserts an empty MEDIUM_SESSION does not
// shadow a present file (empty env is "not set", not "set to nothing wins").
func TestLoad_EmptyEnvFallsThroughToFile(t *testing.T) {
	t.Setenv("MEDIUM_SESSION", "   ")
	t.Setenv("MEDIUM_COOKIE_FILE", "")
	filePath := writeTemp(t, `{"sid":"FILESID2"}`)
	c, err := Load(Options{CookieFile: filePath})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Sid != "FILESID2" {
		t.Errorf("Load Sid = %q, want FILESID2 (empty env should fall through)", c.Sid)
	}
}

// TestLoad_BadFileIsAnError asserts that a configured-but-unreadable/bad file is
// surfaced as an error (it was explicitly configured, so a silent zero would
// hide a user mistake). Env still wins first, so this only triggers when env is
// absent.
func TestLoad_BadFileIsAnError(t *testing.T) {
	t.Setenv("MEDIUM_SESSION", "")
	t.Setenv("MEDIUM_COOKIE_FILE", "")
	path := writeTemp(t, `{bad json`)
	if _, err := Load(Options{CookieFile: path}); err == nil {
		t.Error("Load with a configured bad file should return an error")
	}
}

// ---- selectMediumCookies (Chrome-extract selection seam) ------------------

// selectMediumCookies is the testable core of the build-tagged kooky extractor:
// it picks the Medium session cookies out of a browser cookie jar with no
// kooky/Chrome dependency, so the selection logic is exercised hermetically here
// while the actual extraction lives behind the `kooky` build tag.

func TestSelectMediumCookies_PicksSidAndUid(t *testing.T) {
	got, err := selectMediumCookies([]BrowserCookie{
		{Name: "sid", Value: "abc", Domain: ".medium.com"},
		{Name: "uid", Value: "def", Domain: ".medium.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sid != "abc" || got.Uid != "def" {
		t.Errorf("got %+v, want sid=abc uid=def", got)
	}
}

func TestSelectMediumCookies_IgnoresOtherDomains(t *testing.T) {
	got, err := selectMediumCookies([]BrowserCookie{
		{Name: "sid", Value: "google-sid", Domain: ".google.com"},
		{Name: "sid", Value: "medium-sid", Domain: ".medium.com"},
		{Name: "uid", Value: "other", Domain: "example.org"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sid != "medium-sid" {
		t.Errorf("got sid=%q, want medium-sid (other-domain sid must be ignored)", got.Sid)
	}
	if got.Uid != "" {
		t.Errorf("got uid=%q, want empty (the uid was on a non-medium domain)", got.Uid)
	}
}

func TestSelectMediumCookies_IncludesCfClearance(t *testing.T) {
	got, err := selectMediumCookies([]BrowserCookie{
		{Name: "sid", Value: "s", Domain: "medium.com"},
		{Name: "cf_clearance", Value: "cf", Domain: ".medium.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.CfClearance != "cf" {
		t.Errorf("got cf_clearance=%q, want cf", got.CfClearance)
	}
}

func TestSelectMediumCookies_MatchesBareAndDottedDomain(t *testing.T) {
	got, err := selectMediumCookies([]BrowserCookie{
		{Name: "sid", Value: "s", Domain: "medium.com"}, // no leading dot
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sid != "s" {
		t.Errorf("bare medium.com domain should match; got %+v", got)
	}
}

func TestSelectMediumCookies_NoSidIsError(t *testing.T) {
	_, err := selectMediumCookies([]BrowserCookie{
		{Name: "uid", Value: "def", Domain: ".medium.com"}, // uid but no sid
		{Name: "sid", Value: "x", Domain: ".other.com"},    // sid on the wrong domain
	})
	if !errors.Is(err, ErrNoMediumCookies) {
		t.Errorf("expected ErrNoMediumCookies when no medium.com sid present, got %v", err)
	}
}

// ---- WriteCookieFile (persist extracted cookie) ---------------------------

// TestWriteCookieFile_RoundTrips asserts the written file is exactly the flat
// JSON shape ParseCookieFile reads back, so `auth login --chrome` and the
// --cookie-file load path agree on the format.
func TestWriteCookieFile_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.json")
	want := source.Cookies{Sid: "s-123", Uid: "u-456", CfClearance: "cf-789"}
	if err := WriteCookieFile(path, want); err != nil {
		t.Fatalf("WriteCookieFile: %v", err)
	}
	got, err := ParseCookieFile(path)
	if err != nil {
		t.Fatalf("ParseCookieFile: %v", err)
	}
	if got != want {
		t.Errorf("round-trip mismatch: wrote %+v, read %+v", want, got)
	}
}

// TestWriteCookieFile_Restrictive0600 asserts the file holding a live session is
// written owner-only (not group/other readable). Skipped on Windows (no POSIX
// mode bits).
func TestWriteCookieFile_Restrictive0600(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits not meaningful on Windows")
	}
	path := filepath.Join(t.TempDir(), "saved.json")
	if err := WriteCookieFile(path, source.Cookies{Sid: "s"}); err != nil {
		t.Fatalf("WriteCookieFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("cookie file mode = %#o, want 0600", perm)
	}
}

// TestWriteCookieFile_CreatesParentDirs lets `--chrome` write to a default path
// under a config dir that may not exist yet.
func TestWriteCookieFile_CreatesParentDirs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "cookies.json")
	if err := WriteCookieFile(path, source.Cookies{Sid: "s"}); err != nil {
		t.Fatalf("WriteCookieFile into a fresh dir tree: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file at %s: %v", path, err)
	}
}

// ---- helpers ---------------------------------------------------------------

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "cookies.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	return path
}
