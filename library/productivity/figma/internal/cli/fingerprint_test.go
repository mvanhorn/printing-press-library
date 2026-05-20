// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestCanonicalize_OrderInvariant(t *testing.T) {
	m1 := fingerprintManifest{
		Variables: []canonicalEntry{
			{ID: "b", Name: "B", Value: map[string]any{"x": 1, "y": 2}},
			{ID: "a", Name: "A", Value: map[string]any{"x": 1}},
		},
		Components: []canonicalEntry{{ID: "c2"}, {ID: "c1"}},
		Styles:     []canonicalEntry{{ID: "s2"}, {ID: "s1"}},
	}
	m2 := fingerprintManifest{
		Variables: []canonicalEntry{
			{ID: "a", Name: "A", Value: map[string]any{"x": 1}},
			{ID: "b", Name: "B", Value: map[string]any{"y": 2, "x": 1}}, // value keys reversed
		},
		Components: []canonicalEntry{{ID: "c1"}, {ID: "c2"}},
		Styles:     []canonicalEntry{{ID: "s1"}, {ID: "s2"}},
	}
	a := canonicalize(m1)
	b := canonicalize(m2)
	if string(a) != string(b) {
		t.Errorf("canonicalize must be order-invariant\n a=%s\n b=%s", a, b)
	}
}

func TestCanonicalize_DistinguishesValues(t *testing.T) {
	m1 := fingerprintManifest{
		Variables: []canonicalEntry{{ID: "v1", Name: "color/x", Value: "#000"}},
	}
	m2 := fingerprintManifest{
		Variables: []canonicalEntry{{ID: "v1", Name: "color/x", Value: "#fff"}},
	}
	if string(canonicalize(m1)) == string(canonicalize(m2)) {
		t.Errorf("different values must hash differently")
	}
}
