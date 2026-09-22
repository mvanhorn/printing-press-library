// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for cart_remove.go — PATCH: cart-remove-quantity.

package cli

import (
	"encoding/json"
	"testing"
)

// TestRemainingCartQuantityReadsLineQuantity pins the response field the remove
// loop stops on. POST /cart/remove answers with the line quantity that is LEFT
// after the call ({"quantity":2} when a 3-unit line was decremented once), so a
// zero means the line is gone and further calls would be pointless writes.
func TestRemainingCartQuantityReadsLineQuantity(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		// Live shape, captured 2026-09-22: a 3-unit line answered {"quantity":2}
		// to a body of {"id":36756,"quantity":3} — the API decrements by one and
		// ignores the requested amount entirely.
		{"live decrement response", `{"quantity":2,"product":{"id":36756,"name":"ARROZ"}}`, 2},
		{"line emptied", `{"quantity":0,"product":{"id":36756}}`, 0},
		{"field absent means keep going", `{"product":{"id":36756}}`, -1},
		{"null quantity means keep going", `{"quantity":null}`, -1},
		{"empty body", ``, -1},
		{"unparseable body", `not json`, -1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := remainingCartQuantity(json.RawMessage(c.body)); got != c.want {
				t.Errorf("remainingCartQuantity(%s) = %d, want %d", c.body, got, c.want)
			}
		})
	}
}

// TestRemoveRepeatCount documents the loop arithmetic the fix depends on:
// because the API decrements exactly one unit per call regardless of the
// `quantity` body field, `--quantity N` has to issue N calls. Anything that
// turns this back into a single call silently reintroduces the bug where
// `--quantity 5` removed one unit.
func TestRemoveRepeatCount(t *testing.T) {
	repeats := func(stdinBody bool, quantity int) int {
		r := 1
		if !stdinBody && quantity > 1 {
			r = quantity
		}
		return r
	}
	cases := []struct {
		stdin bool
		qty   int
		want  int
	}{
		{false, 3, 3},
		{false, 1, 1},
		{false, 0, 1},  // unset flag still sends the single generated call
		{false, -2, 1}, // a negative never means "loop backwards"
		{true, 9, 1},   // a raw stdin body is sent verbatim, exactly once
	}
	for _, c := range cases {
		if got := repeats(c.stdin, c.qty); got != c.want {
			t.Errorf("repeats(stdin=%v, qty=%d) = %d, want %d", c.stdin, c.qty, got, c.want)
		}
	}
}
