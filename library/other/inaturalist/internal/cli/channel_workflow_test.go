package cli

import "testing"

func TestWorkflowArchivePlan_DefaultIsBoundedAndAnonymousSafe(t *testing.T) {
	resources, maxPages, err := workflowArchivePlan(false, workflowArchiveDefaultMaxPages, false)
	if err != nil {
		t.Fatalf("workflowArchivePlan() error = %v", err)
	}
	if maxPages != 1 {
		t.Fatalf("default max pages = %d, want 1", maxPages)
	}
	for _, resource := range resources {
		if workflowArchiveCredentialResources[resource] {
			t.Fatalf("anonymous archive includes credential-only resource %q", resource)
		}
	}
}

func TestWorkflowArchivePlan_AuthenticatedIncludesCredentialResources(t *testing.T) {
	resources, maxPages, err := workflowArchivePlan(true, 3, false)
	if err != nil {
		t.Fatalf("workflowArchivePlan() error = %v", err)
	}
	if maxPages != 3 {
		t.Fatalf("max pages = %d, want 3", maxPages)
	}
	for resource := range workflowArchiveCredentialResources {
		found := false
		for _, candidate := range resources {
			if candidate == resource {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("authenticated archive omitted %q", resource)
		}
	}
}

func TestWorkflowArchivePlan_UnboundedRequiresAnExplicitOptIn(t *testing.T) {
	_, maxPages, err := workflowArchivePlan(false, 0, false)
	if err != nil {
		t.Fatalf("explicit zero-page unbounded plan error = %v", err)
	}
	if maxPages != 0 {
		t.Fatalf("unbounded max pages = %d, want 0", maxPages)
	}
	if _, maxPages, err := workflowArchivePlan(false, workflowArchiveDefaultMaxPages, true); err != nil || maxPages != 0 {
		t.Fatalf("--unbounded plan = (%d, %v), want (0, nil)", maxPages, err)
	}
	if _, _, err := workflowArchivePlan(false, -1, false); err == nil {
		t.Fatal("negative max pages unexpectedly succeeded")
	}
}
