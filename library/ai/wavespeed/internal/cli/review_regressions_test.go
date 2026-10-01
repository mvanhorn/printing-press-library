// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/config"
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
// data.db. An upgrade with only archive.db must move it to data.db so the
// archived data stays visible to status, sync and pricing.
func TestArchiveDBPathMigratesLegacyArchive(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_ARCHIVE_DB", "")
	current := defaultDBPath("wavespeed-pp-cli")
	legacy := filepath.Join(filepath.Dir(current), "archive.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("archived"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := archiveDBPath(); got != current {
		t.Fatalf("got %q, want %q", got, current)
	}
	data, err := os.ReadFile(current)
	if err != nil || string(data) != "archived" {
		t.Fatalf("archive.db was not moved to data.db: %q %v", data, err)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatalf("archive.db still present after migration")
	}
	// With both present, data.db wins and archive.db is left alone.
	if err := os.WriteFile(legacy, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := archiveDBPath(); got != current {
		t.Fatalf("both present: got %q, want %q", got, current)
	}
	if data, _ := os.ReadFile(current); string(data) != "archived" {
		t.Fatalf("data.db was overwritten: %q", data)
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

// Logout with an explicit --config must not leave the global credentials file
// for Load to fall back to.
func TestLogoutClearsExplicitAndGlobalCredentials(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_API_KEY", "")
	global := RootCmd()
	global.SetArgs([]string{"auth", "set-token"})
	global.SetIn(strings.NewReader("global-token\n"))
	global.SetOut(&bytes.Buffer{})
	global.SetErr(&bytes.Buffer{})
	if err := global.Execute(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("base_url = \"https://api.wavespeed.ai/api/v3\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "data"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "data", "credentials.toml"), []byte("api_key = \"local-token\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	logout := RootCmd()
	logout.SetArgs([]string{"--config", cfgPath, "auth", "logout"})
	var out bytes.Buffer
	logout.SetOut(&out)
	logout.SetErr(&out)
	if err := logout.Execute(); err != nil {
		t.Fatalf("logout: %v\n%s", err, out.String())
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.WavespeedApiKey != "" {
		t.Fatalf("still authenticated after logout with key %q", cfg.WavespeedApiKey)
	}
}
