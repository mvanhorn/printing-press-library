// Copyright 2026 ChrisDrit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

func TestOwnershipHTMLToText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string // substrings that must be present
		not  []string // substrings that must NOT be present
	}{
		{
			name: "strips tags and decodes entities",
			in:   `<p>Apple&nbsp;Inc. owns 5&#37; of shares</p><script>var x=1;</script>`,
			want: []string{"Apple Inc. owns 5"},
			not:  []string{"<p>", "<script>", "var x=1"},
		},
		{
			name: "block tags become line breaks",
			in:   `<tr><td>Name</td></tr><tr><td>Value</td></tr>`,
			want: []string{"Name", "Value"},
			not:  []string{"<tr>", "<td>"},
		},
		{
			name: "ampersand entity decoded",
			in:   `<div>State Street &amp; Co.</div>`,
			want: []string{"State Street & Co."},
		},
		{
			name: "numeric and hex entities decoded not dropped",
			in:   `<p>owns 5&#37; (or 5&#x25;) of&#160;shares</p>`,
			want: []string{"owns 5%", "5%", "of shares"},
		},
		{
			name: "mismatched script/style tags do not over-strip",
			in:   `<script>JS_GONE</script>KEEP_ONE<style>CSS_GONE</style>KEEP_TWO A<script>STILL_HERE</style>B`,
			want: []string{"KEEP_ONE", "KEEP_TWO", "STILL_HERE"},
			not:  []string{"JS_GONE", "CSS_GONE"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ownershipHTMLToText(tc.in)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("want substring %q in:\n%s", w, got)
				}
			}
			for _, n := range tc.not {
				if strings.Contains(got, n) {
					t.Errorf("unwanted substring %q in:\n%s", n, got)
				}
			}
		})
	}
}

func TestExtractOwnershipSection(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		wantContains string
		wantHeading  string
		wantFallback bool
	}{
		{
			name: "canonical heading wins over table-of-contents mention",
			text: "Table of Contents\n\nSecurity Ownership of Certain Beneficial Owners\n\nProposal 1\n\n" +
				strings.Repeat("filler ", 50) +
				"\n\nSecurity Ownership of Certain Beneficial Owners\n\n" +
				"Vanguard 12,345,678 shares 8% beneficial BlackRock 9,000,000 shares 6% State Street holdings\n\n" +
				"Item 12 Other Matters",
			wantContains: "Vanguard",
			wantHeading:  "security ownership of certain beneficial owners",
			wantFallback: false,
		},
		{
			name: "bounded by next major heading",
			text: "Security Ownership\n\nName Shares Percent\nJohn Doe 1,000,000 5%\n\n" +
				"Item 13 Certain Relationships\n\nUnrelated text that must not appear here.",
			wantContains: "John Doe",
			wantFallback: false,
		},
		{
			name:         "fallback when no heading present",
			text:         "Random preamble. " + strings.Repeat("holdings 1,000 shares percent beneficial stock ", 20),
			wantContains: "shares",
			wantFallback: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res := ExtractOwnershipSection(tc.text)
			if res.Text == "" {
				t.Fatal("expected non-empty section")
			}
			if !strings.Contains(res.Text, tc.wantContains) {
				t.Errorf("want %q in section:\n%s", tc.wantContains, res.Text)
			}
			if tc.wantHeading != "" && res.Heading != tc.wantHeading {
				t.Errorf("heading: got %q want %q", res.Heading, tc.wantHeading)
			}
			if res.Fallback != tc.wantFallback {
				t.Errorf("fallback: got %v want %v", res.Fallback, tc.wantFallback)
			}
			if tc.name == "bounded by next major heading" && strings.Contains(res.Text, "must not appear") {
				t.Errorf("section bled past the next heading:\n%s", res.Text)
			}
		})
	}
}

func TestExtractOwnershipSectionNeverEmptyOnNonEmptyInput(t *testing.T) {
	res := ExtractOwnershipSection("a short document with no ownership tokens at all")
	if res.Text == "" {
		t.Fatal("must never return empty text for non-empty input")
	}
}
