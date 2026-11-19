package cli

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

type r2HTTPGetter struct {
	url    string
	client *http.Client
}

func (g r2HTTPGetter) Get(ctx context.Context, path string, params map[string]string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", g.url+path, nil)
	if err != nil {
		return nil, err
	}
	q := req.URL.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	req.URL.RawQuery = q.Encode()
	r, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	return io.ReadAll(r.Body)
}
func TestR2UserTimeBoundPreservesCheckpoint(t *testing.T) {
	for _, resource := range []string{"documents", "books"} {
		t.Run(resource, func(t *testing.T) {
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
			if err := db.SaveSyncStateAt(key, "", 0, before); err != nil {
				t.Fatal(err)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Query().Get("updatedAfter") != "2026-01-01T00:00:00Z" {
					t.Error(r.URL.RawQuery)
				}
				io.WriteString(w, `{"results":[],"nextPageCursor":null}`)
			}))
			defer srv.Close()
			c := r2HTTPGetter{srv.URL, srv.Client()}
			p := &syncUserParams{perResource: map[string]map[string]string{resource: {"updatedAfter": "2026-01-01T00:00:00Z"}}}
			if resource == "books" {
				for _, r := range syncReadwiseExport(context.Background(), c, nil, db, "", false, 0, false, p, io.Discard) {
					if r.Err != nil {
						t.Fatal(r.Err)
					}
				}
			} else {
				r := syncReadwiseCursorResource(context.Background(), c, db, resource, "/list/", "", false, 0, false, p, io.Discard, false)
				if r.Err != nil {
					t.Fatal(r.Err)
				}
			}
			_, at, _, err := db.GetSyncState(key)
			if err != nil || !at.Equal(before) {
				t.Fatalf("checkpoint advanced: %v %v", at, err)
			}
		})
	}
}
func TestR2RejectPinnedSyncPage(t *testing.T) {
	for _, key := range []string{"page", "pageCursor"} {
		t.Run(key, func(t *testing.T) {
			if _, err := parseSyncUserParams(nil, []string{"documents:" + key + "=2"}, nil); err == nil {
				t.Fatal("pinned pagination accepted")
			}
		})
	}
}

func (g r2HTTPGetter) RateLimit() float64 { return 0 }

func TestR2FilteredFullResetPreservesState(t *testing.T) {
	f := reviewSyncFlags(t)
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveSyncStateAt("documents", "saved", 7, before); err != nil {
		t.Fatal(err)
	}
	old := clientHooks
	defer func() { clientHooks = old }()
	clientHooks = []func(*client.Client) error{func(*client.Client) error { return errors.New("fixture: prevent network") }}
	cmd := RootCmd()
	cmd.SetContext(context.Background())
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	p := &syncUserParams{perResource: map[string]map[string]string{"documents": {"location": "new"}}}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	_ = runReadwiseReaderFullSync(cmd, f, []string{"documents"}, "", path, 1, false, true, false, p)
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursor, at, n, err := db.GetSyncState("documents")
	if err != nil || cursor != "saved" || !at.Equal(before) || n != 7 {
		t.Fatalf("filtered full reset: %s %v %d %v", cursor, at, n, err)
	}
}
