// Hand-authored tests for consensusNote, the single home for the consensus
// command's advisory note. Not generated.
package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
)

func TestConsensusNote(t *testing.T) {
	cases := []struct {
		name       string
		studyCount int
		retracted  int
		dropped    int
		verdict    scengine.Verdict
		want       string
	}{
		{
			name: "empty score, all relevant works retracted", studyCount: 0, retracted: 3, dropped: 0,
			verdict: scengine.VerdictInsufficient,
			want:    "all 3 relevant work(s) were excluded as retracted; try a broader claim, different sources, or review the retraction labels",
		},
		{
			name: "empty score, both gates excluded works", studyCount: 0, retracted: 3, dropped: 5,
			verdict: scengine.VerdictInsufficient,
			want:    "no scorable works remained; 3 relevant work(s) were excluded as retracted and 5 off-topic work(s) were excluded by the relevance gate",
		},
		{
			name: "empty score, relevance gate only", studyCount: 0, retracted: 0, dropped: 5,
			verdict: scengine.VerdictInsufficient,
			want:    "no relevant works remained; 5 fetched work(s) were excluded by the relevance gate",
		},
		{
			name: "empty score, nothing fetched", studyCount: 0, retracted: 0, dropped: 0,
			verdict: scengine.VerdictInsufficient,
			want:    "no works found; try a broader claim or --data-source live",
		},
		{
			name: "non-empty result with retractions", studyCount: 5, retracted: 2, dropped: 0,
			verdict: scengine.VerdictSupports,
			want:    "2 retracted work(s) excluded from the score",
		},
		{
			name: "non-empty result, all three fragments", studyCount: 5, retracted: 2, dropped: 3,
			verdict: scengine.VerdictInsufficient,
			want:    "fewer than 3 directional studies; treat as preliminary; 3 off-topic work(s) excluded by relevance gate; 2 retracted work(s) excluded from the score",
		},
		{
			name: "non-empty result, no note", studyCount: 5, retracted: 0, dropped: 0,
			verdict: scengine.VerdictSupports,
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := consensusNote(tc.studyCount, tc.retracted, tc.dropped, tc.verdict)
			if got != tc.want {
				t.Fatalf("consensusNote(%d, %d, %d, %q) =\n  %q\nwant\n  %q",
					tc.studyCount, tc.retracted, tc.dropped, tc.verdict, got, tc.want)
			}
		})
	}
}

// A study count of zero with retractions must never claim no works were found:
// a retracted work passed the relevance gate first, so works were found.
func TestConsensusNoteEmptyScoreWithRetractionsDoesNotSayNoWorksFound(t *testing.T) {
	got := consensusNote(0, 5, 0, scengine.VerdictInsufficient)
	if strings.Contains(got, "no works found") {
		t.Fatalf("note contradicts itself: %q", got)
	}
}

// The joined form must use exactly one separator between adjacent fragments —
// the drift the old two-stage assembly kept re-introducing.
func TestConsensusNoteFragmentSeparators(t *testing.T) {
	got := consensusNote(5, 2, 3, scengine.VerdictInsufficient)
	if n := strings.Count(got, "; "); n != 3 {
		t.Fatalf("want 3 %q separators (one inside the preliminary fragment, two joins), got %d in %q", "; ", n, got)
	}
	if strings.Contains(got, ";;") || strings.Contains(got, "; ;") {
		t.Fatalf("doubled separator in %q", got)
	}
}
