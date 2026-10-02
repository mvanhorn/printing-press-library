// Copyright 2026 Abdelrahman Shaaban and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/rundown/internal/store"
)

// TestNovelUseCasesHelpWires smoke-tests that the use-cases command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelUseCasesHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"use-cases", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("use-cases --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "use-cases"} {
		if !strings.Contains(help, want) {
			t.Fatalf("use-cases --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestRecordUseCaseCandidate_PreservesLivePayloadOnLocalDuplicate(t *testing.T) {
	merged := map[string]*rdUseCaseRow{}
	byID := map[string]rdPost{}
	live := rdPost{ID: "post-1", Title: "Current title", UpvoteCount: 7}
	staleLocal := rdPost{ID: "post-1", Title: "Stale title", UpvoteCount: 999}

	rdRecordUseCaseCandidate(merged, byID, live, "semantic")
	rdRecordUseCaseCandidate(merged, byID, staleLocal, "local")

	if got := byID["post-1"]; got.Title != live.Title || got.UpvoteCount != live.UpvoteCount {
		t.Fatalf("duplicate local hit replaced live payload: got %+v, want %+v", got, live)
	}
	if got := merged["post-1"].MatchedVia; got != "both" {
		t.Fatalf("MatchedVia = %q, want both", got)
	}
}

func TestRecordUseCaseCandidate_LocalOnlySuppliesPayload(t *testing.T) {
	merged := map[string]*rdUseCaseRow{}
	byID := map[string]rdPost{}
	local := rdPost{ID: "post-2", Title: "Local only", UpvoteCount: 3}

	rdRecordUseCaseCandidate(merged, byID, local, "local")

	if got := byID["post-2"]; got.Title != local.Title {
		t.Fatalf("local-only payload = %+v, want %+v", got, local)
	}
	if got := merged["post-2"].MatchedVia; got != "local" {
		t.Fatalf("MatchedVia = %q, want local", got)
	}
}

func TestUseCasesLocal_MissingMirrorExplainsHowToSync(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing", "data.db")
	stdout, stderr, err := runRootArgs(t,
		"use-cases", "invoice reconciliation", "--local", "--db", dbPath, "--agent", "--no-learn",
	)
	if err != nil {
		t.Fatalf("use-cases --local exited non-zero: %v (stderr=%q)", err, stderr)
	}
	var got rdUseCaseResult
	unmarshalAgentResults(t, stdout, &got)
	if !strings.Contains(got.Note, "rundown-pp-cli sync") {
		t.Fatalf("note = %q, want explicit sync instruction", got.Note)
	}
}

func TestUseCasesLocal_CorruptMirrorReportsOpenError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(dbPath, []byte("this is not a SQLite database"), 0o600); err != nil {
		t.Fatalf("write corrupt mirror: %v", err)
	}

	stdout, stderr, err := runRootArgs(t,
		"use-cases", "invoice reconciliation", "--local", "--db", dbPath, "--agent", "--no-learn",
	)
	if err == nil {
		t.Fatalf("use-cases --local unexpectedly accepted corrupt mirror (stdout=%q stderr=%q)", stdout, stderr)
	}
	combined := stdout + stderr + err.Error()
	if !strings.Contains(combined, "opening database") || !strings.Contains(combined, "not a database") {
		t.Fatalf("error output = %q, want the SQLite open failure", combined)
	}
	if strings.Contains(combined, "rundown-pp-cli sync") {
		t.Fatalf("error output = %q, should not label a corrupt mirror as missing", combined)
	}
}

func TestUseCasesAuto_ReturnsErrorWhenLiveAndLocalSearchFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "semantic search unavailable", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("RUNDOWN_BASE_URL", server.URL)

	dbPath := filepath.Join(t.TempDir(), "broken-search.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open mirror: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close mirror: %v", err)
	}

	originalLocalSearch := rdUseCasesLocalSearch
	rdUseCasesLocalSearch = func(*store.Store, string, int) ([]rdPost, error) {
		return nil, errors.New("forced local search failure")
	}
	t.Cleanup(func() { rdUseCasesLocalSearch = originalLocalSearch })

	stdout, stderr, err := runRootArgs(t,
		"use-cases", "invoice reconciliation", "--db", dbPath, "--agent", "--no-learn",
	)
	if err == nil {
		t.Fatalf("use-cases unexpectedly hid both search failures (stdout=%q stderr=%q)", stdout, stderr)
	}
	combined := stdout + stderr + err.Error()
	for _, want := range []string{"semantic search failed", "local mirror search failed"} {
		if !strings.Contains(combined, want) {
			t.Fatalf("error output = %q, want %q", combined, want)
		}
	}
}
