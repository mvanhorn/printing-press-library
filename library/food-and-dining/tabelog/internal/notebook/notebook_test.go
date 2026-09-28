package notebook

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// This checks real SQLite transaction and connection behavior, not a mocked query sequence.
func TestNotebookPreservesUserDataAndRollsBackSnapshotBatch(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "notebook.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	n := New(db)
	if err := n.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := n.Add(ctx, "should-not-exist", "99999999", "", false); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("unfetched ID: %v", err)
	}
	lists, err := n.Lists(ctx)
	if err != nil || len(lists) != 0 {
		t.Fatalf("failed add created a list: %+v, %v", lists, err)
	}
	old := json.RawMessage(`{"id":"13005012","name":"SAMBOA BAR Ginza ten","url":"https://tabelog.com/en/tokyo/A1301/A130101/13005012/","rating":3.79,"hours":"old hours"}`)
	if err := n.ReplaceSnapshot(ctx, "13005012", old); err != nil {
		t.Fatal(err)
	}
	if err := n.Add(ctx, "Tokyo bars", "13005012", "Personal note", true); err != nil {
		t.Fatal(err)
	}
	if err := n.Add(ctx, "Tokyo bars", "13005012", "", false); err != nil {
		t.Fatal(err)
	}
	entries, err := n.Entries(ctx, "Tokyo bars", nil)
	if err != nil || len(entries) != 1 || entries[0].Note != "Personal note" {
		t.Fatalf("repeat add lost the note: %+v, %v", entries, err)
	}
	// A second query on the one-connection DB proves Entries returned with rows closed.
	if _, err := n.Snapshot(ctx, "13005012"); err != nil {
		t.Fatalf("rows were not drained: %v", err)
	}
	if _, err := db.ExecContext(ctx, `CREATE TRIGGER fixture_reject_snapshot BEFORE INSERT ON tabelog_snapshots WHEN NEW.id='13294162' BEGIN SELECT RAISE(ABORT,'fixture rejection'); END`); err != nil {
		t.Fatal(err)
	}
	updated := json.RawMessage(`{"id":"13005012","name":"SAMBOA BAR Ginza ten","url":"https://tabelog.com/en/tokyo/A1301/A130101/13005012/","rating":3.80}`)
	rejected := json.RawMessage(`{"id":"13294162","name":"Sushi Dokoro Isseki Sanchou","url":"https://tabelog.com/en/tokyo/A1301/A130103/13294162/"}`)
	if err := n.ReplaceSnapshots(ctx, map[string]json.RawMessage{"13005012": updated, "13294162": rejected}); err == nil {
		t.Fatal("fixture SQL rejection was ignored")
	}
	after, err := n.Snapshot(ctx, "13005012")
	if err != nil || string(after) != string(old) {
		t.Fatalf("partial batch overwrote old facts: %s, %v", after, err)
	}
	if _, err := n.Snapshot(ctx, "13294162"); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("failed batch retained a new snapshot: %v", err)
	}
	if err := n.ReplaceSnapshot(ctx, "13005012", updated); err != nil {
		t.Fatal(err)
	}
	entries, err = n.Entries(ctx, "Tokyo bars", []string{"13005012", "13005012"})
	if err != nil || len(entries) != 1 || entries[0].Note != "Personal note" {
		t.Fatalf("refresh changed user notes: %+v, %v", entries, err)
	}
	var facts map[string]any
	if err := json.Unmarshal(entries[0].Snapshot, &facts); err != nil {
		t.Fatal(err)
	}
	if _, exists := facts["hours"]; exists {
		t.Fatal("full snapshot replacement silently retained old hours")
	}
	if err := n.Note(ctx, "Tokyo bars", "13005012", "Edited note"); err != nil {
		t.Fatal(err)
	}
	if err := n.Remove(ctx, "Tokyo bars", "13005012"); err != nil {
		t.Fatal(err)
	}
	entries, err = n.Entries(ctx, "Tokyo bars", nil)
	if err != nil || len(entries) != 0 {
		t.Fatalf("remove failed: %+v, %v", entries, err)
	}
	if _, err := n.Snapshot(ctx, "13005012"); err != nil {
		t.Fatalf("membership removal deleted source facts: %v", err)
	}
}

func TestUnsavedSnapshotBudgetProtectsListMembers(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	n := New(db)
	if err := n.Init(ctx); err != nil {
		t.Fatal(err)
	}
	payload := strings.Repeat("x", 3*1024*1024)
	snapshot := func(id string) json.RawMessage {
		raw, err := json.Marshal(map[string]any{"id": id, "name": "Fixture venue", "url": "https://tabelog.com/en/tokyo/A1301/A130101/" + id + "/", "source_fact": payload})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if err := n.ReplaceSnapshot(ctx, "13005012", snapshot("13005012")); err != nil {
		t.Fatal(err)
	}
	if err := n.Add(ctx, "protected", "13005012", "Keep this note", true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		id := fmt.Sprintf("190000%02d", i)
		if err := n.ReplaceSnapshot(ctx, id, snapshot(id)); err != nil {
			t.Fatal(err)
		}
	}
	var bytes int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(length(CAST(data AS BLOB))),0) FROM tabelog_snapshots WHERE id<>'13005012'`).Scan(&bytes); err != nil {
		t.Fatal(err)
	}
	if bytes > unsavedSnapshotBudget {
		t.Fatalf("unsaved snapshots exceed payload budget: %d", bytes)
	}
	if _, err := n.Snapshot(ctx, "19000000"); !errors.Is(err, ErrSnapshotNotFound) {
		t.Fatalf("oldest unsaved snapshot was not evicted: %v", err)
	}
	entries, err := n.Entries(ctx, "protected", nil)
	if err != nil || len(entries) != 1 || entries[0].Note != "Keep this note" {
		t.Fatalf("retention changed protected membership: %+v, %v", entries, err)
	}
}
