// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelLayoutCsvCmd(flags *rootFlags) *cobra.Command {
	var board, titleCol string
	var cols int
	var gutter, width float64
	var number bool

	cmd := &cobra.Command{
		Use:   "csv <file.csv>",
		Short: "Turn spreadsheet rows such as a shot list into a numbered, evenly spaced grid of cards.",
		Long: `Makes one note per row. The title column becomes the card's heading and every
other non-empty column becomes a "Label: value" line. Rows keep their order,
left to right and top to bottom.`,
		Example: `  milanote-pp-cli layout csv shots.csv --board 1Ab2Cd3EfGh4iJ --title-col Shot --cols 4
  milanote-pp-cli layout csv shots.csv --board 1Ab2Cd3EfGh4iJ --number=false --dry-run`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "layout csv")
			}
			if board == "" {
				return usageErr(errors.New("--board is required"))
			}
			if cols < 1 || gutter < 0 || width <= 0 {
				return usageErr(errors.New("--cols and --width must be positive and --gutter must not be negative"))
			}
			raw, err := readLocalFile(args[0])
			if err != nil {
				return usageErr(err)
			}
			docs, titles, err := csvCards(raw, titleCol, number)
			if err != nil {
				return usageErr(err)
			}
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
			x, y := milanote.FreeOrigin(milanote.CanvasBoxes(payload, boardID), gutter*2)
			heights := make([]float64, len(docs))
			for i, d := range docs {
				heights[i] = milanote.NoteHeight(d, width)
			}
			slots := milanote.Grid{X: x, Y: y, Cols: cols, ColW: width, Gutter: gutter}.Rows(heights)
			rows := make([]map[string]any, len(docs))
			for i := range docs {
				rows[i] = map[string]any{"title": titles[i], "x": slots[i].X, "y": slots[i].Y}
			}
			if !dryRunOK(flags) {
				w, err := c.Writer(ctx)
				if err != nil {
					return mnErr(err)
				}
				defer w.Close(ctx)
				score := milanote.MaxScore(payload, boardID)
				for i, d := range docs {
					content := milanote.NoteContent(d, "")
					if width != milanote.CardWidth {
						content["width"] = milanote.Snap(width)
					}
					score += milanote.ScoreStep
					id, err := w.Create(ctx, boardID, milanote.TypeCard, milanote.CanvasAt(boardID, slots[i].X, slots[i].Y, score), content, "")
					if err != nil {
						return mnErr(fmt.Errorf("row %d of %d: %w", i+1, len(docs), err))
					}
					rows[i]["id"] = id
				}
			}
			return emit(cmd, flags, map[string]any{"board": boardID, "board_url": boardURL(boardID), "cards": rows, "dry_run": dryRunOK(flags)}, func(w io.Writer) error {
				verb := "added"
				if dryRunOK(flags) {
					verb = "would add"
				}
				_, err := fmt.Fprintf(w, "%s %s %d card(s) on a %d-column grid: %s\n", green("✓"), verb, len(docs), cols, boardURL(boardID))
				return err
			})
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Board id or URL to add the cards to")
	cmd.Flags().StringVar(&titleCol, "title-col", "", "Column used as the card heading (default: the first column)")
	cmd.Flags().IntVar(&cols, "cols", 4, "Number of columns")
	cmd.Flags().Float64Var(&gutter, "gutter", milanote.DefaultGutter, "Gap between cards in grid units (1 unit = 9px)")
	cmd.Flags().Float64Var(&width, "width", milanote.CardWidth, "Card width in grid units")
	cmd.Flags().BoolVar(&number, "number", true, "Prefix each heading with its row number")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

// csvCards builds one note document per data row.
func csvCards(raw []byte, titleCol string, number bool) ([]map[string]any, []string, error) {
	r := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))))
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, nil, fmt.Errorf("reading CSV: %w", err)
	}
	if len(records) < 2 {
		return nil, nil, errors.New("the CSV needs a header row and at least one data row")
	}
	header := records[0]
	titleIdx := 0
	if titleCol != "" {
		titleIdx = -1
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), strings.TrimSpace(titleCol)) {
				titleIdx = i
			}
		}
		if titleIdx < 0 {
			return nil, nil, fmt.Errorf("no column named %q (found: %s)", titleCol, strings.Join(header, ", "))
		}
	}
	var docs []map[string]any
	var titles []string
	for n, row := range records[1:] {
		title := ""
		if titleIdx < len(row) {
			title = strings.TrimSpace(row[titleIdx])
		}
		if number {
			title = strings.TrimSpace(fmt.Sprintf("%02d %s", n+1, title))
		}
		blocks := []map[string]any{milanote.Heading(2, title, "left")}
		for i, cell := range row {
			cell = strings.TrimSpace(cell)
			if i == titleIdx || cell == "" || i >= len(header) {
				continue
			}
			blocks = append(blocks, milanote.Paragraph("**"+strings.TrimSpace(header[i])+":** "+cell))
		}
		docs = append(docs, milanote.Doc(blocks...))
		titles = append(titles, title)
	}
	return docs, titles, nil
}
