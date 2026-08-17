// Copyright 2026 nspage and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"testing"
)

func TestResourceStorageID_ParentNamespaceFallbacks(t *testing.T) {
	const resourceType = "test_child_resource"
	const parentKey = "project_id"
	const childID = "space-1"

	prevParentKeys := resourceParentKeyColumns
	prevScopeSources := childScopeColumnSources
	resourceParentKeyColumns = map[string]string{resourceType: parentKey}
	childScopeColumnSources = map[string]string{}
	t.Cleanup(func() {
		resourceParentKeyColumns = prevParentKeys
		childScopeColumnSources = prevScopeSources
	})

	nul := string([]byte{0})

	t.Run("non_parent_keyed_returns_bare_id", func(t *testing.T) {
		got := resourceStorageID("venues", childID, map[string]any{"id": childID})
		if got != childID {
			t.Fatalf("got %q, want bare id %q", got, childID)
		}
	})

	t.Run("parent_field_present", func(t *testing.T) {
		got := resourceStorageID(resourceType, childID, map[string]any{
			"id":         childID,
			"project_id": "proj-A",
		})
		want := childID + nul + "proj-A"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("missing_parent_field_uses_injected_parent_id", func(t *testing.T) {
		// Greptile P1: two parent-scoped syncs must not share a key when the
		// body omits the parent field but sync stamps parent_id from the path.
		gotA := resourceStorageID(resourceType, childID, map[string]any{
			"id":        childID,
			"parent_id": "proj-A",
		})
		gotB := resourceStorageID(resourceType, childID, map[string]any{
			"id":        childID,
			"parent_id": "proj-B",
		})
		wantA := childID + nul + "proj-A"
		wantB := childID + nul + "proj-B"
		if gotA != wantA {
			t.Fatalf("parent A: got %q, want %q", gotA, wantA)
		}
		if gotB != wantB {
			t.Fatalf("parent B: got %q, want %q", gotB, wantB)
		}
		if gotA == gotB {
			t.Fatal("distinct injected parents must not collide")
		}
	})

	t.Run("missing_parent_field_uses_scope_column", func(t *testing.T) {
		childScopeColumnSources = map[string]string{"projects_id": parentKey}
		t.Cleanup(func() { childScopeColumnSources = map[string]string{} })

		got := resourceStorageID(resourceType, childID, map[string]any{
			"id":          childID,
			"projects_id": "proj-scope",
		})
		want := childID + nul + "proj-scope"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("no_parent_context_uses_sentinel_not_bare_id", func(t *testing.T) {
		got := resourceStorageID(resourceType, childID, map[string]any{"id": childID})
		want := childID + nul + "__missing_parent__"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
		if got == childID {
			t.Fatal("missing parent must not collapse to bare child id")
		}
	})
}
