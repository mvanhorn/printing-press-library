// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/bento/internal/store"
	"github.com/spf13/cobra"
)

func newTagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "Local-data tag analytics that the upstream API does not expose",
		Long: `Operates over the snapshot history written by 'sync' runs. Use
'bento-pp-cli sync' on a schedule (cron, GitHub Actions) so drift has a
time series to compare against.`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTagsDriftCmd(flags))
	return cmd
}

func newTagsDriftCmd(flags *rootFlags) *cobra.Command {
	var window string
	var threshold float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Tags whose subscriber counts swung more than N% in the window",
		Example: strings.Trim(`
  # Tags that moved >25% week-over-week
  bento-pp-cli tags drift --window 7d --threshold 25

  # JSON output
  bento-pp-cli tags drift --window 24h --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute tag drift from snapshot history")
				return nil
			}
			windowDur, err := parseSinceDuration(window)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --window %q: %w", window, err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("bento-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			snaps, err := db.ListSnapshots("tags", windowDur, time.Time{})
			if err != nil {
				return fmt.Errorf("listing tag snapshots: %w", err)
			}
			if len(snaps) < 2 {
				if handled, herr := emptyLocalStoreOK(cmd, flags, "run 'bento-pp-cli sync --resources tags' twice with a delay to populate drift snapshots"); handled {
					return herr
				}
				return notFoundErr(fmt.Errorf("need at least 2 tag snapshots in the window; run 'bento-pp-cli sync --resources tags' more than once"))
			}

			first := snaps[0].TagCounts
			last := snaps[len(snaps)-1].TagCounts
			tagSet := map[string]bool{}
			for t := range first {
				tagSet[t] = true
			}
			for t := range last {
				tagSet[t] = true
			}

			type row struct {
				Tag       string  `json:"tag"`
				From      int     `json:"from"`
				To        int     `json:"to"`
				Delta     int     `json:"delta"`
				PctChange float64 `json:"pct_change"`
			}
			var rows []row
			for t := range tagSet {
				f := first[t]
				l := last[t]
				pct := 0.0
				if f > 0 {
					pct = float64(l-f) / float64(f) * 100
				} else if l > 0 {
					pct = 100
				}
				if math.Abs(pct) < threshold {
					continue
				}
				rows = append(rows, row{Tag: t, From: f, To: l, Delta: l - f, PctChange: pct})
			}
			sort.Slice(rows, func(i, j int) bool { return math.Abs(rows[i].PctChange) > math.Abs(rows[j].PctChange) })
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "7d", "Lookback window (e.g. 24h, 7d, 1w)")
	cmd.Flags().Float64Var(&threshold, "threshold", 10, "Minimum absolute percent change to flag")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/bento-pp-cli/data.db)")
	return cmd
}
