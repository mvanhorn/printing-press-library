// Copyright 2026 greatclips. Licensed under Apache-2.0. See LICENSE.

package icssign

import (
	"encoding/hex"
	"testing"
)

// TestDerivedKey verifies that the seed XOR HMAC derivation matches
// the value the browser's WebCrypto implementation produces for the
// same inputs. The expected hex string was captured by running the
// SPA's exact algorithm in app.greatclips.com page context on
// 2026-05-11.
func TestDerivedKey(t *testing.T) {
	got := hex.EncodeToString(derivedKey())
	want := "c9c4721d6670281b3a23e072b43a534cc57a1dacc7d34182cb95c3735b217d66"
	if got != want {
		t.Fatalf("derived key mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

// TestSignGoldenVector verifies that signing a known timestamp+body
// pair produces the signature the browser computed for the same input.
// This is the load-bearing test for byte-identity with the SPA.
func TestSignGoldenVector(t *testing.T) {
	cases := []struct {
		name string
		t    string
		body string
		want string
	}{
		{
			name: "wait time for Island Square (with = padding, matches SPA)",
			t:    "1778530000000",
			body: `[{"storeNumber":"8991"}]`,
			want: "Y1p7j0qekK28DOLVNF2CkxhGhLvEW9PVtm30sEiMZas=",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Sign(tc.t + tc.body)
			if got != tc.want {
				t.Fatalf("signature mismatch:\n  input: %q\n  got:   %s\n  want:  %s", tc.t+tc.body, got, tc.want)
			}
			if len(got) != 44 {
				t.Fatalf("expected 44-char URL-safe base64 with = padding, got %d chars", len(got))
			}
		})
	}
}

// TestSignDeterministic confirms repeated calls with the same input
// produce the same output (no time-varying state in Sign itself).
func TestSignDeterministic(t *testing.T) {
	input := "1700000000000hello"
	a := Sign(input)
	b := Sign(input)
	if a != b {
		t.Fatalf("non-deterministic signing: %s != %s", a, b)
	}
}

// TestSignDistinct confirms different inputs produce different outputs
// (no constant-output bug).
func TestSignDistinct(t *testing.T) {
	a := Sign("1700000000000body1")
	b := Sign("1700000000000body2")
	if a == b {
		t.Fatalf("distinct inputs produced same signature: %s", a)
	}
}

// TestSignRequest confirms the helper wires timestamp and signature
// the way callers will use them.
func TestSignRequest(t *testing.T) {
	tval, sval := SignRequest("1778530000000", `[{"storeNumber":"8991"}]`)
	if tval != "1778530000000" {
		t.Fatalf("timestamp not passed through: got %q", tval)
	}
	if sval != "Y1p7j0qekK28DOLVNF2CkxhGhLvEW9PVtm30sEiMZas=" {
		t.Fatalf("signature mismatch: got %s", sval)
	}
}
