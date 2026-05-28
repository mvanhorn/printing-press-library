// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package youtube

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

func TestCollapseRollingWindow_ExactDups(t *testing.T) {
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "hello world"},
		{TsSec: 1, Speaker: "A", Text: "hello world"},
		{TsSec: 2, Speaker: "A", Text: "hello world"},
	}
	got := collapseRollingWindow(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d: %+v", len(got), got)
	}
	if got[0].Text != "hello world" {
		t.Errorf("got text %q", got[0].Text)
	}
}

func TestCollapseRollingWindow_PrefixExtension(t *testing.T) {
	// Classic YouTube rolling-window: each cue adds words. Expect to collapse
	// to the LAST (longest) form, taking its later timestamp.
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "reinforcement learning is"},
		{TsSec: 1, Speaker: "A", Text: "reinforcement learning is terrible"},
		{TsSec: 2, Speaker: "A", Text: "reinforcement learning is terrible. It just"},
	}
	got := collapseRollingWindow(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 collapsed segment, got %d: %+v", len(got), got)
	}
	if got[0].Text != "reinforcement learning is terrible. It just" {
		t.Errorf("got %q", got[0].Text)
	}
	if got[0].TsSec != 2 {
		t.Errorf("expected ts 2, got %d", got[0].TsSec)
	}
}

func TestCollapseRollingWindow_BackwardBuildup(t *testing.T) {
	// Sometimes a shorter version appears after a longer one (caption stream
	// re-syncs to a new sentence). Drop the shorter when prev contains it.
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "the quick brown fox jumps over the lazy dog"},
		{TsSec: 1, Speaker: "A", Text: "the quick brown fox"},
	}
	got := collapseRollingWindow(in)
	if len(got) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(got))
	}
	if got[0].Text != "the quick brown fox jumps over the lazy dog" {
		t.Errorf("kept the shorter; got %q", got[0].Text)
	}
}

func TestCollapseRollingWindow_DistinctSegmentsPreserved(t *testing.T) {
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "first sentence"},
		{TsSec: 1, Speaker: "A", Text: "second sentence about a totally different topic"},
		{TsSec: 2, Speaker: "A", Text: "third unrelated sentence"},
	}
	got := collapseRollingWindow(in)
	if len(got) != 3 {
		t.Fatalf("expected 3 segments preserved, got %d: %+v", len(got), got)
	}
}

func TestCollapseRollingWindow_EmptyInput(t *testing.T) {
	got := collapseRollingWindow(nil)
	if len(got) != 0 {
		t.Errorf("expected empty, got %+v", got)
	}
}

func TestCollapseRollingWindow_RealYouTubeStream(t *testing.T) {
	// Excerpt from the actual Karpathy YouTube auto-subs that surfaced the bug.
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "Dwarkesh Patel", Text: "reinforcement learning is terrible."},
		{TsSec: 2, Speaker: "Dwarkesh Patel", Text: "reinforcement learning is terrible. It just so happens that everything that"},
		{TsSec: 4, Speaker: "Dwarkesh Patel", Text: "It just so happens that everything that"},
		{TsSec: 4, Speaker: "Dwarkesh Patel", Text: "It just so happens that everything that we tried"},
	}
	got := collapseRollingWindow(in)
	// First segment "reinforcement..." gets extended by second cue.
	// Third+fourth are a re-sync (new prefix); fourth extends third.
	// Expected: 2 segments — first extended chain + second extended chain.
	if len(got) != 2 {
		t.Fatalf("expected 2 segments after rolling-window collapse, got %d: %+v", len(got), got)
	}
	if got[0].Text != "reinforcement learning is terrible. It just so happens that everything that" {
		t.Errorf("first segment text: %q", got[0].Text)
	}
	if got[1].Text != "It just so happens that everything that we tried" {
		t.Errorf("second segment text: %q", got[1].Text)
	}
}
