// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package ondata

import (
	"strings"
	"testing"
)

func TestNormalizeNumericForms(t *testing.T) {
	cases := []struct {
		in   string
		pad  int
		want []string
	}{
		{"508", 4, []string{"508", "0508"}},
		{"0508", 4, []string{"0508", "508"}},
		{"B", 0, []string{"B"}},
		{"", 4, []string{""}},
		{"  12  ", 0, []string{"12"}},
	}
	for _, c := range cases {
		got := normalizeNumericForms(c.in, c.pad)
		if len(got) != len(c.want) {
			t.Errorf("normalizeNumericForms(%q,%d) = %v; want %v", c.in, c.pad, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("normalizeNumericForms(%q,%d)[%d] = %q; want %q", c.in, c.pad, i, got[i], c.want[i])
			}
		}
	}
}

func TestAnyEqual(t *testing.T) {
	if !anyEqual("0508", []string{"508", "0508"}) {
		t.Error("expected 0508 to match")
	}
	if !anyEqual("b", []string{"B"}) {
		t.Error("expected case-insensitive match")
	}
	if anyEqual("999", []string{"508", "0508"}) {
		t.Error("expected no match")
	}
}

func TestAppendUnique(t *testing.T) {
	got := appendUnique([]string{"a", "b"}, "a")
	if len(got) != 2 {
		t.Errorf("appendUnique should dedupe; got %v", got)
	}
	got = appendUnique([]string{"a"}, "b")
	if len(got) != 2 || got[1] != "b" {
		t.Errorf("appendUnique should append new; got %v", got)
	}
}

func TestParcelRowCoords(t *testing.T) {
	r := ParcelRow{X: 12_492_405, Y: 41_890_252}
	if r.Lon() < 12.49 || r.Lon() > 12.50 {
		t.Errorf("Lon() = %v; want ~12.4924", r.Lon())
	}
	if r.Lat() < 41.88 || r.Lat() > 41.90 {
		t.Errorf("Lat() = %v; want ~41.8902", r.Lat())
	}
}

func TestMissingGeometryMessageDistinguishesRecordsFromMap(t *testing.T) {
	got := missingGeometryMessage("G273", "35", "1900", 1530, "19_Sicilia.parquet", []string{"190", "191"})
	for _, want := range []string{
		"not represented in this geometry dataset",
		"may still exist in Catasto Fabbricati or in official records",
		"nearest represented particelle: 190, 191",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missingGeometryMessage() = %q, want substring %q", got, want)
		}
	}
}
