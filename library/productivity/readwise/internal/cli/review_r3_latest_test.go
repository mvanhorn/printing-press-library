package cli

import (
	"context"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestR2LatestOnlyCommandPreservesAllState(t *testing.T) {
	for _, route := range []string{"export", "readerFull", "executeFull"} {
		t.Run(route, func(t *testing.T) {
			flags := reviewSyncFlags(t)
			path := filepath.Join(t.TempDir(), "data.db")
			db, err := store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			for i, key := range []string{"documents", "books", "highlights", "reader-tags", "readwise-export", "readwise-books-list"} {
				if err := db.SaveSyncStateAt(key, "saved-cursor", 17, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
					t.Fatal(err)
				}
				if _, err := db.DB().Exec(`UPDATE sync_state SET scope_key='saved-scope',readwise_chain_started='saved-chain',last_attempt_complete=? WHERE resource_type=?`, i%2, key); err != nil {
					t.Fatal(err)
				}
			}
			before := r2bSyncStateSnapshot(t, db)
			db.Close()
			old := clientHooks
			defer func() { clientHooks = old }()
			calls := 0
			clientHooks = []func(*client.Client) error{func(c *client.Client) error {
				c.HTTPClient.Transport = r4Transport(func(r *http.Request) (*http.Response, error) {
					calls++
					body := `{"results":[{"id":"fixture-row"}],"next":null,"nextPageCursor":null}`
					return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
				})
				return nil
			}}
			cmd := RootCmd()
			cmd.SetContext(context.Background())
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			resources := []string{"documents", "books", "reader-tags"}
			var runErr error
			if route == "export" {
				runErr = runReadwiseExportSyncCmd(cmd, flags, resources, true, "", 1, path, 1, true, true, nil, nil, nil)
			} else if route == "executeFull" {
				cmd.SetArgs([]string{"--config", flags.configPath, "sync", "--full", "--latest-only", "--db", path, "--no-learn", "--resources", "books,documents,reader-tags"})
				runErr = cmd.Execute()
			} else {
				runErr = runReadwiseReaderFullSync(cmd, flags, resources, "", path, 1, true, true, false, nil)
			}
			if runErr != nil {
				t.Fatal(runErr)
			}
			if calls == 0 {
				t.Fatal("client hook not reached")
			}
			db, err = store.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if n, err := db.Count("books"); err != nil || n == 0 {
				t.Fatalf("walk did not upsert books: %d %v", n, err)
			}
			if after := r2bSyncStateSnapshot(t, db); after != before {
				t.Fatalf("state changed\nbefore=%s\nafter=%s", before, after)
			}
		})
	}
}

func TestR2AllowlistedParamsCheckpoint(t *testing.T) {
	for _, tc := range []struct{ name, resource, param string }{{"documentsHtml", "documents", "documents:withHtmlContent=true"}, {"booksPageSize", "books", "books:page_size=100"}} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			params, err := parseSyncUserParams(nil, []string{tc.param}, nil)
			if err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			ctx := context.WithValue(context.Background(), readwiseRunStartKey{}, at)
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"nextPageCursor":null}`)}}
			if tc.resource == "books" {
				for _, r := range syncReadwiseExport(ctx, c, r4EmptyList(), db, "", false, 1, false, params, io.Discard) {
					if r.Err != nil {
						t.Fatal(r.Err)
					}
				}
			} else {
				r := syncReadwiseCursorResource(ctx, c, db, "documents", "/list/", "", false, 1, false, params, io.Discard, false)
				if r.Err != nil {
					t.Fatal(r.Err)
				}
			}
			_, got, _, err := db.GetSyncState(tc.resource)
			if err != nil || !got.Equal(at) {
				t.Fatalf("watermark=%v error=%v", got, err)
			}
		})
	}
}

type r4Transport func(*http.Request) (*http.Response, error)

func (f r4Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
