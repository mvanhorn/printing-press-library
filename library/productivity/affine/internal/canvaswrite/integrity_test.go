package canvaswrite

import "testing"

func TestCheckBlocksIntegrityDetectsMissingChildren(t *testing.T) {
	result := CheckBlocksIntegrity("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{"p1"},
		},
		"p1": {
			"sys:id":      "p1",
			"sys:flavour": "affine:paragraph",
			"prop:text":   "Broken leaf",
		},
	})

	if result.OK {
		t.Fatalf("OK = true, want false")
	}
	if result.Summary["missing_children"] != 1 {
		t.Fatalf("missing_children = %d, want 1", result.Summary["missing_children"])
	}
	if len(result.Issues) != 1 || result.Issues[0].ID != "p1" {
		t.Fatalf("issues = %#v, want one issue for p1", result.Issues)
	}
}

func TestCheckBlocksIntegrityAcceptsValidLeaf(t *testing.T) {
	result := CheckBlocksIntegrity("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{"p1"},
		},
		"p1": {
			"sys:id":       "p1",
			"sys:flavour":  "affine:paragraph",
			"sys:children": []any{},
			"prop:text":    "Valid leaf",
		},
	})

	if !result.OK {
		t.Fatalf("OK = false, issues = %#v", result.Issues)
	}
	if result.IssueCount != 0 {
		t.Fatalf("IssueCount = %d, want 0", result.IssueCount)
	}
}

func TestCheckBlocksIntegrityDetectsMissingChildRef(t *testing.T) {
	result := CheckBlocksIntegrity("doc-1", map[string]map[string]any{
		"page": {
			"sys:id":       "page",
			"sys:flavour":  "affine:page",
			"sys:children": []any{"missing"},
		},
	})

	if result.OK {
		t.Fatalf("OK = true, want false")
	}
	if result.Summary["missing_child_ref"] != 1 {
		t.Fatalf("missing_child_ref = %d, want 1", result.Summary["missing_child_ref"])
	}
}
