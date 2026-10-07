// Hand-authored test for PATCH(clickup-which-index-tracks-novel-features).

package cli

import (
	"encoding/json"
	"os"
	"testing"
)

// Every novel feature the manifest advertises must be reachable via `which`.
func TestWhichIndexCoversManifestNovelFeatures(t *testing.T) {
	raw, err := os.ReadFile("../../.printing-press.json")
	if err != nil {
		t.Skipf("manifest not found: %v", err)
	}
	var m struct {
		NovelFeatures []struct {
			Command string `json:"command"`
		} `json:"novel_features"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	indexed := map[string]bool{}
	for _, e := range whichIndex {
		indexed[e.Command] = true
	}
	for _, nf := range m.NovelFeatures {
		if !indexed[nf.Command] {
			t.Errorf("novel feature %q is missing from whichIndex", nf.Command)
		}
	}
}

func TestWhichResolvesAttachmentQueries(t *testing.T) {
	for query, want := range map[string]string{
		"attach":                    "task attach",
		"upload":                    "task attach",
		"upload attachment to task": "task attach",
		"attachments":               "task attachments",
	} {
		got := rankWhich(whichIndex, query, 3)
		if len(got) == 0 || got[0].Entry.Command != want {
			t.Errorf("which %q: want top match %q, got %+v", query, want, got)
		}
	}
}
