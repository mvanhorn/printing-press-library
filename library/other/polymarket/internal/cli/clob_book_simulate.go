// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: clob book-simulate (sibling of clob book).

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newClobBookSimulateCmd(flags *rootFlags) *cobra.Command {
	var usd float64

	cmd := &cobra.Command{
		Use:         "book-simulate <token-id>",
		Short:       "Walk the order book for a token against a $USD taker order. Outputs slippage vs midpoint and market-impact bps.",
		Example:     `  polymarket-pp-cli clob book-simulate 0xTOKEN_ID --usd 5000`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:novel": "clob.book_simulate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && !flags.dryRun {
				return usageErr(fmt.Errorf("token-id required"))
			}
			if dryRunOK(flags) {
				return nil
			}
			tokenID := args[0]
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			bookRaw, err := c.GetWithHeaders(cmd.Context(), "https://clob.polymarket.com/book",
				map[string]string{"token_id": tokenID}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var book map[string]any
			if err := json.Unmarshal(bookRaw, &book); err != nil {
				return apiErr(fmt.Errorf("parsing book: %w", err))
			}

			bids := parseLevels(book["bids"])
			asks := parseLevels(book["asks"])
			// Sort: bids desc, asks asc (best price first).
			sort.SliceStable(bids, func(i, j int) bool { return bids[i].price > bids[j].price })
			sort.SliceStable(asks, func(i, j int) bool { return asks[i].price < asks[j].price })

			midpoint := 0.0
			if len(bids) > 0 && len(asks) > 0 {
				midpoint = (bids[0].price + asks[0].price) / 2
			}

			// Simulate buying with $USD against the ask side.
			buyFill := simulateFill(asks, usd)
			// Simulate selling proportional outcome tokens worth $USD against bid side.
			sellFill := simulateFill(bids, usd)

			out := map[string]any{
				"token_id":        tokenID,
				"usd_taker":       usd,
				"midpoint":        midpoint,
				"best_bid":        zeroIfEmpty(bids),
				"best_ask":        zeroIfEmpty(asks),
				"buy_simulation":  fillToMap(buyFill, midpoint, "buy"),
				"sell_simulation": fillToMap(sellFill, midpoint, "sell"),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&usd, "usd", 5000, "USD notional to simulate (default: 5000)")
	return cmd
}

type bookLevel struct {
	price float64
	size  float64
}

func parseLevels(raw any) []bookLevel {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]bookLevel, 0, len(arr))
	for _, l := range arr {
		lvl, ok := l.(map[string]any)
		if !ok {
			continue
		}
		var p, s float64
		switch v := lvl["price"].(type) {
		case float64:
			p = v
		case string:
			_, _ = fmt.Sscanf(v, "%f", &p)
		}
		switch v := lvl["size"].(type) {
		case float64:
			s = v
		case string:
			_, _ = fmt.Sscanf(v, "%f", &s)
		}
		if p > 0 && s > 0 {
			out = append(out, bookLevel{price: p, size: s})
		}
	}
	return out
}

type fillResult struct {
	filledNotional float64
	filledSize     float64
	avgPrice       float64
	levelsHit      int
	unfilledUSD    float64
}

// simulateFill walks `levels` in order, consuming USD until exhausted.
// Each level contributes min(level.notional, remainingUSD) to the fill.
func simulateFill(levels []bookLevel, usd float64) fillResult {
	remaining := usd
	totalSize := 0.0
	totalNotional := 0.0
	hit := 0
	for _, lvl := range levels {
		notional := lvl.price * lvl.size
		take := notional
		if take > remaining {
			take = remaining
		}
		size := take / lvl.price
		totalNotional += take
		totalSize += size
		remaining -= take
		hit++
		if remaining <= 0.0001 {
			break
		}
	}
	avg := 0.0
	if totalSize > 0 {
		avg = totalNotional / totalSize
	}
	return fillResult{
		filledNotional: totalNotional,
		filledSize:     totalSize,
		avgPrice:       avg,
		levelsHit:      hit,
		unfilledUSD:    remaining,
	}
}

func fillToMap(f fillResult, mid float64, _ string) map[string]any {
	slipBps := 0.0
	if mid > 0 && f.avgPrice > 0 {
		slipBps = (f.avgPrice - mid) / mid * 10000
	}
	return map[string]any{
		"filled_notional_usd": f.filledNotional,
		"filled_size_tokens":  f.filledSize,
		"avg_fill_price":      f.avgPrice,
		"levels_hit":          f.levelsHit,
		"unfilled_usd":        f.unfilledUSD,
		"slippage_bps_vs_mid": slipBps,
	}
}

func zeroIfEmpty(levels []bookLevel) float64 {
	if len(levels) == 0 {
		return 0
	}
	return levels[0].price
}
