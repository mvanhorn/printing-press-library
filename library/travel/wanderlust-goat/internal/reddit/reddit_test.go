package reddit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const fixtureSearchResponse = `{
	"kind":"Listing",
	"data":{
		"after":null,
		"children":[
			{"kind":"t3","data":{"id":"abc1","subreddit":"travel","title":"Best ramen in Tokyo","url":"https://reddit.com/r/travel/comments/abc1","permalink":"/r/travel/comments/abc1/best_ramen_in_tokyo/","score":150,"num_comments":42,"selftext":"Looking for tonkotsu recommendations near Shinjuku.","created_utc":1700000000.0}},
			{"kind":"t3","data":{"id":"abc2","subreddit":"travel","title":"Tokyo bullet train tips","url":"https://reddit.com/r/travel/comments/abc2","permalink":"/r/travel/comments/abc2/tokyo_bullet_train_tips/","score":5,"num_comments":2,"selftext":"Shinkansen JR pass questions.","created_utc":1700000100.0}},
			{"kind":"t3","data":{"id":"abc3","subreddit":"travel","title":"Kyoto temple guide","url":"https://reddit.com/r/travel/comments/abc3","permalink":"/r/travel/comments/abc3/kyoto_temple_guide/","score":300,"num_comments":120,"selftext":"Walking route through eastern Kyoto.","created_utc":1700000200.0}}
		]
	}
}`

func newTestClient(srvURL string) *Client {
	c := New(nil, "test-ua")
	c.BaseURL = srvURL
	c.RateLimitPerSecond = 0 // disable throttle for tests
	return c
}

func TestSearch_HappyPathFlattens(t *testing.T) {
	var seenPath, seenQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSearchResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	threads, err := c.Search(context.Background(), "travel", "tokyo", SearchOpts{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(threads) != 3 {
		t.Fatalf("expected 3 threads, got %d", len(threads))
	}
	if threads[0].ID != "abc1" || threads[0].Score != 150 || threads[0].NumComments != 42 {
		t.Errorf("first thread: %+v", threads[0])
	}
	if seenPath != "/r/travel/search.json" {
		t.Errorf("path: got %q", seenPath)
	}
	for _, want := range []string{"q=tokyo", "restrict_sr=1", "sort=relevance"} {
		if !strings.Contains(seenQuery, want) {
			t.Errorf("query missing %q: %q", want, seenQuery)
		}
	}
}

func TestSearch_AppliesMinScoreAndComments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSearchResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	threads, err := c.Search(context.Background(), "travel", "tokyo", SearchOpts{
		MinScore:    100,
		MinComments: 10,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// abc1 (150 score, 42 comments) and abc3 (300 score, 120 comments) qualify.
	// abc2 (5/2) is filtered out.
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d (%v)", len(threads), threads)
	}
	for _, th := range threads {
		if th.ID == "abc2" {
			t.Errorf("abc2 should have been filtered: %+v", th)
		}
	}
}

func TestSearch_KeywordFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureSearchResponse))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	threads, err := c.Search(context.Background(), "travel", "tokyo", SearchOpts{
		KeywordFilter: []string{"ramen", "shinkansen"},
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// Title-match: "Best ramen..." (abc1). Body-match: "Shinkansen JR pass..." (abc2).
	// Kyoto guide (abc3) has neither.
	if len(threads) != 2 {
		t.Fatalf("expected 2 threads, got %d (%+v)", len(threads), threads)
	}
	ids := map[string]bool{}
	for _, th := range threads {
		ids[th.ID] = true
	}
	if !ids["abc1"] || !ids["abc2"] {
		t.Errorf("expected abc1 + abc2, got %v", ids)
	}
}

func TestSearch_LimitAndDefault(t *testing.T) {
	var seenLimit string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenLimit = r.URL.Query().Get("limit")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"children":[]}}`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	if _, err := c.Search(context.Background(), "travel", "tokyo", SearchOpts{}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if seenLimit != "25" {
		t.Errorf("default limit: got %q, want 25", seenLimit)
	}

	if _, err := c.Search(context.Background(), "travel", "tokyo", SearchOpts{Limit: 5}); err != nil {
		t.Fatalf("Search: %v", err)
	}
	if seenLimit != "5" {
		t.Errorf("explicit limit: got %q, want 5", seenLimit)
	}
}

func TestSearch_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`blocked`))
	}))
	defer srv.Close()

	c := newTestClient(srv.URL)
	_, err := c.Search(context.Background(), "travel", "x", SearchOpts{})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403: %v", err)
	}
}
