// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package thparse

import "testing"

func TestParseRSS(t *testing.T) {
	raw := []byte(`<rss><channel><item><title>AI Clone Assistants</title><link>https://www.trendhunter.com/trends/ai-clone</link><description><![CDATA[Personalized <b>AI</b> helpers.]]></description><pubDate>Wed, 13 May 2026 10:00:00 -0700</pubDate></item></channel></rss>`)
	trends, err := ParseRSS(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(trends) != 1 {
		t.Fatalf("len = %d, want 1", len(trends))
	}
	if trends[0].Slug != "ai-clone" || trends[0].Title != "AI Clone Assistants" {
		t.Fatalf("unexpected trend: %#v", trends[0])
	}
	if trends[0].PubDate == "" {
		t.Fatal("pub_date was not parsed")
	}
}

func TestParseTrendPageFAQ(t *testing.T) {
	raw := []byte(`<html><script type="application/ld+json">{
		"@type":"FAQPage",
		"mainEntity":[{"@type":"Question","name":"What is it?","acceptedAnswer":{"@type":"Answer","text":"<p>An AI clone.</p>"}}]
	}</script></html>`)
	trend, err := ParseTrendPage(raw, "https://www.trendhunter.com/trends/ai-clone")
	if err != nil {
		t.Fatal(err)
	}
	if len(trend.FAQ) != 1 {
		t.Fatalf("len = %d, want 1", len(trend.FAQ))
	}
	if trend.FAQ[0].Question != "What is it?" || trend.FAQ[0].Answer != "An AI clone." {
		t.Fatalf("unexpected FAQ: %#v", trend.FAQ[0])
	}
}
