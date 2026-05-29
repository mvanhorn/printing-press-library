package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildStoryDiscoveryPlan_AnneBoleynClustersQueriesAndAnalogues(t *testing.T) {
	plan := buildStoryDiscoveryPlan("Anne Boleyn", 3)

	if plan.Subject != "Anne Boleyn" {
		t.Fatalf("Subject = %q, want Anne Boleyn", plan.Subject)
	}
	if len(plan.Clusters) < 4 {
		t.Fatalf("len(Clusters) = %d, want at least 4", len(plan.Clusters))
	}
	if !containsString(plan.Queries, "Anne Boleyn fiction") {
		t.Fatalf("queries missing Anne Boleyn fiction: %#v", plan.Queries)
	}
	if !containsString(plan.Queries, "The secret diary of Anne Boleyn") {
		t.Fatalf("queries missing known Anne Boleyn story title: %#v", plan.Queries)
	}

	executedQueens := findStoryCluster(plan.Clusters, "executed_queens")
	if executedQueens == nil {
		t.Fatalf("missing executed_queens cluster: %#v", plan.Clusters)
	}
	if !containsSimilarCharacter(executedQueens.SimilarCharacters, "Catherine Howard") {
		t.Fatalf("executed_queens missing Catherine Howard: %#v", executedQueens.SimilarCharacters)
	}
	if !containsSimilarCharacter(executedQueens.SimilarCharacters, "Mary, Queen of Scots") {
		t.Fatalf("executed_queens missing Mary, Queen of Scots: %#v", executedQueens.SimilarCharacters)
	}
}

func TestStoryDiscoveryPlan_RespectsPerClusterLimit(t *testing.T) {
	plan := buildStoryDiscoveryPlan("Anne Boleyn", 1)
	for _, cluster := range plan.Clusters {
		if len(cluster.Searches) > 1 {
			t.Fatalf("cluster %s has %d searches, want <= 1", cluster.ID, len(cluster.Searches))
		}
	}
}

func TestStoriesDiscoverDryRunPrintsJSONPlan(t *testing.T) {
	cmd := RootCmd()
	var out strings.Builder
	cmd.SetOut(&out)
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{"stories", "discover", "Anne Boleyn", "--dry-run", "--json", "--per-cluster", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("stories discover dry-run failed: %v", err)
	}
	var got struct {
		Subject  string          `json:"subject"`
		Clusters []storyCluster  `json:"clusters"`
		Queries  []string        `json:"queries"`
		DryRun   bool            `json:"dry_run"`
		Meta     json.RawMessage `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out.String()), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if !got.DryRun {
		t.Fatalf("dry_run = false, want true: %s", out.String())
	}
	if got.Subject != "Anne Boleyn" {
		t.Fatalf("Subject = %q, want Anne Boleyn", got.Subject)
	}
	if len(got.Clusters) == 0 || len(got.Queries) == 0 {
		t.Fatalf("dry-run plan missing clusters/queries: %s", out.String())
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func findStoryCluster(clusters []storyCluster, id string) *storyCluster {
	for i := range clusters {
		if clusters[i].ID == id {
			return &clusters[i]
		}
	}
	return nil
}

func containsSimilarCharacter(chars []similarCharacter, name string) bool {
	for _, char := range chars {
		if char.Name == name {
			return true
		}
	}
	return false
}
