// Copyright 2026 ewanchen. Licensed under Apache-2.0. See LICENSE.
//
// news search — full-text search against the locally pulled news corpus.
// Uses SQLite FTS5. Zero API calls.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type newsSearchResult struct {
	Query   string            `json:"query"`
	Matches []newsSearchMatch `json:"matches"`
}

type newsSearchMatch struct {
	Title                 string                `json:"title"`
	URL                   string                `json:"url"`
	TimePublished         string                `json:"time_published"`
	Source                string                `json:"source"`
	OverallSentimentLabel string                `json:"overall_sentiment_label"`
	OverallSentimentScore float64               `json:"overall_sentiment_score"`
	TickerSentiment       []newsTickerSentiment `json:"ticker_sentiment,omitempty"`
}

type newsTickerSentiment struct {
	Ticker string  `json:"ticker"`
	Score  float64 `json:"score"`
	Label  string  `json:"label"`
}

func newNewsSearchCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTickers string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "search <query>",
		Short:       "FTS5 full-text search over locally pulled news (no API call)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Run a SQLite FTS5 MATCH query against the local news corpus. Supports
boolean operators (AND, OR, NOT), prefix matching ('cloud*'), and quoted
phrases ("data center").

This command makes zero API calls. Populate the local corpus first with
'news sweep' or 'news sentiment'.

Empty results are valid: an unfound query returns an empty matches array.`,
		Example: strings.Trim(`
  alphavantage-pp-cli news search "tariff" --json
  alphavantage-pp-cli news search "AI agent" --tickers NVDA --json
  alphavantage-pp-cli news search "data center OR cloud" --from 20260101 --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if flagLimit <= 0 {
				flagLimit = 50
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			result := newsSearchResult{Query: query, Matches: []newsSearchMatch{}}

			// Build a WHERE clause that joins FTS5 to the structured articles table.
			// AV time_published is "YYYYMMDDTHHMMSS"; the --from filter compares
			// the leading YYYYMMDD prefix.
			sqlText := `
				SELECT a.title, a.url, a.time_published, a.source,
				       a.overall_sentiment_label, a.overall_sentiment_score
				FROM av_news_fts fts
				JOIN av_news_articles a ON a.url = fts.url
				WHERE av_news_fts MATCH ?`
			argsList := []any{query}
			if flagFrom != "" {
				sqlText += ` AND substr(a.time_published, 1, 8) >= ?`
				// Allow user to pass YYYY-MM-DD or YYYYMMDD; normalize.
				normalized := strings.ReplaceAll(flagFrom, "-", "")
				argsList = append(argsList, normalized)
			}
			sqlText += ` ORDER BY a.time_published DESC LIMIT ?`
			argsList = append(argsList, flagLimit)

			rows, err := db.DB().QueryContext(cmd.Context(), sqlText, argsList...)
			if err != nil {
				// FTS5 syntax errors land here — surface them with a hint.
				return fmt.Errorf("FTS5 query failed: %w\nhint: try simpler terms or quote phrases. Examples: \"tariff\", \"data center\", \"cloud OR AI\"", err)
			}
			defer rows.Close()

			tickerFilter := map[string]bool{}
			if flagTickers != "" {
				for _, t := range strings.Split(flagTickers, ",") {
					t = strings.TrimSpace(strings.ToUpper(t))
					if t != "" {
						tickerFilter[t] = true
					}
				}
			}

			for rows.Next() {
				var m newsSearchMatch
				if err := rows.Scan(&m.Title, &m.URL, &m.TimePublished, &m.Source,
					&m.OverallSentimentLabel, &m.OverallSentimentScore); err != nil {
					continue
				}
				// Pull ticker_sentiment rows for context.
				tsRows, terr := db.DB().QueryContext(cmd.Context(),
					`SELECT ticker, ticker_sentiment_score, ticker_sentiment_label
					 FROM av_ticker_sentiment WHERE url = ?`, m.URL)
				if terr == nil {
					for tsRows.Next() {
						var ts newsTickerSentiment
						if err := tsRows.Scan(&ts.Ticker, &ts.Score, &ts.Label); err == nil {
							m.TickerSentiment = append(m.TickerSentiment, ts)
						}
					}
					tsRows.Close()
				}

				// Apply ticker filter post-fetch (cheaper than complex SQL).
				if len(tickerFilter) > 0 {
					hit := false
					for _, ts := range m.TickerSentiment {
						if tickerFilter[ts.Ticker] {
							hit = true
							break
						}
					}
					if !hit {
						continue
					}
				}

				result.Matches = append(result.Matches, m)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest publish date (YYYY-MM-DD or YYYYMMDD)")
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Restrict to articles mentioning these tickers (comma-separated)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max matches to return")
	return cmd
}
