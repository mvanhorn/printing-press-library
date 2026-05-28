// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: screen-local. Filter locally-synced fundamentals by
// arbitrary P/E, ROE, debt/equity, market cap, and dividend-yield bounds.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/store"
	"github.com/spf13/cobra"
)

type screenRow struct {
	Symbol     string  `json:"symbol"`
	TrailingPE float64 `json:"trailing_pe"`
	ROE        float64 `json:"roe"`
	DebtEquity float64 `json:"debt_equity"`
	MarketCap  float64 `json:"market_cap"`
	DivYield   float64 `json:"div_yield"`
}

func newScreenLocalCmd(flags *rootFlags) *cobra.Command {
	var peMax, peMin float64
	var roeMin, deMax, divMin float64
	var capMin int64
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "screen-local",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Run value/growth filters against locally-synced fundamentals",
		Long: strings.Trim(`
Compose arbitrary P/E, ROE, debt-to-equity, market-cap, and dividend-yield
filters against the data you've already synced. Yahoo's remote screener
supports only 12 fixed IDs; this fills the gap with a SQLite path that
runs on rows you've previously synced (resource_type='stats').
`, "\n"),
		Example: strings.Trim(`
  yahoo-finance-pp-cli screen-local --pe-max 25 --roe-min 0.15 --json
  yahoo-finance-pp-cli screen-local --div-yield-min 0.03 --debt-equity-max 1
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, closeFn, err := openScreenLocalDB(flags, dbPath)
			if err != nil {
				return err
			}
			defer closeFn()

			conds := []string{}
			args2 := []any{}
			if peMax > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.trailingPE'), 0) > 0 AND COALESCE(json_extract(data,'$.trailingPE'), 0) <= ?")
				args2 = append(args2, peMax)
			}
			if peMin > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.trailingPE'), 0) >= ?")
				args2 = append(args2, peMin)
			}
			if roeMin > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.returnOnEquity'), 0) >= ?")
				args2 = append(args2, roeMin)
			}
			if deMax > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.debtToEquity'), 0) > 0 AND COALESCE(json_extract(data,'$.debtToEquity'), 0) <= ?")
				args2 = append(args2, deMax)
			}
			if capMin > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.marketCap'), 0) >= ?")
				args2 = append(args2, capMin)
			}
			if divMin > 0 {
				conds = append(conds, "COALESCE(json_extract(data,'$.dividendYield'), 0) >= ?")
				args2 = append(args2, divMin)
			}
			query := `SELECT id,
				COALESCE(json_extract(data,'$.trailingPE'), 0),
				COALESCE(json_extract(data,'$.returnOnEquity'), 0),
				COALESCE(json_extract(data,'$.debtToEquity'), 0),
				COALESCE(json_extract(data,'$.marketCap'), 0),
				COALESCE(json_extract(data,'$.dividendYield'), 0)
				FROM resources WHERE resource_type='stats'`
			if len(conds) > 0 {
				query += " AND " + strings.Join(conds, " AND ")
			}
			query += " ORDER BY COALESCE(json_extract(data,'$.marketCap'), 0) DESC"
			if limit <= 0 {
				limit = 50
			}
			query += fmt.Sprintf(" LIMIT %d", limit)

			rows, err := db.Query(query, args2...)
			if err != nil {
				// "no such table" → hint and emit empty
				if strings.Contains(err.Error(), "no such table") {
					if cmd != nil {
						fmt.Fprintln(cmd.ErrOrStderr(), "hint: local store has no `resources` table yet — run `yahoo-finance-pp-cli sync` to populate it")
					}
					if flags.asJSON {
						return flags.printJSON(cmd, []screenRow{})
					}
					return nil
				}
				return err
			}
			defer rows.Close()
			out := []screenRow{}
			for rows.Next() {
				var r screenRow
				var pe, roe, de, mc, dy sql.NullFloat64
				if err := rows.Scan(&r.Symbol, &pe, &roe, &de, &mc, &dy); err != nil {
					return err
				}
				r.TrailingPE = pe.Float64
				r.ROE = roe.Float64
				r.DebtEquity = de.Float64
				r.MarketCap = mc.Float64
				r.DivYield = dy.Float64
				out = append(out, r)
			}
			if len(out) == 0 {
				// Differentiate "no rows" from "no sync" — try a count.
				var stats int
				_ = db.QueryRow(`SELECT COUNT(*) FROM resources WHERE resource_type='stats'`).Scan(&stats)
				if stats == 0 && cmd != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "hint: no stats rows synced — run `yahoo-finance-pp-cli sync` to populate fundamentals")
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"SYMBOL", "P/E", "ROE", "D/E", "MKT_CAP", "DIV_YIELD"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Symbol,
					fmt.Sprintf("%.2f", r.TrailingPE),
					fmt.Sprintf("%.4f", r.ROE),
					fmt.Sprintf("%.2f", r.DebtEquity),
					fmt.Sprintf("%.0f", r.MarketCap),
					fmt.Sprintf("%.4f", r.DivYield),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().Float64Var(&peMax, "pe-max", 0, "Maximum trailing P/E ratio")
	cmd.Flags().Float64Var(&peMin, "pe-min", 0, "Minimum trailing P/E ratio")
	cmd.Flags().Float64Var(&roeMin, "roe-min", 0, "Minimum return on equity (decimal, e.g. 0.15 for 15%)")
	cmd.Flags().Float64Var(&deMax, "debt-equity-max", 0, "Maximum debt-to-equity ratio")
	cmd.Flags().Int64Var(&capMin, "market-cap-min", 0, "Minimum market cap in dollars")
	cmd.Flags().Float64Var(&divMin, "div-yield-min", 0, "Minimum dividend yield (decimal)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// openScreenLocalDB returns a *sql.DB along with a close func. When dbPath
// is set, opens that file directly (test-friendly). Otherwise opens the
// canonical store path via store.OpenWithContext to ensure the schema
// matches the sync writer's expectations.
func openScreenLocalDB(flags *rootFlags, dbPath string) (*sql.DB, func(), error) {
	if dbPath != "" {
		db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON")
		if err != nil {
			return nil, func() {}, err
		}
		// Ensure resources table exists for clean error paths in tests.
		_, _ = db.Exec(`CREATE TABLE IF NOT EXISTS resources (
			id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			data TEXT,
			synced_at DATETIME,
			updated_at DATETIME,
			PRIMARY KEY (resource_type, id)
		)`)
		return db, func() { db.Close() }, nil
	}
	path := defaultDBPath("yahoo-finance-pp-cli")
	s, err := store.OpenWithContext(context.Background(), path)
	if err != nil {
		// Fallback to raw open so a fresh CLI still answers with a clear
		// hint rather than a migrate error.
		db, derr := sql.Open("sqlite", path+"?_foreign_keys=ON")
		if derr != nil {
			return nil, func() {}, fmt.Errorf("opening store and fallback: %v / %v", err, derr)
		}
		return db, func() { db.Close() }, nil
	}
	return s.DB(), func() { s.Close() }, nil
}
