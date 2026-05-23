// Copyright 2026 ahmad-thariq-syauqi. Licensed under Apache-2.0. See LICENSE.
// Hand-written: novel feature (implied-probability diff). See research.json novel_features.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"polymarket-pp-cli/internal/store"
)

func newDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Time-window deltas — find tokens whose implied probability moved by more than a threshold over a window.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newDiffPricesCmd(flags))
	return cmd
}

func newDiffPricesCmd(flags *rootFlags) *cobra.Command {
	var since time.Duration
	var minMove float64
	var watch string
	var window time.Duration

	cmd := &cobra.Command{
		Use:     "prices",
		Short:   "Find tokens whose implied probability moved by more than a threshold over a time window. Supports a slug watchlist and arbitrary windows down to a few minutes.",
		Example: `  polymarket-pp-cli diff prices --since yesterday --min-move 0.05 --watch 2026-election,fed-may --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:novel":      "diff.prices",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			dbPath := defaultDBPath("polymarket-pp-cli")
			s, err := store.Open(dbPath)
			if err != nil {
				// Local store missing — return clean envelope + hint.
				fmt.Fprintln(cmd.ErrOrStderr(),
					"hint: local store not initialized. Run 'polymarket-pp-cli sync --resources markets' (at least twice, separated by --since/--window) to populate snapshot history.")
				return printJSONFiltered(cmd.OutOrStdout(),
					map[string]any{"diffs": []any{}, "note": "local store not initialized"}, flags)
			}
			defer s.Close()

			// Lazy migration: create price_snapshots table if missing.
			// This is an additive, idempotent DDL — safe to run every invocation.
			if _, err := s.DB().Exec(`CREATE TABLE IF NOT EXISTS price_snapshots (
				token_id   TEXT NOT NULL,
				market_id  TEXT,
				question   TEXT,
				price      REAL NOT NULL,
				captured_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				PRIMARY KEY (token_id, captured_at)
			)`); err != nil {
				return apiErr(fmt.Errorf("creating price_snapshots table: %w", err))
			}
			if _, err := s.DB().Exec(`CREATE INDEX IF NOT EXISTS idx_price_snapshots_captured ON price_snapshots(captured_at)`); err != nil {
				return apiErr(fmt.Errorf("creating index: %w", err))
			}

			// Capture a fresh snapshot of current outcome prices for every
			// market in the local store. This is idempotent — duplicate
			// (token_id, captured_at) rows are blocked by the composite PK.
			// Run captures on EVERY invocation so users build history just
			// by re-running diff prices in cron / agents.
			captured := captureCurrentPriceSnapshot(s)

			// Determine window: --window overrides --since when set; both
			// are absolute durations from now.
			effectiveWindow := since
			if window > 0 && window < since {
				effectiveWindow = window
			}
			startTime := time.Now().Add(-since)
			endTime := time.Now()
			windowStart := time.Now().Add(-effectiveWindow)

			// Watchlist (CSV of slug substrings)
			var watchSlugs []string
			if watch != "" {
				for _, w := range strings.Split(watch, ",") {
					if w = strings.TrimSpace(w); w != "" {
						watchSlugs = append(watchSlugs, w)
					}
				}
			}

			// SQL: for each token in price_snapshots, get first vs last
			// price within (startTime, endTime] and compute delta.
			// Use a CTE because SQLite rejects aggregate-in-subquery-WHERE
			// when the aggregate refers to the outer GROUP BY rowset.
			rows, err := s.DB().Query(`
				WITH bounds AS (
					SELECT token_id, market_id, question,
						MIN(captured_at) AS first_at,
						MAX(captured_at) AS last_at
					FROM price_snapshots
					WHERE captured_at >= ?
					GROUP BY token_id, market_id, question
				)
				SELECT b.token_id, b.market_id, b.question,
					b.first_at, b.last_at,
					(SELECT price FROM price_snapshots p2
						WHERE p2.token_id = b.token_id
						  AND p2.captured_at = b.first_at
						LIMIT 1) AS first_price,
					(SELECT price FROM price_snapshots p3
						WHERE p3.token_id = b.token_id
						  AND p3.captured_at = b.last_at
						LIMIT 1) AS last_price
				FROM bounds b
			`, startTime.Format("2006-01-02 15:04:05"))
			if err != nil {
				return apiErr(fmt.Errorf("query price_snapshots: %w", err))
			}
			defer rows.Close()

			type diffRow struct {
				TokenID   string  `json:"token_id"`
				MarketID  string  `json:"market_id,omitempty"`
				Question  string  `json:"question,omitempty"`
				FirstAt   string  `json:"from_time"`
				LastAt    string  `json:"to_time"`
				FromPrice float64 `json:"from"`
				ToPrice   float64 `json:"to"`
				Move      float64 `json:"move"`
				MovePct   float64 `json:"move_pct"`
			}
			var diffs []diffRow
			for rows.Next() {
				var tokenID, marketID, question, firstAt, lastAt string
				var firstPrice, lastPrice float64
				if err := rows.Scan(&tokenID, &marketID, &question,
					&firstAt, &lastAt, &firstPrice, &lastPrice); err != nil {
					continue
				}
				// Apply watchlist filter
				if len(watchSlugs) > 0 {
					match := false
					for _, w := range watchSlugs {
						if strings.Contains(strings.ToLower(marketID), strings.ToLower(w)) ||
							strings.Contains(strings.ToLower(question), strings.ToLower(w)) {
							match = true
							break
						}
					}
					if !match {
						continue
					}
				}
				move := lastPrice - firstPrice
				if math.Abs(move) < minMove {
					continue
				}
				movePct := 0.0
				if firstPrice > 0 {
					movePct = move / firstPrice * 100
				}
				diffs = append(diffs, diffRow{
					TokenID:   tokenID,
					MarketID:  marketID,
					Question:  question,
					FirstAt:   firstAt,
					LastAt:    lastAt,
					FromPrice: firstPrice,
					ToPrice:   lastPrice,
					Move:      move,
					MovePct:   movePct,
				})
			}
			sort.SliceStable(diffs, func(i, j int) bool {
				return math.Abs(diffs[i].Move) > math.Abs(diffs[j].Move)
			})

			out := map[string]any{
				"since":              since.String(),
				"window":             effectiveWindow.String(),
				"min_move":           minMove,
				"watch":              watchSlugs,
				"window_start":       windowStart.Format(time.RFC3339),
				"window_end":         endTime.Format(time.RFC3339),
				"snapshots_captured": captured,
				"diffs":              diffs,
				"count":              len(diffs),
			}
			if len(diffs) == 0 && captured == 0 {
				out["note"] = "no snapshots in window; sync markets and re-run diff prices on a schedule to build history"
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().DurationVar(&since, "since", 24*time.Hour, "Lookback window for snapshot comparison in Go-duration syntax (e.g. 1h, 24h, 168h for 7 days)")
	cmd.Flags().Float64Var(&minMove, "min-move", 0.05, "Minimum absolute price move to report (in implied probability, e.g. 0.05 = 5c)")
	cmd.Flags().StringVar(&watch, "watch", "", "Comma-separated slug substrings to filter on (e.g. trump-2028,fed-may)")
	cmd.Flags().DurationVar(&window, "window", 0, "Narrower override of --since (e.g. 60m); 0 disables")
	return cmd
}

// captureCurrentPriceSnapshot iterates every market in the local resources
// table and inserts one row per token into price_snapshots. Returns the
// number of new rows inserted. Errors per market are skipped — partial
// capture is better than no capture.
func captureCurrentPriceSnapshot(s *store.Store) int {
	rows, err := s.Query("SELECT data FROM resources WHERE resource_type = 'markets'")
	if err != nil {
		return 0
	}
	defer rows.Close()
	inserted := 0
	now := time.Now().Format("2006-01-02 15:04:05")
	for rows.Next() {
		var raw json.RawMessage
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			continue
		}
		var marketID, question string
		if v, ok := m["id"].(string); ok {
			marketID = v
		} else if v, ok := m["id"].(float64); ok {
			marketID = fmt.Sprintf("%.0f", v)
		}
		if v, ok := m["question"].(string); ok {
			question = v
		}
		// Parse outcomePrices (array of strings: ["0.62","0.38"])
		var prices []string
		if rawPrices, ok := m["outcomePrices"].(string); ok {
			_ = json.Unmarshal([]byte(rawPrices), &prices)
		} else if arr, ok := m["outcomePrices"].([]any); ok {
			for _, p := range arr {
				if ps, ok := p.(string); ok {
					prices = append(prices, ps)
				}
			}
		}
		var tokens []string
		if rawTokens, ok := m["clobTokenIds"].(string); ok {
			_ = json.Unmarshal([]byte(rawTokens), &tokens)
		} else if arr, ok := m["clobTokenIds"].([]any); ok {
			for _, t := range arr {
				if ts, ok := t.(string); ok {
					tokens = append(tokens, ts)
				}
			}
		}
		for i := 0; i < len(prices) && i < len(tokens); i++ {
			var price float64
			_, _ = fmt.Sscanf(prices[i], "%f", &price)
			if price == 0 || tokens[i] == "" {
				continue
			}
			if _, err := s.DB().Exec(
				`INSERT OR IGNORE INTO price_snapshots (token_id, market_id, question, price, captured_at) VALUES (?, ?, ?, ?, ?)`,
				tokens[i], marketID, question, price, now); err == nil {
				inserted++
			}
		}
	}
	return inserted
}
