// Copyright 2026 The plane-pp-cli authors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/store"
)

// TestModuleIssuesCascadeRegistered guards that the module_issues junction is
// registered for cleanup under BOTH per_parent module resources. archived_modules
// also reconciles per_parent, so without its registration a swept archived module
// would orphan its module_issues rows. The package init() performs the wiring.
func TestModuleIssuesCascadeRegistered(t *testing.T) {
	want := store.CascadeJunction{Table: "module_issues", FKColumn: "module_id"}
	for _, resourceType := range []string{"modules", "archived_modules"} {
		got := store.CascadeJunctionsFor(resourceType)
		found := false
		for _, j := range got {
			if j == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("CascadeJunctionsFor(%q) = %+v, want it to include %+v", resourceType, got, want)
		}
	}
}
