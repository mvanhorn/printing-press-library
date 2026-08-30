package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

func TestResolveLocalRejectsEndpointFilters(t *testing.T) {
	_, _, err := resolveLocal(context.Background(), nil, nil, "classes", true, "/classes", map[string]string{"category": "strength"}, "test")
	if err == nil {
		t.Fatal("resolveLocal accepted endpoint filters")
	}
	if !strings.Contains(err.Error(), "local store cannot apply endpoint filters") {
		t.Fatalf("resolveLocal error = %q", err)
	}
}

func seedResolveLocalStore(t *testing.T, resourceType, id string, data json.RawMessage) string {
	t.Helper()
	home := t.TempDir()
	restore, err := cliutil.SetHomeOverride(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)

	db, err := store.Open(defaultDBPath("peloton-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if id != "" {
		if err := db.UpsertWithFacts(resourceType, id, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return home
}

// TestResolveLocalFallsBackToSecondToLastSegmentForTrailingSubResource
// guards the fix for --data-source local always 404ing on
// `workouts performance <id>`: the resolved path
// /api/workout/{id}/performance_graph ends in a static sub-resource
// segment ("performance_graph"), not the id, so extracting only the last
// segment looked up a row that could never exist. Confirmed live before
// this fix.
func TestResolveLocalFallsBackToSecondToLastSegmentForTrailingSubResource(t *testing.T) {
	seedResolveLocalStore(t, "performance", "w1", json.RawMessage(`{"metrics":[]}`))

	data, _, err := resolveLocal(context.Background(), nil, nil, "performance", false, "/api/workout/w1/performance_graph", nil, "test")
	if err != nil {
		t.Fatalf("resolveLocal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("resolveLocal returned empty data")
	}
}

// TestResolveLocalStillPrefersLastSegmentWhenPresent guards against the
// fallback regressing the common case: most single-item endpoints
// (e.g. /api/workout/{id}) already end in the id, and must keep resolving
// on the first try without needing the fallback at all.
func TestResolveLocalStillPrefersLastSegmentWhenPresent(t *testing.T) {
	seedResolveLocalStore(t, "workouts", "w1", json.RawMessage(`{"id":"w1"}`))

	data, _, err := resolveLocal(context.Background(), nil, nil, "workouts", false, "/api/workout/w1", nil, "test")
	if err != nil {
		t.Fatalf("resolveLocal: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("resolveLocal returned empty data")
	}
}

// TestResolveLocalNotFoundReportsOriginalID guards the fallback failing
// safe: when neither the last nor second-to-last segment resolves, the
// error must still name the last-segment id it tried first (existing
// behavior/error semantics), not silently return mismatched data or a
// confusing alternate id.
func TestResolveLocalNotFoundReportsOriginalID(t *testing.T) {
	seedResolveLocalStore(t, "", "", nil)

	_, _, err := resolveLocal(context.Background(), nil, nil, "performance", false, "/api/workout/missing/performance_graph", nil, "test")
	if err == nil {
		t.Fatal("expected a not-found error")
	}
	if !strings.Contains(err.Error(), `ID "performance_graph"`) {
		t.Fatalf("error should report the original last-segment id it tried first, got: %v", err)
	}
}
