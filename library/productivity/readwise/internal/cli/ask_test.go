// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"path/filepath"
	"time"
)

// TestNovelAskHelpWires smoke-tests that the ask command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelAskHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"ask", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("ask --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "ask"} {
		if !strings.Contains(help, want) {
			t.Fatalf("ask --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestNovelAppendBlockGroupsByKindAndSource(t *testing.T) {
	blocks := map[string]*novelAskBlock{}
	order := make([]string, 0)

	novelAppendBlock(blocks, &order, "Book A — Author", "highlight", novelSourceItem{Text: "first"})
	novelAppendBlock(blocks, &order, "Book A — Author", "highlight", novelSourceItem{Text: "second"})
	novelAppendBlock(blocks, &order, "Book A — Author", "document", novelSourceItem{Text: "doc"})

	if len(order) != 2 {
		t.Fatalf("order len = %d, want 2", len(order))
	}
	if got := len(blocks["highlight\x00Book A — Author"].Items); got != 2 {
		t.Fatalf("highlight item count = %d, want 2", got)
	}
	if got := len(blocks["document\x00Book A — Author"].Items); got != 1 {
		t.Fatalf("document item count = %d, want 1", got)
	}
}

func seedNovelCommandStore(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	configureDefaultDBScope("")
	dbPath := defaultDBPath("readwise-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer db.Close()
	docs := []json.RawMessage{
		json.RawMessage(`{"id":"doc-1","title":"AI Retrieval","author":"Ada","location":"shortlist","site_name":"Example","word_count":476,"reading_time":"2 mins","reading_progress":0,"saved_at":"2026-06-10T00:00:00Z","created_at":"2026-06-10T00:00:00Z","summary":"Retrieval systems and ranking","url":"https://example.com/retrieval","source_url":"https://example.com/retrieval?utm_source=seed","tags":{"ai":{},"search":{}}}`),
		json.RawMessage(`{"id":"doc-2","title":"Old Unopened","author":"Grace","location":"later","word_count":238,"reading_progress":0,"saved_at":"2025-01-01T00:00:00Z","summary":"Archival systems","url":"https://example.com/old","tags":{"archive":{}}}`),
		json.RawMessage(`{"id":"doc-3","title":"Retrieval Practice","author":"Ada","location":"later","word_count":238,"reading_progress":0,"saved_at":"2026-06-09T00:00:00Z","summary":"Search operations and ranked systems","url":"https://example.com/practice","tags":{"ai":{},"search":{}}}`),
		json.RawMessage(`{"id":"doc-read","title":"Fully Read No Notes","author":"Grace","location":"archive","word_count":238,"reading_progress":1,"saved_at":"2025-01-02T00:00:00Z","summary":"Completed without highlights","url":"https://example.com/read"}`),
		json.RawMessage(`{"id":"doc-1-child","title":"Child Highlight","parent_id":"doc-1","saved_at":"2026-06-11T00:00:00Z"}`),
	}
	books := []json.RawMessage{
		json.RawMessage(`{"id":1,"title":"AI Retrieval","author":"Ada","category":"books","num_highlights":1,"source_url":"http://www.example.com/retrieval/"}`),
		json.RawMessage(`{"id":2,"title":"Empty Source","author":"Grace","category":"articles","num_highlights":0,"source_url":"https://example.com/empty"}`),
	}
	highlights := []json.RawMessage{
		json.RawMessage(strings.ReplaceAll(`{"id":10,"text":"retrieval is context","note":"connect to ranking","book_id":1,"highlighted_at":"2026-06-14T00:00:00Z","created_at":"2026-06-14T00:00:00Z","readwise_url":"https://readwise.io/hl/10"}`, "2026-06-14", time.Now().UTC().Format("2006-01-02"))),
	}
	for i := range docs {
		docs[i] = json.RawMessage(strings.ReplaceAll(string(docs[i]), "2026-06-10", time.Now().UTC().Format("2006-01-02")))
	}
	for resource, items := range map[string][]json.RawMessage{"documents": docs, "books": books, "highlights": highlights} {
		if stored, failed, err := db.UpsertBatch(resource, items); err != nil {
			t.Fatalf("UpsertBatch(%s): %v", resource, err)
		} else if stored != len(items) || failed != 0 {
			t.Fatalf("UpsertBatch(%s) stored=%d failed=%d", resource, stored, failed)
		}
	}
	return filepath.Dir(dbPath)
}

func runNovelCommand(t *testing.T, args ...string) string {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{"--json"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v\noutput: %s", args, err, out.String())
	}
	return out.String()
}

func runNovelAgentCommand(t *testing.T, args ...string) string {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{"--agent"}, args...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v\noutput: %s", args, err, out.String())
	}
	output := out.String()
	if strings.Contains(output, "Usage:") {
		t.Fatalf("%v printed help instead of computed JSON:\n%s", args, output)
	}
	var envelope struct {
		Results json.RawMessage `json:"results"`
	}
	if json.Unmarshal([]byte(output), &envelope) == nil && len(envelope.Results) > 0 {
		return string(envelope.Results)
	}
	return output
}

func runNovelZeroFlagCommand(t *testing.T, args ...string) string {
	t.Helper()
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v failed: %v\noutput: %s", args, err, out.String())
	}
	output := out.String()
	if strings.Contains(output, "Usage:") {
		t.Fatalf("%v printed help instead of computed output:\n%s", args, output)
	}
	if len(strings.TrimSpace(output)) < 20 {
		t.Fatalf("%v output too short, want computed command output:\n%s", args, output)
	}
	return output
}

func TestNovelBareReadCommandsComputeWithAgent(t *testing.T) {
	seedNovelCommandStore(t)

	t.Run("backlog", func(t *testing.T) {
		var view novelBacklogView
		if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "backlog")), &view); err != nil {
			t.Fatalf("backlog output is not JSON: %v", err)
		}
		if view.Totals.UnreadCount == 0 || len(view.ByLocation) == 0 {
			t.Fatalf("backlog view = %+v, want computed unread summary", view)
		}
	})

	t.Run("digest", func(t *testing.T) {
		var view novelDigestView
		if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "digest")), &view); err != nil {
			t.Fatalf("digest output is not JSON: %v", err)
		}
		if view.Count == 0 || len(view.Groups) == 0 {
			t.Fatalf("digest view = %+v, want computed default 7d digest", view)
		}
	})

	t.Run("dedupe", func(t *testing.T) {
		var view novelDedupeView
		if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "dedupe")), &view); err != nil {
			t.Fatalf("dedupe output is not JSON: %v", err)
		}
		if view.DuplicateCount == 0 || len(view.Clusters) == 0 {
			t.Fatalf("dedupe view = %+v, want computed duplicate clusters", view)
		}
	})

	t.Run("read-next", func(t *testing.T) {
		var items []novelReadNextItem
		if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "read-next")), &items); err != nil {
			t.Fatalf("read-next output is not JSON: %v", err)
		}
		if len(items) == 0 || len(items) > 5 {
			t.Fatalf("read-next items = %+v, want default top recommendations", items)
		}
	})

	t.Run("triage-stale", func(t *testing.T) {
		var view novelTriageStaleView
		if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "triage", "stale")), &view); err != nil {
			t.Fatalf("triage stale output is not JSON: %v", err)
		}
		if len(view.StaleDocuments) == 0 {
			t.Fatalf("triage stale documents = 0, want bare command to default --never-opened")
		}
		if len(view.OrphanSources) == 0 {
			t.Fatalf("triage orphan sources = 0, want bare command to default --zero-highlights")
		}
	})
}

func TestNovelBareReadCommandsComputeZeroFlags(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		marker string
	}{
		{name: "backlog", args: []string{"backlog"}, marker: `"unread_count"`},
		{name: "digest", args: []string{"digest"}, marker: "retrieval is context"},
		{name: "dedupe", args: []string{"dedupe"}, marker: `"duplicate_count"`},
		{name: "read-next", args: []string{"read-next"}, marker: "recently saved"},
		{name: "triage-stale", args: []string{"triage", "stale"}, marker: `"stale_documents"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seedNovelCommandStore(t)
			out := runNovelZeroFlagCommand(t, tt.args...)
			if !strings.Contains(out, tt.marker) {
				t.Fatalf("%v output missing computed marker %q:\n%s", tt.args, tt.marker, out)
			}
		})
	}
}

func TestNovelTriageStaleExplicitNeverOpenedFalseIsRespected(t *testing.T) {
	seedNovelCommandStore(t)
	var view novelTriageStaleView
	if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "triage", "stale", "--never-opened=false")), &view); err != nil {
		t.Fatalf("triage stale output is not JSON: %v", err)
	}
	if len(view.StaleDocuments) != 0 {
		t.Fatalf("stale documents = %+v, want none when --never-opened=false is explicit and --zero-highlights is unset", view.StaleDocuments)
	}
}

func TestDoctorCacheRowsUseActualResourceCounts(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("READWISE_API_TOKEN", "")
	if err := os.Unsetenv("READWISE_API_TOKEN"); err != nil {
		t.Fatal(err)
	}
	// Doctor always probes reachability, even without credentials.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/" {
			t.Errorf("unexpected health path: %s", r.URL)
		}
		w.Write([]byte("{}"))
	}))
	t.Cleanup(server.Close)
	t.Setenv("READWISE_BASE_URL", server.URL)

	db, err := store.OpenWithContext(context.Background(), defaultDBPath("readwise-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if stored, failed, err := db.UpsertBatch("documents", []json.RawMessage{
		json.RawMessage(`{"id":"doc-a","title":"A"}`),
		json.RawMessage(`{"id":"doc-b","title":"B"}`),
		json.RawMessage(`{"id":"doc-c","title":"C"}`),
	}); err != nil || stored != 3 || failed != 0 {
		t.Fatalf("UpsertBatch documents stored=%d failed=%d err=%v", stored, failed, err)
	}
	if _, err := db.DB().Exec(
		`INSERT INTO sync_state(resource_type, last_synced_at, total_count) VALUES (?, ?, ?)`,
		"documents", time.Now().UTC(), 0,
	); err != nil {
		t.Fatalf("seed sync_state: %v", err)
	}
	_ = db.Close()

	var report struct {
		Cache struct {
			Resources []struct {
				Type string `json:"type"`
				Rows int    `json:"rows"`
			} `json:"resources"`
		} `json:"cache"`
	}
	if err := json.Unmarshal([]byte(runNovelAgentCommand(t, "doctor")), &report); err != nil {
		t.Fatalf("doctor output is not JSON: %v", err)
	}
	for _, resource := range report.Cache.Resources {
		if resource.Type == "documents" {
			if resource.Rows != 3 {
				t.Fatalf("doctor documents rows = %d, want actual table count 3 instead of sync_state.total_count 0", resource.Rows)
			}
			return
		}
	}
	t.Fatalf("doctor cache resources = %+v, want documents entry", report.Cache.Resources)
}

func TestNovelCommandsUseSeededTempStore(t *testing.T) {
	seedNovelCommandStore(t)
	commands := []struct {
		name   string
		args   []string
		assert func(t *testing.T, out string)
	}{
		{name: "ask", args: []string{"ask", "retrieval"}, assert: func(t *testing.T, out string) {
			var view novelAskView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("ask output is not JSON: %v\n%s", err, out)
			}
			if len(view.Blocks) == 0 {
				t.Fatalf("ask blocks = 0, want seeded retrieval result")
			}
		}},
		{name: "backlog", args: []string{"backlog", "--budget", "5m"}},
		{name: "dedupe", args: []string{"dedupe", "--by", "url", "--scope", "docs,books"}, assert: func(t *testing.T, out string) {
			var view novelDedupeView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("dedupe output is not JSON: %v\n%s", err, out)
			}
			if view.DuplicateCount == 0 || len(view.Clusters) == 0 || view.Clusters[0].Confidence != "high" {
				t.Fatalf("dedupe view = %+v, want high-confidence duplicate", view)
			}
		}},
		{name: "read-next", args: []string{"read-next", "--have", "5m", "--prefer", "shortlist"}},
		{name: "digest", args: []string{"digest", "--since", "2026-01-01"}, assert: func(t *testing.T, out string) {
			var view novelDigestView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("digest output is not JSON: %v\n%s", err, out)
			}
			if view.Count == 0 {
				t.Fatalf("digest count = 0, want seeded highlight")
			}
		}},
		{name: "connections", args: []string{"connections", "--seed-doc", "doc-1"}, assert: func(t *testing.T, out string) {
			var view novelConnectionsView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("connections output is not JSON: %v\n%s", err, out)
			}
			if len(view.Matches) == 0 || view.Matches[0].ID != "doc-3" {
				t.Fatalf("connections matches = %+v, want doc-3", view.Matches)
			}
		}},
		{name: "triage", args: []string{"triage", "stale", "--older-than", "180d", "--never-opened", "--zero-highlights"}},
		{name: "mirror", args: []string{"mirror", "--export-dir", t.TempDir(), "--verify"}},
	}
	for _, tc := range commands {
		t.Run(tc.name, func(t *testing.T) {
			out := runNovelCommand(t, tc.args...)
			if tc.assert != nil {
				tc.assert(t, out)
				return
			}
			var decoded any
			if err := json.Unmarshal([]byte(out), &decoded); err != nil {
				t.Fatalf("output is not JSON: %v\n%s", err, out)
			}
		})
	}
}

func TestNovelCommandsEmptyStoreGrace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	db, err := store.OpenWithContext(context.Background(), defaultDBPath("readwise-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	_ = db.Close()

	tests := []struct {
		name string
		args []string
		note func(string) string
	}{
		{name: "backlog", args: []string{"backlog"}, note: func(out string) string {
			var view novelBacklogView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("backlog output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},
		{name: "ask", args: []string{"ask", "anything", "--include", "highlights"}, note: func(out string) string {
			var view novelAskView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("ask output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},
		{name: "triage-stale", args: []string{"triage", "stale"}, note: func(out string) string {
			var view novelTriageStaleView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("triage output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if note := tt.note(runNovelCommand(t, tt.args...)); note == "" {
				t.Fatalf("%s note empty, want sync-first note", tt.name)
			}
		})
	}
}

func TestNovelReadOnlyCommandsMissingStoreGrace(t *testing.T) {
	missingDB := filepath.Join(t.TempDir(), "never-synced", "data.db")
	tests := []struct {
		name string
		args []string
		note func(string) string
	}{
		{name: "backlog", args: []string{"backlog", "--db-path", missingDB}, note: func(out string) string {
			var view novelBacklogView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("backlog output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},

		{name: "ask", args: []string{"ask", "anything", "--db-path", missingDB}, note: func(out string) string {
			var view novelAskView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("ask output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},
		{name: "triage-stale", args: []string{"triage", "stale", "--db-path", missingDB}, note: func(out string) string {
			var view novelTriageStaleView
			if err := json.Unmarshal([]byte(out), &view); err != nil {
				t.Fatalf("triage output is not JSON: %v\n%s", err, out)
			}
			return view.Note
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := runNovelCommand(t, tt.args...)
			if note := tt.note(out); note == "" {
				t.Fatalf("%s note empty, want sync-first note", tt.name)
			}
		})
	}
}

func TestNovelAskDocumentNotesFallbackDoesNotDuplicateNote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	db, err := store.OpenWithContext(context.Background(), defaultDBPath("readwise-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if stored, failed, err := db.UpsertBatch("documents", []json.RawMessage{
		json.RawMessage(`{"id":"note-doc","title":"Note Doc","notes":"unique fallback prose","url":"https://example.com/note"}`),
	}); err != nil || stored != 1 || failed != 0 {
		t.Fatalf("UpsertBatch documents stored=%d failed=%d err=%v", stored, failed, err)
	}
	_ = db.Close()

	var view novelAskView
	if err := json.Unmarshal([]byte(runNovelCommand(t, "ask", "unique", "--include", "docs")), &view); err != nil {
		t.Fatal(err)
	}
	if len(view.Blocks) != 1 || len(view.Blocks[0].Items) != 1 {
		t.Fatalf("ask view = %+v, want one document item", view)
	}
	if view.Blocks[0].Items[0].Text != "unique fallback prose" || view.Blocks[0].Items[0].Note != "" {
		t.Fatalf("ask item = %+v, want notes as text without duplicate note", view.Blocks[0].Items[0])
	}
}

func TestE3RuntimeDBPath(t *testing.T) {
	t.Cleanup(func() { _, _ = cliutil.SetHomeOverride("") })
	seedNovelCommandStore(t)
	target := t.TempDir()
	for _, args := range [][]string{{"ask", "retrieval"}, {"digest"}, {"mirror", "--verify"}} {
		cmd := RootCmd()
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{"--home", target, "--json"}, args...))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), "sync' first") {
			t.Fatalf("--home ignored: %s", out.String())
		}
	}
}

func TestE3TokenAllocation(t *testing.T) {
	if got := strings.Join(novelTokens("Retrieval retrieval systems 2026"), ","); got != "retrieval,systems" {
		t.Fatal(got)
	}
	allocs := testing.AllocsPerRun(100, func() { novelTokens("retrieval systems") })
	if allocs > 15 {
		t.Fatalf("token regexp recompiled: %.0f allocations", allocs)
	}
}
