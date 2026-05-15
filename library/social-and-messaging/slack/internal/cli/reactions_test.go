// Copyright 2026 erick-holm. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/slack/internal/store"
)

func TestEmojiClass(t *testing.T) {
	cases := map[string]string{
		"tada":             "celebrate",
		"+1":               "approve",
		"white_check_mark": "approve",
		"sob":              "sad",
		"fire":             "hot",
		"rocket":           "hot",
		"eyes":             "other",
		"":                 "other",
	}
	for emoji, want := range cases {
		if got := emojiClass(emoji); got != want {
			t.Errorf("emojiClass(%q) = %q, want %q", emoji, got, want)
		}
	}
}

func TestSummarizeReactions(t *testing.T) {
	msgs := []store.Message{
		{ChannelID: "C1", TS: "00000100.000000", UserID: "U1", Text: "first", Permalink: "p1"},
		{ChannelID: "C1", TS: "00000200.000000", UserID: "U2", Text: "second", Permalink: "p2"},
		{ChannelID: "C1", TS: "00000300.000000", UserID: "U3", Text: "out of window", Permalink: "p3"},
	}
	reactions := []store.Reaction{
		{MessageTS: "00000100.000000", EmojiName: "tada", Count: 2},
		{MessageTS: "00000100.000000", EmojiName: "+1", Count: 1},
		{MessageTS: "00000200.000000", EmojiName: "fire", Count: 5},
		{MessageTS: "00000300.000000", EmojiName: "+1", Count: 9}, // excluded by window
	}
	// Window covers ts 100..250, so the 3rd message is excluded.
	summary := summarizeReactions(msgs, reactions, "00000050.000000", "00000250.000000")

	if summary.TotalReactions != 8 {
		t.Errorf("TotalReactions = %d, want 8 (2+1+5)", summary.TotalReactions)
	}
	if len(summary.TopMessages) != 2 {
		t.Fatalf("expected 2 top messages, got %d", len(summary.TopMessages))
	}
	// Message 2 has 5 reactions, message 1 has 3 — message 2 ranks first.
	if summary.TopMessages[0].TS != "00000200.000000" {
		t.Errorf("top message = %q, want ts 00000200 (5 reactions)", summary.TopMessages[0].TS)
	}
	if summary.TopMessages[0].ReactionCount != 5 {
		t.Errorf("top message count = %d, want 5", summary.TopMessages[0].ReactionCount)
	}
	if summary.TopMessages[1].ReactionCount != 3 {
		t.Errorf("second message count = %d, want 3 (2+1)", summary.TopMessages[1].ReactionCount)
	}
	if summary.ClassCounts["celebrate"] != 2 {
		t.Errorf("celebrate class count = %d, want 2", summary.ClassCounts["celebrate"])
	}
	if summary.ClassCounts["approve"] != 1 {
		t.Errorf("approve class count = %d, want 1", summary.ClassCounts["approve"])
	}
	if summary.ClassCounts["hot"] != 5 {
		t.Errorf("hot class count = %d, want 5", summary.ClassCounts["hot"])
	}
	if summary.EmojiDistribution["tada"] != 2 {
		t.Errorf("tada distribution = %d, want 2", summary.EmojiDistribution["tada"])
	}
}
