// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// autoRefreshAPIHandler serves the full public-API surface the auto-refresh
// path touches: the folders list (a defaultSyncResources() entry), the notes
// list, and per-note detail.
func autoRefreshAPIHandler(t *testing.T, ids []string, detail map[string]func(http.ResponseWriter)) http.HandlerFunc {
	t.Helper()
	notes := notesListHandler(t, ids, detail)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/folders" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"folders": []map[string]any{{"id": "folder_roadmap", "name": "Roadmap"}},
				"hasMore": false,
				"cursor":  "",
			})
			return
		}
		notes(w, r)
	}
}

// newAutoRefreshEnv is newHydrateEnv plus the two knobs the auto-refresh
// dispatcher reads: an absent Granola support dir (so the cache surface is
// out of the plan and the API surface is the only one under test) and the
// env opt-out explicitly cleared.
func newAutoRefreshEnv(t *testing.T, h http.HandlerFunc) *rootFlags {
	t.Helper()
	flags, _ := newHydrateEnv(t, h)
	t.Setenv("GRANOLA_SUPPORT_DIR", filepath.Join(t.TempDir(), "no-such-granola-dir"))
	t.Setenv("GRANOLA_NO_AUTO_REFRESH", "")
	return flags
}

// captureProcessStdout swaps os.Stdout for a pipe while fn runs and returns
// everything written to it. The generated sync path writes to the process
// stdout directly rather than to cobra's out writer, so nothing short of
// capturing the file descriptor proves the auto-refresh hook stayed quiet.
func captureProcessStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	prev := os.Stdout
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	defer func() {
		os.Stdout = prev
		_ = r.Close()
	}()
	fn()
	_ = w.Close()
	return <-done
}

// TestRunApiSync_HydratesDomainTables is the Finding 1 regression: the
// auto-refresh API path used to run the LIST stage only, which writes the
// generic resources/notes tables that no read command queries. It reported
// "api=ok (N rows)" while `meetings`, `attendees`, `transcript_segments`, and
// `folder_memberships` — the store every read command actually serves from —
// stayed empty forever.
func TestRunApiSync_HydratesDomainTables(t *testing.T) {
	flags := newAutoRefreshEnv(t, autoRefreshAPIHandler(t,
		[]string{"note_alpha", "note_gamma"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeJSON(fixtureNoteAlphaDetail),
			"note_gamma": writeJSON(fixtureNoteGammaDetail),
		}))

	var res ApiSyncResult
	var err error
	_ = captureProcessStdout(t, func() {
		res, err = runApiSync(context.Background(), flags)
	})
	if err != nil {
		t.Fatalf("runApiSync: %v", err)
	}

	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM attendees WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM transcript_segments WHERE meeting_id='note_alpha'`, 2)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM folder_memberships WHERE meeting_id='note_alpha'`, 1)

	// The list stage still ran — this is a chain, not a replacement.
	assertQueryCount(t, db, `SELECT COUNT(*) FROM resources WHERE resource_type='notes'`, 2)

	// The provenance count must reflect what landed in the domain tables,
	// not just the list rows the generic tables received.
	if res.DomainRecords == 0 {
		t.Error("DomainRecords = 0; the provenance line would report a list-only row count")
	}
	if res.TotalRows() <= res.TotalRecords {
		t.Errorf("TotalRows() = %d, want > TotalRecords (%d)", res.TotalRows(), res.TotalRecords)
	}
}

// TestRunApiSync_HydrateFailureIsReported is the other half of Finding 1: a
// detail-stage failure must not read as "api=ok". Auto-refresh captures the
// returned error into its stderr fragment, so returning nil here is what
// produced the false positive on migrated installs.
func TestRunApiSync_HydrateFailureIsReported(t *testing.T) {
	flags := newAutoRefreshEnv(t, autoRefreshAPIHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeStatus(http.StatusInternalServerError, `{"error":"boom"}`),
		}))

	var res ApiSyncResult
	var err error
	_ = captureProcessStdout(t, func() {
		res, err = runApiSync(context.Background(), flags)
	})
	if err == nil {
		t.Fatal("a failing detail stage must surface an error so the provenance line shows api=failed")
	}
	if res.DomainRecords != 0 {
		t.Errorf("DomainRecords = %d, want 0 when the detail stage failed", res.DomainRecords)
	}
}

// TestAutoRefresh_WritesNothingToStdout is the Finding 2 regression:
// syncResource wrote its ndjson progress stream straight to the process
// stdout whenever --human-friendly was off (the default, and the documented
// agent-safe mode). Because auto-refresh fires from PersistentPreRunE on
// nearly every command, `show <id> --json` with an API key configured emitted
// sync ndjson ahead of its real payload and broke strict JSON consumers.
func TestAutoRefresh_WritesNothingToStdout(t *testing.T) {
	flags := newAutoRefreshEnv(t, autoRefreshAPIHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeJSON(fixtureNoteAlphaDetail),
		}))
	flags.asJSON = true

	prev := stderrIsTerminal
	stderrIsTerminal = func() bool { return false }
	t.Cleanup(func() { stderrIsTerminal = prev })

	cmd := &cobra.Command{Use: "show"}
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	out := captureProcessStdout(t, func() { runAutoRefreshImpl(cmd, flags) })

	if strings.Contains(out, `{"event":"sync_`) {
		t.Errorf("auto-refresh leaked sync ndjson onto stdout:\n%s", out)
	}
	if out != "" {
		t.Errorf("auto-refresh must write nothing to stdout, got:\n%s", out)
	}

	// Sanity: the refresh really did run (otherwise this test proves nothing).
	db := openHydratedStore(t)
	assertQueryCount(t, db, `SELECT COUNT(*) FROM meetings WHERE id='note_alpha'`, 1)
}

// TestSyncApiCommand_KeepsNdjsonOnStdout pins the other side of the Finding 2
// contract: the explicit sync surfaces are parsed by agents today, so making
// auto-refresh quiet must not make `sync-api` quiet.
func TestSyncApiCommand_KeepsNdjsonOnStdout(t *testing.T) {
	newAutoRefreshEnv(t, autoRefreshAPIHandler(t,
		[]string{"note_alpha"},
		map[string]func(http.ResponseWriter){
			"note_alpha": writeJSON(fixtureNoteAlphaDetail),
		}))

	rc := RootCmd()
	rc.SetOut(&bytes.Buffer{})
	rc.SetErr(&bytes.Buffer{})
	rc.SetArgs([]string{"sync-api"})

	var execErr error
	out := captureProcessStdout(t, func() { execErr = rc.Execute() })
	if execErr != nil {
		t.Fatalf("sync-api: %v", execErr)
	}
	for _, want := range []string{
		`{"event":"sync_start","resource":"notes"}`,
		`{"event":"sync_complete","resource":"notes"`,
		`{"event":"sync_summary"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("sync-api stdout missing %s:\n%s", want, out)
		}
	}
}

// TestSyncResource_DefaultStreamsStillWriteStdout covers the generated
// entry point directly. `sync` and `sync-api` both reach syncResource with
// its default streams; if that wrapper ever starts defaulting to a quiet
// writer, both commands go silent without any test noticing.
func TestSyncResource_DefaultStreamsStillWriteStdout(t *testing.T) {
	flags := newAutoRefreshEnv(t, autoRefreshAPIHandler(t, nil, nil))
	c, err := flags.newClient()
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	db, err := openGranolaStore(context.Background())
	if err != nil {
		t.Fatalf("openGranolaStore: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	out := captureProcessStdout(t, func() {
		syncResource(c, db, "folders", "", false, 1, false, nil)
	})
	if !strings.Contains(out, `{"event":"sync_start","resource":"folders"}`) {
		t.Errorf("syncResource must keep writing ndjson to stdout by default, got:\n%s", out)
	}
}
