// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
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

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/conduyt-crm/internal/store"
)

// PATCH(novel1219-activities-list-page-number): Codex novel1219-r19 (2 P2). normalizePageContinuation is the one
// page-numbered continuation rule for the flat and the dependent sync loops: a non-empty body continuation proves a
// next page whatever the page's length, a numeric one is used as is, and a non-numeric one never reaches page=.

const novel1219Keyset = "WyIyMDI2LTExLTA5VDE3OjQwOjQ5LjI1NVoiLCJjaGlsZC0yIl0"

// novel1219PageAPI answers page 1 with firstRows rows and the given continuation token, page 2 with 2 rows and no
// token, and any non-integer page with a 500-equivalent error, like GET /activities.
type novel1219PageAPI struct {
	firstRows int
	token     string
	pages     []string
}

func (a *novel1219PageAPI) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	p := params["page"]
	a.pages = append(a.pages, p)
	if _, err := strconv.Atoi(p); p != "" && err != nil {
		return nil, &client.APIError{Method: "GET", Path: "/parents/p1/children", StatusCode: 500, Body: `{"error":"Internal server error"}`}
	}
	rows := func(page, n int) string {
		out := make([]string, n)
		for i := range out {
			out[i] = `{"id":"child-` + strconv.Itoa(page) + `-` + strconv.Itoa(i) + `"}`
		}
		return strings.Join(out, ",")
	}
	switch p {
	case "", "1":
		next := "null"
		if a.token != "" {
			next = strconv.Quote(a.token)
		}
		return json.RawMessage(`{"data":[` + rows(1, a.firstRows) + `],"meta":{"next_cursor":` + next + `}}`), nil
	case "2":
		return json.RawMessage(`{"data":[` + rows(2, 2) + `],"meta":{"next_cursor":null}}`), nil
	}
	return json.RawMessage(`{"data":[],"meta":{"next_cursor":null}}`), nil
}

func (a *novel1219PageAPI) RateLimit() float64 { return 0 }

// The dependent is named activities only because resourceSupportsPagination must hold; the route and pageSize are the test's own.
func novel1219RunDependent(t *testing.T, api *novel1219PageAPI) parentReport {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	dep := dependentResourceDef{Name: "activities", ParentTable: "parents", PathTemplate: "/parents/{id}/children", PathParams: []dependentPathParamDef{{Param: "id", Field: "id"}}}
	pageSize := paginationDefaults{cursorParam: "page", cursorType: "page", initialCursor: "1", limitParam: "per_page", limit: 3}
	return syncOneParent(context.Background(), api, db, dep, map[string]string{"id": "p1"}, dep.PathParams, pageSize, "", "", 0, false, false, nil, io.Discard, "")
}

func novel1219AssertTwoPages(t *testing.T, api *novel1219PageAPI, rep parentReport, wantStored int) {
	t.Helper()
	for _, p := range api.pages {
		if _, err := strconv.Atoi(p); p != "" && err != nil {
			t.Fatalf("a page carried a non-numeric value: pages %q", api.pages)
		}
	}
	if rep.failure != nil || rep.stored != wantStored || !rep.complete || len(api.pages) < 2 || api.pages[1] != "2" {
		t.Fatalf("stored %d complete %v failure %v reason %q pages %q; want %d stored over page one then page=2", rep.stored, rep.complete, rep.failure, rep.reason, api.pages, wantStored)
	}
}

// P2-1, dependent loop, full page with an opaque token: the next request is page=2, not page=<token>.
func TestNovel1219DependentFullPageWithOpaqueTokenAdvancesByNumber(t *testing.T) {
	api := &novel1219PageAPI{firstRows: 3, token: novel1219Keyset}
	novel1219AssertTwoPages(t, api, novel1219RunDependent(t, api), 5)
}

// P2-1, dependent loop, short page with an opaque token: not the end.
func TestNovel1219DependentShortPageWithOpaqueTokenAdvancesByNumber(t *testing.T) {
	api := &novel1219PageAPI{firstRows: 1, token: novel1219Keyset}
	novel1219AssertTwoPages(t, api, novel1219RunDependent(t, api), 3)
}

// P2-2, dependent loop, short page with a numeric continuation: not the end either.
func TestNovel1219DependentShortPageWithNumericTokenAdvances(t *testing.T) {
	api := &novel1219PageAPI{firstRows: 1, token: "2"}
	novel1219AssertTwoPages(t, api, novel1219RunDependent(t, api), 3)
}

// P2-2, flat loop: a short page whose continuation is the numeric next page goes on to that page.
func TestNovel1219SyncShortPageWithNumericTokenAdvances(t *testing.T) {
	s := &novel1219SyncServer{firstRows: 3, token: true, numericToken: true}
	dir := novel1219SyncEnv(t, s.handler(t))
	out, err := novel1219Sync(t, filepath.Join(dir, "a.db"), "--full")
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		t.Fatalf("sync: %v (requests %q)", err, s.reqs)
	}
	if !strings.Contains(out, `"total_records": 5`) || len(s.reqs) < 2 || !strings.Contains(s.reqs[1], "page=2") {
		t.Fatalf("a short page with next_cursor \"2\" ended the walk (requests %q): %s", s.reqs, out)
	}
}

func TestNovel1219NormalizePageContinuation(t *testing.T) {
	page := paginationDefaults{cursorParam: "page", cursorType: "page"}
	cases := []struct {
		cursor, next     string
		hasMore          bool
		wantNext         string
		wantMore, proved bool
	}{
		{"", novel1219Keyset, false, "2", true, true},
		{"3", novel1219Keyset, false, "4", true, true},
		{"1", "2", false, "2", true, true},
		{"1", "", false, "", false, false},
		{"1", "", true, "", true, false},
		{"2", "1", true, "1", true, false},
		{"2", "2", false, "2", false, false},
		{"", "1", true, "1", true, false},
	}
	for _, tc := range cases {
		next, more, proved := normalizePageContinuation(page, tc.cursor, tc.next, tc.hasMore)
		if next != tc.wantNext || more != tc.wantMore || proved != tc.proved {
			t.Errorf("normalize(cursor %q, next %q, more %v) = %q %v %v; want %q %v %v", tc.cursor, tc.next, tc.hasMore, next, more, proved, tc.wantNext, tc.wantMore, tc.proved)
		}
	}
	cursorType := paginationDefaults{cursorParam: "cursor", cursorType: "cursor"}
	if next, more, proved := normalizePageContinuation(cursorType, "", novel1219Keyset, true); next != novel1219Keyset || !more || proved {
		t.Errorf("cursor pagination must pass its token through unchanged; got %q %v %v", next, more, proved)
	}
}

// PATCH(novel1219-activities-list-page-number): Codex novel1219-r20 (1 P2). A page API that answers page=1 with a
// next of "2" and page=2 with a next of "1" must not alternate forever: a numeric continuation proves more only when it
// advances, and syncPageVisits ends a walk that asks for a page twice.

// novel1219CycleAPI serves rows per page and a numeric next that points page 2 back at page 1.
type novel1219CycleAPI struct {
	rows  int
	pages []string
}

// page returns the rows and the next token for a requested page: page 2 points back at "1", any other page at "2".
func (a *novel1219CycleAPI) page(p string) (rows, next string) {
	next, page := `"2"`, 1
	if p == "2" {
		next, page = `"1"`, 2
	}
	out := make([]string, a.rows)
	for i := range out {
		out[i] = `{"id":"row-` + strconv.Itoa(page) + `-` + strconv.Itoa(i) + `","createdAt":"2026-11-0` + strconv.Itoa(9-page) + `T00:00:00Z"}`
	}
	return "[" + strings.Join(out, ",") + "]", next
}

func (a *novel1219CycleAPI) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	a.pages = append(a.pages, params["page"])
	if len(a.pages) > 6 {
		return nil, &client.APIError{Method: "GET", Path: "/cycle", StatusCode: 599, Body: "test: the walk looped"}
	}
	rows, next := a.page(params["page"])
	return json.RawMessage(`{"data":` + rows + `,"meta":{"next_cursor":` + next + `}}`), nil
}

func (a *novel1219CycleAPI) RateLimit() float64 { return 0 }

func novel1219RunDependentCycle(t *testing.T, api *novel1219CycleAPI) parentReport {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	dep := dependentResourceDef{Name: "activities", ParentTable: "parents", PathTemplate: "/parents/{id}/children", PathParams: []dependentPathParamDef{{Param: "id", Field: "id"}}}
	pageSize := paginationDefaults{cursorParam: "page", cursorType: "page", initialCursor: "1", limitParam: "per_page", limit: 3}
	return syncOneParent(context.Background(), api, db, dep, map[string]string{"id": "p1"}, dep.PathParams, pageSize, "", "", 0, false, false, nil, io.Discard, "")
}

// Short A/B pages: page 2's backward "1" proves nothing, so the short page ends the walk after two requests.
func TestNovel1219DependentBackwardNumericTokenOnShortPageEnds(t *testing.T) {
	api := &novel1219CycleAPI{rows: 1}
	rep := novel1219RunDependentCycle(t, api)
	if len(api.pages) != 2 || rep.failure != nil || !rep.complete || rep.stored != 2 {
		t.Fatalf("pages %q stored %d complete %v failure %v; want page one, page=2, then the end", api.pages, rep.stored, rep.complete, rep.failure)
	}
}

// Full A/B pages: following page 2's "1" would revisit page one; the walk ends with an error instead of looping.
func TestNovel1219DependentRevisitedPageEndsWithAnError(t *testing.T) {
	api := &novel1219CycleAPI{rows: 3}
	rep := novel1219RunDependentCycle(t, api)
	if len(api.pages) != 2 || rep.failure == nil || rep.complete || !strings.Contains(rep.failure.Error(), "twice") {
		t.Fatalf("pages %q complete %v failure %v; want two requests and a revisit error", api.pages, rep.complete, rep.failure)
	}
}

func novel1219FlatCycle(t *testing.T, rows int) (pages []string, out string, err error) {
	t.Helper()
	api := &novel1219CycleAPI{rows: rows}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		w.Header().Set("Content-Type", "application/json")
		if len(pages) > 6 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"test: the walk looped"}`)
			return
		}
		rows, next := api.page(p)
		_, _ = io.WriteString(w, `{"data":{"data":`+rows+`,"meta":{"per_page":50,"next_cursor":`+next+`}}}`)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", dir)
	out, err = novel1219Sync(t, filepath.Join(dir, "a.db"), "--full")
	mu.Lock()
	defer mu.Unlock()
	return append([]string(nil), pages...), out, err
}

// Flat loop, short A/B pages (activities sync pages 50 at a time): two requests, then the end.
func TestNovel1219SyncBackwardNumericTokenOnShortPageEnds(t *testing.T) {
	pages, out, err := novel1219FlatCycle(t, 1)
	if err != nil || len(pages) != 2 || !strings.Contains(out, `"total_records": 2`) {
		t.Fatalf("pages %q err %v; want page one, page=2, then the end with 2 records: %s", pages, err, out)
	}
}

// Flat loop, full A/B pages: the revisit of page one ends the sync with an error after two requests.
func TestNovel1219SyncRevisitedPageEndsWithAnError(t *testing.T) {
	pages, out, err := novel1219FlatCycle(t, 50)
	if err == nil || len(pages) != 2 {
		t.Fatalf("pages %q err %v; want two requests and a revisit error: %s", pages, err, out)
	}
}

// PATCH(novel1219-activities-list-page-number): Codex novel1219-r21 (1 P2). Page 1 is full with next "2"; page 2 is
// short and repeats next "2". The repeat does not advance, so it proves nothing, and the short page is a natural end:
// the walk completes (reconcile and watermark run) instead of stopping as stuck_cursor.

type novel1219SelfAPI struct {
	limit int
	pages []string
}

func (a *novel1219SelfAPI) page(p string) string {
	n, page := a.limit, 1
	if p == "2" {
		n, page = 1, 2
	}
	out := make([]string, n)
	for i := range out {
		out[i] = `{"id":"self-` + strconv.Itoa(page) + `-` + strconv.Itoa(i) + `","createdAt":"2026-11-0` + strconv.Itoa(9-page) + `T00:00:00Z"}`
	}
	return "[" + strings.Join(out, ",") + "]"
}

func (a *novel1219SelfAPI) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	a.pages = append(a.pages, params["page"])
	if len(a.pages) > 6 {
		return nil, &client.APIError{Method: "GET", Path: "/self", StatusCode: 599, Body: "test: the walk looped"}
	}
	return json.RawMessage(`{"data":` + a.page(params["page"]) + `,"meta":{"next_cursor":"2"}}`), nil
}

func (a *novel1219SelfAPI) RateLimit() float64 { return 0 }

func TestNovel1219DependentShortPageRepeatingItsNextCompletes(t *testing.T) {
	api := &novel1219SelfAPI{limit: 3}
	db, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	dep := dependentResourceDef{Name: "activities", ParentTable: "parents", PathTemplate: "/parents/{id}/children", PathParams: []dependentPathParamDef{{Param: "id", Field: "id"}}}
	pageSize := paginationDefaults{cursorParam: "page", cursorType: "page", initialCursor: "1", limitParam: "per_page", limit: 3}
	rep := syncOneParent(context.Background(), api, db, dep, map[string]string{"id": "p1"}, dep.PathParams, pageSize, "", "", 0, false, false, nil, io.Discard, "")
	if len(api.pages) != 2 || rep.failure != nil || !rep.complete || rep.reason != "" || rep.stored != 4 {
		t.Fatalf("pages %q stored %d complete %v reason %q failure %v; want two requests, 4 rows, complete", api.pages, rep.stored, rep.complete, rep.reason, rep.failure)
	}
}

func TestNovel1219SyncShortPageRepeatingItsNextCompletes(t *testing.T) {
	api := &novel1219SelfAPI{limit: 50}
	var mu sync.Mutex
	var pages []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		p := r.URL.Query().Get("page")
		pages = append(pages, p)
		w.Header().Set("Content-Type", "application/json")
		if len(pages) > 6 {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":"test: the walk looped"}`)
			return
		}
		_, _ = io.WriteString(w, `{"data":{"data":`+api.page(p)+`,"meta":{"per_page":50,"next_cursor":"2"}}}`)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("CONDUYT_CRM_NO_LEARN", "true")
	t.Setenv("HOME", dir)
	var events strings.Builder
	root := newRootCmd(&rootFlags{})
	root.SetArgs([]string{"sync", "--resources", "activities", "--full", "--db", filepath.Join(dir, "a.db"), "--json"})
	root.SetOut(&events)
	root.SetErr(&events)
	err := root.Execute()
	mu.Lock()
	defer mu.Unlock()
	out := events.String()
	if err != nil || len(pages) != 2 || !strings.Contains(out, `"total_records": 51`) || strings.Contains(out, "stuck_pagination") {
		t.Fatalf("pages %q err %v; want two requests, 51 records, no stuck_pagination: %s", pages, err, out)
	}
	db, err := store.Open(filepath.Join(dir, "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var complete int
	if err := db.DB().QueryRow(`SELECT last_attempt_complete FROM sync_state WHERE resource_type = 'activities'`).Scan(&complete); err != nil || complete != 1 {
		t.Fatalf("sync_state last_attempt_complete = %d (err %v), want 1: the walk must finish as a natural end", complete, err)
	}
}
