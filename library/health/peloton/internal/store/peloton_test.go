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
