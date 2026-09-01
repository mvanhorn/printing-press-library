package ocr

import "testing"

func TestAssertContainsFailsClosedOnLowConfidence(t *testing.T) {
	_, err := AssertContains([]Word{{Text: "pve2", Confidence: 29, X: 10, Y: 10, Width: 20, Height: 10}}, "pve2", 100, 100)
	if err == nil {
		t.Fatal("expected low confidence rejection")
	}
}

func TestAssertContainsRejectsAmbiguousMatches(t *testing.T) {
	_, err := AssertContains([]Word{
		{Text: "login", Confidence: 90, X: 1, Y: 1, Width: 20, Height: 10},
		{Text: "login", Confidence: 90, X: 50, Y: 1, Width: 20, Height: 10},
	}, "login", 100, 100)
	if err == nil {
		t.Fatal("expected ambiguity rejection")
	}
}

func TestAssertContainsValidatesBounds(t *testing.T) {
	_, err := AssertContains([]Word{{Text: "ok", Confidence: 90, X: 95, Y: 1, Width: 20, Height: 10}}, "ok", 100, 100)
	if err == nil {
		t.Fatal("expected bounds rejection")
	}
}

func TestAssertContainsReturnsCenter(t *testing.T) {
	m, err := AssertContains([]Word{{Text: "PVE2", Confidence: 90, X: 10, Y: 20, Width: 30, Height: 10}}, "pve2", 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if m.Pixel != [2]int{25, 25} {
		t.Fatalf("center=%v", m.Pixel)
	}
}
