package ocr

import (
	"testing"
)

type fakeEngine struct {
	w, h  int
	words []Word
}

func (f *fakeEngine) Recognize(_ []byte) (int, int, []Word, error) {
	return f.w, f.h, f.words, nil
}

func TestAnalyzeFiltersAndSorts(t *testing.T) {
	words := []Word{
		{Text: "login", Confidence: 90, X: 10, Y: 1, Width: 20, Height: 10},
		{Text: "login", Confidence: 80, X: 50, Y: 1, Width: 20, Height: 10},
		{Text: "ignored", Confidence: 29, X: 1, Y: 1, Width: 10, Height: 10},
		{Text: "  Login ", Confidence: 95, X: 10, Y: 20, Width: 30, Height: 10},
	}
	a, err := Analyze(words, 100, 100, "login")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Elements) != 3 {
		t.Fatalf("elements %d", len(a.Elements))
	}
	if a.Elements[0].Confidence != 95 {
		t.Fatalf("sorted %v", a.Elements)
	}
	if a.Elements[0].Pixel != [2]int{25, 25} {
		t.Fatalf("pixel %v", a.Elements[0].Pixel)
	}
	if a.Elements[0].XPct != 25.0 || a.Elements[0].YPct != 25.0 {
		t.Fatalf("pct %v", a.Elements[0])
	}
}

func TestAnalyzeConfidenceThresholdAndSearch(t *testing.T) {
	words := []Word{{Text: "pve2", Confidence: 29, X: 10, Y: 10, Width: 20, Height: 10}}
	a, err := Analyze(words, 100, 100, "pve2")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Elements) != 0 {
		t.Fatal("should filter low confidence")
	}
	words = []Word{{Text: "other", Confidence: 90, X: 1, Y: 1, Width: 10, Height: 10}}
	a, _ = Analyze(words, 100, 100, "pve2")
	if len(a.Elements) != 0 {
		t.Fatal("should filter non-matching text")
	}
}

func TestAnalyzeBoundsValidation(t *testing.T) {
	words := []Word{{Text: "ok", Confidence: 90, X: 95, Y: 1, Width: 20, Height: 10}}
	if _, err := Analyze(words, 100, 100, ""); err == nil {
		t.Fatal("expected bounds error")
	}
}

func TestAnalyzeEmptySearchReturnsAll(t *testing.T) {
	words := []Word{
		{Text: "a", Confidence: 90, X: 1, Y: 1, Width: 10, Height: 10},
		{Text: "b", Confidence: 90, X: 20, Y: 1, Width: 10, Height: 10},
	}
	a, err := Analyze(words, 100, 100, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Elements) != 2 {
		t.Fatalf("expected 2 got %d", len(a.Elements))
	}
}

func TestClickForMatch(t *testing.T) {
	words := []Word{{Text: "PVE2", Confidence: 90, X: 10, Y: 20, Width: 30, Height: 10}}
	a, _ := Analyze(words, 100, 100, "pve2")
	click, err := ClickForMatch(a.Elements[0], 100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if click.X != 25 || click.Y != 25 {
		t.Fatalf("click %v", click)
	}
	if click.XPct != 25.0 {
		t.Fatalf("xpct %v", click)
	}
}

func TestAnalyzeImageWithEngine(t *testing.T) {
	eng := &fakeEngine{w: 200, h: 100, words: []Word{{Text: "hello", Confidence: 80, X: 10, Y: 10, Width: 20, Height: 10}}}
	a, err := AnalyzeImage([]byte("fake"), "hello", eng)
	if err != nil {
		t.Fatal(err)
	}
	if a.Width != 200 || len(a.Elements) != 1 {
		t.Fatalf("analysis %v", a)
	}
}
