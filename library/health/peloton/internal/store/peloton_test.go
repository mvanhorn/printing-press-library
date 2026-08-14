package store

import (
	"encoding/json"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

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
