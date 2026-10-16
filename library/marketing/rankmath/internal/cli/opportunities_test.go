// Copyright 2026 ElizabethcStone and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/rankmath/internal/seohead"
)

// TestNovelOpportunitiesHelpWires smoke-tests that the opportunities command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelOpportunitiesHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"opportunities", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("opportunities --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "opportunities"} {
		if !strings.Contains(help, want) {
			t.Fatalf("opportunities --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestBuildOpportunityRequiresPageSnapshot(t *testing.T) {
	row := keywordRow{Query: "yoga retreat", Position: 5, Impressions: 90}
	page := keywordRow{Page: "https://www.example.test/yoga/"}
	snapshots := map[string]opportunitySnapshot{pageKey("https://example.test/other/"): {head: seohead.Head{URL: "https://example.test/other/", Title: "Other page"}}}
	o, matched := buildOpportunity(row, page, snapshots, map[string]opportunitySnapshot{})
	if matched || o.MatchScope != "page" || len(o.MissingIn) != 2 {
		t.Fatalf("unexpected unmatched page result: %+v matched=%v", o, matched)
	}
	head := seohead.Head{URL: "https://example.test/yoga", Title: "Yoga retreat", Description: "Bali yoga retreat"}
	snapshots[pageKey(head.URL)] = opportunitySnapshot{head: head, titleTokens: tokenSet(head.Title), descTokens: tokenSet(head.Description)}
	o, matched = buildOpportunity(row, page, snapshots, map[string]opportunitySnapshot{})
	if !matched || len(o.MissingIn) != 0 {
		t.Fatalf("matching snapshot result: %+v matched=%v", o, matched)
	}
}

func TestDecodeKeywordRows(t *testing.T) {
	for _, tc := range []struct {
		name, raw   string
		n           int
		query       string
		impressions float64
		bad         bool
	}{
		{"empty", `{"response":"No Data"}`, 0, "", 0, false},
		{"array", `[{"query":"yoga","impressions":70,"position":{"total":5,"difference":1},"clicks":{"total":3},"ctr":2}]`, 1, "yoga", 70, false},
		{"map", `{"yoga retreat":{"impressions":{"total":80},"position":6}}`, 1, "yoga retreat", 80, false},
		{"invalid", `{"x":{"impressions":"oops"}}`, 0, "", 0, true},
		{"empty array", `[]`, 0, "", 0, false},
		{"pages map", `{"/yoga/":{"page":"/yoga/","impressions":90}}`, 1, "/yoga/", 90, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows, err := decodeKeywordRows([]byte(tc.raw))
			if (err != nil) != tc.bad {
				t.Fatalf("error %v", err)
			}
			if tc.bad {
				return
			}
			if len(rows) != tc.n {
				t.Fatalf("%+v", rows)
			}
			if tc.name == "array" && (rows[0].Position != 5 || rows[0].Clicks != 3 || rows[0].CTR != 2) {
				t.Fatalf("metrics: %+v", rows[0])
			}
			if tc.n > 0 && (rows[0].Query != tc.query || rows[0].Impressions != tc.impressions) {
				t.Fatalf("%+v", rows)
			}
		})
	}
}
func TestTokenCovered(t *testing.T) {
	for _, tc := range []struct {
		query, text string
		want        bool
	}{
		{"yoga retreat", "Retreat Yoga Bali", true}, {"yoga mats", "Yoga retreats", false}, {"yoga retreat", " YOGA  \n retreat ", true}, {"", "Yoga", false}, {"yoga", "", false}, {"mat", "mats", false},
	} {
		t.Run(tc.query+"/"+tc.text, func(t *testing.T) {
			if got := tokenCovered(tc.query, tc.text); got != tc.want {
				t.Fatalf("got %v", got)
			}
		})
	}
}
