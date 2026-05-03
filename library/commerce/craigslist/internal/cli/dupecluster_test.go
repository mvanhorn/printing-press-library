package cli

import (
	"testing"
)

func TestSimhash64_identicalText(t *testing.T) {
	a := simhash64("the quick brown fox jumps over the lazy dog")
	b := simhash64("the quick brown fox jumps over the lazy dog")
	if a != b {
		t.Errorf("identical text should produce identical simhash, got %x vs %x", a, b)
	}
}

func TestSimhash64_differentText(t *testing.T) {
	a := simhash64("the quick brown fox")
	b := simhash64("entirely different content here so the hash diverges")
	if a == b {
		t.Errorf("very different text should produce different simhashes, got identical %x", a)
	}
	if hamming64(a, b) <= 8 {
		t.Errorf("hamming distance for unrelated texts should exceed 8, got %d", hamming64(a, b))
	}
}

func TestSimhash64_empty(t *testing.T) {
	if simhash64("") != 0 {
		t.Errorf("empty string should hash to 0, got %x", simhash64(""))
	}
	if simhash64("   \n  ") != 0 {
		t.Errorf("whitespace-only should hash to 0, got %x", simhash64("  "))
	}
}

func TestHamming64_zeroAndAllOnes(t *testing.T) {
	if hamming64(0, 0) != 0 {
		t.Errorf("hamming(0,0) should be 0")
	}
	if hamming64(0xffffffffffffffff, 0) != 64 {
		t.Errorf("hamming(allOnes, 0) should be 64")
	}
	if hamming64(0xff, 0x0f) != 4 {
		t.Errorf("hamming(0xff, 0x0f) should be 4, got %d", hamming64(0xff, 0x0f))
	}
}

func TestBuildDupeClusters_basic(t *testing.T) {
	rows := []listingForCluster{
		{PID: 1, Title: "Apt A", BodyText: "Beautiful one bedroom apartment near downtown with parking included"},
		{PID: 2, Title: "Apt B", BodyText: "Beautiful one bedroom apartment near downtown with parking included"},
		{PID: 3, Title: "Apt C", BodyText: "Beautiful one bedroom apartment near downtown with parking included"},
		{PID: 4, Title: "Car", BodyText: "2015 Honda Civic 80k miles automatic transmission clean title"},
	}
	clusters := buildDupeClusters(rows, 2)
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster of size 3, got %d clusters: %+v", len(clusters), clusters)
	}
	if clusters[0].Size != 3 {
		t.Errorf("cluster size: got %d want 3", clusters[0].Size)
	}
}

func TestBuildDupeClusters_minSizeFilter(t *testing.T) {
	rows := []listingForCluster{
		{PID: 1, BodyText: "alpha beta gamma delta epsilon zeta"},
		{PID: 2, BodyText: "alpha beta gamma delta epsilon zeta"},
		{PID: 3, BodyText: "totally unrelated content here goes nowhere"},
	}
	clusters := buildDupeClusters(rows, 3)
	if len(clusters) != 0 {
		t.Errorf("min-size 3 should drop the 2-pid cluster; got %+v", clusters)
	}
	clusters = buildDupeClusters(rows, 2)
	if len(clusters) != 1 || clusters[0].Size != 2 {
		t.Errorf("expected one cluster of size 2; got %+v", clusters)
	}
}

func TestFNV64_isDeterministic(t *testing.T) {
	if fnv64("hello") != fnv64("hello") {
		t.Errorf("fnv64 should be deterministic")
	}
	if fnv64("hello") == fnv64("world") {
		t.Errorf("fnv64 should differ for distinct inputs")
	}
}
