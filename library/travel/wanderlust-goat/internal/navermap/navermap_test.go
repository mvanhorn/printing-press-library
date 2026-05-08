package navermap

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
	if c.Slug() != "navermap" || c.Locale() != "ko" || c.IsStub() {
		t.Errorf("unexpected")
	}
	var _ sourcetypes.Client = c
}

func TestExtract(t *testing.T) {
	html := `<a href="https://place.naver.com/restaurant/12345/home">맛집</a>`
	hits := extractNaverHits(html, 5)
	if len(hits) != 1 || hits[0].Title != "맛집" {
		t.Errorf("got %+v", hits)
	}
}

func TestLookupHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="https://place.naver.com/p/9999/home">테스트 식당</a>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, time.Millisecond)
	hits, err := c.LookupByName(context.Background(), "테스트", "Seoul", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("err=%v hits=%v", err, hits)
	}
}

func TestRejectEmpty(t *testing.T) {
	c := NewClient()
	if _, err := c.LookupByName(context.Background(), "", "", 5); err == nil {
		t.Error("expected error")
	}
}
