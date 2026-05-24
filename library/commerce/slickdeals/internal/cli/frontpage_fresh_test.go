// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/slickdeals/internal/rss"
)

// frontpageFreshFixtureXML — three items in feed order. frontpage-fresh
// preserves order (newest first), unlike hot which sorts by thumbs.
const frontpageFreshFixtureXML = `<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>SD</title>
    <item>
      <title>Newest deal</title>
      <link>https://slickdeals.net/f/19510173-newest?utm_source=rss</link>
      <description>fresh</description>
      <content:encoded><![CDATA[<div>Thumb Score: +5 </div><a data-store-slug="amazon">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:30:00 +0000</pubDate>
      <guid>thread-19510173</guid>
    </item>
    <item>
      <title>Middle deal</title>
      <link>https://slickdeals.net/f/19484037-middle?utm_source=rss</link>
      <description>middle</description>
      <content:encoded><![CDATA[<div>Thumb Score: +50 </div><a data-store-slug="costco-wholesale">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:20:00 +0000</pubDate>
      <guid>thread-19484037</guid>
    </item>
    <item>
      <title>Oldest deal</title>
      <link>https://slickdeals.net/f/19465413-oldest?utm_source=rss</link>
      <description>old</description>
      <content:encoded><![CDATA[<div>Thumb Score: +100 </div><a data-store-slug="amazon">x</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:10:00 +0000</pubDate>
      <guid>thread-19465413</guid>
    </item>
  </channel>
</rss>`

// frontpagePipeline mirrors what newFrontpageFreshCmd does: fetch + apply
// caller's limit. Used to test the command's filter behavior without cobra.
func frontpagePipeline(ctx context.Context, hc *http.Client, feedURL string, limit int) ([]rss.Item, error) {
	items, err := rss.FetchURL(ctx, feedURL, hc)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func TestFrontpageFresh_PreservesFeedOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(frontpageFreshFixtureXML))
	}))
	defer server.Close()

	got, err := frontpagePipeline(context.Background(), server.Client(), server.URL, 25)
	if err != nil {
		t.Fatalf("frontpagePipeline: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d want 3", len(got))
	}
	// Frontpage-fresh is "what RSS gave us" — order preserved, NOT
	// sorted by thumbs (which would put the +100 oldest one first).
	if got[0].DealID != "19510173" {
		t.Errorf("first = %q want 19510173 (newest); did the command accidentally sort?", got[0].DealID)
	}
	if got[2].DealID != "19465413" {
		t.Errorf("last = %q want 19465413 (oldest)", got[2].DealID)
	}
}

func TestFrontpageFresh_LimitTruncates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(frontpageFreshFixtureXML))
	}))
	defer server.Close()

	got, err := frontpagePipeline(context.Background(), server.Client(), server.URL, 2)
	if err != nil {
		t.Fatalf("frontpagePipeline: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d want 2", len(got))
	}
	if got[0].DealID != "19510173" || got[1].DealID != "19484037" {
		t.Errorf("limit kept wrong items: %+v", got)
	}
}

func TestFrontpageFresh_ConstructorRegisters(t *testing.T) {
	cmd := newFrontpageFreshCmd(&rootFlags{})
	if cmd.Use != "frontpage-fresh" {
		t.Errorf("Use = %q want frontpage-fresh", cmd.Use)
	}
	if cmd.Annotations["mcp:read-only"] != "true" {
		t.Errorf("missing mcp:read-only annotation")
	}
	if cmd.Flag("limit") == nil {
		t.Errorf("missing --limit flag")
	}
}
