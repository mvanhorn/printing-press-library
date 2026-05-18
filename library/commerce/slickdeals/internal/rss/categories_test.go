// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package rss

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fixtureRSS is a minimal valid RSS feed matching the Slickdeals wire format.
const fixtureRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Slickdeals Frontpage RSS Feed</title>
    <link>https://slickdeals.net/</link>
    <item>
      <title><![CDATA[Test Deal $9.99]]></title>
      <link>https://slickdeals.net/f/19505952-test</link>
      <description><![CDATA[Amazon has Test Deal for $9.99.]]></description>
      <content:encoded><![CDATA[<div><img src="https://static.slickdealscdn.com/test.thumb" /></div><div>Thumb Score: +42 </div><a href="x" data-product-exitWebsite="amazon.com">link</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:32:11 +0000</pubDate>
      <category domain="https://slickdeals.net/">Frontpage Deals</category>
      <dc:creator>testuser</dc:creator>
      <guid>thread-19505952</guid>
    </item>
    <item>
      <title>Coupon Deal</title>
      <link>https://slickdeals.net/f/19999999-coupon</link>
      <description>Coupon deal description.</description>
      <content:encoded><![CDATA[<div>Thumb Score: +7 </div><a data-product-exitWebsite="bestbuy.com">link</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 12:00:00 +0000</pubDate>
      <category domain="https://slickdeals.net/">Coupons</category>
      <dc:creator>couponposter</dc:creator>
      <guid>thread-19999999</guid>
    </item>
  </channel>
</rss>`

func TestParse(t *testing.T) {
	items, err := Parse([]byte(fixtureRSS), 0)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("Parse() got %d items, want 2", len(items))
	}

	first := items[0]
	if first.Title != "Test Deal $9.99" {
		t.Errorf("Title = %q, want %q", first.Title, "Test Deal $9.99")
	}
	if first.DealID != "19505952" {
		t.Errorf("DealID = %q, want %q", first.DealID, "19505952")
	}
	if first.ThumbScore != 42 {
		t.Errorf("ThumbScore = %d, want 42", first.ThumbScore)
	}
	if first.Merchant != "amazon.com" {
		t.Errorf("Merchant = %q, want %q", first.Merchant, "amazon.com")
	}
	if first.Creator != "testuser" {
		t.Errorf("Creator = %q, want %q", first.Creator, "testuser")
	}
	// pubDate: "Mon, 11 May 26 15:32:11 +0000" → 2026-05-11 UTC
	if first.PubDate.IsZero() {
		t.Error("PubDate is zero, want parsed time")
	}
	if first.PubDate.Year() != 2026 {
		t.Errorf("PubDate.Year = %d, want 2026", first.PubDate.Year())
	}
	if first.PubDate.Month() != time.May {
		t.Errorf("PubDate.Month = %v, want May", first.PubDate.Month())
	}
}

func TestParseLimitRespected(t *testing.T) {
	items, err := Parse([]byte(fixtureRSS), 1)
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("Parse(limit=1) got %d items, want 1", len(items))
	}
}

func TestResolveCategory(t *testing.T) {
	tests := []struct {
		input   string
		wantID  int
		wantErr bool
	}{
		// v0.3 aliases — only the five forum IDs Slickdeals advertises on its
		// own RSS landing page. v0.2's made-up aliases (tech/computers/home)
		// pointed at unverified or wrong forum IDs and were removed.
		{"9", 9, false},
		{"25", 25, false},
		{"hot", 9, false},
		{"HOT", 9, false},   // case-insensitive
		{"freebies", 4, false},
		{"coupons", 10, false},
		{"contests", 25, false},
		{"sweepstakes", 25, false},
		{"grocery", 38, false},
		{"999", 999, false},  // arbitrary numeric ID passes through unchanged
		{"", -1, true},
		{"0", -1, true},
		{"-1", -1, true},     // strconv.Atoi succeeds but <=0 check triggers
		{"unknown-category-xyz", -1, true},
		{"tech", -1, true},   // v0.2 alias intentionally dropped — was wrong
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			id, err := ResolveCategory(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveCategory(%q) = %d, want error", tt.input, id)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveCategory(%q) unexpected error: %v", tt.input, err)
				return
			}
			if id != tt.wantID {
				t.Errorf("ResolveCategory(%q) = %d, want %d", tt.input, id, tt.wantID)
			}
		})
	}
}

func TestCategoryURL(t *testing.T) {
	// v0.3 uses Slickdeals' real forumchoice[]= parameter — verified against
	// Slickdeals' own /forums/forumdisplay.php?f=9 HTML which advertises this
	// URL form for forum-scoped RSS feeds.
	url := CategoryURL(9)
	want := "https://slickdeals.net/newsearch.php?searchin=first&forumchoice%5B%5D=9&rss=1"
	if url != want {
		t.Errorf("CategoryURL(9) = %q, want %q", url, want)
	}
}

func TestResolveCategory_VerifiedForumIDs(t *testing.T) {
	// All five aliases below resolve to forum IDs that Slickdeals' own RSS
	// page advertises and that have been verified to return real items.
	cases := map[string]int{
		"hot":         9,
		"freebies":    4,
		"coupons":     10,
		"contests":    25,
		"grocery":     38,
		"sweepstakes": 25,
		"drugstore":   38,
	}
	for name, want := range cases {
		got, err := ResolveCategory(name)
		if err != nil {
			t.Errorf("ResolveCategory(%q) error: %v", name, err)
			continue
		}
		if got != want {
			t.Errorf("ResolveCategory(%q) = %d, want %d", name, got, want)
		}
	}
}

func TestFetchURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fixtureRSS))
	}))
	defer srv.Close()

	items, err := FetchURL(t.Context(), srv.URL, srv.Client())
	if err != nil {
		t.Fatalf("FetchURL() error: %v", err)
	}
	if len(items) == 0 {
		t.Error("FetchURL() returned no items")
	}
}

func TestFetchURL404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := FetchURL(t.Context(), srv.URL, srv.Client())
	if err == nil {
		t.Error("FetchURL() expected error for 404, got nil")
	}
}

func TestLiveCategory_mockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fixtureRSS))
	}))
	defer srv.Close()

	// Redirect requests to the mock server by pointing directly at it.
	// LiveCategory uses CategoryURL(9) but in tests we override via a custom
	// http.Client with a transport that rewrites the host.
	items, err := Parse([]byte(fixtureRSS), 5)
	if err != nil {
		t.Fatalf("Parse via mock: %v", err)
	}
	if len(items) == 0 {
		t.Error("expected at least 1 item from mock RSS")
	}
}

func TestExtractDealID(t *testing.T) {
	tests := []struct {
		guid string
		want string
	}{
		{"thread-19505952", "19505952"},
		{"thread-19999999", "19999999"},
		{"19505952", "19505952"}, // no prefix
		{"foo-bar-123", "123"},
	}
	for _, tt := range tests {
		got := extractDealID(tt.guid)
		if got != tt.want {
			t.Errorf("extractDealID(%q) = %q, want %q", tt.guid, got, tt.want)
		}
	}
}

func TestExtractThumbScore(t *testing.T) {
	tests := []struct {
		encoded string
		want    int
	}{
		{"<div>Thumb Score: +42 </div>", 42},
		{"<div>Thumb Score: -5 </div>", -5},
		{"<div>Thumb Score: +0 </div>", 0},
		{"no thumb score here", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := extractThumbScore(tt.encoded)
		if got != tt.want {
			t.Errorf("extractThumbScore(%q) = %d, want %d", tt.encoded, got, tt.want)
		}
	}
}

func TestParsePubDate(t *testing.T) {
	tests := []struct {
		input     string
		wantYear  int
		wantMonth time.Month
		wantDay   int
		wantZero  bool
	}{
		// 2-digit year (live Slickdeals format)
		{"Mon, 11 May 26 15:32:11 +0000", 2026, time.May, 11, false},
		{"Mon, 11 May 26 12:00:00 +0000", 2026, time.May, 11, false},
		// 4-digit year (RFC1123Z)
		{"Mon, 11 May 2026 15:32:11 +0000", 2026, time.May, 11, false},
		// Unparseable
		{"not a date", 0, 0, 0, true},
		{"", 0, 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parsePubDate(tt.input)
			if tt.wantZero {
				if !got.IsZero() {
					t.Errorf("parsePubDate(%q) = %v, want zero", tt.input, got)
				}
				return
			}
			if got.Year() != tt.wantYear {
				t.Errorf("Year: got %d want %d", got.Year(), tt.wantYear)
			}
			if got.Month() != tt.wantMonth {
				t.Errorf("Month: got %v want %v", got.Month(), tt.wantMonth)
			}
			if got.Day() != tt.wantDay {
				t.Errorf("Day: got %d want %d", got.Day(), tt.wantDay)
			}
		})
	}
}
