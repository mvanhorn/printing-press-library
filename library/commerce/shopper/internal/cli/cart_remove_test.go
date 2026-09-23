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
// `quantity` body field, `--quantity N` has to issue N calls. An explicit
// quantity below 1 is rejected before any request — leaving it as one call
// would still remove a unit. An unset flag still removes one.
func TestRemoveRepeatCount(t *testing.T) {
	cases := []struct {
		name    string
		stdin   bool
		set     bool
		qty     int
		want    int
		wantErr bool
	}{
		{"three units", false, true, 3, 3, false},
		{"one unit", false, true, 1, 1, false},
		{"unset flag removes one", false, false, 0, 1, false},
		{"explicit zero is rejected", false, true, 0, 0, true},
		{"explicit negative is rejected", false, true, -2, 0, true},
		{"stdin zero is one verbatim call", true, true, 0, 1, false},
		{"stdin negative is one verbatim call", true, true, -2, 1, false},
		{"stdin large quantity is still one call", true, false, 9, 1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := cartRemoveRepeatCount(c.stdin, c.set, c.qty)
			if c.wantErr {
				if err == nil {
					t.Fatalf("cartRemoveRepeatCount(stdin=%v, set=%v, qty=%d) = %d, nil; want error", c.stdin, c.set, c.qty, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("cartRemoveRepeatCount(stdin=%v, set=%v, qty=%d) error = %v", c.stdin, c.set, c.qty, err)
			}
			if got != c.want {
				t.Fatalf("cartRemoveRepeatCount(stdin=%v, set=%v, qty=%d) = %d, want %d", c.stdin, c.set, c.qty, got, c.want)
			}
		})
	}
}
