// `apify-pp-cli cost report` — per-run USD ledger over historical runs.
//
// Joins cached run metadata (compute_units, memory_mbytes, duration_secs)
// with the cost.Estimate() formula to produce a per-run rollup. Apify's
// dashboard shows monthly aggregates; this surfaces the per-actor, per-run
// breakdown that's load-bearing for newsletter cost discipline.
//
//	apify-pp-cli cost report --since 30d --group-by actor --json
//	apify-pp-cli cost report --since 7d --group-by actor --csv
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/cost"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/apify/internal/store"
)

func newCostCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cost",
		Short: "Cost reporting and budgeting helpers (run history + Apify usage)",
		Long: strings.Trim(`
Cost utilities for the Apify platform — built on the local pp_actor_run_history
table populated by 'apify-pp-cli run' and 'apify-pp-cli sync'.

Apify bills in compute units (CU) + RAM-GB-hours + storage + transfer + PPE.
Surprise bills are the #1 G2 complaint about the platform. 'cost report'
turns the raw numbers into a per-run, per-actor USD rollup.

Subcommands:
  report   Per-actor (or per-schedule, per-day) USD rollup over a window
`, "\n"),
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCostReportCmd(flags))
	return cmd
}

func newCostReportCmd(flags *rootFlags) *cobra.Command {
	var (
		sinceStr string
		groupBy  string
		actorID  string
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Per-run USD rollup over historical Actor runs",
		Long: strings.Trim(`
Rolls up cached run history into a USD ledger. Each row shows total cost,
average per run, and total compute units for the chosen grouping.

Examples:
  apify-pp-cli cost report --since 30d --group-by actor
  apify-pp-cli cost report --since 7d --json
  apify-pp-cli cost report --actor apidojo/twitter-scraper --since 90d
`, "\n"),
		Example: strings.Trim(`
  apify-pp-cli cost report --since 30d --group-by actor --json
  apify-pp-cli cost report --since 7d --csv
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("apify-pp-cli"))
			if err != nil {
				return configErr(fmt.Errorf("opening local store: %w", err))
			}
			defer db.Close()
			if err := db.EnsureExtensions(ctx); err != nil {
				return configErr(fmt.Errorf("ensuring extensions: %w", err))
			}

			history, err := db.LoadActorRunHistory(ctx, actorID, 0)
			if err != nil {
				return apiErr(fmt.Errorf("loading run history: %w", err))
			}

			// Filter by --since
			sinceDur := parseSinceWindow(sinceStr)
			if sinceDur > 0 {
				cutoff := time.Now().Add(-sinceDur)
				filtered := history[:0]
				for _, r := range history {
					if r.StartedAt.IsZero() || r.StartedAt.After(cutoff) {
						filtered = append(filtered, r)
					}
				}
				history = filtered
			}

			stats := historyToStats(history)
			rows := cost.Rollup(stats, groupBy)

			var totalUSD, totalCU float64
			var totalRuns int
			for _, r := range rows {
				totalUSD += r.TotalUSD
				totalCU += r.TotalCU
				totalRuns += r.RunCount
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"window":     prettyDuration(sinceDur),
				"group_by":   groupBy,
				"total_usd":  totalUSD,
				"total_cu":   totalCU,
				"total_runs": totalRuns,
				"rows":       rows,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&sinceStr, "since", "30d", "Time window (e.g. 7d, 30d, 90d). Empty disables.")
	cmd.Flags().StringVar(&groupBy, "group-by", "actor", "Rollup grouping: actor | day (more groupings later)")
	cmd.Flags().StringVar(&actorID, "actor", "", "Restrict to one Actor")
	return cmd
}
