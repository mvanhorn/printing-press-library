// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package youtube

import (
	"strings"
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
	// The contract is the reconstructed text: every spoken word exactly once,
	// in order. The pre-fix behavior emitted the "It just so happens that
	// everything that" run twice. Cue boundaries may merge — this is one
	// continuous rolling window.
	joined := joinSegs(got)
	want := "reinforcement learning is terrible. It just so happens that everything that we tried"
	if joined != want {
		t.Errorf("joined output:\n  got:  %q\n  want: %q", joined, want)
	}
}

func TestCollapseRollingWindow_ShiftedWindowShortAdvance(t *testing.T) {
	// Regression for the review repro: when a cue advances FEWER words than
	// it repeats, the overlap must still be computed against the previous
	// cue's ORIGINAL text, not against the trimmed remainder — otherwise the
	// repeated run sneaks back in.
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "a b c d e"},
		{TsSec: 1, Speaker: "A", Text: "c d e f"},
		{TsSec: 2, Speaker: "A", Text: "d e f g"},
	}
	got := collapseRollingWindow(in)
	joined := joinSegs(got)
	if joined != "a b c d e f g" {
		t.Errorf("joined output: got %q, want %q", joined, "a b c d e f g")
	}
}

// joinSegs reconstructs the transcript text from collapsed segments.
func joinSegs(segs []transcript.Segment) string {
	var parts []string
	for _, s := range segs {
		parts = append(parts, s.Text)
	}
	return strings.Join(parts, " ")
}

func TestCollapseRollingWindow_SlidingSuffixPrefixOverlap(t *testing.T) {
	// The live-stream shape that motivated the fix: each cue's first half
	// repeats the previous cue's second half. Observed on real auto-subs where
	// a 43-minute episode rendered ~2x its spoken word count. None of these
	// pairs are exact-prefix relations, so the historical collapse kept every
	// duplicate run.
	in := []transcript.Segment{
		{TsSec: 3, Speaker: "A", Text: "if you have both conditions then you're more likely to experience anxiety and"},
		{TsSec: 6, Speaker: "A", Text: "more likely to experience anxiety and depression than just having one of"},
		{TsSec: 7, Speaker: "A", Text: "depression than just having one of those. If you're going to a social event"},
	}
	got := collapseRollingWindow(in)
	if len(got) == 0 {
		t.Fatal("expected collapsed segments, got none")
	}
	// Segmentation may merge trimmed remainders into the following cue; the
	// contract is the reconstructed text, not the cue boundaries.
	var parts []string
	for _, s := range got {
		parts = append(parts, s.Text)
	}
	joined := strings.Join(parts, " ")
	want := "if you have both conditions then you're more likely to experience anxiety and depression than just having one of those. If you're going to a social event"
	if joined != want {
		t.Errorf("joined output:\n  got:  %q\n  want: %q", joined, want)
	}
	// No word run may appear twice: the overlap must have been trimmed.
	if strings.Count(joined, "more likely to experience anxiety and") != 1 {
		t.Errorf("overlap run duplicated in output: %q", joined)
	}
}

func TestCollapseRollingWindow_NaturalRepetitionPreserved(t *testing.T) {
	// Short shared boundaries (< minOverlapWords) are real speech, not
	// rolling-window artifacts — they must survive untouched.
	in := []transcript.Segment{
		{TsSec: 0, Speaker: "A", Text: "we shipped it in the end"},
		{TsSec: 2, Speaker: "A", Text: "the end of the quarter was rough"},
	}
	got := collapseRollingWindow(in)
	if len(got) != 2 {
		t.Fatalf("expected 2 segments, got %d: %+v", len(got), got)
	}
	if got[1].Text != "the end of the quarter was rough" {
		t.Errorf("2-word boundary was wrongly trimmed: %q", got[1].Text)
	}
}

func TestMatch_AcceptsYtSearchPseudoURLs(t *testing.T) {
	a := New()
	for _, u := range []string{"ytsearch1:some episode title", "ytsearch:another query", "YTSEARCH3:mixed case"} {
		if !a.Match(u) {
			t.Errorf("Match(%q) = false, want true", u)
		}
	}
	if a.Match("spotify:episode:abc") {
		t.Errorf("Match should reject non-YouTube, non-ytsearch inputs")
	}
}
