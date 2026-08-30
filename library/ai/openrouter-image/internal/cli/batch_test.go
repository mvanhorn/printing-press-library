// Copyright 2026 neal-kyle and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	_ "modernc.org/sqlite"
)

// TestNovelBatchHelpWires smoke-tests that the batch command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBatchHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"batch", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("batch --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "batch"} {
		if !strings.Contains(help, want) {
			t.Fatalf("batch --help missing %q in output:\n%s", want, help)
		}
	}
}

// fakeImageAPI serves a fake OpenRouter /images endpoint returning one
// base64 image with a usage cost, so batch execution can run without any
// real API. Tracks request count for the caller.
func fakeImageAPI(t *testing.T, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits != nil {
			hits.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprintln(w, `{"created":1723000000,"data":[{"b64_json":"aGVsbG8=","media_type":"image/png"}],"usage":{"cost":0.02,"total_tokens":10}}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// batchTestEnv writes a one-row CSV spec, a minimal config pointing at the
// fake API, and returns the spec and store paths. --home redirects all
// default state (learn store, caches) into the temp dir.
func batchTestEnv(t *testing.T, srv *httptest.Server) (specPath, dbPath string) {
	t.Helper()
	dir := cliTestEnv(t, srv)
	specPath = filepath.Join(dir, "batch.csv")
	if err := os.WriteFile(specPath, []byte("prompt,model\n\"a red panda astronaut\",openai/gpt-image-1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return specPath, filepath.Join(dir, "store.db")
}

// cliTestEnv writes a minimal config pointing at the fake API, sets the
// env overrides, and returns the temp dir used by the test.
func cliTestEnv(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(cfgPath, []byte("api_key = \"test-key\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENROUTER_IMAGE_CONFIG", cfgPath)
	t.Setenv("OPENROUTER_IMAGE_BASE_URL", srv.URL)
	t.Setenv("OPENROUTER_API_KEY", "test-key")
	return dir
}

// breakLedgerWrite makes every generation_ledger INSERT fail at runtime
// while leaving reads fully functional: a BEFORE INSERT trigger aborts the
// write, deterministically simulating a persistence failure after a billed
// generation.
func breakLedgerWrite(t *testing.T, dbPath string) {
	t.Helper()
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	defer raw.Close()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS generation_ledger (
			id TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			prompt TEXT,
			params TEXT,
			cost_usd REAL,
			tokens TEXT,
			output_path TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TRIGGER IF NOT EXISTS reject_generation_ledger_insert
		 BEFORE INSERT ON generation_ledger
		 BEGIN SELECT RAISE(ABORT, 'simulated ledger write failure'); END`,
	}
	for _, stmt := range stmts {
		if _, err := raw.Exec(stmt); err != nil {
			t.Fatalf("exec %q: %v", stmt, err)
		}
	}
}

// TestNovelBatchLedgerFailureFailsRun is the regression test for the review
// finding "ledger failures are ignored": when a billed generation's ledger
// write fails, the batch must fail the run (non-zero exit) so the missing
// cost history cannot be silently swallowed by automation, and so
// regenerate cannot lose the generation.
func TestNovelBatchLedgerFailureFailsRun(t *testing.T) {
	var hits atomic.Int32
	srv := fakeImageAPI(t, &hits)
	specPath, dbPath := batchTestEnv(t, srv)

	// Break the ledger table before the run: create generation_ledger
	// WITHOUT the params column so the INSERT OR REPLACE in
	// LedgerGeneration fails at runtime, deterministically simulating a
	// persistence failure after a billed generation. Table and index
	// creation still succeed, so the batch reaches the ledger write.
	raw, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw sqlite: %v", err)
	}
	if _, err := raw.Exec(`CREATE TABLE generation_ledger (
		id TEXT PRIMARY KEY,
		model TEXT NOT NULL,
		prompt TEXT,
		cost_usd REAL,
		tokens TEXT,
		output_path TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		t.Fatalf("create broken ledger table: %v", err)
	}
	raw.Close()

	cmd := RootCmd()
	cmd.SetArgs([]string{"batch", "--spec", specPath, "--db", dbPath, "--home", filepath.Dir(dbPath), "--agent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err = cmd.Execute()
	if err == nil {
		t.Fatalf("batch with failing ledger write returned nil error; output:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "ledger") {
		t.Fatalf("error %q does not mention the ledger", err)
	}
	if !strings.Contains(err.Error(), "row 1") {
		t.Fatalf("error %q does not identify the row", err)
	}
	if hits.Load() == 0 {
		t.Fatalf("fake API server never hit; output:\n%s", out.String())
	}
	// The row-level error must still be present in the emitted JSON.
	if !strings.Contains(out.String(), "ledger:") {
		t.Fatalf("row ledger error missing from output:\n%s", out.String())
	}
	if code := ExitCode(err); code == 0 {
		t.Fatalf("ExitCode(%v) = 0, want non-zero", err)
	}
}

// TestNovelBatchSuccessRecordsLedger guards the healthy path used by the
// failure test: with a working store the same batch succeeds, reports the
// row as generated, and records a ledger id.
func TestNovelBatchSuccessRecordsLedger(t *testing.T) {
	var hits atomic.Int32
	srv := fakeImageAPI(t, &hits)
	specPath, dbPath := batchTestEnv(t, srv)

	cmd := RootCmd()
	cmd.SetArgs([]string{"batch", "--spec", specPath, "--db", dbPath, "--home", filepath.Dir(dbPath), "--agent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("batch with healthy store error = %v; output:\n%s", err, out.String())
	}
	if hits.Load() == 0 {
		t.Fatalf("fake API server never hit; output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "generated") {
		t.Fatalf("expected generated row in output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "ledger_id") {
		t.Fatalf("expected ledger_id in output:\n%s", out.String())
	}
}
