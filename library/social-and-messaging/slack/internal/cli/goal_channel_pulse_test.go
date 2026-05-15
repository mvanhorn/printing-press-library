// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

func TestParseRocksYAML(t *testing.T) {
	doc := `
rocks:
  - rock: "Ship CSM v2"
    slack_channel: "#csm-signals"   # mapped channel
  - rock: Attio migration
    slack_channel: "#attio-migration"
  - name: Voice AI GA
    channel: voice-agents
`
	got := parseRocksYAML(doc)
	if len(got) != 3 {
		t.Fatalf("expected 3 mappings, got %d: %+v", len(got), got)
	}
	if got[0].Rock != "Ship CSM v2" || got[0].SlackChannel != "#csm-signals" {
		t.Errorf("mapping 0 = %+v, want {Ship CSM v2, #csm-signals}", got[0])
	}
	if got[1].Rock != "Attio migration" || got[1].SlackChannel != "#attio-migration" {
		t.Errorf("mapping 1 = %+v", got[1])
	}
	// 'name'/'channel' aliases are accepted.
	if got[2].Rock != "Voice AI GA" || got[2].SlackChannel != "voice-agents" {
		t.Errorf("mapping 2 = %+v, want {Voice AI GA, voice-agents}", got[2])
	}
}

func TestParseRocksYAML_Empty(t *testing.T) {
	if got := parseRocksYAML(""); len(got) != 0 {
		t.Errorf("empty doc should yield no mappings, got %+v", got)
	}
	if got := parseRocksYAML("rocks:\n"); len(got) != 0 {
		t.Errorf("rocks-only doc should yield no mappings, got %+v", got)
	}
}

func TestComputeChannelPulse(t *testing.T) {
	msgs := []store.Message{
		{TS: "00000100.000000", UserID: "U1"},
		{TS: "00000200.000000", UserID: "U2"},
		{TS: "00000300.000000", UserID: "U1"}, // repeat participant
		{TS: "00009999.000000", UserID: "U9"}, // out of window
	}
	reactions := []store.Reaction{
		{MessageTS: "00000100.000000", Count: 3},
		{MessageTS: "00000200.000000", Count: 1},
		{MessageTS: "00009999.000000", Count: 7}, // out of window
	}
	count, participants, reactionTotal := computeChannelPulse(
		msgs, reactions, "00000050.000000", "00000350.000000")
	if count != 3 {
		t.Errorf("message count = %d, want 3", count)
	}
	if participants != 2 {
		t.Errorf("unique participants = %d, want 2 (U1, U2)", participants)
	}
	if reactionTotal != 4 {
		t.Errorf("total reactions = %d, want 4 (3+1)", reactionTotal)
	}
}
