// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

type undoEntry struct {
	ID       string            `json:"id"`
	Location milanote.Location `json:"location"`
}

type undoRecord struct {
	Board string      `json:"board"`
	At    string      `json:"at"`
	Moves []undoEntry `json:"moves"`
}

func undoPath(board string) (string, error) {
	dir, err := stateDir("undo")
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, board+".json"), nil
}

type gridMove struct {
	ID   string  `json:"id"`
	Text string  `json:"text,omitempty"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

func newNovelLayoutGridCmd(flags *rootFlags) *cobra.Command {
	var cols int
	var gutter, x, y float64
	var fromInbox, undo, rows bool

	cmd := &cobra.Command{
		Use:   "grid <board>",
		Short: "Re-space a board's cards (or empty its Unsorted panel) onto an even grid with equal gutters",
		Long: `Moves the cards on a board onto an even grid in reading order. By default
each card takes the highest free spot, so every vertical gap equals the
gutter; --rows lines up the tops of each row instead. Columns are as wide as
most of the cards, and a wider card such as a banner heading spans several.
The old positions are saved, and --undo puts them back.`,
		Example: `  milanote-pp-cli layout grid 1Ab2Cd3EfGh4iJ --cols 4 --gutter 4 --dry-run
  milanote-pp-cli layout grid 1Ab2Cd3EfGh4iJ --from-inbox
  milanote-pp-cli layout grid 1Ab2Cd3EfGh4iJ --undo`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "layout grid")
			}
			if cols < 0 || gutter < 0 {
				return usageErr(errors.New("--cols and --gutter must not be negative"))
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
			if undo {
				return undoGrid(cmd, flags, c, boardID, payload)
			}
			els := []*milanote.Element{}
			for _, el := range payload.Children(boardID, milanote.SectionCanvas) {
				if milanote.OnCanvasGrid(el) {
					els = append(els, el)
				}
			}
			originX, originY := 2.0, 2.0
			if len(els) > 0 {
				originX, originY = math.Inf(1), math.Inf(1)
				for _, el := range els {
					originX = math.Min(originX, el.Location.X())
					originY = math.Min(originY, el.Location.Y())
				}
				originX, originY = milanote.Snap(originX), milanote.Snap(originY)
			}
			if fromInbox {
				els = append(els, payload.Children(boardID, milanote.SectionInbox)...)
			}
			if len(els) == 0 {
				return emit(cmd, flags, map[string]any{"board": boardID, "moved": 0, "moves": []gridMove{}}, func(w io.Writer) error {
					_, err := fmt.Fprintln(w, "Nothing to arrange on this board.")
					return err
				})
			}
			if cmd.Flags().Changed("x") {
				originX = x
			}
			if cmd.Flags().Changed("y") {
				originY = y
			}
			n := cols
			if n == 0 {
				n = int(math.Min(4, math.Ceil(math.Sqrt(float64(len(els))))))
			}
			items, widths := make([]milanote.PackItem, len(els)), make([]float64, len(els))
			for i, el := range els {
				b := milanote.BoxIn(payload, el)
				items[i], widths[i] = milanote.PackItem{W: b.W, H: b.H}, b.W
			}
			g := milanote.Grid{X: originX, Y: originY, Cols: n, ColW: milanote.CommonWidth(widths), Gutter: gutter}
			slots := g.Pack(items, rows)
			score := milanote.MaxScore(payload, boardID)
			moves := []milanote.Move{}
			preview := []gridMove{}
			rec := undoRecord{Board: boardID, At: time.Now().UTC().Format(time.RFC3339)}
			for i, el := range els {
				tx, ty := milanote.Snap(slots[i].X), milanote.Snap(slots[i].Y)
				onCanvas := el.Location.Section == milanote.SectionCanvas
				if onCanvas && el.Location.X() == tx && el.Location.Y() == ty {
					continue
				}
				s := el.Location.Score()
				if !onCanvas {
					score += milanote.ScoreStep
					s = score
				}
				moves = append(moves, milanote.Move{ID: el.ID, From: el.Location, To: milanote.CanvasAt(boardID, tx, ty, s)})
				preview = append(preview, gridMove{ID: el.ID, Text: el.Text(), X: tx, Y: ty})
				rec.Moves = append(rec.Moves, undoEntry{ID: el.ID, Location: el.Location})
			}
			result := map[string]any{"board": boardID, "board_url": boardURL(boardID), "cols": n, "gutter": gutter,
				"moved": len(moves), "unchanged": len(els) - len(moves), "dry_run": dryRunOK(flags), "moves": preview}
			if !dryRunOK(flags) && len(moves) > 0 {
				path, err := undoPath(boardID)
				if err != nil {
					return err
				}
				if err := writeJSONState(path, rec); err != nil {
					return err
				}
				w, err := c.Writer(ctx)
				if err != nil {
					return mnErr(err)
				}
				defer w.Close(ctx)
				if err := w.MoveMany(ctx, "DROP", moves); err != nil {
					return mnErr(err)
				}
			}
			return emit(cmd, flags, result, func(w io.Writer) error {
				verb := "moved"
				if dryRunOK(flags) {
					verb = "would move"
				}
				_, err := fmt.Fprintf(w, "%s %s %d card(s) onto a %d-column grid, gutter %g (%d already in place)\n", green("✓"), verb, len(moves), n, gutter, len(els)-len(moves))
				if err == nil && !dryRunOK(flags) && len(moves) > 0 {
					_, err = fmt.Fprintln(w, "  undo with: layout grid "+boardID+" --undo")
				}
				return err
			})
		},
	}
	cmd.Flags().IntVar(&cols, "cols", 0, "Number of columns (default: square-ish, at most 4)")
	cmd.Flags().Float64Var(&gutter, "gutter", milanote.DefaultGutter, "Gap between cards in grid units (1 unit = 9px)")
	cmd.Flags().Float64Var(&x, "x", 0, "Left edge of the grid (default: the leftmost card)")
	cmd.Flags().Float64Var(&y, "y", 0, "Top edge of the grid (default: the topmost card)")
	cmd.Flags().BoolVar(&fromInbox, "from-inbox", false, "Also bring in the cards from the Unsorted panel")
	cmd.Flags().BoolVar(&rows, "rows", false, "Line up the tops of each row instead of packing columns")
	cmd.Flags().BoolVar(&undo, "undo", false, "Put the cards back where the last grid found them")
	cmd.Annotations["pp:happy-args"] = "board=1Ab2Cd3EfGh4iJ;--cols=4;--dry-run"
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

func undoGrid(cmd *cobra.Command, flags *rootFlags, c *milanote.Client, boardID string, payload *milanote.BoardPayload) error {
	path, err := undoPath(boardID)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return notFoundErr(fmt.Errorf("no grid to undo on board %s", boardID))
	}
	if err != nil {
		return err
	}
	var rec undoRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return fmt.Errorf("undo record %s is damaged: %w", path, err)
	}
	moves := []milanote.Move{}
	missing := 0
	for _, m := range rec.Moves {
		el := liveCard(payload, m.ID)
		if el == nil {
			missing++
			continue
		}
		moves = append(moves, milanote.Move{ID: m.ID, From: el.Location, To: m.Location})
	}
	if !dryRunOK(flags) && len(moves) > 0 {
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		w, err := c.Writer(ctx)
		if err != nil {
			return mnErr(err)
		}
		defer w.Close(ctx)
		if err := w.MoveMany(ctx, "DROP", moves); err != nil {
			return mnErr(err)
		}
		_ = os.Remove(path)
	}
	return emit(cmd, flags, map[string]any{"board": boardID, "restored": len(moves), "missing": missing, "dry_run": dryRunOK(flags)}, func(w io.Writer) error {
		_, err := fmt.Fprintf(w, "%s put %d card(s) back (%d no longer on the board)\n", green("✓"), len(moves), missing)
		return err
	})
}
