// Copyright 2026 ewanchen. Licensed under Apache-2.0. See LICENSE.
//
// news sweep — sweep NEWS_SENTIMENT for a watchlist or explicit ticker list,
// persisting articles AND the per-ticker sentiment array. Every other AV
// wrapper drops the ticker_sentiment[] array; we keep it in SQLite so
// follow-on queries (news timeline, watchlist sentiment, screen) can read it.
//
// Cost: 1 AV call per ticker. Rate-limit failures are reported per-ticker so
// a partial sweep is still useful.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type newsSweepResult struct {
	TickersProcessed         int      `json:"tickers_processed"`
	ArticlesAdded            int      `json:"articles_added"`
	ArticlesSkippedDuplicate int      `json:"articles_skipped_duplicate"`
	TickerSentimentRowsAdded int      `json:"ticker_sentiment_rows_added"`
	Errors                   []string `json:"errors,omitempty"`
}

// newsSentimentResponse is the subset of NEWS_SENTIMENT we persist. AV emits
// extra fields (items, sentiment_score_definition, etc.) — we ignore them.
type newsSentimentResponse struct {
	Feed []newsArticle `json:"feed"`
}

type newsArticle struct {
	Title                 string                    `json:"title"`
	URL                   string                    `json:"url"`
	TimePublished         string                    `json:"time_published"`
	Authors               []string                  `json:"authors"`
	Summary               string                    `json:"summary"`
	Source                string                    `json:"source"`
	SourceDomain          string                    `json:"source_domain"`
	CategoryWithinSource  string                    `json:"category_within_source"`
	OverallSentimentScore float64                   `json:"overall_sentiment_score"`
	OverallSentimentLabel string                    `json:"overall_sentiment_label"`
	Topics                []map[string]any          `json:"topics"`
	TickerSentiment       []newsTickerSentimentItem `json:"ticker_sentiment"`
}

type newsTickerSentimentItem struct {
	Ticker               string `json:"ticker"`
	RelevanceScore       string `json:"relevance_score"`
	TickerSentimentScore string `json:"ticker_sentiment_score"`
	TickerSentimentLabel string `json:"ticker_sentiment_label"`
}

func newNewsSweepCmd(flags *rootFlags) *cobra.Command {
	var flagTickers string
	var flagWatchlist string
	var flagTopics string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "sweep",
		Short: "Sweep NEWS_SENTIMENT for tickers; persist articles + per-ticker sentiment",
		Long: `Sweep NEWS_SENTIMENT for an explicit ticker list OR a saved watchlist.
For each ticker, issues one NEWS_SENTIMENT API call and persists the response
to the local SQLite store. Crucially preserves the per-article
ticker_sentiment[] array — every other AV wrapper drops this field, but it's
the highest-information signal in the entire API.

Cost: 1 AV call per ticker. Free tier is 25/day total — use 'quota plan' first
to confirm budget.

Articles are deduped by URL; ticker_sentiment rows are deduped by (url, ticker).`,
		Example: strings.Trim(`
  alphavantage-pp-cli news sweep --tickers AAPL,MSFT,NVDA --json
  alphavantage-pp-cli news sweep --watchlist us-core --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTickers == "" && flagWatchlist == "" {
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

			tickers, err := resolveTickerSet(cmd, db, flagTickers, flagWatchlist)
			if err != nil {
				return err
			}
			if len(tickers) == 0 {
				return fmt.Errorf("no tickers to sweep; pass --tickers AAPL,MSFT or --watchlist NAME")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := newsSweepResult{}
			for _, t := range tickers {
				params := map[string]string{
					"function": "NEWS_SENTIMENT",
					"tickers":  t,
				}
				if flagTopics != "" {
					params["topics"] = flagTopics
				}
				if flagLimit > 0 {
					params["limit"] = fmt.Sprintf("%d", flagLimit)
				}
				data, err := c.Get("/query", params)
				if err != nil {
					logQuotaCall(cmd, db, "NEWS_SENTIMENT", t, false, err.Error())
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", t, err))
					continue
				}
				logQuotaCall(cmd, db, "NEWS_SENTIMENT", t, true, "")
				result.TickersProcessed++

				added, skipped, tsRows, perr := persistNewsResponse(db, data)
				if perr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: persist: %v", t, perr))
					continue
				}
				result.ArticlesAdded += added
				result.ArticlesSkippedDuplicate += skipped
				result.TickerSentimentRowsAdded += tsRows
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Comma-separated tickers (e.g., AAPL,MSFT,NVDA)")
	cmd.Flags().StringVar(&flagWatchlist, "watchlist", "", "Saved watchlist name (alternative to --tickers)")
	cmd.Flags().StringVar(&flagTopics, "topics", "", "Comma-separated topics filter (e.g., earnings,technology)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max articles per ticker (AV cap is 1000)")
	return cmd
}

// resolveTickerSet builds the ticker slice from --tickers (explicit) or
// --watchlist (look up av_watchlist_tickers). When both are provided,
// --tickers wins (explicit beats implicit).
func resolveTickerSet(cmd *cobra.Command, db *store.Store, explicit, watchlist string) ([]string, error) {
	if explicit != "" {
		var out []string
		for _, t := range strings.Split(explicit, ",") {
			t = strings.TrimSpace(strings.ToUpper(t))
			if t != "" {
				out = append(out, t)
			}
		}
		return out, nil
	}
	if watchlist == "" {
		return nil, nil
	}
	// Confirm watchlist exists first so we can issue a helpful error.
	var name string
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT name FROM av_watchlists WHERE name = ?`, watchlist,
	).Scan(&name)
	if err != nil {
		return nil, fmt.Errorf("watchlist %q not found. create it with: alphavantage-pp-cli watchlist add --name %s --ticker AAPL", watchlist, watchlist)
	}
	rows, err := db.DB().QueryContext(cmd.Context(),
		`SELECT ticker FROM av_watchlist_tickers WHERE watchlist = ? ORDER BY inserted_at`,
		watchlist,
	)
	if err != nil {
		return nil, fmt.Errorf("reading watchlist %q: %w", watchlist, err)
	}
	defer rows.Close()
	var tickers []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			tickers = append(tickers, t)
		}
	}
	if len(tickers) == 0 {
		return nil, fmt.Errorf("watchlist %q is empty. add a ticker with: alphavantage-pp-cli watchlist add --name %s --ticker AAPL", watchlist, watchlist)
	}
	return tickers, nil
}

// persistNewsResponse parses the NEWS_SENTIMENT response and upserts articles,
// FTS5 rows, and ticker_sentiment rows. Returns (added, skippedDuplicate, tsRows, error).
func persistNewsResponse(db *store.Store, data json.RawMessage) (int, int, int, error) {
	var resp newsSentimentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, 0, 0, fmt.Errorf("parsing NEWS_SENTIMENT response: %w", err)
	}

	added := 0
	skipped := 0
	tsRows := 0

	tx, err := db.DB().Begin()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	for _, a := range resp.Feed {
		if a.URL == "" {
			continue
		}
		topicsJSON, _ := json.Marshal(a.Topics)
		authors := strings.Join(a.Authors, ", ")

		res, err := tx.Exec(
			`INSERT OR IGNORE INTO av_news_articles
			 (url, title, time_published, authors, summary, source, source_domain,
			  category_within_source, overall_sentiment_score, overall_sentiment_label, topics_json)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			a.URL, a.Title, a.TimePublished, authors, a.Summary, a.Source, a.SourceDomain,
			a.CategoryWithinSource, a.OverallSentimentScore, a.OverallSentimentLabel, string(topicsJSON),
		)
		if err != nil {
			return added, skipped, tsRows, fmt.Errorf("upsert article: %w", err)
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			added++
			// FTS5: insert title + summary + topics so news search can match.
			topicsText := ""
			for _, t := range a.Topics {
				if v, ok := t["topic"].(string); ok {
					topicsText += " " + v
				}
			}
			_, ferr := tx.Exec(
				`INSERT INTO av_news_fts (url, title, summary, topics) VALUES (?, ?, ?, ?)`,
				a.URL, a.Title, a.Summary, topicsText,
			)
			if ferr != nil {
				// FTS failures are non-fatal — the article is still in the table.
				_ = ferr
			}
		} else {
			skipped++
		}

		// Persist ticker_sentiment rows even when the article was a duplicate —
		// new tickers may surface on a re-fetch.
		for _, ts := range a.TickerSentiment {
			rel := parseAVFloat(ts.RelevanceScore)
			sc := parseAVFloat(ts.TickerSentimentScore)
			tsRes, err := tx.Exec(
				`INSERT OR IGNORE INTO av_ticker_sentiment
				 (url, ticker, relevance_score, ticker_sentiment_score, ticker_sentiment_label, time_published)
				 VALUES (?, ?, ?, ?, ?, ?)`,
				a.URL, ts.Ticker, rel, sc, ts.TickerSentimentLabel, a.TimePublished,
			)
			if err == nil {
				if n, _ := tsRes.RowsAffected(); n > 0 {
					tsRows++
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return added, skipped, tsRows, fmt.Errorf("commit: %w", err)
	}
	return added, skipped, tsRows, nil
}

// parseAVFloat is a forgiving string-to-float converter. AV emits sentiment
// fields as JSON strings, not numbers. Returns 0 on parse failure rather than
// erroring — the row still has useful fields even if one score is malformed.
func parseAVFloat(s string) float64 {
	var f float64
	_, _ = fmt.Sscanf(s, "%f", &f)
	return f
}
