// Hand-authored tests for the consensus command's output surface: the JSON
// shape of consensusOutput and its human-readable rendering. No network.
// Not generated.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
)

// consensusOutputFor runs the same pipeline the consensus command runs after
// fetching (score -> scengine.Consensus -> copy into consensusOutput), so the
// near-unanimous value under test is the engine's, not a hand-set literal.
func consensusOutputFor(t *testing.T, claim string, titles []string) consensusOutput {
	t.Helper()
	works := make([]scWork, len(titles))
	for i, ti := range titles {
		works[i] = scWork{ID: "W" + string(rune('1'+i)), Title: ti, Year: 2021, CitedBy: 40 + i, Type: "article"}
	}
	scored, stances := scoreWorks(context.Background(), works, claim)
	result := scengine.Consensus(scored)
	return consensusOutput{
		Claim: claim, Verdict: result.Verdict, ConsensusScore: result.ConsensusScore,
		Confidence: result.Confidence, EvidenceStrength: result.EvidenceStrength,
		NearUnanimous: result.NearUnanimous, ApexDesign: result.ApexDesign,
		StudyCount: result.StudyCount, Supporting: result.Supporting,
		Refuting: result.Refuting, Mixed: result.Mixed, Inconclusive: result.Inconclusive,
		TotalCitations: result.TotalCitations, Method: stanceMethodLabel(stances),
	}
}

// TestConsensusOutputNearUnanimous pins that the near-unanimous flag reaches
// both output surfaces when the corpus has zero dissent, and that it is
// omitted from JSON (and silent in the rendered text) otherwise: omitempty
// keeps the common case byte-identical to the pre-existing JSON.
func TestConsensusOutputNearUnanimous(t *testing.T) {
	tests := []struct {
		name   string
		claim  string
		titles []string
		want   bool
	}{
		{
			name:   "all-supporting corpus is flagged near-unanimous",
			claim:  "statins reduce mortality",
			titles: unanimousTitles,
			want:   true,
		},
		{
			name:   "mixed stances leave the flag off",
			claim:  "statins reduce mortality",
			titles: contestedTitles,
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := consensusOutputFor(t, tt.claim, tt.titles)
			if out.NearUnanimous != tt.want {
				t.Fatalf("NearUnanimous = %v, want %v (score %.2f, support %d, refute %d, mixed %d)",
					out.NearUnanimous, tt.want, out.ConsensusScore, out.Supporting, out.Refuting, out.Mixed)
			}

			raw, err := json.Marshal(out)
			if err != nil {
				t.Fatalf("json.Marshal: %v", err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("json.Unmarshal: %v", err)
			}
			got, present := decoded["near_unanimous"]
			if tt.want {
				if !present || got != true {
					t.Errorf("JSON near_unanimous = %v (present=%v), want true", got, present)
				}
			} else if present {
				t.Errorf("JSON contains near_unanimous = %v; omitempty must drop the false case", got)
			}

			var buf bytes.Buffer
			renderConsensus(&buf, out)
			hasLine := strings.Contains(buf.String(), "Near-unanimous:")
			if hasLine != tt.want {
				t.Errorf("rendered Near-unanimous line present = %v, want %v\nrendered:\n%s",
					hasLine, tt.want, buf.String())
			}
		})
	}
}
