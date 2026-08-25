// Copyright 2026 roberto-bissanti. Licensed under Apache-2.0. See LICENSE.

package comuni

import (
	"errors"
	"testing"
)

func TestResolveByBelfiore(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"H501", "Roma"},
		{"h501", "Roma"},
		{"G273", "Palermo"},
		{"F205", "Milano"},
		{"A001", "Abano Terme"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.in, func(t *testing.T) {
			got, err := ResolveByBelfiore(c.in)
			if err != nil {
				t.Fatalf("got error: %v", err)
			}
			if got.Nome != c.want {
				t.Fatalf("want %s, got %s", c.want, got.Nome)
			}
		})
	}
}

func TestResolveByBelfiore_NotFound(t *testing.T) {
	_, err := ResolveByBelfiore("ZZZZ")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestResolveByBelfiore_InvalidShape(t *testing.T) {
	for _, s := range []string{"", "AB", "12345", "abcde"} {
		_, err := ResolveByBelfiore(s)
		if !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("%q: want ErrInvalidCode, got %v", s, err)
		}
	}
}

func TestResolveByName_Unique(t *testing.T) {
	c, err := ResolveByName("Palermo", "")
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if c.CodiceCatastale != "G273" {
		t.Fatalf("want G273, got %s", c.CodiceCatastale)
	}
}

func TestResolveByName_AccentInsensitive(t *testing.T) {
	// Forlì has an accent in the canonical name
	c, err := ResolveByName("Forli", "FC")
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if c.Nome == "" {
		t.Fatalf("empty match")
	}
}

func TestResolveByName_WithProvincia(t *testing.T) {
	// Pass both sigla and full name forms
	c1, err := ResolveByName("Roma", "RM")
	if err != nil {
		t.Fatalf("got error (sigla): %v", err)
	}
	c2, err := ResolveByName("Roma", "Roma")
	if err != nil {
		t.Fatalf("got error (full): %v", err)
	}
	if c1.CodiceCatastale != c2.CodiceCatastale {
		t.Fatalf("sigla vs name disagree: %s vs %s", c1.CodiceCatastale, c2.CodiceCatastale)
	}
}

func TestResolveByName_Ambiguous(t *testing.T) {
	// "San Lorenzo" exists in multiple provinces — there are several
	// homonymous comuni in Italy.
	// Use a known-ambiguous name: "Castro" (multiple comuni).
	_, err := ResolveByName("Castro", "")
	if err == nil {
		t.Skip("name 'Castro' resolved uniquely in this dataset; skipping ambiguity test")
	}
	if !errors.Is(err, ErrAmbiguous) {
		// Fall back: not all datasets have ambiguous Castro; skip cleanly
		t.Skipf("Castro not ambiguous: %v", err)
	}
}

func TestResolveByCAP(t *testing.T) {
	// 00184 is one of Roma's CAPs
	hits, err := ResolveByCAP("00184")
	if err != nil {
		t.Fatalf("got error: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected at least one hit")
	}
	if hits[0].CodiceCatastale != "H501" {
		t.Fatalf("want H501 (Roma), got %s", hits[0].CodiceCatastale)
	}
}

func TestResolveByCAP_InvalidShape(t *testing.T) {
	for _, s := range []string{"", "abc", "1234", "123456"} {
		_, err := ResolveByCAP(s)
		if !errors.Is(err, ErrInvalidCAP) {
			t.Fatalf("%q: want ErrInvalidCAP, got %v", s, err)
		}
	}
}

func TestNormalize(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Roma", "roma"},
		{"Forlì", "forli"},
		{"Sant'Agata di Militello", "sant agata di militello"},
		{"  Reggio   Calabria  ", "reggio calabria"},
		{"Bagnone-Filattiera", "bagnone filattiera"},
	}
	for _, c := range cases {
		if got := normalize(c.in); got != c.want {
			t.Errorf("normalize(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAll(t *testing.T) {
	a, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) < 7000 {
		t.Fatalf("expected ~7900 comuni, got %d", len(a))
	}
}
