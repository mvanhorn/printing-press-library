// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Behavior tests for the monitor diff / entity report / webset new local
// store commands.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/exa/internal/store"
)

// seedTestStore writes a fresh store at the resolved DB path with the given
// rows so the local-only novel commands have data to read.
func seedTestStore(t *testing.T, seed func(db *store.Store)) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	dbPath := defaultDBPath("exa-pp-cli")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	defer db.Close()
	seed(db)
}

func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	var out, errb bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errb)
	err := cmd.Execute()
	return out.String(), errb.String(), err
}

// TestMonitorDiffLocal compares two seeded runs and reports the URL delta.
func TestMonitorDiffLocal(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		// Seed the monitor row so the sync hint does not claim "never synced".
		_ = db.UpsertMonitors(json.RawMessage(`{"id":"mon-1","status":"active"}`))
		// Earlier run: urls A, B. Later run: urls B, C.
		_ = db.UpsertRuns(json.RawMessage(`{
			"id":"run-1","monitors_id":"mon-1","status":"completed",
			"output":{"results":[{"url":"https://a.example"},{"url":"https://b.example"}]}
		}`))
		_ = db.UpsertRuns(json.RawMessage(`{
			"id":"run-2","monitors_id":"mon-1","status":"completed",
			"output":{"results":[{"url":"https://b.example"},{"url":"https://c.example"}]}
		}`))
	})

	out, _, err := runCLI(t, "monitor", "diff", "mon-1", "--json")
	if err != nil {
		t.Fatalf("monitor diff error: %v", err)
	}
	var view struct {
		Added        []string `json:"added"`
		Removed      []string `json:"removed"`
		KeptCount    int      `json:"keptCount"`
		AddedCount   int      `json:"addedCount"`
		RemovedCount int      `json:"removedCount"`
		TotalTo      int      `json:"totalTo"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if len(view.Added) != 1 || view.Added[0] != "https://c.example" {
		t.Fatalf("unexpected added: %+v", view.Added)
	}
	if len(view.Removed) != 1 || view.Removed[0] != "https://a.example" {
		t.Fatalf("unexpected removed: %+v", view.Removed)
	}
	if view.KeptCount != 1 || view.TotalTo != 2 {
		t.Fatalf("unexpected counts: %+v", view)
	}
}

// TestMonitorDiffExplicitRuns lets --from/--to select specific runs.
func TestMonitorDiffExplicitRuns(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertMonitors(json.RawMessage(`{"id":"mon-2","status":"active"}`))
		_ = db.UpsertRuns(json.RawMessage(`{"id":"run-a","monitors_id":"mon-2","status":"completed","output":{"results":[{"url":"https://x.example"}]}}`))
		_ = db.UpsertRuns(json.RawMessage(`{"id":"run-b","monitors_id":"mon-2","status":"completed","output":{"results":[{"url":"https://x.example"},{"url":"https://y.example"}]}}`))
	})
	out, _, err := runCLI(t, "monitor", "diff", "mon-2", "--from", "run-a", "--to", "run-b", "--json")
	if err != nil {
		t.Fatalf("monitor diff error: %v", err)
	}
	var view struct {
		FromRunID string   `json:"fromRunId"`
		ToRunID   string   `json:"toRunId"`
		Added     []string `json:"added"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if view.FromRunID != "run-a" || view.ToRunID != "run-b" {
		t.Fatalf("run ids not honored: %s", out)
	}
	if len(view.Added) != 1 || view.Added[0] != "https://y.example" {
		t.Fatalf("added url missing: %s", out)
	}
}

// TestEntityReportLocal finds a company mentioned in seeded items + runs.
func TestEntityReportLocal(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertWebsets(json.RawMessage(`{"id":"ws-1","title":"competitors"}`))
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-1","websets_id":"ws-1","entity":{"type":"company","id":"acme","name":"Acme Corp","url":"https://acme.example"}}`))
		_ = db.UpsertRuns(json.RawMessage(`{"id":"run-1","monitors_id":"mon-1","status":"completed","output":{"results":[{"url":"https://acme.example","title":"Acme Corp funding round"}]}}`))
	})

	out, _, err := runCLI(t, "entity", "report", "Acme Corp", "--json")
	if err != nil {
		t.Fatalf("entity report error: %v", err)
	}
	var view struct {
		Entity      string `json:"entity"`
		Mentions    int    `json:"mentionCount"`
		WebsetItems int    `json:"websetItemMentions"`
		MonitorRuns int    `json:"monitorRunMentions"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if view.Entity != "Acme Corp" {
		t.Fatalf("unexpected entity: %+v", view)
	}
	if view.WebsetItems != 1 || view.MonitorRuns != 1 || view.Mentions != 2 {
		t.Fatalf("unexpected counts: %+v", view)
	}
}

// TestEntityReportNoMatch reports zero mentions cleanly.
func TestEntityReportNoMatch(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertWebsets(json.RawMessage(`{"id":"ws-1","title":"competitors"}`))
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-1","websetId":"ws-1","entity":{"type":"company","id":"other","name":"Other Inc","url":"https://other.example"}}`))
	})
	out, _, err := runCLI(t, "entity", "report", "Nonexistent", "--json")
	if err == nil {
		t.Fatal("expected not-found exit for unknown entity")
	}
	var view struct {
		Mentions int `json:"mentionCount"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if view.Mentions != 0 {
		t.Fatalf("expected zero mentions: %s", out)
	}
}

// TestWebsetNewLocal lists items synced within the window.
func TestWebsetNewLocal(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertWebsets(json.RawMessage(`{"id":"ws-1","title":"competitors"}`))
		// Two items: both seeded "now" so they fall inside the default 7d window.
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-1","websets_id":"ws-1","entity":{"type":"company","id":"acme","name":"Acme Corp","url":"https://acme.example"}}`))
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-2","websets_id":"ws-1","entity":{"type":"company","id":"beta","name":"Beta Labs","url":"https://beta.example"}}`))
		// Item in another webset must not leak in.
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-9","websets_id":"ws-other","entity":{"type":"company","id":"z","name":"Zed","url":"https://z.example"}}`))
	})

	out, _, err := runCLI(t, "webset", "new", "ws-1", "--json")
	if err != nil {
		t.Fatalf("webset new error: %v", err)
	}
	var view struct {
		WebsetID string `json:"websetId"`
		Count    int    `json:"addedCount"`
		Items    []struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if view.WebsetID != "ws-1" {
		t.Fatalf("unexpected webset: %+v", view)
	}
	if view.Count != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", view.Count, view.Items)
	}
	for _, it := range view.Items {
		if it.ID == "item-9" {
			t.Fatalf("item from other webset leaked in: %+v", view.Items)
		}
	}
}

// TestWebsetNewRespectsSinceWindow excludes items synced before the window
// and rejects negative durations.
func TestWebsetNewRespectsSinceWindow(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertWebsets(json.RawMessage(`{"id":"ws-1","title":"competitors"}`))
		_ = db.UpsertItems(json.RawMessage(`{"id":"item-old","websets_id":"ws-1","entity":{"type":"company","id":"old","name":"Old Inc","url":"https://old.example"}}`))
		// Backdate the item's synced_at to 30 days ago. Storage ids carry a
		// parent suffix, so update by the webset-scoped row directly.
		oldTS := time.Now().Add(-30 * 24 * time.Hour).UTC().Format(time.RFC3339)
		if _, err := db.DB().Exec(`UPDATE items SET synced_at = ? WHERE websets_id = 'ws-1'`, oldTS); err != nil {
			t.Fatalf("backdating item: %v", err)
		}
	})

	_, _, err := runCLI(t, "webset", "new", "ws-1", "--since", "-1h", "--json")
	if err == nil {
		t.Fatal("expected error for negative --since")
	}

	// 7d window: the 30-day-old item is excluded (exit 3 not-found).
	out, _, err := runCLI(t, "webset", "new", "ws-1", "--since", "7d", "--json")
	if err == nil {
		t.Fatal("expected not-found exit for zero new items")
	}
	var view struct {
		Count int `json:"addedCount"`
	}
	_ = json.Unmarshal([]byte(out), &view)
	if view.Count != 0 {
		t.Fatalf("expected 0 items outside 7d window, got %d: %s", view.Count, out)
	}
}

// TestNovelLocalCommandsRejectLiveDataSource verifies the --data-source live
// rejection for all four local-only commands.
func TestNovelLocalCommandsRejectLiveDataSource(t *testing.T) {
	cases := [][]string{
		{"spend", "--data-source", "live"},
		{"monitor", "diff", "mon-1", "--data-source", "live"},
		{"entity", "report", "Acme", "--data-source", "live"},
		{"webset", "new", "ws-1", "--data-source", "live"},
	}
	for _, args := range cases {
		_, errb, err := runCLI(t, args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
		if !strings.Contains(errb, "no live equivalent") {
			t.Fatalf("%v: expected 'no live equivalent' rejection, stderr: %s", args, errb)
		}
	}
}

// TestMonitorDiffSameRunNoPanic verifies --from and --to pointing at the same
// run cannot escape the runs slice (regression for an index-out-of-range
// panic when both resolve to the oldest run).
func TestMonitorDiffSameRunNoPanic(t *testing.T) {
	seedTestStore(t, func(db *store.Store) {
		_ = db.UpsertMonitors(json.RawMessage(`{"id":"mon-3","status":"active"}`))
		_ = db.UpsertRuns(json.RawMessage(`{"id":"run-1","monitors_id":"mon-3","status":"completed","output":{"results":[{"url":"https://a.example"}]}}`))
		_ = db.UpsertRuns(json.RawMessage(`{"id":"run-2","monitors_id":"mon-3","status":"completed","output":{"results":[{"url":"https://b.example"}]}}`))
	})
	// Both flags point at the newest run; the fallback must pick an in-bounds neighbor.
	out, errb, err := runCLI(t, "monitor", "diff", "mon-3", "--from", "run-2", "--to", "run-2", "--json")
	if err != nil {
		t.Fatalf("monitor diff error: %v (stderr: %s)", err, errb)
	}
	var view struct {
		FromRunID string `json:"fromRunId"`
		ToRunID   string `json:"toRunId"`
	}
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if view.ToRunID != "run-2" || view.FromRunID != "run-1" {
		t.Fatalf("expected from=run-1 to=run-2, got %+v", view)
	}
}

// TestParseHumanDurationStrict rejects trailing garbage and non-positive values.
func TestParseHumanDurationStrict(t *testing.T) {
	good := map[string]time.Duration{
		"7d":  7 * 24 * time.Hour,
		"30d": 30 * 24 * time.Hour,
		"24h": 24 * time.Hour,
		"90m": 90 * time.Minute,
	}
	for in, want := range good {
		got, err := parseHumanDuration(in)
		if err != nil {
			t.Fatalf("parseHumanDuration(%q) error: %v", in, err)
		}
		if got != want {
			t.Fatalf("parseHumanDuration(%q) = %v, want %v", in, got, want)
		}
	}
	for _, bad := range []string{"1d2h", "-24h", "-1d", "abc", "7", "d", "0d", "0h"} {
		if _, err := parseHumanDuration(bad); err == nil {
			t.Fatalf("parseHumanDuration(%q) expected error, got nil", bad)
		}
	}
}

// TestCoerceBoolOrCompositeStrict rejects garbage and skips empty input.
func TestCoerceBoolOrCompositeStrict(t *testing.T) {
	if v, err := coerceBoolOrComposite(""); err != nil || v != nil {
		t.Fatalf("empty: v=%v err=%v, want nil,nil", v, err)
	}
	if v, err := coerceBoolOrComposite("true"); err != nil || v != true {
		t.Fatalf("true: v=%v err=%v", v, err)
	}
	if v, err := coerceBoolOrComposite("false"); err != nil || v != false {
		t.Fatalf("false: v=%v err=%v", v, err)
	}
	if v, err := coerceBoolOrComposite(`{"maxCharacters": 100}`); err != nil || v == nil {
		t.Fatalf("object: v=%v err=%v", v, err)
	}
	if _, err := coerceBoolOrComposite("banana"); err == nil {
		t.Fatal("garbage input expected error")
	}
}

var _ = time.Now
