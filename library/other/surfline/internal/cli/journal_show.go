// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: review a spot's journal snapshots over time.
//
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelJournalShowCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "show <spotId>",
		Short: "Review a spot's journal snapshots over time.",
		Long: "Lists the forecast snapshots captured by 'journal log' for a spot, newest first.\n\n" +
			"Use this command to record and review forecast snapshots over time. Capture them with 'journal log'.",
		Example: strings.Trim(`
  surfline-pp-cli journal show 5842041f4e65fad6a7708807
  surfline-pp-cli journal show 5842041f4e65fad6a7708807 --limit 10 --agent`, "\n"),
		// A spotId with no snapshots is a valid empty result, not an error, so
		// skip the dogfood error-path probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a spotId argument is required"))
			}
			spotID := args[0]
			resolved := dbPath
			if resolved == "" {
				resolved = defaultDBPath(surflineDBName)
			}
			if _, statErr := os.Stat(resolved); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local journal at %s\nrun: surfline-pp-cli journal log %s\n", resolved, spotID)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			snaps, err := listJournalSnapshots(ctx, db, spotID, limit)
			if err != nil {
				return fmt.Errorf("reading journal: %w", err)
			}
			if snaps == nil {
				snaps = []journalSnapshot{}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), snaps, flags)
			}
			if len(snaps) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no snapshots for spot %s yet; capture one with: surfline-pp-cli journal log %s\n", spotID, spotID)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CAPTURED\tSURF\tSWELL\tWIND\tRATING")
			for _, s := range snaps {
				captured := localTime(s.CapturedAt, 0, "2006-01-02 15:04")
				fmt.Fprintf(tw, "%s\t%.0f-%.0fft\t%.1fft@%.0fs\t%.0fkt %s\t%s\n",
					captured, s.SurfMin, s.SurfMax, s.SwellHt, s.SwellPer, s.WindKts, s.WindType, s.RatingKey)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	cmd.Flags().IntVar(&limit, "limit", 30, "Max snapshots to show")
	return cmd
}
