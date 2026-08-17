// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: shooting-habit histograms.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/lightroom-classic/internal/lrcat"
)

// pp:data-source local
func newNovelStatsCmd(flags *rootFlags) *cobra.Command {
	var flagBy, flagSince string

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Histograms of your shooting by focal length, hour, weekday, month, camera, lens, or ISO.",
		Long: "Aggregation command: returns bucketed counts, never image rows. --by camera and --by lens include\n" +
			"first-seen/last-seen capture dates via the cameras/lenses commands. Use 'photos' to get the images themselves.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli stats --by camera
  lightroom-classic-pp-cli stats --by lens --since 2025-01-01
  lightroom-classic-pp-cli stats --by weekday --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--by=camera"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate shooting stats from the local catalog")
				return nil
			}
			if flagBy == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by is required (one of: %s)", strings.Join(lrcat.StatDimensions, ", ")))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			buckets, err := cat.Stats(ctx, flagBy, flagSince)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, buckets, func(w io.Writer) {
				var max int64
				keyWidth := 28
				for _, b := range buckets {
					if b.Count > max {
						max = b.Count
					}
					if len(b.Key) > keyWidth {
						keyWidth = len(b.Key)
					}
				}
				for _, b := range buckets {
					bar := ""
					if max > 0 {
						bar = strings.Repeat("█", int(b.Count*30/max))
					}
					fmt.Fprintf(w, "%-*s %7d  %s\n", keyWidth, b.Key, b.Count, bar)
				}
			})
		},
	}
	cmd.Flags().StringVar(&flagBy, "by", "", "Dimension: "+strings.Join(lrcat.StatDimensions, " | "))
	cmd.Flags().StringVar(&flagSince, "since", "", "Only captures on/after this date (YYYY-MM-DD)")
	return cmd
}
