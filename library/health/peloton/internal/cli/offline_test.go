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
		{"classes", "ride-c", `{"id":"ride-c","title":"Partial structure","duration":600}`},
		{"classes", "ride-d", `{"ride":{"id":"ride-d","title":"Detail-fetched, genuinely no segments"},"averages":{}}`},
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
	// ride-c was synced in catalog list form (no top-level "ride" object):
	// its missing segments should be caveated as "never detail-fetched,"
	// not the generic "has no segments" message -- round-11 verification
	// NEW 1 found the generic message misleading for the common case.
	got, err := executeOffline(t, home, "offline", "classes", "structure", "ride-c")
	if err != nil {
		t.Fatal(err)
	}
	partial, _ := json.Marshal(got)
	if !strings.Contains(string(partial), "catalog list form only") {
		t.Fatalf("list-form structure not caveated as list-form: %s", partial)
	}
	if strings.Contains(string(partial), "stored class has no segment list") {
		t.Fatalf("list-form structure used the genuine-empty caveat instead: %s", partial)
	}
	// ride-d was detail-fetched (top-level "ride" object present) but
	// genuinely has no segments -- this is the real "no segment list"
	// case, distinct from ride-c's "never fetched" case above.
	got, err = executeOffline(t, home, "offline", "classes", "structure", "ride-d")
	if err != nil {
		t.Fatal(err)
	}
	detailEmpty, _ := json.Marshal(got)
	if !strings.Contains(string(detailEmpty), "stored class has no segment list") {
		t.Fatalf("detail-form empty structure not caveated as genuinely empty: %s", detailEmpty)
	}
	if strings.Contains(string(detailEmpty), "catalog list form only") {
		t.Fatalf("detail-form empty structure used the list-form caveat instead: %s", detailEmpty)
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

// TestOfflineRepeatIncludesPerformanceFacts guards a sixth live post-fix
// verification sweep's finding: offline_repeat's own description says
// "Compare two recorded workouts," but it returned no comparative data at
// all -- just same_class, ride_id, and each workout's id/date -- even
// though both workouts' already-synced "performance" records (samples,
// summary) were available. repeatFact must include each workout's stored
// performance record verbatim (raw facts, not a computed delta/ranking,
// keeping this file's "factual, non-prescriptive" contract) when present,
// and an explicit caveat -- not silence -- when it's genuinely missing.
// seedOfflineFacts gives w1 a performance record and w2 none, so this
// exercises both branches in one call.
func TestOfflineRepeatIncludesPerformanceFacts(t *testing.T) {
	home := t.TempDir()
	seedOfflineFacts(t, home)

	got, err := executeOffline(t, home, "offline", "repeat", "w1", "w2")
	if err != nil {
		t.Fatalf("offline repeat w1 w2: %v", err)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", got["data"])
	}
	workouts, ok := data["workouts"].([]any)
	if !ok || len(workouts) != 2 {
		t.Fatalf("workouts=%#v", data["workouts"])
	}
	w1, ok := workouts[0].(map[string]any)
	if !ok {
		t.Fatalf("workouts[0]=%#v", workouts[0])
	}
	if _, ok := w1["performance"]; !ok {
		t.Fatalf("w1 missing performance data: %#v", w1)
	}
	w2, ok := workouts[1].(map[string]any)
	if !ok {
		t.Fatalf("workouts[1]=%#v", workouts[1])
	}
	if _, hasPerf := w2["performance"]; hasPerf {
		t.Fatalf("w2 has no synced performance record but got one anyway: %#v", w2)
	}
	if cav, _ := w2["performance_caveat"].(string); cav == "" {
		t.Fatalf("w2 missing a performance_caveat explaining the absent data: %#v", w2)
	}
}

// TestOfflineRepeatDefaultsToPerformanceSummaryUnlessFull guards NEW ISSUE
// (offline_repeat payload size) from a seventh live post-fix verification
// sweep: a real account's largest performance record was ~5MB, dominated by
// per-second sample arrays (metrics[].values: 522KB, location_data: 4.3MB,
// seconds_since_pedaling_start: 157KB) -- two workouts' worth of that
// unconditionally embedded in offline_repeat's output (added the prior
// round) routinely exceeded the 60KB MCP result budget, discarding almost
// all of the comparative content behind a raw-text truncation fallback.
// Default output must keep the compact summary fields (well under 15KB
// even for the largest real record) plus each metric's aggregate fields
// (average_value/max_value/zones[]) with only its "values" sample array
// stripped -- an eighth round found the first version of this fix dropped
// "metrics" wholesale, which cost the only real comparison signal for any
// workout whose summaries/average_summaries are thin (non-power activities
// like stretches or yoga). --full must still provide the complete record,
// values arrays and all, for callers who actually want it.
func TestOfflineRepeatDefaultsToPerformanceSummaryUnlessFull(t *testing.T) {
	home := t.TempDir()
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	detail := `{"id":"%s","ride":{"id":"ride-a"}}`
	if _, err := db.RecordProviderFact("workout_details", "p1", json.RawMessage(fmt.Sprintf(detail, "p1"))); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("workout_details", "p2", json.RawMessage(fmt.Sprintf(detail, "p2"))); err != nil {
		t.Fatal(err)
	}
	// Mirrors the real API's field names: summaries/average_summaries are
	// the compact top-level aggregates; each metrics[] entry mixes small
	// aggregate fields (average_value/max_value) that must survive by
	// default with a "values" sample array that must not.
	perf := `{"summaries":[{"slug":"calories","value":400}],"average_summaries":[{"slug":"avg_pace","value":9.1}],"duration":1800,"metrics":[{"display_name":"Heart Rate","average_value":141,"max_value":145,"values":[424242,424243,424244]}]}`
	if _, err := db.RecordProviderFact("performance", "p1", json.RawMessage(perf)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("performance", "p2", json.RawMessage(perf)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := executeOffline(t, home, "offline", "repeat", "p1", "p2")
	if err != nil {
		t.Fatalf("offline repeat p1 p2: %v", err)
	}
	encoded, _ := json.Marshal(got)
	if !strings.Contains(string(encoded), "avg_pace") {
		t.Fatalf("default output dropped a summary field it should have kept: %s", encoded)
	}
	if !strings.Contains(string(encoded), "Heart Rate") || !strings.Contains(string(encoded), "141") {
		t.Fatalf("default output dropped metric aggregate fields it should have kept: %s", encoded)
	}
	if strings.Contains(string(encoded), "424242") {
		t.Fatalf("default output included the large per-second metrics values array, defeating the whole point: %s", encoded)
	}

	got, err = executeOffline(t, home, "offline", "repeat", "p1", "p2", "--full")
	if err != nil {
		t.Fatalf("offline repeat p1 p2 --full: %v", err)
	}
	encoded, _ = json.Marshal(got)
	if !strings.Contains(string(encoded), "avg_pace") || !strings.Contains(string(encoded), "424242") {
		t.Fatalf("--full output should include both summary and per-second fields: %s", encoded)
	}
}

// TestOfflineIDCmdKeepsFieldsAtTopLevelRegardlessOfCaveat guards a seventh
// live post-fix verification sweep's finding: offlineIDCmd (shared by
// offline_performance, offline_classes_show/structure, offline_strength)
// only nested its payload under a "result" key when a caveat fired; with
// no caveat, the same fields sat at the top of "data" instead. That made
// the correct --select path data-dependent and unknowable before the
// call -- an agent had to make an unprojected call first just to discover
// which shape it got back, defeating the point of projecting at all.
// printOfflineWithCaveats now merges caveats into the SAME top-level shape
// used when there's no caveat, so a field's location never depends on
// whether this particular call happened to produce one. seedOfflineFacts
// gives w1 a performance record and w2 none, so `offline performance w2`
// reliably exercises the caveat-firing path in one call.
func TestOfflineIDCmdKeepsFieldsAtTopLevelRegardlessOfCaveat(t *testing.T) {
	home := t.TempDir()
	seedOfflineFacts(t, home)

	got, err := executeOffline(t, home, "offline", "performance", "w2")
	if err != nil {
		t.Fatalf("offline performance w2: %v", err)
	}
	data, ok := got["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", got["data"])
	}
	if _, ok := data["result"]; ok {
		t.Fatalf("caveat-firing response still nests under a \"result\" key: %#v", data)
	}
	if _, ok := data["samples"]; !ok {
		t.Fatalf("caveat-firing response should still have \"samples\" at the top level: %#v", data)
	}
	if _, ok := data["caveats"]; !ok {
		t.Fatalf("caveat-firing response is missing its caveats entirely: %#v", data)
	}

	// --select samples must work identically whether or not this specific
	// call happens to produce a caveat -- the whole point of the fix.
	selected, err := executeOffline(t, home, "offline", "performance", "w2", "--select", "samples")
	if err != nil {
		t.Fatalf("offline performance w2 --select samples: %v", err)
	}
	selData, ok := selected["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", selected["data"])
	}
	if _, ok := selData["samples"]; !ok {
		t.Fatalf("--select samples returned nothing usable on a caveat-firing call: %#v", selData)
	}
	// An eighth round found --select silently dropped "caveats" even though
	// it isn't in the requested field list -- caveats is metadata about the
	// response (why a field is empty), not selectable content, and must
	// survive projection unconditionally the same way "meta" already does.
	if _, ok := selData["caveats"]; !ok {
		t.Fatalf("--select dropped caveats, hiding why the selected field is empty: %#v", selData)
	}

	// newOfflineWorkoutCmd builds its own RunE rather than routing through
	// offlineIDCmd (see its doc comment), so it carried the identical
	// wrap-in-"result" bug independently and needed the same fix. Seed a
	// workout_details-only record (no matching workouts record) to reliably
	// fire its "recorded history fact is unavailable" caveat.
	db, err := store.Open(filepath.Join(home, "data", "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.RecordProviderFact("workout_details", "w4", json.RawMessage(`{"id":"w4","ride_id":"ride-a"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	workoutGot, err := executeOffline(t, home, "offline", "workout", "w4")
	if err != nil {
		t.Fatalf("offline workout w4: %v", err)
	}
	workoutData, ok := workoutGot["data"].(map[string]any)
	if !ok {
		t.Fatalf("data=%#v", workoutGot["data"])
	}
	if _, ok := workoutData["result"]; ok {
		t.Fatalf("offline workout caveat-firing response still nests under a \"result\" key: %#v", workoutData)
	}
	if _, ok := workoutData["detail"]; !ok {
		t.Fatalf("offline workout caveat-firing response should still have \"detail\" at the top level: %#v", workoutData)
	}
	if _, ok := workoutData["caveats"]; !ok {
		t.Fatalf("offline workout caveat-firing response is missing its caveats entirely: %#v", workoutData)
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
