package news

import (
	"strings"
	"testing"
	"time"
)

func TestParseRSS(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/">
<channel>
  <title>Test Feed</title>
  <item>
    <title>DoD picks Leidos for $500M cloud contract</title>
    <link>https://example.com/article-1</link>
    <description>Short summary here</description>
    <content:encoded><![CDATA[<p>Long body with <strong>HTML</strong> tags.</p>]]></content:encoded>
    <dc:creator>Reporter Name</dc:creator>
    <pubDate>Mon, 11 May 2026 14:30:00 -0400</pubDate>
    <category>Cloud</category>
    <category>DoD</category>
    <guid>tag:example.com,2026:article-1</guid>
  </item>
  <item>
    <title>GSA awards Booz Allen modernization deal</title>
    <link>https://example.com/article-2</link>
    <description>Another summary</description>
    <pubDate>Sun, 10 May 2026 09:00:00 -0400</pubDate>
    <guid>tag:example.com,2026:article-2</guid>
  </item>
</channel>
</rss>`)
	items, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Title != "DoD picks Leidos for $500M cloud contract" {
		t.Errorf("title 1 mismatch: %q", items[0].Title)
	}
	if items[0].GUID != "tag:example.com,2026:article-1" {
		t.Errorf("guid 1 mismatch: %q", items[0].GUID)
	}
	if items[0].Author != "Reporter Name" {
		t.Errorf("author 1 mismatch: %q", items[0].Author)
	}
	if len(items[0].Categories) != 2 || items[0].Categories[0] != "Cloud" {
		t.Errorf("categories mismatch: %v", items[0].Categories)
	}
	if items[0].PublishedAt.IsZero() {
		t.Error("pub date should be parsed")
	}
	if !strings.Contains(items[0].Content, "Long body") {
		t.Errorf("content should have HTML stripped to plain text, got: %q", items[0].Content)
	}
	if strings.Contains(items[0].Content, "<strong>") {
		t.Error("content should not contain HTML tags")
	}
}

func TestParseAtom(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Atom Test</title>
  <entry>
    <id>urn:uuid:abc-123</id>
    <title>FedRAMP authorization milestone</title>
    <link rel="alternate" href="https://example.com/atom-1"/>
    <published>2026-05-11T18:00:00Z</published>
    <updated>2026-05-11T18:00:00Z</updated>
    <author><name>Atom Author</name></author>
    <summary>Atom summary</summary>
    <content type="html">&lt;p&gt;HTML-escaped content&lt;/p&gt;</content>
    <category term="FedRAMP"/>
  </entry>
</feed>`)
	items, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Title != "FedRAMP authorization milestone" {
		t.Errorf("title mismatch: %q", it.Title)
	}
	if it.Link != "https://example.com/atom-1" {
		t.Errorf("link mismatch: %q", it.Link)
	}
	if it.Author != "Atom Author" {
		t.Errorf("author mismatch: %q", it.Author)
	}
	if it.PublishedAt.IsZero() || !it.PublishedAt.Equal(time.Date(2026, 5, 11, 18, 0, 0, 0, time.UTC)) {
		t.Errorf("published mismatch: %v", it.PublishedAt)
	}
}

func TestExtractMentions(t *testing.T) {
	text := "The Department of Defense awarded Leidos a $500M cloud contract. GSA also picked Microsoft for a separate modernization deal. Booz Allen Hamilton was the runner-up."
	vendors := []string{"Leidos", "Microsoft", "Booz Allen Hamilton", "IBM"}
	agencies := []struct{ Name, Abbrev string }{
		{"Department of Defense", "DoD"},
		{"General Services Administration", "GSA"},
		{"Department of Energy", "DoE"},
	}
	tags := ExtractMentions(text, vendors, agencies)
	if len(tags) == 0 {
		t.Fatal("expected at least one tag")
	}
	hits := map[string]bool{}
	for _, t := range tags {
		hits[t.Kind+"|"+t.Value] = true
	}
	wantVendors := []string{"Leidos", "Microsoft", "Booz Allen Hamilton"}
	for _, w := range wantVendors {
		if !hits["recipient|"+w] {
			t.Errorf("expected to find recipient tag for %q, got tags=%v", w, tags)
		}
	}
	if !hits["agency|Department of Defense"] && !hits["agency|General Services Administration"] {
		t.Errorf("expected at least one agency tag, got tags=%v", tags)
	}
	if hits["recipient|IBM"] {
		t.Errorf("should not have matched IBM (not in text), got tags=%v", tags)
	}
}

func TestExtractMentionsWordBoundary(t *testing.T) {
	// "Microsoft" should match only when bounded, not inside "macrosoftware"
	text := "We discussed macrosoftware risks. Microsoft also was mentioned."
	tags := ExtractMentions(text, []string{"Microsoft"}, nil)
	if len(tags) != 1 {
		t.Errorf("expected 1 word-bounded match for Microsoft, got %d: %v", len(tags), tags)
	}
}

func TestExtractMentionsDedup(t *testing.T) {
	// Multiple occurrences of same vendor produce one tag
	text := "Leidos won. Then Leidos won again. Then Leidos won a third time."
	tags := ExtractMentions(text, []string{"Leidos"}, nil)
	if len(tags) != 1 {
		t.Errorf("expected dedup to one tag, got %d: %v", len(tags), tags)
	}
}

func TestExtractMentionsCaseInsensitive(t *testing.T) {
	text := "leidos and LEIDOS and Leidos all appear here"
	tags := ExtractMentions(text, []string{"Leidos"}, nil)
	if len(tags) != 1 {
		t.Errorf("expected case-insensitive single match, got %d", len(tags))
	}
}

func TestExtractMentionsShortNamesSkipped(t *testing.T) {
	// Names shorter than 4 chars are skipped to avoid noise (e.g., "IBM" inside "IBMs", "AT" inside "AT&T", etc.)
	text := "This mentions IBM and HP and AT&T and Dell."
	tags := ExtractMentions(text, []string{"IBM", "HP", "Dell"}, nil)
	hits := map[string]bool{}
	for _, t := range tags {
		hits[t.Value] = true
	}
	if hits["IBM"] || hits["HP"] {
		t.Errorf("3-char names should be skipped, got tags=%v", tags)
	}
	if !hits["Dell"] {
		t.Errorf("expected Dell (4 chars) to match, got tags=%v", tags)
	}
}

func TestItemIDDeterministic(t *testing.T) {
	a := ItemID("fedscoop", "guid-1")
	b := ItemID("fedscoop", "guid-1")
	c := ItemID("fedscoop", "guid-2")
	if a != b {
		t.Errorf("same inputs should produce same ID: %s vs %s", a, b)
	}
	if a == c {
		t.Errorf("different inputs should produce different ID: %s vs %s", a, c)
	}
	if len(a) == 0 {
		t.Errorf("ID should be non-empty")
	}
}

func TestStripHTMLBasic(t *testing.T) {
	in := `<p>Hello &amp; goodbye &#39;world&#39;.</p><br/>`
	out := stripHTMLBasic(in)
	if strings.Contains(out, "<p>") || strings.Contains(out, "<br") {
		t.Errorf("expected HTML stripped, got %q", out)
	}
	if !strings.Contains(out, "Hello & goodbye 'world'.") {
		t.Errorf("expected entities decoded, got %q", out)
	}
}

func TestParseDateMultipleFormats(t *testing.T) {
	cases := []struct {
		in   string
		zero bool
	}{
		{"Mon, 11 May 2026 14:30:00 -0400", false},
		{"2026-05-11T18:00:00Z", false},
		{"2026-05-11", false},
		{"", true},
		{"garbage", true},
	}
	for _, c := range cases {
		got := parseDate(c.in)
		if c.zero != got.IsZero() {
			t.Errorf("parseDate(%q) zero=%t expected zero=%t", c.in, got.IsZero(), c.zero)
		}
	}
}

func TestDefaultSourcesValid(t *testing.T) {
	if len(DefaultSources) < 8 {
		t.Errorf("expected at least 8 default sources, got %d", len(DefaultSources))
	}
	for _, s := range DefaultSources {
		if s.ID == "" || s.FeedURL == "" || s.Name == "" {
			t.Errorf("default source has empty required field: %+v", s)
		}
		if !strings.HasPrefix(s.FeedURL, "https://") {
			t.Errorf("source %s feed URL should be HTTPS: %s", s.ID, s.FeedURL)
		}
	}
}
