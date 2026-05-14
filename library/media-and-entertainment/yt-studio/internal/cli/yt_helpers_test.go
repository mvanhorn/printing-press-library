package cli

import (
	"testing"
)

func TestAsciiSparkline_Empty(t *testing.T) {
	t.Parallel()
	got := asciiSparkline(nil, 40)
	if got == "" {
		t.Fatal("expected non-empty string for empty input")
	}
}

func TestAsciiSparkline_RisingThenFalling(t *testing.T) {
	t.Parallel()
	// 0.5 ramping to 1.0 ramping back to 0.0 — visually a hill
	pts := make([]float64, 100)
	for i := 0; i < 50; i++ {
		pts[i] = 0.5 + 0.01*float64(i)
	}
	for i := 50; i < 100; i++ {
		pts[i] = 1.0 - 0.02*float64(i-50)
	}
	out := asciiSparkline(pts, 80)
	if len([]rune(out)) != 80 {
		t.Fatalf("expected 80 runes, got %d", len([]rune(out)))
	}
}

func TestFindSharpestDrops_OrdersByMagnitude(t *testing.T) {
	t.Parallel()
	// craft a curve with two obvious drops
	pts := []float64{1.0, 1.0, 0.5, 0.5, 0.2, 0.2}
	drops := findSharpestDrops(pts, 3)
	if len(drops) < 2 {
		t.Fatalf("expected at least 2 drops, got %d", len(drops))
	}
	if drops[0].DropMagnitude < drops[1].DropMagnitude {
		t.Fatalf("expected drops sorted by magnitude desc, got %v", drops)
	}
	// largest drop should be from 1.0 to 0.5 (index 2) — magnitude 0.5
	if drops[0].BeforeRatio != 1.0 || drops[0].AfterRatio != 0.5 {
		t.Errorf("largest drop should be 1.0→0.5, got %v", drops[0])
	}
}

func TestFindSharpestDrops_FlatCurve(t *testing.T) {
	t.Parallel()
	pts := []float64{0.8, 0.8, 0.8, 0.8}
	drops := findSharpestDrops(pts, 3)
	if len(drops) != 0 {
		t.Fatalf("flat curve should produce no drops, got %d", len(drops))
	}
}

func TestTokenizeTitle_StripsStopWordsAndShort(t *testing.T) {
	t.Parallel()
	got := tokenizeTitle("The Best PoE2 Build Guide of 2026")
	// "the" is a stop-word, "of" too short, "2026" stays
	want := map[string]bool{"best": true, "poe2": true, "build": true, "guide": true, "2026": true}
	for _, tok := range got {
		if !want[tok] {
			t.Errorf("unexpected token %q", tok)
		}
		delete(want, tok)
	}
	if len(want) > 0 {
		t.Errorf("missing expected tokens: %v", want)
	}
}

func TestParsePeriodDays(t *testing.T) {
	t.Parallel()
	cases := map[string]int{
		"7d":   7,
		"28d":  28,
		"4w":   28,
		"":     28,
		"junk": 28,
	}
	for in, want := range cases {
		if got := parsePeriodDays(in); got != want {
			t.Errorf("parsePeriodDays(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestOneLine_TruncatesLong(t *testing.T) {
	t.Parallel()
	long := ""
	for i := 0; i < 150; i++ {
		long += "x"
	}
	got := oneLine(long)
	if len([]rune(got)) > 101 { // 97 chars + "…" + safety
		t.Fatalf("expected truncation, got len=%d", len([]rune(got)))
	}
	if oneLine("") != "(missing)" {
		t.Errorf("empty should yield (missing)")
	}
}
