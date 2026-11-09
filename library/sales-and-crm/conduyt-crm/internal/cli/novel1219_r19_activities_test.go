// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

// PATCH(novel1219-activities-list-page-number): Codex novel1219-r18 (1 P1, 3 P2). Fixtures answer like
// app/api/v1/activities/route.ts: a non-integer page is a 500.

type novel1219SyncServer struct {
	mu           sync.Mutex
	reqs         []string
	firstRows    int
	token        bool
	numericToken bool
}

func (s *novel1219SyncServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		s.mu.Lock()
		s.reqs = append(s.reqs, r.URL.RawQuery)
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if p := q.Get("page"); p != "" {
			if _, err := strconv.Atoi(p); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":"Internal server error"}`)
				return
			}
		}
		rows := func(page, n int) string {
			out := make([]string, n)
			for i := range out {
				out[i] = `{"id":"act-p` + strconv.Itoa(page) + `-` + strconv.Itoa(i) + `","type":"note","createdAt":"2026-11-0` + strconv.Itoa(9-page) + `T00:00:00Z"}`
			}
			return strings.Join(out, ",")
		}
		next := `null`
		if s.token {
			next = `"WyIyMDI2LTExLTA5VDE3OjQwOjQ5LjI1NVoiLCJhY3QtcDEtMCJd"`
			if s.numericToken {
				next = `"2"`
			}
		}
		switch q.Get("page") {
		case "", "1":
			_, _ = io.WriteString(w, `{"data":{"data":[`+rows(1, s.firstRows)+`],"meta":{"page":1,"per_page":50,"total":`+strconv.Itoa(s.firstRows+2)+`,"total_capped":false,"next_cursor":`+next+`}}}`)
		case "2":
			_, _ = io.WriteString(w, `{"data":{"data":[`+rows(2, 2)+`],"meta":{"page":2,"per_page":50,"total":`+strconv.Itoa(s.firstRows+2)+`,"total_capped":false,"next_cursor":null}}}`)
		default:
			_, _ = io.WriteString(w, `{"data":{"data":[],"meta":{"page":3,"per_page":50,"total":0,"total_capped":false,"next_cursor":null}}}`)
		}
	}
}

func novel1219SyncEnv(t *testing.T, h http.Handler) (dir string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	dir = t.TempDir()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", dir)
	return dir
}

func novel1219Sync(t *testing.T, dbPath string, extra ...string) (string, error) {
	t.Helper()
	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetArgs(append([]string{"sync", "--resources", "activities", "--db", dbPath, "--json"}, extra...))
	root.SetOut(&out)
	root.SetErr(io.Discard)
	err := root.Execute()
	return out.String(), err
}

// P1: an r17 checkpoint holds an opaque keyset cursor; an incremental sync must not resume it as page=<cursor>.
func TestNovel1219SyncResumesAnR17CheckpointByPageNumber(t *testing.T) {
	s := &novel1219SyncServer{firstRows: 50, token: true}
	dir := novel1219SyncEnv(t, s.handler(t))
	dbPath := filepath.Join(dir, "a.db")
	if out, err := novel1219Sync(t, dbPath, "--full"); err != nil {
		t.Fatalf("seed sync: %v: %s", err, out)
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := db.DB().Exec(`UPDATE sync_state SET last_cursor = ?, last_attempt_complete = 0 WHERE resource_type = 'activities'`, "WyIyMDI2LTExLTA5VDEzOjAwOjAwLjAwMFoiLCJhY3QtcDEtNDkiXQ")
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		t.Fatalf("r17 checkpoint rows updated = %d, want 1", n)
	}
	_ = db.Close()
	s.mu.Lock()
	s.reqs = nil
	s.mu.Unlock()

	out, err := novel1219Sync(t, dbPath)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		t.Fatalf("incremental sync from an r17 checkpoint: %v (requests %q): %s", err, s.reqs, out)
	}
	if len(s.reqs) == 0 {
		t.Fatal("incremental sync sent no request")
	}
	for _, raw := range s.reqs {
		if strings.Contains(raw, "page=Wy") {
			t.Fatalf("incremental sync sent a keyset cursor as page: %q", s.reqs)
		}
	}
}

// P2-1: a short page whose body carries a continuation token is not the end; the page-numbered walk goes on.
func TestNovel1219SyncShortPageWithTokenAdvancesByNumber(t *testing.T) {
	s := &novel1219SyncServer{firstRows: 3, token: true}
	dir := novel1219SyncEnv(t, s.handler(t))
	out, err := novel1219Sync(t, filepath.Join(dir, "a.db"), "--full")
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		t.Fatalf("sync: %v (requests %q)", err, s.reqs)
	}
	if !strings.Contains(out, `"total_records": 5`) {
		t.Fatalf("sync stopped on the short page with a token (requests %q): %s", s.reqs, out)
	}
	if len(s.reqs) < 2 || !strings.Contains(s.reqs[1], "page=2") {
		t.Fatalf("requests = %q, want page one then page=2", s.reqs)
	}
}

type novel1219Getter struct {
	pages map[string]string
}

func (g novel1219Getter) GetWithHeaders(_ context.Context, _ string, params map[string]string, _ map[string]string) (json.RawMessage, error) {
	return json.RawMessage(g.pages[params["cursor"]]), nil
}

func (novel1219Getter) IsDryRun() bool { return false }

// P2-2: a short cursor page without a next_cursor key is not proof of the end.
func TestNovel1219ActivitiesWalkRefusesAShortCursorPageWithoutNextCursor(t *testing.T) {
	g := novel1219Getter{pages: map[string]string{
		"":   `{"data":{"data":[{"id":"a"},{"id":"b"}],"meta":{"page":1,"per_page":2,"next_cursor":"C1"}}}`,
		"C1": `{"data":{"data":[{"id":"c"}],"meta":{"page":1,"per_page":2}}}`,
	}}
	if data, err := walkActivitiesAll(context.Background(), g, "/activities", map[string]string{"per_page": "2"}); err == nil {
		t.Fatalf("walk returned %s as complete; want an error for the cursor page without next_cursor", data)
	}
	g.pages["C1"] = `{"data":{"data":[{"id":"c"}],"meta":{"page":1,"per_page":2,"next_cursor":null}}}`
	data, err := walkActivitiesAll(context.Background(), g, "/activities", map[string]string{"per_page": "2"})
	if err != nil || !strings.Contains(string(data), `"c"`) {
		t.Fatalf("walk with a null next_cursor = %s, %v; want three rows", data, err)
	}
}

// P2-3: default (auto) --all writes the walked rows through to the local store and, when the API is unreachable,
// answers from the local store.
func TestNovel1219ActivitiesListAllAutoWritesThroughAndFallsBackLocal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") == "C1" {
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-c","type":"note","createdAt":"2026-11-08T00:00:00Z"}],"meta":{"page":1,"per_page":2,"total":3,"next_cursor":null}}}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-a","type":"note","createdAt":"2026-11-09T00:00:00Z"},{"id":"act-b","type":"note","createdAt":"2026-11-09T00:00:00Z"}],"meta":{"page":1,"per_page":2,"total":3,"next_cursor":"C1"}}}`)
	}))
	dir := t.TempDir()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", dir)
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	run := func(args ...string) (string, error) {
		root := newRootCmd(&rootFlags{})
		var out bytes.Buffer
		root.SetArgs(args)
		root.SetOut(&out)
		root.SetErr(io.Discard)
		err := root.Execute()
		return out.String(), err
	}
	out, err := run("activities", "list", "--all", "--per-page", "2", "--no-cache", "--json")
	if err != nil || !strings.Contains(out, `"source": "live"`) {
		t.Fatalf("auto --all live: %v: %s", err, out)
	}
	db, err := store.Open(defaultDBPath("conduyt-crm-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	n, err := db.Count("activities")
	_ = db.Close()
	if err != nil || n != 3 {
		t.Fatalf("write-through stored %d activities (err %v), want 3", n, err)
	}
	srv.Close()
	out, err = run("activities", "list", "--all", "--no-cache", "--json")
	if err != nil {
		t.Fatalf("auto --all with the API unreachable: %v: %s", err, out)
	}
	if !strings.Contains(out, `"source": "local"`) || !strings.Contains(out, "act-c") {
		t.Fatalf("auto --all with the API unreachable did not answer from the local store: %s", out)
	}
}
