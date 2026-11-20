package cli

import "testing"

// etsy-ext: archive uses listings' all-state walk rather than the retired resource.
func TestEtsyWorkflowArchiveResources(t *testing.T) {
	hasListings := false
	for _, resource := range workflowArchiveResources {
		if resource == "listings-active" {
			t.Fatal("archive contains retired listings-active")
		}
		if resource == "listings" {
			hasListings = true
		}
	}
	if !hasListings {
		t.Fatal("archive must sync listings")
	}
}
