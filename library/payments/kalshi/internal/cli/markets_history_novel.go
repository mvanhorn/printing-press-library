// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/store"
)

// newMarketsHistoryCmd renders a market's price history from the local
// market_price_history snapshot table populated by every 'sync markets' run.
// Read-only and store-backed; no external API calls.
func newMarketsHistoryCmd(flags *rootFlags) *cobra.Command {
	var since string
	var sparkline bool
	var dbPath string

	cmd := &cobra.Command{
		Use:         "history <ticker>",
		Short:       "Show a market's price history captured by previous syncs",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read price snapshots written by every 'sync markets' run for the given ticker
and emit a time-series. Each sync inserts one row into market_price_history
keyed (ticker, snapshot_ts); older snapshots are kept indefinitely so windows
can stretch back as far as the local store has been syncing.`,
		Example: `  # Last 7 days (default) of yes/last/volume snapshots
  kalshi-pp-cli markets history KXTSLA-26-T125

  # Last 24 hours, with an inline sparkline of yes_bid
  kalshi-pp-cli markets history KXTSLA-26-T125 --since 24h --sparkline

  # JSON for downstream tooling
  kalshi-pp-cli markets history KXTSLA-26-T125 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				if dryRunOK(flags) {
					return nil
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("requires <ticker> argument; see --help")
			}
			ticker := args[0]

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

			rows, err := queryMarketHistory(db.DB(), ticker, ts.Unix())
			if err != nil {
				return fmt.Errorf("querying market history: %w", err)
			}

			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No price history yet. Run 'sync markets' at least twice (an hour apart) to capture snapshots.")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Timestamp", "Yes Bid", "Yes Ask", "Last", "Volume"}
			tableRows := make([][]string, 0, len(rows))
			for _, r := range rows {
				tableRows = append(tableRows, []string{
					time.Unix(r.TimestampUnix, 0).UTC().Format(time.RFC3339),
					formatPriceCents(r.YesBid),
					formatPriceCents(r.YesAsk),
					formatPriceCents(r.LastPrice),
					fmt.Sprintf("%d", r.Volume),
				})
			}
			if err := flags.printTable(cmd, headers, tableRows); err != nil {
				return err
			}
			if sparkline {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintf(cmd.OutOrStdout(), "yes_bid: %s\n", renderSparkline(rows))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Lookback window (e.g., 24h, 7d, 30d)")
	cmd.Flags().BoolVar(&sparkline, "sparkline", false, "Render a Unicode block sparkline of yes_bid alongside the table")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type marketHistoryRow struct {
	TimestampUnix int64   `json:"TimestampUnix"`
	YesBid        float64 `json:"YesBid"`
	YesAsk        float64 `json:"YesAsk"`
	LastPrice     float64 `json:"LastPrice"`
	Volume        int64   `json:"Volume"`
}

func queryMarketHistory(db *sql.DB, ticker string, sinceUnix int64) ([]marketHistoryRow, error) {
	rows, err := db.Query(`
		SELECT snapshot_ts,
		       COALESCE(yes_bid, 0),
		       COALESCE(yes_ask, 0),
		       COALESCE(last_price, 0),
		       COALESCE(volume, 0)
		FROM market_price_history
		WHERE ticker = ? AND snapshot_ts >= ?
		ORDER BY snapshot_ts ASC
	`, ticker, sinceUnix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []marketHistoryRow
	for rows.Next() {
		var r marketHistoryRow
		if err := rows.Scan(&r.TimestampUnix, &r.YesBid, &r.YesAsk, &r.LastPrice, &r.Volume); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// formatPriceCents formats a Kalshi cents-priced field as $X.YY (NN%).
func formatPriceCents(cents float64) string {
	return fmt.Sprintf("$%.2f (%d%%)", cents/100.0, int(cents))
}

// renderSparkline maps yes_bid values onto eight Unicode block characters.
// Returns "-" when there's no spread (single point or all equal values).
func renderSparkline(rows []marketHistoryRow) string {
	if len(rows) == 0 {
		return "-"
	}
	blocks := []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}
	min, max := rows[0].YesBid, rows[0].YesBid
	for _, r := range rows[1:] {
		if r.YesBid < min {
			min = r.YesBid
		}
		if r.YesBid > max {
			max = r.YesBid
		}
	}
	if max == min {
		return strings.Repeat(string(blocks[0]), len(rows))
	}
	var b strings.Builder
	for _, r := range rows {
		idx := int((r.YesBid - min) / (max - min) * 7)
		if idx < 0 {
			idx = 0
		}
		if idx > 7 {
			idx = 7
		}
		b.WriteRune(blocks[idx])
	}
	return b.String()
}
