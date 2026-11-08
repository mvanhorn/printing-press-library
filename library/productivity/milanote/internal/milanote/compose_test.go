// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

const sampleDoc = `# Short film

Intro line with **bold**.

## Scene 1
- wide shot
  - nested close-up

- [ ] book location
- [x] write script

---

More notes after the rule.

## Scene 2
` + "```" + `
# not a heading
` + "```" + `
`

func TestChunkMarkdown(t *testing.T) {
	chunks := ChunkMarkdown(sampleDoc)
	var kinds []string
	for _, c := range chunks {
		kinds = append(kinds, c.Kind)
	}
	want := "heading note heading note tasks note heading note"
	if got := strings.Join(kinds, " "); got != want {
		t.Fatalf("kinds = %q, want %q", got, want)
	}
	if !strings.Contains(chunks[7].MD, "# not a heading") {
		t.Errorf("fenced heading should stay inside the note: %q", chunks[7].MD)
	}
	tasks := chunks[4].Tasks
	if len(tasks) != 2 || tasks[0].Checked || !tasks[1].Checked {
		t.Errorf("tasks = %+v", tasks)
	}
	lines := strings.Split(sampleDoc, "\n")
	for _, c := range chunks {
		if got := strings.Join(lines[c.Start:c.End], "\n"); strings.TrimSpace(got) != strings.TrimSpace(c.MD) {
			t.Errorf("line range %d-%d = %q, want %q", c.Start, c.End, got, c.MD)
		}
	}
}

func TestNormalizedRoundTripIsStable(t *testing.T) {
	for _, c := range ChunkMarkdown(sampleDoc) {
		once := c.Normalized()
		again := Chunk{Kind: c.Kind, Level: c.Level, Text: c.Text, MD: once, Tasks: c.Tasks}
		if c.Kind == ChunkHeading {
			m := reHeading.FindStringSubmatch(once)
			if m == nil {
				t.Fatalf("heading did not round trip: %q", once)
			}
			again.Text = m[2]
		}
		if got := again.Normalized(); got != once {
			t.Errorf("%s not stable:\n first %q\nsecond %q", c.Kind, once, got)
		}
	}
}

func TestTaskChunkMatchesTaskElement(t *testing.T) {
	chunks := ChunkMarkdown("- [ ] one\n- [x] **two**")
	el := &Element{ElementType: TypeTaskList, Content: TaskListContent("", chunks[0].Tasks)}
	if got, want := ElementMarkdown(el), chunks[0].Normalized(); got != want {
		t.Fatalf("element %q != chunk %q", got, want)
	}
	if !strings.Contains(chunks[0].Normalized(), "- [x] **two**") {
		t.Fatalf("lost tick or bold: %q", chunks[0].Normalized())
	}
}

func TestPlanLayoutEqualGaps(t *testing.T) {
	chunks := ChunkMarkdown(sampleDoc + "\n## Scene 3\nThird section\n")
	plan := PlanLayout(chunks, PlanOpts{X: 2, Y: 2, Cols: 3, ColW: CardWidth, Gutter: 3})
	if plan[0].W != 3*CardWidth+2*3 {
		t.Errorf("title width = %v", plan[0].W)
	}
	byCol := map[float64][]Planned{}
	for _, p := range plan[1:] {
		byCol[p.X] = append(byCol[p.X], p)
	}
	if len(byCol) != 3 {
		t.Fatalf("columns used = %d, want 3", len(byCol))
	}
	for x, col := range byCol {
		if x != 2 && x != 2+37 && x != 2+74 {
			t.Errorf("column x = %v is off the grid", x)
		}
		for i := 1; i < len(col); i++ {
			if gap := col[i].Y - (col[i-1].Y + col[i-1].H); gap != 3 {
				t.Errorf("gap in column %v = %v, want 3", x, gap)
			}
		}
	}
	for i := range plan {
		for j := i + 1; j < len(plan); j++ {
			a := Box{X: plan[i].X, Y: plan[i].Y, W: plan[i].W, H: plan[i].H}
			b := Box{X: plan[j].X, Y: plan[j].Y, W: plan[j].W, H: plan[j].H}
			if a.Overlaps(b) {
				t.Errorf("cards %d and %d overlap: %+v %+v", i, j, plan[i], plan[j])
			}
		}
	}
}

func TestSniffImage(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 48))); err != nil {
		t.Fatal(err)
	}
	info, err := SniffImage(buf.Bytes())
	if err != nil || info.Width != 64 || info.Height != 48 || info.Ext != "png" {
		t.Fatalf("png = %+v err %v", info, err)
	}
	if _, err := SniffImage([]byte("not an image at all")); err == nil {
		t.Fatal("garbage should not sniff")
	}
}

func TestMediaURL(t *testing.T) {
	c := &Client{}
	got, err := c.mediaURL("https://bucket.s3.amazonaws.com/p/images/1Abc/8Oa/frame%2001.png?X-Amz-Signature=x")
	want := DefaultBaseURL + "/media/p/images/1Abc/8Oa/frame%2001.png"
	if err != nil || got != want {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestCheckBoardRules(t *testing.T) {
	p := &BoardPayload{Elements: map[string]*Element{
		"b": {ID: "b", ElementType: TypeBoard, Location: Location{ParentID: "root"}},
		"a": {ID: "a", ElementType: TypeCard, Location: CanvasAt("b", 2.5, 2, 1), Content: NoteContent(Doc(Paragraph("# fake heading")), "")},
		"i": {ID: "i", ElementType: TypeCard, Location: ListAt("b", 0, 1), Content: NoteContent(Doc(Paragraph("loose")), "")},
	}}
	got := map[string]bool{}
	for _, is := range CheckBoard(p, "b", 2).Issues {
		got[is.Kind] = true
	}
	for _, k := range []string{"off-grid", "unsorted", "plain-markup"} {
		if !got[k] {
			t.Errorf("missing %s in %v", k, got)
		}
	}
}
