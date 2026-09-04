package cli

import (
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

func TestFillGitHubSyncRepoPath(t *testing.T) {
	syncRepoOwner, syncRepoName = "cli", "cli"
	t.Cleanup(func() { syncRepoOwner, syncRepoName = "", "" })
	got := fillGitHubSyncRepoPath("/repos/{owner}/{repo}/issues")
	if got != "/repos/cli/cli/issues" {
		t.Fatalf("got %q", got)
	}
}
