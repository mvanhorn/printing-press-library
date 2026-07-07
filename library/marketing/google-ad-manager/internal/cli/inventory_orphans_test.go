// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestFindOrphans(t *testing.T) {
	units := []orphanAdUnit{
		{ID: "covered", Name: "Covered", Status: "ACTIVE"},
		{ID: "orphan1", Name: "Orphan One", Status: "ACTIVE"},
		{ID: "orphan2", Name: "Orphan Two", Status: "ACTIVE"},
		{ID: "inactive", Name: "Inactive", Status: "INACTIVE"}, // uncovered but not ACTIVE -> not an orphan
		{ID: "archived", Name: "Archived", Status: "ARCHIVED"},
	}

	tests := []struct {
		name    string
		covered map[string]bool
		want    []string // expected orphan ids, sorted
	}{
		{
			name:    "covered active unit is excluded; uncovered active units are orphans",
			covered: map[string]bool{"covered": true},
			want:    []string{"orphan1", "orphan2"},
		},
		{
			name:    "non-active uncovered units never count as orphans",
			covered: map[string]bool{"covered": true, "orphan1": true, "orphan2": true},
			want:    []string{}, // inactive + archived stay out
		},
		{
			name:    "empty coverage flags every active unit",
			covered: map[string]bool{},
			want:    []string{"covered", "orphan1", "orphan2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findOrphans(units, tt.covered)
			gotIDs := make([]string, 0, len(got))
			for _, o := range got {
				gotIDs = append(gotIDs, o.AdUnitID)
				if o.Reason == "" {
					t.Errorf("orphan %s has empty reason", o.AdUnitID)
				}
			}
			if !equalStrings(gotIDs, tt.want) {
				t.Fatalf("orphan ids = %v, want %v", gotIDs, tt.want)
			}
		})
	}
}

func TestSubtreeIDs(t *testing.T) {
	// root -> a -> a1 ; root -> b
	units := []orphanAdUnit{
		{ID: "root", ParentID: ""},
		{ID: "a", ParentID: "root"},
		{ID: "a1", ParentID: "a"},
		{ID: "b", ParentID: "root"},
	}

	if got := subtreeIDs(units, ""); len(got) != 4 {
		t.Fatalf("empty root should return all 4 ids, got %d", len(got))
	}

	sub := subtreeIDs(units, "a")
	if !sub["a"] || !sub["a1"] {
		t.Fatalf("subtree(a) must include a and a1, got %v", sub)
	}
	if sub["root"] || sub["b"] {
		t.Fatalf("subtree(a) must exclude root and b, got %v", sub)
	}
}
