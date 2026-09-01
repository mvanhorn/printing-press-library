package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/kvmctl/internal/sequence"
)

const workflowFixture = `[{"name":"zeta","target_independent":true,"steps":[{"type":"text","value":"super-secret"},{"type":"assert_screen","contains":"password"}]},{"name":"alpha","target":"host-a","steps":[{"type":"release_all"}]}]`

func workflowCommand(t *testing.T, args ...string) (string, error) {
	t.Helper()
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append(args, "--no-learn"))
	_, err := root.ExecuteC()
	return out.String(), err
}

func workflowRepoFile(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "workflows.json")
	if err := os.Chmod(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(workflowFixture), 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestWorkflowCommandsAreDiscoverable(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"workflow-list", "workflow-inspect", "workflow-authorize", "workflow-execute"} {
		if _, _, err := root.Find([]string{name}); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestWorkflowListIsDeterministicAndDoesNotPrintSteps(t *testing.T) {
	out, err := workflowCommand(t, "workflow-list", "--repository", workflowRepoFile(t), "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "alpha") > strings.Index(out, "zeta") {
		t.Fatalf("not sorted: %s", out)
	}
	if strings.Contains(out, "super-secret") || strings.Contains(out, "password") {
		t.Fatalf("leaked step secret: %s", out)
	}
}

func TestWorkflowInspectRedactsValues(t *testing.T) {
	out, err := workflowCommand(t, "workflow-inspect", "--repository", workflowRepoFile(t), "zeta", "--target", "host-b", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "super-secret") || strings.Contains(out, "password") {
		t.Fatalf("redaction failed: %s", out)
	}
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("missing redaction: %s", out)
	}
}

func TestWorkflowAuthorizeReturnsOneTimeTokenOnlyOnExplicitAuthorization(t *testing.T) {
	repoPath := workflowRepoFile(t)
	out, err := workflowCommand(t, "workflow-authorize", "--repository", repoPath, "zeta", "--target", "host-b", "--yes", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"token"`) || strings.Contains(out, "super-secret") {
		t.Fatalf("unexpected authorization output: %s", out)
	}
}

func TestWorkflowExecuteDryRunNeedsNoHardware(t *testing.T) {
	repoPath := workflowRepoFile(t)
	repo, err := sequence.LoadWorkflowRepository(repoPath)
	if err != nil {
		t.Fatal(err)
	}
	revision := repo.List()[1].Revision
	out, err := workflowCommand(t, "workflow-execute", "--repository", repoPath, "zeta", "--target", "host-b", "--revision", revision, "--token", "token", "--dry-run", "--yes", "--json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `"dry_run": true`) || strings.Contains(out, "super-secret") {
		t.Fatalf("unexpected dry-run output: %s", out)
	}
}

func TestWorkflowAuthorizeValidation(t *testing.T) {
	_, err := workflowCommand(t, "workflow-authorize", "--repository", workflowRepoFile(t), "zeta", "--target", "host-b")
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected explicit approval error: %v", err)
	}
}
