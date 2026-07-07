// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type mispricedPair struct {
	Match        float64      `json:"match"`
	PM           compareVenue `json:"polymarket"`
	Kalshi       compareVenue `json:"kalshi"`
	Delta        float64      `json:"delta"`
	DeltaPercent float64      `json:"deltaPercent"`
}

type mispricedResult struct {
	Threshold       float64         `json:"threshold"`
	Count           int             `json:"count"`
	Considered      int             `json:"considered,omitempty"`
	UntradedSkipped int             `json:"untradedSkipped,omitempty"`
	MaxDelta        float64         `json:"maxDelta,omitempty"`
	Pairs           []mispricedPair `json:"pairs"`
	Meta            *freshnessMeta  `json:"meta,omitempty"`
}

func newMispricedCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "mispriced",
		Short: "Find same-outcome Polymarket and Kalshi markets with price disagreement",
		Example: `  prediction-goat-pp-cli mispriced --threshold 0.05 --json
  prediction-goat-pp-cli mispriced --threshold 0.1 --limit 20`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit < 1 {
				return fmt.Errorf("mispriced: --limit must be greater than zero")
			}
			if threshold < 0 {
				return fmt.Errorf("mispriced: --threshold must be non-negative")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.Open(dbPath)
			if err != nil {
				return fmt.Errorf("mispriced: %w", err)
			}
			defer db.Close()
			result, err := runMispriced(cmd, db, threshold, limit)
			if err != nil {
				return fmt.Errorf("mispriced: %w", err)
			}
			// Live-on-read freshness: refresh prices on every pair and
			// recompute Delta from the refreshed values so the displayed
			// spread is never a stale cache artifact. Pairs that fall
			// below the threshold under fresh prices are dropped.
			outcome := refreshMispricedPairs(cmd.Context(), nil, &result, threshold)
			result.Meta = buildFreshnessMeta(outcome, indexSyncedAt(db))
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				if err := printSimpleTable(cmd.OutOrStdout(), []string{"Match", "PM Title", "Kalshi Title", "PM%", "Kalshi%", "Delta"}, mispricedRows(result.Pairs)); err != nil {
					return err
				}
				if footer := freshnessFooterLine(result.Meta); footer != "" {
					fmt.Fprintln(cmd.OutOrStdout(), footer)
				}
			}
			if len(result.Pairs) == 0 {
				if hint := emptyStoreHint(cmd, dbPath, "mispriced", "all"); hint != nil {
					return hint
				}
			}
			return nil
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 0.05, "Minimum probability delta")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max pairs returned")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

func runMispriced(cmd *cobra.Command, db *store.Store, threshold float64, limit int) (mispricedResult, error) {
	pmMarkets, err := loadMispricedMarkets(cmd, db, `SELECT id, data FROM resources
WHERE resource_type='markets'
AND json_extract(data,'$.closed')=0
AND json_extract(data,'$.lastTradePrice') IS NOT NULL
ORDER BY CAST(COALESCE(json_extract(data,'$.volumeNum'),0) AS REAL) DESC LIMIT 500`, "markets")
	if err != nil {
		return mispricedResult{}, err
	}
	kalshiMarkets, err := loadMispricedMarkets(cmd, db, `SELECT id, data FROM resources
WHERE resource_type='kalshi_markets'
AND json_extract(data,'$.status')='active'
AND json_extract(data,'$.last_price_dollars') IS NOT NULL
ORDER BY CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) DESC LIMIT 500`, "kalshi_markets")
	if err != nil {
		return mispricedResult{}, err
	}

	pairs := make([]mispricedPair, 0)
	considered := 0
	untradedSkipped := 0
	maxDelta := 0.0
	// usedKalshi tracks indices already paired so two differently-worded
	// Polymarket markets for the same event don't both pair to the same
	// Kalshi market, double-counting the implied divergence. Matches the
	// pairCompareMarkets guard in compare.go.
	usedKalshi := make(map[int]bool, len(kalshiMarkets))
	for _, pm := range pmMarkets {
		bestIdx := -1
		bestScore := 0.0
		for i, kalshi := range kalshiMarkets {
			if usedKalshi[i] {
				continue
			}
			if score := tokenJaccard(pm.Title, kalshi.Title); score > bestScore {
				bestIdx = i
				bestScore = score
			}
		}
		if bestIdx < 0 || bestScore < 0.20 {
			continue
		}
		kalshi := kalshiMarkets[bestIdx]
		// Untraded Kalshi markets carry a platform-default ask rather than
		// a real implied probability; pairing them with Polymarket markets
		// produces false-positive divergence (e.g. Polymarket 6% vs an
		// untraded Kalshi default of 17%). Skip them silently — the screen
		// is for actionable cross-venue mispricings, not noise. Don't mark
		// the index used so a subsequent Polymarket market could still
		// pair (in case a different PM title is the right match), but the
		// candidate is dropped regardless.
		if kalshi.Untraded {
			untradedSkipped++
			continue
		}
		usedKalshi[bestIdx] = true
		considered++
		delta := pm.YesProbability - kalshi.YesProbability
		if math.Abs(delta) > math.Abs(maxDelta) {
			maxDelta = delta
		}
		if math.Abs(delta) < threshold {
			continue
		}
		pairs = append(pairs, mispricedPair{Match: bestScore, PM: compareVenueFromRaw(pm), Kalshi: compareVenueFromRaw(kalshi), Delta: delta, DeltaPercent: roundDelta(delta)})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return math.Abs(pairs[i].Delta) > math.Abs(pairs[j].Delta)
	})
	if len(pairs) > limit {
		pairs = pairs[:limit]
	}
	return mispricedResult{Threshold: threshold, Count: len(pairs), Considered: considered, UntradedSkipped: untradedSkipped, MaxDelta: maxDelta, Pairs: pairs}, nil
}

// refreshMispricedPairs refreshes every pair's PM and Kalshi prices
// from upstream, recomputes Delta from the live values, and drops
// pairs that fall below the threshold under refreshed prices.
// Result.Count is updated to reflect the surviving pairs.
func refreshMispricedPairs(ctx context.Context, fc freshnessClient, result *mispricedResult, threshold float64) refreshOutcome {
	polySlugs := make([]string, 0, len(result.Pairs))
	kalshiTickers := make([]string, 0, len(result.Pairs))
	for _, p := range result.Pairs {
		polySlugs = append(polySlugs, p.PM.ID)
		kalshiTickers = append(kalshiTickers, p.Kalshi.ID)
	}
	outcome := refreshVenues(ctx, fc, polySlugs, kalshiTickers)
	var dummyStatus string
	filtered := result.Pairs[:0]
	for _, p := range result.Pairs {
		if outcome.PolymarketOK {
			if v, ok := outcome.Polymarket[p.PM.ID]; ok {
				applyLiveValuesIfPresent(v, &p.PM.YesProbability, &p.PM.Volume24h, &dummyStatus)
			}
		}
		if outcome.KalshiOK {
			if v, ok := outcome.Kalshi[p.Kalshi.ID]; ok {
				applyLiveValuesIfPresent(v, &p.Kalshi.YesProbability, &p.Kalshi.Volume24h, &dummyStatus)
			}
		}
		p.Delta = p.PM.YesProbability - p.Kalshi.YesProbability
		// PATCH(mispriced-refresh-derived-fields): keep YesPercent and
		// DeltaPercent in sync with the refreshed YesProbability/Delta
		// so agent-facing JSON never reports yesProbability=0.60
		// alongside yesPercent=55.0 (the pre-refresh value). Mirrors
		// the post-refresh recompute already in compare.go. Greptile
		// P1 on PR #780.
		p.PM.YesPercent = yesPercent(p.PM.YesProbability)
		p.Kalshi.YesPercent = yesPercent(p.Kalshi.YesProbability)
		p.DeltaPercent = roundDelta(p.Delta)
		// Re-filter on threshold against refreshed prices. We use a
		// math.Abs comparison consistent with runMispriced. When the
		// refresh failed for either venue, keep the pair so the user
		// still sees the index answer with the staleness flag — the
		// stale value is the best signal we have.
		if outcome.PolymarketOK && outcome.KalshiOK && math.Abs(p.Delta) < threshold {
			continue
		}
		filtered = append(filtered, p)
	}
	// Re-sort by absolute spread under refreshed prices.
	sort.Slice(filtered, func(i, j int) bool {
		return math.Abs(filtered[i].Delta) > math.Abs(filtered[j].Delta)
	})
	result.Pairs = filtered
	result.Count = len(filtered)
	return outcome
}

func loadMispricedMarkets(cmd *cobra.Command, db *store.Store, query, resourceType string) ([]rawMarket, error) {
	rows, err := db.DB().QueryContext(cmd.Context(), query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	markets := make([]rawMarket, 0)
	for rows.Next() {
		var id, data sql.NullString
		if err := rows.Scan(&id, &data); err != nil {
			return nil, err
		}
		if !data.Valid {
			continue
		}
		market, ok := rawMarketFromJSON(resourceType, id.String, data.String)
		if ok {
			markets = append(markets, market)
		}
	}
	return markets, rows.Err()
}

// roundDelta returns the signed probability delta rendered as a percent
// rounded to one decimal place. Pairs the canonical 0-1 `delta` field with
// a `deltaPercent` companion for apples-to-apples cross-venue display.
func roundDelta(delta float64) float64 {
	sign := 1.0
	abs := delta
	if delta < 0 {
		sign = -1.0
		abs = -delta
	}
	return sign * float64(int(abs*1000+0.5)) / 10
}

func tokenJaccard(a, b string) float64 {
	aTokens := tokenSet(a)
	bTokens := tokenSet(b)
	if len(aTokens) == 0 || len(bTokens) == 0 {
		return 0
	}
	intersection := 0
	for token := range aTokens {
		if bTokens[token] {
			intersection++
		}
	}
	union := len(aTokens) + len(bTokens) - intersection
	return float64(intersection) / float64(union)
}

func tokenSet(s string) map[string]bool {
	parts := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := make(map[string]bool, len(parts))
	for _, part := range parts {
		if len(part) > 1 {
			out[part] = true
		}
	}
	return out
}

func mispricedRows(pairs []mispricedPair) [][]string {
	rows := make([][]string, 0, len(pairs))
	for _, pair := range pairs {
		rows = append(rows, []string{
			fmt.Sprintf("%.2f", pair.Match),
			truncate(pair.PM.Title, 48),
			truncate(pair.Kalshi.Title, 48),
			formatProb(pair.PM.YesProbability),
			formatProb(pair.Kalshi.YesProbability),
			fmt.Sprintf("%+.1f%%", pair.Delta*100),
		})
	}
	return rows
}
