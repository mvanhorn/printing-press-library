// Copyright 2026 mayank-lavania. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/nse-india/internal/store"
	"github.com/spf13/cobra"
)

// holdingRow represents one line from a holdings CSV file.
type holdingRow struct {
	Symbol  string
	Qty     float64
	AvgCost float64
}

// portfolioPnLResult holds unrealized P&L for one holding.
type portfolioPnLResult struct {
	Symbol       string  `json:"symbol"`
	Qty          float64 `json:"qty"`
	AvgCost      float64 `json:"avg_cost"`
	CurrentPrice float64 `json:"current_price"`
	MarketValue  float64 `json:"market_value"`
	PnL          float64 `json:"unrealized_pnl"`
	PnLPct       float64 `json:"unrealized_pnl_pct"`
	DayChange    float64 `json:"day_change"`
}

// portfolioMarginResult holds margin-at-risk for one holding.
type portfolioMarginResult struct {
	Symbol      string  `json:"symbol"`
	Qty         float64 `json:"qty"`
	Price       float64 `json:"price"`
	VaRMargin   float64 `json:"var_margin_pct"`
	ELM         float64 `json:"elm_pct"`
	TotalMargin float64 `json:"total_margin_value"`
}

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Portfolio P&L and margin health against synced quote history",
	}

	cmd.AddCommand(newPortfolioPnLCmd(flags))
	cmd.AddCommand(newPortfolioMarginHealthCmd(flags))

	return cmd
}

func parseHoldings(path string) ([]holdingRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening holdings file: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("holdings file is empty")
	}

	// Detect header row
	start := 0
	header := make(map[string]int)
	first := records[0]
	for i, col := range first {
		header[strings.ToLower(strings.TrimSpace(col))] = i
	}
	if _, hasSymbol := header["symbol"]; hasSymbol {
		start = 1
	}

	// Column index detection: symbol, qty, avg_cost/avg_price/cost
	symIdx := header["symbol"]
	qtyIdx, ok := header["qty"]
	if !ok {
		qtyIdx = header["quantity"]
	}
	costIdx, ok := header["avg_cost"]
	if !ok {
		costIdx, ok = header["avg_price"]
		if !ok {
			costIdx = header["cost"]
		}
	}

	var holdings []holdingRow
	for _, rec := range records[start:] {
		if len(rec) == 0 {
			continue
		}
		symbol := ""
		if symIdx < len(rec) {
			symbol = strings.TrimSpace(strings.ToUpper(rec[symIdx]))
		}
		if symbol == "" {
			continue
		}
		qty := 0.0
		if qtyIdx < len(rec) {
			qty, _ = strconv.ParseFloat(strings.TrimSpace(rec[qtyIdx]), 64)
		}
		cost := 0.0
		if costIdx < len(rec) {
			cost, _ = strconv.ParseFloat(strings.TrimSpace(rec[costIdx]), 64)
		}
		if qty > 0 {
			holdings = append(holdings, holdingRow{Symbol: symbol, Qty: qty, AvgCost: cost})
		}
	}
	if len(holdings) == 0 {
		return nil, fmt.Errorf("no valid holdings parsed from %s (expected columns: symbol, qty, avg_cost)", path)
	}
	return holdings, nil
}

func newPortfolioPnLCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var holdingsPath string

	cmd := &cobra.Command{
		Use:   "pnl",
		Short: "Track unrealized P&L for a holdings CSV against synced quote history",
		Long: `Reads a holdings CSV (columns: symbol, qty, avg_cost) and cross-references
each position against the most recent equity quote in the local store to
compute unrealized P&L, market value, and day change.

No brokerage API integration required — works entirely from local store data
populated by 'nse-india-pp-cli equity quote --symbol <SYMBOL>'.`,
		Example: `  # P&L for holdings in a CSV file
  nse-india-pp-cli portfolio pnl --holdings holdings.csv

  # Agent output
  nse-india-pp-cli portfolio pnl --holdings holdings.csv --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] portfolio pnl: would read holdings CSV and cross-reference with store quotes`)
				return nil
			}
			if holdingsPath == "" {
				return usageErr(fmt.Errorf("--holdings <file.csv> is required"))
			}

			holdings, err := parseHoldings(holdingsPath)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nse-india-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var results []portfolioPnLResult
			totalMV, totalCost, totalDayChange := 0.0, 0.0, 0.0

			for _, h := range holdings {
				row := s.DB().QueryRow(`
					SELECT json_extract(data, '$.lastPrice') as last_price,
					       json_extract(data, '$.change') as change
					FROM resources
					WHERE resource_type = 'equity'
					  AND UPPER(json_extract(data, '$.symbol')) = UPPER(?)
					ORDER BY updated_at DESC
					LIMIT 1
				`, h.Symbol)

				var lastPriceRaw, changeRaw *string
				_ = row.Scan(&lastPriceRaw, &changeRaw)

				currentPrice := 0.0
				if lastPriceRaw != nil {
					currentPrice, _ = strconv.ParseFloat(*lastPriceRaw, 64)
				}
				dayChange := 0.0
				if changeRaw != nil {
					changePerShare, _ := strconv.ParseFloat(*changeRaw, 64)
					dayChange = changePerShare * h.Qty
				}

				mv := currentPrice * h.Qty
				cost := h.AvgCost * h.Qty
				pnl := mv - cost
				pnlPct := 0.0
				if cost > 0 {
					pnlPct = pnl / cost * 100
				}

				totalMV += mv
				totalCost += cost
				totalDayChange += dayChange

				results = append(results, portfolioPnLResult{
					Symbol:       h.Symbol,
					Qty:          h.Qty,
					AvgCost:      h.AvgCost,
					CurrentPrice: currentPrice,
					MarketValue:  mv,
					PnL:          pnl,
					PnLPct:       pnlPct,
					DayChange:    dayChange,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].PnL > results[j].PnL
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				type portfolioSummary struct {
					Holdings     []portfolioPnLResult `json:"holdings"`
					TotalMV      float64              `json:"total_market_value"`
					TotalCost    float64              `json:"total_cost"`
					TotalPnL     float64              `json:"total_unrealized_pnl"`
					TotalPnLPct  float64              `json:"total_pnl_pct"`
					TodayDeltaRs float64              `json:"today_delta_rs"`
				}
				totalPnLPct := 0.0
				if totalCost > 0 {
					totalPnLPct = (totalMV - totalCost) / totalCost * 100
				}
				summary := portfolioSummary{
					Holdings:     results,
					TotalMV:      totalMV,
					TotalCost:    totalCost,
					TotalPnL:     totalMV - totalCost,
					TotalPnLPct:  totalPnLPct,
					TodayDeltaRs: totalDayChange,
				}
				data, _ := json.Marshal(summary)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Portfolio P&L\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8s %9s %9s %12s %12s %8s %10s\n",
				"SYMBOL", "QTY", "AVG_COST", "CUR_PRICE", "MKT_VALUE", "P&L", "P&L%", "DAY_CHG")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8.0f %9.2f %9.2f %12.0f %12.0f %8.2f %10.0f\n",
					r.Symbol, r.Qty, r.AvgCost, r.CurrentPrice, r.MarketValue, r.PnL, r.PnLPct, r.DayChange)
			}
			totalPnL := totalMV - totalCost
			totalPnLPct := 0.0
			if totalCost > 0 {
				totalPnLPct = totalPnL / totalCost * 100
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8s %9s %9s %12.0f %12.0f %8.2f %10.0f\n",
				"TOTAL", "", "", "", totalMV, totalPnL, totalPnLPct, totalDayChange)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().StringVar(&holdingsPath, "holdings", "", "Path to holdings CSV file (columns: symbol, qty, avg_cost)")

	return cmd
}

func newPortfolioMarginHealthCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var holdingsPath string

	cmd := &cobra.Command{
		Use:   "margin-health",
		Short: "Aggregate VaR and ELM margin across a portfolio — shows total margin-at-risk per holding",
		Long: `Reads a holdings CSV and cross-references each position against the VaR
margin and extreme loss margin (ELM) data in the local equity quote store.
Computes total margin-at-risk per holding and portfolio-wide margin exposure.

Check this before market open to verify overnight VaR changes haven't
pushed margin utilization above a safe threshold.`,
		Example: `  # Margin health for a holdings file
  nse-india-pp-cli portfolio margin-health --holdings holdings.csv

  # Agent output
  nse-india-pp-cli portfolio margin-health --holdings holdings.csv --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				fmt.Fprintln(cmd.OutOrStdout(), `[dry-run] portfolio margin-health: would compute margin-at-risk from holdings x store margin data`)
				return nil
			}
			if holdingsPath == "" {
				return usageErr(fmt.Errorf("--holdings <file.csv> is required"))
			}

			holdings, err := parseHoldings(holdingsPath)
			if err != nil {
				return err
			}

			if dbPath == "" {
				dbPath = defaultDBPath("nse-india-pp-cli")
			}
			s, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening store: %w", err)
			}
			defer s.Close()

			var results []portfolioMarginResult
			totalMargin := 0.0

			for _, h := range holdings {
				row := s.DB().QueryRow(`
					SELECT json_extract(data, '$.lastPrice') as price,
					       json_extract(data, '$.varMargin') as var_margin,
					       json_extract(data, '$.extremeLossMargin') as elm
					FROM resources
					WHERE resource_type = 'equity'
					  AND UPPER(json_extract(data, '$.symbol')) = UPPER(?)
					ORDER BY updated_at DESC
					LIMIT 1
				`, h.Symbol)

				var priceRaw, varRaw, elmRaw *string
				_ = row.Scan(&priceRaw, &varRaw, &elmRaw)

				price := 0.0
				if priceRaw != nil {
					price, _ = strconv.ParseFloat(*priceRaw, 64)
				}
				varPct := 0.0
				if varRaw != nil {
					varPct, _ = strconv.ParseFloat(*varRaw, 64)
				}
				elm := 0.0
				if elmRaw != nil {
					elm, _ = strconv.ParseFloat(*elmRaw, 64)
				}

				positionValue := price * h.Qty
				marginValue := positionValue * (varPct + elm) / 100

				totalMargin += marginValue

				results = append(results, portfolioMarginResult{
					Symbol:      h.Symbol,
					Qty:         h.Qty,
					Price:       price,
					VaRMargin:   varPct,
					ELM:         elm,
					TotalMargin: marginValue,
				})
			}

			sort.Slice(results, func(i, j int) bool {
				return results[i].TotalMargin > results[j].TotalMargin
			})

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				type marginSummary struct {
					Holdings      []portfolioMarginResult `json:"holdings"`
					TotalMarginRs float64                 `json:"total_margin_at_risk_rs"`
				}
				summary := marginSummary{Holdings: results, TotalMarginRs: totalMargin}
				data, _ := json.Marshal(summary)
				return printOutput(cmd.OutOrStdout(), data, true)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Portfolio Margin Health\n\n")
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8s %10s %7s %7s %14s\n",
				"SYMBOL", "QTY", "PRICE", "VAR%", "ELM%", "MARGIN_AT_RISK")
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			for _, r := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8.0f %10.2f %7.2f %7.2f %14.0f\n",
					r.Symbol, r.Qty, r.Price, r.VaRMargin, r.ELM, r.TotalMargin)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------")
			fmt.Fprintf(cmd.OutOrStdout(), "%-14s %8s %10s %7s %7s %14.0f\n",
				"TOTAL", "", "", "", "", totalMargin)
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/nse-india-pp-cli/data.db)")
	cmd.Flags().StringVar(&holdingsPath, "holdings", "", "Path to holdings CSV file (columns: symbol, qty, avg_cost)")

	return cmd
}
