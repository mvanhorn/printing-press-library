// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeFaultString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"designer fault line", "Fault - LAN A Streaming Error - Not Connected", "fault-lan-a-streaming-error-not-connected"},
		{"already a slug", "error-fault-lan-a-streaming-error-not-connected", "error-fault-lan-a-streaming-error-not-connected"},
		{"collapses separator runs", "Fault  --  LAN A   Streaming", "fault-lan-a-streaming"},
		{"trims leading and trailing punctuation", "  ...Not Connected!!  ", "not-connected"},
		{"keeps digits", "Core 110f LAN B Down", "core-110f-lan-b-down"},
		{"mixed unicode separators", "Fault — LAN A – Down", "fault-lan-a-down"},
		{"empty", "   ", ""},
		{"punctuation only", "---", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeFaultString(tc.in); got != tc.want {
				t.Fatalf("normalizeFaultString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestFaultKeyJoinsDesignerStringToVendorSlug is the load-bearing case: the
// string Designer prints and the slug QSC files the article under must reduce
// to one key, with or without the classification prefixes.
func TestFaultKeyJoinsDesignerStringToVendorSlug(t *testing.T) {
	const wantKey = "lan-a-streaming-error-not-connected"
	tests := []struct {
		name string
		in   string
	}{
		{"designer string with fault prefix", "Fault - LAN A Streaming Error - Not Connected"},
		{"designer string without prefix", "LAN A Streaming Error - Not Connected"},
		{"vendor slug", "error-fault-lan-a-streaming-error-not-connected"},
		{"vendor title", "Fault — LAN A Streaming Error — Not Connected"},
		{"shouted", "FAULT: LAN A STREAMING ERROR / NOT CONNECTED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := faultKey(tc.in); got != wantKey {
				t.Fatalf("faultKey(%q) = %q, want %q", tc.in, got, wantKey)
			}
		})
	}

	// And the pairing the command actually performs.
	const slug = "error-fault-lan-a-streaming-error-not-connected"
	const title = "Fault - LAN A Streaming Error - Not Connected"
	if rank := faultRank(faultKey(title), slug, title); rank < 0 {
		t.Fatalf("faultRank(key(%q), %q, %q) = %d, want a match", title, slug, title, rank)
	}
}

func TestFaultRank(t *testing.T) {
	const slug = "error-fault-lan-a-streaming-error-not-connected"
	const title = "Fault - LAN A Streaming Error - Not Connected"
	tests := []struct {
		name      string
		query     string
		wantMatch bool
	}{
		{"exact designer string", "Fault - LAN A Streaming Error - Not Connected", true},
		{"without fault prefix", "LAN A Streaming Error - Not Connected", true},
		{"leading substring", "LAN A Streaming Error", true},
		{"pasted with surrounding words", "Status: error-fault-lan-a-streaming-error-not-connected on Core", true},
		{"different fault", "LAN B Link Down", false},
		{"nonsense", "zzzz flurb qqq nonsense string", false},
		{"too short to identify", "lan", false},
		{"empty", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := faultRank(faultKey(tc.query), slug, title) >= 0
			if got != tc.wantMatch {
				t.Fatalf("faultRank for %q matched = %v, want %v", tc.query, got, tc.wantMatch)
			}
		})
	}
}

// TestFaultRankRejectsUnrelatedArticles guards the behavior the negative test
// depends on: a nonsense string must not drag in a plausible-looking article.
func TestFaultRankRejectsUnrelatedArticles(t *testing.T) {
	corpus := []struct{ slug, title string }{
		{"error-fault-lan-a-streaming-error-not-connected", "Fault - LAN A Streaming Error - Not Connected"},
		{"error-status-initializing", "Status - Initializing"},
		{"troubleshooting-dante-clock-sync", "Troubleshooting Dante Clock Sync"},
		{"error-fault-fan-failure", "Fault - Fan Failure"},
	}
	key := faultKey("Flux Capacitor Desynchronization Overload")
	for _, a := range corpus {
		if rank := faultRank(key, a.slug, a.title); rank >= 0 {
			t.Fatalf("nonsense query matched %q with rank %d", a.slug, rank)
		}
	}
}

func TestMentionsModel(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		model string
		want  bool
	}{
		{"standalone token", "affects the cx-q series amplifiers", "CX-Q", true},
		{"sku suffix after separator", "the cx-q 8k8 is affected", "CX-Q", true},
		{"hyphenated sku", "affects cx-q-8k8 units", "CX-Q", true},
		{"start of text", "cx-q amplifiers", "CX-Q", true},
		{"end of text", "applies to the cx-q", "CX-Q", true},
		{"longer model is not a hit for shorter suffix", "nc-900 cameras", "NC-90", false},
		{"embedded in a longer word", "the tsc-70-g3x prototype", "TSC-70-G3", false},
		{"absent", "core 110f only", "CX-Q", false},
		{"too short to be a model", "the q core", "Q", false},
		{"empty text", "", "CX-Q", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := mentionsModel(tc.text, tc.model); got != tc.want {
				t.Fatalf("mentionsModel(%q, %q) = %v, want %v", tc.text, tc.model, got, tc.want)
			}
		})
	}
}

func TestModelsMentionedIsSortedAndDeduped(t *testing.T) {
	models := []string{"TSC-70-G3", "CX-Q", "Core 110f"}
	got := modelsMentioned(models, "The CX-Q and the TSC-70-G3 are both affected; CX-Q again.")
	want := []string{"CX-Q", "TSC-70-G3"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("modelsMentioned = %v, want %v", got, want)
	}
}

func TestArticleVersions(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"designer prose", "Fixed in Q-SYS Designer 10.0.2 and later", []string{"10.0.2"}},
		{"qds shorthand", "Applies to QDS 9.4", []string{"9.4"}},
		{"version word", "Designer version 10.1 introduces", []string{"10.1"}},
		{"v prefix", "Designer Software v10.4", []string{"10.4"}},
		{"multiple, version sorted", "Designer 10.1 and Designer 9.4 both", []string{"9.4", "10.1"}},
		{"deduped", "QDS 10.0 and QDS 10.0", []string{"10.0"}},
		{"ignores bare numbers", "the 8.2 amp rating and firmware 3.1", []string{}},
		{"none", "General wiring guidance", []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := articleVersions(tc.text)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("articleVersions(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestVersionSeriesMatch(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"10.0", "10.0", true},
		{"10.0", "10.0.2", true},
		{"10.0.2", "10.0", true},
		{"10.0", "10.1", false},
		{"10.0", "1.0", false},
		{"9.4", "9.4.1", true},
		{"", "10.0", false},
		{"10.0", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.a+"~"+tc.b, func(t *testing.T) {
			if got := versionSeriesMatch(tc.a, tc.b); got != tc.want {
				t.Fatalf("versionSeriesMatch(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestVersionRelevant(t *testing.T) {
	tests := []struct {
		name   string
		target string
		have   []string
		want   bool
	}{
		{"no target means no filter", "", []string{"9.4"}, true},
		{"version-agnostic article is kept", "10.0", nil, true},
		{"same release line", "10.0", []string{"10.0.2"}, true},
		{"different line is dropped", "10.0", []string{"9.4"}, false},
		{"any listed version may match", "10.0", []string{"9.4", "10.0.1"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := versionRelevant(tc.target, tc.have); got != tc.want {
				t.Fatalf("versionRelevant(%q, %v) = %v, want %v", tc.target, tc.have, got, tc.want)
			}
		})
	}
}

func TestDetectLTS(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		wantLTS bool
		wantEnd string
	}{
		{"lts with long-form date", "Q-SYS Designer 9.4 is an LTS release. LTS support ends December 31, 2026.", true, "December 31, 2026"},
		{"end of life wording", "Designer 9.4 LTS: end of support March 2027 for this branch.", true, "March 2027"},
		{"iso date", "This is a long-term support release; support ends 2027-03-31.", true, "2027-03-31"},
		{"lts with no stated date", "Designer 10.0 is designated LTS.", true, ""},
		{"not an lts article", "Designer 10.0 fixes a Dante clocking defect.", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			end, ok := detectLTS(tc.text)
			if ok != tc.wantLTS {
				t.Fatalf("detectLTS(%q) ok = %v, want %v", tc.text, ok, tc.wantLTS)
			}
			if end != tc.wantEnd {
				t.Fatalf("detectLTS(%q) end = %q, want %q", tc.text, end, tc.wantEnd)
			}
		})
	}
}

func TestRiskCategoryOrder(t *testing.T) {
	if riskCategoryOrder("known-issues") >= riskCategoryOrder("troubleshooting") {
		t.Fatal("known-issues must sort before troubleshooting")
	}
	if riskCategoryOrder("faq") != len(riskCategories) {
		t.Fatalf("an unknown category must sort last, got %d", riskCategoryOrder("faq"))
	}
}

func TestExcerpt(t *testing.T) {
	long := strings.Repeat("a", 100)
	tests := []struct {
		name string
		in   string
		n    int
		want string
	}{
		{"under the cap is untouched", "short", 20, "short"},
		{"zero means no cap", long, 0, long},
		{"truncates with an ellipsis", long, 10, strings.Repeat("a", 10) + "…"},
		{"trims surrounding space", "  padded  ", 20, "padded"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := excerpt(tc.in, tc.n); got != tc.want {
				t.Fatalf("excerpt(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
			}
		})
	}
}
