package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/peloton/internal/store"
)

func seedOfflineFacts(t *testing.T, home string) {
	t.Helper()
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	seed := []struct{ family, id, body string }{
		{"workouts", "w1", `{"id":"w1","ride_id":"ride-a","start_time":"2026-01-01T10:00:00Z"}`},
		{"workouts", "w2", `{"id":"w2","ride_id":"ride-a","start_time":"2026-01-08T10:00:00Z"}`},
		{"workouts", "w3", `{"id":"w3","ride_id":"ride-b","start_time":"2026-01-09T10:00:00Z"}`},
		{"workout_details", "w1", `{"id":"w1","ride_id":"ride-a","movement_tracker_data":[{"name":"squat","reps":10}]}`},
		{"workout_details", "w2", `{"id":"w2","ride_id":"ride-a"}`},
		{"workout_details", "w3", `{"id":"w3","ride_id":"ride-b"}`},
		{"performance", "w1", `{"samples":[{"seconds":0,"output":120}],"summary":{"avg_output":120}}`},
		{"classes", "ride-a", `{"id":"ride-a","title":"Synthetic Ride","instructor":{"name":"Ada"},"duration":1800,"fitness_discipline":"cycling","class_type":"ride","segments":[{"role":"warmup","metric":"cadence","targets":[55,65]},{"role":"effort","metric":"cadence","targets":[65,75]}]}`},
		{"catalog_classes", "ride-a", `{"id":"ride-a","title":"Duplicate catalog copy","instructor":{"name":"Other"},"duration":900}`},
		{"catalog_classes", "ride-b", `{"id":"ride-b","title":"Short Walk","instructor":{"name":"Bea"},"duration":900,"fitness_discipline":"walking","class_type":"walk","segments":[{"role":"walk","metric":"pace","targets":[3,4]}]}`},
		{"classes", "ride-c", `{"id":"ride-c","title":"Partial structure","duration":600}`},
		{"filters", "v1", `{"instructors":[{"name":"Ada"}],"disciplines":["cycling"]}`},
	}
	for _, item := range seed {
		if _, err := db.RecordProviderFact(item.family, item.id, json.RawMessage(item.body)); err != nil {
			t.Fatalf("seed %s/%s: %v", item.family, item.id, err)
		}
	}
}

func executeOffline(t *testing.T, home string, args ...string) (map[string]any, error) {
	t.Helper()
	root := newRootCmd(&rootFlags{})
	var out, stderr bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&stderr)
	root.SetArgs(append(args, "--home", home, "--json"))
	err := root.Execute()
	if err != nil {
		return nil, err
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("invalid JSON %q stderr=%q: %v", out.String(), stderr.String(), err)
	}
	return got, nil
}

func offlineItems(t *testing.T, got map[string]any) []any {
	t.Helper()
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", got["data"])
	}
	items, ok := data["items"].([]any)
	if !ok {
		t.Fatalf("items=%#v", data["items"])
	}
	return items
}

func TestOfflineQueriesUseOnlyU3FactsAndCaveatGaps(t *testing.T) {
	home := t.TempDir()
	seedOfflineFacts(t, home)
	for _, args := range [][]string{{"offline", "history"}, {"offline", "workout", "w1"}, {"offline", "performance", "w1"}, {"offline", "intervals", "w1"}, {"offline", "classes", "show", "ride-a"}, {"offline", "classes", "structure", "ride-a"}, {"offline", "classes", "filters"}, {"offline", "strength", "w1"}} {
		got, err := executeOffline(t, home, args...)
		if err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		meta := got["meta"].(map[string]any)
		if meta["source"] != "local" || meta["network"] != false {
			t.Fatalf("%v meta=%#v", args, meta)
		}
	}
	got, err := executeOffline(t, home, "offline", "classes", "structure", "ride-c")
	if err != nil {
		t.Fatal(err)
	}
	partial, _ := json.Marshal(got)
	if !strings.Contains(string(partial), "no comparable segment list") {
		t.Fatalf("partial structure not caveated: %s", partial)
	}
	got, err = executeOffline(t, home, "offline", "classes", "search", "--instructor", "Ada", "--duration-min", "1800", "--duration-max", "1800", "--category", "cycling", "--type", "ride", "--segment-role", "effort", "--segment-count", "2", "--metric", "cadence", "--target-min", "55", "--target-max", "55")
	if err != nil {
		t.Fatal(err)
	}
	if items := offlineItems(t, got); len(items) != 1 {
		t.Fatalf("intersection items=%#v", items)
	}
	got, err = executeOffline(t, home, "offline", "classes", "search", "--instructor", "missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(offlineItems(t, got)) != 0 {
		t.Fatal("zero-result search returned items")
	}
	got, err = executeOffline(t, home, "offline", "classes", "search", "--instructor", "Synthetic")
	if err != nil {
		t.Fatal(err)
	}
	if len(offlineItems(t, got)) != 0 {
		t.Fatal("title text matched an instructor predicate")
	}
	got, err = executeOffline(t, home, "offline", "classes", "search", "--target-min", "900", "--target-max", "900")
	if err != nil {
		t.Fatal(err)
	}
	if len(offlineItems(t, got)) != 0 {
		t.Fatal("non-target numeric field matched a provider-target predicate")
	}
	got, err = executeOffline(t, home, "offline", "performance", "w2")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	if !strings.Contains(string(encoded), "unavailable") {
		t.Fatalf("missing graph not caveated: %s", encoded)
	}
	got, err = executeOffline(t, home, "offline", "repeat", "w1", "w2")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ = json.Marshal(got)
	if !strings.Contains(string(encoded), `"same_class":true`) || !strings.Contains(string(encoded), "2026-01-01") {
		t.Fatalf("repeat=%s", encoded)
	}
	_, err = executeOffline(t, home, "offline", "repeat", "w1", "w3")
	if err == nil || ExitCode(err) != 3 {
		t.Fatalf("different class err=%v code=%d, want typed not-found", err, ExitCode(err))
	}
}

func TestOfflineOutputAvoidsCoachingSemantics(t *testing.T) {
	home := t.TempDir()
	seedOfflineFacts(t, home)
	got, err := executeOffline(t, home, "offline", "classes", "search")
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	lower := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"recommend", "readiness", "fitness label", "you should"} {
		if strings.Contains(lower, forbidden) {
			t.Fatalf("output contains %q: %s", forbidden, encoded)
		}
	}
}
