package hotpepper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestSlugLocale(t *testing.T) {
	c := NewClient()
	if c.Slug() != "hotpepper" || c.Locale() != "ja" || c.IsStub() {
		t.Errorf("unexpected slug/locale/stub")
	}
	var _ sourcetypes.Client = c
}

func TestLookupAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/hotpepper/gourmet/v1/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"results":{"shop":[{"id":"J001","name":"テスト居酒屋","urls":{"pc":"https://hotpepper.jp/strJ001/"},"catch":"絶品もつ煮"}]}}`))
	}))
	defer srv.Close()

	c := NewClientWithBase("https://hotpepper.jp", srv.URL, "test-key", time.Millisecond)
	hits, err := c.LookupByName(context.Background(), "テスト", "Tokyo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "テスト居酒屋" {
		t.Errorf("got %+v", hits)
	}
}

func TestLookupHTML_Fallback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/strJ12345/">テストカフェ</a>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, "https://api.example.com", "" /* no key */, time.Millisecond)
	hits, err := c.LookupByName(context.Background(), "テスト", "", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("hits=%v err=%v", hits, err)
	}
	if hits[0].Title != "テストカフェ" {
		t.Errorf("title = %q", hits[0].Title)
	}
}

func TestRejectsEmpty(t *testing.T) {
	c := NewClient()
	if _, err := c.LookupByName(context.Background(), "", "", 5); err == nil {
		t.Error("expected error on empty name")
	}
}
