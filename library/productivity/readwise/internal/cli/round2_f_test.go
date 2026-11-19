package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFMutationPreservesDocument(t *testing.T) {
	for _, response := range []string{`{"id":"d","url":"https://example.test/d"}`, `{"id":"d","url":"https://example.test/d","location":"archive"}`} {
		cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(response)) })
		db, err := store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertDocuments(json.RawMessage(`{"id":"d","title":"Reading","content":"passage","location":"later"}`)); err != nil {
			t.Fatal(err)
		}
		db.Close()
		_, stderr, code := sliceBExecute(t, cfg, "documents", "update", "d", "--location", "archive", "--yes", "--json")
		if code != 0 {
			t.Fatalf("update: %s", stderr)
		}
		db, err = store.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := db.Get("documents", "d")
		db.Close()
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		wantLocation := "later"
		if response != `{"id":"d","url":"https://example.test/d"}` {
			wantLocation = "archive"
		}
		if got["title"] != "Reading" || got["content"] != "passage" || got["location"] != wantLocation {
			t.Fatalf("sparse response corrupted document: %s", raw)
		}
	}
}

func TestFBackupPurge(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected request") })
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	backup := path + ".v4.bak"
	if err := os.WriteFile(backup, []byte("private"), 0600); err != nil {
		t.Fatal(err)
	}
	_, stderr, code := sliceBExecute(t, cfg, "local-data", "purge", "--yes")
	if code != 0 {
		t.Fatal(stderr)
	}
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Fatalf("backup survived purge: %v", err)
	}
}

func TestFPruneFailsClosed(t *testing.T) {
	for _, body := range []string{`{"results":[]}`, `{}`, `{"detail":"Request was throttled."}`, `{"count":500,"results":[]}`, `{"count":500,"results":[{"id":"keep"}]}`, `{"results":[{"id":"keep"}]}`} {
		{
			db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, id := range []string{"keep", "old1", "old2"} {
				if err := db.UpsertDocuments(json.RawMessage(fmt.Sprintf(`{"id":%q}`, id))); err != nil {
					t.Fatal(err)
				}
			}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(body)}}
			var events bytes.Buffer
			res := syncReadwiseCursorResource(context.Background(), c, db, "documents", "/list/", "", true, 0, false, nil, &events, true)
			if res.Err != nil {
				t.Fatal(res.Err)
			}
			n, err := db.Count("documents")
			if err != nil || n != 3 {
				t.Fatalf("pruned unsafe snapshot: %d %v", n, err)
			}
			if !bytes.Contains(events.Bytes(), []byte(`"event":"sync_warning"`)) {
				t.Fatalf("missing warning: %s", events.String())
			}
		}
	}
}

func TestFChildReconciliation(t *testing.T) {
	for _, childEvidence := range []bool{true, false} {
		db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range []string{`{"id":"parent"}`, `{"id":"gone","parent_id":"parent"}`, `{"id":"live","parent_id":"parent"}`} {
			if err := db.UpsertDocuments(json.RawMessage(raw)); err != nil {
				t.Fatal(err)
			}
		}
		body := `{"results":[{"id":"parent"}]}`
		if childEvidence {
			body = `{"results":[{"id":"parent"},{"id":"live","parent_id":"parent"}]}`
		}
		var events bytes.Buffer
		res := syncReadwiseCursorResource(context.Background(), &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(body)}}, db, "documents", "/list/", "", true, 0, false, nil, &events, true)
		if res.Err != nil {
			t.Fatal(res.Err)
		}
		n, err := db.Count("documents")
		want := 3
		if childEvidence {
			want = 2
		}
		if err != nil || n != want {
			t.Fatalf("children count=%d want=%d err=%v", n, want, err)
		}
		if !childEvidence && !bytes.Contains(events.Bytes(), []byte("child_pruning_skipped")) {
			t.Fatal("skip absent from summary")
		}
		db.Close()
	}
}

func TestFDeadBookChildren(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.UpsertBooks(json.RawMessage(`{"id":1,"title":"book"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertHighlights(json.RawMessage(`{"id":2,"book_id":1,"text":"old"}`)); err != nil {
		t.Fatal(err)
	}
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[{"user_book_id":1,"is_deleted":true,"highlights":[{"id":3,"text":"nested"}]}]}`)}}
	for _, res := range syncReadwiseExport(context.Background(), c, r4EmptyList(), db, "", false, 0, false, nil, nil) {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	for _, resource := range []string{"books", "highlights"} {
		n, err := db.Count(resource)
		if err != nil || n != 0 {
			t.Fatalf("%s count=%d err=%v", resource, n, err)
		}
	}
}

func TestFDocumentsResume(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	since := "2020-01-01T00:00:00Z"
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"nextPageCursor":"next","results":[]}`)}}
	res := syncReadwiseCursorResource(context.Background(), c, db, "documents", "/list/", since, false, 1, false, nil, &bytes.Buffer{}, false)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	_, at, _, err := db.GetSyncState("documents")
	if err != nil || !at.IsZero() {
		t.Fatalf("partial watermark=%v err=%v", at, err)
	}
	c = &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[]}`)}}
	res = syncReadwiseCursorResource(context.Background(), c, db, "documents", "/list/", "", false, 0, false, nil, &bytes.Buffer{}, false)
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if q := c.calls[0]; q["pageCursor"] != "next" || q["updatedAfter"] != since {
		t.Fatalf("resume query=%v", q)
	}
}

func TestFBookTombstone(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.UpsertBooks(json.RawMessage(`{"id":1,"title":"book"}`)); err != nil {
		t.Fatal(err)
	}
	c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(`{"results":[{"user_book_id":1,"is_deleted":true,"highlights":[]}]}`)}}
	for _, res := range syncReadwiseExport(context.Background(), c, r4EmptyList(), db, "", false, 0, false, nil, nil) {
		if res.Err != nil {
			t.Fatal(res.Err)
		}
	}
	n, err := db.Count("books")
	if err != nil || n != 0 {
		t.Fatalf("ghost book: count=%d err=%v", n, err)
	}
}

func TestFBulkPatch(t *testing.T) {
	method := ""
	cfg, _ := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { method = r.Method; w.Write([]byte(`{"results":[]}`)) })
	_, stderr, code := sliceBExecute(t, cfg, "documents", "bulk-update", "--updates", `[{"id":"d","location":"archive"}]`, "--yes", "--json")
	if code != 0 || method != "PATCH" {
		t.Fatalf("method=%s code=%d stderr=%s", method, code, stderr)
	}
}

func TestFResumeChainStart(t *testing.T) {
	first := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, resource := range []string{"documents", "readwise-export"} {
		db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
		if err != nil {
			t.Fatal(err)
		}
		for leg := 0; leg < 2; leg++ {
			body := `{"nextPageCursor":"next","results":[]}`
			capPages := 1
			if leg == 1 {
				body = `{"results":[]}`
				capPages = 0
			}
			c := &stubReadwiseGetter{responses: []json.RawMessage{json.RawMessage(body)}}
			ctx := context.WithValue(context.Background(), readwiseRunStartKey{}, first.Add(time.Duration(leg)*time.Hour))
			if resource == "documents" {
				res := syncReadwiseCursorResource(ctx, c, db, resource, "/list/", "", false, capPages, false, nil, &bytes.Buffer{}, false)
				if res.Err != nil {
					t.Fatal(res.Err)
				}
			} else {
				for _, res := range syncReadwiseExport(ctx, c, r4EmptyList(), db, "", false, capPages, false, nil, nil) {
					if res.Err != nil {
						t.Fatal(res.Err)
					}
				}
			}
		}
		_, at, _, err := db.GetSyncState(resource)
		if err != nil || !at.Equal(first) {
			t.Fatalf("%s watermark=%v want=%v err=%v", resource, at, first, err)
		}
		db.Close()
	}
}

func TestFSyncErrorRedacted(t *testing.T) {
	cause := errors.New("private document text")
	err := finishReadwiseSyncResults([]syncResult{{Resource: "documents", Err: cause}}, true, time.Now(), &bytes.Buffer{})
	if err == nil || !errors.Is(err, cause) || strings.Contains(err.Error(), "private document text") {
		t.Fatalf("unsafe error %v", err)
	}
}

func TestFDocumentsFullPreservesExport(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"results":[]}`)) })
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := db.SaveSyncStateAt("readwise-export", "saved", 42, at); err != nil {
		t.Fatal(err)
	}
	db.Close()
	_, stderr, code := sliceBExecute(t, cfg, "sync", "--full", "--resources", "documents", "--json")
	if code != 0 {
		t.Fatal(stderr)
	}
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cursor, got, n, err := db.GetSyncState("readwise-export")
	if err != nil || cursor != "saved" || !got.Equal(at) || n != 42 {
		t.Fatalf("export reset: %q %v %d %v", cursor, got, n, err)
	}
}

func TestFMCPMirrorDataDir(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("unexpected request") })
	t.Setenv("READWISE_MCP_CHILD", "1")
	out, stderr, code := sliceBExecute(t, cfg, "mirror", "--force", "--json")
	if code != 0 {
		t.Fatalf("%s %s", out, stderr)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(filepath.Dir(path), "readwise-backup")
	if got["export_dir"] != want || !filepath.IsAbs(want) {
		t.Fatalf("mirror destination=%v want=%s", got, want)
	}
}

func TestFMassPruneOverride(t *testing.T) {
	cfg, path := sliceBSetup(t, func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"results":[{"id":"keep"}]}`)) })
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"keep", "old1", "old2"} {
		if err := db.UpsertDocuments(json.RawMessage(fmt.Sprintf(`{"id":%q}`, id))); err != nil {
			t.Fatal(err)
		}
	}
	db.Close()
	_, stderr, code := sliceBExecute(t, cfg, "sync", "--full", "--resources", "documents", "--allow-mass-prune", "--json")
	if code != 0 {
		t.Fatal(stderr)
	}
	db, err = store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	n, err := db.Count("documents")
	if err != nil || n != 1 {
		t.Fatalf("override count=%d err=%v", n, err)
	}
}
