// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: investment holdings gain/loss across brokerages — market value
// vs cost basis, per position and in aggregate. Almost no SimpleFIN tool
// implements holdings; this is the ecosystem-wide gap.
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/simplefin/internal/simplefin"
)

type portfolioPosition struct {
	Symbol      string  `json:"symbol"`
	Description string  `json:"description"`
	Account     string  `json:"account"`
	Shares      float64 `json:"shares"`
	MarketValue float64 `json:"market_value"`
	CostBasis   float64 `json:"cost_basis"`
	// CostKnown is false when the institution reported no cost basis (common
	// for ESPP/RSU stock-plan positions). When false, Gain/GainPct are not
	// meaningful and the position is excluded from the aggregate gain.
	CostKnown bool    `json:"cost_basis_known"`
	Gain      float64 `json:"gain,omitempty"`
	GainPct   float64 `json:"gain_pct,omitempty"`
}

func newNovelPortfolioCmd(flags *rootFlags) *cobra.Command {
	var flagGain bool
	var dbPath, account string

	cmd := &cobra.Command{
		Use:   "portfolio",
		Short: "Investment positions with market value vs cost basis and total gain/loss across brokerages.",
		Long: "List investment holdings across every brokerage account with market value, cost basis, and\n" +
			"gain/loss per position and in aggregate. --gain sorts by absolute gain so winners and losers\n" +
			"surface first.\n\n" +
			"Use this command for investment positions and gain/loss. For a raw position dump use\n" +
			"'holdings'. For cash + investment combined net worth use 'networth'.",
		Example:     "  simplefin-pp-cli portfolio --gain --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			if dryRunOK(flags) {
				return nil
			}
			db, ok, err := resolveSimplefinDB(ctx, cmd, flags, dbPath)
			if err != nil || !ok {
				return err
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "holdings")

			holdings, err := loadHoldings(ctx, db)
			if err != nil {
				return err
			}
			positions := make([]portfolioPosition, 0, len(holdings))
			// Aggregates: market value sums over ALL positions; gain/cost only
			// over positions with a reported cost basis, so a stock-plan
			// position with no cost basis doesn't masquerade as 100% gain.
			var totMV, totCBKnown, totMVKnown, totMVUnknown float64
			var unknownCount int
			for _, h := range holdings {
				if account != "" && !containsAny(lowerName(h.AccountName), lowerName(account)) && h.AccountID != account {
					continue
				}
				mv, _ := simplefin.ParseAmount(h.MarketValue)
				cb, _ := simplefin.ParseAmount(h.CostBasis)
				sh, _ := simplefin.ParseAmount(h.Shares)
				sym := h.Symbol
				if sym == "" {
					sym = "—"
				}
				p := portfolioPosition{
					Symbol: sym, Description: h.Description, Account: h.AccountName,
					Shares: sh, MarketValue: mv, CostBasis: cb, CostKnown: cb > 0,
				}
				if p.CostKnown {
					p.Gain = mv - cb
					p.GainPct = (mv - cb) / cb * 100
					totCBKnown += cb
					totMVKnown += mv
				} else {
					unknownCount++
					totMVUnknown += mv
				}
				positions = append(positions, p)
				totMV += mv
			}
			// Sort: positions with a known gain first (by abs gain under --gain,
			// else market value); unknown-cost positions after, by market value.
			sort.Slice(positions, func(i, j int) bool {
				a, b := positions[i], positions[j]
				if a.CostKnown != b.CostKnown {
					return a.CostKnown
				}
				if flagGain && a.CostKnown {
					return abs(a.Gain) > abs(b.Gain)
				}
				return a.MarketValue > b.MarketValue
			})
			totGain := totMVKnown - totCBKnown
			var totPct float64
			if totCBKnown != 0 {
				totPct = totGain / totCBKnown * 100
			}
			view := map[string]any{
				"positions":                       positions,
				"total_market_value":              totMV,
				"total_cost_basis":                totCBKnown,
				"total_gain":                      totGain,
				"total_gain_pct":                  totPct,
				"positions_without_cost_basis":    unknownCount,
				"market_value_without_cost_basis": totMVUnknown,
			}
			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return flags.printJSON(cmd, view)
			}
			if len(positions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no holdings in the local store — run: simplefin-pp-cli sync")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-32s %15s %15s %15s %8s\n", "symbol", "account", "value", "cost", "gain", "gain%")
			for _, p := range positions {
				if p.CostKnown {
					fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-32s %15s %15s %15s %7.1f%%\n",
						p.Symbol, truncate(p.Account, 32), humanMoney(p.MarketValue), humanMoney(p.CostBasis), humanMoney(p.Gain), p.GainPct)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%-8s %-32s %15s %15s %15s %8s\n",
						p.Symbol, truncate(p.Account, 32), humanMoney(p.MarketValue), "—", "—", "—")
				}
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n", "--------------------------------------------------------------------------------------------")
			fmt.Fprintf(cmd.OutOrStdout(), "Total value %s  cost %s  gain %s (%.1f%%)\n", humanMoney(totMV), humanMoney(totCBKnown), humanMoney(totGain), totPct)
			if unknownCount > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "(%d position(s) worth %s have no reported cost basis — excluded from gain)\n", unknownCount, humanMoney(totMVUnknown))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&flagGain, "gain", false, "Sort by absolute gain/loss (winners and losers first)")
	cmd.Flags().StringVar(&account, "account", "", "Restrict to a single account (id or name substring)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	return cmd
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
