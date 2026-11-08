// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"errors"
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelLayoutMdCmd(flags *rootFlags) *cobra.Command {
	var board string
	var cols int
	var gutter, width, x, y float64
	var update bool

	cmd := &cobra.Command{
		Use:   "md <file.md>",
		Short: "Turn a Markdown file into native heading cards, rich-text notes, bullet lists and checkbox lists in evenly spaced columns.",
		Long: `Lays a Markdown document out on a board. Every # heading starts a centred
title across the columns, every ## heading starts a section stacked in its
own column, "- [ ]" lines become real to-do lists, and everything else becomes
rich-text notes. --- ends a note without making a card.

Run it again with --update to change the same cards in place: edited text is
updated, new sections are added and removed sections go to the trash. Cards you
edited on the board are left alone and reported.`,
		Example: `  milanote-pp-cli layout md script.md --board 1Ab2Cd3EfGh4iJ --cols 3 --gutter 4
  milanote-pp-cli layout md script.md --board 1Ab2Cd3EfGh4iJ --update
  milanote-pp-cli layout md script.md --board 1Ab2Cd3EfGh4iJ --dry-run --json`,
		Annotations: writeAnnotations(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) && requiredInputWaived() {
				return writeDryRun(cmd.OutOrStdout(), flags, "layout md")
			}
			if board == "" {
				return usageErr(errors.New("--board is required"))
			}
			if gutter < 0 || width <= 0 || cols < 0 {
				return usageErr(errors.New("--cols, --gutter and --width must be positive"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := mnClient(flags)
			if err != nil {
				return err
			}
			opts := syncOptions{direction: "push", cols: cols, colW: width, gutter: gutter, dryRun: dryRunOK(flags),
				originSet: cmd.Flags().Changed("x") || cmd.Flags().Changed("y"), x: x, y: y}
			res, err := runSync(ctx, c, args[0], boardRef(board), opts, !update)
			if err != nil {
				return err
			}
			return printSync(cmd, flags, res)
		},
	}
	cmd.Flags().StringVar(&board, "board", "", "Board id or URL to lay the document out on")
	cmd.Flags().IntVar(&cols, "cols", 0, "Number of columns (default: one per section, at most 4)")
	cmd.Flags().Float64Var(&gutter, "gutter", milanote.DefaultGutter, "Gap between cards in grid units (1 unit = 9px)")
	cmd.Flags().Float64Var(&width, "width", milanote.CardWidth, "Column width in grid units")
	cmd.Flags().Float64Var(&x, "x", 0, "Left edge of the layout (default: below what is on the board)")
	cmd.Flags().Float64Var(&y, "y", 0, "Top edge of the layout")
	cmd.Flags().BoolVar(&update, "update", false, "Update the cards made by an earlier run instead of adding new ones")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}

// printSync reports a sync run.
func printSync(cmd *cobra.Command, flags *rootFlags, res *mirrorResult) error {
	return emit(cmd, flags, res, func(w io.Writer) error {
		verb := "synced"
		if res.DryRun {
			verb = "would sync"
		}
		counts := map[string]int{}
		for _, a := range res.Actions {
			counts[a.Action]++
		}
		if res.DryRun && len(res.Actions) > 0 {
			tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTION\tKIND\tX\tY\tTEXT")
			for _, a := range res.Actions {
				fmt.Fprintf(tw, "%s\t%s\t%g\t%g\t%s\n", a.Action, a.Kind, a.X, a.Y, a.Text)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
		}
		summary := ""
		for _, k := range []string{"create", "update", "trash", "pull", "append-to-file", "remove-from-file"} {
			if counts[k] > 0 {
				summary += fmt.Sprintf(" %s=%d", k, counts[k])
			}
		}
		if summary == "" {
			summary = " nothing to change"
		}
		if _, err := fmt.Fprintf(w, "%s %s %s ↔ %s:%s\n", green("✓"), verb, res.File, res.URL, summary); err != nil {
			return err
		}
		for _, a := range res.Actions {
			if len(a.Action) > 5 && a.Action[:5] == "skip-" {
				fmt.Fprintf(w, "  left alone (%s): %s\n", a.Action[5:], a.Text)
			}
		}
		for _, conflict := range res.Conflicts {
			fmt.Fprintf(w, "  %s conflict: %s (%s)\n", yellow("!"), conflict.Text, conflict.Reason)
		}
		if len(res.Conflicts) > 0 {
			fmt.Fprintln(w, "  Settle these by editing one side, or rerun with --prefer file|board.")
		}
		return nil
	})
}
