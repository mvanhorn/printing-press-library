// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// PATCH(novel1219-activities-list-page-number): Greptile PR #2450 thread 4514458610. GET /activities answers
// {"data":{"data":[...],"meta":{page,per_page,total,total_capped,next_cursor}}} and takes a keyset `cursor` and a
// numeric `page`. `activities list --all` follows meta.next_cursor through `cursor`, falls back to numeric pages only
// when no next_cursor key is returned, and never sends a cursor as `page` (the route answers that with a 500).

const novel1219Cursor1 = "WyIyMDI2LTExLTA5VDE3OjQwOjQ5LjI1NVoiLCJhY3QtcDEtYiJd"

type novel1219ActivitiesServer struct {
	mu   sync.Mutex
	reqs []url.Values
}

func (s *novel1219ActivitiesServer) start(t *testing.T, withCursor bool) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/activities" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		q := r.URL.Query()
		s.mu.Lock()
		s.reqs = append(s.reqs, q)
		s.mu.Unlock()
		if p := q.Get("page"); p != "" {
			if _, err := strconv.Atoi(p); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"error":"Internal server error"}`)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		page2 := q.Get("cursor") == novel1219Cursor1 || (!withCursor && q.Get("page") == "2")
		switch {
		case q.Get("cursor") != "" && q.Get("cursor") != novel1219Cursor1:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"error":"invalid cursor"}`)
		case page2 && withCursor:
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p2-c"}],"meta":{"page":1,"per_page":2,"total":3,"total_capped":false,"next_cursor":null}}}`)
		case page2:
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p2-c"}],"meta":{"page":2,"per_page":2,"total":3,"total_capped":false}}}`)
		case withCursor:
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p1-a"},{"id":"act-p1-b"}],"meta":{"page":1,"per_page":2,"total":3,"total_capped":false,"next_cursor":"`+novel1219Cursor1+`"}}}`)
		default:
			_, _ = io.WriteString(w, `{"data":{"data":[{"id":"act-p1-a"},{"id":"act-p1-b"}],"meta":{"page":1,"per_page":2,"total":3,"total_capped":false}}}`)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", t.TempDir())
}

func novel1219RunActivitiesAll(t *testing.T) string {
	t.Helper()
	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetArgs([]string{"activities", "list", "--all", "--per-page", "2", "--data-source", "live", "--no-cache", "--json"})
	root.SetOut(&out)
	root.SetErr(io.Discard)
	if err := root.Execute(); err != nil {
		t.Fatalf("activities list --all: %v", err)
	}
	return out.String()
}

func novel1219AssertRows(t *testing.T, out string) {
	t.Helper()
	var env struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("decode output: %v: %s", err, out)
	}
	var ids []string
	for _, r := range env.Results {
		ids = append(ids, r.ID)
	}
	if strings.Join(ids, ",") != "act-p1-a,act-p1-b,act-p2-c" {
		t.Fatalf("rows = %q, want both pages in order (act-p1-a, act-p1-b, act-p2-c)", ids)
	}
}

func TestNovel1219ActivitiesListAllFollowsNextCursor(t *testing.T) {
	s := &novel1219ActivitiesServer{}
	s.start(t, true)
	novel1219AssertRows(t, novel1219RunActivitiesAll(t))
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reqs) != 2 {
		t.Fatalf("requests = %v, want two", s.reqs)
	}
	if s.reqs[0].Get("cursor") != "" || s.reqs[1].Get("cursor") != novel1219Cursor1 || s.reqs[1].Get("page") != "" {
		t.Fatalf("requests = %v, want page one, then cursor=<meta.next_cursor> with no page", s.reqs)
	}
}

func TestNovel1219ActivitiesListAllFallsBackToPageNumbersWithoutNextCursor(t *testing.T) {
	s := &novel1219ActivitiesServer{}
	s.start(t, false)
	novel1219AssertRows(t, novel1219RunActivitiesAll(t))
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.reqs) != 2 || s.reqs[1].Get("page") != "2" || s.reqs[1].Get("cursor") != "" {
		t.Fatalf("requests = %v, want page one then page=2", s.reqs)
	}
}

// PATCH(novel1219-activities-list-page-number): live, GET /activities also returns meta.next_cursor (an opaque string).
// The page-numbered sync must not send that string as page= (the API answers 500); it advances by number.
func TestNovel1219ActivitiesSyncWalksNumberedPagesPastAnOpaqueNextCursor(t *testing.T) {
	var mu sync.Mutex
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		pages = append(pages, q.Get("page"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		rows := func(page, n int) string {
			out := make([]string, n)
			for i := range out {
				out[i] = `{"id":"act-p` + string(rune('0'+page)) + `-` + string(rune('a'+i)) + `","type":"note","createdAt":"2026-11-0` + string(rune('0'+9-page)) + `T00:00:00Z"}`
			}
			return strings.Join(out, ",")
		}
		switch q.Get("page") {
		case "", "1":
			_, _ = io.WriteString(w, `{"data":{"data":[`+rows(1, 50)+`],"meta":{"page":1,"per_page":50,"total":53,"total_capped":false,"next_cursor":"WyIyMDI2LTExLTA5VDE3OjQwOjQ5LjI1NVoiLCJhY3QtcDEtYSJd"}}}`)
		case "2":
			_, _ = io.WriteString(w, `{"data":{"data":[`+rows(2, 3)+`],"meta":{"page":2,"per_page":50,"total":53,"total_capped":false,"next_cursor":null}}}`)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"Internal server error"}`)
		}
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", dir)

	root := newRootCmd(&rootFlags{})
	var out bytes.Buffer
	root.SetArgs([]string{"sync", "--resources", "activities", "--full", "--db", filepath.Join(dir, "a.db"), "--json"})
	root.SetOut(&out)
	root.SetErr(io.Discard)
	err := root.Execute()
	mu.Lock()
	defer mu.Unlock()
	if err != nil {
		t.Fatalf("sync activities: %v (pages requested %q)", err, pages)
	}
	if len(pages) != 2 || pages[1] != "2" {
		t.Fatalf("pages requested = %q, want page one then page=2", pages)
	}
	if !strings.Contains(out.String(), `"total_records": 53`) {
		t.Fatalf("sync summary does not report 53 records: %s", out.String())
	}
}
