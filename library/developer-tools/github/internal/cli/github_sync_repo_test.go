package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultSyncResources_RepoScoped(t *testing.T) {
	t.Parallel()
	got := defaultSyncResources()
	if len(got) == 0 {
		t.Fatal("defaultSyncResources must not be empty")
	}
	for _, name := range got {
		path, err := syncResourcePath(name)
		if err != nil {
			t.Fatalf("syncResourcePath(%q): %v", name, err)
		}
		if !strings.Contains(path, "{owner}") || !strings.Contains(path, "{repo}") {
			t.Fatalf("path for %q = %q, want owner/repo placeholders", name, path)
		}
		if !resourceSupportsPagination(name) {
			t.Fatalf("resourceSupportsPagination(%q) = false, want true", name)
		}
	}
	if extractID("commits", map[string]any{"sha": "abc123"}) != "abc123" {
		t.Fatal("commits must key on sha")
	}
}

func TestDropGitHubIssuePullRequests(t *testing.T) {
	t.Parallel()
	issue, err := json.Marshal(map[string]any{"id": 1, "title": "bug"})
	if err != nil {
		t.Fatal(err)
	}
	pr, err := json.Marshal(map[string]any{"id": 2, "title": "feat", "pull_request": map[string]any{"url": "https://api.github.com/repos/cli/cli/pulls/2"}})
	if err != nil {
		t.Fatal(err)
	}
	got := dropGitHubIssuePullRequests("issues", []json.RawMessage{issue, pr})
	if len(got) != 1 {
		t.Fatalf("kept %d items, want 1 issue", len(got))
	}
	if dropGitHubIssuePullRequests("pulls", []json.RawMessage{pr})[0] == nil {
		t.Fatal("pulls resource should keep PR payloads")
	}
}

func TestFillGitHubSyncRepoPath(t *testing.T) {
	syncRepoOwner, syncRepoName = "cli", "cli"
	t.Cleanup(func() { syncRepoOwner, syncRepoName = "", "" })
	got := fillGitHubSyncRepoPath("/repos/{owner}/{repo}/issues")
	if got != "/repos/cli/cli/issues" {
		t.Fatalf("got %q", got)
	}
}
