package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR3BooksListCompletionAtomic(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveReadwiseSyncProgress("readwise-export", "", 0, "since"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReadwiseChainStart("readwise-export", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, "since"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`CREATE TRIGGER reject_complete BEFORE UPDATE ON sync_state WHEN NEW.last_attempt_complete=1 BEGIN SELECT RAISE(ABORT,'checkpoint rejected'); END`); err != nil {
		t.Fatal(err)
	}
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"next":null}`)}}
	var events bytes.Buffer
	results := syncReadwiseExport(context.Background(), c, c, db, "", false, 1, false, nil, &events)
	if !strings.Contains(events.String(), `"sync_error"`) {
		t.Fatalf("missing error event: %s", events.String())
	}
	if results[0].Err == nil {
		t.Fatal("expected checkpoint failure")
	}
	cursor, _, _, _ := db.GetSyncState("readwise-books-list")
	if cursor != "3:1000" {
		t.Fatalf("continuation lost: %q", cursor)
	}
}

func TestR3BooksListRequiresClient(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SaveReadwiseSyncProgress("readwise-export", "", 0, "since")
	db.ReadwiseChainStart("readwise-export", false, time.Now())
	db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, "since")
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"nextPageCursor":null}`)}}
	before := r2bSyncStateSnapshot(t, db)
	results := syncReadwiseExport(context.Background(), c, nil, db, "", false, 1, false, nil, io.Discard)
	if results[0].Err == nil {
		t.Fatal("nil list client completed")
	}
	cursor, _, _, _ := db.GetSyncState("readwise-books-list")
	if cursor != "3:1000" {
		t.Fatalf("lost list continuation: %q; before=%s", cursor, before)
	}
	var complete int
	if err := db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type='readwise-export'`).Scan(&complete); err != nil || complete != 0 {
		t.Fatalf("completed=%d error=%v", complete, err)
	}

}

func TestR3ListResumePreservesExportCounts(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, _, _, err := upsertResourceBatch(db, "books", []json.RawMessage{[]byte(`{"id":1}`), []byte(`{"id":2}`), []byte(`{"id":3}`), []byte(`{"id":4}`), []byte(`{"id":5}`)}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseExportCheckpoint("", "since", 9, 5, 4, time.Now(), false); err != nil {
		t.Fatal(err)
	}
	db.ReadwiseChainStart("readwise-export", false, time.Now())
	db.SaveReadwiseSyncProgress("readwise-books-list", "2:1000", 2, "since")
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"next":null}`)}}
	for _, r := range syncReadwiseExport(context.Background(), c, c, db, "", false, 1, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	for key, want := range map[string]int{"readwise-export": 9, "highlights": 4, "books": 5} {
		_, _, got, err := db.GetSyncState(key)
		if err != nil || got != want {
			t.Fatalf("%s count=%d want=%d error=%v", key, got, want, err)
		}
	}
}
