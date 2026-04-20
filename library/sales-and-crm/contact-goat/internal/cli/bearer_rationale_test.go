// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
)

// TestBearerRationale_DecisionTable pins down the exact rationale strings
// every bearer-surface command should emit, keyed off the shape of the
// bridge slice. Fixing these strings here keeps coverage, prospect,
// hp-people, warm-intro, and api-hpn-search in lockstep — if a renderer
// drifts, this test catches it before a user sees a rationale mismatch.
func TestBearerRationale_DecisionTable(t *testing.T) {
	cases := []struct {
		name     string
		bridges  []client.Bridge
		want     string
	}{
		{
			name:    "friend bridge with positive affinity",
			bridges: []client.Bridge{{Name: "Jeff Clavier", Kind: client.BridgeKindFriend, AffinityScore: 104.38}},
			want:    "via Jeff Clavier (affinity 104.4)",
		},
		{
			name: "multiple friend bridges picks highest affinity",
			bridges: []client.Bridge{
				{Name: "Weak", Kind: client.BridgeKindFriend, AffinityScore: 3.0},
				{Name: "Strong", Kind: client.BridgeKindFriend, AffinityScore: 50.0},
				{Name: "Mid", Kind: client.BridgeKindFriend, AffinityScore: 10.0},
			},
			want: "via Strong (affinity 50.0)",
		},
		{
			name:    "friend bridge with zero affinity is weak-signal",
			bridges: []client.Bridge{{Name: "Garry Tan", Kind: client.BridgeKindFriend, AffinityScore: 0}},
			want:    "Happenstance bearer (weak signal, no graph affinity)",
		},
		{
			name:    "only self-graph bridge renders as synced graph",
			bridges: []client.Bridge{{Name: "Matt", Kind: client.BridgeKindSelfGraph, AffinityScore: 0}},
			want:    "in your synced graph",
		},
		{
			name: "friend bridge with real affinity wins over self-graph",
			bridges: []client.Bridge{
				{Name: "Matt", Kind: client.BridgeKindSelfGraph, AffinityScore: 5},
				{Name: "Jeff", Kind: client.BridgeKindFriend, AffinityScore: 10},
			},
			want: "via Jeff (affinity 10.0)",
		},
		{
			name:    "no bridges at all",
			bridges: nil,
			want:    "Happenstance bearer (no graph match)",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bearerRationale(tc.bridges)
			if got != tc.want {
				t.Errorf("bearerRationale(%+v) = %q, want %q", tc.bridges, got, tc.want)
			}
		})
	}
}

// TestBearerScore_AffinityWinsWhenPositive verifies the score-selection
// policy: max friend-bridge affinity trumps traits score when positive,
// otherwise traits score is the fallback.
func TestBearerScore_AffinityWinsWhenPositive(t *testing.T) {
	cases := []struct {
		name    string
		bridges []client.Bridge
		traits  float64
		want    float64
	}{
		{"positive affinity beats traits", []client.Bridge{{Kind: client.BridgeKindFriend, AffinityScore: 50}}, 0.9, 50},
		{"zero affinity falls back to traits", []client.Bridge{{Kind: client.BridgeKindFriend, AffinityScore: 0}}, 0.75, 0.75},
		{"no friend bridges falls back to traits", []client.Bridge{{Kind: client.BridgeKindSelfGraph, AffinityScore: 100}}, 0.42, 0.42},
		{"no bridges at all falls back to traits", nil, 0.33, 0.33},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := bearerScore(tc.bridges, tc.traits)
			if got != tc.want {
				t.Errorf("bearerScore = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestBridgesToFlagship_RoundTrip verifies the canonical->render
// projection preserves every field and returns nil (not an empty slice)
// when the input is empty so JSON output omits the field.
func TestBridgesToFlagship_RoundTrip(t *testing.T) {
	in := []client.Bridge{
		{Name: "Jeff", HappenstanceUUID: "u1", AffinityScore: 104.4, Kind: client.BridgeKindFriend},
		{Name: "Self", HappenstanceUUID: "u2", AffinityScore: 0, Kind: client.BridgeKindSelfGraph},
	}
	got := bridgesToFlagship(in)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "Jeff" || got[0].Kind != "friend" || got[0].AffinityScore != 104.4 || got[0].HappenstanceUUID != "u1" {
		t.Errorf("[0] = %+v", got[0])
	}
	if got[1].Kind != "self_graph" {
		t.Errorf("[1].Kind = %q, want self_graph", got[1].Kind)
	}
	if bridgesToFlagship(nil) != nil {
		t.Errorf("empty input should return nil, not empty slice")
	}
}

// TestMergePeople_BridgesCarriedAcrossDedup covers the integration case
// where a cookie-path row and a bearer-path row dedupe to the same
// Person (matched by LinkedIn URL). The bearer row's bridges are
// additive graph context and must survive the merge onto the cookie
// row. Without this the user sees a "1st-degree" cookie tag but loses
// the affinity-weighted bridge detail the bearer API gave us.
func TestMergePeople_BridgesCarriedAcrossDedup(t *testing.T) {
	cookieRow := flagshipPerson{
		Name:        "Ira Ehrenpreis",
		LinkedInURL: "https://www.linkedin.com/in/iraehrenpreis",
		Sources:     []string{"hp_graph_2deg"},
	}
	bearerRow := flagshipPerson{
		Name:        "Ira Ehrenpreis",
		LinkedInURL: "https://linkedin.com/in/iraehrenpreis/",
		Sources:     []string{"hp_api"},
		Bridges: []bridgeRef{
			{Name: "Jeff Clavier", AffinityScore: 104.4, Kind: "friend"},
		},
	}
	merged := mergePeople([]flagshipPerson{cookieRow, bearerRow})
	if len(merged) != 1 {
		t.Fatalf("merged rows = %d, want 1 (should dedupe by canonical LinkedIn URL)", len(merged))
	}
	row := merged[0]
	if len(row.Bridges) != 1 || row.Bridges[0].Name != "Jeff Clavier" {
		t.Errorf("merged bridges = %+v, want one Jeff Clavier entry", row.Bridges)
	}
	// Cookie source should appear first (kept as primary); the bearer
	// source is appended but the merge retains both for provenance.
	if len(row.Sources) != 2 {
		t.Errorf("merged sources = %v, want both cookie + api tags", row.Sources)
	}
}
