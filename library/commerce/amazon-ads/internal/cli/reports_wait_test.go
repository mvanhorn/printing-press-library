package cli

import "testing"

func TestReportsWaitCommandSurface(t *testing.T) {
	root := RootCmd()
	reports, _, err := root.Find([]string{"reports"})
	if err != nil {
		t.Fatalf("finding reports command: %v", err)
	}
	wait, _, err := reports.Find([]string{"wait"})
	if err != nil {
		t.Fatalf("finding reports wait command: %v", err)
	}
	if wait == nil || wait.Name() != "wait" {
		t.Fatalf("reports wait command not found")
	}
	if got := wait.Annotations["mcp:read-only"]; got != "true" {
		t.Fatalf("reports wait mcp:read-only = %q, want true", got)
	}
	if got := wait.Annotations["mcp:open-world"]; got != "true" {
		t.Fatalf("reports wait mcp:open-world = %q, want true", got)
	}
	for _, flag := range []string{"status-path", "wait-timeout", "wait-interval"} {
		if wait.Flags().Lookup(flag) == nil {
			t.Fatalf("reports wait missing --%s flag", flag)
		}
	}
}
