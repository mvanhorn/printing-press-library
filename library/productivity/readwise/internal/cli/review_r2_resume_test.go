package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestR2BooksListCappedResume(t *testing.T) {
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
	exports := 0
	pages := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/export/" {
			exports++
			io.WriteString(w, `{"results":[],"nextPageCursor":null}`)
			return
		}
		if got := r.URL.Query().Get("updated__gt"); got != before.Format(time.RFC3339) {
			t.Errorf("list since = %q", got)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		next := `"more"`
		if page == "3" {
			next = "null"
		}
		fmt.Fprintf(w, `{"results":[{"id":%s}],"next":%s}`, page, next)
	}))
	defer srv.Close()
	c := r2HTTPGetter{srv.URL, srv.Client()}
	for run := 1; run <= 3; run++ {
		for _, r := range syncReadwiseExport(context.WithValue(context.Background(), readwiseRunStartKey{}, before.Add(time.Duration(run)*time.Hour)), c, c, db, "", false, 1, false, nil, io.Discard) {
			if r.Err != nil {
				t.Fatal(r.Err)
			}
		}
		if exports != 1 || len(pages) != 2*run-1 || pages[len(pages)-1] != fmt.Sprint(run) {
			t.Fatalf("run %d export=%d pages=%v", run, exports, pages)
		}
		for _, key := range []string{"readwise-export", "books", "highlights"} {
			_, at, _, _ := db.GetSyncState(key)
			if run < 3 && !at.Equal(before) {
				t.Fatalf("%s advanced on cap", key)
			}
			if run == 3 && !at.Equal(before.Add(time.Hour)) {
				t.Fatalf("%s not completed", key)
			}
		}
	}
	cursor, _, _, _ := db.GetSyncState("readwise-books-list")
	if cursor != "" {
		t.Fatal("list continuation retained")
	}
}

func TestR2BooksListFullReset(t *testing.T) {
	for _, name := range []string{"unfiltered", "filtered"} {
		t.Run(name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3", 0, "since"); err != nil {
				t.Fatal(err)
			}
			var p *syncUserParams
			if name == "filtered" {
				p = &syncUserParams{perResource: map[string]map[string]string{"books": {"category": "books"}}}
			}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"nextPageCursor":null}`)}}
			for _, r := range syncReadwiseExport(context.Background(), c, r4EmptyList(), db, "", true, 1, false, p, io.Discard) {
				if r.Err != nil {
					t.Fatal(r.Err)
				}
			}
			cursor, _, _, err := db.GetSyncState("readwise-books-list")
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if name == "filtered" {
				want = "3"
			}
			if cursor != want {
				t.Fatalf("full list reset %s: %q want %q", name, cursor, want)
			}
		})
	}
}

func TestR3BooksListRejectsStaleChain(t *testing.T) {
	for _, mode := range []string{"resetExport", "resetBooks", "scopeMismatch", "missingChain", "completeExport", "exportCursor"} {
		t.Run(mode, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.SaveReadwiseSyncProgress("readwise-export", "", 4, "old"); err != nil {
				t.Fatal(err)
			}
			if mode != "missingChain" {
				if _, err := db.ReadwiseChainStart("readwise-export", false, time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			scope := "old"
			if mode == "scopeMismatch" {
				scope = "other"
			}
			if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, scope); err != nil {
				t.Fatal(err)
			}
			if mode == "exportCursor" {
				if err := db.SaveReadwiseSyncProgress("readwise-export", "stale-export-cursor", 4, "old"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "completeExport" {
				if err := db.SaveSyncStateAt("readwise-export", "", 4, time.Now()); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "resetExport" || mode == "resetBooks" {
				key := "books"
				if mode == "resetExport" {
					key = "readwise-export"
				}
				if err := db.ResetReadwiseSyncState(key); err != nil {
					t.Fatal(err)
				}
				cursor, _, _, _ := db.GetSyncState("readwise-books-list")
				if cursor != "" {
					t.Fatalf("reset retained list cursor %q", cursor)
				}
			}
			exportCalls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/export/" {
					exportCalls++
					wantCursor := ""
					if mode == "exportCursor" {
						wantCursor = "stale-export-cursor"
					}
					if r.URL.Query().Get("pageCursor") != wantCursor {
						t.Errorf("cursor=%q want %q", r.URL.Query().Get("pageCursor"), wantCursor)
					}
					if mode == "exportCursor" && r.URL.Query().Get("updatedAfter") != "old" {
						t.Error("lost saved scope")
					}
					io.WriteString(w, `{"results":[],"nextPageCursor":null}`)
					return
				}
				if r.URL.Query().Get("page") != "1" {
					t.Error("resumed stale list")
				}
				io.WriteString(w, `{"results":[],"next":null}`)
			}))
			defer srv.Close()
			c := r2HTTPGetter{srv.URL, srv.Client()}
			for _, result := range syncReadwiseExport(context.Background(), c, c, db, "", false, 1, false, nil, io.Discard) {
				if result.Err != nil {
					t.Fatal(result.Err)
				}
			}
			if cursor, _, _, _ := db.GetSyncState("readwise-books-list"); cursor != "" {
				t.Fatalf("stale list retained: %q", cursor)
			}
			if exportCalls != 1 {
				t.Fatalf("export calls = %d", exportCalls)
			}
		})
	}
}
