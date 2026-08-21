package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestSaveSyncStates_WritesAllRowsTogether guards --full's two-tier
// dependent-sync resumability (internal/cli/sync.go): the sweep cursor,
// failed-id backlog, and tier-alternation turn bit are three independent
// sync_state rows that must never be allowed to diverge (a live PR review
// finding: separate SaveSyncState calls left a window where a crash
// between them, or any one alone failing, could commit some checkpoints
// without others). Asserts all three rows land with the values passed,
// confirming the atomic-write path is wired correctly; the atomicity
// guarantee itself (all succeed or none does) comes from wrapping every
// statement in one SQL transaction, which this test does not attempt to
// fault-inject -- that guarantee is SQLite's own transactional contract,
// not custom logic worth re-verifying here.
func TestSaveSyncStates_WritesAllRowsTogether(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.SaveSyncStates(
		SyncStateWrite{ResourceType: "performance:full_progress", Cursor: "", Count: 7},
		SyncStateWrite{ResourceType: "performance:full_failed", Cursor: "w1,w2", Count: 2},
		SyncStateWrite{ResourceType: "performance:full_turn", Cursor: "", Count: 1},
	); err != nil {
		t.Fatalf("SaveSyncStates: %v", err)
	}

	cursor1, _, count1, err := s.GetSyncState("performance:full_progress")
	if err != nil {
		t.Fatalf("GetSyncState(full_progress): %v", err)
	}
	if cursor1 != "" || count1 != 7 {
		t.Errorf("full_progress = (%q, %d), want (\"\", 7)", cursor1, count1)
	}

	cursor2, _, count2, err := s.GetSyncState("performance:full_failed")
	if err != nil {
		t.Fatalf("GetSyncState(full_failed): %v", err)
	}
	if cursor2 != "w1,w2" || count2 != 2 {
		t.Errorf("full_failed = (%q, %d), want (\"w1,w2\", 2)", cursor2, count2)
	}

	_, _, count3, err := s.GetSyncState("performance:full_turn")
	if err != nil {
		t.Fatalf("GetSyncState(full_turn): %v", err)
	}
	if count3 != 1 {
		t.Errorf("full_turn count = %d, want 1", count3)
	}
}

// TestUpsertBatchWithFacts_UnwrapsIDBearingEnvelopeItems guards
// recordProviderFactsBestEffort against silently dropping items UpsertBatch
// itself successfully stores. UpsertBatch falls back to
// unwrapIDBearingEnvelopeItem when the outer envelope has no direct id (see
// TestUpsertBatch_UnwrapsIDBearingEnvelopeItems); recordProviderFactsBestEffort
// must apply the same fallback, or an item like {"workout":{"id":"w1",...}}
// lands in `resources` (via UpsertBatch) but never in `provider_payloads`
// (via the *WithFacts wrapper), reproducing exactly the resources/
// provider_payloads drift the WithFacts wrappers exist to close.
func TestUpsertBatchWithFacts_UnwrapsIDBearingEnvelopeItems(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	items := []json.RawMessage{
		json.RawMessage(`{"customer":{"id":"cust-1","name":"Ada"}}`),
		json.RawMessage(`{"id":"cust-2","name":"Grace"}`),
	}
	stored, extractFailures, err := s.UpsertBatchWithFacts("customers", items)
	if err != nil {
		t.Fatalf("UpsertBatchWithFacts: %v", err)
	}
	if stored != 2 || extractFailures != 0 {
		t.Fatalf("stored=%d extractFailures=%d, want stored=2 extractFailures=0", stored, extractFailures)
	}

	// cust-1 only resolves an id via the envelope-unwrap fallback -- this is
	// the case that was previously dropped from provider_payloads even
	// though UpsertBatch stored it correctly in `resources`.
	if _, err := s.GetProviderFact("customers", "cust-1"); err != nil {
		t.Fatalf("GetProviderFact(customers, cust-1): %v (envelope-unwrapped item missing from provider_payloads)", err)
	}
	fact, err := s.GetProviderFact("customers", "cust-2")
	if err != nil {
		t.Fatalf("GetProviderFact(customers, cust-2): %v", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("cust-2 provider fact has empty payload")
	}
}

// TestUpsertBatchWithFacts_ClassesShowNestedRideIDStillCaches guards a
// round-9 live verification finding: classes_show's live response wraps its
// real id under a top-level "ride" object ({"ride":{"id":"...",...},
// "segments":{...},"averages":{...},...}) rather than exposing "id" at the
// top level like the classes LIST endpoint's flat items do. The generic
// unwrapIDBearingEnvelopeItem fallback can't help here -- it only fires when
// exactly one top-level field is an object, and classes_show's response has
// several (ride, playlist, averages, segments, ...). Before this fix,
// EVERY classes_show call silently failed to cache -- confirmed live: 99
// class ids present in `resources` (via the bulk classes sync) had no
// corresponding provider_payloads row, and offline_classes_show/
// offline_classes_structure/offline_intervals errored "stored catalog_classes
// fact <id> not found" for every one of them, even after directly fetching
// each one live via classes_show. nestedContainerResourceID must resolve the
// id from obj["ride"]["id"] while still caching the FULL outer object (not
// just the inner "ride" value) -- its sibling fields (segments, averages)
// are exactly what offline_classes_structure/offline_intervals need and
// aren't present at all in the classes LIST endpoint's flatter item shape.
func TestUpsertBatchWithFacts_ClassesShowNestedRideIDStillCaches(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	item := json.RawMessage(`{"ride":{"id":"ride-nested-1","title":"30 min Pop Run"},"segments":{"segment_list":[{"id":"seg-1"}]},"averages":{"output":150}}`)
	stored, extractFailures, err := s.UpsertBatchWithFacts("classes", []json.RawMessage{item})
	if err != nil {
		t.Fatalf("UpsertBatchWithFacts: %v", err)
	}
	if stored != 1 || extractFailures != 0 {
		t.Fatalf("stored=%d extractFailures=%d, want stored=1 extractFailures=0", stored, extractFailures)
	}

	fact, err := s.GetProviderFact("classes", "ride-nested-1")
	if err != nil {
		t.Fatalf("GetProviderFact(classes, ride-nested-1): %v (nested ride.id not resolved, or resources/provider_payloads drifted)", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(fact.Payload, &payload); err != nil {
		t.Fatalf("unmarshal cached payload: %v", err)
	}
	if _, ok := payload["segments"]; !ok {
		t.Fatalf("cached payload lost its \"segments\" sibling field -- must cache the full outer object, not just the inner \"ride\" value: %#v", payload)
	}
	if _, ok := payload["ride"]; !ok {
		t.Fatalf("cached payload lost its \"ride\" field: %#v", payload)
	}
}

// TestUpsertWithFacts_UsesCallerSuppliedID guards against
// recordProviderFactsBestEffort's body-derived id resolution ever creeping
// into UpsertWithFacts: callers of Upsert already know the id (sometimes,
// as with Peloton's performance_graph, the response body carries no id
// field at all -- the id comes from the request path, not the body), so
// re-deriving it from the body would silently drop exactly those facts.
func TestUpsertWithFacts_UsesCallerSuppliedID(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	data := json.RawMessage(`{"metrics":[{"display_name":"Output","values":[1,2,3]}]}`)
	if err := s.UpsertWithFacts("performance", "w1", data); err != nil {
		t.Fatalf("UpsertWithFacts: %v", err)
	}
	fact, err := s.GetProviderFact("performance", "w1")
	if err != nil {
		t.Fatalf("GetProviderFact(performance, w1): %v (caller-supplied id was not used)", err)
	}
	if len(fact.Payload) == 0 {
		t.Fatal("performance provider fact has empty payload")
	}
}

// TestRecordProviderFact_AdvancesFetchedAtEvenWhenContentUnchanged guards
// NEW ISSUE E's --stale-before mechanism from a fourth live post-fix
// verification sweep: a live-verify run found that refetching a record
// whose content genuinely hadn't changed left fetched_at untouched (the
// content-hash dedup short-circuited the write entirely), so --stale-before
// would report the exact same record as stale on every subsequent call
// forever, never actually clearing its stale status. fetched_at must mean
// "when was this record last confirmed fresh," not "when did its content
// last change" -- the payload rewrite can still be skipped as an
// optimization, but the timestamp must always advance on a successful
// fetch.
func TestRecordProviderFact_AdvancesFetchedAtEvenWhenContentUnchanged(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	body := json.RawMessage(`{"id":"w1","metrics":[1,2,3]}`)
	if _, err := s.RecordProviderFact("performance", "w1", body); err != nil {
		t.Fatalf("first RecordProviderFact: %v", err)
	}
	first, err := s.GetProviderFact("performance", "w1")
	if err != nil {
		t.Fatalf("GetProviderFact after first write: %v", err)
	}

	// Re-record byte-identical content -- content_hash matches, so this
	// exercises the short-circuit path specifically.
	changed, err := s.RecordProviderFact("performance", "w1", body)
	if err != nil {
		t.Fatalf("second RecordProviderFact: %v", err)
	}
	if changed {
		t.Fatal("changed=true for byte-identical content, want false (content really is unchanged)")
	}
	second, err := s.GetProviderFact("performance", "w1")
	if err != nil {
		t.Fatalf("GetProviderFact after second write: %v", err)
	}
	if !second.FetchedAt.After(first.FetchedAt) {
		t.Fatalf("fetched_at did not advance on unchanged-content refetch: first=%v second=%v", first.FetchedAt, second.FetchedAt)
	}
}

// TestRedactProviderPayload_PreservesJoinTokensButRedactsRealCredentials
// guards MINOR #8 from a live post-fix verification sweep: "join_tokens" is
// a short-lived identifier used to join a live class session in progress,
// not a credential -- but its normalized form ("jointokens") substring-
// matches the "token" redaction needle, so it was being redacted from every
// stored live-class-join response. isSensitiveProviderKey must special-case
// it while still catching genuinely sensitive fields (verified with a
// sibling field, session_id, which legitimately should stay redacted).
func TestRedactProviderPayload_PreservesJoinTokensButRedactsRealCredentials(t *testing.T) {
	body := json.RawMessage(`{"id":"live-1","join_tokens":["abc123"],"session_id":"sess-should-redact"}`)
	redacted, err := RedactProviderPayload(body)
	if err != nil {
		t.Fatalf("RedactProviderPayload: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(redacted, &got); err != nil {
		t.Fatalf("unmarshal redacted payload: %v", err)
	}
	if _, ok := got["join_tokens"].([]any); !ok {
		t.Fatalf("join_tokens was redacted, want it preserved: %v", got["join_tokens"])
	}
	if got["session_id"] != "[REDACTED]" {
		t.Fatalf("session_id = %v, want [REDACTED] (real credential-shaped field must still be redacted)", got["session_id"])
	}
	if got["id"] != "live-1" {
		t.Fatalf("id = %v, want unchanged \"live-1\"", got["id"])
	}
}

// TestListProviderFacts_OrdersWorkoutsByStartTimeNotFetchOrder guards MINOR
// #7 from a live post-fix verification sweep: concurrent sync workers write
// rows in whatever order their HTTP requests happen to complete, not the
// order the underlying workouts were actually recorded in, so ordering
// `offline history` by fetched_at (write time) scrambled the listing —
// e.g. an older workout that happened to sync last would appear before a
// newer one that synced first. ListProviderFacts must order "workouts" by
// the payload's own start_time field, newest first, regardless of the order
// the rows were written in.
func TestListProviderFacts_OrdersWorkoutsByStartTimeNotFetchOrder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	// Write oldest-by-start_time first, so a fetched_at-ordered query would
	// return them in write order (w-old, w-mid, w-new) instead of the
	// correct newest-first chronological order (w-new, w-mid, w-old).
	if err := s.UpsertWithFacts("workouts", "w-old", json.RawMessage(`{"id":"w-old","start_time":1000}`)); err != nil {
		t.Fatalf("UpsertWithFacts w-old: %v", err)
	}
	if err := s.UpsertWithFacts("workouts", "w-mid", json.RawMessage(`{"id":"w-mid","start_time":2000}`)); err != nil {
		t.Fatalf("UpsertWithFacts w-mid: %v", err)
	}
	if err := s.UpsertWithFacts("workouts", "w-new", json.RawMessage(`{"id":"w-new","start_time":3000}`)); err != nil {
		t.Fatalf("UpsertWithFacts w-new: %v", err)
	}

	facts, err := s.ListProviderFacts("workouts", 0)
	if err != nil {
		t.Fatalf("ListProviderFacts: %v", err)
	}
	if len(facts) != 3 {
		t.Fatalf("got %d facts, want 3", len(facts))
	}
	var gotOrder []string
	for _, f := range facts {
		gotOrder = append(gotOrder, f.ProviderID)
	}
	want := []string{"w-new", "w-mid", "w-old"}
	if gotOrder[0] != want[0] || gotOrder[1] != want[1] || gotOrder[2] != want[2] {
		t.Fatalf("ListProviderFacts order = %v, want newest-start_time-first %v (fetch/write order was old,mid,new)", gotOrder, want)
	}
}

// TestListProviderFacts_UnaffectedFamiliesKeepFetchOrder guards that the
// start_time ordering fix is scoped to "workouts" only: a family with no
// providerFactDateField entry (and no start_time field at all) must keep
// its original fetched_at-based ordering rather than erroring or silently
// reordering on a COALESCE fallback to 0 for every row.
func TestListProviderFacts_UnaffectedFamiliesKeepFetchOrder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	if err := s.UpsertWithFacts("classes", "c1", json.RawMessage(`{"id":"c1","title":"Ride A"}`)); err != nil {
		t.Fatalf("UpsertWithFacts c1: %v", err)
	}
	if err := s.UpsertWithFacts("classes", "c2", json.RawMessage(`{"id":"c2","title":"Ride B"}`)); err != nil {
		t.Fatalf("UpsertWithFacts c2: %v", err)
	}

	facts, err := s.ListProviderFacts("classes", 0)
	if err != nil {
		t.Fatalf("ListProviderFacts: %v", err)
	}
	if len(facts) != 2 {
		t.Fatalf("got %d facts, want 2", len(facts))
	}
	// c2 was written after c1, so newest-fetched-first means c2, then c1.
	if facts[0].ProviderID != "c2" || facts[1].ProviderID != "c1" {
		t.Fatalf("ListProviderFacts order = [%s, %s], want [c2, c1] (fetched_at DESC)", facts[0].ProviderID, facts[1].ProviderID)
	}
}
