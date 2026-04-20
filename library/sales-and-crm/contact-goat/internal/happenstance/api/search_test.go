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
