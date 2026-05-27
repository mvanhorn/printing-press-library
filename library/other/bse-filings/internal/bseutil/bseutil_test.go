// Copyright 2026 rushyant-m. Licensed under Apache-2.0. See LICENSE.

package bseutil

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSplitParagraphs(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{
			name: "two blank-separated blocks each below target stay separate",
			in:   "Operating margins expanded this quarter on better mix.\n\nDemand recovery in retail was the key driver of growth.",
			want: []string{
				"Operating margins expanded this quarter on better mix.",
				"Demand recovery in retail was the key driver of growth.",
			},
		},
		{
			name: "wrapped single-newline lines join into one chunk",
			in:   "We expect debt reduction to continue\nover the next two quarters as guidance\nremains intact.",
			want: []string{"We expect debt reduction to continue over the next two quarters as guidance remains intact."},
		},
		{
			name: "drops short header/page-number fragments",
			in:   "3\n\nRELIANCE\n\nThis is a sufficiently long paragraph about pricing and demand trends.",
			want: []string{"This is a sufficiently long paragraph about pricing and demand trends."},
		},
		{
			name: "empty input yields nothing",
			in:   "   \n\n  ",
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SplitParagraphs(tt.in))
		})
	}
}

func TestSplitParagraphsChunksLongRun(t *testing.T) {
	// A single-newline run with no blank lines (the BSE transcript shape)
	// must yield multiple chunks, not one giant paragraph.
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString("This is sentence number ")
		b.WriteString("about margins and demand and growth in the quarter. ")
	}
	got := SplitParagraphs(b.String())
	assert.Greater(t, len(got), 1, "long single-block text should split into multiple chunks")
	for _, c := range got {
		assert.LessOrEqual(t, len(c), paragraphTargetLen+120, "chunks stay near the target length")
	}
}

func TestParseBSEDate(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantOK  bool
		wantStr string // YYYY-MM-DD when ok
	}{
		{"forthcoming format", "27 May 2026", true, "2026-05-27"},
		{"zero-padded day", "02 January 2026", true, "2026-01-02"},
		{"announcement datetime", "2026-05-26T20:33:59.6", true, "2026-05-26"},
		{"compact", "20260527", true, "2026-05-27"},
		{"junk", "not a date", false, ""},
		{"empty", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseBSEDate(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantStr, got.Format("2006-01-02"))
			}
		})
	}
}

func TestCountTerms(t *testing.T) {
	texts := []string{
		"Margins improved and margins guidance is strong.",
		"Demand was soft but pricing held.",
	}
	got := CountTerms(texts, []string{"margin", "demand", "pricing", "debt"})
	assert.Equal(t, 2, got["margin"]) // "margins" twice
	assert.Equal(t, 1, got["demand"])
	assert.Equal(t, 1, got["pricing"])
	assert.Equal(t, 0, got["debt"])
}

func TestParsePeerSearch(t *testing.T) {
	html := `"<li class='quotemenu quotemenuselect' ng-click=\"liclick('500325','RELIANCE INDUSTRIES LTD')\"><a>RELIANCE</a></li>` +
		`<li ng-click=\"liclick('500390','RELIANCE INFRASTRUCTURE LTD')\"></li>"`
	got := ParsePeerSearch(html)
	if assert.Len(t, got, 2) {
		assert.Equal(t, "500325", got[0].ScripCode)
		assert.Equal(t, "RELIANCE INDUSTRIES LTD", got[0].Name)
		assert.Equal(t, "500390", got[1].ScripCode)
	}
	assert.Empty(t, ParsePeerSearch("no match here"))
}

func TestQuarterFromDate(t *testing.T) {
	mk := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	}
	tests := []struct {
		in   time.Time
		want string
	}{
		{mk(2026, time.May, 27), "Q1 FY27"},   // Apr-Jun -> Q1, FY ends 2027
		{mk(2026, time.August, 1), "Q2 FY27"}, // Jul-Sep
		{mk(2025, time.November, 1), "Q3 FY26"},
		{mk(2026, time.February, 18), "Q4 FY26"}, // Jan-Mar -> Q4, FY ends 2026
		{mk(2026, time.April, 26), "Q1 FY27"},    // Reliance transcript filing
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, QuarterFromDate(tt.in))
		})
	}
}
