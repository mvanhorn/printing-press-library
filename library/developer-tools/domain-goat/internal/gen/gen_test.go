package gen

import (
	"strings"
	"testing"
)

func TestPair(t *testing.T) {
	got := Pair([]string{"kindred"}, []string{"com", "io", ".ai"})
	if len(got) != 3 {
		t.Fatalf("want 3, got %d (%v)", len(got), got)
	}
	for _, want := range []string{"kindred.com", "kindred.io", "kindred.ai"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s in %v", want, got)
		}
	}
}

func TestAffix(t *testing.T) {
	got := Affix("brand", nil, nil)
	if len(got) == 0 {
		t.Fatal("expected non-empty")
	}
	hasGetPrefix := false
	for _, g := range got {
		if strings.HasPrefix(g, "get") {
			hasGetPrefix = true
		}
	}
	if !hasGetPrefix {
		t.Error("expected at least one get-prefix variant")
	}
}

func TestBlend(t *testing.T) {
	got := Blend("snap", "apple")
	found := false
	for _, g := range got {
		if g == "snapple" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected snapple in blend(snap, apple), got %v", got)
	}
}

func TestHack(t *testing.T) {
	got := Hack("kubes")
	found := false
	for _, g := range got {
		if g == "kub.es" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected kub.es in hack(kubes), got %v", got)
	}
}

func TestPermuteOmission(t *testing.T) {
	got := Permute("kindred.io", []string{"omission"})
	if len(got) == 0 {
		t.Fatal("expected omission permutations")
	}
	for _, g := range got {
		if g.Kind != "omission" {
			t.Errorf("kind=%s, want omission", g.Kind)
		}
	}
}

func TestPermuteTLDSwap(t *testing.T) {
	got := Permute("kindred.io", []string{"tld-swap"})
	if len(got) == 0 {
		t.Fatal("expected tld swaps")
	}
	for _, g := range got {
		if g.FQDN == "kindred.io" {
			t.Errorf("self-swap leaked")
		}
	}
}
