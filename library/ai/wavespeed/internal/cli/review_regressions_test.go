// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/store"
)

// Regressions from the 4.32.6 reprint review.

// An explicit --config with a colocated data/credentials.toml must have
// set-token write that same file, or the old token stays active.
func TestSetTokenWritesExplicitConfigCredentials(t *testing.T) {
	testenv.Isolate(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.wavespeed.ai/api/v3\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	credPath := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(credPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(credPath, []byte("api_key = \"old-token\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAVESPEED_API_KEY", "")

	cmd := RootCmd()
	cmd.SetArgs([]string{"--config", cfgPath, "auth", "set-token"})
	cmd.SetIn(strings.NewReader("new-token\n"))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token: %v\n%s", err, out.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WavespeedApiKey != "new-token" {
		t.Fatalf("after set-token the active key is %q, want new-token", cfg.WavespeedApiKey)
	}
}

// Earlier releases took the token as a positional argument; scripts that
// still do that must keep working.
func TestSetTokenAcceptsLegacyPositionalToken(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_API_KEY", "")
	cmd := RootCmd()
	cmd.SetArgs([]string{"auth", "set-token", "positional-token"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("set-token <token>: %v\n%s", err, errOut.String())
	}
	if !strings.Contains(errOut.String(), "stdin") {
		t.Fatalf("expected a stdin hint on stderr, got %q", errOut.String())
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WavespeedApiKey != "positional-token" {
		t.Fatalf("active key %q, want positional-token", cfg.WavespeedApiKey)
	}
}

// Releases before the reprint archived into archive.db while sync wrote
// data.db. A leftover archive.db must be merged into data.db even when data.db
// already exists (learning setup creates it first), including rows still only
// in the legacy WAL, and the legacy file is renamed aside afterwards.
func TestArchiveDBPathMergesLegacyArchive(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_ARCHIVE_DB", "")
	current := defaultDBPath("wavespeed-pp-cli")
	legacy := filepath.Join(filepath.Dir(current), "archive.db")

	// data.db already exists with one row of its own.
	cur, err := store.Open(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := cur.Upsert("models", "keep", json.RawMessage(`{"id":"keep","v":"current"}`)); err != nil {
		t.Fatal(err)
	}
	if err := cur.Close(); err != nil {
		t.Fatal(err)
	}
	// The legacy store stays open, so its rows are still in archive.db-wal.
	old, err := store.Open(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := old.Upsert("models", "archived", json.RawMessage(`{"id":"archived"}`)); err != nil {
		t.Fatal(err)
	}
	if err := old.Upsert("models", "keep", json.RawMessage(`{"id":"keep","v":"legacy"}`)); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(legacy + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("expected uncheckpointed legacy WAL: %v", err)
	}

	if got := archiveDBPath(); got != current {
		t.Fatalf("got %q, want %q", got, current)
	}
	merged, err := store.OpenReadOnly(current)
	if err != nil {
		t.Fatal(err)
	}
	defer merged.Close()
	status, err := merged.Status()
	if err != nil {
		t.Fatal(err)
	}
	if status["models"] != 2 {
		t.Fatalf("want both models after merge, got %v", status)
	}
	var data string
	if err := merged.DB().QueryRow(`SELECT data FROM resources WHERE resource_type='models' AND id='keep'`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "current") {
		t.Fatalf("existing data.db row was overwritten: %s", data)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("archive.db still present after merge")
	}
	if _, err := os.Stat(legacy + ".migrated"); err != nil {
		t.Fatalf("archive.db was not kept aside as archive.db.migrated: %v", err)
	}
	// A second call is a no-op.
	if got := archiveDBPath(); got != current {
		t.Fatalf("second call: got %q", got)
	}
}

// An LLM-planner dry run must not try to parse a prediction it never made.
func TestPlanBriefLLMDryRunPreviews(t *testing.T) {
	testenv.Isolate(t)
	flags := &rootFlags{dryRun: true}
	shots, used, warnings, err := planBrief(context.Background(), flags, "llm", planBriefFlags{plannerModel: "wavespeed-ai/any-llm"}, "a red mug")
	if err != nil {
		t.Fatalf("planBrief dry run: %v", err)
	}
	if len(shots) == 0 || !strings.Contains(used, "dry-run") || len(warnings) == 0 {
		t.Fatalf("unexpected preview: shots=%v used=%q warnings=%v", shots, used, warnings)
	}
}

// Logout with an explicit --config removes that config's own credentials
// file, keeps the separate default login, and says that the default login is
// still active for the config. Plain logout clears the default login.
func TestLogoutClearsOnlyTheSelectedLogin(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_API_KEY", "")
	run := func(in string, args ...string) string {
		t.Helper()
		cmd := RootCmd()
		cmd.SetArgs(args)
		cmd.SetIn(strings.NewReader(in))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out.String())
		}
		return out.String()
	}
	keyFor := func(path string) string {
		t.Helper()
		cfg, err := config.Load(path)
		if err != nil {
			t.Fatal(err)
		}
		return cfg.WavespeedApiKey
	}
	run("global-token\n", "auth", "set-token")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.wavespeed.ai/api/v3\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "data", "credentials.toml")
	if err := os.MkdirAll(filepath.Dir(local), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(local, []byte("api_key = \"local-token\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := run("", "--config", cfgPath, "auth", "logout")
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("explicit config credentials file not removed")
	}
	if got := keyFor(cfgPath); got == "local-token" {
		t.Fatalf("explicit config token survived logout")
	}
	if got := keyFor(""); got != "global-token" {
		t.Fatalf("logout of an explicit config removed the default login: got %q", got)
	}
	if !strings.Contains(out, "default login") {
		t.Fatalf("logout did not report that the default login is still active: %q", out)
	}

	out = run("", "auth", "logout")
	if got := keyFor(""); got != "" {
		t.Fatalf("default logout left key %q", got)
	}
	if strings.Contains(out, "default login") {
		t.Fatalf("plain logout should not mention a remaining default login: %q", out)
	}
}
