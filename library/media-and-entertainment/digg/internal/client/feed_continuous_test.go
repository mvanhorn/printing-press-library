package client

import (
	"context"
	"encoding/json"
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

func TestFetchFeedPageDecodesStoriesAndSendsQuery(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"": feedPageJSON(0, 20, "cur-2")}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	page, err := newTestClient().FetchFeedPage(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "trending", Range: "7d"}, "")
	if err != nil {
		t.Fatalf("FetchFeedPage: %v", err)
	}
	if len(page.Posts) != 20 {
		t.Fatalf("got %d stories, want 20", len(page.Posts))
	}
	if page.NextCursor == nil || *page.NextCursor != "cur-2" {
		t.Fatalf("NextCursor = %v, want cur-2", page.NextCursor)
	}
	first := page.Posts[0]
	if first.ClusterID != "c-0" || first.ClusterURLID != "u0" || first.Summary.Title != "Story 0" ||
		first.Summary.Description != "About story 0" || first.PostCount != 12 || first.UniqueAuthors != 7 ||
		first.Rank != 1 || first.DiggScore != 100 || first.CreatedAt != "2026-10-10T08:00:00+00:00" {
		t.Fatalf("first story decoded wrong: %+v", first)
	}
	if len(fs.queries) != 1 {
		t.Fatalf("made %d requests, want exactly one page", len(fs.queries))
	}
	for _, want := range []string{"topic=ai", "view=trending", "range=7d", "published=1"} {
		if !strings.Contains(fs.queries[0], want) {
			t.Fatalf("request %q missing %s", fs.queries[0], want)
		}
	}
	if strings.Contains(fs.queries[0], "cursor") {
		t.Fatalf("first-page request %q should carry no cursor", fs.queries[0])
	}
}

func TestFetchFeedPageSendsCursorAndReportsLastPage(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"cur-2": feedPageJSON(20, 5, "")}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	page, err := newTestClient().FetchFeedPage(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "24h"}, "cur-2")
	if err != nil {
		t.Fatalf("FetchFeedPage: %v", err)
	}
	if len(page.Posts) != 5 || page.NextCursor != nil {
		t.Fatalf("got %d stories and cursor %v, want 5 stories and no cursor", len(page.Posts), page.NextCursor)
	}
	if !strings.Contains(fs.queries[0], "cursor=cur-2") {
		t.Fatalf("request %q did not send the cursor", fs.queries[0])
	}
}

func TestFetchFeedPageSurfacesHTTPErrorBody(t *testing.T) {
	fs := &feedServer{pages: map[string]string{}, status: map[string]int{"": 400}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	_, err := newTestClient().FetchFeedPage(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, "")
	if err == nil || !strings.Contains(err.Error(), "Invalid query parameters") {
		t.Fatalf("err = %v, want HTTP 400 error carrying the body", err)
	}
}

func TestFetchFeedPageMissingPostsKeyIsError(t *testing.T) {
	fs := &feedServer{pages: map[string]string{"": `{"nextCursor": null, "nowMs": 1}`}}
	srv := httptest.NewServer(http.HandlerFunc(fs.handler))
	defer srv.Close()

	_, err := newTestClient().FetchFeedPage(context.Background(), srv.URL,
		FeedParams{Topic: "ai", View: "top", Range: "30d"}, "")
	if err == nil || !strings.Contains(err.Error(), "layout") {
		t.Fatalf("err = %v, want a layout-change error when posts is missing", err)
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
