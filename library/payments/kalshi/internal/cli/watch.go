// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/store"
)

// newWatchCmd is the parent for watchlist commands. The watchlist is a
// local-only convenience: tickers in it can be diffed against per-sync price
// snapshots without re-typing them every run. No external API calls.
func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Maintain a local watchlist of tickers and diff against price snapshots",
	}
	cmd.AddCommand(newWatchAddCmd(flags))
	cmd.AddCommand(newWatchListCmd(flags))
	cmd.AddCommand(newWatchRemoveCmd(flags))
	cmd.AddCommand(newWatchDiffCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "add <ticker>",
		Short:   "Add a ticker to the local watchlist",
		Example: `  kalshi-pp-cli watch add KXTSLA-26-T125`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				if dryRunOK(flags) {
					return nil
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("requires <ticker> argument; see --help")
			}
			ticker := args[0]
			if ticker == "__printing_press_invalid__" {
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid ticker placeholder")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if _, err := db.DB().Exec(
				`INSERT OR IGNORE INTO watchlist (ticker, added_at) VALUES (?, ?)`,
				ticker, time.Now().Unix(),
			); err != nil {
				return fmt.Errorf("inserting watchlist row: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"added": true, "ticker": ticker}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watching %s\n", ticker)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List tickers in the local watchlist",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  kalshi-pp-cli watch list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().Query(`SELECT ticker, added_at FROM watchlist ORDER BY added_at ASC`)
			if err != nil {
				return fmt.Errorf("querying watchlist: %w", err)
			}
			defer rows.Close()

			type entry struct {
				Ticker  string `json:"ticker"`
				AddedAt int64  `json:"added_at"`
			}
			var entries []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Ticker, &e.AddedAt); err != nil {
					return err
				}
				entries = append(entries, e)
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add a ticker with 'watch add <ticker>'.")
				return nil
			}
			headers := []string{"Ticker", "Added"}
			tableRows := make([][]string, 0, len(entries))
			for _, e := range entries {
				tableRows = append(tableRows, []string{e.Ticker, time.Unix(e.AddedAt, 0).UTC().Format(time.RFC3339)})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:     "remove <ticker>",
		Aliases: []string{"rm"},
		Short:   "Remove a ticker from the local watchlist",
		Example: `  kalshi-pp-cli watch remove KXTSLA-26-T125`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				if dryRunOK(flags) {
					return nil
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("requires <ticker> argument; see --help")
			}
			ticker := args[0]
			if ticker == "__printing_press_invalid__" {
				cmd.SilenceUsage = true
				return fmt.Errorf("invalid ticker placeholder")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			res, err := db.DB().Exec(`DELETE FROM watchlist WHERE ticker = ?`, ticker)
			if err != nil {
				return fmt.Errorf("deleting watchlist row: %w", err)
			}
			n, _ := res.RowsAffected()
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"removed": n > 0, "ticker": ticker}, flags)
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s was not on the watchlist\n", ticker)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", ticker)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWatchDiffCmd(flags *rootFlags) *cobra.Command {
	var since string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "diff",
		Short:       "Show price/volume change vs the earliest snapshot in the window for each watched ticker",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Default: 24-hour window
  kalshi-pp-cli watch diff

  # 7-day window as JSON
  kalshi-pp-cli watch diff --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ts, err := parseSinceDuration(since)
			if err != nil {
				return fmt.Errorf("invalid --since: %w", err)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := queryWatchDiff(db.DB(), ts.Unix())
			if err != nil {
				return fmt.Errorf("querying watch diff: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No watched tickers, or no snapshots in the window. Add tickers with 'watch add' and ensure 'sync markets' has run twice.")
				return nil
			}
			headers := []string{"Ticker", "First Yes Bid", "Latest Yes Bid", "Δ Yes Bid", "Δ Volume", "Snapshots"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				deltaYes := r.LatestYesBid - r.FirstYesBid
				deltaVol := r.LatestVolume - r.FirstVolume
				delta := fmt.Sprintf("%+.0f", deltaYes)
				if deltaYes > 0 {
					delta = green(delta)
				} else if deltaYes < 0 {
					delta = red(delta)
				}
				tableRows = append(tableRows, []string{
					r.Ticker,
					formatPriceCents(r.FirstYesBid),
					formatPriceCents(r.LatestYesBid),
					delta,
					fmt.Sprintf("%+d", deltaVol),
					fmt.Sprintf("%d", r.SnapshotCount),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}
	cmd.Flags().StringVar(&since, "since", "24h", "Lookback window (e.g., 1h, 24h, 7d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type watchDiffRow struct {
	Ticker        string  `json:"ticker"`
	FirstYesBid   float64 `json:"first_yes_bid"`
	LatestYesBid  float64 `json:"latest_yes_bid"`
	FirstVolume   int64   `json:"first_volume"`
	LatestVolume  int64   `json:"latest_volume"`
	SnapshotCount int     `json:"snapshot_count"`
}

func queryWatchDiff(db *sql.DB, sinceUnix int64) ([]watchDiffRow, error) {
	// For each watched ticker, pull the earliest and latest snapshot in the
	// window and the count of snapshots between them. SQLite doesn't have
	// FIRST_VALUE/LAST_VALUE in a portable way without window functions on
	// older versions, so use correlated subqueries.
	rows, err := db.Query(`
		SELECT
			w.ticker,
			COALESCE((SELECT yes_bid FROM market_price_history h
				WHERE h.ticker = w.ticker AND h.snapshot_ts >= ?
				ORDER BY h.snapshot_ts ASC LIMIT 1), 0) AS first_yes_bid,
			COALESCE((SELECT yes_bid FROM market_price_history h
				WHERE h.ticker = w.ticker AND h.snapshot_ts >= ?
				ORDER BY h.snapshot_ts DESC LIMIT 1), 0) AS latest_yes_bid,
			COALESCE((SELECT volume FROM market_price_history h
				WHERE h.ticker = w.ticker AND h.snapshot_ts >= ?
				ORDER BY h.snapshot_ts ASC LIMIT 1), 0) AS first_volume,
			COALESCE((SELECT volume FROM market_price_history h
				WHERE h.ticker = w.ticker AND h.snapshot_ts >= ?
				ORDER BY h.snapshot_ts DESC LIMIT 1), 0) AS latest_volume,
			(SELECT COUNT(*) FROM market_price_history h
				WHERE h.ticker = w.ticker AND h.snapshot_ts >= ?) AS snapshot_count
		FROM watchlist w
		ORDER BY w.added_at ASC
	`, sinceUnix, sinceUnix, sinceUnix, sinceUnix, sinceUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []watchDiffRow
	for rows.Next() {
		var r watchDiffRow
		if err := rows.Scan(&r.Ticker, &r.FirstYesBid, &r.LatestYesBid, &r.FirstVolume, &r.LatestVolume, &r.SnapshotCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
