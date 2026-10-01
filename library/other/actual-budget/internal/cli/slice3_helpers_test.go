// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Shared helpers for the local-mirror command tests (ledger, report, audit,
// categorize, templates, changes, import-csv).

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil/testenv"
)

const s3SyncID = "fixture-budget"

// s3Fixture sandboxes the test, points the data dir at a temp dir and builds
// the fixture budget mirror there.
func s3Fixture(t *testing.T) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	t.Setenv("ACTUAL_SYNC_ID", s3SyncID)
	path, err := actual.DBPath(s3SyncID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := actualtest.Build(path); err != nil {
		t.Fatal(err)
	}
}

// s3NoMirror sandboxes the test with a sync id but no pulled mirror.
func s3NoMirror(t *testing.T) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("ACTUAL_BUDGET_DATA_DIR", t.TempDir())
	t.Setenv("ACTUAL_SYNC_ID", s3SyncID)
}

// s3Invoke executes the real root command with the output-mode flag (e.g.
// --json) and --max-age 0 appended, returning stdout, stderr and the error.
func s3Invoke(t *testing.T, mode string, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	cmd.SetArgs(append(append([]string{}, args...), mode, "--max-age", "0"))
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// s3Run executes the real root command with --json.
func s3Run(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	return s3Invoke(t, "--json", args...)
}

// s3RunHuman executes the real root command with --human-friendly (human
// tables).
func s3RunHuman(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	t.Cleanup(func() { humanFriendly = false })
	return s3Invoke(t, "--human-friendly", args...)
}

// s3RunJSON runs the command, requires success, and decodes stdout into v.
func s3RunJSON(t *testing.T, v any, args ...string) string {
	t.Helper()
	out, stderr, err := s3Run(t, args...)
	if err != nil {
		t.Fatalf("%v: %v\nstdout: %s\nstderr: %s", args, err, out, stderr)
	}
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("%v: decoding JSON: %v\n%s", args, err, out)
	}
	return out
}

// s3ExitCode returns the CLI exit code an error maps to (0 for nil).
func s3ExitCode(err error) int {
	if err == nil {
		return 0
	}
	return ExitCode(err)
}

// s3IDs collects the "id" field of each row, sorted.
func s3IDs(rows []map[string]any) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r["id"].(string))
	}
	sort.Strings(ids)
	return ids
}

// s3Exec applies SQL statements to the fixture mirror built by s3Fixture.
func s3Exec(t *testing.T, stmts ...string) {
	t.Helper()
	path, err := actual.DBPath(s3SyncID)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			t.Fatalf("%s: %v", s, err)
		}
	}
}

// s3Request is one request recorded by the fake sidecar.
type s3Request struct {
	Method string
	Path   string
	APIKey string
	Body   map[string]any
}

// s3Sidecar starts an httptest server standing in for the actual-http-api
// sidecar, points ACTUAL_BUDGET_BASE_URL / ACTUAL_HTTP_API_KEY at it, and
// records every request. statusFor picks the response status per path
// (nil = 200).
func s3Sidecar(t *testing.T, statusFor func(path string) int) func() []s3Request {
	t.Helper()
	var mu sync.Mutex
	var reqs []s3Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		rec := s3Request{Method: r.Method, Path: r.URL.Path, APIKey: r.Header.Get("x-api-key")}
		_ = json.Unmarshal(data, &rec.Body)
		mu.Lock()
		reqs = append(reqs, rec)
		mu.Unlock()
		status := http.StatusOK
		if statusFor != nil {
			status = statusFor(r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status >= 300 {
			_, _ = w.Write([]byte(`{"error":"rejected by test sidecar"}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"added":["new-1"],"updated":[],"errors":[]}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("ACTUAL_BUDGET_BASE_URL", srv.URL)
	t.Setenv("ACTUAL_HTTP_API_KEY", "test-key")
	return func() []s3Request {
		mu.Lock()
		defer mu.Unlock()
		return append([]s3Request(nil), reqs...)
	}
}
