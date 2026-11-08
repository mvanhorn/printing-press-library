// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"crypto/sha256"
	"encoding/hex"
	"math"
	"regexp"
	"strings"
)

// Chunk kinds.
const (
	ChunkHeading = "heading"
	ChunkNote    = "note"
	ChunkTasks   = "tasks"
)

// Chunk is the part of a Markdown file that becomes one card.
type Chunk struct {
	Kind    string
	Level   int    // heading level (1 or 2)
	Text    string // heading text
	MD      string // the source Markdown of this card
	Tasks   []Task
	Band    int // increments at every level-1 heading
	Section int // increments at every level-2 heading
	Start   int // first source line (0-based)
	End     int // one past the last source line
}

// ChunkMarkdown cuts a document into cards: one heading card per # or ##
// line, one to-do list per run of "- [ ]" lines, and one note for everything
// between. A horizontal rule ends a note without becoming a card.
func ChunkMarkdown(md string) []Chunk {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []Chunk
	band, section := 0, 0
	seenHeading := false
	noteStart := -1
	flush := func(end int) {
		if noteStart < 0 {
			return
		}
		start, stop := noteStart, end
		noteStart = -1
		for start < stop && strings.TrimSpace(lines[start]) == "" {
			start++
		}
		for stop > start && strings.TrimSpace(lines[stop-1]) == "" {
			stop--
		}
		if start == stop {
			return
		}
		out = append(out, Chunk{Kind: ChunkNote, MD: strings.Join(lines[start:stop], "\n"),
			Band: band, Section: section, Start: start, End: stop})
	}
	inFence := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if noteStart < 0 {
				noteStart = i
			}
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if m := reHeading.FindStringSubmatch(trimmed); m != nil && len(m[1]) <= 2 {
			flush(i)
			level := len(m[1])
			if level == 1 {
				if seenHeading {
					band++
				}
				section++
			} else {
				section++
			}
			seenHeading = true
			out = append(out, Chunk{Kind: ChunkHeading, Level: level, Text: m[2], MD: trimmed,
				Band: band, Section: section, Start: i, End: i + 1})
			continue
		}
		if reRule.MatchString(trimmed) && trimmed != "" {
			flush(i)
			continue
		}
		if reTask.MatchString(line) {
			flush(i)
			start := i
			var tasks []Task
			for ; i < len(lines) && reTask.MatchString(lines[i]); i++ {
				m := reTask.FindStringSubmatch(lines[i])
				tasks = append(tasks, Task{Text: m[2], Checked: m[1] != " "})
			}
			out = append(out, Chunk{Kind: ChunkTasks, MD: strings.Join(lines[start:i], "\n"), Tasks: tasks,
				Band: band, Section: section, Start: start, End: i})
			i--
			continue
		}
		if noteStart < 0 && trimmed != "" {
			noteStart = i
		}
	}
	flush(len(lines))
	return out
}

// Element returns the element type and content this chunk becomes. align is
// used for level-2 headings; level-1 headings are centred.
func (c Chunk) Element() (string, map[string]any) {
	switch c.Kind {
	case ChunkHeading:
		align := "left"
		if c.Level == 1 {
			align = "center"
		}
		return TypeCard, HeadingContent(c.Level, c.Text, align)
	case ChunkTasks:
		return TypeTaskList, TaskListContent("", c.Tasks)
	}
	return TypeCard, NoteContent(MarkdownDoc(c.MD), "")
}

// Doc returns the textContent document for this chunk.
func (c Chunk) Doc() map[string]any {
	_, content := c.Element()
	doc, _ := content["textContent"].(map[string]any)
	return doc
}

// Normalized is the chunk rendered through the same path a board card takes
// back to Markdown, so file text and board text compare like for like.
func (c Chunk) Normalized() string { return ToMarkdown(c.Doc()) }

// Height estimates the card height at widthUnits.
func (c Chunk) Height(widthUnits float64) float64 {
	switch c.Kind {
	case ChunkHeading:
		return HeadingHeight(c.Level, c.Text, widthUnits)
	case ChunkTasks:
		return TaskListHeight(c.Tasks, widthUnits)
	}
	return NoteHeight(c.Doc(), widthUnits)
}

// HashText is the short content hash used to detect edits.
func HashText(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:8])
}

// ElementMarkdown renders a card or to-do list as Markdown.
func ElementMarkdown(el *Element) string {
	if doc, ok := el.Content["textContent"]; ok {
		return ToMarkdown(doc)
	}
	return el.Text()
}

// ChunkKindOf classifies an existing element the way ChunkMarkdown would.
func ChunkKindOf(el *Element) string {
	if el.ElementType == TypeTaskList {
		return ChunkTasks
	}
	if _, _, ok := headingCard(el); ok {
		return ChunkHeading
	}
	return ChunkNote
}

// Planned is a card with its computed canvas position.
type Planned struct {
	Index int     `json:"index"`
	Kind  string  `json:"kind"`
	Text  string  `json:"text"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
}

// PlanOpts sets the grid a document is laid out on.
type PlanOpts struct {
	X, Y   float64
	Cols   int
	ColW   float64
	Gutter float64
}

type planSection struct {
	idx []int
	h   float64
}

// DefaultCols picks a column count for a document: one per section, at most
// four.
func DefaultCols(chunks []Chunk) int {
	seen := map[int]bool{}
	for _, c := range chunks {
		if c.Kind == ChunkHeading && c.Level == 1 {
			continue
		}
		seen[c.Section] = true
	}
	n := len(seen)
	if n < 1 {
		return 1
	}
	if n > 4 {
		return 4
	}
	return n
}

// PlanLayout computes where every chunk goes. Each level-1 heading starts a
// band and is centred across the columns; each section (a level-2 heading
// and what follows it) is stacked, and sections are dropped into the
// shortest column so every vertical gap on the board equals the gutter.
func PlanLayout(chunks []Chunk, o PlanOpts) []Planned {
	if o.Cols < 1 {
		o.Cols = 1
	}
	if o.ColW <= 0 {
		o.ColW = CardWidth
	}
	out := make([]Planned, len(chunks))
	full := float64(o.Cols)*o.ColW + float64(o.Cols-1)*o.Gutter
	y := o.Y
	place := func(i int, x, y, w, h float64) {
		out[i] = Planned{Index: i, Kind: chunks[i].Kind, Text: firstLine(chunks[i].plainPreview(), 60), X: Snap(x), Y: Snap(y), W: w, H: h}
	}
	for i := 0; i < len(chunks); {
		band := chunks[i].Band
		j := i
		for j < len(chunks) && chunks[j].Band == band {
			j++
		}
		var sections []*planSection
		bySection := map[int]*planSection{}
		for k := i; k < j; k++ {
			c := chunks[k]
			if c.Kind == ChunkHeading && c.Level == 1 {
				h := c.Height(full)
				place(k, o.X, y, full, h)
				y += h + o.Gutter
				continue
			}
			s := bySection[c.Section]
			if s == nil {
				s = &planSection{}
				bySection[c.Section] = s
				sections = append(sections, s)
			}
			h := c.Height(o.ColW)
			if len(s.idx) > 0 {
				s.h += o.Gutter
			}
			s.idx = append(s.idx, k)
			s.h += h
		}
		heights := make([]float64, len(sections))
		for n, s := range sections {
			heights[n] = s.h
		}
		slots := Grid{X: o.X, Y: y, Cols: o.Cols, ColW: o.ColW, Gutter: o.Gutter}.Masonry(heights)
		bottom := y
		for n, s := range sections {
			cy := slots[n].Y
			for m, k := range s.idx {
				if m > 0 {
					cy += o.Gutter
				}
				h := chunks[k].Height(o.ColW)
				place(k, slots[n].X, cy, o.ColW, h)
				cy += h
			}
			bottom = math.Max(bottom, cy)
		}
		if len(sections) > 0 {
			y = bottom + o.Gutter*2
		}
		i = j
	}
	return out
}

func (c Chunk) plainPreview() string {
	if c.Kind == ChunkHeading {
		return c.Text
	}
	return PlainText(c.Doc())
}

// --- board-wide checks ---

var plainMarkup = regexp.MustCompile(`^(#{1,6}\s|[-*+]\s|\[[ xX]\]\s|\d+[.)]\s)`)

// CheckBoard extends Check with the rules that need the full element list:
// off-grid coordinates, cards stuck in Unsorted, and paragraphs that are
// plain-text imitations of a heading, list or checkbox.
func CheckBoard(p *BoardPayload, boardID string, tolerance float64) CheckReport {
	r := Check(CanvasBoxes(p, boardID), tolerance)
	for _, el := range p.Children(boardID, SectionCanvas) {
		x, y := el.Location.X(), el.Location.Y()
		if x != math.Trunc(x) || y != math.Trunc(y) {
			r.Issues = append(r.Issues, Issue{Kind: "off-grid", IDs: []string{el.ID},
				Detail: quote(firstLine(el.Text(), 60)) + " sits between grid units", Penalty: 1})
		}
	}
	if inbox := p.Children(boardID, SectionInbox); len(inbox) > 0 {
		ids := make([]string, len(inbox))
		for i, el := range inbox {
			ids[i] = el.ID
		}
		r.Issues = append(r.Issues, Issue{Kind: "unsorted", IDs: ids,
			Detail: fmtCount(len(inbox)) + " still in the Unsorted panel", Penalty: float64(len(inbox))})
	}
	for _, el := range p.Elements {
		if el.ElementType != TypeCard || el.Location.Section == SectionTrash || BoardOf(p, el.ID) != boardID {
			continue
		}
		doc, _ := el.Content["textContent"].(map[string]any)
		for _, raw := range asList(doc["content"]) {
			b, _ := raw.(map[string]any)
			if b == nil || b["type"] != "paragraph" {
				continue
			}
			text := PlainText(b)
			if plainMarkup.MatchString(text) {
				r.Issues = append(r.Issues, Issue{Kind: "plain-markup", IDs: []string{el.ID},
					Detail: quote(firstLine(text, 60)) + " is typed by hand; use a real heading, list or checkbox", Penalty: 1})
				break
			}
		}
	}
	penalty := 0.0
	for _, is := range r.Issues {
		penalty += is.Penalty
	}
	r.Score = int(math.Max(0, 100-penalty))
	return r
}

func fmtCount(n int) string {
	if n == 1 {
		return "1 card"
	}
	return strconvItoa(n) + " cards"
}
