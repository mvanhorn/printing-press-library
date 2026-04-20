// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// readBody is a small test helper: drain and JSON-decode the request body
// into target. Fails the test on error so call sites stay tidy.
func readBody(t *testing.T, r *http.Request, target any) {
	t.Helper()
	defer r.Body.Close()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read request body: %v", err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatalf("decode request body: %v\nbody: %s", err, string(raw))
	}
}

func TestSearch_HappyPath_PostsExpectedBodyShape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/search" {
			t.Errorf("path = %q, want /search", r.URL.Path)
		}
		var body map[string]any
		readBody(t, r, &body)
		if body["text"] != "VPs at NBA" {
			t.Errorf("body.text = %v, want %q", body["text"], "VPs at NBA")
		}
		// Default options: connection-scope flags omit (omitempty), group_ids omit.
		if _, ok := body["group_ids"]; ok {
			t.Errorf("body.group_ids should be absent under default opts; got %v", body["group_ids"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_abc123","url":"https://happenstance.ai/search/srch_abc123"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.Search(context.Background(), "VPs at NBA", nil)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if env.Id != "srch_abc123" {
		t.Errorf("env.Id = %q, want srch_abc123", env.Id)
	}
}

func TestSearch_WithOptions_SerializesEveryField(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body createSearchRequest
		readBody(t, r, &body)
		if body.Text != "engineering leaders" {
			t.Errorf("text = %q", body.Text)
		}
		if len(body.GroupIDs) != 2 || body.GroupIDs[0] != "grp_1" || body.GroupIDs[1] != "grp_2" {
			t.Errorf("group_ids = %v", body.GroupIDs)
		}
		if !body.IncludeFriendsConnections || !body.IncludeMyConnections {
			t.Errorf("include flags = %+v, both want true", body)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_xyz"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Search(context.Background(), "engineering leaders", &SearchOptions{
		GroupIDs:                  []string{"grp_1", "grp_2"},
		IncludeFriendsConnections: true,
		IncludeMyConnections:      true,
	})
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
}

func TestSearch_EmptyText_FailsBeforeNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Fatal("server should not be hit for empty text")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.Search(context.Background(), "  ", nil)
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestPollSearch_RunningTwiceThenCompleted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if !strings.HasPrefix(r.URL.Path, "/search/") {
			t.Errorf("path = %q, want /search/{id}", r.URL.Path)
		}
		n := atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		switch n {
		case 1, 2:
			io.WriteString(w, `{"id":"srch_1","status":"RUNNING"}`)
		default:
			io.WriteString(w, `{"id":"srch_1","status":"COMPLETED","results":[
				{"name":"Alice","current_title":"VP","current_company":"NBA","weighted_traits_score":0.92},
				{"name":"Bob","current_title":"SVP","current_company":"NBA","weighted_traits_score":0.81}
			]}`)
		}
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	// Tight interval so the test is fast; real callers default to 1s.
	env, err := c.PollSearch(context.Background(), "srch_1", &PollSearchOptions{
		Timeout:  5 * time.Second,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollSearch() error = %v", err)
	}
	if env.Status != StatusCompleted {
		t.Errorf("status = %q, want COMPLETED", env.Status)
	}
	if len(env.Results) != 2 {
		t.Errorf("len(Results) = %d, want 2", len(env.Results))
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3", got)
	}
}

func TestPollSearch_TimeoutHitsRunningCeilingNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_stuck","status":"RUNNING"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	// 50ms ceiling stand-in for the production 180s default. The plan calls
	// out using a tiny override here so the test runs in milliseconds.
	env, err := c.PollSearch(context.Background(), "srch_stuck", &PollSearchOptions{
		Timeout:  50 * time.Millisecond,
		Interval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("PollSearch() should not error on timeout; got %v", err)
	}
	// The Completed:false equivalent is "Status is still RUNNING after the
	// timeout fired". Document it explicitly so a future status-rename
	// (e.g. PENDING) trips this assertion.
	if env.Status != StatusRunning {
		t.Errorf("status = %q, want still RUNNING after timeout", env.Status)
	}
}

func TestPollSearch_ContextCancelledMidLoop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_cancel","status":"RUNNING"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after a few millis so the first GetSearch likely succeeds and the
	// loop is sitting in select waiting on the next tick.
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	_, err := c.PollSearch(ctx, "srch_cancel", &PollSearchOptions{
		Timeout:  5 * time.Second,
		Interval: 100 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("expected ctx.Err() on cancellation, got nil")
	}
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestPollSearch_ForwardsPageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page_id"); got != "page_42" {
			t.Errorf("page_id = %q, want page_42", got)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_p","status":"COMPLETED"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.PollSearch(context.Background(), "srch_p", &PollSearchOptions{
		Timeout:  100 * time.Millisecond,
		Interval: 5 * time.Millisecond,
		PageID:   "page_42",
	})
	if err != nil {
		t.Fatalf("PollSearch() error = %v", err)
	}
}

func TestGetSearch_PlainAndPaginated(t *testing.T) {
	cases := []struct {
		name     string
		pageID   string
		wantPath string
		wantQS   string
	}{
		{"no page id", "", "/search/srch_abc", ""},
		{"with page id", "pg_1", "/search/srch_abc", "page_id=pg_1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.wantPath {
					t.Errorf("path = %q, want %q", r.URL.Path, tc.wantPath)
				}
				if r.URL.RawQuery != tc.wantQS {
					t.Errorf("query = %q, want %q", r.URL.RawQuery, tc.wantQS)
				}
				w.WriteHeader(http.StatusOK)
				io.WriteString(w, `{"id":"srch_abc","status":"COMPLETED"}`)
			}))
			defer srv.Close()
			c := newTestClient(t, srv)
			if _, err := c.GetSearch(context.Background(), "srch_abc", tc.pageID); err != nil {
				t.Fatalf("GetSearch() error = %v", err)
			}
		})
	}
}

func TestFindMore_Happy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/search/srch_parent/find-more" {
			t.Errorf("path = %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"page_id":"pg_next","parent_search_id":"srch_parent"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	env, err := c.FindMore(context.Background(), "srch_parent")
	if err != nil {
		t.Fatalf("FindMore() error = %v", err)
	}
	if env.PageId != "pg_next" {
		t.Errorf("page_id = %q, want pg_next", env.PageId)
	}
	if env.ParentSearchId != "srch_parent" {
		t.Errorf("parent_search_id = %q, want srch_parent", env.ParentSearchId)
	}
}

func TestFindMore_NonParentSearch_Returns422WithParentOnlyMention(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		io.WriteString(w, `{"error":"this search was spawned from a previous find-more page; find-more is callable on a parent search only"}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv)
	_, err := c.FindMore(context.Background(), "srch_child")
	if err == nil {
		t.Fatal("expected error on 422")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "parent search only") {
		t.Errorf("error %q does not mention 'parent search only'", err.Error())
	}
}

func TestFindMore_EmptyID(t *testing.T) {
	c := NewClient(testKey)
	_, err := c.FindMore(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty parent id")
	}
}

func TestGroupRoundTripIntoSearchTextAsAtMention(t *testing.T) {
	// Integration: Group(ctx, id) returns members; their names format as
	// @-mentions and round-trip into a Search request body's text field
	// without quoting issues. The plan calls out this scenario explicitly.

	memberCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/groups/grp_nba", func(w http.ResponseWriter, _ *http.Request) {
		memberCalls++
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"grp_nba","name":"NBA Front Office","member_count":2,"members":[{"name":"Alice O'Hara"},{"name":"Bob Smith"}]}`)
	})

	var capturedText string
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		readBody(t, r, &body)
		capturedText, _ = body["text"].(string)
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, `{"id":"srch_after_group"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv)
	g, err := c.Group(context.Background(), "grp_nba")
	if err != nil {
		t.Fatalf("Group() error = %v", err)
	}
	if memberCalls != 1 {
		t.Errorf("members fetched %d times, want 1", memberCalls)
	}
	if len(g.Members) != 2 {
		t.Fatalf("member_count = %d, want 2", len(g.Members))
	}

	// Build a search query that mentions every group member.
	mentions := make([]string, 0, len(g.Members))
	for _, m := range g.Members {
		mentions = append(mentions, FormatGroupMention(m.Name))
	}
	queryText := fmt.Sprintf("intros to %s", strings.Join(mentions, " and "))
	if _, err := c.Search(context.Background(), queryText, nil); err != nil {
		t.Fatalf("Search() error = %v", err)
	}

	// The captured request body must contain the formatted mentions verbatim.
	wantA := `@"Alice O'Hara"`
	wantB := `@"Bob Smith"`
	if !strings.Contains(capturedText, wantA) {
		t.Errorf("captured text %q missing %q", capturedText, wantA)
	}
	if !strings.Contains(capturedText, wantB) {
		t.Errorf("captured text %q missing %q", capturedText, wantB)
	}
}

// TestSearchEnvelope_UnmarshalsBridgesAndAffinity verifies that the full
// bearer-API response shape — top-level mutuals plus per-result mutuals,
// socials, and traits — round-trips through encoding/json without losing
// bridge detail. Fixture mirrors the live 2026-04-19 Tesla response.
func TestSearchEnvelope_UnmarshalsBridgesAndAffinity(t *testing.T) {
	raw := `{
	  "id":"srch_tesla",
	  "status":"COMPLETED",
	  "text":"people at Tesla",
	  "mutuals":[
	    {"index":0,"id":"friend-1","name":"Jeff Clavier","happenstance_url":"https://happenstance.ai/u/friend-1"},
	    {"index":1,"id":"friend-2","name":"Garry Tan"},
	    {"index":2,"id":"friend-3","name":"Alex Teichman"},
	    {"index":3,"id":"user-self","name":"Matt Van Horn"}
	  ],
	  "results":[
	    {
	      "id":"p-ira",
	      "name":"Ira Ehrenpreis",
	      "current_title":"Board of Directors",
	      "current_company":"Tesla Motors",
	      "summary":"Board of Directors at Tesla Motors since 2007.",
	      "weighted_traits_score":1.0,
	      "mutuals":[{"index":0,"affinity_score":104.38174315088183}],
	      "socials":{"linkedin_url":"https://www.linkedin.com/in/iraehrenpreis"},
	      "traits":[{"index":0,"score":1.0,"evidence":"Board seat since 2007."}]
	    },
	    {
	      "name":"Nathan Ng",
	      "current_title":"Manager, Senior Staff Mechanical Engineer",
	      "current_company":"Tesla",
	      "weighted_traits_score":1.0,
	      "mutuals":[{"index":3,"affinity_score":0}]
	    }
	  ],
	  "has_more":true
	}`
	var env SearchEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(env.Mutuals) != 4 {
		t.Fatalf("envelope mutuals = %d, want 4", len(env.Mutuals))
	}
	if env.Mutuals[0].Name != "Jeff Clavier" || env.Mutuals[0].Id != "friend-1" {
		t.Errorf("mutual[0] = %+v", env.Mutuals[0])
	}
	if env.Mutuals[3].Name != "Matt Van Horn" {
		t.Errorf("self-entry = %+v, want name 'Matt Van Horn'", env.Mutuals[3])
	}
	if len(env.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(env.Results))
	}

	ira := env.Results[0]
	if ira.Id != "p-ira" || ira.Summary == "" {
		t.Errorf("ira scalar fields = %+v", ira)
	}
	if len(ira.Mutuals) != 1 || ira.Mutuals[0].Index != 0 || ira.Mutuals[0].AffinityScore != 104.38174315088183 {
		t.Errorf("ira bridges = %+v", ira.Mutuals)
	}
	if ira.Socials == nil || ira.Socials.LinkedInURL != "https://www.linkedin.com/in/iraehrenpreis" {
		t.Errorf("ira socials = %+v", ira.Socials)
	}
	if len(ira.Traits) != 1 || ira.Traits[0].Evidence == "" {
		t.Errorf("ira traits = %+v", ira.Traits)
	}

	nathan := env.Results[1]
	if len(nathan.Mutuals) != 1 || nathan.Mutuals[0].Index != 3 || nathan.Mutuals[0].AffinityScore != 0 {
		t.Errorf("nathan bridges = %+v", nathan.Mutuals)
	}
	if nathan.Socials != nil {
		t.Errorf("nathan socials should be nil (omitted in fixture); got %+v", nathan.Socials)
	}
}

// TestSearchEnvelope_EmptyMutuals covers cookie-surface responses and
// edge-case bearer responses where mutuals slices are explicitly empty
// or entirely absent. Both should produce a valid, nil/empty-slice
// envelope with no panic.
func TestSearchEnvelope_EmptyMutuals(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"empty arrays", `{"id":"s","status":"COMPLETED","mutuals":[],"results":[{"name":"X","mutuals":[]}]}`},
		{"absent fields", `{"id":"s","status":"COMPLETED","results":[{"name":"X"}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var env SearchEnvelope
			if err := json.Unmarshal([]byte(tc.raw), &env); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if len(env.Mutuals) != 0 {
				t.Errorf("envelope mutuals = %d, want 0", len(env.Mutuals))
			}
			if len(env.Results) != 1 {
				t.Fatalf("results = %d, want 1", len(env.Results))
			}
			if len(env.Results[0].Mutuals) != 0 {
				t.Errorf("result mutuals = %d, want 0", len(env.Results[0].Mutuals))
			}
		})
	}
}

// TestSearchEnvelope_ExtremeFloatAffinity guards against precision loss on
// the AffinityScore field. Observed values span seven orders of magnitude
// (4.77e-39 on zero-weight synthetic bridges, 245 on the strongest observed
// real bridge). Both must round-trip.
func TestSearchEnvelope_ExtremeFloatAffinity(t *testing.T) {
	raw := `{"id":"s","status":"COMPLETED","results":[
	  {"name":"Tiny","mutuals":[{"index":1,"affinity_score":4.772363296454658e-39}]},
	  {"name":"Huge","mutuals":[{"index":2,"affinity_score":245.23555284520657}]}
	]}`
	var env SearchEnvelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := env.Results[0].Mutuals[0].AffinityScore; got != 4.772363296454658e-39 {
		t.Errorf("tiny affinity = %g, want 4.77e-39", got)
	}
	if got := env.Results[1].Mutuals[0].AffinityScore; got != 245.23555284520657 {
		t.Errorf("huge affinity = %g, want 245.235...", got)
	}
}

func TestFormatGroupMention(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"  ", ""},
		{"Alice", "@Alice"},
		{"Alice Smith", `@"Alice Smith"`},
		{`Bob "Bo" Marley`, `@"Bob \"Bo\" Marley"`},
		{"   trim_me   ", "@trim_me"},
	}
	for _, tc := range cases {
		got := FormatGroupMention(tc.in)
		if got != tc.want {
			t.Errorf("FormatGroupMention(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
