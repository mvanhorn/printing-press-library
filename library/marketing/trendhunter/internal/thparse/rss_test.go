package thparse

import (
	"os"
	"testing"
)

func TestParseRSSFixtures(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantMin int
	}{
		{name: "tech category rss can be empty", path: "testdata/th-rss-tech.xml", wantMin: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatal(err)
			}
			trends, err := ParseRSS(body)
			if err != nil {
				t.Fatal(err)
			}
			if len(trends) < tt.wantMin {
				t.Fatalf("got %d trends, want at least %d", len(trends), tt.wantMin)
			}
		})
	}
}

func TestParseRSSItemFields(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><rss><channel><item><title><![CDATA[AI Widgets - Smart widgets for homes (TrendHunter.com)]]></title><link>http://www.trendhunter.com/innovation/ai-widgets</link><description><![CDATA[<a><img src='https://cdn.trendhunterstatic.com/thumbs/ai.jpeg' /></a> body]]></description><pubDate>Wed, 13 May 2026 16:43 GMT</pubDate></item></channel></rss>`)
	trends, err := ParseRSS(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(trends) != 1 {
		t.Fatalf("got %d trends, want 1", len(trends))
	}
	if trends[0].Slug != "ai-widgets" || trends[0].Title != "AI Widgets" || trends[0].Description != "Smart widgets for homes" || trends[0].ImageURL == "" {
		t.Fatalf("unexpected parsed trend: %+v", trends[0])
	}
}
