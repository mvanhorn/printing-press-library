package logodownload

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSearchHTMLReturnsImageURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" || r.URL.Query().Get("s") != "nike" {
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
			<html><body>
				<article class="grid-post">
					<div class="post-thumbnail">
						<a href="/nike-logo/" title="Nike Logo">
							<img src="/wp-content/uploads/nike.png">
						</a>
					</div>
					<h2 class="entry-title"><a href="/nike-logo/">Nike Logo</a></h2>
				</article>
			</body></html>
		`))
	}))
	defer server.Close()

	restoreBaseURL(t, server.URL)

	results, err := Search(context.Background(), server.Client(), "nike", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Title != "Nike Logo" {
		t.Fatalf("unexpected title: %q", results[0].Title)
	}
	if results[0].URL != server.URL+"/nike-logo/" {
		t.Fatalf("unexpected URL: %q", results[0].URL)
	}
	if results[0].ImageURL != server.URL+"/wp-content/uploads/nike.png" {
		t.Fatalf("unexpected image URL: %q", results[0].ImageURL)
	}
}

func TestSearchWordPressFallbackFetchesFeaturedImage(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html><body><p>No article results</p></body></html>`))
		case r.URL.Path == "/wp-json/wp/v2/search":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"id":338,"title":"Nike Logo","url":"` + server.URL + `/nike-logo/"}]`))
		case r.URL.Path == "/wp-json/wp/v2/posts/338":
			if r.URL.Query().Get("_embed") != "wp:featuredmedia" {
				t.Fatalf("expected embedded media query, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{
				"_embedded": {
					"wp:featuredmedia": [{
						"source_url": "` + server.URL + `/uploads/original.png",
						"media_details": {
							"sizes": {
								"large": {"source_url": "` + server.URL + `/uploads/large.png"}
							}
						}
					}]
				}
			}`))
		default:
			t.Fatalf("unexpected request: %s", r.URL.String())
		}
	}))
	defer server.Close()

	restoreBaseURL(t, server.URL)

	results, err := Search(context.Background(), server.Client(), "nike", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ImageURL != server.URL+"/uploads/large.png" {
		t.Fatalf("unexpected image URL: %q", results[0].ImageURL)
	}
}

func TestSearchReturnsEmptySliceForEmptyQuery(t *testing.T) {
	results, err := Search(context.Background(), http.DefaultClient, "   ", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if results == nil {
		t.Fatal("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func restoreBaseURL(t *testing.T, value string) {
	t.Helper()
	previous := baseURL
	baseURL = value
	t.Cleanup(func() {
		baseURL = previous
	})
}
