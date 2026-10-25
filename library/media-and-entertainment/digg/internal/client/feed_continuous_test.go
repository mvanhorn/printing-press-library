package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/digg/internal/config"
)

// feedPageJSON renders one /api/feed/continuous page with n stories whose
// ids start at offset. cursor is the nextCursor value ("" renders null).
func feedPageJSON(offset, n int, cursor string) string {
	posts := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := offset + i
		posts = append(posts, fmt.Sprintf(`{
			"clusterId": "c-%d", "clusterUrlId": "u%d", "type": "cluster",
			"entryStatus": "ranked", "rank": %d, "diggScore": 100,
			"createdAt": "2026-10-10T08:00:00+00:00",
			"lastSourcePostAt": "2026-10-11T08:00:00+00:00",
			"postCount": 12, "uniqueAuthors": 7, "likes": 3, "views": 40,
			"summary": {"title": "Story %d", "description": "About story %d"},
			"authors": [{"username": "alice", "displayName": "Alice"}]
		}`, id, id, id+1, id, id))
	}
	next := "null"
	if cursor != "" {
		next = fmt.Sprintf("%q", cursor)
	}
	return fmt.Sprintf(`{"nextCursor": %s, "nowMs": 1792648074472, "posts": [%s],
		"publishedClusterIds": [], "publishedOnly": true, "range": "30d",
		"unavailablePosts": [], "view": "top"}`, next, strings.Join(posts, ","))
}

func newTestClient() *Client {
	return New(&config.Config{BaseURL: "http://unused"}, 5*time.Second, 0)
}

type feedServer struct {
	mu      sync.Mutex
	queries []string
	pages   map[string]string // cursor ("" = first) -> body
	status  map[string]int    // cursor -> non-200 status
}

func (f *feedServer) handler(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.queries = append(f.queries, r.URL.RawQuery)
	f.mu.Unlock()
	cur := r.URL.Query().Get("cursor")
	if code, ok := f.status[cur]; ok {
		w.WriteHeader(code)
		_, _ = w.Write([]byte(`{"error":"Invalid query parameters"}`))
		return
	}
	body, ok := f.pages[cur]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(body))
}

func TestWalkFeedFollowsCursorUntilNull(t *testing.T) {
	fs := &feedServer{pages: map[string]string{
		"":      feedPageJSON(0, 20, "cur-2"),
		"cur-2": feedPageJSON(20, 20, ""),
	}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	var got []FeedStory
	res, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 10,
		func(p *FeedPage) error { got = append(got, p.Posts...); return nil })
	if err != nil {
		t.Fatalf("WalkFeedFrom: %v", err)
	}
	if len(got) != 40 {
		t.Fatalf("got %d stories, want 40", len(got))
	}
	if res.Pages != 2 || !res.Complete || res.Capped {
		t.Fatalf("result = %+v, want 2 pages, complete, not capped", res)
	}
	if !strings.Contains(fs.queries[1], "cursor=cur-2") {
		t.Fatalf("second request %q did not send the page-1 cursor", fs.queries[1])
	}
	for _, want := range []string{"topic=ai", "view=top", "range=30d", "published=1"} {
		if !strings.Contains(fs.queries[0], want) {
			t.Fatalf("first request %q missing %s", fs.queries[0], want)
		}
	}
	if got[0].Summary.Title != "Story 0" || got[0].PostCount != 12 || got[0].UniqueAuthors != 7 || got[0].Rank != 1 {
		t.Fatalf("first story decoded wrong: %+v", got[0])
	}
}

func TestWalkFeedStopsAtMaxPages(t *testing.T) {
	fs := &feedServer{pages: map[string]string{
		"":   feedPageJSON(0, 20, "c2"),
		"c2": feedPageJSON(20, 20, "c3"),
		"c3": feedPageJSON(40, 20, "c4"),
	}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	res, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 2,
		func(p *FeedPage) error { return nil })
	if err != nil {
		t.Fatalf("WalkFeedFrom: %v", err)
	}
	if res.Pages != 2 || !res.Capped || res.Complete {
		t.Fatalf("result = %+v, want 2 pages, capped, not complete", res)
	}
}

func TestWalkFeedSurfacesHTTPErrorBody(t *testing.T) {
	fs := &feedServer{pages: map[string]string{}, status: map[string]int{"": 400}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	_, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "Invalid query parameters") {
		t.Fatalf("err = %v, want HTTP 400 error carrying the body", err)
	}
}

func TestWalkFeedEmptyFirstPageIsError(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"": feedPageJSON(0, 0, "")}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	_, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("err = %v, want a layout-change error for an empty first page", err)
	}
}

func TestWalkFeedMissingPostsKeyIsError(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"": `{"nextCursor": null, "nowMs": 1}`}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	_, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("err = %v, want a layout-change error when posts is missing", err)
	}
}

func TestWalkFeedEmptyLaterPageEndsWalk(t *testing.T) {
	fs := &feedServer{pages: map[string]string{
		"":   feedPageJSON(0, 20, "c2"),
		"c2": feedPageJSON(0, 0, "c3"),
	}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	res, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { return nil })
	if err != nil {
		t.Fatalf("WalkFeedFrom: %v", err)
	}
	if res.Pages != 2 || !res.Complete {
		t.Fatalf("result = %+v, want walk to end complete after the empty page", res)
	}
}

func TestWalkFeedLaterPageErrorKeepsEarlierPages(t *testing.T) {
	fs := &feedServer{
		pages:  map[string]string{"": feedPageJSON(0, 20, "c2")},
		status: map[string]int{"c2": 500},
	}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	seen := 0
	res, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { seen += len(p.Posts); return nil })
	if err == nil {
		t.Fatalf("want an error from the failed page 2")
	}
	if seen != 20 || res.Pages != 1 || res.Complete {
		t.Fatalf("seen=%d result=%+v, want page 1 delivered and walk incomplete", seen, res)
	}
}

func TestFeedStoryNullSummaryDecodes(t *testing.T) {
	var s FeedStory
	if err := json.Unmarshal([]byte(`{"clusterId":"c","clusterUrlId":"u","summary":null}`), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if s.Summary.Title != "" {
		t.Fatalf("title = %q, want empty", s.Summary.Title)
	}
}

func TestWalkFeedDoesNotCountPageWhoseCallbackFails(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"": feedPageJSON(0, 20, "c2")}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	res, err := newTestClient().WalkFeedFrom(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, 5,
		func(p *FeedPage) error { return errors.New("store write failed") })
	if err == nil {
		t.Fatal("want the callback error")
	}
	if res.Pages != 0 || res.Stories != 0 {
		t.Fatalf("result = %+v, want no pages counted for a page that was not stored", res)
	}
}
