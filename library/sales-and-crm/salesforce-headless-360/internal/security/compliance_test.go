package security

import (
	"context"
	"errors"
	"testing"
)

type memoryComplianceStore struct {
	fields []ComplianceField
}

func (s *memoryComplianceStore) SaveComplianceFields(fields []ComplianceField) error {
	s.fields = append(s.fields, fields...)
	return nil
}

func (s *memoryComplianceStore) ComplianceFields(sobject string) ([]ComplianceField, error) {
	var out []ComplianceField
	for _, field := range s.fields {
		if field.SObject == sobject {
			out = append(out, field)
		}
	}
	return out, nil
}

func TestComplianceRedactsTaggedFieldsAndCountsGroups(t *testing.T) {
	store := &memoryComplianceStore{fields: []ComplianceField{
		{SObject: "Contact", FieldAPIName: "Email", ComplianceGroup: "PII,GLBA"},
	}}
	filter := NewComplianceFilter(nil, store)
	record := &Record{SObject: "Contact", Fields: map[string]any{"Id": "003", "Email": "avery@example.com"}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := filter.Apply(context.Background(), record)
	marker, ok := got.Fields["Email"].(RedactionMarker)
	if !ok || marker["redacted"] != "PII" {
		t.Fatalf("Email marker = %#v, want PII redaction", got.Fields["Email"])
	}
	if got.Provenance.Redactions["PII"] != 1 || got.Provenance.Redactions["GLBA"] != 1 {
		t.Fatalf("redactions = %#v, want PII and GLBA counted", got.Provenance.Redactions)
	}
}

func TestComplianceShieldEncryptedUsesShieldReason(t *testing.T) {
	store := &memoryComplianceStore{fields: []ComplianceField{
		{SObject: "Contact", FieldAPIName: "Salary__c", ComplianceGroup: "GLBA", IsEncrypted: true},
	}}
	filter := NewComplianceFilter(nil, store)
	record := &Record{SObject: "Contact", Fields: map[string]any{"Id": "003", "Salary__c": 142000}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := filter.Apply(context.Background(), record)
	marker, ok := got.Fields["Salary__c"].(RedactionMarker)
	if !ok || marker["redacted"] != ReasonShieldEncrypted {
		t.Fatalf("Salary__c marker = %#v, want shield redaction", got.Fields["Salary__c"])
	}
	if got.Provenance.Counters.Shield != 1 {
		t.Fatalf("shield counter = %d, want 1", got.Provenance.Counters.Shield)
	}
}

func TestComplianceToolingFailureFailsClosed(t *testing.T) {
	client := &fakeGetter{errs: map[string]error{"/services/data/v63.0/tooling/query": errors.New("tooling unavailable")}}
	filter := NewComplianceFilter(client, nil)
	record := &Record{SObject: "Contact", Fields: map[string]any{"Id": "003", "Email": "avery@example.com"}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := filter.Apply(context.Background(), record)
	marker, ok := got.Fields["Email"].(RedactionMarker)
	if !ok || marker["redacted"] != "PII" {
		t.Fatalf("Email marker = %#v, want fail-closed PII", got.Fields["Email"])
	}
	if got.Provenance.Redactions[ReasonComplianceFailClosed] != 1 {
		t.Fatalf("redactions = %#v, want fail closed provenance", got.Provenance.Redactions)
	}
}

func TestValidateSensitiveOverrideRequiresYesWhenNonTTY(t *testing.T) {
	if err := ValidateSensitiveOverride("--include-pii", true, false, false); err == nil {
		t.Fatal("expected non-TTY include-pii without --yes to fail")
	}
	if err := ValidateSensitiveOverride("--include-pii", true, true, false); err != nil {
		t.Fatalf("with --yes err = %v, want nil", err)
	}
}
