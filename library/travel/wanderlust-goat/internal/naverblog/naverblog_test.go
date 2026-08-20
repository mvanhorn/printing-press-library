package naverblog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestSlug(t *testing.T) {
	c := NewClient()
	if c.Slug() != "naverblog" || c.Locale() != "ko" || c.IsStub() {
		t.Errorf("unexpected")
	}
	var _ sourcetypes.Client = c
}

func TestExtract(t *testing.T) {
	html := `<a href="https://blog.naver.com/foodlover/12345">맛집 리뷰</a>`
	hits := extractBlogHits(html, 5)
	if len(hits) != 1 || hits[0].Title != "맛집 리뷰" {
		t.Errorf("got %+v", hits)
	}
}

func TestHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="https://blog.naver.com/foodie/9999">테스트 리뷰</a>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, time.Millisecond)
	hits, err := c.LookupByName(context.Background(), "테스트", "Seoul", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("err=%v hits=%v", err, hits)
	}
}
