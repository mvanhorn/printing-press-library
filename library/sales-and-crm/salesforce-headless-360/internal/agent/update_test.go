package agent

import "testing"

func TestNewUpdateWriteOptionsInfersSObject(t *testing.T) {
	opts := NewUpdateWriteOptions("001ACME0001", map[string]any{"Industry": "Fintech"})
	if opts.Operation != WriteOperationUpdate {
		t.Fatalf("operation = %q", opts.Operation)
	}
	if opts.SObject != "Account" {
		t.Fatalf("sobject = %q, want Account", opts.SObject)
	}
	if opts.RecordID != "001ACME0001" {
		t.Fatalf("record id = %q", opts.RecordID)
	}
}
