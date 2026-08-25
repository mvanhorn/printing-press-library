package pg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	xhtml "golang.org/x/net/html"
)

func TestParseIndexOnlyTakesEssayRows(t *testing.T) {
	doc, err := xhtml.Parse(strings.NewReader(`<!doctype html>
<html><body>
  <a href="index.html">Home</a>
  <font><a href="greatwork.html">How to Do Great Work</a></font>
  <table><tr valign="top"><td width="435">
    <img src="https://s.turbifycdn.com/aah/paulgraham/the-reddits-2.gif">
    <font size="2"><a href="earn.html">How to Earn a Billion Dollars</a></font>
  </td></tr></table>
  <table><tr valign="top"><td width="435">
    <img src="https://s.turbifycdn.com/aah/paulgraham/the-reddits-2.gif">
    <font size="2"><a href="winc.html">How to Convert Between Wealth and Income Tax</a></font>
  </td></tr></table>
  <table><tr valign="top"><td width="435">
    <img src="https://s.turbifycdn.com/aah/paulgraham/the-reddits-2.gif">
    <font size="2"><a href="earn.html">How to Earn a Billion Dollars</a></font>
  </td></tr></table>
  <a href="rss.html">RSS</a>
</body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	essays := parseIndex(doc)
	if len(essays) != 2 {
		t.Fatalf("len(essays) = %d, want 2: %#v", len(essays), essays)
	}
	if essays[0].Slug != "earn" || essays[0].Order != 1 {
		t.Fatalf("first essay = %#v, want earn order 1", essays[0])
	}
	if essays[1].Title != "How to Convert Between Wealth and Income Tax" {
		t.Fatalf("second title = %q", essays[1].Title)
	}
}

func TestExtractMainTextChoosesLongestContentCell(t *testing.T) {
	doc, err := xhtml.Parse(strings.NewReader(`<!doctype html>
<html><body>
  <table><tr><td width="435">Short nav text that should not win.</td></tr></table>
  <table><tr><td width="435">
    <font>Essay Title</font><br><br>
    This is the essay body. It has enough words to be considered article content
    and should be selected over the shorter navigation text cell that appears earlier.
    The extractor collapses whitespace and keeps readable prose.
  </td></tr></table>
</body></html>`))
	if err != nil {
		t.Fatal(err)
	}

	got := extractMainText(doc)
	if !strings.Contains(got, "This is the essay body") {
		t.Fatalf("extractMainText() = %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("extractMainText() did not collapse whitespace: %q", got)
	}
}

func TestFindMatchesSlugURLTitleAndSubstring(t *testing.T) {
	essays := []Essay{
		{Title: "How to Do Great Work", Slug: "greatwork", URL: "https://www.paulgraham.com/greatwork.html"},
		{Title: "Founder Mode", Slug: "foundermode", URL: "https://www.paulgraham.com/foundermode.html"},
	}
	cases := []string{"greatwork", "greatwork.html", "Founder Mode", "https://www.paulgraham.com/foundermode.html", "founder"}
	for _, tc := range cases {
		if _, ok := Find(essays, tc); !ok {
			t.Fatalf("Find(%q) = false, want true", tc)
		}
	}
}

func TestReadReturnsErrorWhenEssayBodyIsMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!doctype html><html><head><title>Moved</title></head><body><p>No essay content cell.</p></body></html>`))
	}))
	defer srv.Close()

	_, err := Read(context.Background(), Essay{Title: "Moved", Slug: "moved", URL: srv.URL}, time.Second)
	if err == nil {
		t.Fatal("Read() err = nil, want missing essay body error")
	}
	if !strings.Contains(err.Error(), "page layout may have changed") {
		t.Fatalf("Read() err = %v, want page-layout hint", err)
	}
}

func TestSearchFullTextKeepsPartialMatchesWhenOneEssayFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/match.html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!doctype html><html><body><table><tr><td width="435">
				Startup ideas are often hiding in ordinary work. This paragraph is long enough
				to look like essay content and should be returned as a full text match.
				It has additional ordinary prose so the extraction heuristic accepts the cell
				as article content rather than rejecting a tiny fixture body.
			</td></tr></table></body></html>`))
		default:
			http.Error(w, "temporarily unavailable", http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()

	essays := []Essay{
		{Title: "Broken Essay", Slug: "broken", URL: srv.URL + "/broken.html", Order: 1},
		{Title: "Working Essay", Slug: "match", URL: srv.URL + "/match.html", Order: 2},
	}

	matches, err := SearchFullText(context.Background(), essays, "startup", time.Second, 10)
	if err != nil {
		t.Fatalf("SearchFullText() err = %v, want nil", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches) = %d, want 1: %#v", len(matches), matches)
	}
	if matches[0].Slug != "match" {
		t.Fatalf("match slug = %q, want match", matches[0].Slug)
	}
}

func TestExcerptDoesNotSplitUTF8Runes(t *testing.T) {
	got := excerpt("alpha beta café déjà vu résumé", 16)
	if !utf8.ValidString(got) {
		t.Fatalf("excerpt() produced invalid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Fatalf("excerpt() = %q, want ellipsis", got)
	}
}
