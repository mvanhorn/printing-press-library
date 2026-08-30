// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseQCLocation(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		ok                bool
	}{
		{"canonicalizes coordinates", "12.9021,77.6639", "12.902100,77.663900", true},
		{"rejects missing longitude", "12.9021", "", false},
		{"rejects invalid latitude", "91,77", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, got, err := parseQCLocation(tc.input)
			if (err == nil) != tc.ok || got != tc.want {
				t.Fatalf("parseQCLocation(%q) = %q, %v", tc.input, got, err)
			}
		})
	}
}

func TestQCPackNormalizesUnits(t *testing.T) {
	for _, tc := range []struct {
		input, unit string
		base        float64
		ok          bool
	}{
		{"1 L", "ml", 1000, true}, {"500 g", "g", 500, true}, {"2 pack", "pack", 2, true}, {"unknown", "", 0, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			_, unit, base, ok := qcPack(tc.input)
			if ok != tc.ok || unit != tc.unit || base != tc.base {
				t.Fatalf("qcPack(%q) = %q %v %v", tc.input, unit, base, ok)
			}
		})
	}
}
