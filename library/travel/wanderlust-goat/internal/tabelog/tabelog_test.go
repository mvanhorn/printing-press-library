package tabelog

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sourcetypes"
)

func TestSlugLocaleStub(t *testing.T) {
	c := NewClient()
	if c.Slug() != "tabelog" {
		t.Errorf("Slug() = %q, want tabelog", c.Slug())
	}
	if c.Locale() != "ja" {
		t.Errorf("Locale() = %q, want ja", c.Locale())
	}
	if c.IsStub() {
		t.Error("real tabelog client must not be a stub")
	}
	var _ sourcetypes.Client = c // interface compliance
}

func TestExtractTabelogHits_ListingMarkup(t *testing.T) {
	html := `
		<div class="list-rst">
			<a href="/tokyo/A1303/A130301/13123456/" class="list-rst__rst-name-target cpy-rst-name">鮨善</a>
		</div>
		<div class="list-rst">
			<a href="/tokyo/A1303/A130301/13234567/" class="list-rst__rst-name-target cpy-rst-name">珈琲館</a>
		</div>
	`
	hits := extractTabelogHits(html, "https://tabelog.com", 10)
	if len(hits) != 2 {
		t.Fatalf("got %d hits, want 2", len(hits))
	}
	if hits[0].Title != "鮨善" {
		t.Errorf("hit[0].Title = %q, want 鮨善", hits[0].Title)
	}
	if !strings.HasPrefix(hits[0].URL, "https://tabelog.com/tokyo/") {
		t.Errorf("hit[0].URL = %q, want https://tabelog.com/tokyo/...", hits[0].URL)
	}
	if hits[0].Locale != "ja" {
		t.Errorf("hit[0].Locale = %q, want ja", hits[0].Locale)
	}
}

func TestExtractTabelogHits_DedupesURLs(t *testing.T) {
	html := `
		<a href="/tokyo/A1/A1/13456/" class="list-rst__rst-name-target">A</a>
		<a href="/tokyo/A1/A1/13456/" class="list-rst__rst-name-target">A duplicate</a>
	`
	hits := extractTabelogHits(html, "https://tabelog.com", 10)
	if len(hits) != 1 {
		t.Errorf("expected dedupe to 1, got %d", len(hits))
	}
}

func TestLookupByName_HTTPMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/rstLst/") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("missing User-Agent")
		}
		_, _ = w.Write([]byte(`<a href="/tokyo/A1303/A130301/13999999/" class="list-rst__rst-name-target">Test 鮨</a>`))
	}))
	defer srv.Close()

	c := NewClientWithBase(srv.URL, time.Millisecond) // no throttle delay in tests
	hits, err := c.LookupByName(context.Background(), "Test 鮨", "Tokyo", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Title != "Test 鮨" {
		t.Errorf("got %+v, want one hit titled Test 鮨", hits)
	}
}

func TestLookupByName_RejectsEmpty(t *testing.T) {
	c := NewClient()
	_, err := c.LookupByName(context.Background(), "", "", 5)
	if err == nil {
		t.Error("empty name should error")
	}
}

func TestCheckClosed_DetectsKanji(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<title>店名 - 閉店しました</title>`))
	}))
	defer srv.Close()

	c := NewClientWithBase(srv.URL, time.Millisecond)
	v := c.CheckClosed(context.Background(), sourcetypes.Hit{URL: srv.URL + "/restaurant/123/"})
	if !v.Closed {
		t.Errorf("expected Closed verdict, got %+v", v)
	}
}

func TestCheckClosed_EmptyURL(t *testing.T) {
	c := NewClient()
	v := c.CheckClosed(context.Background(), sourcetypes.Hit{})
	if v.Closed {
		t.Error("empty URL should be Open")
	}
}
