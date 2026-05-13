// Copyright 2026 ewanchen. Licensed under Apache-2.0. See LICENSE.
//
// screen — multi-predicate filter across local SQLite tables.
// No single AV call expresses "tickers with mean 7d sentiment > X AND
// earnings in next N days AND insider net-buy in last 30 days" — we compose
// it in SQL from av_ticker_sentiment, av_earnings_calendar, and
// av_insider_transactions. Zero API calls.

package cli

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type screenResult struct {
	FiltersApplied []string    `json:"filters_applied"`
	Matched        []screenRow `json:"matched"`
}

type screenRow struct {
	Ticker           string   `json:"ticker"`
	Sentiment7D      *float64 `json:"sentiment_7d,omitempty"`
	NextEarningsDate string   `json:"next_earnings_date,omitempty"`
	InsiderNetShares *float64 `json:"insider_net_shares,omitempty"`
}

func newScreenCmd(flags *rootFlags) *cobra.Command {
	var flagWatchlist string
	var flagSentimentMin string
	var flagHasEarningsIn string
	var flagInsiderNetBuy bool

	cmd := &cobra.Command{
		Use:         "screen",
		Short:       "Compound local-table screen across sentiment + earnings + insider data",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Filter tickers by multi-predicate SQL across the local sentiment / earnings /
insider tables. All predicates AND together. Universe is the watchlist
specified with --watchlist; if omitted, all tickers known to
av_ticker_sentiment.

Filters:
  --watchlist NAME        Restrict to tickers in a named watchlist
  --sentiment-min X       Mean ticker_sentiment_score over last 7 days >= X
  --has-earnings-in Nd    av_earnings_calendar has a row within next N days
  --insider-net-buy       av_insider_transactions net acquisitions > 0 in last 30 days

Zero API calls — runs entirely against the local store. Empty results are
valid (return []), not an error.`,
		Example: strings.Trim(`
  alphavantage-pp-cli screen --watchlist us-core --sentiment-min 0.2 --json
  alphavantage-pp-cli screen --watchlist us-core --sentiment-min 0.2 --has-earnings-in 14d --insider-net-buy --json
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

			filters := []string{}
			if flagWatchlist != "" {
				filters = append(filters, "watchlist="+flagWatchlist)
			}
			if flagSentimentMin != "" {
				filters = append(filters, "sentiment_7d_mean>="+flagSentimentMin)
			}
			if flagHasEarningsIn != "" {
				filters = append(filters, "earnings_within="+flagHasEarningsIn)
			}
			if flagInsiderNetBuy {
				filters = append(filters, "insider_net_buy=true")
			}

			tickers, err := screenUniverse(cmd, db, flagWatchlist)
			if err != nil {
				return err
			}

			// Parse sentiment-min if present.
			var sentMin float64
			haveSent := false
			if flagSentimentMin != "" {
				v, perr := strconv.ParseFloat(flagSentimentMin, 64)
				if perr != nil {
					return usageErr(fmt.Errorf("invalid --sentiment-min value %q: %w", flagSentimentMin, perr))
				}
				sentMin = v
				haveSent = true
			}

			// Parse has-earnings-in (e.g. "14d", "30d").
			earningsDays := 0
			if flagHasEarningsIn != "" {
				s := strings.TrimSuffix(flagHasEarningsIn, "d")
				v, perr := strconv.Atoi(s)
				if perr != nil || v <= 0 {
					return usageErr(fmt.Errorf("invalid --has-earnings-in value %q: use e.g. 14d, 30d", flagHasEarningsIn))
				}
				earningsDays = v
			}

			matched := []screenRow{}
			for _, t := range tickers {
				// Sentiment predicate
				var sent float64
				var sentCount int
				if err := db.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(AVG(ticker_sentiment_score), 0), COUNT(*)
					 FROM av_ticker_sentiment
					 WHERE ticker = ?
					   AND substr(time_published, 1, 8) >= strftime('%Y%m%d', 'now', '-7 days')`,
					t,
				).Scan(&sent, &sentCount); err != nil {
					continue
				}
				if haveSent {
					// require at least 1 article AND mean >= threshold
					if sentCount == 0 || sent < sentMin {
						continue
					}
				} else if !flagInsiderNetBuy && earningsDays == 0 {
					// No filters at all: screen acts as a "find tickers with
					// SOME local signal" — require at least one sentiment row.
					// Without this, an empty store still returns the full
					// watchlist with all-null fields, which isn't useful as a
					// screen result.
					if sentCount == 0 {
						continue
					}
				}

				// Earnings predicate
				var earningsDate string
				if earningsDays > 0 {
					err := db.DB().QueryRowContext(cmd.Context(),
						`SELECT report_date FROM av_earnings_calendar
						 WHERE symbol = ?
						   AND date(report_date) >= date('now')
						   AND date(report_date) <= date('now', ?)
						 ORDER BY report_date LIMIT 1`,
						t, fmt.Sprintf("+%d days", earningsDays),
					).Scan(&earningsDate)
					if err == sql.ErrNoRows {
						continue
					}
					if err != nil {
						continue
					}
				} else {
					// Optional next-earnings fetch when filter not applied
					_ = db.DB().QueryRowContext(cmd.Context(),
						`SELECT report_date FROM av_earnings_calendar
						 WHERE symbol = ?
						   AND date(report_date) >= date('now')
						 ORDER BY report_date LIMIT 1`,
						t,
					).Scan(&earningsDate)
				}

				// Insider predicate
				var netShares float64
				_ = db.DB().QueryRowContext(cmd.Context(),
					`SELECT COALESCE(SUM(
					   CASE WHEN UPPER(acquisition_or_disposal) = 'A' THEN shares
					        WHEN UPPER(acquisition_or_disposal) = 'D' THEN -shares
					        ELSE 0 END
					 ), 0)
					 FROM av_insider_transactions
					 WHERE symbol = ?
					   AND date(transaction_date) >= date('now', '-30 days')`,
					t,
				).Scan(&netShares)
				if flagInsiderNetBuy && netShares <= 0 {
					continue
				}

				row := screenRow{Ticker: t}
				if sentCount > 0 {
					s := sent
					row.Sentiment7D = &s
				}
				if earningsDate != "" {
					row.NextEarningsDate = earningsDate
				}
				if netShares != 0 {
					n := netShares
					row.InsiderNetShares = &n
				}
				matched = append(matched, row)
			}

			result := screenResult{FiltersApplied: filters, Matched: matched}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagWatchlist, "watchlist", "", "Restrict universe to this saved watchlist")
	cmd.Flags().StringVar(&flagSentimentMin, "sentiment-min", "", "Minimum mean ticker_sentiment_score over last 7 days (e.g. 0.2)")
	cmd.Flags().StringVar(&flagHasEarningsIn, "has-earnings-in", "", "Tickers with earnings within this window (e.g. 14d, 30d)")
	cmd.Flags().BoolVar(&flagInsiderNetBuy, "insider-net-buy", false, "Tickers with positive insider net share count over last 30 days")
	return cmd
}

// screenUniverse returns the candidate ticker list. With --watchlist, reads
// from av_watchlist_tickers; otherwise, returns all tickers that appear in
// av_ticker_sentiment (the universe with usable sentiment data).
func screenUniverse(cmd *cobra.Command, db *store.Store, watchlist string) ([]string, error) {
	if watchlist != "" {
		var n int
		err := db.DB().QueryRowContext(cmd.Context(),
			`SELECT COUNT(*) FROM av_watchlists WHERE name = ?`, watchlist,
		).Scan(&n)
		if err != nil || n == 0 {
			return nil, notFoundErr(fmt.Errorf("watchlist %q not found — create it with: alphavantage-pp-cli watchlist add --name %s --ticker AAPL", watchlist, watchlist))
		}
		rows, err := db.DB().QueryContext(cmd.Context(),
			`SELECT ticker FROM av_watchlist_tickers WHERE watchlist = ? ORDER BY inserted_at`, watchlist,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err == nil {
				out = append(out, t)
			}
		}
		return out, nil
	}
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT DISTINCT ticker FROM av_ticker_sentiment ORDER BY ticker`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			out = append(out, t)
		}
	}
	return out, nil
}
