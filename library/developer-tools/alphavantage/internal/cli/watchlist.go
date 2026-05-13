// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// watchlist — named persisted ticker lists, with a sentiment-aggregator
// subcommand that reads from the local av_ticker_sentiment store.

package cli

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Named persisted watchlists for sweep / screen / sentiment",
		Long: `Manage named persisted ticker lists. Watchlists are stored in the local
SQLite database and consumed by:
  news sweep --watchlist NAME
  watchlist sentiment --name NAME
  screen --watchlist NAME
  pulse us (reads the 'us-core' watchlist by convention)`,
	}
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistShowCmd(flags))
	cmd.AddCommand(newWatchlistSentimentCmd(flags))
	return cmd
}

type watchlistAddResult struct {
	Watchlist string `json:"watchlist"`
	Ticker    string `json:"ticker"`
	Added     bool   `json:"added"`
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	var flagTicker string
	var flagDescription string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a ticker to a named watchlist (creates the watchlist if absent)",
		Example: strings.Trim(`
  alphavantage-pp-cli watchlist add --name us-core --ticker AAPL --json
  alphavantage-pp-cli watchlist add --name us-core --ticker NVDA --description "core holdings" --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagName == "" || flagTicker == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			_, _ = db.DB().ExecContext(cmd.Context(),
				`INSERT OR IGNORE INTO av_watchlists (name, description) VALUES (?, ?)`,
				flagName, flagDescription,
			)
			ticker := strings.ToUpper(flagTicker)
			res, err := db.DB().ExecContext(cmd.Context(),
				`INSERT OR IGNORE INTO av_watchlist_tickers (watchlist, ticker) VALUES (?, ?)`,
				flagName, ticker,
			)
			if err != nil {
				return fmt.Errorf("adding ticker: %w", err)
			}
			n, _ := res.RowsAffected()
			return printJSONFiltered(cmd.OutOrStdout(), watchlistAddResult{
				Watchlist: flagName,
				Ticker:    ticker,
				Added:     n > 0,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagName, "name", "", "Watchlist name (e.g., us-core)")
	cmd.Flags().StringVar(&flagTicker, "ticker", "", "Ticker symbol to add (e.g., AAPL)")
	cmd.Flags().StringVar(&flagDescription, "description", "", "Optional description (only used on first creation)")
	return cmd
}

type watchlistRemoveResult struct {
	Watchlist string `json:"watchlist"`
	Ticker    string `json:"ticker"`
	Removed   bool   `json:"removed"`
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	var flagTicker string
	cmd := &cobra.Command{
		Use:   "remove",
		Short: "Remove a ticker from a watchlist",
		Example: strings.Trim(`
  alphavantage-pp-cli watchlist remove --name us-core --ticker AAPL --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagName == "" || flagTicker == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			ticker := strings.ToUpper(flagTicker)
			res, err := db.DB().ExecContext(cmd.Context(),
				`DELETE FROM av_watchlist_tickers WHERE watchlist = ? AND ticker = ?`,
				flagName, ticker,
			)
			if err != nil {
				return fmt.Errorf("removing ticker: %w", err)
			}
			n, _ := res.RowsAffected()
			return printJSONFiltered(cmd.OutOrStdout(), watchlistRemoveResult{
				Watchlist: flagName,
				Ticker:    ticker,
				Removed:   n > 0,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagName, "name", "", "Watchlist name")
	cmd.Flags().StringVar(&flagTicker, "ticker", "", "Ticker symbol to remove")
	return cmd
}

type watchlistListItem struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	TickerCount int    `json:"ticker_count"`
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all saved watchlists with their ticker counts",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  alphavantage-pp-cli watchlist list --json
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
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT w.name, COALESCE(w.description, ''),
				        (SELECT COUNT(*) FROM av_watchlist_tickers t WHERE t.watchlist = w.name) AS n
				 FROM av_watchlists w
				 ORDER BY w.name`,
			)
			if err != nil {
				return fmt.Errorf("reading watchlists: %w", err)
			}
			defer rows.Close()
			items := []watchlistListItem{}
			for rows.Next() {
				var it watchlistListItem
				if err := rows.Scan(&it.Name, &it.Description, &it.TickerCount); err == nil {
					items = append(items, it)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), items, flags)
		},
	}
	return cmd
}

type watchlistShowResult struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tickers     []string `json:"tickers"`
}

func newWatchlistShowCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	cmd := &cobra.Command{
		Use:         "show",
		Short:       "Show the tickers in a watchlist",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  alphavantage-pp-cli watchlist show --name us-core --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagName == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			var description string
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(description, '') FROM av_watchlists WHERE name = ?`, flagName,
			).Scan(&description)
			if errors.Is(err, sql.ErrNoRows) {
				return notFoundErr(fmt.Errorf("watchlist %q not found — create it with: alphavantage-pp-cli watchlist add --name %s --ticker AAPL", flagName, flagName))
			}
			if err != nil {
				return fmt.Errorf("reading watchlist: %w", err)
			}
			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT ticker FROM av_watchlist_tickers WHERE watchlist = ? ORDER BY inserted_at`,
				flagName,
			)
			if err != nil {
				return fmt.Errorf("reading tickers: %w", err)
			}
			defer rows.Close()
			result := watchlistShowResult{Name: flagName, Description: description, Tickers: []string{}}
			for rows.Next() {
				var t string
				if err := rows.Scan(&t); err == nil {
					result.Tickers = append(result.Tickers, t)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagName, "name", "", "Watchlist name")
	return cmd
}

type watchlistSentimentResult struct {
	Watchlist string                  `json:"watchlist"`
	ScanDate  string                  `json:"scan_date"`
	Tickers   []watchlistSentimentRow `json:"tickers"`
}

type watchlistSentimentRow struct {
	Ticker              string   `json:"ticker"`
	MeanSentiment7D     *float64 `json:"mean_sentiment_7d"`
	MeanSentimentPrev7D *float64 `json:"mean_sentiment_prev_7d"`
	Delta               *float64 `json:"delta"`
	ArticleCount7D      int      `json:"article_count_7d"`
}

func newWatchlistSentimentCmd(flags *rootFlags) *cobra.Command {
	var flagName string
	cmd := &cobra.Command{
		Use:         "sentiment",
		Short:       "Per-ticker mean sentiment for a watchlist with 7d delta (no API call)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `For each ticker in a watchlist, computes:
  - mean_sentiment_7d:       average ticker_sentiment_score over the last 7 days
  - mean_sentiment_prev_7d:  average over the preceding 7 days (days 8..14)
  - delta:                   mean_sentiment_7d - mean_sentiment_prev_7d
  - article_count_7d:        rows in av_ticker_sentiment over last 7 days

Pure local SQL — zero API calls. Run 'news sweep --watchlist NAME' first to
populate the sentiment data.`,
		Example: strings.Trim(`
  alphavantage-pp-cli watchlist sentiment --name us-core --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagName == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			// Confirm watchlist exists
			var n int
			err = db.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM av_watchlists WHERE name = ?`, flagName,
			).Scan(&n)
			if err != nil || n == 0 {
				return notFoundErr(fmt.Errorf("watchlist %q not found — create it with: alphavantage-pp-cli watchlist add --name %s --ticker AAPL", flagName, flagName))
			}

			rows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT ticker FROM av_watchlist_tickers WHERE watchlist = ? ORDER BY inserted_at`,
				flagName,
			)
			if err != nil {
				return fmt.Errorf("reading tickers: %w", err)
			}
			defer rows.Close()

			var tickers []string
			for rows.Next() {
				var t string
				if err := rows.Scan(&t); err == nil {
					tickers = append(tickers, t)
				}
			}

			result := watchlistSentimentResult{
				Watchlist: flagName,
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

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagName, "name", "", "Watchlist name (required)")
	return cmd
}

// windowMean returns mean ticker_sentiment_score for a ticker over the window
// (now - high_days, now - low_days]. Used by watchlist sentiment to compute
// both current and prior week means with a single helper.
func windowMean(cmd *cobra.Command, db *store.Store, ticker string, lowDays, highDays int) (float64, int, bool) {
	var mean float64
	var count int
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT COALESCE(AVG(ticker_sentiment_score), 0), COUNT(*)
		 FROM av_ticker_sentiment
		 WHERE ticker = ?
		   AND substr(time_published, 1, 8) >= strftime('%Y%m%d', 'now', ?)
		   AND substr(time_published, 1, 8) < strftime('%Y%m%d', 'now', ?)`,
		ticker, fmt.Sprintf("-%d days", highDays), fmt.Sprintf("-%d days", lowDays),
	).Scan(&mean, &count)
	if err != nil {
		return 0, 0, false
	}
	return mean, count, count > 0
}

// todayYMD returns the UTC date in YYYY-MM-DD form for the scan_date field.
func todayYMD() string {
	return time.Now().UTC().Format("2006-01-02")
}
