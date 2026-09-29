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
