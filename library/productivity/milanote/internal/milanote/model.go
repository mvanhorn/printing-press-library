// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"encoding/json"
	"sort"
	"strings"
)

// Element types the web app stores.
const (
	TypeBoard         = "BOARD"
	TypeCard          = "CARD"
	TypeTaskList      = "TASK_LIST"
	TypeImage         = "IMAGE"
	TypeColumn        = "COLUMN"
	TypeLink          = "LINK"
	TypeCommentThread = "COMMENT_THREAD"
	TypeLine          = "LINE"
	TypeFile          = "FILE"
	TypeDocument      = "DOCUMENT"
	TypeSketch        = "SKETCH"
	TypeSwatch        = "COLOR_SWATCH"
	TypeAnnotation    = "ANNOTATION"
	TypeAlias         = "ALIAS"
	TypeTable         = "TABLE"
)

// Location sections.
const (
	SectionCanvas = "CANVAS"
	SectionInbox  = "INBOX"
	SectionTrash  = "TRASH"
)

// User is the subset of /api/users/me the CLI relies on.
type User struct {
	ID               string `json:"_id"`
	Email            string `json:"email"`
	ClientID         string `json:"clientId"`
	ClientTick       int64  `json:"clientTick"`
	RootBoardID      string `json:"rootBoardId"`
	QuickNotesRootID string `json:"quickNotesRootId"`
	Timezone         any    `json:"timezone"`
	Name             struct {
		DisplayName string `json:"displayName"`
		GivenName   string `json:"givenName"`
		FamilyName  string `json:"familyName"`
	} `json:"name"`
}

// Location places an element inside a parent.
type Location struct {
	ParentID string         `json:"parentId,omitempty"`
	Section  string         `json:"section,omitempty"`
	Position map[string]any `json:"position,omitempty"`
}

// X returns the canvas x position in grid units.
func (l Location) X() float64 { return num(l.Position["x"]) }

// Y returns the canvas y position in grid units.
func (l Location) Y() float64 { return num(l.Position["y"]) }

// Index returns the list index inside a column or the unsorted panel.
func (l Location) Index() float64 { return num(l.Position["index"]) }

// Score returns the stacking/ordering score.
func (l Location) Score() float64 { return num(l.Position["score"]) }

// Element is one item on a board.
type Element struct {
	ID          string         `json:"_id"`
	ElementType string         `json:"elementType"`
	Location    Location       `json:"location"`
	Content     map[string]any `json:"content"`
	Meta        map[string]any `json:"meta"`
	ACL         map[string]any `json:"acl,omitempty"`
	Version     int            `json:"__v"`
}

// UnmarshalJSON accepts both _id and id.
func (e *Element) UnmarshalJSON(data []byte) error {
	type plain Element
	var raw struct {
		plain
		AltID string `json:"id"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*e = Element(raw.plain)
	if e.ID == "" {
		e.ID = raw.AltID
	}
	if e.Content == nil {
		e.Content = map[string]any{}
	}
	return nil
}

// Title returns the element's title (boards, columns, task lists).
func (e *Element) Title() string {
	if s, ok := e.Content["title"].(string); ok {
		return s
	}
	return ""
}

// Text renders the element's visible text as plain text.
func (e *Element) Text() string {
	switch e.ElementType {
	case TypeBoard, TypeColumn:
		return e.Title()
	case TypeLink:
		title, _ := e.Content["title"].(string)
		u, _ := e.Content["url"].(string)
		if title != "" && u != "" {
			return title + " (" + u + ")"
		}
		return title + u
	case TypeImage:
		if c, ok := e.Content["caption"]; ok {
			return PlainText(c)
		}
		if f, ok := e.Content["file"].(map[string]any); ok {
			name, _ := f["filename"].(string)
			return name
		}
	case TypeCommentThread:
		return ""
	}
	if tc, ok := e.Content["textContent"]; ok {
		return PlainText(tc)
	}
	if s, ok := e.Content["text"].(string); ok {
		return s
	}
	return e.Title()
}

// Color returns the element's colour name, lowercased, or "".
func (e *Element) Color() string {
	for _, key := range []string{"color", "background"} {
		switch v := e.Content[key].(type) {
		case string:
			return strings.ToLower(v)
		case map[string]any:
			if s, ok := v["color"].(string); ok {
				return strings.ToLower(s)
			}
		}
	}
	return ""
}

// Width returns the card width in grid units (0 means default).
func (e *Element) Width() float64 { return num(e.Content["width"]) }

// Tasks returns the to-do items of a TASK_LIST.
func (e *Element) Tasks() []Task {
	doc, _ := e.Content["textContent"].(map[string]any)
	var out []Task
	walk(doc, func(node map[string]any) bool {
		if node["type"] != "taskItem" {
			return true
		}
		attrs, _ := node["attrs"].(map[string]any)
		t := Task{Text: PlainText(node)}
		if attrs != nil {
			t.ID, _ = attrs["taskId"].(string)
			t.Checked, _ = attrs["checked"].(bool)
		}
		out = append(out, t)
		return false
	})
	return out
}

// Task is one to-do item.
type Task struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

// Label is an account label.
type Label struct {
	ID    string `json:"_id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// HistoryEntry is a recently viewed board.
type HistoryEntry struct {
	ElementID string `json:"elementId"`
	LastView  int64  `json:"lastView"`
	ViewCount int    `json:"viewCount"`
}

// SearchHit is one result of Milanote's board search.
type SearchHit struct {
	ID            string         `json:"_id"`
	Board         Element        `json:"board"`
	ModifiedTime  int64          `json:"modifiedTime"`
	ViewedTime    int64          `json:"viewedTime"`
	Score         float64        `json:"score"`
	ElementCounts map[string]int `json:"elementCounts"`
	Ancestors     []any          `json:"ancestors"`
}

// BoardPayload is the answer of /api/boards and /api/users/me/trash.
type BoardPayload struct {
	BoardIDs     []string            `json:"boardIds"`
	ElementCount int                 `json:"elementCount"`
	Elements     map[string]*Element `json:"elements"`
	Comments     json.RawMessage     `json:"comments"`
	Labels       json.RawMessage     `json:"labels"`
	TotalCount   int                 `json:"totalCount"`
	HasMore      bool                `json:"hasMore"`
	FetchedTime  int64               `json:"fetchedTime"`
}

// Children returns the elements directly inside parentID, in reading order:
// canvas items top-to-bottom then left-to-right, list items by index.
func (p *BoardPayload) Children(parentID string, section string) []*Element {
	var out []*Element
	for _, el := range p.Elements {
		if el.ID == parentID || el.Location.ParentID != parentID {
			continue
		}
		if section != "" && el.Location.Section != section {
			continue
		}
		out = append(out, el)
	}
	SortReading(out)
	return out
}

// Comments decodes the payload's comments into a flat list.
func (p *BoardPayload) CommentList() []map[string]any {
	if len(p.Comments) == 0 {
		return nil
	}
	var asMap map[string]map[string]any
	if json.Unmarshal(p.Comments, &asMap) == nil {
		out := make([]map[string]any, 0, len(asMap))
		for _, c := range asMap {
			out = append(out, c)
		}
		sort.Slice(out, func(i, j int) bool { return num(out[i]["createdAt"]) < num(out[j]["createdAt"]) })
		return out
	}
	var asList []map[string]any
	_ = json.Unmarshal(p.Comments, &asList)
	return asList
}

// SortReading orders canvas elements in rows (top to bottom, then left to
// right) and list elements by index. Elements whose tops are within
// rowTolerance units share a row.
func SortReading(els []*Element) {
	const rowTolerance = 4
	sort.SliceStable(els, func(i, j int) bool {
		a, b := els[i].Location, els[j].Location
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		if _, ok := a.Position["index"]; ok {
			if a.Index() != b.Index() {
				return a.Index() < b.Index()
			}
			return a.Score() < b.Score()
		}
		dy := a.Y() - b.Y()
		if dy < -rowTolerance || dy > rowTolerance {
			return a.Y() < b.Y()
		}
		if a.X() != b.X() {
			return a.X() < b.X()
		}
		return a.Y() < b.Y()
	})
}

func num(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func walk(node any, visit func(map[string]any) bool) {
	switch n := node.(type) {
	case map[string]any:
		if !visit(n) {
			return
		}
		if kids, ok := n["content"].([]any); ok {
			for _, k := range kids {
				walk(k, visit)
			}
		}
	case []any:
		for _, k := range n {
			walk(k, visit)
		}
	}
}
