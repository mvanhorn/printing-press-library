package namethatui

import (
	"strings"
	"testing"
	"time"
)

func TestParseTranslationsUsesThreeColumnTable(t *testing.T) {
	page := []byte(`<html><body><table><thead><tr><th>The thing</th><th>AppKit</th><th>SwiftUI</th></tr></thead><tbody><tr><td><a href="/macos/button"><span>Button</span></a><span class="mt-0.5 block">Used for actions</span></td><td><code>NSButton</code></td><td><code>Button</code></td></tr><tr><td>Checkbox</td><td><code>NSButton with switch type</code></td><td><code>Toggle + .toggleStyle(.checkbox)</code></td></tr></tbody></table></body></html>`)
	rows, err := ParseTranslations(page, "https://namethatui.example/")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Plain != "Button" || rows[0].Note != "Used for actions" || rows[0].AppKit != "NSButton" || rows[0].SwiftUI != "Button" || rows[0].SourceURL != "https://namethatui.example/translate" {
		t.Fatalf("rows = %#v", rows)
	}
	if _, err := ParseTranslations([]byte(`<table><tr><th>Name</th></tr></table>`), "https://example.test"); err == nil {
		t.Fatal("expected structure error")
	}
}

func TestParseAndMergeUpdates(t *testing.T) {
	feed, err := ParseFeed([]byte(`<rss><channel><item><title>Button</title><link>https://example.test/button</link><pubDate>Mon, 20 Jul 2026 12:00:00 +0000</pubDate></item><item><title>Unknown</title><link>https://example.test/unknown</link></item></channel></rss>`))
	if err != nil {
		t.Fatal(err)
	}
	sitemap, err := ParseSitemap([]byte(`<urlset><url><loc>https://example.test/button</loc><lastmod>2026-07-19</lastmod></url><url><loc>https://example.test/card</loc><lastmod>2026-07-21T00:00:00Z</lastmod></url></urlset>`))
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeUpdates(feed, sitemap)
	if len(merged) != 3 || merged[0].SourceURL != "https://example.test/card" || merged[1].SourceKind != "feed" || len(merged[1].SourceKinds) != 2 || merged[2].TimestampKnown || merged[2].SourceKinds == nil {
		t.Fatalf("merged = %#v", merged)
	}
	since := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	filtered := FilterUpdates(merged, since, true, 10)
	if len(filtered) != 3 || filtered[2].SourceURL != "https://example.test/unknown" {
		t.Fatalf("filtered = %#v", filtered)
	}
	if _, err := ParseFeed([]byte(`<rss>`)); err == nil || !strings.Contains(err.Error(), "parse RSS") {
		t.Fatalf("malformed RSS error = %v", err)
	}
}
