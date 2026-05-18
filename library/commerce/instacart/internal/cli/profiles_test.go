package cli

// PATCH (instacart-address-profiles): tests for `config profiles` subtree and
// `--profile` per-call override. Stays in-package so we can drive the cobra
// command tree directly and assert on persisted state through config.Load().

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
)

// withTempConfig redirects config + store to a per-test temp dir.
func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)
	return dir
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()
	return runCmdCtx(t, context.Background(), args...)
}

// runCmdCtx is the variant that lets a test inject a fake
// userAddressFetcher (via context) without mutating package globals.
// Tests doing `profiles import` should call this with a context built by
// withFetcher() so the real GraphQL stack is bypassed.
func runCmdCtx(t *testing.T, ctx context.Context, args ...string) (string, error) {
	t.Helper()
	root := Root()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	return buf.String(), err
}

// withFetcher returns a context that pins f as the user-address fetcher
// for the lifetime of one command run.
func withFetcher(f userAddressFetcher) context.Context {
	return context.WithValue(context.Background(), userAddressFetcherKey{}, f)
}

func TestProfilesAddListCoordsOnly(t *testing.T) {
	withTempConfig(t)

	out, err := runCmd(t, "config", "profiles", "add", "home",
		"--lat", "47.6331", "--lon", "-122.2850", "--postal", "98112",
		"--label", "1528 37th Ave E")
	if err != nil {
		t.Fatalf("add home: %v (out=%s)", err, out)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	p, ok := cfg.GetProfile("home")
	if !ok {
		t.Fatalf("home not saved (out=%s)", out)
	}
	if p.PostalCode != "98112" || p.Latitude != 47.6331 {
		t.Errorf("home profile wrong: %+v", p)
	}

	listOut, err := runCmd(t, "config", "profiles", "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !bytes.Contains([]byte(listOut), []byte("home")) {
		t.Errorf("list output missing home: %s", listOut)
	}
}

func TestProfilesAddIDAndCoordsMutuallyExclusive(t *testing.T) {
	withTempConfig(t)
	out, err := runCmd(t, "config", "profiles", "add", "x",
		"--id", "73256642", "--lat", "1", "--lon", "1")
	if err == nil {
		t.Fatalf("expected error when both --id and --lat/--lon given (out=%s)", out)
	}
}

func TestProfilesAddRequiresOneSource(t *testing.T) {
	withTempConfig(t)
	out, err := runCmd(t, "config", "profiles", "add", "x")
	if err == nil {
		t.Fatalf("expected error with neither --id nor --lat/--lon (out=%s)", out)
	}
}

func TestProfilesAddRejectsInvalidName(t *testing.T) {
	withTempConfig(t)
	out, err := runCmd(t, "config", "profiles", "add", "BadName",
		"--lat", "1", "--lon", "1")
	if err == nil {
		t.Fatalf("expected invalid-name error (out=%s)", out)
	}
}

func TestProfilesUseAndRm(t *testing.T) {
	withTempConfig(t)
	if _, err := runCmd(t, "config", "profiles", "add", "home", "--lat", "47.63", "--lon", "-122.28", "--postal", "98112"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, err := runCmd(t, "config", "profiles", "use", "home"); err != nil {
		t.Fatalf("use: %v", err)
	}
	cfg, _ := config.Load()
	if cfg.ActiveProfile != "home" || cfg.PostalCode != "98112" {
		t.Fatalf("after use: active=%q postal=%q", cfg.ActiveProfile, cfg.PostalCode)
	}

	if _, err := runCmd(t, "config", "profiles", "rm", "home"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	cfg, _ = config.Load()
	if _, ok := cfg.GetProfile("home"); ok {
		t.Errorf("home still present after rm")
	}
	if cfg.ActiveProfile != "" {
		t.Errorf("ActiveProfile not cleared on rm of active; got %q", cfg.ActiveProfile)
	}

	// rm of unknown profile must error
	if _, err := runCmd(t, "config", "profiles", "rm", "nope"); err == nil {
		t.Errorf("rm of unknown profile should error")
	}
}

func TestProfilesUseAndAddWithUseFlag(t *testing.T) {
	withTempConfig(t)
	if _, err := runCmd(t, "config", "profiles", "add", "work",
		"--lat", "47.67", "--lon", "-122.12", "--postal", "98052", "--use"); err != nil {
		t.Fatalf("add --use: %v", err)
	}
	cfg, _ := config.Load()
	if cfg.ActiveProfile != "work" {
		t.Errorf("expected work active, got %q", cfg.ActiveProfile)
	}
}

func TestProfilesListJSON(t *testing.T) {
	withTempConfig(t)
	if _, err := runCmd(t, "config", "profiles", "add", "home",
		"--lat", "47.63", "--lon", "-122.28", "--postal", "98112", "--use"); err != nil {
		t.Fatalf("add: %v", err)
	}
	out, err := runCmd(t, "config", "profiles", "list", "--json")
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var got struct {
		Active   string `json:"active"`
		Profiles []struct {
			Name   string `json:"name"`
			Active bool   `json:"active"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("parse json: %v (raw=%s)", err, out)
	}
	if got.Active != "home" || len(got.Profiles) != 1 || !got.Profiles[0].Active {
		t.Errorf("unexpected list payload: %+v", got)
	}
}

func TestSlugifyName(t *testing.T) {
	cases := map[string]string{
		"1528 37th Ave E":             "1528-37th-ave-e",
		"990 Lake Whatcom Boulevard":  "990-lake-whatcom-boulevard",
		"  Padded   Spaces  ":         "padded-spaces",
		"a/b/c":                       "a-b-c",
		"!!!":                         "",
		// Truncated to the 40-char cap; tail isn't a dash so no further trim happens.
		"this is a way way way way way way way way way too long for forty characters": "this-is-a-way-way-way-way-way-way-way-wa",
	}
	for in, want := range cases {
		if got := slugifyName(in); got != want {
			t.Errorf("slugifyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRootProfileFlagAppliesOnAppContext(t *testing.T) {
	withTempConfig(t)
	// Seed a profile + a different top-level coord, then verify --profile
	// resolves through newAppContext (we use doctor as a no-network-required
	// AppContext consumer).
	if _, err := runCmd(t, "config", "profiles", "add", "work",
		"--lat", "47.67", "--lon", "-122.12", "--postal", "98052"); err != nil {
		t.Fatalf("add work: %v", err)
	}
	cfg, _ := config.Load()
	cfg.Latitude = 1.0
	cfg.Longitude = 2.0
	cfg.PostalCode = "00000"
	if err := cfg.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}

	// Build an AppContext directly through the helper to keep this test
	// independent of any one command's output. ctx must be non-nil because
	// newAppContext calls context.WithCancel(cmd.Context()).
	root := Root()
	root.SetContext(context.Background())
	if err := root.ParseFlags([]string{"--profile", "work"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	app, err := newAppContext(root)
	if err != nil {
		t.Fatalf("newAppContext: %v", err)
	}
	defer app.Store.Close()
	if app.Cfg.PostalCode != "98052" || app.Cfg.Latitude != 47.67 {
		t.Errorf("--profile work did not apply; cfg=%+v", app.Cfg)
	}
}

func TestRootProfileFlagUnknownFails(t *testing.T) {
	withTempConfig(t)
	root := Root()
	root.SetContext(context.Background())
	if err := root.ParseFlags([]string{"--profile", "nope"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := newAppContext(root); err == nil {
		t.Fatalf("expected error for unknown profile")
	}
}

// --- Codex review P2 follow-ups -------------------------------------------

func TestResolveImportNameIdempotentReimport(t *testing.T) {
	existing := map[string]config.Profile{
		"1528-37th-ave-e": {Name: "1528-37th-ave-e", AddressID: "73256642"},
	}
	a := fetchedAddress{ID: "73256642", StreetAddress: "1528 37th Ave E"}

	name, kind := resolveImportName("1528-37th-ave-e", a, existing, map[string]bool{}, false)
	if name != "1528-37th-ave-e" || kind != "skipped" {
		t.Errorf("re-import without --overwrite: got (%q,%q), want (1528-37th-ave-e, skipped)", name, kind)
	}
	name, kind = resolveImportName("1528-37th-ave-e", a, existing, map[string]bool{}, true)
	if name != "1528-37th-ave-e" || kind != "updated" {
		t.Errorf("re-import with --overwrite: got (%q,%q), want (1528-37th-ave-e, updated)", name, kind)
	}
}

func TestResolveImportNameCollidesWithDifferentAddress(t *testing.T) {
	existing := map[string]config.Profile{
		"home": {Name: "home", AddressID: "111"},
	}
	a := fetchedAddress{ID: "222", StreetAddress: "Home"}
	name, kind := resolveImportName("home", a, existing, map[string]bool{}, false)
	if name != "home-2" || kind != "created" {
		t.Errorf("different-address collision: got (%q,%q), want (home-2, created)", name, kind)
	}
}

func TestResolveImportNameLongPrefixStillFits(t *testing.T) {
	// Long prefix + long slug must still produce a name within the 40-char
	// cap, otherwise SetProfile rejects it and the whole import aborts.
	existing := map[string]config.Profile{}
	a := fetchedAddress{ID: "1", StreetAddress: "1528 37th Avenue East"}
	base := "very-long-prefix-" + slugifyName(a.StreetAddress)
	if len(base) > profileNameMaxLen {
		base = strings.TrimRight(base[:profileNameMaxLen], "-")
	}
	name, kind := resolveImportName(base, a, existing, map[string]bool{}, false)
	if kind != "created" {
		t.Fatalf("expected created, got %q", kind)
	}
	if !config.ValidProfileName(name) {
		t.Errorf("resolveImportName produced an invalid name %q (len=%d)", name, len(name))
	}
}

func TestApplyProfileClearsStaleZoneID(t *testing.T) {
	c := &config.Config{
		Profiles: map[string]config.Profile{
			"a": {Name: "a", ZoneID: "42", PostalCode: "98112"},
			"b": {Name: "b", PostalCode: "98052"}, // no ZoneID
		},
	}
	if err := c.ApplyProfile("a"); err != nil {
		t.Fatalf("apply a: %v", err)
	}
	if c.ZoneID != "42" {
		t.Fatalf("ZoneID after apply(a) = %q, want 42", c.ZoneID)
	}
	if err := c.ApplyProfile("b"); err != nil {
		t.Fatalf("apply b: %v", err)
	}
	if c.ZoneID != "" {
		t.Errorf("ZoneID after apply(b) = %q, want empty (let EffectiveZoneID fall back to default)", c.ZoneID)
	}
}

// TestResolveSetActivePrefersImportedOverExisting covers the Greptile P1:
// when `--use home` is passed but an existing profile already holds the
// name "home" for a different address, the import-loop creates the new
// profile as "home-2"; resolveSetActive must point UseProfile at "home-2"
// rather than letting it silently re-activate the stale "home".
func TestResolveSetActivePrefersImportedOverExisting(t *testing.T) {
	existing := map[string]config.Profile{
		"home": {Name: "home", AddressID: "111", PostalCode: "98112"},
	}
	imported := []importedRef{
		{Name: "home-2", Base: "home"},
	}
	got, warn, err := resolveSetActive("home", imported, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "home-2" {
		t.Errorf("resolveSetActive(home) = %q, want %q", got, "home-2")
	}
	if warn == "" {
		t.Errorf("expected a warning when --use silently rerouted; got empty string")
	}
}

// TestResolveSetActiveExactImportedName covers the happy path: the user
// names the import slug exactly, no collision, no warning.
func TestResolveSetActiveExactImportedName(t *testing.T) {
	imported := []importedRef{{Name: "home", Base: "home"}}
	got, warn, err := resolveSetActive("home", imported, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "home" || warn != "" {
		t.Errorf("got (%q, %q), want (\"home\", \"\")", got, warn)
	}
}

// TestResolveSetActiveFallsBackToExisting: --use may name a pre-existing
// profile that wasn't part of this import run (e.g., import was a top-up).
// That should still succeed.
func TestResolveSetActiveFallsBackToExisting(t *testing.T) {
	existing := map[string]config.Profile{
		"work": {Name: "work", AddressID: "999"},
	}
	imported := []importedRef{{Name: "home", Base: "home"}}
	got, _, err := resolveSetActive("work", imported, existing)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "work" {
		t.Errorf("resolveSetActive(work) = %q, want \"work\"", got)
	}
}

// TestResolveSetActiveAmbiguousBase: two imported rows shouldn't ever share
// the same base in practice (each address has one slug + one suffix path),
// but if it happens the function errors instead of silently picking.
func TestResolveSetActiveAmbiguousBase(t *testing.T) {
	imported := []importedRef{
		{Name: "home-2", Base: "home"},
		{Name: "home-3", Base: "home"},
	}
	_, _, err := resolveSetActive("home", imported, nil)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
}

// TestResolveSetActiveUnknown: nothing matches → error.
func TestResolveSetActiveUnknown(t *testing.T) {
	_, _, err := resolveSetActive("nope", nil, nil)
	if err == nil {
		t.Fatal("expected not-found error, got nil")
	}
}

// seedFakeSession writes a minimal session.json into the temp config dir so
// the import command's `auth.LoadSession()` precondition passes. The
// cookies are inert because tests swap `fetchUserAddressesFn` to a fake
// before any network call.
func seedFakeSession(t *testing.T) {
	t.Helper()
	dir, err := config.Dir()
	if err != nil {
		t.Fatalf("config.Dir: %v", err)
	}
	sess := auth.Session{
		Cookies:   []auth.Cookie{{Name: "__Host-instacart_sid", Value: "fake", Domain: ".instacart.com", Path: "/"}},
		Source:    "test",
		CreatedAt: time.Now(),
	}
	data, _ := json.Marshal(sess)
	if err := os.WriteFile(filepath.Join(dir, "session.json"), data, 0o600); err != nil {
		t.Fatalf("write session: %v", err)
	}
}

// TestProfilesImportPersistsOnBadUse covers the Greptile P1 on PR #643's
// second review: when `profiles import --use <bad-name>` is run, the
// command must persist the imported profiles to disk BEFORE attempting
// the --use activation. Otherwise a typo in --use would silently discard
// every address we just fetched from Instacart, forcing a full re-import.
func TestProfilesImportPersistsOnBadUse(t *testing.T) {
	withTempConfig(t)
	seedFakeSession(t)

	ctx := withFetcher(func(ctx context.Context, sess *auth.Session, cfg *config.Config) ([]fetchedAddress, error) {
		return []fetchedAddress{
			{ID: "111", StreetAddress: "1 Apple St", PostalCode: "98101", Latitude: 47.6, Longitude: -122.3},
			{ID: "222", StreetAddress: "2 Pear Ave", PostalCode: "98102", Latitude: 47.7, Longitude: -122.4},
		}, nil
	})

	_, err := runCmdCtx(t, ctx, "config", "profiles", "import", "--use", "totally-bogus-name")
	if err == nil {
		t.Fatal("expected error from --use of a name that doesn't match any imported or pre-existing profile, got nil")
	}

	// Despite the --use failure, the two fetched addresses must be on disk
	// as profiles (the whole point of the save-before-use ordering).
	got, loadErr := config.Load()
	if loadErr != nil {
		t.Fatalf("reload: %v", loadErr)
	}
	if len(got.Profiles) != 2 {
		t.Fatalf("expected 2 saved profiles after import (despite --use failure), got %d: %v",
			len(got.Profiles), got.ProfileNames())
	}
	if got.ActiveProfile != "" {
		t.Errorf("expected ActiveProfile to remain empty when --use failed, got %q", got.ActiveProfile)
	}
}

// TestProfilesImportActivatesOnGoodUse covers the happy path: --use with a
// resolvable imported name persists the profiles AND activates the named one.
func TestProfilesImportActivatesOnGoodUse(t *testing.T) {
	withTempConfig(t)
	seedFakeSession(t)

	ctx := withFetcher(func(ctx context.Context, sess *auth.Session, cfg *config.Config) ([]fetchedAddress, error) {
		return []fetchedAddress{
			{ID: "111", StreetAddress: "1 Apple St", PostalCode: "98101", Latitude: 47.6, Longitude: -122.3},
		}, nil
	})

	if _, err := runCmdCtx(t, ctx, "config", "profiles", "import", "--use", "1-apple-st"); err != nil {
		t.Fatalf("import --use: %v", err)
	}
	got, _ := config.Load()
	if got.ActiveProfile != "1-apple-st" {
		t.Errorf("ActiveProfile = %q, want %q", got.ActiveProfile, "1-apple-st")
	}
	if got.Latitude == 0 {
		t.Errorf("expected ApplyProfile to have hydrated lat from the imported profile, got 0")
	}
}

// TestProfilesImportBadPrefixIsUsageError covers Greptile P1 on PR #643's
// third review: when `--prefix` introduces an invalid character into the
// resulting slug, SetProfile rejects the name. The CLI must surface that
// as ExitUsage (input is wrong) rather than ExitTransient (which agents
// interpret as "retry, the network is flaky") — otherwise an agent loop
// would spin on the same bad prefix forever.
func TestProfilesImportBadPrefixIsUsageError(t *testing.T) {
	withTempConfig(t)
	seedFakeSession(t)

	ctx := withFetcher(func(ctx context.Context, sess *auth.Session, cfg *config.Config) ([]fetchedAddress, error) {
		return []fetchedAddress{
			{ID: "111", StreetAddress: "1 Apple St", PostalCode: "98101", Latitude: 47.6, Longitude: -122.3},
		}, nil
	})

	// Uppercase + spaces are rejected by ValidProfileName; the prefix
	// drags those characters into the final name regardless of how the
	// slugifier handles the street portion.
	_, err := runCmdCtx(t, ctx, "config", "profiles", "import", "--prefix", "Bad Prefix!")
	if err == nil {
		t.Fatal("expected error from invalid --prefix, got nil")
	}
	ce, ok := err.(CodedError)
	if !ok {
		t.Fatalf("err type=%T, want CodedError", err)
	}
	if ce.Code() != ExitUsage {
		t.Errorf("ExitCode = %d, want ExitUsage (%d) so agents stop retrying", ce.Code(), ExitUsage)
	}
}

