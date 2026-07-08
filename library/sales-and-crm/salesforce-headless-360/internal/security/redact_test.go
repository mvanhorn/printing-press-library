package security

import "testing"

func TestMarkRedactedUsesConsistentMarker(t *testing.T) {
	fields := map[string]any{"Email": "a@example.com"}
	MarkRedacted(fields, "Email", "PII")
	if got := fields["Email"].(RedactionMarker)["redacted"]; got != "PII" {
		t.Fatalf("marker reason = %q, want PII", got)
	}
}

func TestProvenanceCounterInitializesMapAndCounters(t *testing.T) {
	prov := &Provenance{}
	ProvenanceCounter(prov, ReasonShieldEncrypted)
	if prov.Redactions[ReasonShieldEncrypted] != 1 || prov.Counters.Shield != 1 {
		t.Fatalf("provenance = %#v, want shield count", prov)
	}
}
