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

func TestReviewFilteredCheckpoint(t *testing.T) {
	for _, resource := range []string{"documents", "books"} {
		for _, full := range []bool{false, true} {
			name := resource + "_incremental"
			if full {
				name = resource + "_full"
			}
			t.Run(name, func(t *testing.T) {
				testenv.Isolate(t)
				db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				key := resource
				if resource == "books" {
					key = "readwise-export"
				}
				before := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
				if err := db.SaveSyncStateAt(key, "original", 7, before); err != nil {
					t.Fatal(err)
				}
				params := &syncUserParams{perResource: map[string]map[string]string{resource: {"location": "new"}}}
				run := func(p *syncUserParams) {
					c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":"next","results":[]}`), json.RawMessage(`{"nextPageCursor":null,"results":[]}`)}}
					if resource == "books" {
						for _, r := range syncReadwiseExport(context.Background(), c, r4EmptyList(), db, "", full, 0, false, p, io.Discard) {
							if r.Err != nil {
								t.Fatal(r.Err)
							}
						}
					} else {
						r := syncReadwiseCursorResource(context.Background(), c, db, resource, "/list/", "", full, 0, false, p, io.Discard, false)
						if r.Err != nil {
							t.Fatal(r.Err)
						}
					}
					if p != nil {
						if len(c.calls) == 0 {
							t.Fatal("no requests")
						}
						if c.calls[0]["pageCursor"] != "" || c.calls[0]["updatedAfter"] != "" {
							t.Fatalf("filtered walk inherited state: %v", c.calls[0])
						}
					}
				}
				run(params)
				cursor, at, count, err := db.GetSyncState(key)
				if err != nil {
					t.Fatal(err)
				}
				if !at.Equal(before) || cursor != "original" || count != 7 {
					t.Fatalf("filtered walk changed checkpoint: %s %s %d", cursor, at, count)
				}
				run(nil)
				_, at, _, _ = db.GetSyncState(key)
				if !at.After(before) {
					t.Fatal("plain walk did not advance")
				}
			})
		}
	}
}
