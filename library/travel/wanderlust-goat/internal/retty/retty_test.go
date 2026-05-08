package retty

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestSlug(t *testing.T) {
	c := NewClient()
	// Retty is now flagged as a stub at the LookupByName layer because the
	// JS-rendered search isn't reachable via stdlib HTTP. Slug + Locale stay
	// the same so the registry still wires it.
	if c.Slug() != "retty" || c.Locale() != "ja" || !c.IsStub() {
		t.Errorf("unexpected: slug=%q locale=%q stub=%v", c.Slug(), c.Locale(), c.IsStub())
	}
	var _ sourcetypes.Client = c
}

func TestExtractHits(t *testing.T) {
	html := `<a href="/restaurants/100000/">あおき</a><a href="/restaurants/200000/">かつや</a>`
	hits := extractRettyHits(html, "https://retty.me", 10)
	if len(hits) != 2 {
		t.Fatalf("got %d hits", len(hits))
	}
	if hits[0].Title != "あおき" {
		t.Errorf("title = %q", hits[0].Title)
	}
}

func TestLookupByName_StubReturnsNotImplemented(t *testing.T) {
	// Retty's user-facing search is JS-rendered; LookupByName is intentionally
	// stubbed so the dispatcher records it under StubsSkipped instead of as
	// a hard error. The dead-code HTML scrape path remains and is exercised
	// by TestLookupByNameLive_HTTP below.
	c := NewClient()
	hits, err := c.LookupByName(context.Background(), "テスト", "Tokyo", 5)
	if !errors.Is(err, sourcetypes.ErrNotImplemented) {
		t.Fatalf("expected ErrNotImplemented, got err=%v hits=%v", err, hits)
	}
}

func TestLookupByNameLive_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/restaurants/9999/">テスト店</a>`))
	}))
	defer srv.Close()
	c := NewClientWithBase(srv.URL, time.Millisecond)
	hits, err := c.lookupByNameLive(context.Background(), "テスト", "Tokyo", 5)
	if err != nil || len(hits) != 1 {
		t.Fatalf("err=%v hits=%v", err, hits)
	}
}
