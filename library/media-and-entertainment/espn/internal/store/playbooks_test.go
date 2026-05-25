package store

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "playbooks.db")
	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertPlaybook_Happy(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	id, inserted, err := s.UpsertPlaybook(UpsertPlaybookInput{
		QueryFamily:  "doing end led ppg rpg season spg",
		PlaybookJSON: `{"steps":[{"cmd":"teams basketball nba {team.id}"}]}`,
		NotesText:    "byathlete needs seasontype=2; categories has dup labels",
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if !inserted {
		t.Errorf("first insert should report inserted=true")
	}
	if id <= 0 {
		t.Errorf("want positive id, got %d", id)
	}

	row, ok, err := s.GetPlaybookByFamily("doing end led ppg rpg season spg")
	if err != nil || !ok {
		t.Fatalf("get: err=%v ok=%v", err, ok)
	}
	if !strings.Contains(row.PlaybookJSON, "byathlete") && !strings.Contains(row.PlaybookJSON, "teams basketball") {
		t.Errorf("playbook content mismatch: %q", row.PlaybookJSON)
	}
	if !strings.Contains(row.NotesText, "seasontype=2") {
		t.Errorf("notes content mismatch: %q", row.NotesText)
	}
}

func TestUpsertPlaybook_Idempotent(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	for i := 0; i < 3; i++ {
		_, inserted, err := s.UpsertPlaybook(UpsertPlaybookInput{
			QueryFamily:  "fam-x",
			PlaybookJSON: `{"steps":[]}`,
			NotesText:    "v" + string(rune('a'+i)),
		})
		if err != nil {
			t.Fatalf("upsert %d: %v", i, err)
		}
		if i == 0 && !inserted {
			t.Errorf("first call should insert")
		}
		if i > 0 && inserted {
			t.Errorf("call %d should update not insert", i)
		}
	}

	rows, err := s.ListPlaybooks()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	famXCount := 0
	for _, r := range rows {
		if r.QueryFamily == "fam-x" {
			famXCount++
		}
	}
	if famXCount != 1 {
		t.Errorf("want 1 row for fam-x, got %d", famXCount)
	}
}

func TestUpsertPlaybook_RejectsEmptyContent(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	_, _, err := s.UpsertPlaybook(UpsertPlaybookInput{
		QueryFamily: "fam-empty",
	})
	if err == nil {
		t.Fatal("expected error when both playbook_json and notes_text are empty")
	}
}

func TestUpsertPlaybook_RejectsEmptyFamily(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	_, _, err := s.UpsertPlaybook(UpsertPlaybookInput{
		QueryFamily: "  ",
		NotesText:   "nope",
	})
	if err == nil {
		t.Fatal("expected error when query_family is empty/whitespace")
	}
}

func TestUpsertPlaybook_NotesOnly(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	_, _, err := s.UpsertPlaybook(UpsertPlaybookInput{
		QueryFamily: "fam-notes-only",
		NotesText:   "remember the seasontype hack",
	})
	if err != nil {
		t.Fatalf("notes-only upsert should succeed: %v", err)
	}
	row, ok, _ := s.GetPlaybookByFamily("fam-notes-only")
	if !ok {
		t.Fatal("row missing")
	}
	if row.PlaybookJSON != "" {
		t.Errorf("playbook_json should be empty; got %q", row.PlaybookJSON)
	}
	if row.NotesText == "" {
		t.Errorf("notes_text should be populated")
	}
}

func TestUpsertPlaybook_Concurrent(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = s.UpsertPlaybook(UpsertPlaybookInput{
				QueryFamily: "race-fam",
				NotesText:   "concurrent",
			})
		}()
	}
	wg.Wait()

	rows, _ := s.ListPlaybooks()
	count := 0
	for _, r := range rows {
		if r.QueryFamily == "race-fam" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("concurrent upserts should produce exactly 1 row, got %d", count)
	}
}

func TestGetPlaybookByFamily_NotFound(t *testing.T) {
	t.Parallel()
	s := openTestStore(t)
	_, ok, err := s.GetPlaybookByFamily("never-taught")
	if err != nil {
		t.Fatalf("get nonexistent: %v", err)
	}
	if ok {
		t.Error("ok should be false for missing family")
	}
}

func TestPlaybooksTable_LegacyDBHeal(t *testing.T) {
	t.Parallel()
	// New store creates the table from scratch (the table didn't exist
	// before this plan landed, so any pre-existing user DB will hit this
	// path on first Open after upgrade). Verify the table exists post-Open.
	s := openTestStore(t)
	var name string
	err := s.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='learning_playbooks'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("learning_playbooks table should exist after Open: %v", err)
	}
	if name != "learning_playbooks" {
		t.Errorf("got table name %q", name)
	}
}
