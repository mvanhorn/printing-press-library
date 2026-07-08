package agent

import "testing"

func TestNewCreateWriteOptionsSetsOperationAndKey(t *testing.T) {
	opts := NewCreateWriteOptions("Event", "event-1", map[string]any{"Subject": "Demo"})
	if opts.Operation != WriteOperationCreate {
		t.Fatalf("operation = %q", opts.Operation)
	}
	if opts.SObject != "Event" {
		t.Fatalf("sobject = %q", opts.SObject)
	}
	if opts.IdempotencyKey != "event-1" {
		t.Fatalf("key = %q", opts.IdempotencyKey)
	}
}
