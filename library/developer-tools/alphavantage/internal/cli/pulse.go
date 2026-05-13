// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// pulse us — daily US-market bundle for /market-pulse. Designed to cost <=2
// AV calls/day on a steady-state run. Hits TOP_GAINERS_LOSERS (cached if
// already snapshotted today) and macro snapshot (0 calls when all five
// indicators are within their TTL), plus a 0-call read of the us-core
// watchlist sentiment.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type pulseUSResult struct {
	Date               string                    `json:"date"`
	Movers             pulseMoversBlock          `json:"movers"`
	Macro              *macroSnapshotResult      `json:"macro,omitempty"`
	WatchlistSentiment *watchlistSentimentResult `json:"watchlist_sentiment,omitempty"`
	APICallsUsedToday  int                       `json:"api_calls_used_today"`
	Warnings           []string                  `json:"warnings,omitempty"`
}

type pulseMoversBlock struct {
	Gainers      []moversBriefEntry `json:"gainers"`
	Losers       []moversBriefEntry `json:"losers"`
	SnapshotDate string             `json:"snapshot_date"`
	FromCache    bool               `json:"from_cache"`
}

func newPulseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pulse",
		Short: "Daily market-pulse bundles (US, etc.)",
	}
	cmd.AddCommand(newPulseUSCmd(flags))
	return cmd
}

func newPulseUSCmd(flags *rootFlags) *cobra.Command {
	var flagWatchlist string

	cmd := &cobra.Command{
		Use:         "us",
		Short:       "US market pulse: top movers + macro + watchlist sentiment, <=2 AV calls/day",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Daily bundle designed for /market-pulse and morning-briefing workflows.
Cost is engineered to fit the 25/day budget:

  - Top gainers/losers: 1 AV call. If today's snapshot is already in
    av_movers_snapshots, reads from cache (0 calls).
  - Macro snapshot: 0 calls when all five indicators are within their TTL;
    otherwise up to 5 calls on first run of the month.
  - Watchlist sentiment: 0 calls — pure local SQL aggregation.

Total steady-state cost: 1 AV call/day after macros are seeded.`,
		Example: strings.Trim(`
  alphavantage-pp-cli pulse us --json
  alphavantage-pp-cli pulse us --watchlist us-core --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			result := pulseUSResult{Date: todayYMD()}

			today := time.Now().UTC().Format("2006-01-02")
			gainers, fromCache, gerr := loadMoversForToday(cmd, db, today, "gainers")
			if gerr == nil {
				result.Movers.Gainers = gainers
				result.Movers.SnapshotDate = today
				result.Movers.FromCache = fromCache
			} else {
				// Fall back to a live TOP_GAINERS_LOSERS call.
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				data, err := c.Get("/query", map[string]string{"function": "TOP_GAINERS_LOSERS"})
				if err != nil {
					logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", false, err.Error())
					result.Warnings = append(result.Warnings, fmt.Sprintf("TOP_GAINERS_LOSERS: %v", trimErrorMessage(err)))
				} else {
					logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", true, "")
					if g, err := extractMoversSide(data, "gainers"); err == nil {
						result.Movers.Gainers = g
						persistMoversSnapshot(cmd, db, today, "gainers", g)
					}
					if l, err := extractMoversSide(data, "losers"); err == nil {
						result.Movers.Losers = l
						persistMoversSnapshot(cmd, db, today, "losers", l)
					}
					result.Movers.SnapshotDate = today
					result.Movers.FromCache = false
				}
			}

			// If gainers came from cache, also fill losers from cache. When the
			// losers snapshot is missing, surface a warning so a human reading
			// the output doesn't mistake the bare null for "AV had no losers
			// today" — the more likely cause is that movers brief was run with
			// --side gainers and the losers row never got persisted.
			if fromCache && len(result.Movers.Losers) == 0 {
				if losers, _, err := loadMoversForToday(cmd, db, today, "losers"); err == nil {
					result.Movers.Losers = losers
				} else {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("losers snapshot unavailable for %s; run `alphavantage-pp-cli sync movers` or `movers brief --side losers` to populate", today))
				}
			}

			// Macro: reuse the snapshot helper which already caches per-indicator.
			macro, mErr := buildMacroSnapshot(cmd, flags, db)
			if mErr == nil {
				result.Macro = macro
			} else {
				result.Warnings = append(result.Warnings, fmt.Sprintf("macro: %v", trimErrorMessage(mErr)))
			}

			// Watchlist sentiment: 0 API calls.
			wlName := flagWatchlist
			if wlName == "" {
				wlName = "us-core"
			}
			// Probe watchlist existence; if absent, return an empty watchlist_sentiment block.
			var n int
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM av_watchlists WHERE name = ?`, wlName,
			).Scan(&n)
			if n > 0 {
				wls := computeWatchlistSentiment(cmd, db, wlName)
				result.WatchlistSentiment = wls
			}

			// API calls used today (from quota_log)
			now := time.Now().UTC()
			todayUTC := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			var used int
			_ = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM av_quota_log WHERE called_at >= ?`,
				todayUTC.Format("2006-01-02 15:04:05"),
			).Scan(&used)
			result.APICallsUsedToday = used

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagWatchlist, "watchlist", "us-core", "Which watchlist to include sentiment for (default: us-core)")
	return cmd
}

// loadMoversForToday reads today's snapshot for one side. Returns
// (rows, fromCache=true, nil) on hit; (nil, false, err) on miss.
func loadMoversForToday(cmd *cobra.Command, db *store.Store, date, side string) ([]moversBriefEntry, bool, error) {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT ticker, price, change_amount, change_percentage, volume FROM av_movers_snapshots
		 WHERE snapshot_date = ? AND side = ?
		 ORDER BY rank LIMIT 20`,
		date, side,
	)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []moversBriefEntry
	for rows.Next() {
		var (
			e            moversBriefEntry
			price        sql.NullFloat64
			changeAmount sql.NullFloat64
			changePct    sql.NullString
			volume       sql.NullInt64
		)
		if err := rows.Scan(&e.Ticker, &price, &changeAmount, &changePct, &volume); err != nil {
			continue
		}
		if price.Valid {
			e.Price = fmt.Sprintf("%.4f", price.Float64)
		}
		if changeAmount.Valid {
			e.ChangeAmount = fmt.Sprintf("%.4f", changeAmount.Float64)
		}
		if changePct.Valid {
			e.ChangePercentage = changePct.String
		}
		if volume.Valid {
			e.Volume = strconv.FormatInt(volume.Int64, 10)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, false, fmt.Errorf("no snapshot for %s on %s", side, date)
	}
	return out, true, nil
}

// buildMacroSnapshot drives the macro snapshot helper without the CLI shell.
// Returns the same result shape as `macro snapshot`.
func buildMacroSnapshot(cmd *cobra.Command, flags *rootFlags, db *store.Store) (*macroSnapshotResult, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	result := &macroSnapshotResult{}
	for _, spec := range macroIndicatorCatalog {
		ind, fromCache, ferr := fetchOrLoadIndicator(cmd, c, db, spec)
		if ferr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", spec.name, trimErrorMessage(ferr)))
			continue
		}
		if fromCache {
			result.CacheHits++
		} else {
			result.FreshCalls++
		}
		assignMacroResultField(result, spec.name, ind)
	}
	return result, nil
}

// computeWatchlistSentiment is the headless version of watchlist sentiment so
// pulse can call it without re-entering the cobra command.
func computeWatchlistSentiment(cmd *cobra.Command, db *store.Store, name string) *watchlistSentimentResult {
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT ticker FROM av_watchlist_tickers WHERE watchlist = ? ORDER BY inserted_at`, name,
	)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var tickers []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tickers = append(tickers, t)
		}
	}
	result := &watchlistSentimentResult{
		Watchlist: name,
		ScanDate:  todayYMD(),
		Tickers:   []watchlistSentimentRow{},
	}
	for _, t := range tickers {
		row := watchlistSentimentRow{Ticker: t}
		mean7, count7, ok7 := windowMean(cmd, db, t, 0, 7)
		meanPrev, _, okPrev := windowMean(cmd, db, t, 7, 14)
		if ok7 {
			m := mean7
			row.MeanSentiment7D = &m
			row.ArticleCount7D = count7
		}
		if okPrev {
			m := meanPrev
			row.MeanSentimentPrev7D = &m
		}
		if ok7 && okPrev {
			d := mean7 - meanPrev
			row.Delta = &d
		}
		result.Tickers = append(result.Tickers, row)
	}
	return result
}
