// Copyright 2026 Rob Zehner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestResolveMatchRankPrefersExactTitleAndRequestedArtist(t *testing.T) {
	tests := []struct {
		name        string
		candName    string
		candArtists []string
		wantTitle   string
		wantArtist  string
		want        int
	}{
		{"exact title, right artist", "4D", []string{"Northlane"}, "4D", "Northlane", 0},
		{"decorated title collapses to normalized", "4D - Live", []string{"Northlane"}, "4D", "Northlane", 1},
		{"remaster suffix collapses to normalized", "Quantum Flux - Remastered 2011", []string{"Northlane"}, "Quantum Flux", "Northlane", 1},
		{"prefix match", "Bloodline Reprise", []string{"Northlane"}, "Bloodline", "Northlane", 2},
		{"substring match", "The Bloodline Sessions", []string{"Northlane"}, "Bloodline", "Northlane", 3},
		{"exact title, wrong artist is penalised not rejected", "4D", []string{"Cover Band"}, "4D", "Northlane", 100},
		{"unrelated title is rejected", "Talking Heads", []string{"Northlane"}, "4D", "Northlane", -1},
		{"no artist constraint scores on title alone", "4D", []string{"Anyone"}, "4D", "", 0},
		{"artist match is case- and substring-insensitive", "4D", []string{"NORTHLANE feat. X"}, "4D", "northlane", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveMatchRank(tc.candName, tc.candArtists, tc.wantTitle, tc.wantArtist)
			if got != tc.want {
				t.Fatalf("resolveMatchRank(%q, %v, %q, %q) = %d, want %d",
					tc.candName, tc.candArtists, tc.wantTitle, tc.wantArtist, got, tc.want)
			}
		})
	}
}

// The whole reason this command exists: Spotify's relevance order is not
// title-exactness order, so the right-artist studio cut must win even when the
// API returns a cover or a live version first.
func TestResolveMatchRankBeatsSpotifyRelevanceOrder(t *testing.T) {
	// Order as the API returned it: cover first, live second, studio last.
	candidates := []struct {
		name    string
		artists []string
	}{
		{"4D", []string{"Tribute Players"}},
		{"4D - Live at Wembley", []string{"Northlane"}},
		{"4D", []string{"Northlane"}},
	}
	best, bestRank := -1, -1
	for i, c := range candidates {
		rank := resolveMatchRank(c.name, c.artists, "4D", "Northlane")
		if rank < 0 {
			continue
		}
		if bestRank >= 0 && rank >= bestRank {
			continue
		}
		best, bestRank = i, rank
	}
	if best != 2 {
		t.Fatalf("picked candidate %d (%q by %v), want the studio cut at index 2",
			best, candidates[best].name, candidates[best].artists)
	}
}

func TestNormalizeTrackTitleStripsDecorations(t *testing.T) {
	tests := map[string]string{
		"4D":                             "4d",
		"4D (Live)":                      "4d",
		"Quantum Flux - Remastered 2011": "quantumflux",
		"Bloodline feat. Someone":        "bloodline",
		"Mesmer [Deluxe]":                "mesmer",
		"Rot ft. Guest":                  "rot",
	}
	for in, want := range tests {
		if got := normalizeTrackTitle(in); got != want {
			t.Errorf("normalizeTrackTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSearchQueryForOmitsEmptyArtist(t *testing.T) {
	if got := searchQueryFor("", "4D"); got != "track:4D" {
		t.Errorf("searchQueryFor(\"\", \"4D\") = %q", got)
	}
	if got := searchQueryFor("Northlane", "4D"); got != "track:4D artist:Northlane" {
		t.Errorf("searchQueryFor(\"Northlane\", \"4D\") = %q", got)
	}
}

// filterFields must never hand back the full payload for a --select that
// matched nothing — that silent passthrough is what made a typo'd path look
// like a successful (and enormous) response.
func TestFilterFieldsReportsTotalMissInsteadOfFullPayload(t *testing.T) {
	payload := json.RawMessage(`{"tracks":{"items":[{"name":"4D","uri":"spotify:track:1"}],"total":1}}`)

	recordSelectNoMatch("")
	got := filterFields(payload, "results.tracks.items.name")
	if got := selectNoMatchSpec(); got != "results.tracks.items.name" {
		t.Fatalf("selectNoMatchSpec() = %q, want the missed spec recorded for exit-code classification", got)
	}
	var diag struct {
		Error           string   `json:"error"`
		Select          string   `json:"select"`
		AvailableFields []string `json:"available_fields"`
	}
	if err := json.Unmarshal(got, &diag); err != nil {
		t.Fatalf("unmarshal diagnostic: %v (got %s)", err, got)
	}
	if diag.Error == "" {
		t.Fatalf("expected an error diagnostic, got %s", got)
	}
	if len(diag.AvailableFields) == 0 {
		t.Fatalf("diagnostic must name the fields that were available, got %s", got)
	}
	if diag.Select != "results.tracks.items.name" {
		t.Fatalf("diagnostic must echo the offending spec, got %q", diag.Select)
	}
}

func TestFilterFieldsKeepsWorkingSelectAndLeavesNoMissRecorded(t *testing.T) {
	payload := json.RawMessage(`{"tracks":{"items":[{"name":"4D","uri":"spotify:track:1"}],"total":1}}`)

	recordSelectNoMatch("")
	got := filterFields(payload, "tracks.items.name")
	if got := selectNoMatchSpec(); got != "" {
		t.Fatalf("a matching --select must not record a miss, got %q", got)
	}
	var out map[string]any
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := out["tracks"]; !ok {
		t.Fatalf("expected the projected tracks subtree, got %s", got)
	}
}

// An intermediate segment that matches but whose leaf does not is still a miss:
// `--select tracks.bogus` must not read as success.
func TestFilterFieldsTreatsUnmatchedLeafAsMiss(t *testing.T) {
	payload := json.RawMessage(`{"tracks":{"items":[{"name":"4D"}],"total":1}}`)

	recordSelectNoMatch("")
	if got := filterFields(payload, "tracks.bogus"); selectNoMatchSpec() == "" {
		t.Fatalf("expected a recorded miss for an unmatched leaf, got %s", got)
	}
}
