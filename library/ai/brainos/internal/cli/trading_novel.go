package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/brainos/internal/store"
	"github.com/spf13/cobra"
)

func newTradingNovelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "trading",
		Short:       "Trading analytics: pulse, calibrate, drift",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newTradingPulseCmd(flags))
	cmd.AddCommand(newTradingCalibrateCmd(flags))
	cmd.AddCommand(newTradingDriftCmd(flags))
	return cmd
}

func newTradingPulseCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "pulse",
		Short: "Instant overview of today's trading session",
		Long:  "Shows wins, losses, PnL, regime, and setup distribution for the most recent session.",
		Example: strings.Trim(`
  brainos-pp-cli trading pulse --json
  brainos-pp-cli trading pulse --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `{}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			row := db.DB().QueryRowContext(cmd.Context(), `
				SELECT
					json_extract(data, '$.trade_date') as trade_date,
					json_extract(data, '$.wins') as wins,
					json_extract(data, '$.losses') as losses,
					json_extract(data, '$.breakevens') as breakevens,
					json_extract(data, '$.gross_pnl') as gross_pnl,
					json_extract(data, '$.net_pnl') as net_pnl,
					json_extract(data, '$.regime') as regime,
					json_extract(data, '$.trades_taken') as trades_taken,
					json_extract(data, '$.account_mode') as account_mode
				FROM resources WHERE resource_type = 'trading-sessions'
				ORDER BY json_extract(data, '$.trade_date') DESC LIMIT 1
			`)

			var (
				tradeDate, regime, accountMode  sql.NullString
				wins, losses, breakevens, taken sql.NullInt64
				grossPnl, netPnl                sql.NullFloat64
			)
			if err := row.Scan(&tradeDate, &wins, &losses, &breakevens, &grossPnl, &netPnl, &regime, &taken, &accountMode); err != nil {
				if err == sql.ErrNoRows {
					enc := json.NewEncoder(cmd.OutOrStdout())
					enc.SetIndent("", "  ")
					return enc.Encode(map[string]any{"message": "no sessions synced yet"})
				}
				return fmt.Errorf("querying sessions: %w", err)
			}

			result := map[string]any{
				"trade_date":   tradeDate.String,
				"wins":         wins.Int64,
				"losses":       losses.Int64,
				"breakevens":   breakevens.Int64,
				"trades_taken": taken.Int64,
				"gross_pnl":    grossPnl.Float64,
				"net_pnl":      netPnl.Float64,
				"regime":       regime.String,
				"account_mode": accountMode.String,
			}
			if wins.Int64+losses.Int64 > 0 {
				result["win_rate"] = float64(wins.Int64) / float64(wins.Int64+losses.Int64)
			}

			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(result)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newTradingCalibrateCmd(flags *rootFlags) *cobra.Command {
	var dbPath, setup, regime string
	cmd := &cobra.Command{
		Use:   "calibrate",
		Short: "Win rate by setup type and market regime",
		Example: strings.Trim(`
  brainos-pp-cli trading calibrate --json
  brainos-pp-cli trading calibrate --setup gap-go --json
  brainos-pp-cli trading calibrate --regime trending --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			query := `
				SELECT
					json_extract(data, '$.setup_type') as setup_type,
					json_extract(data, '$.regime') as regime,
					CAST(json_extract(data, '$.win_rate') AS REAL) as win_rate,
					CAST(json_extract(data, '$.expected_value') AS REAL) as expected_value,
					CAST(json_extract(data, '$.trade_count') AS INTEGER) as trade_count
				FROM resources WHERE resource_type = 'trading-calibration'`
			var whereArgs []any
			var conditions []string
			if setup != "" {
				conditions = append(conditions, "json_extract(data, '$.setup_type') = ?")
				whereArgs = append(whereArgs, setup)
			}
			if regime != "" {
				conditions = append(conditions, "json_extract(data, '$.regime') = ?")
				whereArgs = append(whereArgs, regime)
			}
			if len(conditions) > 0 {
				query += " AND " + strings.Join(conditions, " AND ")
			}
			query += " ORDER BY COALESCE(CAST(json_extract(data, '$.expected_value') AS REAL), 0) DESC"

			rows, err := db.DB().QueryContext(cmd.Context(), query, whereArgs...)
			if err != nil {
				return fmt.Errorf("querying calibration: %w", err)
			}
			defer rows.Close()

			var results []map[string]any
			for rows.Next() {
				var setupType, reg sql.NullString
				var winRate, ev sql.NullFloat64
				var count sql.NullInt64
				if err := rows.Scan(&setupType, &reg, &winRate, &ev, &count); err != nil {
					continue
				}
				results = append(results, map[string]any{
					"setup_type":     setupType.String,
					"regime":         reg.String,
					"win_rate":       winRate.Float64,
					"expected_value": ev.Float64,
					"trade_count":    count.Int64,
				})
			}
			if results == nil {
				results = []map[string]any{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&setup, "setup", "", "Filter by setup type (e.g. gap-go)")
	cmd.Flags().StringVar(&regime, "regime", "", "Filter by market regime (e.g. trending, mean-revert)")
	return cmd
}

func newTradingDriftCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect setup distribution drift before P&L shows it",
		Long:  "Rolling window of premortems joined to calibration EV. Surfaces when you're unconsciously trading into lower-EV setups.",
		Example: strings.Trim(`
  brainos-pp-cli trading drift --json
  brainos-pp-cli trading drift --weeks 2 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), `[]`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("brainos-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'brainos-pp-cli sync' first.", err)
			}
			defer db.Close()

			days := weeks * 7
			rows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT
					json_extract(data, '$.setup_type') as setup_type,
					COUNT(*) as count,
					AVG(CAST(json_extract(data, '$.conviction') AS REAL)) as avg_conviction
				FROM resources WHERE resource_type = 'trading-premortems'
				AND date(json_extract(data, '$.trade_date')) >= date('now', '-' || ? || ' days')
				GROUP BY json_extract(data, '$.setup_type')
				ORDER BY count DESC
			`, strconv.Itoa(days))
			if err != nil {
				return fmt.Errorf("querying premortems: %w", err)
			}
			defer rows.Close()

			type setupRow struct {
				SetupType     string  `json:"setup_type"`
				Count         int64   `json:"count"`
				AvgConviction float64 `json:"avg_conviction"`
			}
			var setupRows []setupRow
			for rows.Next() {
				var s sql.NullString
				var c sql.NullInt64
				var conv sql.NullFloat64
				if err := rows.Scan(&s, &c, &conv); err != nil {
					continue
				}
				setupRows = append(setupRows, setupRow{s.String, c.Int64, conv.Float64})
			}

			// Load calibration EV per setup_type
			calRows, err := db.DB().QueryContext(cmd.Context(), `
				SELECT json_extract(data, '$.setup_type'), AVG(CAST(json_extract(data, '$.expected_value') AS REAL))
				FROM resources WHERE resource_type = 'trading-calibration'
				GROUP BY json_extract(data, '$.setup_type')
			`)
			if err == nil {
				defer calRows.Close()
			}
			calEV := map[string]float64{}
			if calRows != nil {
				for calRows.Next() {
					var setup sql.NullString
					var ev sql.NullFloat64
					if err := calRows.Scan(&setup, &ev); err == nil {
						calEV[setup.String] = ev.Float64
					}
				}
			}

			var results []map[string]any
			for _, s := range setupRows {
				r := map[string]any{
					"setup_type":     s.SetupType,
					"count":          s.Count,
					"avg_conviction": s.AvgConviction,
					"expected_value": calEV[s.SetupType],
					"weeks":          weeks,
				}
				results = append(results, r)
			}
			if results == nil {
				results = []map[string]any{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().IntVar(&weeks, "weeks", 2, "Rolling window in weeks")
	return cmd
}
