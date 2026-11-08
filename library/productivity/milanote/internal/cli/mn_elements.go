// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		el := mnGroup(root, flags, "elements", "Move, colour, resize and trash cards")
		addNovelCommandIfAbsent(el, newElementsMoveCmd(flags))
		addNovelCommandIfAbsent(el, newElementsColorCmd(flags))
		addNovelCommandIfAbsent(el, newElementsWidthCmd(flags))
		addNovelCommandIfAbsent(el, newElementsTrashCmd(flags))
		labels := mnGroup(root, flags, "labels", "List and apply labels")
		addNovelCommandIfAbsent(labels, newLabelsAddCmd(flags))
		trash := mnGroup(root, flags, "trash", "List and restore trashed cards")
		addNovelCommandIfAbsent(trash, newTrashListCmd(flags))
		addNovelCommandIfAbsent(trash, newTrashRestoreCmd(flags))
		comments := mnGroup(root, flags, "comments", "Read and write board comments")
		addNovelCommandIfAbsent(comments, newCommentsListCmd(flags))
		addNovelCommandIfAbsent(comments, newCommentsAddCmd(flags))
		addNovelCommandIfAbsent(comments, newCommentsEditCmd(flags))
		addNovelCommandIfAbsent(comments, newCommentsDeleteCmd(flags))
	})
}

// findAll resolves element refs on a board, failing on the first miss.
func findAll(payload *milanote.BoardPayload, refs []string) ([]*milanote.Element, error) {
	out := make([]*milanote.Element, 0, len(refs))
	for _, ref := range refs {
		el, err := milanote.Find(payload, ref)
		if err != nil {
			return nil, notFoundErr(fmt.Errorf("element %s: %w", ref, err))
		}
		out = append(out, el)
	}
	return out, nil
}

func newElementsMoveCmd(flags *rootFlags) *cobra.Command {
	var x, y, dx, dy float64
	var toBoard, column string
	var unsorted bool
	cmd := &cobra.Command{
		Use:   "move <board> <element-id...>",
		Short: "Move cards to an exact x/y, by an offset, into a column or onto another board",
		Long: `Move one or more cards. --x/--y put the first card at that spot and keep the
others in the same arrangement relative to it; --dx/--dy shift every card;
--column appends them to a column in the order given; --to-board moves them
onto another board (at --x/--y there, or below its content).`,
		Example: `  milanote-pp-cli elements move 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --x 40 --y 12
  milanote-pp-cli elements move 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK 1Ab2Cd3EfGh4iL --dx 10
  milanote-pp-cli elements move 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --column 1Ab2Cd3EfGh4iM
  milanote-pp-cli elements move 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --to-board 1Ab2Cd3EfGh4iN`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "elements move")
			}
			if len(args) < 2 {
				return needArg(cmd, "at least one element id")
			}
			exact := cmd.Flags().Changed("x") || cmd.Flags().Changed("y")
			offset := cmd.Flags().Changed("dx") || cmd.Flags().Changed("dy")
			modes := 0
			for _, on := range []bool{exact && toBoard == "", offset, column != "", unsorted, toBoard != ""} {
				if on {
					modes++
				}
			}
			if modes != 1 {
				return usageErr(errors.New("choose exactly one of --x/--y, --dx/--dy, --column, --unsorted or --to-board"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			els, err := findAll(payload, args[1:])
			if err != nil {
				return err
			}
			score := milanote.MaxScore(payload, boardID)
			var moves []milanote.Move
			switch {
			case column != "":
				col, err := milanote.Find(payload, column)
				if err != nil {
					return notFoundErr(fmt.Errorf("column %s: %w", column, err))
				}
				if col.ElementType != milanote.TypeColumn {
					return usageErr(fmt.Errorf("%s is not a column", col.ID))
				}
				end := milanote.ColumnEnd(payload, col.ID)
				for i, el := range els {
					moves = append(moves, milanote.Move{ID: el.ID, From: el.Location,
						To: milanote.ListAt(col.ID, int(end.Index())+i, end.Score()+float64(i)*milanote.ScoreStep)})
				}
			case unsorted:
				top := milanote.Unsorted(payload, boardID)
				for i, el := range els {
					moves = append(moves, milanote.Move{ID: el.ID, From: el.Location,
						To: milanote.ListAt(boardID, i, top.Score()-float64(len(els)-i))})
				}
			case toBoard != "":
				dest := boardRef(toBoard)
				target, err := loadBoard(ctx, c, dest)
				if err != nil {
					return err
				}
				ox, oy := milanote.FreeOrigin(milanote.CanvasBoxes(target, dest), milanote.DefaultGutter)
				if exact {
					ox, oy = x, y
				}
				ax, ay := anchor(els)
				dscore := milanote.MaxScore(target, dest)
				for i, el := range els {
					b := milanote.BoxOf(el)
					moves = append(moves, milanote.Move{ID: el.ID, From: el.Location,
						To: milanote.CanvasAt(dest, milanote.Snap(ox+b.X-ax), milanote.Snap(oy+b.Y-ay), dscore+float64(i+1)*milanote.ScoreStep)})
				}
			default:
				ax, ay := anchor(els)
				if !cmd.Flags().Changed("x") {
					x = ax
				}
				if !cmd.Flags().Changed("y") {
					y = ay
				}
				if offset {
					x, y = ax+dx, ay+dy
				}
				for i, el := range els {
					b := milanote.BoxOf(el)
					if el.Location.Section != milanote.SectionCanvas {
						b.X, b.Y = ax, ay+float64(i)*(b.H+milanote.DefaultGutter)
					}
					moves = append(moves, milanote.Move{ID: el.ID, From: el.Location,
						To: milanote.CanvasAt(boardID, milanote.Snap(x+b.X-ax), milanote.Snap(y+b.Y-ay), score+float64(i+1)*milanote.ScoreStep)})
				}
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			if err := w.MoveMany(ctx, "DROP", moves); err != nil {
				return mnErr(err)
			}
			rows := make([]map[string]any, len(moves))
			for i, m := range moves {
				rows[i] = map[string]any{"id": m.ID, "parent_id": m.To.ParentID, "section": strings.ToLower(m.To.Section), "position": m.To.Position}
			}
			return emit(cmd, flags, map[string]any{"moved": len(moves), "elements": rows}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s moved %d element(s)\n", green("✓"), len(moves))
				return err
			})
		},
	}
	cmd.Flags().Float64Var(&x, "x", 0, "Target x in grid units for the first element")
	cmd.Flags().Float64Var(&y, "y", 0, "Target y in grid units for the first element")
	cmd.Flags().Float64Var(&dx, "dx", 0, "Shift every element right by this many units (negative = left)")
	cmd.Flags().Float64Var(&dy, "dy", 0, "Shift every element down by this many units (negative = up)")
	cmd.Flags().StringVar(&column, "column", "", "Append the elements to this column")
	cmd.Flags().BoolVar(&unsorted, "unsorted", false, "Move the elements into the Unsorted panel")
	cmd.Flags().StringVar(&toBoard, "to-board", "", "Move the elements onto another board")
	return cmd
}

// anchor is the top-left of a group of elements.
func anchor(els []*milanote.Element) (float64, float64) {
	ax, ay, set := 0.0, 0.0, false
	for _, el := range els {
		if el.Location.Section != milanote.SectionCanvas {
			continue
		}
		if !set || el.Location.X() < ax {
			ax = el.Location.X()
		}
		if !set || el.Location.Y() < ay {
			ay = el.Location.Y()
		}
		set = true
	}
	return ax, ay
}

func newElementsColorCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "color <board> <colour|none> <element-id...>",
		Short: "Set (or clear) the colour of one or more cards",
		Example: `  milanote-pp-cli elements color 1Ab2Cd3EfGh4iJ yellow 1Ab2Cd3EfGh4iK 1Ab2Cd3EfGh4iL
  milanote-pp-cli elements color 1Ab2Cd3EfGh4iJ none 1Ab2Cd3EfGh4iK`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "elements color")
			}
			if len(args) < 3 {
				return needArg(cmd, "colour and at least one element id")
			}
			val, err := milanote.ColorValue(args[1])
			if err != nil {
				return usageErr(err)
			}
			return updateEach(cmd, flags, args[0], args[2:], "colour", func(*milanote.Element) map[string]any {
				return map[string]any{"color": val}
			})
		},
	}
	return cmd
}

func newElementsWidthCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "width <board> <units|default> <element-id...>",
		Short:       "Set card width in grid units (34 = Milanote's default 304px)",
		Example:     "  milanote-pp-cli elements width 1Ab2Cd3EfGh4iJ 40 1Ab2Cd3EfGh4iK 1Ab2Cd3EfGh4iL",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "elements width")
			}
			if len(args) < 3 {
				return needArg(cmd, "width and at least one element id")
			}
			var width any
			if args[1] != "default" {
				var n float64
				if _, err := fmt.Sscanf(args[1], "%g", &n); err != nil || n < 8 || n > 200 {
					return usageErr(fmt.Errorf("width must be a number of grid units between 8 and 200, or 'default' (got %q)", args[1]))
				}
				width = milanote.Snap(n)
			}
			return updateEach(cmd, flags, args[0], args[2:], "width", func(*milanote.Element) map[string]any {
				return map[string]any{"width": width}
			})
		},
	}
	return cmd
}

// updateEach applies the same content change to several elements.
func updateEach(cmd *cobra.Command, flags *rootFlags, board string, refs []string, what string, change func(*milanote.Element) map[string]any) error {
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	c, err := mnClient(flags)
	if err != nil {
		return err
	}
	boardID := boardRef(board)
	payload, err := loadBoard(ctx, c, boardID)
	if err != nil {
		return err
	}
	els, err := findAll(payload, refs)
	if err != nil {
		return err
	}
	changes := make([]milanote.Change, len(els))
	for i, el := range els {
		changes[i] = milanote.Change{ID: el.ID, Changes: change(el)}
	}
	w, err := c.Writer(ctx)
	if err != nil {
		return mnErr(err)
	}
	defer w.Close(ctx)
	if err := w.Update(ctx, boardID, changes...); err != nil {
		return mnErr(err)
	}
	ids := make([]string, len(els))
	for i, el := range els {
		ids[i] = el.ID
	}
	return emit(cmd, flags, map[string]any{"updated": ids, "change": what}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s %s set on %d element(s)\n", green("✓"), what, len(ids))
		return err
	})
}

func newElementsTrashCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trash <board> <element-id...>",
		Short: "Move cards to the trash (restorable with 'trash restore')",
		Example: `  milanote-pp-cli elements trash 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK 1Ab2Cd3EfGh4iL
  milanote-pp-cli elements trash 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK --dry-run`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "elements trash")
			}
			if len(args) < 2 {
				return needArg(cmd, "at least one element id")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			els, err := findAll(payload, args[1:])
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			if err := w.Trash(ctx, els); err != nil {
				return mnErr(err)
			}
			ids := make([]string, len(els))
			for i, el := range els {
				ids[i] = el.ID
			}
			return emit(cmd, flags, map[string]any{"trashed": ids}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s trashed %d element(s); undo with: milanote-pp-cli trash restore %s\n", green("✓"), len(ids), strings.Join(ids, " "))
				return err
			})
		},
	}
	return cmd
}

func newLabelsAddCmd(flags *rootFlags) *cobra.Command {
	var color string
	cmd := &cobra.Command{
		Use:         "add <board> <element-id> <label>",
		Short:       "Apply a label to a card, creating the label if it does not exist",
		Example:     "  milanote-pp-cli labels add 1Ab2Cd3EfGh4iJ 1Ab2Cd3EfGh4iK \"needs review\"",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "labels add")
			}
			if len(args) < 3 || strings.TrimSpace(args[2]) == "" {
				return needArg(cmd, "element id and label name")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			el, err := milanote.Find(payload, args[1])
			if err != nil {
				return notFoundErr(fmt.Errorf("element %s: %w", args[1], err))
			}
			labels, err := c.Labels(ctx)
			if err != nil {
				return mnErr(err)
			}
			name := strings.TrimSpace(args[2])
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			labelID, created := "", false
			for _, l := range labels {
				if strings.EqualFold(l.Name, name) {
					labelID, name = l.ID, l.Name
				}
			}
			if labelID == "" {
				if labelID, err = w.CreateLabel(ctx, name, color); err != nil {
					return mnErr(err)
				}
				created = true
			}
			var current []string
			for _, v := range asAnyList(el.Content["labels"]) {
				if s, ok := v.(string); ok {
					current = append(current, s)
				}
			}
			already := false
			for _, id := range current {
				already = already || id == labelID
			}
			if !already {
				if err := w.SetLabels(ctx, milanote.BoardOf(payload, el.ID), el.ID, append(current, labelID), []string{labelID}); err != nil {
					return mnErr(err)
				}
			}
			result := map[string]any{"element_id": el.ID, "label_id": labelID, "label": name, "label_created": created, "already_applied": already}
			return emit(cmd, flags, result, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s label %q on %s\n", green("✓"), name, el.ID)
				return err
			})
		},
	}
	cmd.Flags().StringVar(&color, "label-color", "#ac2c75", "Hex colour for a newly created label")
	return cmd
}

func newTrashListCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List trashed cards with where they came from",
		Example:     "  milanote-pp-cli trash list --json --limit 20",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trash list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			payload, err := c.Trash(ctx)
			if err != nil {
				return mnErr(err)
			}
			rows := trashRows(payload)
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return emit(cmd, flags, rows, func(w io.Writer) error {
				if len(rows) == 0 {
					_, err := fmt.Fprintln(w, "Trash is empty.")
					return err
				}
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "ID\tTYPE\tTRASHED\tFROM BOARD\tTEXT")
				for _, r := range rows {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.Type, r.Trashed, r.From, truncate(r.Text, 50))
				}
				return tw.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Show at most this many (newest first)")
	return cmd
}

type trashRow struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Trashed string `json:"trashed_at"`
	From    string `json:"from_parent_id"`
	Text    string `json:"text"`
	added   float64
}

func trashRows(p *milanote.BoardPayload) []trashRow {
	rows := []trashRow{}
	for _, el := range p.Elements {
		if el.Location.Section != milanote.SectionTrash {
			continue
		}
		added, _ := el.Location.Position["addedDate"].(float64)
		r := trashRow{ID: el.ID, Type: strings.ToLower(el.ElementType), Text: el.Text(), added: added,
			From: strings.TrimSuffix(el.Location.ParentID, "-trash")}
		if added > 0 {
			r.Trashed = time.UnixMilli(int64(added)).UTC().Format(time.RFC3339)
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].added > rows[j].added })
	return rows
}

func newTrashRestoreCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "restore <element-id...>",
		Short:       "Put trashed cards back exactly where they were",
		Example:     "  milanote-pp-cli trash restore 1Ab2Cd3EfGh4iK 1Ab2Cd3EfGh4iL",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "trash restore")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			payload, err := c.Trash(ctx)
			if err != nil {
				return mnErr(err)
			}
			els, err := findAll(payload, args)
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			if err := w.Restore(ctx, els); err != nil {
				return mnErr(err)
			}
			ids := make([]string, len(els))
			for i, el := range els {
				ids[i] = el.ID
			}
			return emit(cmd, flags, map[string]any{"restored": ids}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s restored %d element(s)\n", green("✓"), len(ids))
				return err
			})
		},
	}
	return cmd
}

type commentRow struct {
	ID        string `json:"id"`
	ThreadID  string `json:"thread_id"`
	ElementID string `json:"element_id"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at,omitempty"`
	Text      string `json:"text"`
}

func commentRows(p *milanote.BoardPayload) []commentRow {
	out := []commentRow{}
	for _, cm := range p.CommentList() {
		r := commentRow{Text: milanote.ToMarkdown(cm["text"])}
		r.ID, _ = cm["_id"].(string)
		r.ThreadID, _ = cm["threadId"].(string)
		r.ElementID, _ = cm["elementId"].(string)
		r.UserID, _ = cm["userId"].(string)
		if ts, ok := cm["createdAt"].(float64); ok {
			r.CreatedAt = time.UnixMilli(int64(ts)).UTC().Format(time.RFC3339)
		}
		out = append(out, r)
	}
	return out
}

func findComment(p *milanote.BoardPayload, ref string) (commentRow, error) {
	var hits []commentRow
	for _, r := range commentRows(p) {
		if r.ID == ref || strings.HasPrefix(r.ID, ref) || strings.HasPrefix(r.ID, "cm_"+ref) {
			hits = append(hits, r)
		}
	}
	switch len(hits) {
	case 1:
		return hits[0], nil
	case 0:
		return commentRow{}, notFoundErr(fmt.Errorf("no comment %s on this board", ref))
	}
	return commentRow{}, usageErr(fmt.Errorf("%q matches %d comments; use more of the id", ref, len(hits)))
}

func newCommentsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list <board>",
		Short:       "List a board's comments, oldest first",
		Example:     "  milanote-pp-cli comments list 1Ab2Cd3EfGh4iJ --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "comments list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			payload, err := loadBoard(ctx, c, args[0])
			if err != nil {
				return err
			}
			rows := commentRows(payload)
			return emit(cmd, flags, rows, func(w io.Writer) error {
				if len(rows) == 0 {
					_, err := fmt.Fprintln(w, "No comments on this board.")
					return err
				}
				for _, r := range rows {
					fmt.Fprintf(w, "%s  %s  (thread %s)\n  %s\n", r.CreatedAt, r.ID, r.ThreadID, strings.ReplaceAll(r.Text, "\n", "\n  "))
				}
				return nil
			})
		},
	}
	return cmd
}

func newCommentsAddCmd(flags *rootFlags) *cobra.Command {
	var replyTo string
	var place placeFlags
	cmd := &cobra.Command{
		Use:   "add <board> <markdown|-|@file>",
		Short: "Start a comment thread (or --reply-to an existing one)",
		Example: `  milanote-pp-cli comments add 1Ab2Cd3EfGh4iJ "Can we swap scenes 3 and 4?" --after 1Ab2Cd3EfGh4iK
  milanote-pp-cli comments add 1Ab2Cd3EfGh4iJ "Agreed" --reply-to cm_1Ab2Cd3EfGh4iL`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "comments add")
			}
			if len(args) < 2 {
				return needArg(cmd, "comment text")
			}
			text, err := readTextArg(cmd, args[1])
			if err != nil {
				return err
			}
			if strings.TrimSpace(text) == "" {
				return usageErr(errors.New("comment text must not be empty"))
			}
			if err := place.validate(); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			threadID := ""
			var loc milanote.Location
			if replyTo != "" {
				if el := payload.Elements[replyTo]; el != nil && el.ElementType == milanote.TypeCommentThread {
					threadID = el.ID
				} else {
					cm, err := findComment(payload, replyTo)
					if err != nil {
						return err
					}
					threadID = cm.ThreadID
				}
			} else if place.exact() || place.after != "" || place.column != "" {
				if loc, err = place.resolve(payload, boardID); err != nil {
					return err
				}
			} else {
				loc = milanote.Unsorted(payload, boardID)
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			id, thread, err := w.AddComment(ctx, boardID, threadID, loc, text)
			if err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"comment_id": id, "thread_id": thread, "board_id": boardID}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s comment %s posted (thread %s)\n", green("✓"), id, thread)
				return err
			})
		},
	}
	place.bind(cmd)
	cmd.Flags().StringVar(&replyTo, "reply-to", "", "Reply in the thread of this comment id or thread element id")
	return cmd
}

func newCommentsEditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "edit <board> <comment-id> <markdown|-|@file>",
		Short:       "Replace a comment's text",
		Example:     "  milanote-pp-cli comments edit 1Ab2Cd3EfGh4iJ cm_1Ab2Cd3EfGh4iL \"Swap 3 and 5 instead\"",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "comments edit")
			}
			if len(args) < 3 {
				return needArg(cmd, "comment id and text")
			}
			text, err := readTextArg(cmd, args[2])
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			cm, err := findComment(payload, args[1])
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			elementID := cm.ElementID
			if elementID == "" {
				elementID = cm.ThreadID
			}
			if err := w.EditComment(ctx, boardID, cm.ID, elementID, text); err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"comment_id": cm.ID, "text": text}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s comment %s updated\n", green("✓"), cm.ID)
				return err
			})
		},
	}
	return cmd
}

func newCommentsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete <board> <comment-id>",
		Short:       "Delete a comment",
		Example:     "  milanote-pp-cli comments delete 1Ab2Cd3EfGh4iJ cm_1Ab2Cd3EfGh4iL",
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "comments delete")
			}
			if len(args) < 2 {
				return needArg(cmd, "comment id")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			boardID := boardRef(args[0])
			payload, err := loadBoard(ctx, c, boardID)
			if err != nil {
				return err
			}
			cm, err := findComment(payload, args[1])
			if err != nil {
				return err
			}
			w, err := c.Writer(ctx)
			if err != nil {
				return mnErr(err)
			}
			defer w.Close(ctx)
			elementID := cm.ElementID
			if elementID == "" {
				elementID = cm.ThreadID
			}
			if err := w.DeleteComment(ctx, boardID, cm.ID, elementID, cm.ThreadID); err != nil {
				return mnErr(err)
			}
			return emit(cmd, flags, map[string]any{"deleted": cm.ID}, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "%s comment %s deleted\n", green("✓"), cm.ID)
				return err
			})
		},
	}
	return cmd
}
