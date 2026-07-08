package agent

import "testing"

func TestNewUpsertWriteOptionsRequiresIdempotencyInValidation(t *testing.T) {
	opts := NewUpsertWriteOptions("Task", "", map[string]any{"Subject": "Call"})
	opts.Client = noopWriteClient{}
	opts.Signer = noopWriteSigner{}

	if err := validateWriteOptions(normalizeWriteOptions(opts)); err == nil {
		t.Fatal("expected missing idempotency key to fail")
	}
}
