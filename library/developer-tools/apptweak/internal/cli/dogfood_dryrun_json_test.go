package cli

import (
	"encoding/json"
	"testing"
)

func TestDogfoodLearningCommandsDryRunJSONWithNoLearn(t *testing.T) {
	t.Setenv("APPTWEAK_NO_LEARN", "true")
	t.Setenv("PRINTING_PRESS_DOGFOOD", "1")
	cases := [][]string{
		{"playbook", "amend", "--query", "find items in category", "--add-note", "example correction", "--dry-run", "--json"},
		{"teach", "--query", "find items in category", "--resource-type", "items", "--resource", "GROUP-category", "--dry-run", "--json"},
		{"teach-playbook", "--query", "find items in category", "--notes", "example playbook note", "--dry-run", "--json"},
	}
	for _, args := range cases {
		out, _, err := runRootArgs(t, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		var body struct {
			DryRun bool   `json:"dry_run"`
			Action string `json:"action"`
		}
		if err := json.Unmarshal([]byte(out), &body); err != nil {
			t.Fatalf("%v invalid JSON: %v", args, err)
		}
		if !body.DryRun || body.Action == "" {
			t.Fatalf("%v missing dry-run action: %+v", args, body)
		}
	}
}
