// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package substack

import (
	"strings"
	"testing"
)

func TestHTMLToText(t *testing.T) {
	in := `<h1>Title</h1><p>First&nbsp;paragraph with <a href="x">a link</a>.</p>` +
		`<figure><img src="https://substackcdn.com/image/fetch/abc.jpg" alt="a chart"><figcaption>caption</figcaption></figure>` +
		`<p>Second paragraph.</p>`
	out := HTMLToText(in)
	if !strings.Contains(out, "First paragraph with a link.") {
		t.Errorf("expected unescaped prose with link text, got:\n%s", out)
	}
	if !strings.Contains(out, "![a chart](https://substackcdn.com/image/fetch/abc.jpg)") {
		t.Errorf("expected image rendered as markdown, got:\n%s", out)
	}
	if strings.Contains(out, "<") || strings.Contains(out, ">") {
		t.Errorf("expected all tags stripped, got:\n%s", out)
	}
}

func TestHTMLToTextStripsScriptAndStyle(t *testing.T) {
	// <script>/<style> inner text must be removed as whole blocks BEFORE the
	// generic tag stripper runs — otherwise their contents leak into the prose
	// and inflate WordCount, which skews the fullBodyRatio entitlement gate.
	// Regression for the Greptile P2 finding.
	in := `<style type="text/css">.foo{color:red;font-size:14px}</style>` +
		`<p>Real prose here.</p>` +
		`<script>var x = 1; alert("leaky leaky leaky");</script>` +
		`<p>More real prose.</p>`
	out := HTMLToText(in)
	for _, leak := range []string{"color:red", "font-size", "alert", "leaky", "var x"} {
		if strings.Contains(out, leak) {
			t.Errorf("script/style inner text leaked into prose (found %q) in:\n%s", leak, out)
		}
	}
	// Only the six real prose words should count; script/style words excluded.
	if got := WordCount(out); got != 6 {
		t.Errorf("WordCount = %d, want 6 (script/style contents must not count):\n%s", got, out)
	}
}

func TestWordCountExcludesImagesAndURLs(t *testing.T) {
	text := "one two three ![alt text here](https://cdn/x.jpg) https://example.com/page four"
	if got := WordCount(text); got != 4 {
		t.Errorf("WordCount = %d, want 4 (image markdown + bare URL excluded)", got)
	}
}

func TestDetectAccess(t *testing.T) {
	cases := []struct {
		name     string
		meta     PostMeta
		rendered int
		authed   bool
		wantFull bool
		wantTier string
	}{
		{
			name:     "free post is full keyless",
			meta:     PostMeta{Audience: "everyone", Wordcount: 1000},
			rendered: 40, // even a short free post is full
			wantFull: true, wantTier: "free",
		},
		{
			name:     "paid full body when rendered ~ wordcount",
			meta:     PostMeta{Audience: "only_paid", Wordcount: 1469, HasIsViewed: true},
			rendered: 1400, authed: true,
			wantFull: true, wantTier: "entitled",
		},
		{
			name:     "paid preview when rendered far below wordcount",
			meta:     PostMeta{Audience: "only_paid", Wordcount: 1469},
			rendered: 300,
			wantFull: false, wantTier: "preview",
		},
		{
			// A high anonymous ratio must NOT be reported as session-unlocked
			// "entitled" — it is full-but-not-via-a-session ("full"), never
			// "entitled" (the false-"entitled"-on-an-anonymous-read gate bug).
			name:     "paid anonymous but ratio-full is full, not entitled",
			meta:     PostMeta{Audience: "only_paid", Wordcount: 1000},
			rendered: 900, authed: false,
			wantFull: true, wantTier: "full",
		},
		{
			// Ratio is authoritative for completeness: an authed read whose body
			// is still short of the declared length is a preview, not entitled.
			name:     "paid authed but short body is preview",
			meta:     PostMeta{Audience: "only_paid", Wordcount: 1469, HasIsViewed: true},
			rendered: 300, authed: true,
			wantFull: false, wantTier: "preview",
		},
		{
			name:     "paid no-wordcount falls back to authed per-user signal",
			meta:     PostMeta{Audience: "founding", HasIsViewed: true},
			rendered: 500, authed: true,
			wantFull: true, wantTier: "entitled",
		},
		{
			name:     "paid no-wordcount anonymous is preview",
			meta:     PostMeta{Audience: "only_paid"},
			rendered: 500,
			wantFull: false, wantTier: "preview",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DetectAccess(tc.meta, tc.rendered, tc.authed)
			if got.Full != tc.wantFull {
				t.Errorf("Full = %v, want %v (%s)", got.Full, tc.wantFull, got.Reason)
			}
			if got.Tier != tc.wantTier {
				t.Errorf("Tier = %q, want %q", got.Tier, tc.wantTier)
			}
		})
	}
}
