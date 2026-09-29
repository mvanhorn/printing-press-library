package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/activity-japan/internal/activityjapan"
	"github.com/spf13/cobra"
)

func TestActivityJapanSelectedAgentOutputKeepsOneEnvelope(t *testing.T) {
	for _, selectFields := range []string{"plan_id", "results.plan_id"} {
		t.Run(selectFields, func(t *testing.T) {
			var out bytes.Buffer
			cmd := &cobra.Command{Use: "sample"}
			cmd.SetOut(&out)
			flags := &rootFlags{asJSON: true, agent: true, compact: true, selectFields: selectFields}
			err := ajOutput(cmd, flags, map[string]any{"plan_id": "62375", "name": "source name"}, 0, nil)
			if err != nil {
				t.Fatal(err)
			}
			var doc struct {
				Meta    map[string]any `json:"meta"`
				Results map[string]any `json:"results"`
			}
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Results["plan_id"] != "62375" || len(doc.Results) != 1 {
				t.Fatalf("selected result %+v", doc.Results)
			}
			if doc.Meta["provider"] != "activity-japan" || doc.Meta["source"] != "live" {
				t.Fatalf("metadata %+v", doc.Meta)
			}
		})
	}
}

func TestBriefPartyGuardUsesPlanBounds(t *testing.T) {
	min, max := 2, 6
	plan := activityjapan.Plan{PartyMin: &min, PartyMax: &max}
	if err := ajPlanParty(plan, 2); err != nil {
		t.Fatal(err)
	}
	if err := ajPlanParty(plan, 1); err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Fatalf("minimum not enforced: %v", err)
	}
	if err := ajPlanParty(plan, 7); err == nil || !strings.Contains(err.Error(), "at most 6") {
		t.Fatalf("maximum not enforced: %v", err)
	}
}

func TestCompareRequiresInputsForDatedConstraints(t *testing.T) {
	cases := []struct {
		name, date, want string
		adults, budget   int
		instant          bool
	}{
		{name: "instant without date", instant: true, want: "--instant-only requires --date"},
		{name: "budget without date", adults: 2, budget: 7000, want: "--max-jpy requires --date and --adults"},
		{name: "budget without party", date: "2026-10-08", budget: 7000, want: "--max-jpy requires --date and --adults"},
		{name: "dated constraints", date: "2026-10-08", adults: 2, budget: 7000, instant: true},
		{name: "unknown constraints"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ajCompareRequestBounds(tc.date, tc.adults, -1, tc.budget, 0, tc.instant)
			if tc.want == "" && err != nil {
				t.Fatal(err)
			}
			if tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) {
				t.Fatalf("wanted %q, got %v", tc.want, err)
			}
		})
	}
}

func TestRawStockMirrorLabelsUnvalidatedEligibility(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	leaf, _, err := root.Find([]string{"source-plan", "stock-check"})
	if err != nil || leaf == nil || !strings.Contains(leaf.Short, "raw stock") || !strings.Contains(leaf.Long, "does not validate the plan's party or age limits") {
		t.Fatalf("raw stock warning missing from CLI help: command=%v err=%v", leaf, err)
	}
}
