// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"strings"
	"testing"
)

// byAthleteFixture mirrors the common/v3 statistics/byathlete payload shape:
// top-level categories carry index-aligned `names`/`displayNames`, and each
// athlete's categories carry a `values` array aligned to those names.
const byAthleteFixture = `{
  "requestedSeason": {"name": "2024", "year": 2024},
  "categories": [
    {"name": "batting", "names": ["gamesPlayed", "homeRuns", "avg"], "displayNames": ["Games Played", "Home Runs", "Batting Average"]},
    {"name": "pitching", "names": ["ERA", "strikeouts"], "displayNames": ["Earned Run Average", "Strikeouts"]}
  ],
  "athletes": [
    {"athlete": {"displayName": "Aaron Judge", "team": {"abbreviation": "NYY"}, "position": {"abbreviation": "RF"}},
     "categories": [{"name": "batting", "values": [158, 58, 0.322]}]},
    {"athlete": {"displayName": "Shohei Ohtani", "team": {"abbreviation": "LAD"}, "position": {"abbreviation": "DH"}},
     "categories": [{"name": "batting", "values": [159, 54, 0.310]}]}
  ]
}`

func TestResolveStatSortKey(t *testing.T) {
	cases := map[string]string{
		"homeRuns":   "batting.homeRuns",
		"Home Runs":  "batting.homeRuns",
		"avg":        "batting.avg",
		"ERA":        "pitching.ERA",
		"strikeouts": "pitching.strikeouts",
	}
	for in, want := range cases {
		got, ok := resolveStatSortKey([]byte(byAthleteFixture), in)
		if !ok {
			t.Errorf("resolveStatSortKey(%q): not found", in)
			continue
		}
		if got != want {
			t.Errorf("resolveStatSortKey(%q) = %q, want %q", in, got, want)
		}
	}
	if _, ok := resolveStatSortKey([]byte(byAthleteFixture), "nonsense"); ok {
		t.Errorf("resolveStatSortKey(nonsense) should not resolve")
	}
}

func TestRenderStatLeaders(t *testing.T) {
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(byAthleteFixture), "homeRuns", 0); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"Home Runs leaders — 2024", "Aaron Judge", "NYY", "RF", "58", "Shohei Ohtani", "54"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
	// Whole-count HR should render without a trailing ".0".
	if strings.Contains(out, "58.0") {
		t.Errorf("home-run count should render as 58, not 58.0\n%s", out)
	}
}

func TestRenderStatLeadersRateStat(t *testing.T) {
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(byAthleteFixture), "avg", 0); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "0.322") {
		t.Errorf("rate stat should render as 0.322\n%s", out)
	}
}

// The byathlete endpoint returns athletes in a default composite order, not
// sorted by the requested stat. renderStatLeaders must rank client-side.
func TestRenderStatLeadersRanksClientSide(t *testing.T) {
	// Athletes deliberately out of HR order in the payload.
	payload := `{
      "requestedSeason": {"name": "2024"},
      "categories": [{"name": "batting", "names": ["homeRuns"], "displayNames": ["Home Runs"]}],
      "athletes": [
        {"athlete": {"displayName": "Mid Guy", "team": {"abbreviation": "AAA"}, "position": {"abbreviation": "1B"}}, "categories": [{"name": "batting", "values": [38]}]},
        {"athlete": {"displayName": "Top Guy", "team": {"abbreviation": "BBB"}, "position": {"abbreviation": "RF"}}, "categories": [{"name": "batting", "values": [58]}]},
        {"athlete": {"displayName": "Second Guy", "team": {"abbreviation": "CCC"}, "position": {"abbreviation": "DH"}}, "categories": [{"name": "batting", "values": [41]}]}
      ]
    }`
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(payload), "homeRuns", 0); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	out := buf.String()
	iTop := strings.Index(out, "Top Guy")
	iSecond := strings.Index(out, "Second Guy")
	iMid := strings.Index(out, "Mid Guy")
	if !(iTop < iSecond && iSecond < iMid) {
		t.Errorf("expected descending HR order Top(58) < Second(41) < Mid(38) by position; got order Top=%d Second=%d Mid=%d\n%s", iTop, iSecond, iMid, out)
	}
	// Rank 1 must be the 58-HR hitter.
	if !strings.Contains(out, "1\tTop Guy") {
		t.Errorf("rank 1 should be Top Guy (58 HR)\n%s", out)
	}
}

func TestRenderStatLeadersLimit(t *testing.T) {
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(byAthleteFixture), "homeRuns", 1); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Aaron Judge") {
		t.Errorf("rank-1 leader should be present with limit=1\n%s", out)
	}
	if strings.Contains(out, "Shohei Ohtani") {
		t.Errorf("limit=1 should drop rank-2 leader\n%s", out)
	}
}

func TestRenderStatLeadersUnknownStat(t *testing.T) {
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(byAthleteFixture), "nonsense", 0); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	if !strings.Contains(buf.String(), "not found") {
		t.Errorf("unknown stat should report not-found, got: %q", buf.String())
	}
}

func TestRenderStatLeadersEmpty(t *testing.T) {
	payload := `{"requestedSeason":{"name":"2026"},"categories":[{"name":"batting","names":["homeRuns"],"displayNames":["Home Runs"]}],"athletes":[]}`
	var buf bytes.Buffer
	if err := renderStatLeaders(&buf, []byte(payload), "homeRuns", 5); err != nil {
		t.Fatalf("renderStatLeaders: %v", err)
	}
	if !strings.Contains(buf.String(), "No leaders found") {
		t.Errorf("empty athletes should render a no-leaders message, got: %q", buf.String())
	}
}

func TestListAvailableStats(t *testing.T) {
	var buf bytes.Buffer
	if err := listAvailableStats(&buf, []byte(byAthleteFixture)); err != nil {
		t.Fatalf("listAvailableStats: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"batting", "homeRuns", "Home Runs", "pitching", "ERA"} {
		if !strings.Contains(out, want) {
			t.Errorf("stat listing missing %q\n---\n%s", want, out)
		}
	}
}

func TestSeasonTypeCode(t *testing.T) {
	cases := map[string]int{
		"":          2,
		"regular":   2,
		"reg":       2,
		"pre":       1,
		"preseason": 1,
		"playoffs":  3,
		"post":      3,
		"garbage":   2,
	}
	for in, want := range cases {
		if got := seasonTypeCode(in); got != want {
			t.Errorf("seasonTypeCode(%q) = %d, want %d", in, got, want)
		}
	}
}
