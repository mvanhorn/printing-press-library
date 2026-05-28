// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"github.com/spf13/cobra"
)

func newNovelScoreDriftCmd(flags *rootFlags) *cobra.Command {
	var window string
	var dropPct float64
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "score-drift",
		Short: "Find contacts whose HubSpot score dropped meaningfully over a window — leading indicator before lifecycle catches up.",
		Long: strings.Trim(`
On every invocation this command snapshots the current hubspotscore for
every contact into the local contact_score_snapshots table, then compares
the current score against the oldest snapshot in [now-2*window, now-window].
Contacts whose score has dropped by at least --drop-pct percent are
returned, ranked by absolute drift.

First-run note: when no prior snapshots exist inside the window, the items
list will be empty and the response includes a 'note' explaining why; run
the command again after enough time has passed to populate history.
`, "\n"),
		Example: strings.Trim(`
  # Score drops of at least 25% over the last 14 days
  hubspot-pp-cli score-drift --window 14d --drop-pct 25 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			win, err := cliutil.ParseDurationLoose(window)
			if err != nil {
				return usageErr(fmt.Errorf("--window %q: %w", window, err))
			}
			if win <= 0 {
				return usageErr(fmt.Errorf("--window must be positive"))
			}
			if dropPct <= 0 {
				return usageErr(fmt.Errorf("--drop-pct must be > 0"))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be > 0"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("hubspot-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			now := time.Now().UTC()
			snapsTaken, err := snapshotScores(cmd.Context(), db, now)
			if err != nil {
				return fmt.Errorf("taking score snapshot: %w", err)
			}

			items, _, parseErrors, err := computeScoreDrift(cmd.Context(), db, now, win, dropPct, "drop")
			if err != nil {
				return err
			}
			matches := len(items)
			items = limitInt(items, limit)
			result := map[string]any{
				"window":          window,
				"drop_pct":        dropPct,
				"items":           items,
				"snapshots_taken": snapsTaken,
				"matches":         matches,
			}
			if parseErrors > 0 {
				result["parse_errors"] = parseErrors
			}
			if matches == 0 {
				// Distinguish "no prior data" from "no contact dropped enough".
				hasPrior, perr := scoreSnapshotHasPriorInWindow(cmd.Context(), db.DB(), now, win)
				if perr == nil && !hasPrior {
					result["note"] = "no prior score snapshots within window; run again after another sync cycle"
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&window, "window", "14d", "Lookback window for prior score snapshot comparison (e.g. 14d, 4w, 336h)")
	cmd.Flags().Float64Var(&dropPct, "drop-pct", 25, "Minimum percentage drop (current vs prior) to flag a contact")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of items to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override default SQLite database path")
	return cmd
}

// scoreSnapshotHasPriorInWindow reports whether at least one row exists
// inside [now-2*window, now-window]. Used to distinguish the first-run
// "no prior snapshots" note from "no matching contacts".
func scoreSnapshotHasPriorInWindow(ctx context.Context, db *sql.DB, now time.Time, window time.Duration) (bool, error) {
	upper := now.Add(-window).UTC().Format(time.RFC3339Nano)
	lower := now.Add(-2 * window).UTC().Format(time.RFC3339Nano)
	var count int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM contact_score_snapshots WHERE snapshot_at >= ? AND snapshot_at <= ?`,
		lower, upper,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
