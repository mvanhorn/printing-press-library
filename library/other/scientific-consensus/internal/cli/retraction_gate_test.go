// Hand-authored tests for the retraction gate on the live consensus path:
// normalization of OpenAlex is_retracted, the two detection signals, and an
// end-to-end proof that a retracted work reaches neither the score nor the
// apex design. No network: a fake apiGetter serves canned OpenAlex works.
// Not generated.
package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/scientific-consensus/internal/scengine"
)

// fakeRetractionClient implements apiGetter and records the select list it was
// asked for, so a test can assert that is_retracted was requested AND that the
// flag survives normalization into scWork.
type fakeRetractionClient struct {
	works      []retractionFixture
	lastSelect string
}

type retractionFixture struct {
	title       string
	isRetracted bool
	workType    string
	citedBy     int
}

func (f *fakeRetractionClient) Get(_ context.Context, _ string, params map[string]string) (json.RawMessage, error) {
	f.lastSelect = params["select"]
	results := make([]map[string]any, 0, len(f.works))
	for i, w := range f.works {
		wt := w.workType
		if wt == "" {
			wt = "article"
		}
		results = append(results, map[string]any{
			"id":               "https://openalex.org/W" + string(rune('1'+i)),
			"display_name":     w.title,
			"publication_year": 2021,
			"cited_by_count":   w.citedBy,
			"type":             wt,
			"is_retracted":     w.isRetracted,
		})
	}
	raw, err := json.Marshal(map[string]any{
		"meta":    map[string]any{"count": len(results)},
		"results": results,
	})
	return json.RawMessage(raw), err
}

// TestFetchWorksRequestsAndCarriesIsRetracted pins the drop-at-normalization
// bug this change exists to prevent: the field must be in the OpenAlex select
// list, and the parsed value must still be on scWork after normalization.
func TestFetchWorksRequestsAndCarriesIsRetracted(t *testing.T) {
	c := &fakeRetractionClient{works: []retractionFixture{
		{title: "A trial that was later withdrawn", isRetracted: true},
		{title: "An ordinary trial", isRetracted: false},
	}}
	works, _, err := fetchWorks(context.Background(), c, "claim", "", "", 10)
	if err != nil {
		t.Fatalf("fetchWorks error: %v", err)
	}
	if !strings.Contains(c.lastSelect, "is_retracted") {
		t.Fatalf("select list does not request is_retracted: %q", c.lastSelect)
	}
	if len(works) != 2 {
		t.Fatalf("fetchWorks returned %d works, want 2", len(works))
	}
	if !works[0].IsRetracted {
		t.Errorf("works[0].IsRetracted = false; the index flag was dropped during normalization")
	}
	if works[1].IsRetracted {
		t.Errorf("works[1].IsRetracted = true, want false")
	}
}

// TestFilterRetracted covers the three signal combinations the gate has to
// distinguish, and pins that the input slice is not mutated.
func TestFilterRetracted(t *testing.T) {
	tests := []struct {
		name         string
		work         scWork
		wantExcluded bool
		wantStatus   scengine.Retraction
	}{
		{
			name:         "title marker alone is declared",
			work:         scWork{Title: "RETRACTED: Vitamin C prevents the common cold", IsRetracted: false},
			wantExcluded: true,
			wantStatus:   scengine.RetractionDeclared,
		},
		{
			name:         "index flag alone is flagged",
			work:         scWork{Title: "Vitamin C and the common cold: a meta-analysis", IsRetracted: true},
			wantExcluded: true,
			wantStatus:   scengine.RetractionFlagged,
		},
		{
			name:         "neither signal is kept",
			work:         scWork{Title: "Vitamin C and the common cold: a meta-analysis", IsRetracted: false},
			wantExcluded: false,
			wantStatus:   scengine.NotRetracted,
		},
		{
			name:         "a paper about retraction is kept (no start-anchored marker)",
			work:         scWork{Title: "Retracted Science and the Retraction Index", IsRetracted: false},
			wantExcluded: false,
			wantStatus:   scengine.NotRetracted,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := []scWork{tt.work}
			kept, excluded := filterRetracted(in)
			gotExcluded := len(excluded) == 1
			if gotExcluded != tt.wantExcluded {
				t.Fatalf("excluded = %v (kept %d, excluded %d), want %v",
					gotExcluded, len(kept), len(excluded), tt.wantExcluded)
			}
			got := append(append([]scWork{}, kept...), excluded...)
			if got[0].Retraction != tt.wantStatus {
				t.Errorf("Retraction = %q, want %q", got[0].Retraction, tt.wantStatus)
			}
			if in[0].Retraction != scengine.NotRetracted {
				t.Errorf("input slice mutated: Retraction = %q", in[0].Retraction)
			}
		})
	}
}

// TestConsensusExcludesRetractedFromScoreAndApex is the end-to-end proof.
// The retracted work is a meta-analysis (the apex tier) with far more
// citations than anything else and a supporting finding, so if it reached the
// scoring loop it would raise apex_design to meta-analysis, add its citations
// to total_citations, and pull the score toward +1. The control run is the
// same corpus with that one work not retracted.
func TestConsensusExcludesRetractedFromScoreAndApex(t *testing.T) {
	claim := "vitamin C prevents the common cold"
	corpus := func(retracted bool) []retractionFixture {
		return []retractionFixture{
			{title: "Vitamin C showed no significant effect on cold incidence in a cohort study",
				citedBy: 10},
			{title: "Vitamin C did not reduce cold duration in a cohort study",
				citedBy: 12},
			{title: "Vitamin C had no effect on common cold risk in a cohort study",
				citedBy: 14},
			{title: "Meta-analysis: vitamin C significantly reduced common cold incidence and improved recovery",
				citedBy: 5000, isRetracted: retracted},
		}
	}

	ctrl := &fakeRetractionClient{works: corpus(false)}
	control, err := computeConsensus(context.Background(), ctrl, claim, 10, 0, false)
	if err != nil {
		t.Fatalf("computeConsensus (control) error: %v", err)
	}
	if control.ApexDesign != scengine.DesignMetaAnalysis {
		t.Fatalf("control apex_design = %q, want %q — fixture no longer exercises the case",
			control.ApexDesign, scengine.DesignMetaAnalysis)
	}
	if control.RetractedExcluded != 0 {
		t.Fatalf("control retracted_excluded = %d, want 0", control.RetractedExcluded)
	}

	rc := &fakeRetractionClient{works: corpus(true)}
	got, err := computeConsensus(context.Background(), rc, claim, 10, 0, false)
	if err != nil {
		t.Fatalf("computeConsensus (retracted) error: %v", err)
	}

	if got.RetractedExcluded != 1 {
		t.Errorf("retracted_excluded = %d, want 1", got.RetractedExcluded)
	}
	if got.StudyCount != control.StudyCount-1 {
		t.Errorf("study_count = %d, want %d (control %d minus the retracted work)",
			got.StudyCount, control.StudyCount-1, control.StudyCount)
	}
	if got.ApexDesign == scengine.DesignMetaAnalysis {
		t.Errorf("apex_design = %q: the retracted meta-analysis still set the evidence tier",
			got.ApexDesign)
	}
	if got.TotalCitations != control.TotalCitations-5000 {
		t.Errorf("total_citations = %d, want %d: the retracted work's citation mass still counted",
			got.TotalCitations, control.TotalCitations-5000)
	}
	if got.Supporting != control.Supporting-1 {
		t.Errorf("supporting = %d, want %d: the retracted work still counted as supporting evidence",
			got.Supporting, control.Supporting-1)
	}
	if got.ConsensusScore >= control.ConsensusScore {
		t.Errorf("consensus_score = %+.2f, control %+.2f: excluding a heavily cited supporting "+
			"retraction must not leave the score at or above the control",
			got.ConsensusScore, control.ConsensusScore)
	}

	// PRISMA visibility: the excluded work must still be listed, with its
	// reason attached, rather than vanishing from the output.
	var found *workBrief
	for i := range got.AllStudies {
		if strings.HasPrefix(got.AllStudies[i].Title, "Meta-analysis: vitamin C") {
			found = &got.AllStudies[i]
		}
	}
	if found == nil {
		t.Fatalf("the retracted work disappeared from all_studies")
	}
	if found.Retraction != scengine.RetractionFlagged {
		t.Errorf("all_studies retraction = %q, want %q", found.Retraction, scengine.RetractionFlagged)
	}
	if found.RetractionNote == "" {
		t.Errorf("all_studies retraction_note is empty; the reader cannot see why it was dropped")
	}
	if found.Stance != "" {
		t.Errorf("all_studies stance = %q for an unscored work, want empty", found.Stance)
	}
}
