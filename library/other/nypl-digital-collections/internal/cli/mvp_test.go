package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchRunPlanDryRunReadsStoryPlan(t *testing.T) {
	plan := buildStoryDiscoveryPlan("Anne Boleyn", 1)
	planFile := filepath.Join(t.TempDir(), "plan.json")
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	if err := os.WriteFile(planFile, data, 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	cmd := RootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"search", "run-plan", planFile, "--dry-run", "--json", "--limit", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("search run-plan dry-run failed: %v", err)
	}

	var got struct {
		Subject  string `json:"subject"`
		DryRun   bool   `json:"dry_run"`
		Requests []struct {
			Cluster string            `json:"cluster"`
			Query   string            `json:"query"`
			Path    string            `json:"path"`
			Params  map[string]string `json:"params"`
		} `json:"requests"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Subject != "Anne Boleyn" || !got.DryRun {
		t.Fatalf("unexpected run-plan envelope: %#v", got)
	}
	if len(got.Requests) == 0 {
		t.Fatalf("expected planned requests, got none: %s", out.String())
	}
	if got.Requests[0].Path != "/items/search" || got.Requests[0].Params["per_page"] != "2" {
		t.Fatalf("unexpected first request: %#v", got.Requests[0])
	}
}

func TestRankAndDedupeStoryEvidenceItems(t *testing.T) {
	items := []storyEvidenceItem{
		{Title: "", UUID: "empty"},
		{Title: "Anne Boleyn portrait", UUID: "u1", ImageID: "img1", ItemLink: "http://example/1", TypeOfResource: "still image"},
		{Title: "Anne Boleyn portrait", UUID: "u1", ImageID: "img1", ItemLink: "http://example/1", TypeOfResource: "still image"},
		{Title: "Unrelated pamphlet", UUID: "u2"},
	}

	got := rankStoryEvidenceItems(dedupeStoryEvidenceItems(items), []string{"Anne", "portrait"})
	if len(got) != 3 {
		t.Fatalf("len ranked deduped items = %d, want 3: %#v", len(got), got)
	}
	if got[0].UUID != "u1" {
		t.Fatalf("top ranked item = %#v, want image/title-relevant item first", got[0])
	}
}

func TestWorkspaceInitAndAddRunFile(t *testing.T) {
	dir := t.TempDir()
	run := storySearchRun{
		Subject: "Anne Boleyn",
		Results: []storyEvidence{{Cluster: "core", Query: "Anne Boleyn", Items: []storyEvidenceItem{{Title: "Anne Boleyn portrait", UUID: "u1"}}}},
	}
	runFile := filepath.Join(dir, "run.json")
	runData, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("marshal run: %v", err)
	}
	if err := os.WriteFile(runFile, runData, 0o644); err != nil {
		t.Fatalf("write run: %v", err)
	}

	cmd := RootCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"workspace", "init", "tudor", "--dir", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace init failed: %v", err)
	}

	cmd = RootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"workspace", "add-run", "tudor", runFile, "--dir", dir, "--json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workspace add-run failed: %v", err)
	}

	var got workspaceDocument
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("workspace output is not JSON: %v\n%s", err, out.String())
	}
	if len(got.Items) != 1 || got.Items[0].UUID != "u1" {
		t.Fatalf("workspace items = %#v, want imported u1", got.Items)
	}
}

func TestStoriesDossierDryRunMarkdown(t *testing.T) {
	cmd := RootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"stories", "dossier", "Anne Boleyn", "--dry-run", "--markdown", "--per-cluster", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("stories dossier dry-run failed: %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "# Anne Boleyn dossier") || !strings.Contains(text, "## Narrative clusters") {
		t.Fatalf("markdown dossier missing expected sections:\n%s", text)
	}
}
