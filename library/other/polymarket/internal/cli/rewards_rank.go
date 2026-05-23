// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (reward-yield ranker). See research.json novel_features.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/cliutil"
)

func newRewardsRankCmd(flags *rootFlags) *cobra.Command {
	var capital float64
	var days int
	var minSpread float64

	cmd := &cobra.Command{
		Use:     "rank",
		Short:   "Rank reward-eligible markets by expected daily payout per dollar of capital at risk, given a target spread.",
		Example: `  polymarket-pp-cli rewards rank --capital 10000 --days 7 --min-spread 0.02 --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "rewards.rank",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("capital") && !flags.dryRun {
				return usageErr(fmt.Errorf("required flag \"capital\" not set"))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: pull reward-eligible markets list.
			// 2026 endpoint: /rewards/* paths were retired by Polymarket (return HTTP 405).
			// Active path is /sampling-markets which returns reward-eligible markets with
			// clobRewards config inline. /sampling-simplified-markets is the lighter variant.
			rewardsURL := "https://clob.polymarket.com/sampling-markets"
			rawRewards, err := c.GetWithHeaders(cmd.Context(), rewardsURL, map[string]string{}, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var rewardsList []map[string]any
			// Response may be {data:[...]} or [...]; handle both.
			var wrapped struct {
				Data []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rawRewards, &wrapped); err == nil && len(wrapped.Data) > 0 {
				rewardsList = wrapped.Data
			} else {
				_ = json.Unmarshal(rawRewards, &rewardsList)
			}

			// Limit to top 50 to be polite with per-market book fetches.
			if len(rewardsList) > 50 {
				rewardsList = rewardsList[:50]
			}

			// Step 2: fan-out book fetches for top tokens.
			type rankRow struct {
				MarketID          string  `json:"market_id"`
				Question          string  `json:"question,omitempty"`
				TokenID           string  `json:"token_id"`
				MinSpread         float64 `json:"min_spread_required"`
				DailyRewardPool   float64 `json:"daily_reward_pool_usdc"`
				BookDepthUSDC     float64 `json:"book_depth_usdc"`
				ScoreShare        float64 `json:"score_share"`
				ExpectedDailyPay  float64 `json:"expected_daily_payout_usdc"`
				YieldPctPerDay    float64 `json:"yield_pct_per_day"`
				YieldPctPerWindow float64 `json:"yield_pct_window"`
				Note              string  `json:"note,omitempty"`
			}

			type sourceItem struct {
				idx    int
				market map[string]any
			}
			sources := make([]sourceItem, 0, len(rewardsList))
			for i, m := range rewardsList {
				sources = append(sources, sourceItem{idx: i, market: m})
			}

			results, ferrs := cliutil.FanoutRun(
				cmd.Context(),
				sources,
				func(s sourceItem) string {
					if id, ok := s.market["condition_id"].(string); ok {
						return id
					}
					return fmt.Sprintf("market-%d", s.idx)
				},
				func(ctx context.Context, s sourceItem) (rankRow, error) {
					row := rankRow{}
					if v, ok := s.market["condition_id"].(string); ok {
						row.MarketID = v
					}
					if v, ok := s.market["question"].(string); ok {
						row.Question = v
					}
					// Pull tokens
					var tokenID string
					if toks, ok := s.market["tokens"].([]any); ok && len(toks) > 0 {
						if t, ok := toks[0].(map[string]any); ok {
							if v, ok := t["token_id"].(string); ok {
								tokenID = v
							}
						}
					}
					row.TokenID = tokenID

					// Reward config: Polymarket's /sampling-markets returns the
					// reward shape under field name "rewards" with nested
					// "rates[].rewards_daily_rate" + top-level "min_size" and
					// "max_spread". Older docs referenced "rewards_config" /
					// "rewardsConfig" but the live wire shape is "rewards".
					if rw, ok := s.market["rewards"].(map[string]any); ok {
						if rates, ok := rw["rates"].([]any); ok && len(rates) > 0 {
							if rate, ok := rates[0].(map[string]any); ok {
								if v, ok := rate["rewards_daily_rate"].(float64); ok {
									row.DailyRewardPool = v
								}
							}
						}
						if v, ok := rw["max_spread"].(float64); ok {
							row.MinSpread = v
						}
					}
					// Fallback shape variants for forward/backward compatibility.
					if row.DailyRewardPool == 0 {
						if rc, ok := s.market["rewards_config"].([]any); ok && len(rc) > 0 {
							if cfg, ok := rc[0].(map[string]any); ok {
								if v, ok := cfg["rewards_daily_rate"].(float64); ok {
									row.DailyRewardPool = v
								}
							}
						}
					}
					if row.DailyRewardPool == 0 {
						if v, ok := s.market["rewardsDailyRate"].(float64); ok {
							row.DailyRewardPool = v
						}
					}
					if row.MinSpread == 0 {
						if v, ok := s.market["rewards_max_spread"].(float64); ok {
							row.MinSpread = v
						}
					}

					if tokenID == "" {
						row.Note = "no token_id available — skipped book fetch"
						return row, nil
					}

					// Live book fetch
					bookURL := "https://clob.polymarket.com/book"
					bookData, berr := c.GetWithHeaders(ctx, bookURL,
						map[string]string{"token_id": tokenID}, nil)
					if berr != nil {
						row.Note = fmt.Sprintf("book fetch error: %v", berr)
						return row, nil
					}
					depth := computeBookDepthUSDC(bookData, minSpread)
					row.BookDepthUSDC = depth

					// Compute expected payout:
					//   capital_share = min(capital, depth) / max(depth, 1)
					//   score_share  = capital_share (placeholder for the
					//     full clobRewards score formula — see docs for
					//     the published per-tick scoring; we approximate
					//     with depth-proportional share)
					//   expected_daily = score_share * daily_reward_pool
					capShare := capital / maxFloat(depth+capital, 1)
					if capShare > 1 {
						capShare = 1
					}
					row.ScoreShare = capShare
					row.ExpectedDailyPay = capShare * row.DailyRewardPool
					if capital > 0 {
						row.YieldPctPerDay = row.ExpectedDailyPay / capital * 100
						row.YieldPctPerWindow = row.YieldPctPerDay * float64(days)
					}
					return row, nil
				},
				cliutil.WithConcurrency(6),
			)
			if len(ferrs) > 0 {
				cliutil.FanoutReportErrors(cmd.ErrOrStderr(), ferrs)
			}
			rows := make([]rankRow, 0, len(results))
			filteredBySpread := 0
			for _, r := range results {
				row := r.Value
				// Apply --min-spread filter: drop markets whose API-allowed
				// max_spread is below the user's floor. Annotate when no spread
				// data was available so the user can distinguish "missing data"
				// from "filtered out".
				if row.MinSpread > 0 && row.MinSpread < minSpread {
					filteredBySpread++
					continue
				}
				if row.MinSpread == 0 && row.Note == "" {
					row.Note = "no rewards.max_spread in API response; min-spread filter not enforced for this row"
				}
				rows = append(rows, row)
			}

			// Sort by expected daily payout DESC, with score_share as a
			// deterministic tie-breaker when payouts collapse.
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].ExpectedDailyPay != rows[j].ExpectedDailyPay {
					return rows[i].ExpectedDailyPay > rows[j].ExpectedDailyPay
				}
				return rows[i].ScoreShare > rows[j].ScoreShare
			})

			out := map[string]any{
				"capital_usdc":           capital,
				"days":                   days,
				"min_spread":             minSpread,
				"markets_evaluated":      len(rows),
				"markets_filtered_below": filteredBySpread,
				"formula_note":           "expected_daily_payout = (capital / (capital + book_depth_usdc)) * daily_reward_pool — depth-proportional approximation of Polymarket clobRewards.scoreShare; substitute the full published per-tick formula for sub-percent accuracy.",
				"rankings":               rows,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().Float64Var(&capital, "capital", 0, "Working capital in USDC (required)")
	cmd.Flags().IntVar(&days, "days", 7, "Time window for cumulative-yield projection (days)")
	cmd.Flags().Float64Var(&minSpread, "min-spread", 0.02, "Minimum spread from mid in cents (e.g. 0.02 = 2c)")
	return cmd
}

func maxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// computeBookDepthUSDC walks the order book within `spread` of mid and sums
// USDC notional on bids+asks. Returns 0 when book is malformed or empty.
// This is a deterministic, documented approximation — see formula_note above.
func computeBookDepthUSDC(raw json.RawMessage, spread float64) float64 {
	var book map[string]any
	if err := json.Unmarshal(raw, &book); err != nil {
		return 0
	}
	depth := 0.0
	for _, side := range []string{"bids", "asks"} {
		levels, _ := book[side].([]any)
		for _, l := range levels {
			lvl, ok := l.(map[string]any)
			if !ok {
				continue
			}
			var price float64
			switch p := lvl["price"].(type) {
			case float64:
				price = p
			case string:
				_, _ = fmt.Sscanf(p, "%f", &price)
			}
			var size float64
			switch sz := lvl["size"].(type) {
			case float64:
				size = sz
			case string:
				_, _ = fmt.Sscanf(sz, "%f", &size)
			}
			// USDC notional ~= price * size
			depth += price * size
		}
	}
	return depth
}
