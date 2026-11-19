package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"path/filepath"
	"testing"
	"time"
)

func TestReviewBooksListCapCheckpoint(t *testing.T) {
	for _, next := range []string{"page2", ""} {
		name := "terminal"
		if next != "" {
			name = "capped"
		}
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			before := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
			for _, key := range []string{"readwise-export", "books", "highlights"} {
				if err := db.SaveSyncStateAt(key, "", 0, before); err != nil {
					t.Fatal(err)
				}
			}
			export := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":null,"results":[]}`)}}
			page, _ := json.Marshal(map[string]any{"next": next, "results": []map[string]any{{"id": 1, "title": "One"}}})
			books := &stubReadwiseGetter{responses: []json.RawMessage{page}}
			for _, res := range syncReadwiseExport(context.Background(), export, books, db, "", false, 1, false, nil, io.Discard) {
				if res.Err != nil {
					t.Fatal(res.Err)
				}
			}
			for _, key := range []string{"readwise-export", "books", "highlights"} {
				_, at, _, err := db.GetSyncState(key)
				if err != nil {
					t.Fatal(err)
				}
				if next != "" && !at.Equal(before) {
					t.Fatalf("%s capped pass advanced checkpoint: %s", key, at)
				}
				if next == "" && !at.After(before) {
					t.Fatalf("%s terminal pass did not complete: %s", key, at)
				}
				var complete int
				if err := db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type=?`, key).Scan(&complete); err != nil {
					t.Fatal(err)
				}
				if (complete == 1) != (next == "") {
					t.Fatalf("complete=%d next=%q", complete, next)
				}
			}
			if next != "" {
				export.responses = []json.RawMessage{json.RawMessage(`{"nextPageCursor":null,"results":[]}`)}
				books.responses = []json.RawMessage{page, json.RawMessage(`{"next":"page3","results":[{"id":2}]}`), json.RawMessage(`{"next":null,"results":[{"id":3}]}`)}
				for _, res := range syncReadwiseExport(context.Background(), export, books, db, "", false, 0, false, nil, io.Discard) {
					if res.Err != nil {
						t.Fatal(res.Err)
					}
				}
				_, at, _, _ := db.GetSyncState("readwise-export")
				if !at.After(before) {
					t.Fatal("uncapped pass did not advance")
				}
			}
		})
	}
}
