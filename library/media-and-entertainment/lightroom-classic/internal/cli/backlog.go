// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: keepers with no develop work yet.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelBacklogCmd(flags *rootFlags) *cobra.Command {
	var flagMinRating float64
	var flagPickedOnly bool
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "backlog",
		Short: "Keepers you flagged or rated but never developed.",
		Long: "Lists flagged picks (and images rated at or above --min-rating) that have no develop adjustments —\n" +
			"workflow debt Lightroom smart collections cannot express. Requires the develop-settings join, so this\n" +
			"predicate is not available as a 'photos' filter.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli backlog --picked-only --json
  lightroom-classic-pp-cli backlog --min-rating 4 --limit 20`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--min-rating=3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list undeveloped keepers from the local catalog")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			photos, err := cat.Backlog(ctx, flagMinRating, flagPickedOnly, flagLimit)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, photos, func(w io.Writer) {
				for _, p := range photos {
					photoLine(w, p)
				}
				fmt.Fprintf(w, "%d undeveloped keepers\n", len(photos))
			})
		},
	}
	cmd.Flags().Float64Var(&flagMinRating, "min-rating", 3, "Include images rated at or above this (ignored with --picked-only)")
	cmd.Flags().BoolVar(&flagPickedOnly, "picked-only", false, "Only flagged picks")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum images returned")
	return cmd
}
