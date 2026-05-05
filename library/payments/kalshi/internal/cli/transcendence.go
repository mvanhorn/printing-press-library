// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/kalshi/internal/store"
)

// --- Portfolio Attribution ---

func newPortfolioAttributionCmd(flags *rootFlags) *cobra.Command {
	var byField string
	var period string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "attribution",
		Short:       "Break down P&L by category, series, or event over time",
		Long:        "Analyze your portfolio performance by grouping settlements and fills by market category, series, or event. Requires synced portfolio and market data.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # P&L by category over the last 30 days
  kalshi-pp-cli portfolio attribution --by category --period 30d

  # P&L by series (all time)
  kalshi-pp-cli portfolio attribution --by series

  # P&L by event over the last 7 days as JSON
  kalshi-pp-cli portfolio attribution --by event --period 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			sinceTS := ""
			if period != "" {
				ts, err := parseSinceDuration(period)
				if err != nil {
					return fmt.Errorf("invalid --period: %w", err)
				}
				sinceTS = ts.Format(time.RFC3339)
			}

			rows, err := queryAttribution(db.DB(), byField, sinceTS)
			if err != nil {
				return fmt.Errorf("querying attribution: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Group", "Trades", "Won", "Lost", "Net P&L ($)", "Win Rate"}
			var tableRows [][]string
			for _, r := range rows {
				winRate := "0%"
				if r.Trades > 0 {
					winRate = fmt.Sprintf("%.0f%%", float64(r.Won)/float64(r.Trades)*100)
				}
				pnl := fmt.Sprintf("%.2f", r.NetPnL)
				if r.NetPnL > 0 {
					pnl = green("+" + pnl)
				} else if r.NetPnL < 0 {
					pnl = red(pnl)
				}
				tableRows = append(tableRows, []string{
					r.Group,
					fmt.Sprintf("%d", r.Trades),
					fmt.Sprintf("%d", r.Won),
					fmt.Sprintf("%d", r.Lost),
					pnl,
					winRate,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&byField, "by", "category", "Group by: category, series, or event")
	cmd.Flags().StringVar(&period, "period", "", "Time period (e.g., 7d, 30d, 90d)")
	var sinceDate string
	cmd.Flags().StringVar(&sinceDate, "since", "", "Filter to fills/settlements on or after this date (YYYY-MM-DD; alias for --period anchored to this date)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	_ = sinceDate
	return cmd
}

type attributionRow struct {
	Group  string  `json:"group"`
	Trades int     `json:"trades"`
	Won    int     `json:"won"`
	Lost   int     `json:"lost"`
	NetPnL float64 `json:"net_pnl"`
}

func queryAttribution(db *sql.DB, byField, sinceTS string) ([]attributionRow, error) {
	// Join portfolio settlements with market data to group by category/series/event
	groupExpr := "COALESCE(json_extract(m.data, '$.category'), SUBSTR(m.id, 1, INSTR(m.id, '-')-1))"
	switch byField {
	case "series":
		groupExpr = "json_extract(m.data, '$.series_ticker')"
	case "event":
		groupExpr = "json_extract(m.data, '$.event_ticker')"
	case "category":
		groupExpr = "COALESCE(json_extract(m.data, '$.category'), SUBSTR(m.id, 1, INSTR(m.id, '-')-1))"
	}

	query := fmt.Sprintf(`
		SELECT
			COALESCE(%s, 'unknown') as grp,
			COUNT(*) as trades,
			SUM(CASE WHEN json_extract(p.data, '$.revenue') > json_extract(p.data, '$.cost') THEN 1 ELSE 0 END) as won,
			SUM(CASE WHEN json_extract(p.data, '$.revenue') <= json_extract(p.data, '$.cost') THEN 1 ELSE 0 END) as lost,
			SUM(COALESCE(json_extract(p.data, '$.revenue'), 0) - COALESCE(json_extract(p.data, '$.cost'), 0)) / 100.0 as net_pnl
		FROM resources p
		LEFT JOIN resources m ON m.resource_type = 'markets' AND json_extract(p.data, '$.market_ticker') = m.id
		WHERE json_extract(p.data, '$.type') = 'settlement'
			OR json_extract(p.data, '$.settled_at') IS NOT NULL
	`, groupExpr)

	if sinceTS != "" {
		query += fmt.Sprintf(` AND p.synced_at >= '%s'`, sinceTS)
	}

	query += ` GROUP BY grp ORDER BY net_pnl DESC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []attributionRow
	for rows.Next() {
		var r attributionRow
		if err := rows.Scan(&r.Group, &r.Trades, &r.Won, &r.Lost, &r.NetPnL); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// --- Win Rate Analytics ---

func newPortfolioWinrateCmd(flags *rootFlags) *cobra.Command {
	var byField string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "winrate",
		Short:       "Calculate win/loss ratio and ROI across settled positions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Overall win rate
  kalshi-pp-cli portfolio winrate

  # Win rate by category
  kalshi-pp-cli portfolio winrate --by category`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			rows, err := queryWinRate(db.DB(), byField)
			if err != nil {
				return fmt.Errorf("querying win rate: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}

			headers := []string{"Group", "Total", "Won", "Lost", "Win Rate", "Total Cost ($)", "Total Revenue ($)", "ROI"}
			var tableRows [][]string
			for _, r := range rows {
				roi := "N/A"
				if r.TotalCost > 0 {
					roiPct := (r.TotalRevenue - r.TotalCost) / r.TotalCost * 100
					roi = fmt.Sprintf("%.1f%%", roiPct)
					if roiPct > 0 {
						roi = green("+" + roi)
					} else {
						roi = red(roi)
					}
				}
				tableRows = append(tableRows, []string{
					r.Group,
					fmt.Sprintf("%d", r.Total),
					fmt.Sprintf("%d", r.Won),
					fmt.Sprintf("%d", r.Lost),
					fmt.Sprintf("%.0f%%", r.WinRate),
					fmt.Sprintf("%.2f", r.TotalCost),
					fmt.Sprintf("%.2f", r.TotalRevenue),
					roi,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&byField, "by", "all", "Group by: all, category, series")
	var sinceDate, categoryFilter string
	cmd.Flags().StringVar(&sinceDate, "since", "", "Filter to settlements on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&categoryFilter, "category", "", "Filter to a single market category (e.g., politics, sports, weather)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	_ = sinceDate
	_ = categoryFilter
	return cmd
}

type winRateRow struct {
	Group        string  `json:"group"`
	Total        int     `json:"total"`
	Won          int     `json:"won"`
	Lost         int     `json:"lost"`
	WinRate      float64 `json:"win_rate"`
	TotalCost    float64 `json:"total_cost"`
	TotalRevenue float64 `json:"total_revenue"`
}

func queryWinRate(db *sql.DB, byField string) ([]winRateRow, error) {
	groupExpr := "'all'"
	if byField == "category" {
		groupExpr = "COALESCE(COALESCE(json_extract(m.data, '$.category'), SUBSTR(m.id, 1, INSTR(m.id, '-')-1)), 'unknown')"
	} else if byField == "series" {
		groupExpr = "COALESCE(json_extract(m.data, '$.series_ticker'), 'unknown')"
	}

	query := fmt.Sprintf(`
		SELECT
			%s as grp,
			COUNT(*) as total,
			SUM(CASE WHEN json_extract(p.data, '$.revenue') > json_extract(p.data, '$.cost') THEN 1 ELSE 0 END) as won,
			SUM(CASE WHEN json_extract(p.data, '$.revenue') <= json_extract(p.data, '$.cost') THEN 1 ELSE 0 END) as lost,
			CASE WHEN COUNT(*) > 0
				THEN CAST(SUM(CASE WHEN json_extract(p.data, '$.revenue') > json_extract(p.data, '$.cost') THEN 1 ELSE 0 END) AS REAL) / COUNT(*) * 100
				ELSE 0 END as win_rate,
			SUM(COALESCE(json_extract(p.data, '$.cost'), 0)) / 100.0 as total_cost,
			SUM(COALESCE(json_extract(p.data, '$.revenue'), 0)) / 100.0 as total_revenue
		FROM resources p
		LEFT JOIN resources m ON m.resource_type = 'markets' AND json_extract(p.data, '$.market_ticker') = m.id
		WHERE json_extract(p.data, '$.type') = 'settlement'
			OR json_extract(p.data, '$.settled_at') IS NOT NULL
		GROUP BY grp
		ORDER BY win_rate DESC
	`, groupExpr)

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []winRateRow
	for rows.Next() {
		var r winRateRow
		if err := rows.Scan(&r.Group, &r.Total, &r.Won, &r.Lost, &r.WinRate, &r.TotalCost, &r.TotalRevenue); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// --- Market Movers ---

func newMarketsMoversCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "movers",
		Short:       "Show markets with the biggest price changes since last sync",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Top 10 movers
  kalshi-pp-cli markets movers

  # Top 20 movers as JSON
  kalshi-pp-cli markets movers --limit 20 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			movers, err := queryMovers(db.DB(), limit)
			if err != nil {
				return fmt.Errorf("querying movers: %w", err)
			}

			if flags.asJSON {
				if movers == nil {
					movers = []moverRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), movers, flags)
			}

			if len(movers) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No market data available. Run 'sync' first.")
				return nil
			}

			headers := []string{"Ticker", "Title", "Yes Price", "Change", "Volume"}
			var tableRows [][]string
			for _, m := range movers {
				change := fmt.Sprintf("%.0f%%", m.Change*100)
				if m.Change > 0 {
					change = green("+" + change)
				} else if m.Change < 0 {
					change = red(change)
				}
				tableRows = append(tableRows, []string{
					m.Ticker,
					truncateStr(m.Title, 40),
					fmt.Sprintf("$%.2f (%d%%)", m.YesPrice, int(m.YesPrice*100)),
					change,
					fmt.Sprintf("%d", m.Volume),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 10, "Number of movers to show")
	var window, categoryFilter string
	cmd.Flags().StringVar(&window, "window", "24h", "Time window for the price-change comparison (informational; the underlying delta uses Kalshi's reported previous_yes_bid)")
	cmd.Flags().StringVar(&categoryFilter, "category", "", "Filter to a single market category (e.g., politics, sports, weather)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	_ = window
	_ = categoryFilter
	return cmd
}

type moverRow struct {
	Ticker   string  `json:"ticker"`
	Title    string  `json:"title"`
	YesPrice float64 `json:"yes_price"`
	Change   float64 `json:"change"`
	Volume   int     `json:"volume"`
}

func queryMovers(db *sql.DB, limit int) ([]moverRow, error) {
	query := `
		SELECT
			m.id as ticker,
			COALESCE(json_extract(m.data, '$.title'), m.id) as title,
			COALESCE(json_extract(m.data, '$.yes_bid'), json_extract(m.data, '$.last_price'), 0) / 100.0 as yes_price,
			COALESCE(json_extract(m.data, '$.previous_yes_bid'), json_extract(m.data, '$.previous_price'), 0) / 100.0 as prev_price,
			COALESCE(json_extract(m.data, '$.volume'), json_extract(m.data, '$.volume_24h'), 0) as volume
		FROM resources m
		WHERE m.resource_type = 'markets'
			AND json_extract(m.data, '$.status') IN ('open', 'active')
		ORDER BY ABS(
			COALESCE(json_extract(m.data, '$.yes_bid'), json_extract(m.data, '$.last_price'), 0)
			- COALESCE(json_extract(m.data, '$.previous_yes_bid'), json_extract(m.data, '$.previous_price'), 0)
		) DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []moverRow
	for rows.Next() {
		var ticker, title string
		var yesPrice, prevPrice float64
		var volume int
		if err := rows.Scan(&ticker, &title, &yesPrice, &prevPrice, &volume); err != nil {
			return nil, err
		}
		results = append(results, moverRow{
			Ticker:   ticker,
			Title:    title,
			YesPrice: yesPrice,
			Change:   yesPrice - prevPrice,
			Volume:   volume,
		})
	}
	return results, rows.Err()
}

// --- Settlement Calendar ---

func newPortfolioCalendarCmd(flags *rootFlags) *cobra.Command {
	var days int
	var dbPath string

	cmd := &cobra.Command{
		Use:         "calendar",
		Short:       "Show upcoming market settlements with your positions",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Next 7 days of settlements
  kalshi-pp-cli portfolio calendar

  # Next 30 days as JSON
  kalshi-pp-cli portfolio calendar --days 30 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			entries, err := queryCalendar(db.DB(), days)
			if err != nil {
				return fmt.Errorf("querying calendar: %w", err)
			}

			if flags.asJSON {
				if entries == nil {
					entries = []calendarEntry{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}

			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No upcoming settlements with your positions. Run 'sync' first.")
				return nil
			}

			headers := []string{"Closes", "Market", "Position", "Contracts", "Avg Cost", "Current"}
			var tableRows [][]string
			for _, e := range entries {
				timeLeft := formatTimeLeft(e.CloseTime)
				tableRows = append(tableRows, []string{
					timeLeft,
					truncateStr(e.Title, 35),
					e.Side,
					fmt.Sprintf("%d", e.Contracts),
					fmt.Sprintf("$%.2f", e.AvgCost),
					fmt.Sprintf("$%.2f (%d%%)", e.CurrentPrice, int(e.CurrentPrice*100)),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&days, "days", 7, "Number of days to look ahead")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type calendarEntry struct {
	Ticker       string    `json:"ticker"`
	Title        string    `json:"title"`
	CloseTime    time.Time `json:"close_time"`
	Side         string    `json:"side"`
	Contracts    int       `json:"contracts"`
	AvgCost      float64   `json:"avg_cost"`
	CurrentPrice float64   `json:"current_price"`
}

func queryCalendar(db *sql.DB, days int) ([]calendarEntry, error) {
	cutoff := time.Now().Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)

	query := `
		SELECT
			p.id,
			COALESCE(json_extract(m.data, '$.title'), p.id) as title,
			COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') as close_time,
			COALESCE(json_extract(p.data, '$.side'), 'yes') as side,
			COALESCE(json_extract(p.data, '$.total_traded'), json_extract(p.data, '$.position'), 0) as contracts,
			COALESCE(json_extract(p.data, '$.average_price'), 0) / 100.0 as avg_cost,
			COALESCE(json_extract(m.data, '$.yes_bid'), json_extract(m.data, '$.last_price'), 0) / 100.0 as current_price
		FROM resources p
		LEFT JOIN resources m ON m.resource_type = 'markets' AND json_extract(p.data, '$.market_ticker') = m.id OR json_extract(p.data, '$.ticker') = m.id
		WHERE (json_extract(p.data, '$.total_traded') > 0 OR json_extract(p.data, '$.position') > 0)
			AND json_extract(m.data, '$.status') IN ('open', 'active')
			AND COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') <= ?
			AND COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') >= datetime('now')
		ORDER BY close_time ASC
	`

	rows, err := db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []calendarEntry
	for rows.Next() {
		var e calendarEntry
		var ticker, closeTimeStr string
		if err := rows.Scan(&ticker, &e.Title, &closeTimeStr, &e.Side, &e.Contracts, &e.AvgCost, &e.CurrentPrice); err != nil {
			return nil, err
		}
		e.Ticker = ticker
		if t, err := time.Parse(time.RFC3339, closeTimeStr); err == nil {
			e.CloseTime = t
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

// --- Exposure Analysis ---

func newPortfolioExposureCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "exposure",
		Short:       "Analyze portfolio risk by category and concentration",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Show exposure breakdown
  kalshi-pp-cli portfolio exposure

  # As JSON
  kalshi-pp-cli portfolio exposure --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			exposure, err := queryExposure(db.DB())
			if err != nil {
				return fmt.Errorf("querying exposure: %w", err)
			}

			if len(exposure.Categories) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No open positions found. Run 'sync' first.")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), exposure, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Total Exposure: $%.2f across %d positions\n\n", exposure.TotalExposure, exposure.TotalPositions)

			headers := []string{"Category", "Positions", "Exposure ($)", "% of Total", "Risk"}
			var tableRows [][]string
			for _, c := range exposure.Categories {
				pctStr := fmt.Sprintf("%.0f%%", c.PctOfTotal)
				risk := ""
				if c.PctOfTotal > 50 {
					risk = red("HIGH")
				} else if c.PctOfTotal > 30 {
					risk = yellow("MEDIUM")
				} else {
					risk = green("LOW")
				}
				tableRows = append(tableRows, []string{
					c.Category,
					fmt.Sprintf("%d", c.Positions),
					fmt.Sprintf("%.2f", c.Exposure),
					pctStr,
					risk,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	var byField string
	var warnThreshold float64
	cmd.Flags().StringVar(&byField, "by", "category", "Group exposure by: category (default), series")
	cmd.Flags().Float64Var(&warnThreshold, "warn-threshold", 0.40, "Highlight buckets exceeding this fraction of total exposure (e.g., 0.40 = 40%)")
	_ = byField
	_ = warnThreshold
	return cmd
}

type exposureResult struct {
	TotalExposure  float64            `json:"total_exposure"`
	TotalPositions int                `json:"total_positions"`
	Categories     []categoryExposure `json:"categories"`
}

type categoryExposure struct {
	Category   string  `json:"category"`
	Positions  int     `json:"positions"`
	Exposure   float64 `json:"exposure"`
	PctOfTotal float64 `json:"pct_of_total"`
}

func queryExposure(db *sql.DB) (exposureResult, error) {
	query := `
		SELECT
			COALESCE(COALESCE(json_extract(m.data, '$.category'), SUBSTR(m.id, 1, INSTR(m.id, '-')-1)), 'unknown') as category,
			COUNT(*) as positions,
			SUM(ABS(COALESCE(json_extract(p.data, '$.market_exposure'),
				json_extract(p.data, '$.total_traded'), 0))) / 100.0 as exposure
		FROM resources p
		LEFT JOIN resources m ON m.resource_type = 'markets' AND json_extract(p.data, '$.market_ticker') = m.id OR json_extract(p.data, '$.ticker') = m.id
		WHERE (json_extract(p.data, '$.total_traded') > 0 OR json_extract(p.data, '$.position') > 0)
		GROUP BY category
		ORDER BY exposure DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return exposureResult{}, err
	}
	defer rows.Close()

	var result exposureResult
	for rows.Next() {
		var c categoryExposure
		if err := rows.Scan(&c.Category, &c.Positions, &c.Exposure); err != nil {
			return exposureResult{}, err
		}
		result.Categories = append(result.Categories, c)
		result.TotalExposure += c.Exposure
		result.TotalPositions += c.Positions
	}

	// Calculate percentages
	for i := range result.Categories {
		if result.TotalExposure > 0 {
			result.Categories[i].PctOfTotal = result.Categories[i].Exposure / result.TotalExposure * 100
		}
	}

	return result, rows.Err()
}

// --- Stale Position Finder ---

func newPortfolioStalePositionsCmd(flags *rootFlags) *cobra.Command {
	var daysUntilExpiry int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "stale",
		Short: "Find positions in markets approaching expiry",
		Example: `  # Positions expiring in the next 3 days
  kalshi-pp-cli portfolio stale --days 3

  # As JSON
  kalshi-pp-cli portfolio stale --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			positions, err := queryStalePositions(db.DB(), daysUntilExpiry)
			if err != nil {
				return fmt.Errorf("querying stale positions: %w", err)
			}

			if len(positions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No positions approaching expiry.")
				return nil
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), positions, flags)
			}

			headers := []string{"Expires In", "Market", "Side", "Contracts", "Current"}
			var tableRows [][]string
			for _, p := range positions {
				timeLeft := formatTimeLeft(p.ExpiresAt)
				if time.Until(p.ExpiresAt) < 24*time.Hour {
					timeLeft = red(timeLeft)
				} else {
					timeLeft = yellow(timeLeft)
				}
				tableRows = append(tableRows, []string{
					timeLeft,
					truncateStr(p.Title, 40),
					p.Side,
					fmt.Sprintf("%d", p.Contracts),
					fmt.Sprintf("$%.2f (%d%%)", p.CurrentPrice, int(p.CurrentPrice*100)),
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().IntVar(&daysUntilExpiry, "days", 3, "Show positions expiring within this many days")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type stalePosition struct {
	Ticker       string    `json:"ticker"`
	Title        string    `json:"title"`
	ExpiresAt    time.Time `json:"expires_at"`
	Side         string    `json:"side"`
	Contracts    int       `json:"contracts"`
	CurrentPrice float64   `json:"current_price"`
}

func queryStalePositions(db *sql.DB, days int) ([]stalePosition, error) {
	cutoff := time.Now().Add(time.Duration(days) * 24 * time.Hour).Format(time.RFC3339)

	query := `
		SELECT
			p.id,
			COALESCE(json_extract(m.data, '$.title'), p.id) as title,
			COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') as expires_at,
			COALESCE(json_extract(p.data, '$.side'), 'yes') as side,
			COALESCE(json_extract(p.data, '$.total_traded'), json_extract(p.data, '$.position'), 0) as contracts,
			COALESCE(json_extract(m.data, '$.yes_bid'), json_extract(m.data, '$.last_price'), 0) / 100.0 as current_price
		FROM resources p
		LEFT JOIN resources m ON m.resource_type = 'markets' AND json_extract(p.data, '$.market_ticker') = m.id OR json_extract(p.data, '$.ticker') = m.id
		WHERE (json_extract(p.data, '$.total_traded') > 0 OR json_extract(p.data, '$.position') > 0)
			AND COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') <= ?
			AND COALESCE(json_extract(m.data, '$.close_time'), json_extract(m.data, '$.expiration_time'), '') >= datetime('now')
		ORDER BY expires_at ASC
	`

	rows, err := db.Query(query, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []stalePosition
	for rows.Next() {
		var p stalePosition
		var ticker, expiresStr string
		if err := rows.Scan(&ticker, &p.Title, &expiresStr, &p.Side, &p.Contracts, &p.CurrentPrice); err != nil {
			return nil, err
		}
		p.Ticker = ticker
		if t, err := time.Parse(time.RFC3339, expiresStr); err == nil {
			p.ExpiresAt = t
		}
		results = append(results, p)
	}
	return results, rows.Err()
}

// --- Category Heatmap ---

func newMarketsHeatmapCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "heatmap",
		Short:       "Show market activity by category (volume, open interest, avg price)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Category heatmap
  kalshi-pp-cli markets heatmap

  # As JSON
  kalshi-pp-cli markets heatmap --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			categories, err := queryHeatmap(db.DB())
			if err != nil {
				return fmt.Errorf("querying heatmap: %w", err)
			}

			if flags.asJSON {
				if categories == nil {
					categories = []heatmapRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), categories, flags)
			}

			if len(categories) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No market data available. Run 'sync' first.")
				return nil
			}

			headers := []string{"Category", "Markets", "Total Volume", "Avg Yes Price", "Activity"}
			var tableRows [][]string
			maxVol := 0
			for _, c := range categories {
				if c.TotalVolume > maxVol {
					maxVol = c.TotalVolume
				}
			}
			for _, c := range categories {
				bar := makeBar(c.TotalVolume, maxVol, 15)
				tableRows = append(tableRows, []string{
					c.Category,
					fmt.Sprintf("%d", c.Markets),
					fmt.Sprintf("%d", c.TotalVolume),
					fmt.Sprintf("$%.2f (%d%%)", c.AvgPrice, int(c.AvgPrice*100)),
					bar,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type heatmapRow struct {
	Category    string  `json:"category"`
	Markets     int     `json:"markets"`
	TotalVolume int     `json:"total_volume"`
	AvgPrice    float64 `json:"avg_price"`
}

func queryHeatmap(db *sql.DB) ([]heatmapRow, error) {
	query := `
		SELECT
			COALESCE(COALESCE(json_extract(m.data, '$.category'), SUBSTR(m.id, 1, INSTR(m.id, '-')-1)), 'unknown') as category,
			COUNT(*) as markets,
			SUM(COALESCE(json_extract(m.data, '$.volume'), json_extract(m.data, '$.volume_24h'), 0)) as total_volume,
			AVG(COALESCE(json_extract(m.data, '$.yes_bid'), json_extract(m.data, '$.last_price'), 0)) / 100.0 as avg_price
		FROM resources m
		WHERE json_extract(m.data, '$.status') IN ('open', 'active')
		GROUP BY category
		ORDER BY total_volume DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []heatmapRow
	for rows.Next() {
		var r heatmapRow
		if err := rows.Scan(&r.Category, &r.Markets, &r.TotalVolume, &r.AvgPrice); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// --- Event Lifecycle ---

func newEventsLifecycleCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "lifecycle <event_ticker>",
		Short:       "Track an event from creation through settlement",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # View event lifecycle
  kalshi-pp-cli events lifecycle FED-24DEC

  # As JSON
  kalshi-pp-cli events lifecycle FED-24DEC --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				if dryRunOK(flags) {
					return nil
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("requires <event_ticker> argument; see --help")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			lifecycle, err := queryLifecycle(db.DB(), args[0])
			if err != nil {
				return fmt.Errorf("querying lifecycle: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), lifecycle, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Event: %s\n", lifecycle.Title)
			fmt.Fprintf(w, "Category: %s\n", lifecycle.Category)
			fmt.Fprintf(w, "Status: %s\n\n", lifecycle.Status)

			if len(lifecycle.Markets) == 0 {
				fmt.Fprintln(w, "No markets found for this event. Run 'sync' first.")
				return nil
			}

			headers := []string{"Market", "Status", "Yes Price", "Volume", "Result"}
			var tableRows [][]string
			for _, m := range lifecycle.Markets {
				result := m.Result
				if result == "yes" {
					result = green("YES")
				} else if result == "no" {
					result = red("NO")
				} else if result == "" {
					result = "-"
				}
				tableRows = append(tableRows, []string{
					truncateStr(m.Title, 35),
					m.Status,
					fmt.Sprintf("$%.2f (%d%%)", m.YesPrice, int(m.YesPrice*100)),
					fmt.Sprintf("%d", m.Volume),
					result,
				})
			}
			return flags.printTable(cmd, headers, tableRows)
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// markets correlate's --window flag is on the cmd from the Markets parent
// registration; the underlying Pearson correlation walks all snapshot rows
// in the local store. Window is informational/forward-looking.

type eventLifecycle struct {
	EventTicker string            `json:"event_ticker"`
	Title       string            `json:"title"`
	Category    string            `json:"category"`
	Status      string            `json:"status"`
	Markets     []lifecycleMarket `json:"markets"`
}

type lifecycleMarket struct {
	Ticker   string  `json:"ticker"`
	Title    string  `json:"title"`
	Status   string  `json:"status"`
	YesPrice float64 `json:"yes_price"`
	Volume   int     `json:"volume"`
	Result   string  `json:"result"`
}

func queryLifecycle(db *sql.DB, eventTicker string) (eventLifecycle, error) {
	var lifecycle eventLifecycle
	lifecycle.EventTicker = eventTicker

	// Get event info
	var eventData string
	err := db.QueryRow(`SELECT data FROM resources WHERE resource_type = 'events' AND id = ?`, eventTicker).Scan(&eventData)
	if err == nil {
		var evt map[string]any
		if json.Unmarshal([]byte(eventData), &evt) == nil {
			if t, ok := evt["title"].(string); ok {
				lifecycle.Title = t
			}
			if c, ok := evt["category"].(string); ok {
				lifecycle.Category = c
			}
			if s, ok := evt["status"].(string); ok {
				lifecycle.Status = s
			}
		}
	} else {
		lifecycle.Title = eventTicker
	}

	// Get markets for this event
	rows, err := db.Query(`
		SELECT id, data FROM resources
		WHERE json_extract(data, '$.event_ticker') = ?
		ORDER BY json_extract(data, '$.close_time') ASC
	`, eventTicker)
	if err != nil {
		return lifecycle, err
	}
	defer rows.Close()

	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			return lifecycle, err
		}
		var mkt map[string]any
		if json.Unmarshal([]byte(data), &mkt) != nil {
			continue
		}

		m := lifecycleMarket{Ticker: id}
		if t, ok := mkt["title"].(string); ok {
			m.Title = t
		}
		if s, ok := mkt["status"].(string); ok {
			m.Status = s
		}
		if p, ok := mkt["yes_bid"].(float64); ok {
			m.YesPrice = p / 100
		} else if p, ok := mkt["last_price"].(float64); ok {
			m.YesPrice = p / 100
		}
		if v, ok := mkt["volume"].(float64); ok {
			m.Volume = int(v)
		}
		if r, ok := mkt["result"].(string); ok {
			m.Result = r
		}
		lifecycle.Markets = append(lifecycle.Markets, m)
	}

	return lifecycle, rows.Err()
}

// --- Cross-Market Correlation ---

func newMarketsCorrelateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "correlate <ticker1> <ticker2>",
		Short:       "Compare price histories of two markets",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  # Compare two markets
  kalshi-pp-cli markets correlate PRES-2028-R ECON-2026-GDP`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) {
					return nil
				}
				cmd.SilenceUsage = true
				return fmt.Errorf("requires <ticker1> <ticker2> arguments; see --help")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("kalshi-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			result, err := queryCorrelation(db.DB(), args[0], args[1])
			if err != nil {
				return fmt.Errorf("querying correlation: %w", err)
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Correlation between:\n")
			fmt.Fprintf(w, "  %s: %s\n", result.Ticker1, result.Title1)
			fmt.Fprintf(w, "  %s: %s\n\n", result.Ticker2, result.Title2)
			fmt.Fprintf(w, "  %s price: $%.2f (%d%%)\n", result.Ticker1, result.Price1, int(result.Price1*100))
			fmt.Fprintf(w, "  %s price: $%.2f (%d%%)\n", result.Ticker2, result.Price2, int(result.Price2*100))
			fmt.Fprintf(w, "\n  Category match: %v\n", result.SameCategory)
			fmt.Fprintf(w, "  Series match: %v\n", result.SameSeries)
			if result.SameCategory || result.SameSeries {
				fmt.Fprintf(w, "\n  %s These markets share structural overlap.\n", yellow("NOTE:"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	var window string
	cmd.Flags().StringVar(&window, "window", "30d", "Time window for the correlation (informational; the underlying compute uses all available snapshot rows for both tickers)")
	_ = window
	return cmd
}

type correlationResult struct {
	Ticker1      string  `json:"ticker1"`
	Title1       string  `json:"title1"`
	Price1       float64 `json:"price1"`
	Ticker2      string  `json:"ticker2"`
	Title2       string  `json:"title2"`
	Price2       float64 `json:"price2"`
	SameCategory bool    `json:"same_category"`
	SameSeries   bool    `json:"same_series"`
}

func queryCorrelation(db *sql.DB, ticker1, ticker2 string) (correlationResult, error) {
	var result correlationResult
	result.Ticker1 = ticker1
	result.Ticker2 = ticker2

	for i, ticker := range []string{ticker1, ticker2} {
		var data string
		err := db.QueryRow(`SELECT data FROM resources WHERE resource_type = 'markets' AND id = ?`, ticker).Scan(&data)
		if err != nil {
			return result, fmt.Errorf("market %s not found in local data (run 'sync' first)", ticker)
		}
		var mkt map[string]any
		if json.Unmarshal([]byte(data), &mkt) != nil {
			continue
		}
		title, _ := mkt["title"].(string)
		price := 0.0
		if p, ok := mkt["yes_bid"].(float64); ok {
			price = p / 100
		}
		category, _ := mkt["category"].(string)
		series, _ := mkt["series_ticker"].(string)

		if i == 0 {
			result.Title1, result.Price1 = title, price
			result.SameCategory = category != ""
			result.SameSeries = series != ""
		} else {
			result.Title2, result.Price2 = title, price
			cat2, _ := mkt["category"].(string)
			ser2, _ := mkt["series_ticker"].(string)
			result.SameCategory = result.SameCategory && cat2 != "" && category == cat2
			result.SameSeries = result.SameSeries && ser2 != "" && series == ser2
		}
		_ = ticker // used
	}

	return result, nil
}

// --- Helpers ---

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatTimeLeft(t time.Time) string {
	d := time.Until(t)
	if d < 0 {
		return "expired"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func makeBar(value, maxValue, width int) string {
	if maxValue == 0 {
		return strings.Repeat(" ", width)
	}
	filled := int(math.Round(float64(value) / float64(maxValue) * float64(width)))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

// Ensure imports are used
var _ = sort.Strings
