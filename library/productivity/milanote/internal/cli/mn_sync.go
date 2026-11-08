// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

// stateDir is where the CLI keeps pairings, undo records and snapshots.
func stateDir(sub string) (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "milanote-pp-cli", sub)
	return dir, os.MkdirAll(dir, 0o700)
}

// atomicWrite replaces path in one step so a crash never leaves half a file.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), perm); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// boardFileMode keeps the permissions of a file being replaced; a new file
// holds private board text, so only its owner can read it.
func boardFileMode(path string) os.FileMode {
	if info, err := os.Stat(path); err == nil {
		return info.Mode().Perm()
	}
	return 0o600
}

func writeJSONState(path string, v any) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return atomicWrite(path, data, 0o600)
}

// pairItem ties one chunk of a file to one card. Hash is the content both
// sides had when they last agreed.
type pairItem struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
	Hash string `json:"hash"`
}

// pairRecord is the memory of one file/board pairing.
type pairRecord struct {
	File      string     `json:"file"`
	Board     string     `json:"board"`
	Items     []pairItem `json:"items"`
	Ignored   []string   `json:"ignored,omitempty"` // cards that were on the board before pairing
	UpdatedAt string     `json:"updated_at"`
}

func pairPath(file, board string) (string, error) {
	dir, err := stateDir("mirror")
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(file)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, milanote.HashText(abs+"\x00"+board)+".json"), nil
}

func loadPair(file, board string) (*pairRecord, error) {
	path, err := pairPath(file, board)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rec pairRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, fmt.Errorf("pairing record %s is damaged: %w", path, err)
	}
	return &rec, nil
}

func savePair(rec *pairRecord) error {
	path, err := pairPath(rec.File, rec.Board)
	if err != nil {
		return err
	}
	rec.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeJSONState(path, rec)
}

// syncOptions controls one run of the file/board sync.
type syncOptions struct {
	direction string // push, pull or both
	prefer    string // file or board: how to settle conflicts; "" reports them
	cols      int
	colW      float64
	gutter    float64
	originSet bool
	x, y      float64
	dryRun    bool
}

type syncAction struct {
	Action string  `json:"action"`
	Kind   string  `json:"kind,omitempty"`
	ID     string  `json:"id,omitempty"`
	Text   string  `json:"text,omitempty"`
	X      float64 `json:"x,omitempty"`
	Y      float64 `json:"y,omitempty"`
}

type syncConflict struct {
	ID     string `json:"id"`
	Text   string `json:"text"`
	Reason string `json:"reason"`
}

type mirrorResult struct {
	File      string         `json:"file"`
	Board     string         `json:"board"`
	URL       string         `json:"board_url"`
	Direction string         `json:"direction"`
	Paired    bool           `json:"newly_paired"`
	DryRun    bool           `json:"dry_run,omitempty"`
	Actions   []syncAction   `json:"actions"`
	Conflicts []syncConflict `json:"conflicts"`
}

func (r *mirrorResult) note(action, kind, id, text string) {
	r.Actions = append(r.Actions, syncAction{Action: action, Kind: kind, ID: id, Text: preview(text)})
}

func preview(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	r := []rune(s)
	if len(r) > 70 {
		return string(r[:69]) + "…"
	}
	return s
}

// boardCards lists the text cards on a board in reading order, columns
// expanded in place and the Unsorted panel last.
func boardCards(p *milanote.BoardPayload, boardID string) []*milanote.Element {
	var out []*milanote.Element
	add := func(el *milanote.Element) {
		if el.ElementType == milanote.TypeCard || el.ElementType == milanote.TypeTaskList {
			out = append(out, el)
		}
	}
	for _, section := range []string{milanote.SectionCanvas, milanote.SectionInbox} {
		for _, el := range p.Children(boardID, section) {
			if el.ElementType == milanote.TypeColumn {
				for _, kid := range p.Children(el.ID, milanote.SectionInbox) {
					add(kid)
				}
				continue
			}
			add(el)
		}
	}
	return out
}

func liveCard(p *milanote.BoardPayload, id string) *milanote.Element {
	el := p.Elements[id]
	if el == nil || el.Location.Section == milanote.SectionTrash {
		return nil
	}
	return el
}

// matchChunks pairs chunks with remembered cards. Identical content pairs
// first, wherever it moved to; what is left pairs in document order between
// the fixed points, which is how an edited paragraph keeps its card.
func matchChunks(chunks []milanote.Chunk, hashes []string, items []pairItem) (chunkItem []int, used []bool) {
	chunkItem = make([]int, len(chunks))
	used = make([]bool, len(items))
	for i := range chunkItem {
		chunkItem[i] = -1
		for j, it := range items {
			if !used[j] && it.Hash == hashes[i] && it.Kind == chunks[i].Kind {
				chunkItem[i], used[j] = j, true
				break
			}
		}
	}
	for i := range chunks {
		if chunkItem[i] >= 0 {
			continue
		}
		lo := -1
		for k := i - 1; k >= 0; k-- {
			if chunkItem[k] >= 0 {
				lo = chunkItem[k]
				break
			}
		}
		hi := len(items)
		for k := i + 1; k < len(chunks); k++ {
			if chunkItem[k] >= 0 {
				hi = chunkItem[k]
				break
			}
		}
		for j := lo + 1; j < hi; j++ {
			if !used[j] && items[j].Kind == chunks[i].Kind {
				chunkItem[i], used[j] = j, true
				break
			}
		}
	}
	return chunkItem, used
}

// lazyWriter opens the sync connection only when something must be written.
type lazyWriter struct {
	c *milanote.Client
	w *milanote.Writer
}

func (l *lazyWriter) get(ctx context.Context) (*milanote.Writer, error) {
	if l.w != nil {
		return l.w, nil
	}
	w, err := l.c.Writer(ctx)
	if err != nil {
		return nil, err
	}
	l.w = w
	return w, nil
}

func (l *lazyWriter) close(ctx context.Context) {
	if l.w != nil {
		l.w.Close(ctx)
	}
}

// readSyncFile allows a missing file only when a first pull is about to create
// it. For a paired file, absence must not read as empty text, or a sync would
// take it to mean every card was removed.
func readSyncFile(path, direction string, starting bool) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if starting && direction == "pull" {
			return "", nil
		}
		if !starting {
			return "", usageErr(fmt.Errorf("%s is paired with this board but is missing: restore it and sync again; no cards were changed", path))
		}
	}
	if err != nil {
		return "", usageErr(err)
	}
	return string(data), nil
}

// runSync brings a Markdown file and a board together. With no pairing yet it
// lays the file out as new cards (or, for pull, writes the board to the file).
func runSync(ctx context.Context, c *milanote.Client, file, boardID string, o syncOptions, fresh bool) (*mirrorResult, error) {
	rec, err := loadPair(file, boardID)
	if err != nil {
		return nil, err
	}
	starting := rec == nil || fresh
	text, err := readSyncFile(file, o.direction, starting)
	if err != nil {
		return nil, err
	}
	if !starting && o.direction != "pull" && strings.TrimSpace(text) == "" && len(rec.Items) > 0 {
		return nil, usageErr(fmt.Errorf("%s is empty but is paired with %d card(s): refusing to trash them all; no cards were changed", file, len(rec.Items)))
	}
	payload, err := loadBoard(ctx, c, boardID)
	if err != nil {
		return nil, err
	}
	res := &mirrorResult{File: file, Board: boardID, URL: boardURL(boardID), Direction: o.direction,
		DryRun: o.dryRun, Actions: []syncAction{}, Conflicts: []syncConflict{}}
	writer := &lazyWriter{c: c}
	defer writer.close(ctx)
	if starting {
		res.Paired = true
		if o.direction == "pull" {
			return res, firstPull(file, boardID, payload, o, res)
		}
		return res, firstPush(ctx, writer, file, boardID, text, payload, o, res)
	}
	return res, incremental(ctx, writer, file, boardID, text, payload, rec, o, res)
}

// cardSeparator keeps neighbouring cards apart when the file is read back.
const cardSeparator = "\n\n---\n\n"

func firstPull(file, boardID string, payload *milanote.BoardPayload, o syncOptions, res *mirrorResult) error {
	cards := boardCards(payload, boardID)
	rec := &pairRecord{File: file, Board: boardID}
	parts := make([]string, 0, len(cards))
	for _, el := range cards {
		md := milanote.ElementMarkdown(el)
		parts = append(parts, milanote.CardBlock(md))
		rec.Items = append(rec.Items, pairItem{ID: el.ID, Kind: milanote.ChunkKindOf(el), Hash: milanote.HashText(md)})
		res.note("pull", milanote.ChunkKindOf(el), el.ID, md)
	}
	if o.dryRun {
		return nil
	}
	if err := atomicWrite(file, []byte(strings.Join(parts, cardSeparator)+"\n"), boardFileMode(file)); err != nil {
		return err
	}
	return savePair(rec)
}

func firstPush(ctx context.Context, writer *lazyWriter, file, boardID, text string, payload *milanote.BoardPayload, o syncOptions, res *mirrorResult) error {
	chunks := milanote.ChunkMarkdown(text)
	if len(chunks) == 0 {
		return usageErr(fmt.Errorf("%s has nothing to lay out", file))
	}
	x, y := milanote.FreeOrigin(milanote.CanvasBoxes(payload, boardID), o.gutter*2)
	if o.originSet {
		x, y = o.x, o.y
	}
	cols := o.cols
	if cols < 1 {
		cols = milanote.DefaultCols(chunks)
	}
	plan := milanote.PlanLayout(chunks, milanote.PlanOpts{X: x, Y: y, Cols: cols, ColW: o.colW, Gutter: o.gutter})
	rec := &pairRecord{File: file, Board: boardID}
	for _, el := range boardCards(payload, boardID) {
		rec.Ignored = append(rec.Ignored, el.ID)
	}
	score := milanote.MaxScore(payload, boardID)
	for i, ch := range chunks {
		p := plan[i]
		action := syncAction{Action: "create", Kind: ch.Kind, Text: p.Text, X: p.X, Y: p.Y}
		id := ""
		if !o.dryRun {
			w, err := writer.get(ctx)
			if err != nil {
				return mnErr(err)
			}
			elType, content := ch.Element()
			if p.W != milanote.CardWidth {
				content["width"] = p.W
			}
			score += milanote.ScoreStep
			if id, err = w.Create(ctx, boardID, elType, milanote.CanvasAt(boardID, p.X, p.Y, score), content, ""); err != nil {
				return mnErr(fmt.Errorf("card %d of %d: %w", i+1, len(chunks), err))
			}
		}
		action.ID = id
		res.Actions = append(res.Actions, action)
		rec.Items = append(rec.Items, pairItem{ID: id, Kind: ch.Kind, Hash: milanote.HashText(ch.Normalized())})
		if !o.dryRun { // a later failure must not lose the cards already made
			if err := savePair(rec); err != nil {
				return err
			}
		}
	}
	return nil
}

type lineEdit struct {
	start, end int
	text       *string // nil deletes the lines
}

func applyLineEdits(text string, edits []lineEdit, tail []string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i := len(edits) - 1; i >= 0; i-- { // edits are in file order
		e := edits[i]
		var repl []string
		if e.text != nil {
			repl = strings.Split(*e.text, "\n")
		}
		lines = append(lines[:e.start:e.start], append(repl, lines[e.end:]...)...)
	}
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	for _, t := range tail {
		if out != "" {
			out += cardSeparator
		}
		out += t
	}
	return out + "\n"
}

func incremental(ctx context.Context, writer *lazyWriter, file, boardID, text string, payload *milanote.BoardPayload, rec *pairRecord, o syncOptions, res *mirrorResult) error {
	chunks := milanote.ChunkMarkdown(text)
	hashes := make([]string, len(chunks))
	for i, ch := range chunks {
		hashes[i] = milanote.HashText(ch.Normalized())
	}
	chunkItem, used := matchChunks(chunks, hashes, rec.Items)
	push, pull := o.direction != "pull", o.direction != "push"

	var edits []lineEdit
	var tail []string
	newItems := make([]pairItem, 0, len(chunks))
	x, y := milanote.FreeOrigin(milanote.CanvasBoxes(payload, boardID), o.gutter)
	if o.originSet {
		x, y = o.x, o.y
	}
	score := milanote.MaxScore(payload, boardID)

	var created []pairItem
	createCard := func(ch milanote.Chunk, hash string) error {
		h := ch.Height(milanote.CardWidth)
		act := syncAction{Action: "create", Kind: ch.Kind, Text: preview(ch.MD), X: x, Y: y}
		id := ""
		if !o.dryRun {
			w, err := writer.get(ctx)
			if err != nil {
				return mnErr(err)
			}
			elType, content := ch.Element()
			score += milanote.ScoreStep
			if id, err = w.Create(ctx, boardID, elType, milanote.CanvasAt(boardID, x, y, score), content, ""); err != nil {
				return mnErr(err)
			}
		}
		act.ID = id
		res.Actions = append(res.Actions, act)
		y += h + o.gutter
		item := pairItem{ID: id, Kind: ch.Kind, Hash: hash}
		newItems = append(newItems, item)
		if o.dryRun {
			return nil
		}
		// Record it now: if a later write fails, a retry must find this card
		// instead of creating it again.
		created = append(created, item)
		progress := *rec
		progress.Items = append(append([]pairItem{}, rec.Items...), created...)
		return savePair(&progress)
	}
	updateCard := func(el *milanote.Element, ch milanote.Chunk) error {
		res.note("update", ch.Kind, el.ID, ch.MD)
		if o.dryRun {
			return nil
		}
		w, err := writer.get(ctx)
		if err != nil {
			return mnErr(err)
		}
		return mnErr(w.Update(ctx, milanote.BoardOf(payload, el.ID), milanote.Change{ID: el.ID, Changes: map[string]any{"textContent": ch.Doc()}}))
	}

	for i, ch := range chunks {
		j := chunkItem[i]
		if j < 0 {
			if !push {
				res.note("skip-new-in-file", ch.Kind, "", ch.MD)
				continue
			}
			if err := createCard(ch, hashes[i]); err != nil {
				return err
			}
			continue
		}
		item := rec.Items[j]
		fileChanged := hashes[i] != item.Hash
		el := liveCard(payload, item.ID)
		if el == nil {
			keepFile := fileChanged && o.prefer == "file"
			switch {
			case fileChanged && o.prefer == "":
				res.Conflicts = append(res.Conflicts, syncConflict{ID: item.ID, Text: preview(ch.MD), Reason: "edited in the file but deleted on the board"})
				newItems = append(newItems, item)
			case keepFile && push, !fileChanged && !pull:
				if err := createCard(ch, hashes[i]); err != nil {
					return err
				}
			case pull && !keepFile:
				res.note("remove-from-file", ch.Kind, item.ID, ch.MD)
				edits = append(edits, lineEdit{start: ch.Start, end: ch.End})
			default: // the winning side's change is outside the selected direction
				res.note("skip-file-edit", ch.Kind, item.ID, ch.MD)
				newItems = append(newItems, item)
			}
			continue
		}
		md := milanote.ElementMarkdown(el)
		boardHash := milanote.HashText(md)
		boardChanged := boardHash != item.Hash
		if fileChanged && boardChanged && boardHash != hashes[i] {
			switch o.prefer {
			case "file":
				boardChanged = false
			case "board":
				fileChanged = false
			default:
				res.Conflicts = append(res.Conflicts, syncConflict{ID: item.ID, Text: preview(ch.MD), Reason: "edited on both sides since the last sync"})
				newItems = append(newItems, item)
				continue
			}
		}
		switch {
		case fileChanged && boardChanged: // both sides made the same edit
			item.Hash = hashes[i]
		case fileChanged && push:
			if err := updateCard(el, ch); err != nil {
				return err
			}
			item.Hash = hashes[i]
		case boardChanged && pull:
			res.note("pull", ch.Kind, item.ID, md)
			m := milanote.CardBlock(md)
			edits = append(edits, lineEdit{start: ch.Start, end: ch.End, text: &m})
			item.Hash = boardHash
		case fileChanged || boardChanged:
			side := "file"
			if boardChanged {
				side = "board"
			}
			res.note("skip-"+side+"-edit", ch.Kind, item.ID, ch.MD)
		}
		newItems = append(newItems, item)
	}

	// Remembered cards whose text left the file.
	var trash []*milanote.Element
	for j, item := range rec.Items {
		if used[j] {
			continue
		}
		el := liveCard(payload, item.ID)
		if el == nil {
			continue
		}
		boardChanged := milanote.HashText(milanote.ElementMarkdown(el)) != item.Hash
		switch {
		case boardChanged && o.prefer != "file":
			if o.prefer == "board" && pull {
				md := milanote.ElementMarkdown(el)
				res.note("pull", item.Kind, item.ID, md)
				tail = append(tail, milanote.CardBlock(md))
				item.Hash = milanote.HashText(md)
				newItems = append(newItems, item)
				continue
			}
			res.Conflicts = append(res.Conflicts, syncConflict{ID: item.ID, Text: preview(milanote.ElementMarkdown(el)), Reason: "removed from the file but edited on the board"})
			newItems = append(newItems, item)
		case push:
			res.note("trash", item.Kind, item.ID, milanote.ElementMarkdown(el))
			trash = append(trash, el)
		default:
			newItems = append(newItems, item)
		}
	}
	if len(trash) > 0 && !o.dryRun {
		w, err := writer.get(ctx)
		if err != nil {
			return mnErr(err)
		}
		if err := w.Trash(ctx, trash); err != nil {
			return mnErr(err)
		}
	}

	// Cards added on the board since pairing.
	if pull {
		known := map[string]bool{}
		for _, id := range rec.Ignored {
			known[id] = true
		}
		for _, it := range rec.Items {
			known[it.ID] = true
		}
		for _, it := range newItems {
			known[it.ID] = true
		}
		for _, el := range boardCards(payload, boardID) {
			if known[el.ID] {
				continue
			}
			md := milanote.ElementMarkdown(el)
			res.note("append-to-file", milanote.ChunkKindOf(el), el.ID, md)
			tail = append(tail, milanote.CardBlock(md))
			newItems = append(newItems, pairItem{ID: el.ID, Kind: milanote.ChunkKindOf(el), Hash: milanote.HashText(md)})
		}
	}

	if o.dryRun {
		return nil
	}
	if len(edits) > 0 || len(tail) > 0 {
		if dir, err := stateDir("mirror"); err == nil { // keep the previous text in case a pull surprises you
			_ = atomicWrite(filepath.Join(dir, milanote.HashText(file)+".bak.md"), []byte(text), 0o600)
		}
		if err := atomicWrite(file, []byte(applyLineEdits(text, edits, tail)), boardFileMode(file)); err != nil {
			return err
		}
	}
	rec.Items = newItems
	return savePair(rec)
}
