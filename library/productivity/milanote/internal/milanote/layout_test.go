// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import "testing"

// Heights measured from cards rendered by the web app at 100% zoom
// (rendered px + 2px border, divided by the 9px grid).
func TestHeightEstimatesMatchRenderedCards(t *testing.T) {
	long := "The quick brown fox jumps over the lazy dog and keeps running across the field until the sun goes down behind the hills."
	notes := []struct {
		name string
		md   string
		want float64
	}{
		{"one line", "One line", 7},
		{"two paragraphs", "One line\n\nSecond paragraph", 10},
		{"wrapping paragraph", long, 11},
		{"long paragraph", long + " " + long, 17},
		{"h1 then body", "# Big title\nbody", 11},
		{"h2 then body", "## Sub title\nbody", 10},
		{"bullets", "- a\n- b\n- c", 13},
		{"numbers", "1. a\n2. b\n3. c", 13},
		{"short callout", "> [!NOTE] short callout", 9},
		{"long callout", "> [!NOTE] " + long, 15},
		{"code fence", "```\nline1\nline2\n```", 10},
		{"empty", "", 7},
	}
	for _, tc := range notes {
		if got := NoteHeight(MarkdownDoc(tc.md), CardWidth); got != tc.want {
			t.Errorf("note %s: got %v units, want %v", tc.name, got, tc.want)
		}
	}
	headings := []struct {
		level int
		text  string
		want  float64
	}{
		{1, "Heading one", 7},
		{2, "Heading two", 7},
		{1, "A much longer heading that wraps onto lines", 10},
	}
	for _, tc := range headings {
		if got := HeadingHeight(tc.level, tc.text, CardWidth); got != tc.want {
			t.Errorf("heading %q: got %v, want %v", tc.text, got, tc.want)
		}
	}
	tasks := []struct {
		name  string
		items []Task
		want  float64
	}{
		{"one", []Task{{Text: "a"}}, 7},
		{"six", []Task{{Text: "a"}, {Text: "b"}, {Text: "c"}, {Text: "d"}, {Text: "e"}, {Text: "f"}}, 22},
		{"long item", []Task{{Text: long}, {Text: "b"}}, 16},
	}
	for _, tc := range tasks {
		if got := TaskListHeight(tc.items, CardWidth); got != tc.want {
			t.Errorf("tasks %s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestGridRowsAlignTopsAndKeepEqualGaps(t *testing.T) {
	g := Grid{X: 2, Y: 2, Cols: 3, ColW: 34, Gutter: 3}
	slots := g.Rows([]float64{7, 10, 13, 7, 7})
	want := []Slot{{2, 2, 0, 0}, {39, 2, 1, 0}, {76, 2, 2, 0}, {2, 18, 0, 1}, {39, 18, 1, 1}}
	for i, s := range slots {
		if s != want[i] {
			t.Errorf("slot %d: got %+v, want %+v", i, s, want[i])
		}
	}
}

func TestGridMasonryFillsShortestColumn(t *testing.T) {
	g := Grid{X: 0, Y: 0, Cols: 2, ColW: 34, Gutter: 3}
	slots := g.Masonry([]float64{20, 7, 7, 7})
	want := []Slot{{0, 0, 0, 0}, {37, 0, 1, 0}, {37, 10, 1, 1}, {37, 20, 1, 2}}
	for i, s := range slots {
		if s != want[i] {
			t.Errorf("slot %d: got %+v, want %+v", i, s, want[i])
		}
	}
}

func TestPackSpansWideCardsWithoutStretchingColumns(t *testing.T) {
	widths := []float64{71, 34, 34, 34, 34}
	if got := CommonWidth(widths); got != 34 {
		t.Fatalf("CommonWidth = %v", got)
	}
	g := Grid{X: 2, Y: 2, Cols: 2, ColW: 34, Gutter: 3}
	items := []PackItem{{71, 7}, {34, 7}, {34, 10}, {34, 7}, {34, 7}}
	got := g.Pack(items, false)
	want := []Slot{{2, 2, 0, 0}, {2, 12, 0, 1}, {39, 12, 1, 1}, {2, 22, 0, 2}, {39, 25, 1, 2}}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("slot %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
	rows := g.Pack(items, true)
	wantRows := []Slot{{2, 2, 0, 0}, {2, 12, 0, 1}, {39, 12, 1, 1}, {2, 25, 0, 2}, {39, 25, 1, 2}}
	for i := range wantRows {
		if rows[i] != wantRows[i] {
			t.Errorf("row slot %d: got %+v, want %+v", i, rows[i], wantRows[i])
		}
	}
}

func TestCheckFlagsWideColumnHole(t *testing.T) {
	boxes := []Box{
		{ID: "a", X: 2, Y: 2, W: 34, H: 7}, {ID: "b", X: 78, Y: 2, W: 34, H: 7},
		{ID: "c", X: 2, Y: 14, W: 34, H: 7}, {ID: "d", X: 78, Y: 14, W: 34, H: 7},
		{ID: "e", X: 2, Y: 26, W: 34, H: 7}, {ID: "f", X: 78, Y: 26, W: 34, H: 7},
	}
	r := Check(boxes, 2)
	if r.Score >= 100 || r.GapSpread < 30 {
		t.Fatalf("a 42u hole between columns should be reported: %+v", r)
	}
}

func TestCheckFindsOverlapsNearMissesAndUnevenGaps(t *testing.T) {
	tidy := []Box{{ID: "a", X: 0, Y: 0, W: 34, H: 7}, {ID: "b", X: 0, Y: 10, W: 34, H: 7}, {ID: "c", X: 0, Y: 20, W: 34, H: 7}}
	if r := Check(tidy, 2); r.Score != 100 || len(r.Issues) != 0 {
		t.Fatalf("tidy board scored %d with issues %+v", r.Score, r.Issues)
	}
	messy := []Box{
		{ID: "a", X: 0, Y: 0, W: 34, H: 7},
		{ID: "b", X: 1, Y: 10, W: 34, H: 7},  // 1u off a's left edge
		{ID: "c", X: 10, Y: 12, W: 34, H: 7}, // overlaps b
		{ID: "d", X: 0, Y: 40, W: 34, H: 7},  // big gap
	}
	r := Check(messy, 2)
	if r.Overlaps != 1 {
		t.Errorf("overlaps = %d, want 1", r.Overlaps)
	}
	if r.Misaligned == 0 {
		t.Errorf("expected a near-miss edge")
	}
	if r.Score >= 100 {
		t.Errorf("messy board should lose points, got %d", r.Score)
	}
}

func TestWrapLinesBreaksOnWords(t *testing.T) {
	if got := wrapLines("aaaa bbbb cccc", 8*5.9+1, 5.9); got != 3 {
		t.Errorf("got %d lines, want 3", got)
	}
	if got := wrapLines("", 100, 5.9); got != 1 {
		t.Errorf("empty text should still take one line, got %d", got)
	}
}
