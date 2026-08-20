// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: portfolio dividends. Computes dividend income and
// yield-on-cost from local portfolio lots + synced dividend rows.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// dividendRow is one entry in the JSON output of `portfolio dividends`.
type dividendRow struct {
	Symbol           string  `json:"symbol"`
	Shares           float64 `json:"shares"`
	CostBasis        float64 `json:"cost_basis"`
	PeriodDPS        float64 `json:"period_dividends_per_share"`
	TotalDividendInc float64 `json:"total_dividend_income"`
	YieldOnCostPct   float64 `json:"yield_on_cost_pct"`
	Note             string  `json:"note,omitempty"`
}

func newPortfolioDividendsCmd(flags *rootFlags) *cobra.Command {
	var year int
	var symbolFilter string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "dividends",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Dividend income and yield-on-cost per holding for a year",
		Long: strings.Trim(`
Sums local dividend rows for each holding in the portfolio and computes
yield-on-cost (YoC = total dividend $ for the period / cost basis of the lot).
Reads from the local resources table (resource_type IN ('dividends','history_dividends'))
and from portfolio_lots; live API calls are not made.

Run 'yahoo-finance-pp-cli sync' first if no dividend rows are present.
`, "\n"),
		Example: strings.Trim(`
  yahoo-finance-pp-cli portfolio dividends --year 2026
  yahoo-finance-pp-cli portfolio dividends --year 2025 --symbol AAPL --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if year == 0 {
				year = time.Now().Year()
			}
			db, err := openDividendsDB(flags, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			// Pull lots, optionally filtered by symbol.
			lotQuery := `SELECT symbol, SUM(shares), SUM(shares*cost_basis)
				FROM portfolio_lots`
			var lotArgs []any
			if symbolFilter != "" {
				lotQuery += ` WHERE symbol = ?`
				lotArgs = append(lotArgs, strings.ToUpper(symbolFilter))
			}
			lotQuery += ` GROUP BY symbol ORDER BY symbol`
			rows, err := db.Query(lotQuery, lotArgs...)
			if err != nil {
				return fmt.Errorf("querying portfolio_lots: %w", err)
			}
			defer rows.Close()
			type lot struct {
				symbol    string
				shares    float64
				costTotal float64
			}
			var lots []lot
			for rows.Next() {
				var l lot
				if err := rows.Scan(&l.symbol, &l.shares, &l.costTotal); err != nil {
					return err
				}
				lots = append(lots, l)
			}
			if len(lots) == 0 {
				if flags.asJSON {
					return flags.printJSON(cmd, []dividendRow{})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "(no portfolio lots — add one with `portfolio add SYMBOL SHARES COST`)")
				return nil
			}

			out := make([]dividendRow, 0, len(lots))
			for _, l := range lots {
				dps, hasData := sumDividendsForSymbol(db, l.symbol, year)
				row := dividendRow{
					Symbol:    l.symbol,
					Shares:    l.shares,
					CostBasis: l.costTotal,
					PeriodDPS: dps,
				}
				if !hasData {
					row.Note = "no dividend data synced — run `sync` first"
					out = append(out, row)
					continue
				}
				row.TotalDividendInc = dps * l.shares
				if l.costTotal > 0 {
					row.YieldOnCostPct = row.TotalDividendInc / l.costTotal
				}
				out = append(out, row)
			}

			// Order by total dividend descending; "no-data" rows sink to the bottom.
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].TotalDividendInc > out[j].TotalDividendInc
			})

			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"SYMBOL", "SHARES", "DPS", "INCOME", "YoC%", "NOTE"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Symbol,
					fmt.Sprintf("%.2f", r.Shares),
					fmt.Sprintf("%.4f", r.PeriodDPS),
					fmt.Sprintf("%.2f", r.TotalDividendInc),
					fmt.Sprintf("%.4f", r.YieldOnCostPct),
					r.Note,
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().IntVar(&year, "year", 0, "Year to sum dividends for (defaults to current year)")
	cmd.Flags().StringVar(&symbolFilter, "symbol", "", "Limit to one symbol")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// openDividendsDB opens the local SQLite database with the dividends-friendly
// schema. When dbPath is set, it overrides the default ~/.local/share path.
// Always ensures the transcendence (watchlist/portfolio) tables exist and
// adds the resources table the sync resource feed populates.
func openDividendsDB(flags *rootFlags, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return openDB(flags)
	}
	// User-overridden DB path: skip the default. Initialize schema same
	// as openDB so portfolio_lots exists and resources is queryable.
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(transcendenceSchema); err != nil {
		db.Close()
		return nil, err
	}
	// Mirror the minimum resources shape so an injected --db works for tests.
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS resources (
		id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		data TEXT,
		synced_at DATETIME,
		updated_at DATETIME,
		PRIMARY KEY (resource_type, id)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// sumDividendsForSymbol returns (sumPerShare, hasData) for the given
// year by walking rows in the resources table with resource_type IN
// ('dividends','history_dividends'). Each row's data column is JSON
// holding {date, amount} (the canonical Yahoo dividend shape).
// hasData=false means no rows of the right resource_type existed at all
// for that symbol (regardless of year) — distinguishing "synced but the
// company paid nothing this year" (return 0, true) from "no sync"
// (return 0, false).
func sumDividendsForSymbol(db *sql.DB, symbol string, year int) (float64, bool) {
	sym := strings.ToUpper(symbol)
	rows, err := db.Query(`SELECT data FROM resources
		WHERE resource_type IN ('dividends','history_dividends')
		AND (id LIKE ? || ':%' OR id = ?)`,
		sym, sym)
	if err != nil {
		return 0, false
	}
	defer rows.Close()
	hasAny := false
	total := 0.0
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		hasAny = true
		// Each row may be a single object or an array of objects. Support both.
		// Single shape: {"date":"2026-02-09","amount":0.24}
		// Array shape: [{"date":"...","amount":0.24},...]
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if raw[0] == '[' {
			var arr []struct {
				Date   string  `json:"date"`
				Amount float64 `json:"amount"`
			}
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				for _, e := range arr {
					if matchYear(e.Date, year) {
						total += e.Amount
					}
				}
			}
		} else {
			var single struct {
				Date   string  `json:"date"`
				Amount float64 `json:"amount"`
			}
			if err := json.Unmarshal([]byte(raw), &single); err == nil {
				if matchYear(single.Date, year) {
					total += single.Amount
				}
			}
		}
	}
	if err := rows.Err(); err != nil {
		// Treat as no usable data; the caller already differentiates
		// hasAny=false ("no sync") from hasAny=true ("synced, zero paid").
		return 0, hasAny
	}
	return total, hasAny
}

// matchYear returns true when an RFC3339/YYYY-MM-DD date string falls in year.
// Tolerates trailing time/timezone garbage by checking only the first 4 chars.
func matchYear(date string, year int) bool {
	if len(date) < 4 {
		return false
	}
	return date[:4] == fmt.Sprintf("%04d", year)
}
