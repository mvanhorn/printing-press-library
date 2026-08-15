package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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

// TestOfflineSelectFiltersThePayloadNotTheEnvelope guards SIGNIFICANT #3
// from a live post-fix verification sweep: printOffline used to wrap its
// value into {"meta":...,"data":value} and only then hand the whole
// envelope to the shared --select filter, which looks for the requested
// field names among the envelope's own top-level keys (meta/data) instead
// of the real fields inside "data" -- so `offline performance <id> --select
// summary` silently returned {} regardless of what field name was
// requested, for every offline_* command. --select must filter the inner
// value before it gets wrapped, matching how live single-fetch commands
// (e.g. workouts_performance.go) already filter before wrapping their own
// provenance envelope.
func TestOfflineSelectFiltersThePayloadNotTheEnvelope(t *testing.T) {
	home := t.TempDir()
	seedOfflineFacts(t, home)

	got, err := executeOffline(t, home, "offline", "performance", "w1", "--select", "summary")
	if err != nil {
		t.Fatal(err)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", got["data"])
	}
	if _, ok := data["summary"]; !ok {
		t.Fatalf("--select summary dropped the requested field entirely: %#v", data)
	}
	if _, ok := data["samples"]; ok {
		t.Fatalf("--select summary should have dropped the unrequested \"samples\" field: %#v", data)
	}
	if len(data) != 1 {
		t.Fatalf("--select summary should leave exactly one field, got %#v", data)
	}
}

// TestOfflineCompactRecursesIntoWrapperShapes guards NEW ISSUE C from a
// fourth live post-fix verification sweep: --compact is documented as
// "only key fields (id, name, status, timestamps)" but the shared
// compactFields (helpers.go) only strips its blocklist at the top level.
// offline_workout's output wraps the real payload under "detail"/"history"
// keys, neither of which matches the blocklist, so --compact previously
// returned the complete, unstripped payload -- achievement_templates and
// all. compactOfflineFields must recurse so the blocklist actually reaches
// the nested content.
func TestOfflineCompactRecursesIntoWrapperShapes(t *testing.T) {
	home := t.TempDir()
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	body := `{"id":"w1","name":"Ride","description":"verbose prose","achievement_templates":[{"id":"a1"}],"nested":{"comments":["noisy"],"keep":"yes"}}`
	if _, err := db.RecordProviderFact("workout_details", "w1", json.RawMessage(body)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := executeOffline(t, home, "offline", "workout", "w1", "--compact")
	if err != nil {
		t.Fatalf("offline workout w1 --compact: %v", err)
	}
	encoded, _ := json.Marshal(got)
	for _, verbose := range []string{"verbose prose", "achievement_templates", "noisy"} {
		if strings.Contains(string(encoded), verbose) {
			t.Fatalf("--compact left verbose content %q in nested output: %s", verbose, encoded)
		}
	}
	for _, kept := range []string{`"id":"w1"`, `"name":"Ride"`, `"keep":"yes"`} {
		if !strings.Contains(string(encoded), kept) {
			t.Fatalf("--compact dropped legitimate content %q it should have kept: %s", kept, encoded)
		}
	}
}

// TestOfflineIntervalsAndRepeatResolveNestedRideID guards NEW ISSUE A from a
// third live post-fix verification sweep: workout_details payloads never
// carry a top-level ride_id/rideId -- the real Peloton API only nests it
// under ride.id (confirmed against a real stored record) -- unlike the
// "workouts" family, which enrichWorkoutRideMetadata promotes to a
// top-level field at write time for exactly this reason. offline_intervals
// and offline_repeat both read workout_details directly, so both hit the
// same bug: every class-based workout looked like it had no class
// association at all. The existing seedOfflineFacts fixture happens to put
// ride_id at the top level already, which is why this slipped past earlier
// tests -- this test uses the real, nested-only shape deliberately.
func TestOfflineIntervalsAndRepeatResolveNestedRideID(t *testing.T) {
	home := t.TempDir()
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	nested := `{"id":"%s","ride":{"id":"ride-nested","title":"Nested Ride"}}`
	if _, err := db.RecordProviderFact("workout_details", "n1", json.RawMessage(fmt.Sprintf(nested, "n1"))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("workout_details", "n2", json.RawMessage(fmt.Sprintf(nested, "n2"))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("classes", "ride-nested", json.RawMessage(`{"id":"ride-nested","segments":[{"role":"warmup"}]}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := executeOffline(t, home, "offline", "intervals", "n1")
	if err != nil {
		t.Fatalf("offline intervals n1: %v", err)
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "not class-based") {
		t.Fatalf("offline intervals failed to resolve the nested ride.id: %s", encoded)
	}
	if !strings.Contains(string(encoded), "ride-nested") {
		t.Fatalf("offline intervals did not resolve ride_id=ride-nested: %s", encoded)
	}

	got, err = executeOffline(t, home, "offline", "repeat", "n1", "n2")
	if err != nil {
		t.Fatalf("offline repeat n1 n2: %v", err)
	}
	encoded, _ = json.Marshal(got)
	if !strings.Contains(string(encoded), `"same_class":true`) {
		t.Fatalf("offline repeat failed to resolve the nested ride.id on both workouts: %s", encoded)
	}
}

// TestOfflineIntervalsAndRepeatTreatFreestyleSentinelAsNoClass guards a
// sixth live post-fix verification sweep's finding: Peloton uses
// "00000000000000000000000000000000" (32 zeros) as ride.id for
// freestyle/non-class workouts (Just Run, Outdoor Running, Just Ride) --
// a genuine-looking value, not an empty/absent field. Before this fix,
// offline_repeat treated any two freestyle workouts (both carrying the
// identical sentinel) as belonging to the SAME class -- a false positive
// confirmed live on the real account (84 of 430 workout_details records
// carry this sentinel, so unrelated freestyle sessions like "Outdoor
// Running" and "Just Run" were reported same_class:true) -- and
// offline_intervals reported a misleading "stored class structure is
// unavailable" caveat implying a sync gap, for a workout that was never a
// class at all.
func TestOfflineIntervalsAndRepeatTreatFreestyleSentinelAsNoClass(t *testing.T) {
	home := t.TempDir()
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	freestyle := `{"id":"%s","ride":{"id":"00000000000000000000000000000000","title":null}}`
	if _, err := db.RecordProviderFact("workout_details", "f1", json.RawMessage(fmt.Sprintf(freestyle, "f1"))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("workout_details", "f2", json.RawMessage(fmt.Sprintf(freestyle, "f2"))); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := executeOffline(t, home, "offline", "intervals", "f1")
	if err != nil {
		t.Fatalf("offline intervals f1: %v", err)
	}
	encoded, _ := json.Marshal(got)
	if !strings.Contains(string(encoded), "not class-based") {
		t.Fatalf("offline intervals did not recognize the freestyle sentinel as no-class: %s", encoded)
	}
	if strings.Contains(string(encoded), "00000000000000000000000000000000") {
		t.Fatalf("offline intervals treated the freestyle sentinel as a real ride_id: %s", encoded)
	}

	got, err = executeOffline(t, home, "offline", "repeat", "f1", "f2")
	if err != nil {
		t.Fatalf("offline repeat f1 f2: %v", err)
	}
	encoded, _ = json.Marshal(got)
	if !strings.Contains(string(encoded), `"same_class":false`) {
		t.Fatalf("offline repeat false-positived two unrelated freestyle workouts as same_class: %s", encoded)
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
