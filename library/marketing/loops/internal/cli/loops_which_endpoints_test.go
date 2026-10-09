package cli

import (
	"encoding/json"
	"testing"
)

func TestLoopsWhichFindsEndpointCommands(t *testing.T) {
	cases := map[string]string{
		"campaign metrics":            "campaigns metrics get-campaign",
		"transactional email metrics": "transactional-emails metrics get-transactional-email",
		"workflow node metrics":       "workflows nodes get-workflow-metrics",
		"campaign preflight":          "campaigns preflight",
		"verify team":                 "team verify",
	}
	for query, want := range cases {
		out, err := runLoopsCommand(t, "which", "--agent", "--limit", "1", query)
		if err != nil {
			t.Fatalf("which %q: %v (%s)", query, err, out)
		}
		var result struct {
			Results struct {
				Matches []whichMatch `json:"matches"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(out), &result); err != nil {
			t.Fatalf("which %q: invalid JSON %s", query, out)
		}
		if len(result.Results.Matches) != 1 || result.Results.Matches[0].Entry.Command != want {
			t.Errorf("which %q top match = %+v, want %q", query, result.Results.Matches, want)
		}
	}
}

func TestLoopsWhichIndexKeepsCuratedEntriesFirstWithoutDuplicates(t *testing.T) {
	index := loopsWhichIndex(RootCmd())
	if len(index) <= len(whichIndex) {
		t.Fatalf("expected endpoint commands beyond the %d curated entries, got %d", len(whichIndex), len(index))
	}
	for i, entry := range whichIndex {
		if index[i].Command != entry.Command {
			t.Fatalf("curated entry %d moved: got %q want %q", i, index[i].Command, entry.Command)
		}
	}
	seen := map[string]bool{}
	for _, entry := range index {
		if seen[entry.Command] {
			t.Errorf("duplicate which entry %q", entry.Command)
		}
		seen[entry.Command] = true
	}
}
