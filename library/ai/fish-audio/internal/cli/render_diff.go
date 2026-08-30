// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: compare two rows of the local render log.
// pp:data-source local
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/store"
	"github.com/spf13/cobra"
)

// renderDiffField is one compared attribute of two renders.
type renderDiffField struct {
	Field   string `json:"field"`
	Left    string `json:"left"`
	Right   string `json:"right"`
	Changed bool   `json:"changed"`
	// Delta is right minus left for the numeric fields, formatted the same
	// way the field itself is. It is empty for text fields.
	Delta string `json:"delta,omitempty"`
}

// renderDiffView is the JSON contract `render diff` prints.
type renderDiffView struct {
	LeftID  int64             `json:"left_id"`
	RightID int64             `json:"right_id"`
	Fields  []renderDiffField `json:"fields"`
	Changed int               `json:"changed"`
}

func newNovelRenderDiffCmd(flags *rootFlags) *cobra.Command {
	var flagDB string

	cmd := &cobra.Command{
		Use:   "diff [id1] [id2]",
		Short: "Show the cost, model, and byte deltas between two past renders.",
		Long: `Compares two rows of the local render log by their log id and reports which
attributes changed. Use it to pick between two takes of the same line.

Both ids come from 'render log'. Two identical renders report no changed
fields rather than inventing a difference.

Use 'render log' for the full list or 'render spend' for totals.`,
		Example: strings.Trim(`
  fish-audio-pp-cli render diff 1 2
  fish-audio-pp-cli render diff 1 2 --json
  fish-audio-pp-cli render diff 1 2 --db ./renders.db --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "id1=1;id2=2",
			"pp:typed-exit-codes": "0,2,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// The dry-run short-circuit comes before the positional gate so a
			// verify probe of `render diff --dry-run` gets an envelope instead
			// of a usage error for arguments it was never going to use.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "render diff")
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("render diff needs two render log ids; run 'render log' to find them"))
			}
			leftID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid render log id %q: expected a number from 'render log'", args[0]))
			}
			rightID, err := strconv.ParseInt(args[1], 10, 64)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid render log id %q: expected a number from 'render log'", args[1]))
			}

			dbPath := fishRenderDBPath(flagDB)
			if stop, mirrorErr := fishMissingMirror(cmd, flags, dbPath, renderDiffView{LeftID: leftID, RightID: rightID, Fields: make([]renderDiffField, 0)}); stop {
				return mirrorErr
			}
			db, err := openRenderStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			left, err := db.RenderRowByID(cmd.Context(), leftID)
			if err != nil {
				return err
			}
			right, err := db.RenderRowByID(cmd.Context(), rightID)
			if err != nil {
				return err
			}
			if left == nil || right == nil {
				missing := leftID
				if left != nil {
					missing = rightID
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "render log has no row %d; run 'render log' to list the ids that exist\n", missing)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if printErr := printJSONFiltered(cmd.OutOrStdout(), renderDiffView{LeftID: leftID, RightID: rightID, Fields: make([]renderDiffField, 0)}, flags); printErr != nil {
						return printErr
					}
				}
				// A diff over ids that are not in the local log is an empty local result,
				// the same contract as 'render log' on an empty mirror: exit 0, empty fields.
				_ = missing
				return nil
			}

			view := buildRenderDiff(left, right)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "FIELD\t#%d\t#%d\tDELTA\n", leftID, rightID)
			for _, f := range view.Fields {
				marker := ""
				if f.Changed {
					marker = f.Delta
					if marker == "" {
						marker = "changed"
					}
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", f.Field, f.Left, f.Right, marker)
			}
			if err := w.Flush(); err != nil {
				return err
			}
			if view.Changed == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "The two renders are identical on every compared field.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// buildRenderDiff compares two rows field by field.
func buildRenderDiff(left, right *store.RenderRow) renderDiffView {
	view := renderDiffView{LeftID: left.ID, RightID: right.ID, Fields: make([]renderDiffField, 0, 9)}
	addText := func(name, l, r string) {
		view.Fields = append(view.Fields, renderDiffField{Field: name, Left: l, Right: r, Changed: l != r})
	}
	addInt := func(name string, l, r int64) {
		view.Fields = append(view.Fields, renderDiffField{
			Field:   name,
			Left:    strconv.FormatInt(l, 10),
			Right:   strconv.FormatInt(r, 10),
			Changed: l != r,
			Delta:   formatSignedInt(r - l),
		})
	}
	addMoney := func(name string, l, r float64) {
		view.Fields = append(view.Fields, renderDiffField{
			Field:   name,
			Left:    fmt.Sprintf("%.6f", l),
			Right:   fmt.Sprintf("%.6f", r),
			Changed: l != r,
			Delta:   formatSignedFloat(r - l),
		})
	}
	addText("created_at", left.CreatedAt, right.CreatedAt)
	addText("model", left.Model, right.Model)
	addText("voice_id", left.VoiceID, right.VoiceID)
	addText("format", left.Format, right.Format)
	addText("text", truncate(strings.ReplaceAll(left.Text, "\n", " "), 60), truncate(strings.ReplaceAll(right.Text, "\n", " "), 60))
	addInt("bytes_in", left.BytesIn, right.BytesIn)
	addInt("bytes_out", left.BytesOut, right.BytesOut)
	addMoney("cost_usd", left.CostUSD, right.CostUSD)
	addMoney("cost_usd_paid_equiv", left.CostUSDPaidEquiv, right.CostUSDPaidEquiv)
	addText("file_sha256", left.FileSHA256, right.FileSHA256)
	for _, f := range view.Fields {
		if f.Changed {
			view.Changed++
		}
	}
	return view
}

// formatSignedInt renders a delta with an explicit sign so a table row reads
// as a direction, not just a number.
func formatSignedInt(v int64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%+d", v)
}

// formatSignedFloat is formatSignedInt for money values.
func formatSignedFloat(v float64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%+.6f", v)
}
