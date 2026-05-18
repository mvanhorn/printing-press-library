// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixtureFrontpageRSS contains two items: deal 19510173 ("Found Deal") and
// deal 88888888 ("Other Deal"). Used to verify fetchWatchItem can find the
// matching deal and return notFoundErr when the ID isn't on the page.
const fixtureFrontpageRSS = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Slickdeals: Frontpage</title>
    <link>https://slickdeals.net/</link>
    <description>Frontpage</description>
    <item>
      <title>Found Deal — 50% off Widget</title>
      <link>https://slickdeals.net/f/19510173-found-deal</link>
      <description><![CDATA[Thumb Score: +42]]></description>
      <content:encoded><![CDATA[<a class="outclick" data-store-slug="acme">Buy</a>Thumb Score: +42]]></content:encoded>
      <pubDate>Mon, 11 May 26 15:23:37 +0000</pubDate>
      <guid>https://slickdeals.net/f/19510173-found-deal</guid>
    </item>
    <item>
      <title>Other Deal</title>
      <link>https://slickdeals.net/f/88888888-other-deal</link>
      <description><![CDATA[Thumb Score: +12]]></description>
      <content:encoded><![CDATA[<a class="outclick" data-store-slug="bigstore">Buy</a>Thumb Score: +12]]></content:encoded>
      <pubDate>Mon, 11 May 26 14:00:00 +0000</pubDate>
      <guid>https://slickdeals.net/f/88888888-other-deal</guid>
    </item>
  </channel>
</rss>`

func newWatchFixtureServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestFetchWatchItem_DealFound(t *testing.T) {
	srv := newWatchFixtureServer(t, fixtureFrontpageRSS)

	item, err := fetchWatchItem(context.Background(), srv.Client(), srv.URL, "19510173")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if item == nil {
		t.Fatal("expected non-nil item")
	}
	if item.DealID != "19510173" {
		t.Errorf("want DealID 19510173, got %q", item.DealID)
	}
	if !strings.HasPrefix(item.Title, "Found Deal") {
		t.Errorf("want Found Deal title, got %q", item.Title)
	}
	if item.Thumbs != 42 {
		t.Errorf("want Thumbs=42, got %d", item.Thumbs)
	}
	if item.Merchant != "acme" {
		t.Errorf("want Merchant=acme, got %q", item.Merchant)
	}
}

func TestFetchWatchItem_DealNotFound(t *testing.T) {
	srv := newWatchFixtureServer(t, fixtureFrontpageRSS)

	_, err := fetchWatchItem(context.Background(), srv.Client(), srv.URL, "11111111")
	if err == nil {
		t.Fatal("expected notFoundErr for missing deal, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 3 {
		t.Fatalf("expected notFoundErr (code 3), got %v", err)
	}
	if !strings.Contains(err.Error(), "11111111") {
		t.Errorf("error should mention requested deal ID, got %q", err.Error())
	}
}

func TestNewWatchCmd_InvalidDealID(t *testing.T) {
	flags := &rootFlags{}
	cmd := newWatchCmd(flags)
	cmd.SetArgs([]string{"not-a-number"})
	// Discard help/usage output — we only care about the error.
	cmd.SetOut(discardWriter{})
	cmd.SetErr(discardWriter{})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for non-numeric deal-id, got nil")
	}
	var ce *cliError
	if !errors.As(err, &ce) || ce.code != 2 {
		t.Fatalf("expected usageErr (code 2), got %v", err)
	}
	if !strings.Contains(err.Error(), "numeric") {
		t.Errorf("error should explain numeric requirement, got %q", err.Error())
	}
}

func TestNewWatchCmd_DryRun(t *testing.T) {
	flags := &rootFlags{dryRun: true}
	cmd := newWatchCmd(flags)
	cmd.SetArgs([]string{"19510173"})
	cmd.SetOut(discardWriter{})
	cmd.SetErr(discardWriter{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("dry-run should short-circuit cleanly, got %v", err)
	}
}

// discardWriter is a no-op io.Writer for tests that don't care about output.
type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }
