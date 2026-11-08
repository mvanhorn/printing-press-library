// Copyright 2026 Beau Sterling and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/milanote/internal/milanote"
)

func newNovelLayoutCheckCmd(flags *rootFlags) *cobra.Command {
	var tolerance float64

	cmd := &cobra.Command{
		Use:   "check <board>",
		Short: "Report off-grid cards, uneven gutters, overlaps, cards stuck in Unsorted, and text that should be a heading or list.",
		Long: `Scores a board's tidiness out of 100 and lists each problem with the card ids
involved. It changes nothing; use 'layout grid' to fix spacing. Card heights are
estimates, so very long cards can report small gap differences.`,
		Example:     "  milanote-pp-cli layout check 1Ab2Cd3EfGh4iJ --json",
		Annotations: readAnnotations("live"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "layout check")
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
			report := milanote.CheckBoard(payload, boardID, tolerance)
			if report.Issues == nil {
				report.Issues = []milanote.Issue{}
			}
			return emit(cmd, flags, report, func(w io.Writer) error {
				if _, err := fmt.Fprintf(w, "Tidiness %d/100 across %d card(s)\n", report.Score, report.Elements); err != nil {
					return err
				}
				for _, is := range report.Issues {
					if _, err := fmt.Fprintf(w, "  [%s] %s\n", is.Kind, is.Detail); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
	cmd.Flags().Float64Var(&tolerance, "tolerance", 2, "How many grid units off still counts as a near miss")
	cmd.Annotations["pp:verified-by-proof"] = "milanote-lifecycle.md"
	return cmd
}
