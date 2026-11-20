package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/config"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type r4Getter func(string, map[string]string) (json.RawMessage, error)

func (g r4Getter) Get(_ context.Context, path string, p map[string]string) (json.RawMessage, error) {
	return g(path, p)
}
func (g r4Getter) RateLimit() float64 { return 0 }

func TestR4DrainedExportResumesListAfterFailure(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	start := w0.Add(time.Hour)
	if err := db.SaveSyncStateAt("readwise-export", "", 0, w0); err != nil {
		t.Fatal(err)
	}
	run, exports := 1, 0
	g := r4Getter(func(path string, p map[string]string) (json.RawMessage, error) {
		if path == "/export/" {
			exports++
			if run == 2 {
				t.Fatal("re-walked drained export")
			}
			if p["updatedAfter"] != w0.Format(time.RFC3339) {
				t.Fatal(p)
			}
			if p["pageCursor"] == "" {
				return json.RawMessage(`{"results":[{"id":10}],"nextPageCursor":"second"}`), nil
			}
			return json.RawMessage(`{"results":[{"id":11}],"nextPageCursor":null}`), nil
		}
		if p["updated__gt"] != w0.Format(time.RFC3339) {
			t.Fatal(p)
		}
		if p["page"] == "2" && run == 1 {
			return nil, fmt.Errorf("fixture unavailable")
		}
		if p["page"] == "1" {
			return json.RawMessage(`{"results":[{"id":1}],"next":"more"}`), nil
		}
		return json.RawMessage(`{"results":[{"id":2}],"next":null}`), nil
	})
	ctx := context.WithValue(context.Background(), readwiseRunStartKey{}, start)
	first := syncReadwiseExport(ctx, g, g, db, "", false, 0, false, nil, io.Discard)
	if first[0].Err == nil {
		t.Fatal("expected list failure")
	}
	cursor, _, _, _ := db.GetSyncState("readwise-export")
	if cursor != "" {
		t.Fatalf("export not drained: %s", cursor)
	}
	run = 2
	for _, r := range syncReadwiseExport(context.WithValue(context.Background(), readwiseRunStartKey{}, start.Add(time.Hour)), g, g, db, "", false, 0, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	_, at, _, _ := db.GetSyncState("readwise-export")
	_, _, books, _ := db.GetSyncState("books")
	if books != 4 {
		t.Fatalf("overlap inflated books count: %d", books)
	}
	if exports != 2 || !at.Equal(start) {
		t.Fatalf("exports=%d watermark=%v", exports, at)
	}
}

func TestR4List404RestartScopeAndBudget(t *testing.T) {
	for _, firstMissing := range []bool{true, false} {
		t.Run(fmt.Sprint(firstMissing), func(t *testing.T) {
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, ""); err != nil {
				t.Fatal(err)
			}
			var pages []string
			g := r4Getter(func(_ string, p map[string]string) (json.RawMessage, error) {
				page := p["page"]
				pages = append(pages, page)
				if (firstMissing && page == "3") || (!firstMissing && page == "4") {
					return nil, &client.APIError{StatusCode: 404}
				}
				return json.RawMessage(`{"results":[{"id":1}],"next":"more"}`), nil
			})
			_, cap, err := syncReadwiseBooksList(context.Background(), g, db, "", 1, nil, io.Discard, false)
			cursor, _, _, _ := db.GetSyncState("readwise-books-list")
			if firstMissing {
				if err != nil || !cap || strings.Join(pages, ",") != "3,1" || cursor != "1:1000" {
					t.Fatalf("pages=%v cursor=%s cap=%v err=%v", pages, cursor, cap, err)
				}
			} else {
				if err == nil || strings.Join(pages, ",") != "3,4" || cursor != "3:1000" {
					t.Fatalf("pages=%v cursor=%s err=%v", pages, cursor, err)
				}
			}
		})
	}
}

func TestR4BooksPageSizeUsage(t *testing.T) {
	for _, value := range []string{"0", "-1", "abc", "1.5", "", "999999999999999999999"} {
		_, err := parseSyncUserParams(nil, []string{"books:page_size=" + value}, nil)
		if err == nil || !strings.Contains(err.Error(), "--resource-param books:page_size must be a positive integer") {
			t.Fatalf("value=%q error=%v", value, err)
		}
	}
	if _, err := parseSyncUserParams(nil, []string{"books:page_size=2"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestR4NullExportCursorBindsList(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.SaveReadwiseSyncProgress("readwise-export", "", 0, "scope"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReadwiseChainStart("readwise-export", false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "2:1000", 0, "scope"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB().Exec(`UPDATE sync_state SET last_cursor=NULL WHERE resource_type='readwise-export'`); err != nil {
		t.Fatal(err)
	}
	scope, ok, err := db.ReadwiseBooksListResumeScope()
	if err != nil || !ok || scope != "scope" {
		t.Fatalf("scope=%s valid=%v error=%v", scope, ok, err)
	}
	for _, name := range []string{"readwise-export", "readwise-books-list"} {
		if !store.IsInternalSyncKey(name) {
			t.Fatal(name)
		}
	}
	if store.IsInternalSyncKey("books") {
		t.Fatal("books is public")
	}
}

func r4EmptyList() readwiseGetter {
	return r4Getter(func(string, map[string]string) (json.RawMessage, error) {
		return json.RawMessage(`{"results":[],"next":null}`), nil
	})
}

func TestR4IsolationClearsDBScope(t *testing.T) {
	t.Setenv("READWISE_BASE_URL", "http://127.0.0.1:9")
	setDefaultDBScopeIdentity("fixture-token", "old-token", "https://fixture.invalid")
	defaultDBScopeState.Lock()
	defaultDBScopeState.claimSuppressed = true
	defaultDBScopeState.Unlock()
	testenv.Isolate(t)
	if got := os.Getenv("READWISE_BASE_URL"); got != "" {
		t.Fatalf("base URL leaked: %s", got)
	}
	hash, legacy, label := currentDefaultDBScope()
	if hash != "" || legacy != "" || label != "" || currentDefaultDBClaimSuppressed() {
		t.Fatalf("scope leaked: %q %q %q", hash, legacy, label)
	}
}

func TestR4RealHTTP404RestartsResumedPage(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveReadwiseExportCheckpoint("", "since", 0, 0, 0, start, false); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ReadwiseChainStart("readwise-export", false, start); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, "since"); err != nil {
		t.Fatal(err)
	}
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/books/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		page := r.URL.Query().Get("page")
		pages = append(pages, page)
		if page == "3" {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, `{"detail":"Invalid page."}`)
			return
		}
		io.WriteString(w, `{"results":[{"id":1}],"next":null}`)
	}))
	defer srv.Close()
	real := client.New(&config.Config{BaseURL: srv.URL, ReadwiseApiToken: "fixture"}, time.Second, 0)
	real.HTTPClient = srv.Client()
	real.NoCache = true
	paced := newFixedPacedClient(real, 0)
	for _, r := range syncReadwiseExport(context.Background(), paced, paced, db, "", false, 1, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
	if strings.Join(pages, ",") != "3,1" {
		t.Fatalf("pages=%v", pages)
	}
	cursor, at, _, _ := db.GetSyncState("readwise-export")
	if cursor != "" || !at.Equal(start) {
		t.Fatalf("cursor=%s watermark=%v", cursor, at)
	}
	cursor, _, _, _ = db.GetSyncState("readwise-books-list")
	if cursor != "" {
		t.Fatalf("list retained=%s", cursor)
	}
}

func TestR4StaleListPreservesExportWatermark(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	w0 := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.SaveSyncStateAt("readwise-export", "", 0, w0); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveReadwiseSyncProgress("readwise-books-list", "3:1000", 0, "foreign"); err != nil {
		t.Fatal(err)
	}
	g := r4Getter(func(path string, p map[string]string) (json.RawMessage, error) {
		if path == "/export/" {
			if p["updatedAfter"] != w0.Format(time.RFC3339) {
				t.Fatalf("lost watermark: %v", p)
			}
			return json.RawMessage(`{"results":[],"nextPageCursor":null}`), nil
		}
		if p["page"] != "1" {
			t.Fatalf("foreign list resumed: %v", p)
		}
		return json.RawMessage(`{"results":[],"next":null}`), nil
	})
	for _, r := range syncReadwiseExport(context.Background(), g, g, db, "", false, 0, false, nil, io.Discard) {
		if r.Err != nil {
			t.Fatal(r.Err)
		}
	}
}

func TestR4BooksCountIgnoresOverlap(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var pages []string
	g := r4Getter(func(path string, p map[string]string) (json.RawMessage, error) {
		if path == "/export/" {
			return json.RawMessage(`{"results":[],"nextPageCursor":null}`), nil
		}
		pages = append(pages, p["page"])
		if p["page"] == "1" {
			return json.RawMessage(`{"results":[{"id":1}],"next":"more"}`), nil
		}
		return json.RawMessage(`{"results":[{"id":2}],"next":null}`), nil
	})
	for run := 0; run < 2; run++ {
		for _, r := range syncReadwiseExport(context.Background(), g, g, db, "", false, 1, false, nil, io.Discard) {
			if r.Err != nil {
				t.Fatal(r.Err)
			}
		}
	}
	if strings.Join(pages, ",") != "1,1,2" {
		t.Fatalf("expected nonempty overlap: %v", pages)
	}
	_, _, count, err := db.GetSyncState("books")
	if err != nil || count != 2 {
		t.Fatalf("books total_count=%d error=%v", count, err)
	}
	if rows, err := db.Count("books"); err != nil || rows != count {
		t.Fatalf("rows=%d count=%d error=%v", rows, count, err)
	}
}
