package cli

import (
	"bytes"
	"encoding/json"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
	"testing"
)

func TestSendfoxEvidenceCommandFlagsAndOutputs(t *testing.T) {
	for _, name := range []string{"campaign-preflight", "campaign-review", "audience-health", "contact-dossier", "automation-plan", "export-bundle", "migration-readiness"} {
		t.Run(name, func(t *testing.T) {
			testenv.Isolate(t)
			cmd := RootCmd()
			args := []string{"workflow", name, "--input", "../../examples/snapshot.json", "--max-age", "0", "--json"}
			if name == "contact-dossier" {
				args = append(args, "--id", "1")
			}
			cmd.SetArgs(args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var report map[string]any
			if err := json.Unmarshal(out.Bytes(), &report); err != nil {
				t.Fatal(err)
			}
			if report["workflow"] != name || report["data"] == nil {
				t.Fatal(report)
			}
		})
	}
}

func TestAgentPlanRetainsBodies(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"workflow", "automation-plan", "--input", "../../examples/snapshot.json", "--max-age", "0", "--agent"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if results, ok := doc["results"].(map[string]any); ok {
		doc = results
	}
	actions := doc["proposed_actions"].([]any)
	body := actions[0].(map[string]any)["body"].(map[string]any)
	if body["active"] != false || body["title"] == "" {
		t.Fatal(body)
	}
}

func TestBatchAcknowledgementRejectsAmbiguousResults(t *testing.T) {
	for _, result := range []any{nil, []any{}, map[string]any{}, map[string]any{"created": 1.5, "updated": 0.5}, map[string]any{"created": -1.0, "updated": 3.0}} {
		if validateSendfoxBatchAcknowledgement(result, 2) == nil {
			t.Fatalf("accepted %#v", result)
		}
	}
	if err := validateSendfoxBatchAcknowledgement(map[string]any{"created": 2.0, "updated": 0.0}, 2); err != nil {
		t.Fatal(err)
	}
}
