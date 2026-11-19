package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type r3ListGetter struct {
	pages   []string
	missing bool
}

func (g *r3ListGetter) Get(_ context.Context, _ string, p map[string]string) (json.RawMessage, error) {
	g.pages = append(g.pages, p["page"])
	if g.missing && p["page"] == "3" {
		return nil, &client.APIError{StatusCode: 404, Body: `{"detail":"Invalid page."}`}
	}
	return json.RawMessage(`{"results":[{"id":1}],"next":null}`), nil
}
func TestR3BooksListResumePages(t *testing.T) {
	for _, tc := range []struct {
		name, cursor string
		missing      bool
		want         string
	}{
		{"overlap", "3:1000", false, "3"},
		{"pageSizeChanged", "3:100", false, "1"},
		{"invalidCursor", "garbage", false, "1"},
		{"legacyCursor", "3", false, "1"},
		{"missingPage", "3:1000", true, "3,1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.SaveReadwiseSyncProgress("readwise-books-list", tc.cursor, 0, ""); err != nil {
				t.Fatal(err)
			}
			g := &r3ListGetter{missing: tc.missing}
			_, cap, err := syncReadwiseBooksList(context.Background(), g, db, "", 1, nil, io.Discard, false)
			if err != nil || cap {
				t.Fatalf("cap=%v error=%v", cap, err)
			}
			got := ""
			for _, p := range g.pages {
				if got != "" {
					got += ","
				}
				got += p
			}
			if got != tc.want {
				t.Fatalf("pages=%s want=%s", got, tc.want)
			}
		})
	}
}
func (g *r3ListGetter) RateLimit() float64 { return 0 }

func TestR3BooksListSinceNotice(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveReadwiseSyncProgress("readwise-export", "", 0, "chain-since"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReadwiseChainStart("readwise-export", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "1:1000", 0, "chain-since"); err != nil {
		t.Fatal(err)
	}
	f, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	old := os.Stderr
	os.Stderr = f
	defer func() { os.Stderr = old }()
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"next":null}`)}}
	for _, r := range syncReadwiseExport(context.Background(), c, c, db, "explicit-since", false, 1, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "resuming capped books-list chain; --since ignored until it completes") {
		t.Fatalf("stderr=%s", b)
	}
	if c.calls[0]["updated__gt"] != "chain-since" {
		t.Fatalf("params=%v", c.calls)
	}
}

func TestR3LatestOnlyKeepsUserWindow(t *testing.T) {
	for _, resource := range []string{"books", "documents"} {
		t.Run(resource, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			params := &syncUserParams{perResource: map[string]map[string]string{resource: {"updatedAfter": "user-window", "updated__gt": "user-window"}}}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"nextPageCursor":null}`)}}
			if resource == "books" {
				list := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"next":null}`)}}
				for _, r := range syncReadwiseExport(context.Background(), c, list, db, "", false, 1, true, params, io.Discard) {
					if r.Err != nil {
						t.Fatal(r.Err)
					}
				}
				if list.calls[0]["updated__gt"] != "user-window" {
					t.Fatalf("list params=%v", list.calls)
				}
			} else {
				r := syncReadwiseCursorResource(context.Background(), c, db, resource, "/list/", "", false, 1, true, params, io.Discard, false)
				if r.Err != nil {
					t.Fatal(r.Err)
				}
			}
			if c.calls[0]["updatedAfter"] != "user-window" {
				t.Fatalf("params=%v", c.calls)
			}
		})
	}
}

func TestR3ManagedPaginationCommandError(t *testing.T) {
	for _, tc := range []struct{ name, flag, token string }{
		{"resource", "--resource-param", "documents:pageCursor=next"},
		{"flat", "--param", "page=2"},
		{"global", "--global-param", "page=2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { calls++ })
			out, stderr, code := sliceBExecute(t, cfg, "sync", "documents", tc.flag, tc.token, "--no-learn")
			key := strings.Split(tc.token, "=")[0]
			want := tc.flag + " " + key + " is managed by sync (resume is automatic)"
			if code != 2 || calls != 0 || !strings.Contains(out+stderr, want) {
				t.Fatalf("code=%d calls=%d output=%s%s want=%s", code, calls, out, stderr, want)
			}
		})
	}
}

func TestR3BooksListPageSizeChangeFetchesAll(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/export/" {
			io.WriteString(w, `{"results":[],"nextPageCursor":null}`)
			return
		}
		p, _ := strconv.Atoi(r.URL.Query().Get("page"))
		size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		pages = append(pages, strconv.Itoa(p))
		start := (p - 1) * size
		end := start + size
		if end > 7 {
			end = 7
		}
		var rows []string
		for i := start; i < end; i++ {
			rows = append(rows, fmt.Sprintf(`{"id":%d}`, i+1))
		}
		next := "null"
		if end < 7 {
			next = `"more"`
		}
		fmt.Fprintf(w, `{"results":[%s],"next":%s}`, strings.Join(rows, ","), next)
	}))
	defer srv.Close()
	c := r2HTTPGetter{srv.URL, srv.Client()}
	run := func(size string, cap int) {
		t.Helper()
		p, err := parseSyncUserParams(nil, []string{"books:page_size=" + size}, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range syncReadwiseExport(context.Background(), c, c, db, "", false, cap, false, p, io.Discard) {
			if r.Err != nil {
				t.Fatal(r.Err)
			}
		}
	}
	run("2", 2)
	cursor, _, _, _ := db.GetSyncState("readwise-books-list")
	if cursor != "2:2" {
		t.Fatalf("saved=%q", cursor)
	}
	pages = nil
	run("5", 0)
	if strings.Join(pages, ",") != "1,2" {
		t.Fatalf("pages=%v", pages)
	}
	if n, err := db.Count("books"); err != nil || n != 7 {
		t.Fatalf("books=%d err=%v", n, err)
	}
	_, at, _, _ := db.GetSyncState("readwise-export")
	if at.IsZero() {
		t.Fatal("chain incomplete")
	}
}

func TestR3MissingListPageCompletesChain(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveReadwiseSyncProgress("readwise-export", "", 0, "since"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReadwiseChainStart("readwise-export", false, start); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, "since"); err != nil {
		t.Fatal(err)
	}
	c := &r3ListGetter{missing: true}
	for _, r := range syncReadwiseExport(context.WithValue(context.Background(), readwiseRunStartKey{}, start.Add(time.Hour)), c, c, db, "", false, 1, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if strings.Join(c.pages, ",") != "3,1" {
		t.Fatalf("pages=%v", c.pages)
	}
	cursor, at, _, err := db.GetSyncState("readwise-export")
	if err != nil || cursor != "" || !at.Equal(start) {
		t.Fatalf("chain state=%q %v %v", cursor, at, err)
	}
	cursor, _, _, _ = db.GetSyncState("readwise-books-list")
	if cursor != "" {
		t.Fatalf("retained list=%q", cursor)
	}
}

func TestR3HeadRefreshNotice(t *testing.T) {
	for _, resource := range []string{"books", "documents"} {
		t.Run(resource, func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			f, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			old := os.Stderr
			os.Stderr = f
			defer func() { os.Stderr = old }()
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[],"nextPageCursor":null}`)}}
			if resource == "books" {
				for _, r := range syncReadwiseExport(context.Background(), c, nil, db, "", false, 1, true, nil, io.Discard) {
					if r.Err != nil {
						t.Fatal(r.Err)
					}
				}
			} else {
				r := syncReadwiseCursorResource(context.Background(), c, db, resource, "/list/", "", false, 1, true, nil, io.Discard, false)
				if r.Err != nil {
					t.Fatal(r.Err)
				}
			}
			b, err := os.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(b), resource+": head refresh — checkpoint not advanced") {
				t.Fatalf("stderr=%s", b)
			}
		})
	}
}
