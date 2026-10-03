// Copyright 2026 Brad Knight and contributors. Licensed under Apache-2.0. See LICENSE.
//
// steam_demos_test.go - offline tests for the `steam demos` command and the
// `has_demo` field on `steam app`. Every case runs against an httptest
// server; nothing here touches the network.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/game-goat/internal/source/steam"
)

const (
	demosQueryPath      = "/IStoreQueryService/Query/v1/"
	demosSearchPath     = "/IStoreQueryService/SearchSuggestions/v1/"
	demosGetItemsPath   = "/IStoreBrowseService/GetItems/v1/"
	demosGetTagListPath = "/IStoreService/GetTagList/v1/"
)

// demosTagListJSON is the shared tag dictionary fixture (tagid -> name). The
// client caches it, so one test serves it once for every tag lookup.
const demosTagListJSON = `{"response":{"tags":[{"tagid":1716,"name":"Roguelike"},{"tagid":1628,"name":"Metroidvania"}]}}`

// demosReqLog records every request path and the decoded input_json payload so a
// test can assert both the request count (budget) and the encoded filters.
type demosReqLog struct {
	mu       sync.Mutex
	counts   map[string]int
	payloads map[string][]map[string]any
}

func newDemosReqLog() *demosReqLog {
	return &demosReqLog{counts: map[string]int{}, payloads: map[string][]map[string]any{}}
}

func (l *demosReqLog) record(r *http.Request) map[string]any {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.counts[r.URL.Path]++
	var p map[string]any
	if raw := r.URL.Query().Get("input_json"); raw != "" {
		_ = json.Unmarshal([]byte(raw), &p)
	}
	l.payloads[r.URL.Path] = append(l.payloads[r.URL.Path], p)
	return p
}

func (l *demosReqLog) count(path string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.counts[path]
}

func (l *demosReqLog) total() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := 0
	for _, c := range l.counts {
		n += c
	}
	return n
}

func (l *demosReqLog) last(t *testing.T, path string) map[string]any {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.payloads[path]
	if len(list) == 0 {
		t.Fatalf("no request recorded for %s (have %v)", path, l.counts)
	}
	return list[len(list)-1]
}

// demosIsolateEnv clears the locale env so a developer machine's STEAM_COUNTRY /
// STEAM_LANG cannot change the fixtures.
func demosIsolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"STEAM_COUNTRY", "ITAD_COUNTRY", "STEAM_LANG", "STEAM_STORE_BASE_URL", "STEAM_STORE_API_BASE_URL"} {
		t.Setenv(k, "")
	}
}

// withSteamHook points the per-request Steam client at srv for the test.
func withSteamHook(t *testing.T, srv *httptest.Server) {
	t.Helper()
	old := steamClientHook
	steamClientHook = func(c *steam.Client) {
		c.BaseURL = srv.URL
		c.APIBaseURL = srv.URL
	}
	t.Cleanup(func() { steamClientHook = old })
}

func demoItemJSON(appid int64, name string, parent int64) string {
	related := ""
	if parent > 0 {
		related = fmt.Sprintf(`,"related_items":{"parent_appid":%d}`, parent)
	}
	return fmt.Sprintf(`{"item_type":0,"id":%d,"appid":%d,"success":1,"name":%q,"type":1,"is_free":true%s}`, appid, appid, name, related)
}

// demoItemJSONWithTags is demoItemJSON plus a weighted-tags array so a test can
// pin tag names. tagsJSON is raw, e.g. `{"tagid":1716,"weight":7}`.
func demoItemJSONWithTags(appid int64, name string, parent int64, tagsJSON string) string {
	related := ""
	if parent > 0 {
		related = fmt.Sprintf(`,"related_items":{"parent_appid":%d}`, parent)
	}
	tags := ""
	if tagsJSON != "" {
		tags = `,"tags":[` + tagsJSON + `]`
	}
	return fmt.Sprintf(`{"item_type":0,"id":%d,"appid":%d,"success":1,"name":%q,"type":1,"is_free":true%s%s}`, appid, appid, name, related, tags)
}

// rowTagNames reads the attached tag names off a JSON result row.
func rowTagNames(row map[string]any) []string {
	raw, _ := row["tags"].([]any)
	out := make([]string, 0, len(raw))
	for _, tv := range raw {
		tm, _ := tv.(map[string]any)
		name, _ := tm["name"].(string)
		out = append(out, name)
	}
	return out
}

func manyDemoItems(n int) []string {
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, demoItemJSON(int64(900000+i), fmt.Sprintf("Demo %d", i), 0))
	}
	return out
}

func storeResponse(total, start, count int, items []string) string {
	return fmt.Sprintf(`{"response":{"metadata":{"total_matching_records":%d,"start":%d,"count":%d},"store_items":[%s]}}`, total, start, count, strings.Join(items, ","))
}

func nestedMap(t *testing.T, m map[string]any, keys ...string) map[string]any {
	t.Helper()
	cur := m
	for _, k := range keys {
		next, ok := cur[k].(map[string]any)
		if !ok {
			t.Fatalf("payload missing object %q; have keys %v", k, cur)
		}
		cur = next
	}
	return cur
}

func getItemsIDs(t *testing.T, payload map[string]any) []int64 {
	t.Helper()
	raw, ok := payload["ids"].([]any)
	if !ok {
		t.Fatalf("GetItems payload missing ids: %v", payload)
	}
	ids := make([]int64, 0, len(raw))
	for _, x := range raw {
		m, ok := x.(map[string]any)
		if !ok {
			t.Fatalf("ids entry is not an object: %v", x)
		}
		ids = append(ids, int64(m["appid"].(float64)))
	}
	return ids
}

func tagGroupsFromFilters(t *testing.T, filters map[string]any) [][]int {
	t.Helper()
	raw, ok := filters["tagids_must_match"].([]any)
	if !ok {
		t.Fatalf("tagids_must_match missing from filters: %v", filters)
	}
	groups := make([][]int, 0, len(raw))
	for _, g := range raw {
		gm, ok := g.(map[string]any)
		if !ok {
			t.Fatalf("tag group is not an object: %v", g)
		}
		idsAny, ok := gm["tagids"].([]any)
		if !ok {
			t.Fatalf("tag group missing tagids: %v", gm)
		}
		ids := make([]int, 0, len(idsAny))
		for _, id := range idsAny {
			ids = append(ids, int(id.(float64)))
		}
		groups = append(groups, ids)
	}
	return groups
}

func runDemosCmd(t *testing.T, args ...string) (map[string]any, string, error) {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := newSteamDemosCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if out.Len() == 0 {
		return nil, errOut.String(), err
	}
	var env map[string]any
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		return nil, errOut.String(), fmt.Errorf("stdout is not JSON (%v): %q", uerr, out.String())
	}
	return env, errOut.String(), err
}

func TestDemosQueryEncodesDemoOnlyType(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(600, "Demo One", 0)}))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	if _, _, err := runDemosCmd(t, "--limit", "20"); err != nil {
		t.Fatalf("steam demos: %v", err)
	}
	filters := nestedMap(t, log.last(t, demosQueryPath), "query", "filters")
	if got, want := filters["type_filters"], (map[string]any{"include_demos": true}); !reflect.DeepEqual(got, want) {
		t.Errorf("type_filters = %#v, want exactly %#v", got, want)
	}
	if filters["released_only"] != true {
		t.Errorf("released_only = %v, want true (default is released)", filters["released_only"])
	}
	if _, ok := filters["coming_soon_only"]; ok {
		t.Errorf("coming_soon_only must be absent by default, got %v", filters["coming_soon_only"])
	}
}

func TestDemosTitleDefaultsToReleasedOnly(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case demosSearchPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(600, "Demo One", 0)}))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	if _, _, err := runDemosCmd(t, "--title", "portal", "--limit", "1000"); err != nil {
		t.Fatalf("steam demos --title: %v", err)
	}
	filters := nestedMap(t, log.last(t, demosSearchPath), "filters")
	if filters["released_only"] != true {
		t.Errorf("released_only = %v, want true (title path defaults to released)", filters["released_only"])
	}
	if _, ok := filters["coming_soon_only"]; ok {
		t.Errorf("coming_soon_only must be absent by default, got %v", filters["coming_soon_only"])
	}
}

func TestDemosComingSoonSwapsReleaseFilter(t *testing.T) {
	t.Run("browse path", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			if r.URL.Path == demosGetTagListPath {
				fmt.Fprint(w, demosTagListJSON)
				return
			}
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(601, "Demo Two", 0)}))
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		if _, _, err := runDemosCmd(t, "--coming-soon"); err != nil {
			t.Fatalf("steam demos --coming-soon: %v", err)
		}
		filters := nestedMap(t, log.last(t, demosQueryPath), "query", "filters")
		if filters["coming_soon_only"] != true {
			t.Errorf("coming_soon_only = %v, want true", filters["coming_soon_only"])
		}
		if _, ok := filters["released_only"]; ok {
			t.Errorf("released_only must be absent under --coming-soon, got %v", filters["released_only"])
		}
	})

	t.Run("title path", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			switch r.URL.Path {
			case demosSearchPath:
				fmt.Fprint(w, storeResponse(273, 0, 1, []string{demoItemJSON(602, "Demo Three", 0)}))
			case demosGetTagListPath:
				fmt.Fprint(w, demosTagListJSON)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		env, _, err := runDemosCmd(t, "--title", "portal", "--coming-soon", "--limit", "1000")
		if err != nil {
			t.Fatalf("steam demos --title --coming-soon: %v", err)
		}
		filters := nestedMap(t, log.last(t, demosSearchPath), "filters")
		if filters["coming_soon_only"] != true {
			t.Errorf("coming_soon_only = %v, want true", filters["coming_soon_only"])
		}
		if _, ok := filters["released_only"]; ok {
			t.Errorf("released_only must be absent under --coming-soon, got %v", filters["released_only"])
		}
		meta, _ := env["meta"].(map[string]any)
		if meta["total"] != float64(273) {
			t.Errorf("meta.total = %v, want 273 (the service total, not a filtered count)", meta["total"])
		}
	})
}

func TestDemosPageIsThreeRequestsWithTagNames(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	items := []string{
		demoItemJSONWithTags(1001, "Demo A", 101, `{"tagid":1716,"weight":7},{"tagid":1628,"weight":3}`),
		demoItemJSONWithTags(1002, "Demo B", 101, `{"tagid":1716,"weight":5}`),
		demoItemJSONWithTags(1003, "Demo C", 102, `{"tagid":1628,"weight":2}`),
		demoItemJSONWithTags(1004, "Demo D", 103, `{"tagid":1716,"weight":1}`),
		demoItemJSONWithTags(1005, "Demo E", 103, ""),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(50, 0, 5, items))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				parts = append(parts, fmt.Sprintf(`{"appid":%d,"success":1,"visible":true,"name":"Full game %d"}`, id, id))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, _, err := runDemosCmd(t, "--limit", "20")
	if err != nil {
		t.Fatalf("steam demos: %v", err)
	}
	if got := log.count(demosQueryPath); got != 1 {
		t.Errorf("Query requests = %d, want 1", got)
	}
	if got := log.count(demosGetTagListPath); got != 1 {
		t.Errorf("GetTagList requests = %d, want 1 (tag dictionary for row names)", got)
	}
	if got := log.count(demosGetItemsPath); got != 1 {
		t.Errorf("GetItems requests = %d, want 1 (one lookup for every parent)", got)
	}
	if got := log.total(); got != 3 {
		t.Errorf("total requests = %d, want exactly 3 (Query + GetTagList + GetItems)", got)
	}
	wantIDs := []int64{101, 102, 103}
	if got := getItemsIDs(t, log.last(t, demosGetItemsPath)); !reflect.DeepEqual(got, wantIDs) {
		t.Errorf("GetItems ids = %v, want the 3 unique parents %v", got, wantIDs)
	}
	results, _ := env["results"].([]any)
	if len(results) != 5 {
		t.Fatalf("results = %d, want 5", len(results))
	}
	wantParents := []string{"Full game 101", "Full game 101", "Full game 102", "Full game 103", "Full game 103"}
	wantTags := [][]string{{"Roguelike", "Metroidvania"}, {"Roguelike"}, {"Metroidvania"}, {"Roguelike"}, {}}
	for i, r := range results {
		row := r.(map[string]any)
		if row["parent_name"] != wantParents[i] {
			t.Errorf("results[%d].parent_name = %v, want %q", i, row["parent_name"], wantParents[i])
		}
		if got := rowTagNames(row); !reflect.DeepEqual(got, wantTags[i]) {
			t.Errorf("results[%d] tag names = %v, want %v", i, got, wantTags[i])
		}
	}
	meta, _ := env["meta"].(map[string]any)
	if meta["next_page"] != float64(2) {
		t.Errorf("meta.next_page = %v, want 2 when total > page window", meta["next_page"])
	}
}

func TestDemosParentLookupFailureDegrades(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(1001, "Demo A", 101)}))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, errOut, err := runDemosCmd(t, "--limit", "20")
	if err != nil {
		t.Fatalf("parent lookup failure must not fail the command: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (rows survive)", len(results))
	}
	if _, ok := results[0].(map[string]any)["parent_name"]; ok {
		t.Errorf("parent_name must be absent when the lookup fails")
	}
	meta, _ := env["meta"].(map[string]any)
	missing, _ := meta["sources_missing"].([]any)
	found := false
	for _, m := range missing {
		if m == "steam_parent" {
			found = true
		}
	}
	if !found {
		t.Errorf("meta.sources_missing = %v, want to contain steam_parent", meta["sources_missing"])
	}
	if !strings.Contains(errOut, "steam_parent") {
		t.Errorf("stderr = %q, want a one-line warning naming steam_parent", errOut)
	}
}

func TestDemosTitleSetsTruncatedWhenTotalExceedsReturned(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			switch r.URL.Path {
			case demosSearchPath:
				fmt.Fprint(w, storeResponse(1500, 0, 1000, manyDemoItems(1000)))
			case demosGetTagListPath:
				fmt.Fprint(w, demosTagListJSON)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		env, _, err := runDemosCmd(t, "--title", "portal", "--limit", "1000")
		if err != nil {
			t.Fatalf("steam demos --title: %v", err)
		}
		meta, _ := env["meta"].(map[string]any)
		if meta["truncated"] != true {
			t.Errorf("meta.truncated = %v, want true (total 1500 > 1000 returned)", meta["truncated"])
		}
		if _, ok := meta["next_page"]; ok {
			t.Errorf("title path must not carry next_page, got %v", meta["next_page"])
		}
		if got := log.count(demosSearchPath); got != 1 {
			t.Errorf("SearchSuggestions requests = %d, want 1", got)
		}
	})
	t.Run("not truncated field present", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			if r.URL.Path == demosGetTagListPath {
				fmt.Fprint(w, demosTagListJSON)
				return
			}
			fmt.Fprint(w, storeResponse(25, 0, 25, manyDemoItems(25)))
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		env, _, err := runDemosCmd(t, "--title", "portal", "--limit", "1000")
		if err != nil {
			t.Fatalf("steam demos --title: %v", err)
		}
		meta, _ := env["meta"].(map[string]any)
		val, present := meta["truncated"]
		if !present {
			t.Fatalf("meta must carry truncated on the title path even when false: %v", meta)
		}
		if val != false {
			t.Errorf("meta.truncated = %v, want false (25 of 25)", val)
		}
		if _, ok := meta["next_page"]; ok {
			t.Errorf("title path must not carry next_page, got %v", meta["next_page"])
		}
	})
}

func TestDemosTitleRejectsPage(t *testing.T) {
	demosIsolateEnv(t)
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	flags := &rootFlags{asJSON: true}
	cmd := newSteamDemosCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"--title", "portal", "--page", "2"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("--page with --title must be a usage error")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode = %d, want 2 (usage error)", got)
	}
	if !strings.Contains(err.Error(), "ignores offsets") {
		t.Errorf("error = %q, want it to explain the endpoint ignores offsets", err.Error())
	}
	if requests != 0 {
		t.Errorf("HTTP requests = %d, want 0 (rejected before any fetch)", requests)
	}
}

func TestDemosPageWithTagFlagIsStillThreeRequests(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSONWithTags(1001, "Demo A", 101, `{"tagid":1716,"weight":7}`)}))
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				parts = append(parts, fmt.Sprintf(`{"appid":%d,"success":1,"visible":true,"name":"Full game %d"}`, id, id))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	if _, _, err := runDemosCmd(t, "--tag", "Roguelike"); err != nil {
		t.Fatalf("steam demos --tag: %v", err)
	}
	if got := log.count(demosGetTagListPath); got != 1 {
		t.Errorf("GetTagList requests = %d, want 1 (fetched once and reused for --tag resolution)", got)
	}
	if got := log.count(demosQueryPath); got != 1 {
		t.Errorf("Query requests = %d, want 1", got)
	}
	if got := log.count(demosGetItemsPath); got != 1 {
		t.Errorf("GetItems requests = %d, want 1", got)
	}
	if got := log.total(); got != 3 {
		t.Errorf("total requests = %d, want exactly 3 even with --tag (dictionary is shared)", got)
	}
	filters := nestedMap(t, log.last(t, demosQueryPath), "query", "filters")
	if got := tagGroupsFromFilters(t, filters); !reflect.DeepEqual(got, [][]int{{1716}}) {
		t.Errorf("tagids_must_match = %v, want [[1716]]", got)
	}
}

func TestDemosTitleDefaultLimitIs100(t *testing.T) {
	t.Run("title default", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			switch r.URL.Path {
			case demosSearchPath:
				fmt.Fprint(w, storeResponse(0, 0, 0, nil))
			case demosGetTagListPath:
				fmt.Fprint(w, demosTagListJSON)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		if _, _, err := runDemosCmd(t, "--title", "portal"); err != nil {
			t.Fatalf("steam demos --title: %v", err)
		}
		if got := log.last(t, demosSearchPath)["max_results"]; got != float64(100) {
			t.Errorf("SearchSuggestions max_results = %v, want 100 (--title default when --limit is unset)", got)
		}
	})
	t.Run("title explicit limit", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			switch r.URL.Path {
			case demosSearchPath:
				fmt.Fprint(w, storeResponse(0, 0, 0, nil))
			case demosGetTagListPath:
				fmt.Fprint(w, demosTagListJSON)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		if _, _, err := runDemosCmd(t, "--title", "portal", "--limit", "1000"); err != nil {
			t.Fatalf("steam demos --title --limit: %v", err)
		}
		if got := log.last(t, demosSearchPath)["max_results"]; got != float64(1000) {
			t.Errorf("SearchSuggestions max_results = %v, want 1000 (explicit --limit wins)", got)
		}
	})
	t.Run("browse default", func(t *testing.T) {
		demosIsolateEnv(t)
		log := newDemosReqLog()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.record(r)
			switch r.URL.Path {
			case demosQueryPath:
				fmt.Fprint(w, storeResponse(0, 0, 0, nil))
			case demosGetTagListPath:
				fmt.Fprint(w, demosTagListJSON)
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		defer srv.Close()
		withSteamHook(t, srv)

		if _, _, err := runDemosCmd(t); err != nil {
			t.Fatalf("steam demos: %v", err)
		}
		if got := int(nestedMap(t, log.last(t, demosQueryPath), "query")["count"].(float64)); got != 20 {
			t.Errorf("Query count = %d, want 20 (browse default without --title)", got)
		}
	})
}

func TestDemosBareCommandLists(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(600, "Demo One", 0)}))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, _, err := runDemosCmd(t)
	if err != nil {
		t.Fatalf("bare steam demos must list, not print help: %v", err)
	}
	if env == nil {
		t.Fatal("bare steam demos printed no JSON results")
	}
	if got := log.count(demosQueryPath); got != 1 {
		t.Errorf("Query requests = %d, want 1 (bare command lists the first page)", got)
	}
	results, _ := env["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
}

func TestDemosTagsAreOneGroupEach(t *testing.T) {
	const tagList = `{"response":{"tags":[{"tagid":1716,"name":"Roguelike"},{"tagid":1628,"name":"Metroidvania"}]}}`
	want := [][]int{{1716}, {1628}}

	cases := []struct {
		name string
		args []string
		path string
	}{
		{"browse repeated tags", []string{"--tag", "Roguelike", "--tag", "Metroidvania"}, demosQueryPath},
		{"browse comma tags", []string{"--tag", "Roguelike,Metroidvania"}, demosQueryPath},
		{"title comma tags", []string{"--title", "x", "--tag", "Roguelike,Metroidvania"}, demosSearchPath},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			demosIsolateEnv(t)
			log := newDemosReqLog()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				log.record(r)
				switch r.URL.Path {
				case demosGetTagListPath:
					fmt.Fprint(w, tagList)
				case demosQueryPath:
					fmt.Fprint(w, storeResponse(0, 0, 0, nil))
				case demosSearchPath:
					fmt.Fprint(w, storeResponse(0, 0, 0, nil))
				case demosGetItemsPath:
					fmt.Fprint(w, `{"response":{"store_items":[]}}`)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			withSteamHook(t, srv)

			if _, _, err := runDemosCmd(t, tc.args...); err != nil {
				t.Fatalf("steam demos: %v", err)
			}
			if got := log.count(demosGetTagListPath); got != 1 {
				t.Errorf("GetTagList requests = %d, want 1 (dictionary is cached per client)", got)
			}
			payload := log.last(t, tc.path)
			var filters map[string]any
			if tc.path == demosQueryPath {
				filters = nestedMap(t, payload, "query", "filters")
			} else {
				filters = nestedMap(t, payload, "filters")
			}
			if got := tagGroupsFromFilters(t, filters); !reflect.DeepEqual(got, want) {
				t.Errorf("tagids_must_match = %v, want exactly one group per tag %v", got, want)
			}
		})
	}
}

func TestSteamAppRowHasDemo(t *testing.T) {
	const tagList = `{"response":{"tags":[{"tagid":19,"name":"Action"}]}}`
	cases := []struct {
		name       string
		storeItem  string
		wantHasDem bool
	}{
		{
			name:       "demo links present",
			storeItem:  `{"item_type":0,"id":379720,"appid":379720,"success":1,"visible":true,"name":"DOOM","type":0,"related_items":{"demo_appid":[479030],"demos":[{"appid":479030}]}}`,
			wantHasDem: true,
		},
		{
			name:       "no demo links",
			storeItem:  `{"item_type":0,"id":1199790,"appid":1199790,"success":1,"visible":true,"name":"DOOM Eternal","type":0}`,
			wantHasDem: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			demosIsolateEnv(t)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case demosGetItemsPath:
					fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, tc.storeItem)
				case demosGetTagListPath:
					fmt.Fprint(w, tagList)
				case "/appreviews/379720", "/appreviews/1199790":
					fmt.Fprint(w, `{"success":1,"query_summary":{"review_score_desc":"Very Positive","total_positive":9,"total_negative":1,"total_reviews":10,"review_score":9}}`)
				default:
					t.Errorf("unexpected path %s", r.URL.Path)
				}
			}))
			defer srv.Close()
			withSteamHook(t, srv)

			var appid string
			if tc.wantHasDem {
				appid = "379720"
			} else {
				appid = "1199790"
			}
			flags := &rootFlags{asJSON: true}
			cmd := newSteamAppCmd(flags)
			var out, errOut bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs([]string{appid})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("steam app: %v (stderr %s)", err, errOut.String())
			}
			var env struct {
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &env); err != nil {
				t.Fatalf("stdout is not JSON: %v (%s)", err, out.String())
			}
			if len(env.Results) != 1 {
				t.Fatalf("results = %d, want 1", len(env.Results))
			}
			row := env.Results[0]
			val, present := row["has_demo"]
			if !present {
				t.Fatalf("has_demo must always be present, got row %v", row)
			}
			if val != tc.wantHasDem {
				t.Errorf("has_demo = %v, want %v", val, tc.wantHasDem)
			}
		})
	}
}

// parentItemJSON is a GetItems fixture record for a full game. When tagsJSON is
// non-empty it carries the weighted tags array Steam returns for
// include_tag_count. tagsJSON is raw, e.g. `{"tagid":1716,"weight":7}`.
func parentItemJSON(appid int64, name, tagsJSON string) string {
	tags := ""
	if tagsJSON != "" {
		tags = `,"tags":[` + tagsJSON + `]`
	}
	return fmt.Sprintf(`{"appid":%d,"success":1,"visible":true,"name":%q%s}`, appid, name, tags)
}

// hiddenItemJSON mimics a GetItems record for an app hidden from anonymous
// requests (age or region gate): success:15, visible:false, appid:0.
func hiddenItemJSON(appid int64) string {
	return fmt.Sprintf(`{"id":%d,"appid":0,"success":15,"visible":false,"name":""}`, appid)
}

// runDemosAgentCmd executes steam demos with the same flags the real --agent
// path sets (asJSON + agent imply compact) and returns the decoded envelope.
func runDemosAgentCmd(t *testing.T, args ...string) map[string]any {
	t.Helper()
	flags := &rootFlags{asJSON: true, agent: true, compact: true}
	cmd := newSteamDemosCmd(flags)
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("steam demos --agent: %v stderr=%s", err, errOut.String())
	}
	var env map[string]any
	if uerr := json.Unmarshal(out.Bytes(), &env); uerr != nil {
		t.Fatalf("stdout is not JSON (%v): %q", uerr, out.String())
	}
	return env
}

// TestDemosParentTagsFromFullGame: a demo with no store tags of its own whose
// full game has tags must surface those tag NAMES on the row as parent_tags, in
// Steam's order. The demo's own (empty) tags stay omitted so the row has no
// "tags" key. The GetItems fixture only returns tags when the request asks for
// include_tag_count, so a dropped request field fails this test.
func TestDemosParentTagsFromFullGame(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(1001, "Demo A", 101)}))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			dataReq, _ := p["data_request"].(map[string]any)
			tagsJSON := ""
			if dataReq != nil && dataReq["include_tag_count"] != nil {
				tagsJSON = `{"tagid":1716,"weight":7},{"tagid":1628,"weight":3}`
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, parentItemJSON(101, "Full game 101", tagsJSON))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, _, err := runDemosCmd(t, "--limit", "20")
	if err != nil {
		t.Fatalf("steam demos: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	row := results[0].(map[string]any)
	want := []any{"Roguelike", "Metroidvania"}
	got, _ := row["parent_tags"].([]any)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parent_tags = %v, want the full game's tag names in order %v", row["parent_tags"], want)
	}
	if _, ok := row["tags"]; ok {
		t.Errorf("the demo has no own tags, so the row must omit the tags key, got %v", row["tags"])
	}
}

// TestDemosParentTagsPageIsThreeRequests: a browse page of five demos over three
// unique parents stays at exactly three requests, and the one GetItems request
// (which now also yields the full games' tags) asks for include_tag_count.
func TestDemosParentTagsPageIsThreeRequests(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	items := []string{
		demoItemJSON(1001, "Demo A", 101),
		demoItemJSON(1002, "Demo B", 101),
		demoItemJSON(1003, "Demo C", 102),
		demoItemJSON(1004, "Demo D", 103),
		demoItemJSON(1005, "Demo E", 103),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(50, 0, 5, items))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				parts = append(parts, parentItemJSON(id, fmt.Sprintf("Full game %d", id), `{"tagid":1716,"weight":1}`))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	if _, _, err := runDemosCmd(t, "--limit", "20"); err != nil {
		t.Fatalf("steam demos: %v", err)
	}
	if got := log.count(demosQueryPath); got != 1 {
		t.Errorf("Query requests = %d, want 1", got)
	}
	if got := log.count(demosGetTagListPath); got != 1 {
		t.Errorf("GetTagList requests = %d, want 1 (shared dictionary)", got)
	}
	if got := log.count(demosGetItemsPath); got != 1 {
		t.Errorf("GetItems requests = %d, want 1 (names and tags in one lookup)", got)
	}
	if got := log.total(); got != 3 {
		t.Errorf("total requests = %d, want exactly 3 (Query + GetTagList + GetItems)", got)
	}
	wantIDs := []int64{101, 102, 103}
	if got := getItemsIDs(t, log.last(t, demosGetItemsPath)); !reflect.DeepEqual(got, wantIDs) {
		t.Errorf("GetItems ids = %v, want the 3 unique parents %v", got, wantIDs)
	}
	dataReq, _ := log.last(t, demosGetItemsPath)["data_request"].(map[string]any)
	if dataReq == nil || dataReq["include_tag_count"] == nil {
		t.Errorf("GetItems data_request must ask for tag counts, got %v", dataReq)
	}
}

// TestDemosParentTagsUnresolvableParentIsEmpty: a demo whose parent_appid is 0
// and one whose parent is hidden (success:15, visible:false) both yield rows
// with no parent_name and null parent_tags, while the resolvable rows stay
// populated. No error, exit 0.
func TestDemosParentTagsUnresolvableParentIsEmpty(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	const hiddenParent = int64(1245690)
	items := []string{
		demoItemJSON(1001, "Demo No Parent", 0),
		demoItemJSON(1002, "Demo Hidden Parent", hiddenParent),
		demoItemJSON(1003, "Demo Fine A", 201),
		demoItemJSON(1004, "Demo Fine B", 202),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(4, 0, 4, items))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				if id == hiddenParent {
					parts = append(parts, hiddenItemJSON(hiddenParent))
					continue
				}
				parts = append(parts, parentItemJSON(id, fmt.Sprintf("Full game %d", id), `{"tagid":1716,"weight":1}`))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, _, err := runDemosCmd(t, "--limit", "20")
	if err != nil {
		t.Fatalf("unresolvable parents must not fail the command: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 4 {
		t.Fatalf("results = %d, want 4", len(results))
	}
	for i, row := range []map[string]any{results[0].(map[string]any), results[1].(map[string]any)} {
		if row["parent_tags"] != nil {
			t.Errorf("results[%d].parent_tags = %v, want null when the parent cannot be resolved", i, row["parent_tags"])
		}
		if _, ok := row["parent_name"]; ok {
			t.Errorf("results[%d].parent_name must be absent when the parent cannot be resolved, got %v", i, row["parent_name"])
		}
	}
	for i := 2; i < 4; i++ {
		row := results[i].(map[string]any)
		if got, _ := row["parent_tags"].([]any); !reflect.DeepEqual(got, []any{"Roguelike"}) {
			t.Errorf("results[%d].parent_tags = %v, want [Roguelike]", i, row["parent_tags"])
		}
		if row["parent_name"] == nil {
			t.Errorf("results[%d].parent_name missing, want the resolved full-game name", i)
		}
	}
}

// TestDemosParentTagsSurviveCompact: parent_tags is sparse (one resolvable row
// out of five), so the 80% frequency rule in the --agent/--compact projector
// would drop it without the always-keep allowance.
func TestDemosParentTagsSurviveCompact(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	items := []string{
		demoItemJSON(1001, "Demo A", 101),
		demoItemJSON(1002, "Demo B", 0),
		demoItemJSON(1003, "Demo C", 0),
		demoItemJSON(1004, "Demo D", 0),
		demoItemJSON(1005, "Demo E", 0),
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(5, 0, 5, items))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				parts = append(parts, parentItemJSON(id, fmt.Sprintf("Full game %d", id), `{"tagid":1716,"weight":1}`))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env := runDemosAgentCmd(t, "--limit", "20")
	results, _ := env["results"].([]any)
	if len(results) != 5 {
		t.Fatalf("results = %d, want 5", len(results))
	}
	first := results[0].(map[string]any)
	if got, _ := first["parent_tags"].([]any); !reflect.DeepEqual(got, []any{"Roguelike"}) {
		t.Errorf("compact row parent_tags = %v, want [Roguelike] (dropped by compaction?)", first["parent_tags"])
	}
}

// TestDemosPartialParentLookupMarksSteamParent: a title batch whose parent
// appids span two GetItems chunks must keep the rows the first chunk resolved
// (parent_name present) and mark steam_parent, without failing the command.
// The second chunk answers malformed JSON (status 200) so there is no 5xx retry.
func TestDemosPartialParentLookupMarksSteamParent(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	items := make([]string, 0, 201)
	for i := 0; i < 201; i++ {
		items = append(items, demoItemJSON(int64(5000+i), fmt.Sprintf("Demo %d", i), int64(7000+i)))
	}
	var getItemsCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosSearchPath:
			fmt.Fprint(w, storeResponse(201, 0, 201, items))
		case demosGetTagListPath:
			fmt.Fprint(w, demosTagListJSON)
		case demosGetItemsPath:
			if atomic.AddInt32(&getItemsCalls, 1) == 1 {
				ids := getItemsIDs(t, p)
				parts := make([]string, 0, len(ids))
				for _, id := range ids {
					parts = append(parts, parentItemJSON(id, fmt.Sprintf("Full game %d", id), ""))
				}
				fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
				return
			}
			fmt.Fprint(w, `{"response":`)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, errOut, err := runDemosCmd(t, "--title", "portal", "--limit", "1000")
	if err != nil {
		t.Fatalf("partial parent lookup must not fail the command: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 201 {
		t.Fatalf("results = %d, want 201", len(results))
	}
	first := results[0].(map[string]any)
	if first["parent_name"] == nil {
		t.Errorf("results[0].parent_name missing, want the first-chunk full-game name")
	}
	last := results[200].(map[string]any)
	if _, ok := last["parent_name"]; ok {
		t.Errorf("results[200].parent_name = %v, want absent for the failed chunk", last["parent_name"])
	}
	meta, _ := env["meta"].(map[string]any)
	missing, _ := meta["sources_missing"].([]any)
	found := false
	for _, m := range missing {
		if m == "steam_parent" {
			found = true
		}
	}
	if !found {
		t.Errorf("meta.sources_missing = %v, want to contain steam_parent", meta["sources_missing"])
	}
	if !strings.Contains(errOut, "steam_parent") {
		t.Errorf("stderr = %q, want a one-line warning naming steam_parent", errOut)
	}
	if strings.Contains(errOut, "steam_tags") {
		t.Errorf("stderr = %q, must not name steam_tags when the tag list succeeded", errOut)
	}
}

// TestDemosTagListFailureMarksSteamTags: when the tag dictionary fails, the
// demo rows still carry the full game's NAME but no parent_tags, and the
// envelope marks steam_tags instead of failing.
func TestDemosTagListFailureMarksSteamTags(t *testing.T) {
	demosIsolateEnv(t)
	log := newDemosReqLog()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := log.record(r)
		switch r.URL.Path {
		case demosQueryPath:
			fmt.Fprint(w, storeResponse(1, 0, 1, []string{demoItemJSON(1001, "Demo A", 101)}))
		case demosGetTagListPath:
			fmt.Fprint(w, `{"response":`)
		case demosGetItemsPath:
			ids := getItemsIDs(t, p)
			parts := make([]string, 0, len(ids))
			for _, id := range ids {
				parts = append(parts, parentItemJSON(id, fmt.Sprintf("Full game %d", id), `{"tagid":1716,"weight":7}`))
			}
			fmt.Fprintf(w, `{"response":{"store_items":[%s]}}`, strings.Join(parts, ","))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	defer srv.Close()
	withSteamHook(t, srv)

	env, errOut, err := runDemosCmd(t, "--limit", "20")
	if err != nil {
		t.Fatalf("tag-list failure must not fail the command: %v", err)
	}
	results, _ := env["results"].([]any)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1 (rows survive)", len(results))
	}
	row := results[0].(map[string]any)
	if row["parent_name"] == nil {
		t.Errorf("parent_name must still be filled when only the tag dictionary failed")
	}
	if _, ok := row["parent_tags"]; ok {
		t.Errorf("parent_tags must be absent when tag names are unavailable, got %v", row["parent_tags"])
	}
	meta, _ := env["meta"].(map[string]any)
	missing, _ := meta["sources_missing"].([]any)
	found := false
	for _, m := range missing {
		if m == "steam_tags" {
			found = true
		}
	}
	if !found {
		t.Errorf("meta.sources_missing = %v, want to contain steam_tags", meta["sources_missing"])
	}
	if !strings.Contains(errOut, "steam_tags") {
		t.Errorf("stderr = %q, want a one-line warning naming steam_tags", errOut)
	}
}
