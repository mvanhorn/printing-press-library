// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"context"
	"strings"
)

// treeBatch is how many boards one /api/boards request asks for.
const treeBatch = 8

// TreeBoard is one board reached while walking a board tree.
type TreeBoard struct {
	ID       string        `json:"id"`
	Title    string        `json:"title"`
	ParentID string        `json:"parent_id,omitempty"`
	Depth    int           `json:"depth"`
	Path     []string      `json:"path"`
	Payload  *BoardPayload `json:"-"`
}

// PathString joins the board's breadcrumb.
func (t *TreeBoard) PathString() string { return strings.Join(t.Path, " / ") }

// ChildBoards returns the sub-boards placed on boardID (on the canvas, in
// its columns or its unsorted panel), in reading order.
func ChildBoards(p *BoardPayload, boardID string) []*Element {
	var out []*Element
	for _, el := range p.Elements {
		if el.ID == boardID || el.ElementType != TypeBoard || el.Location.Section == SectionTrash {
			continue
		}
		if BoardOf(p, el.ID) == boardID {
			out = append(out, el)
		}
	}
	SortReading(out)
	return out
}

// Scope returns the part of a merged payload that belongs to one board: the
// board itself plus every element whose nearest board is boardID.
func Scope(p *BoardPayload, boardID string) *BoardPayload {
	out := &BoardPayload{BoardIDs: []string{boardID}, Elements: map[string]*Element{}, Comments: p.Comments, Labels: p.Labels, FetchedTime: p.FetchedTime}
	for id, el := range p.Elements {
		if id == boardID || BoardOf(p, id) == boardID {
			out.Elements[id] = el
		}
	}
	out.ElementCount = len(out.Elements)
	return out
}

// Walk visits rootID and every board below it breadth-first, fetching
// boards in small batches. maxDepth < 0 means unlimited; 0 visits only the
// root. fn may return an error to stop.
func (c *Client) Walk(ctx context.Context, rootID string, maxDepth int, fn func(*TreeBoard) error) error {
	type pending struct {
		id, parent string
		depth      int
		path       []string
	}
	queue := []pending{{id: rootID}}
	seen := map[string]bool{rootID: true}
	for len(queue) > 0 {
		n := treeBatch
		if n > len(queue) {
			n = len(queue)
		}
		batch := queue[:n]
		queue = queue[n:]
		ids := make([]string, len(batch))
		for i, b := range batch {
			ids[i] = b.id
		}
		merged, err := c.Boards(ctx, ids...)
		if err != nil {
			return err
		}
		for _, b := range batch {
			el := merged.Elements[b.id]
			if el == nil {
				continue
			}
			title := el.Title()
			if title == "" && b.parent == "" {
				title = "Home"
			}
			tb := &TreeBoard{ID: b.id, Title: title, ParentID: b.parent, Depth: b.depth,
				Path: append(append([]string{}, b.path...), title), Payload: Scope(merged, b.id)}
			if err := fn(tb); err != nil {
				return err
			}
			if maxDepth >= 0 && b.depth >= maxDepth {
				continue
			}
			for _, child := range ChildBoards(merged, b.id) {
				if seen[child.ID] {
					continue
				}
				seen[child.ID] = true
				queue = append(queue, pending{id: child.ID, parent: b.id, depth: b.depth + 1, path: tb.Path})
			}
		}
	}
	return nil
}
