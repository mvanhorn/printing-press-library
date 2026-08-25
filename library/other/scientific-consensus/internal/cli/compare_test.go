// Hand-authored tests for the shared consensus computation behind the compare
// and batch commands. No network: a fake apiGetter serves canned OpenAlex
// works. Not generated.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// fakeWorksClient implements apiGetter and serves one canned /works envelope
// per search query, so a single client can back both sides of a compare.
type fakeWorksClient struct {
	byQuery map[string][]string // query -> work titles
}

func (f *fakeWorksClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	titles := f.byQuery[params["search"]]
	var sb strings.Builder
	sb.WriteString(`{"meta":{"count":`)
	fmt.Fprintf(&sb, "%d", len(titles))
	sb.WriteString(`},"results":[`)
	for i, ti := range titles {
		if i > 0 {
			sb.WriteByte(',')
		}
		fmt.Fprintf(&sb,
			`{"id":"https://openalex.org/W%d","display_name":%q,"publication_year":2021,`+
				`"cited_by_count":%d,"type":"article"}`, i+1, ti, 40+i)
	}
	sb.WriteString(`]}`)
	return json.RawMessage(sb.String()), nil
}

// unanimousTitles all classify as supporting under the heuristic, so the
// weighted score lands at +1.00 with zero refuting and zero mixed studies.
var unanimousTitles = []string{
	"Statins significantly reduced all-cause mortality in a randomized controlled trial",
	"Meta-analysis: statin therapy lowers mortality and improves survival",
	"Statin treatment reduced mortality and improved survival in a large cohort",
}

// contestedTitles mix supporting and refuting findings, so dissent survives
// into the scored corpus and the near-unanimous flag must stay off.
var contestedTitles = []string{
	"Statins significantly reduced all-cause mortality in a randomized controlled trial",
	"Statins showed no significant effect on mortality in a randomized trial",
	"Statin therapy increased the risk of death in a large cohort study",
}

// TestComputeConsensusPropagatesNearUnanimous pins that computeConsensus (the
// shared copy site used by BOTH compare and batch) carries
// scengine.ConsensusResult.NearUnanimous into consensusOutput instead of
// dropping it, and that the two sides of a compare can disagree on the flag.
func TestComputeConsensusPropagatesNearUnanimous(t *testing.T) {
	claimA := "statins reduce mortality"
	claimB := "statins increase mortality risk"
	c := &fakeWorksClient{byQuery: map[string][]string{
		claimA: unanimousTitles,
		claimB: contestedTitles,
	}}

	a, err := computeConsensus(context.Background(), c, claimA, 10, 0, false)
	if err != nil {
		t.Fatalf("computeConsensus(%q) error: %v", claimA, err)
	}
	if a.Refuting != 0 || a.Mixed != 0 {
		t.Fatalf("side A is not dissent-free: refuting=%d mixed=%d", a.Refuting, a.Mixed)
	}
	if !a.NearUnanimous {
		t.Errorf("side A NearUnanimous = false, want true (score %.2f, support %d)",
			a.ConsensusScore, a.Supporting)
	}

	b, err := computeConsensus(context.Background(), c, claimB, 10, 0, false)
	if err != nil {
		t.Fatalf("computeConsensus(%q) error: %v", claimB, err)
	}
	if b.Refuting == 0 && b.Mixed == 0 {
		t.Fatalf("side B has no dissent to test against: %+v", b)
	}
	if b.NearUnanimous {
		t.Errorf("side B NearUnanimous = true, want false (score %.2f, refuting %d, mixed %d)",
			b.ConsensusScore, b.Refuting, b.Mixed)
	}

	// The point of the flag is that it is per-claim: both sides of one compare
	// must be able to hold different values simultaneously.
	if a.NearUnanimous == b.NearUnanimous {
		t.Errorf("both compare sides reported NearUnanimous=%v; want them to differ", a.NearUnanimous)
	}
}
