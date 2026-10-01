// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"database/sql"
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
// data.db. The writers merge a leftover archive.db into data.db even when
// data.db already exists, including rows still only in the legacy WAL, index
// them for search, keep existing rows, and never move or delete archive.db,
// so a later commit by an older process still open on it merges next run.
// The read-only path resolver never writes.
func TestArchiveDBPathMergesLegacyArchive(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_ARCHIVE_DB", "")
	current := defaultDBPath("wavespeed-pp-cli")
	legacy := filepath.Join(filepath.Dir(current), "archive.db")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o700); err != nil {
		t.Fatal(err)
	}

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
	// An older process keeps archive.db open, so its rows sit in the WAL.
	old, err := store.Open(legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := old.Upsert("models", "archived", json.RawMessage(`{"id":"archived","name":"zebracorn"}`)); err != nil {
		t.Fatal(err)
	}
	if err := old.Upsert("models", "keep", json.RawMessage(`{"id":"keep","v":"legacy"}`)); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(legacy + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("expected uncheckpointed legacy WAL: %v", err)
	}

	before, _ := os.Stat(current)
	if got := archiveDBPath(); got != current {
		t.Fatalf("got %q, want %q", got, current)
	}
	if after, _ := os.Stat(current); !after.ModTime().Equal(before.ModTime()) || !legacyArchivePending(context.Background()) {
		t.Fatalf("read-only resolver must not merge or write")
	}

	if got := archiveDBPathForWrite(context.Background()); got != current {
		t.Fatalf("write path: got %q", got)
	}
	countModels := func() (int, *store.Store) {
		t.Helper()
		s, err := store.OpenReadOnly(current)
		if err != nil {
			t.Fatal(err)
		}
		status, err := s.Status()
		if err != nil {
			t.Fatal(err)
		}
		return status["models"], s
	}
	n, merged := countModels()
	if n != 2 {
		t.Fatalf("want both models after merge, got %d", n)
	}
	var data string
	if err := merged.DB().QueryRow(`SELECT data FROM resources WHERE resource_type='models' AND id='keep'`).Scan(&data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(data, "current") {
		t.Fatalf("existing data.db row was overwritten: %s", data)
	}
	hits, err := merged.Search("zebracorn", 10)
	if err != nil || len(hits) != 1 {
		t.Fatalf("merged row not searchable: %d hits, %v", len(hits), err)
	}
	merged.Close()
	if _, err := os.Stat(legacy); err != nil {
		t.Fatalf("archive.db must be left in place: %v", err)
	}
	if legacyArchivePending(context.Background()) {
		t.Fatalf("merge not recorded")
	}

	// The older process commits again after the merge (same size, same
	// mtime granularity does not matter): the next writer run merges it.
	if err := old.Upsert("models", "late", json.RawMessage(`{"id":"late"}`)); err != nil {
		t.Fatal(err)
	}
	if !legacyArchivePending(context.Background()) {
		t.Fatalf("a legacy commit after the merge must be reported as pending")
	}
	archiveDBPathForWrite(context.Background())
	if legacyArchivePending(context.Background()) {
		t.Fatalf("still pending after the second merge")
	}
	n, merged = countModels()
	merged.Close()
	if n != 3 {
		t.Fatalf("late legacy commit lost: %d models", n)
	}
}

// The legacy archive is attached by URI; a path containing '?', '#' or '%'
// must still name the file rather than be read as URI syntax.
func TestLegacyArchiveURIEscapesPathSyntax(t *testing.T) {
	plain := filepath.Join(t.TempDir(), "archive.db")
	src, err := store.Open(plain)
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Upsert("models", "m", json.RawMessage(`{"id":"m"}`)); err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "odd?dir#x%20y")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(plain)
	if err != nil {
		t.Fatal(err)
	}
	odd := filepath.Join(dir, "archive.db")
	if err := os.WriteFile(odd, data, 0o600); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`ATTACH DATABASE ? AS legacy`, legacyArchiveURI(odd)); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM legacy.resources`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("attached wrong file: n=%d err=%v uri=%s", n, err, legacyArchiveURI(odd))
	}
}

// workflow status must flag a legacy archive whose only missing rows are in
// a typed table (here model_pricing), not just generic resources.
func TestLegacyArchivePendingCoversTypedTables(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_ARCHIVE_DB", "")
	current := defaultDBPath("wavespeed-pp-cli")
	legacy := filepath.Join(filepath.Dir(current), "archive.db")
	for _, path := range []string{current, legacy} {
		s, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Upsert("models", "m", json.RawMessage(`{"id":"m"}`)); err != nil {
			t.Fatal(err)
		}
		if path == legacy {
			if _, err := s.DB().Exec(`INSERT INTO model_pricing (id, data) VALUES ('model_pricing:x', '{}')`); err != nil {
				t.Fatal(err)
			}
		}
		if err := s.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !legacyArchivePending(context.Background()) {
		t.Fatalf("missing legacy pricing row not reported")
	}
	archiveDBPathForWrite(context.Background())
	if legacyArchivePending(context.Background()) {
		t.Fatalf("still pending after merge")
	}
}

// Older archives keyed resources on id alone. A legacy resource missing from
// data.db must still be reported pending even though the declared keys differ.
func TestLegacyArchivePendingCoversOlderResourcesKey(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("WAVESPEED_ARCHIVE_DB", "")
	current := defaultDBPath("wavespeed-pp-cli")
	s, err := store.Open(current)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(filepath.Dir(current), "archive.db")
	db, err := sql.Open("sqlite", legacy)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE resources (id TEXT PRIMARY KEY, resource_type TEXT NOT NULL, data JSON NOT NULL)`,
		`INSERT INTO resources (id, resource_type, data) VALUES ('old', 'models', '{"id":"old"}')`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if !legacyArchivePending(context.Background()) {
		t.Fatalf("legacy resource under the older key not reported")
	}
	archiveDBPathForWrite(context.Background())
	if legacyArchivePending(context.Background()) {
		t.Fatalf("still pending after merge")
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

// Logout with an explicit config (here via WAVESPEED_CONFIG) removes that config's own credentials
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
	t.Setenv("WAVESPEED_CONFIG", cfgPath)
	out := run("", "auth", "logout")
	t.Setenv("WAVESPEED_CONFIG", "")
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
		t.Fatalf("logout via WAVESPEED_CONFIG did not report that the default login is still active: %q", out)
	}

	out = run("", "auth", "logout")
	if got := keyFor(""); got != "" {
		t.Fatalf("default logout left key %q", got)
	}
	if strings.Contains(out, "default login") {
		t.Fatalf("plain logout should not mention a remaining default login: %q", out)
	}
}
