package cli

import (
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
)

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

func TestRefreshLocalStoreDeletesMissingCMSItems(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer db.Close()

	if err := db.Upsert("cms-items", "old", []byte(`{"id":"old","slug":"old","title":"vanished copy"}`)); err != nil {
		t.Fatalf("upsert stale item: %v", err)
	}
	raw := []byte(`{"collections":[],"items":[{"id":"kept","slug":"kept","title":"current copy"}]}`)
	if err := refreshLocalStore(db, raw); err != nil {
		t.Fatalf("refreshLocalStore: %v", err)
	}

	if _, err := db.Get("cms-items", "old"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Get stale item err = %v, want sql.ErrNoRows", err)
	}
	matches, err := db.Search("vanished", 10, "cms-items")
	if err != nil {
		t.Fatalf("search stale item: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("stale item search = %q, want no matches", matches)
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
