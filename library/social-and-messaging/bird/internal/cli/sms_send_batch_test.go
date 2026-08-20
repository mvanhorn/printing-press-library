// Copyright 2026 Stephan Stoeber and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestApplyTemplate(t *testing.T) {
	header := []string{"name", "code"}
	rec := []string{"Alex", "1234"}
	got := applyTemplate("Hi {{name}}, code {{code}}", header, rec)
	want := "Hi Alex, code 1234"
	if got != want {
		t.Errorf("applyTemplate = %q, want %q", got, want)
	}
}

func TestApplyTemplateUnknownPlaceholder(t *testing.T) {
	header := []string{"name"}
	rec := []string{"Alex"}
	got := applyTemplate("Hello {{name}} {{missing}}", header, rec)
	// Unknown placeholders pass through verbatim.
	want := "Hello Alex {{missing}}"
	if got != want {
		t.Errorf("applyTemplate = %q, want %q", got, want)
	}
}

func TestRowIdempotencyKey_Deterministic(t *testing.T) {
	a := rowIdempotencyKey("batch_1", "+31612345678", "Hello Alex")
	b := rowIdempotencyKey("batch_1", "+31612345678", "Hello Alex")
	if a != b {
		t.Errorf("idempotency key not deterministic: %q vs %q", a, b)
	}
	c := rowIdempotencyKey("batch_2", "+31612345678", "Hello Alex")
	if a == c {
		t.Errorf("idempotency key collided across batches: %q", a)
	}
	d := rowIdempotencyKey("batch_1", "+31612345678", "Hello Bob")
	if a == d {
		t.Errorf("idempotency key collided across bodies: %q", a)
	}
}
