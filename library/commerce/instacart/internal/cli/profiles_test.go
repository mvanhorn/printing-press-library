package cli

// PATCH (instacart-address-profiles): tests for `config profiles` subtree and
// `--profile` per-call override. Stays in-package so we can drive the cobra
// command tree directly and assert on persisted state through config.Load().

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/instacart/internal/store"
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
	root := Root()
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	return buf.String(), err
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

// silence unused-import warnings when go vet runs against this file alone.
var _ = filepath.Join
var _ = store.OpenAt
