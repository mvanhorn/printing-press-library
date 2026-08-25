// Copyright 2026 Micah Baldwin and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fixed-length project progress accounting.
package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

// pp:data-source local
func newNovelProjectCmd(flags *rootFlags) *cobra.Command {
	var flagCollection, flagStart string
	var flagTarget int

	cmd := &cobra.Command{
		Use:   "project",
		Short: "Progress report for a fixed-length photo project: day N of target, missed days, projected finish.",
		Long: "Progress accounting for a finite dated project backed by a collection. Reports days completed against\n" +
			"--target, missed days since --start, completion percentage, and projected finish. 'streaks' is the\n" +
			"open-ended catalog-wide equivalent.",
		Example: strings.Trim(`
  lightroom-classic-pp-cli project --collection "100 Faces" --target 100 --json
  lightroom-classic-pp-cli project --collection "photo a day" --target 365 --start 2026-01-01`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--collection=faces;--target=100"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute project progress from the local catalog")
				return nil
			}
			if flagCollection == "" || flagTarget <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--collection and a positive --target are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			cat, err := openCatalog(ctx, flags)
			if err != nil {
				return err
			}
			defer cat.Close()
			rep, err := cat.Project(ctx, flagCollection, flagTarget, flagStart)
			if err != nil {
				return err
			}
			return emitLrcat(cmd, flags, rep, func(w io.Writer) {
				if rep.Note != "" {
					fmt.Fprintln(w, rep.Note)
					return
				}
				fmt.Fprintf(w, "%s: day %d of %d (%.0f%%), started %s\n",
					rep.Collection, rep.DaysWithPhotos, rep.Target, rep.PercentComplete, rep.Start)
				if len(rep.MissedDays) > 0 {
					fmt.Fprintf(w, "missed %d days (last: %s)\n", len(rep.MissedDays), rep.MissedDays[len(rep.MissedDays)-1])
				} else {
					fmt.Fprintln(w, "no missed days")
				}
				if rep.Complete {
					fmt.Fprintln(w, "project complete")
				} else if rep.ProjectedFinish != "" {
					fmt.Fprintf(w, "on daily pace, finishes %s\n", rep.ProjectedFinish)
				}
			})
		},
	}
	cmd.Flags().StringVar(&flagCollection, "collection", "", "Collection name (substring match)")
	cmd.Flags().IntVar(&flagTarget, "target", 0, "Project length in days, e.g. 100")
	cmd.Flags().StringVar(&flagStart, "start", "", "Project start YYYY-MM-DD (default: first capture day in the collection)")
	return cmd
}
