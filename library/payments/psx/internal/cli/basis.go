// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// newNovelBasisCmd computes the futures-to-spot spread across PSX's separate
// market boards. Every surveyed tool reads at most one board, so the spread
// itself exists nowhere upstream.
func newNovelBasisCmd(flags *rootFlags) *cobra.Command {
	var market, board string
	var top int
	cmd := &cobra.Command{
		Use:   "basis",
		Short: "Compare futures-board prices against the regular spot board to see premium or discount per symbol.",
		Long: "Use this command to compare futures-board prices against the regular spot board.\n" +
			"Do NOT use it to inspect bid/offer depth on a single market and board; use 'board' instead.\n" +
			"Markets: DFC deliverable futures, CSF cash settled futures, compared against REG spot.",
		Example:     "  psx-pp-cli basis --market DFC --top 20 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "basis")
			}
			mk := strings.ToUpper(strings.TrimSpace(market))
			switch mk {
			case "DFC", "CSF", "ODL", "SQR":
			case "REG":
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--market REG is the spot leg itself; choose DFC or CSF"))
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--market must be one of DFC, CSF, ODL, SQR (got %q)", market))
			}
			bd := strings.ToLower(strings.TrimSpace(board))
			if bd == "" {
				bd = "main"
			}
			// Whitelist rather than concatenating a caller string into the URL
			// path; newTableCmd escapes its positionals and this path must not
			// be the one exception.
			switch bd {
			case "main", "gem", "bnb":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--board must be one of main, gem, bnb (got %q)", board))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c := psxClient(flags)

			spot, err := boardPrices(ctx, c, "REG", bd)
			if err != nil {
				return fmt.Errorf("fetching spot board: %w", err)
			}
			fut, err := boardPrices(ctx, c, mk, bd)
			if err != nil {
				return fmt.Errorf("fetching %s board: %w", mk, err)
			}
			type spread struct {
				Symbol     string  `json:"symbol"`
				Spot       float64 `json:"spot"`
				Future     float64 `json:"future"`
				Basis      float64 `json:"basis"`
				PremiumPct float64 `json:"premium_pct"`
			}
			view := struct {
				Market   string   `json:"market"`
				Board    string   `json:"board"`
				SpotRows int      `json:"spot_rows"`
				FutRows  int      `json:"future_rows"`
				Count    int      `json:"count"`
				Spreads  []spread `json:"spreads"`
				Note     string   `json:"note,omitempty"`
			}{Market: mk, Board: bd, SpotRows: len(spot), FutRows: len(fut), Spreads: make([]spread, 0)}

			for sym, f := range fut {
				sp, ok := spot[sym]
				if !ok || sp == 0 {
					continue
				}
				view.Spreads = append(view.Spreads, spread{
					Symbol: sym, Spot: sp, Future: f,
					Basis: f - sp, PremiumPct: (f - sp) / sp * 100,
				})
			}
			sort.Slice(view.Spreads, func(i, j int) bool {
				return absf(view.Spreads[i].PremiumPct) > absf(view.Spreads[j].PremiumPct)
			})
			if top > 0 && len(view.Spreads) > top {
				view.Spreads = view.Spreads[:top]
			}
			view.Count = len(view.Spreads)
			if view.Count == 0 {
				view.Note = fmt.Sprintf("no symbol appears on both REG and %s for board %q right now (futures boards are often empty outside contract season)", mk, bd)
			}
			// Annualisation is deliberately not emitted: the board tables expose
			// no contract expiry column, so any annualised figure would be invented.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10s %10s %10s %10s\n", "SYMBOL", "SPOT", mk, "BASIS", "PREM%")
			for _, s := range view.Spreads {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %10.2f %10.2f %10.2f %9.2f%%\n", s.Symbol, s.Spot, s.Future, s.Basis, s.PremiumPct)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&market, "market", "DFC", "futures market to compare against spot: DFC, CSF, ODL or SQR")
	cmd.Flags().StringVar(&board, "board", "main", "board segment: main, gem or bnb")
	cmd.Flags().IntVar(&top, "top", 20, "maximum rows to return (0 = all)")
	return cmd
}

// boardPrices returns symbol -> last/current price for one market+board.
func boardPrices(ctx context.Context, c *psx.Client, market, board string) (map[string]float64, error) {
	tables, err := c.GetTables(ctx, "/trading-board/"+url.PathEscape(market)+"/"+url.PathEscape(board))
	if err != nil {
		return nil, err
	}
	out := map[string]float64{}
	for _, t := range tables {
		for _, row := range t.Rows {
			sym := strings.ToUpper(strings.TrimSpace(row["symbol"]))
			if sym == "" {
				continue
			}
			for _, col := range []string{"current", "ltp", "last", "price", "close"} {
				if v, ok := parseNum(row[col]); ok && v > 0 {
					out[sym] = v
					break
				}
			}
		}
	}
	return out, nil
}
