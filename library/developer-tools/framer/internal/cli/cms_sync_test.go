package cli

import "testing"

// TestFlattenStoredCMSItem verifies that a stored item's nested fieldData is
// unwrapped to a flat map while id/slug are preserved.
func TestFlattenStoredCMSItem(t *testing.T) {
	obj := map[string]any{
		"id":   "item1",
		"slug": "hello",
		"fieldData": map[string]any{
			"title": map[string]any{"value": "Hello"},
			"body":  map[string]any{"value": "World"},
		},
	}
	flat := flattenStoredCMSItem(obj, "hello")
	if flat["id"] != "item1" {
		t.Fatalf("id not preserved: got %v", flat["id"])
	}
	if flat["slug"] != "hello" {
		t.Fatalf("slug not preserved: got %v", flat["slug"])
	}
	if flat["title"] != "Hello" || flat["body"] != "World" {
		t.Fatalf("fieldData not unwrapped: got title=%v body=%v", flat["title"], flat["body"])
	}
}

// TestComputeCMSDiff_NoFalseChanges guards the regression Greptile flagged: a
// stored item identical to the incoming item must NOT be reported as changed.
func TestComputeCMSDiff_NoFalseChanges(t *testing.T) {
	existing := map[string]map[string]any{
		"hello": flattenStoredCMSItem(map[string]any{
			"id":   "i1",
			"slug": "hello",
			"fieldData": map[string]any{
				"title": map[string]any{"value": "Hello"},
				"body":  map[string]any{"value": "World"},
			},
		}, "hello"),
	}
	incoming := map[string]map[string]any{
		"hello": {"slug": "hello", "title": "Hello", "body": "World"},
	}
	diff := computeCMSDiff(existing, incoming)
	if len(diff.Updated) != 0 {
		t.Fatalf("expected no updates for identical content, got %d: %+v", len(diff.Updated), diff.Updated)
	}
	if len(diff.Added) != 0 || len(diff.Deleted) != 0 {
		t.Fatalf("expected no add/delete, got added=%d deleted=%d", len(diff.Added), len(diff.Deleted))
	}
}

// TestComputeCMSDiff_DetectsRealChange verifies an actual field change is caught.
func TestComputeCMSDiff_DetectsRealChange(t *testing.T) {
	existing := map[string]map[string]any{
		"hello": flattenStoredCMSItem(map[string]any{
			"id":        "i1",
			"slug":      "hello",
			"fieldData": map[string]any{"title": map[string]any{"value": "Hello"}},
		}, "hello"),
	}
	incoming := map[string]map[string]any{
		"hello": {"slug": "hello", "title": "Hola"},
	}
	diff := computeCMSDiff(existing, incoming)
	if len(diff.Updated) != 1 {
		t.Fatalf("expected 1 update, got %d: %+v", len(diff.Updated), diff.Updated)
	}
	if _, ok := diff.Updated[0].ChangedFields["title"]; !ok {
		t.Fatalf("expected 'title' among changed fields, got %+v", diff.Updated[0].ChangedFields)
	}
}
