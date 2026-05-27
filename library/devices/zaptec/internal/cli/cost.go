// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/devices/zaptec/internal/store"

	"github.com/spf13/cobra"
)

// costRow is one aggregated period or charger.
type costRow struct {
	Group    string  `json:"group"`
	Sessions int     `json:"sessions"`
	KWh      float64 `json:"kwh"`
	Cost     float64 `json:"cost,omitempty"`
}

// newCostCmd rolls up locally synced charge history (the chargehistory table)
// into kWh totals per month or per charger. The Zaptec API does not return a
// monetary cost, so cost is computed only when --price (currency per kWh) is
// supplied; otherwise the rollup is energy + session counts. Run `sync` first.
func newCostCmd(flags *rootFlags) *cobra.Command {
	var by string
	var from, to string
	var price float64
	var dbPath string

	cmd := &cobra.Command{
		Use:         "cost",
		Short:       "Energy (and optional cost) rollup from your charging history",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregate locally synced charge sessions into total kWh and session counts,
grouped by month (default) or by charger. The portal only shows sessions one at
a time; this is the rolled-up view.

The Zaptec API does not include a price, so monetary cost is shown only when you
pass --price (your tariff in currency per kWh). Sync first with 'zaptec-pp-cli sync'.`,
		Example: strings.Trim(`
  zaptec-pp-cli cost --by month
  zaptec-pp-cli cost --by charger --price 0.30 --json
  zaptec-pp-cli cost --from 2026-01-01 --to 2026-04-01
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch by {
			case "month", "charger":
			default:
				return usageErr(fmt.Errorf("--by must be 'month' or 'charger', got %q", by))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zaptec-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'zaptec-pp-cli sync' first.", err)
			}
			defer db.Close()

			var query string
			if by == "month" {
				query = `SELECT strftime('%Y-%m', start_date_time) AS grp,
				                COUNT(*) AS sessions, COALESCE(SUM(energy),0) AS kwh
				         FROM chargehistory WHERE start_date_time IS NOT NULL`
			} else {
				query = `SELECT COALESCE(NULLIF(device_name,''), charger_id) AS grp,
				                COUNT(*) AS sessions, COALESCE(SUM(energy),0) AS kwh
				         FROM chargehistory WHERE 1=1`
			}
			var qargs []any
			if from != "" {
				query += " AND start_date_time >= ?"
				qargs = append(qargs, from)
			}
			if to != "" {
				query += " AND start_date_time < ?"
				qargs = append(qargs, to)
			}
			if by == "month" {
				query += " GROUP BY grp ORDER BY grp"
			} else {
				query += " GROUP BY grp ORDER BY kwh DESC"
			}

			rows, err := db.DB().QueryContext(cmd.Context(), query, qargs...)
			if err != nil {
				return fmt.Errorf("querying charge history: %w", err)
			}
			defer rows.Close()

			results := []costRow{}
			var totalKWh float64
			var totalSessions int
			for rows.Next() {
				var grp sql.NullString
				var sessions int
				var kwh float64
				if err := rows.Scan(&grp, &sessions, &kwh); err != nil {
					return fmt.Errorf("scanning row: %w", err)
				}
				cr := costRow{Group: grp.String, Sessions: sessions, KWh: round2(kwh)}
				if price > 0 {
					cr.Cost = round2(kwh * price)
				}
				results = append(results, cr)
				totalKWh += kwh
				totalSessions += sessions
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				out := map[string]any{
					"by":             by,
					"rows":           results,
					"total_kwh":      round2(totalKWh),
					"total_sessions": totalSessions,
				}
				if price > 0 {
					out["total_cost"] = round2(totalKWh * price)
					out["price_per_kwh"] = price
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}

			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No charge sessions found. Run 'zaptec-pp-cli sync' to pull history.")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			header := strings.ToUpper(by) + "\tSESSIONS\tKWH"
			if price > 0 {
				header += "\tCOST"
			}
			fmt.Fprintln(tw, header)
			for _, r := range results {
				line := fmt.Sprintf("%s\t%d\t%.2f", r.Group, r.Sessions, r.KWh)
				if price > 0 {
					line += fmt.Sprintf("\t%.2f", r.Cost)
				}
				fmt.Fprintln(tw, line)
			}
			total := fmt.Sprintf("TOTAL\t%d\t%.2f", totalSessions, round2(totalKWh))
			if price > 0 {
				total += fmt.Sprintf("\t%.2f", round2(totalKWh*price))
			}
			fmt.Fprintln(tw, total)
			return tw.Flush()
		},
	}

	cmd.Flags().StringVar(&by, "by", "month", "Group by: month or charger")
	cmd.Flags().StringVar(&from, "from", "", "Only sessions on/after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&to, "to", "", "Only sessions before this date (YYYY-MM-DD)")
	cmd.Flags().Float64Var(&price, "price", 0, "Price per kWh to compute cost (your tariff)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}
