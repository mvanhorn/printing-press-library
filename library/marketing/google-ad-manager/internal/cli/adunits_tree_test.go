// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestBuildAdUnitTree(t *testing.T) {
	// root -> {a, b}; a -> {a1}. "orphan" has a parent not present in input
	// and must surface as its own root rather than being dropped.
	units := func() []adUnitNode {
		return []adUnitNode{
			{AdUnitID: "root", Name: "Root", Status: "ACTIVE", ParentID: ""},
			{AdUnitID: "a", Name: "A", Status: "ACTIVE", ParentID: "root"},
			{AdUnitID: "b", Name: "B", Status: "INACTIVE", ParentID: "root"},
			{AdUnitID: "a1", Name: "A1", Status: "ACTIVE", ParentID: "a"},
			{AdUnitID: "orphan", Name: "Orphan", Status: "ACTIVE", ParentID: "ghost"},
		}
	}

	tests := []struct {
		name      string
		root      string
		wantRoots []string // top-level ids, in order
		check     func(t *testing.T, forest []*adUnitNode)
	}{
		{
			name:      "full forest keeps unparented nodes as roots",
			root:      "",
			wantRoots: []string{"orphan", "root"}, // sorted by id
			check: func(t *testing.T, forest []*adUnitNode) {
				var root *adUnitNode
				for _, n := range forest {
					if n.AdUnitID == "root" {
						root = n
					}
				}
				if root == nil {
					t.Fatal("root node missing")
				}
				if len(root.Children) != 2 || root.Children[0].AdUnitID != "a" || root.Children[1].AdUnitID != "b" {
					t.Fatalf("root children = %v, want [a b]", ids(root.Children))
				}
				if len(root.Children[0].Children) != 1 || root.Children[0].Children[0].AdUnitID != "a1" {
					t.Fatalf("a children = %v, want [a1]", ids(root.Children[0].Children))
				}
			},
		},
		{
			name:      "root filter returns just that subtree",
			root:      "a",
			wantRoots: []string{"a"},
			check: func(t *testing.T, forest []*adUnitNode) {
				if len(forest) != 1 || forest[0].AdUnitID != "a" {
					t.Fatalf("forest = %v, want [a]", ids(forest))
				}
				if len(forest[0].Children) != 1 || forest[0].Children[0].AdUnitID != "a1" {
					t.Fatalf("subtree children = %v, want [a1]", ids(forest[0].Children))
				}
			},
		},
		{
			name:      "root given as full resource name resolves to id",
			root:      "networks/123/adUnits/a",
			wantRoots: []string{"a"},
		},
		{
			name:      "unknown root yields empty forest",
			root:      "does-not-exist",
			wantRoots: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			forest := buildAdUnitTree(units(), tt.root)
			if got := ids(forest); !equalStrings(got, tt.wantRoots) {
				t.Fatalf("roots = %v, want %v", got, tt.wantRoots)
			}
			if tt.check != nil {
				tt.check(t, forest)
			}
		})
	}
}

func TestFilterAdUnitTreeStatus(t *testing.T) {
	// root(INACTIVE) -> a(INACTIVE) -> a1(ACTIVE). Filtering to ACTIVE must
	// retain a1 plus its ancestors (root, a) so the path to the match survives.
	forest := buildAdUnitTree([]adUnitNode{
		{AdUnitID: "root", Status: "INACTIVE", ParentID: ""},
		{AdUnitID: "a", Status: "INACTIVE", ParentID: "root"},
		{AdUnitID: "a1", Status: "ACTIVE", ParentID: "a"},
		{AdUnitID: "b", Status: "INACTIVE", ParentID: "root"},
	}, "")

	got := filterAdUnitTreeStatus(forest, "active")
	if len(got) != 1 || got[0].AdUnitID != "root" {
		t.Fatalf("filtered roots = %v, want [root]", ids(got))
	}
	if len(got[0].Children) != 1 || got[0].Children[0].AdUnitID != "a" {
		t.Fatalf("root kept children = %v, want [a] (b has no active descendant)", ids(got[0].Children))
	}
	if len(got[0].Children[0].Children) != 1 || got[0].Children[0].Children[0].AdUnitID != "a1" {
		t.Fatalf("a kept children = %v, want [a1]", ids(got[0].Children[0].Children))
	}

	// Empty status filter is a pass-through.
	if same := filterAdUnitTreeStatus(forest, ""); len(same) != len(forest) {
		t.Fatalf("empty status filter changed forest size: got %d want %d", len(same), len(forest))
	}
}

func TestAdUnitNameToID(t *testing.T) {
	cases := map[string]string{
		"networks/123/adUnits/456": "456",
		"456":                      "456",
		"":                         "",
	}
	for in, want := range cases {
		if got := adUnitNameToID(in); got != want {
			t.Errorf("adUnitNameToID(%q) = %q, want %q", in, got, want)
		}
	}
}

func ids(nodes []*adUnitNode) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.AdUnitID)
	}
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
