// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelBalanceBurnCmd(flags *rootFlags) *cobra.Command {
	var flagSince string

	cmd := &cobra.Command{
		Use:   "burn",
		Short: "Diff local balance snapshots against local run volume over a window.",
		Example: strings.Trim(`
  parallel-pp-cli balance burn --since 7d --json --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}

			sinceStr := flagSince
			if sinceStr == "" {
				sinceStr = "7d"
			}
			sinceTime, err := parseSinceDuration(sinceStr)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", sinceStr, err))
			}

			db, err := openStoreForRead(cmd.Context(), "parallel-pp-cli")
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			if db == nil {
				out := map[string]any{
					"since":       sinceStr,
					"start_cents": nil,
					"end_cents":   nil,
					"delta_cents": 0,
					"run_count":   0,
					"note":        "no local store; run tasks guard or sync balance first",
				}
				return flags.printJSON(cmd, out)
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "service")
			hintIfStale(cmd, db, "service", flags.maxAge)

			oldest, newest, err := db.BalanceSnapshotWindow(sinceTime)
			if err != nil {
				return fmt.Errorf("balance burn: %w", err)
			}

			runCount, err := db.CountResourcesCreatedSince(sinceTime, []string{"tasks", "findall"})
			if err != nil {
				return fmt.Errorf("counting local runs: %w", err)
			}

			out := map[string]any{
				"since":       sinceStr,
				"run_count":   runCount,
				"delta_cents": 0,
			}

			if oldest == nil || newest == nil {
				out["start_cents"] = nil
				out["end_cents"] = nil
				out["note"] = "no balance_snapshots in window; run tasks guard to capture live balance"
				return flags.printJSON(cmd, out)
			}

			start := oldest.CreditBalanceCents
			end := newest.CreditBalanceCents
			out["start_cents"] = start
			out["end_cents"] = end
			out["delta_cents"] = start - end
			if oldest.ID == newest.ID {
				out["note"] = "only one balance snapshot in window"
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Duration window (e.g. 7d, 24h, 30m, 1w)")
	return cmd
}
