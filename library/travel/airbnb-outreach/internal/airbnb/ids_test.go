// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import "testing"

// All IDs here are synthetic (not real account data); the numeric values are
// deliberately not 10 digits so they don't resemble a phone number.
func TestEncodeDecodeGlobalID(t *testing.T) {
	cases := []struct{ typ, id string }{
		{"MessageThread", "980001234567"},
		{"Viewer", "980007654321"},
		{"StayListing", "12345678"},
	}
	for _, c := range cases {
		enc := EncodeGlobalID(c.typ, c.id)
		typ, id, ok := DecodeGlobalID(enc)
		if !ok || typ != c.typ || id != c.id {
			t.Errorf("roundtrip %s:%s -> %q -> (%q,%q,%v)", c.typ, c.id, enc, typ, id, ok)
		}
	}
}

func TestDecodeKnownGlobalIDs(t *testing.T) {
	// base64 of "<type>:<synthetic id>", the same encoding Airbnb uses.
	cases := map[string]struct{ typ, id string }{
		"TWVzc2FnZVRocmVhZDo5ODAwMDEyMzQ1Njc=": {"MessageThread", "980001234567"},
		"RGVtYW5kU3RheUxpc3Rpbmc6MTIzNDU2Nzg=": {"DemandStayListing", "12345678"},
		"U3RheUxpc3Rpbmc6MTIzNDU2Nzg=":         {"StayListing", "12345678"},
	}
	for enc, want := range cases {
		typ, id, ok := DecodeGlobalID(enc)
		if !ok || typ != want.typ || id != want.id {
			t.Errorf("DecodeGlobalID(%q) = (%q,%q,%v), want (%q,%q,true)", enc, typ, id, ok, want.typ, want.id)
		}
	}
}

func TestNormalizeThreadID(t *testing.T) {
	// Numeric input gets encoded; already-encoded input is unchanged.
	got := NormalizeThreadID("980001234567")
	if _, id, ok := DecodeGlobalID(got); !ok || id != "980001234567" {
		t.Errorf("NormalizeThreadID(numeric) = %q, decode gave id=%q ok=%v", got, id, ok)
	}
	already := "TWVzc2FnZVRocmVhZDo5ODAwMDEyMzQ1Njc="
	if NormalizeThreadID(already) != already {
		t.Errorf("NormalizeThreadID(encoded) changed the value")
	}
}

func TestNumericID(t *testing.T) {
	if got := NumericID("RGVtYW5kU3RheUxpc3Rpbmc6MTIzNDU2Nzg="); got != "12345678" {
		t.Errorf("NumericID(encoded) = %q, want 12345678", got)
	}
	if got := NumericID("12345678"); got != "12345678" {
		t.Errorf("NumericID(numeric) = %q, want 12345678", got)
	}
}
