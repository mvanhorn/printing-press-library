// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package rss

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fixtureXML is a faithful (slightly trimmed) copy of the real Slickdeals
// frontpage RSS feed captured 2026-05-11. Two items: one hot (+22 thumbs at
// Amazon), one lower (+18 thumbs at Costco). Critical bits preserved:
//   - 2-digit year pubDate (`Mon, 11 May 26 15:23:37 +0000`)
//   - /f/<id>-slug?utm_... link pattern
//   - "Thumb Score: +N" inside CDATA description
//   - data-store-slug="amazon" / "costco-wholesale" inside content:encoded
const fixtureXML = `<?xml version="1.0"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>Slickdeals Frontpage RSS Feed</title>
    <link>https://slickdeals.net/</link>
    <description>test feed</description>
    <language>en</language>
    <lastBuildDate>Mon, 11 May 2026 15:28:57 GMT</lastBuildDate>
    <item>
      <title><![CDATA[Select Accounts: 2.2-Lb Lavazza Espresso Whole Bean Coffee $11.35]]></title>
      <link>https://slickdeals.net/f/19510173-sns-ac-11-37-35-2-oz-lavazza?utm_source=rss&amp;utm_content=fp&amp;utm_medium=RSS2</link>
      <description><![CDATA[Amazon [amazon.com] has *35.2-Oz Lavazza Whole Bean Coffee* for $11.37.]]></description>
      <content:encoded><![CDATA[<div>Thumb Score: +22 </div><div><a href="https://slickdeals.net/click?u=xyz" data-store-slug="amazon" rel="nofollow">Amazon</a> has the deal.</div>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:23:37 +0000</pubDate>
      <category domain="https://slickdeals.net/">Frontpage Deals</category>
      <dc:creator>phoinix</dc:creator>
      <guid>thread-19510173</guid>
    </item>
    <item>
      <title>Costco Stores: Tresanti Aurora 47" Desk $200</title>
      <link>https://slickdeals.net/f/19484037-costco-tresanti-desk?utm_source=rss</link>
      <description><![CDATA[Select Costco Locations have desks on sale.]]></description>
      <content:encoded><![CDATA[<div>Thumb Score: +18 </div><div><a data-store-slug="costco-wholesale">Costco</a></div>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:21:19 +0000</pubDate>
      <category domain="https://slickdeals.net/">Frontpage Deals</category>
      <guid>thread-19484037</guid>
    </item>
  </channel>
</rss>`

func TestParsePubDate_Core(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    string // RFC3339 form for comparison
		wantErr bool
	}{
		{"slickdeals 2-digit year numeric tz", "Mon, 11 May 26 15:23:37 +0000", "2026-05-11T15:23:37Z", false},
		{"slickdeals 2-digit year GMT", "Mon, 11 May 26 15:23:37 GMT", "2026-05-11T15:23:37Z", false},
		{"rfc1123z 4-digit year", "Mon, 11 May 2026 15:23:37 +0000", "2026-05-11T15:23:37Z", false},
		{"rfc1123 4-digit year GMT", "Mon, 11 May 2026 15:23:37 GMT", "2026-05-11T15:23:37Z", false},
		{"empty", "", "", true},
		{"garbage", "yesterday", "", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParsePubDate(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if got.UTC().Format(time.RFC3339) != tc.want {
				t.Errorf("got %s want %s", got.UTC().Format(time.RFC3339), tc.want)
			}
		})
	}
}

func TestExtractDealID_Core(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"https://slickdeals.net/f/19510173-foo-bar?utm_source=rss", "19510173"},
		{"https://slickdeals.net/f/19484037-costco-desk", "19484037"},
		{"https://slickdeals.net/", ""},
		{"", ""},
		{"https://example.com/something/else", ""},
	}
	for _, tc := range tests {
		if got := ExtractDealID(tc.in); got != tc.want {
			t.Errorf("ExtractDealID(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestExtractThumbs(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"<div>Thumb Score: +22 </div>", 22},
		{"<div>Thumb Score: +114 </div>", 114},
		{"<div>Thumb Score: -3 </div>", -3},
		{"<div>Thumb Score: 0 </div>", 0},
		{"no thumb info here", 0},
		{"", 0},
	}
	for _, tc := range tests {
		if got := ExtractThumbs(tc.in); got != tc.want {
			t.Errorf("ExtractThumbs(%q) = %d want %d", tc.in, got, tc.want)
		}
	}
}

func TestExtractMerchant(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{`<a data-store-slug="amazon" rel="nofollow">Amazon</a>`, "amazon"},
		{`<a data-store-slug="costco-wholesale">Costco</a>`, "costco-wholesale"},
		{`<a href="x">no slug</a>`, ""},
		{"", ""},
		// First match wins when multiple anchors are present.
		{`<a data-store-slug="amazon">A</a> <a data-store-slug="bestbuy">B</a>`, "amazon"},
	}
	for _, tc := range tests {
		if got := ExtractMerchant(tc.in); got != tc.want {
			t.Errorf("ExtractMerchant(%q) = %q want %q", tc.in, got, tc.want)
		}
	}
}

func TestParse_RSSCore(t *testing.T) {
	items, err := ParseReader(strings.NewReader(fixtureXML), 0)
	if err != nil {
		t.Fatalf("ParseReader error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items want 2", len(items))
	}

	first := items[0]
	if first.DealID != "19510173" {
		t.Errorf("DealID = %q want 19510173", first.DealID)
	}
	if first.Thumbs != 22 {
		t.Errorf("Thumbs = %d want 22", first.Thumbs)
	}
	if first.Merchant != "amazon" {
		t.Errorf("Merchant = %q want amazon", first.Merchant)
	}
	if first.GUID != "thread-19510173" {
		t.Errorf("GUID = %q want thread-19510173", first.GUID)
	}
	if first.PubDate.IsZero() {
		t.Errorf("PubDate is zero; should have parsed 2-digit year format")
	}
	if !strings.Contains(first.Title, "Lavazza") {
		t.Errorf("Title = %q want it to contain Lavazza", first.Title)
	}
	if first.Description == "" {
		t.Errorf("Description was not cleaned/populated")
	}

	second := items[1]
	if second.Merchant != "costco-wholesale" {
		t.Errorf("second.Merchant = %q want costco-wholesale", second.Merchant)
	}
	if second.Thumbs != 18 {
		t.Errorf("second.Thumbs = %d want 18", second.Thumbs)
	}
}

func TestFetchURL_Mocked(t *testing.T) {
	gotUA := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fixtureXML))
	}))
	defer server.Close()

	items, err := FetchURL(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("FetchURL: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items want 2", len(items))
	}
	if !strings.Contains(gotUA, "slickdeals-pp-cli") {
		t.Errorf("User-Agent = %q want it to identify slickdeals-pp-cli", gotUA)
	}
}

func TestFetchURL_Non2xxIsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := FetchURL(context.Background(), server.URL, server.Client())
	if err == nil {
		t.Fatalf("expected non-2xx to return error")
	}
}

func TestFilterHot(t *testing.T) {
	items := []Item{
		{DealID: "1", Thumbs: 100},
		{DealID: "2", Thumbs: 10},
		{DealID: "3", Thumbs: 50},
		{DealID: "4", Thumbs: 200},
	}
	got := FilterHot(items, 50, 2)
	if len(got) != 2 {
		t.Fatalf("got %d want 2", len(got))
	}
	// Sorted DESC by thumbs after filter
	if got[0].DealID != "4" || got[1].DealID != "1" {
		t.Errorf("ordering wrong: %+v", got)
	}

	// No filter, no limit
	all := FilterHot(items, 0, 0)
	if len(all) != 4 {
		t.Fatalf("got %d want 4", len(all))
	}
	if all[0].Thumbs != 200 {
		t.Errorf("not sorted DESC: %+v", all)
	}
}

func TestLiveHot_Mocked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureXML))
	}))
	defer server.Close()

	// Override frontpageURL via FetchURL directly — we mirror what LiveHot
	// does internally so we can target the mock server.
	items, err := FetchURL(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("FetchURL: %v", err)
	}
	hot := FilterHot(items, 20, 5)
	// Only the +22 Amazon item beats min-thumbs=20.
	if len(hot) != 1 {
		t.Fatalf("got %d hot items want 1", len(hot))
	}
	if hot[0].DealID != "19510173" {
		t.Errorf("got DealID %q want 19510173", hot[0].DealID)
	}
}

func TestLiveSearchRSS_LimitTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(fixtureXML))
	}))
	defer server.Close()

	// LiveSearchRSS hardcodes the production URL, so test the limit logic
	// through FetchURL + manual truncation to mirror what LiveSearchRSS does.
	items, err := FetchURL(context.Background(), server.URL, server.Client())
	if err != nil {
		t.Fatalf("FetchURL: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d want 2", len(items))
	}
	if items[:1][0].DealID != "19510173" {
		t.Errorf("truncation order wrong")
	}
}
