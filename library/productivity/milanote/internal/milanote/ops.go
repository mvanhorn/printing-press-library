// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package milanote

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ScoreStep is the gap the web app leaves between stacking scores.
const ScoreStep = 65536

// Colors maps the CLI's colour names to Milanote's stored values.
var Colors = map[string]string{
	"grey": "GREY", "gray": "GREY", "cyan": "CYAN", "green": "GREEN", "brown": "BROWN",
	"yellow": "YELLOW", "orange": "ORANGE", "red": "RED", "pink": "PINK",
	"purple": "PURPLE", "blue": "BLUE", "deep_blue": "DEEP_BLUE", "deep-blue": "DEEP_BLUE", "navy": "DEEP_BLUE",
}

// ColorNames lists the colours in palette order for help text.
var ColorNames = []string{"grey", "cyan", "green", "brown", "yellow", "orange", "red", "pink", "purple", "blue", "deep_blue"}

// DefaultBoardPalette is the palette Milanote gives new boards.
var DefaultBoardPalette = []any{"#8bafba", "#a1c8cb", "#82cec7", "#a1dbd0", "#f5ceaa", "#e2aca8", "#f07167"}

// Writer sends changes over one sync connection.
type Writer struct {
	*Conn
	versionID string
}

// Writer opens a sync connection for a batch of writes. Close it when done.
func (c *Client) Writer(ctx context.Context) (*Writer, error) {
	conn, err := c.Open(ctx)
	if err != nil {
		return nil, err
	}
	return &Writer{Conn: conn, versionID: strings.TrimPrefix(conn.session, "msid-") + "-1"}, nil
}

func nowMS() int64 { return time.Now().UnixMilli() }

// CanvasAt is a canvas location in grid units.
func CanvasAt(parentID string, x, y, score float64) Location {
	return Location{ParentID: parentID, Section: SectionCanvas, Position: map[string]any{"x": x, "y": y, "score": score}}
}

// ListAt is a position inside a column or the unsorted panel.
func ListAt(parentID string, index int, score float64) Location {
	return Location{ParentID: parentID, Section: SectionInbox, Position: map[string]any{"index": index, "score": score}}
}

// Create adds an element and returns its id. channel is the board whose
// subscribers must hear about it (the board itself, even when parentID is a
// column inside it).
func (w *Writer) Create(ctx context.Context, channel, elementType string, loc Location, content map[string]any, source string) (string, error) {
	id := w.NewID()
	return id, w.CreateWithID(ctx, id, channel, elementType, loc, content, source)
}

// CreateWithID is Create with a caller-chosen id (comment threads reference
// their own id inside their content).
func (w *Writer) CreateWithID(ctx context.Context, id, channel, elementType string, loc Location, content map[string]any, source string) error {
	if source == "" {
		source = "TOOLBAR"
	}
	now := nowMS()
	uid := w.User().ID
	action := map[string]any{
		"id":       id,
		"location": loc,
		"content":  content,
		"meta": map[string]any{
			"creator": uid, "modifiedBy": uid, "createdTime": now, "modifiedTime": now,
			"platform": "Desktop web", "locationSectionModifiedTime": now, "versionId": w.versionID,
		},
		"elementType":    elementType,
		"type":           "ELEMENT_CREATE",
		"timestamp":      now,
		"creationSource": source,
		"markAsFetched":  true,
		"activity":       map[string]any{"track": false, "boardId": channel},
	}
	if err := w.Send(ctx, action, []string{channel}); err != nil {
		return fmt.Errorf("creating %s: %w", strings.ToLower(elementType), err)
	}
	return nil
}

// Change is one element's partial content update.
type Change struct {
	ID      string
	Changes map[string]any
}

// Update applies content changes to elements on one board.
func (w *Writer) Update(ctx context.Context, boardID string, changes ...Change) error {
	if len(changes) == 0 {
		return nil
	}
	updates := make([]any, len(changes))
	for i, c := range changes {
		updates[i] = map[string]any{"id": c.ID, "changes": c.Changes}
	}
	action := map[string]any{
		"type": "ELEMENT_UPDATE", "updates": updates, "timestamp": nowMS(),
		"recalculateChannels": false, "activity": map[string]any{"track": false, "boardId": boardID},
	}
	return w.Send(ctx, action, []string{boardID})
}

// UpdateBoard changes a board element itself (title, colour), notifying
// both its parent board and the board's own subscribers.
func (w *Writer) UpdateBoard(ctx context.Context, board *Element, changes map[string]any) error {
	parent := board.Location.ParentID
	channels := []string{board.ID}
	if parent != "" {
		channels = []string{parent, board.ID}
	}
	action := map[string]any{
		"type": "ELEMENT_UPDATE", "updates": []any{map[string]any{"id": board.ID, "changes": changes}},
		"timestamp": nowMS(), "recalculateChannels": false,
		"activity": map[string]any{"track": false, "boardId": parent, "elementTypes": map[string]any{board.ID: TypeBoard}},
	}
	return w.Send(ctx, action, channels)
}

// SetLabels replaces an element's label ids, recording which were added.
func (w *Writer) SetLabels(ctx context.Context, boardID, elementID string, labels, added []string) error {
	action := map[string]any{
		"type": "ELEMENT_UPDATE", "updateType": "LABEL",
		"updates": []any{map[string]any{
			"id": elementID, "changes": map[string]any{"labels": labels},
			"meta": map[string]any{"addedLabelIds": added},
		}},
		"timestamp": nowMS(), "recalculateChannels": false,
		"activity": map[string]any{"track": false, "boardId": boardID},
	}
	return w.Send(ctx, action, []string{boardID})
}

// CreateLabel adds an account label and returns its id.
func (w *Writer) CreateLabel(ctx context.Context, name, color string) (string, error) {
	if color == "" {
		color = "#ac2c75"
	}
	id := "lbl_" + w.NewID()
	now := nowMS()
	label := map[string]any{"_id": id, "name": name, "color": color,
		"meta": map[string]any{"creator": w.User().ID, "createdTime": now, "modifiedTime": now}}
	err := w.Send(ctx, map[string]any{"type": "LABEL_CREATE", "label": label}, []string{w.User().ID + "-PERSONAL"})
	return id, err
}

// Move is one element relocation.
type Move struct {
	ID   string
	To   Location
	From Location
}

func boardChannel(parentID string) string { return strings.TrimSuffix(parentID, "-trash") }

// MoveMany relocates elements in one action. op is DROP (rearrange) or TRASH.
func (w *Writer) MoveMany(ctx context.Context, op string, moves []Move) error {
	if len(moves) == 0 {
		return nil
	}
	if op == "" {
		op = "DROP"
	}
	list := make([]any, len(moves))
	seen := map[string]bool{}
	var channels []string
	add := func(id string) {
		ch := boardChannel(id)
		if ch != "" && !seen[ch] {
			seen[ch] = true
			channels = append(channels, ch)
		}
	}
	for i, m := range moves {
		list[i] = map[string]any{"id": m.ID, "location": m.To, "from": m.From}
		add(m.From.ParentID)
		add(m.To.ParentID)
	}
	src := boardChannel(moves[0].From.ParentID)
	dst := boardChannel(moves[0].To.ParentID)
	action := map[string]any{
		"type": "ELEMENT_MOVE_MULTI", "moves": list, "timestamp": nowMS(),
		"moveOperation": op, "monitoring": map[string]any{"operation": op},
		"activity": map[string]any{"track": false, "sourceBoardId": src, "destinationBoardId": dst,
			"isSourceShared": false, "isDestinationShared": false},
	}
	return w.Send(ctx, action, channels)
}

// Trash moves elements to the trash, remembering where they were.
func (w *Writer) Trash(ctx context.Context, els []*Element) error {
	now := nowMS()
	moves := make([]Move, 0, len(els))
	for _, el := range els {
		if el.Location.Section == SectionTrash {
			continue
		}
		moves = append(moves, Move{
			ID:   el.ID,
			From: el.Location,
			To: Location{ParentID: el.Location.ParentID + "-trash", Section: SectionTrash,
				Position: map[string]any{"addedDate": now, "restoreLocation": el.Location}},
		})
	}
	return w.MoveMany(ctx, "TRASH", moves)
}

// Restore moves trashed elements back to where they were.
func (w *Writer) Restore(ctx context.Context, els []*Element) error {
	moves := make([]Move, 0, len(els))
	for _, el := range els {
		raw, ok := el.Location.Position["restoreLocation"].(map[string]any)
		if !ok {
			return fmt.Errorf("element %s has no restore location", el.ID)
		}
		to := Location{}
		to.ParentID, _ = raw["parentId"].(string)
		to.Section, _ = raw["section"].(string)
		to.Position, _ = raw["position"].(map[string]any)
		if to.ParentID == "" {
			return fmt.Errorf("element %s has no restore location", el.ID)
		}
		moves = append(moves, Move{ID: el.ID, From: el.Location, To: to})
	}
	return w.MoveMany(ctx, "DROP", moves)
}

// AddComment posts a comment. With threadID empty a new thread element is
// created at loc first. It returns the comment and thread ids.
func (w *Writer) AddComment(ctx context.Context, boardID, threadID string, loc Location, markdown string) (string, string, error) {
	if threadID == "" {
		threadID = w.NewID()
		if err := w.CreateWithID(ctx, threadID, boardID, TypeCommentThread, loc, map[string]any{"threadId": threadID}, "TOOLBAR"); err != nil {
			return "", "", err
		}
	}
	id := "cm_" + w.NewID()
	action := map[string]any{
		"_id": id, "type": "COMMENTS_ADD", "createdAt": nowMS(), "userId": w.User().ID,
		"elementId": threadID, "threadId": threadID, "text": MarkdownDoc(markdown),
		"activity": map[string]any{"track": false, "boardId": boardID},
	}
	if err := w.Send(ctx, action, []string{boardID}); err != nil {
		return "", "", fmt.Errorf("posting comment: %w", err)
	}
	return id, threadID, nil
}

// EditComment replaces a comment's text.
func (w *Writer) EditComment(ctx context.Context, boardID, commentID, elementID, markdown string) error {
	action := map[string]any{
		"_id": commentID, "elementId": elementID, "type": "COMMENTS_UPDATE",
		"text": MarkdownDoc(markdown), "userId": w.User().ID,
		"activity": map[string]any{"track": false, "boardId": boardID},
	}
	return w.Send(ctx, action, []string{boardID})
}

// DeleteComment removes a comment.
func (w *Writer) DeleteComment(ctx context.Context, boardID, commentID, elementID, threadID string) error {
	action := map[string]any{
		"_id": commentID, "elementId": elementID, "type": "COMMENTS_DELETE", "threadId": threadID,
		"userId": w.User().ID, "activity": map[string]any{"track": false, "boardId": boardID},
	}
	return w.Send(ctx, action, []string{boardID})
}

// SetType turns a placeholder element into its final type (link previews).
func (w *Writer) SetType(ctx context.Context, boardID, id, elementType string, changes map[string]any) error {
	action := map[string]any{
		"type": "ELEMENT_SET_TYPE", "id": id, "elementType": elementType, "changes": changes,
		"timestamp": nowMS(), "activity": map[string]any{"track": false, "boardId": boardID},
	}
	return w.Send(ctx, action, []string{boardID})
}

// --- content builders ---

// NoteContent is a CARD's content.
func NoteContent(doc map[string]any, color string) map[string]any {
	c := map[string]any{"textContent": doc}
	if color != "" {
		c["color"] = color
	}
	return c
}

// HeadingContent is a transparent CARD holding one heading, the way the
// heading tool makes it.
func HeadingContent(level int, text, align string) map[string]any {
	if align == "" {
		align = "center"
	}
	return map[string]any{"textContent": Doc(Heading(level, text, align)), "background": nil, "transparent": map[string]any{"enabled": true}}
}

// BoardContent is a new BOARD's content.
func BoardContent(title string) map[string]any {
	return map[string]any{"title": title, "icon": nil, "color": nil, "secondaryColor": nil, "defaultColorPalette": DefaultBoardPalette}
}

// ColorValue resolves a colour name; "none" clears it.
func ColorValue(name string) (any, error) {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "none" || n == "" || n == "clear" {
		return nil, nil
	}
	if v, ok := Colors[n]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("unknown colour %q (choose from %s, or none)", name, strings.Join(ColorNames, ", "))
}

// --- placement helpers ---

// MaxScore returns the highest stacking score among children of parentID.
func MaxScore(p *BoardPayload, parentID string) float64 {
	best := 0.0
	for _, el := range p.Elements {
		if el.Location.ParentID == parentID && el.Location.Score() > best {
			best = el.Location.Score()
		}
	}
	return best
}

// MinInboxScore returns the lowest score in a board's unsorted panel.
func MinInboxScore(p *BoardPayload, parentID string) float64 {
	best, found := 0.0, false
	for _, el := range p.Elements {
		if el.Location.ParentID == parentID && el.Location.Section == SectionInbox {
			if !found || el.Location.Score() < best {
				best, found = el.Location.Score(), true
			}
		}
	}
	return best
}

// Unsorted is the top of a board's unsorted panel.
func Unsorted(p *BoardPayload, boardID string) Location {
	return ListAt(boardID, 0, MinInboxScore(p, boardID)-1)
}

// ColumnEnd is the bottom slot of a column.
func ColumnEnd(p *BoardPayload, columnID string) Location {
	kids := p.Children(columnID, SectionInbox)
	return ListAt(columnID, len(kids), MaxScore(p, columnID)+ScoreStep)
}

// BoardOf returns the board that holds an element (walking out of columns).
func BoardOf(p *BoardPayload, id string) string {
	for i := 0; i < 8; i++ {
		el := p.Elements[id]
		if el == nil {
			return ""
		}
		parent := p.Elements[el.Location.ParentID]
		if parent == nil || parent.ElementType == TypeBoard {
			return el.Location.ParentID
		}
		id = parent.ID
	}
	return ""
}

// Find resolves an element by id or by unique id prefix within a payload.
func Find(p *BoardPayload, ref string) (*Element, error) {
	if el := p.Elements[ref]; el != nil {
		return el, nil
	}
	var hits []*Element
	for id, el := range p.Elements {
		if strings.HasPrefix(id, ref) {
			hits = append(hits, el)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return nil, ErrNoElement
	}
	return nil, fmt.Errorf("%q matches %d elements; use more of the id", ref, len(hits))
}

// ErrNoElement means an element id is not on the board.
var ErrNoElement = errors.New("no such element on this board")
