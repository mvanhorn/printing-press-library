// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// news timeline — day-by-day mean sentiment and article count for a ticker,
// computed from the local store. Zero API calls.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type newsTimelineResult struct {
	Ticker     string                `json:"ticker"`
	WindowDays int                   `json:"window_days"`
	ByTopic    bool                  `json:"by_topic,omitempty"`
	Points     []newsTimelinePoint   `json:"points"`
	TopicAgg   []newsTimelineByTopic `json:"topic_breakdown,omitempty"`
}

type newsTimelinePoint struct {
	Day           string  `json:"day"`
	MeanSentiment float64 `json:"mean_sentiment"`
	ArticleCount  int     `json:"article_count"`
}

type newsTimelineByTopic struct {
	Topic         string  `json:"topic"`
	MeanSentiment float64 `json:"mean_sentiment"`
	ArticleCount  int     `json:"article_count"`
}

func newNewsTimelineCmd(flags *rootFlags) *cobra.Command {
	var flagDays int
	var flagByTopic bool

	cmd := &cobra.Command{
		Use:         "timeline <symbol>",
		Short:       "Day-by-day mean sentiment for a ticker (local SQL, no API call)",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Aggregate the local av_ticker_sentiment table into a day-by-day timeline
of mean sentiment and article count for a single ticker. Sentiment data must
be present locally — populate it with 'news sweep' or 'news sentiment' first.

This command makes zero API calls.

Empty results are valid: a ticker with no articles in the window returns an
empty points array, not an error.`,
		Example: strings.Trim(`
  alphavantage-pp-cli news timeline NVDA --days 30 --json
  alphavantage-pp-cli news timeline AAPL --days 7 --by-topic --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ticker := strings.ToUpper(args[0])
			days := flagDays
			if days <= 0 {
				days = 30
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			result := newsTimelineResult{
				Ticker:     ticker,
				WindowDays: days,
				ByTopic:    flagByTopic,
				Points:     []newsTimelinePoint{},
			}

			// AV time_published is "20260513T120000". Pull the leading 8 chars
			// as YYYYMMDD and group on it. Compare against a YYYYMMDD cutoff
			// computed by SQLite to avoid Go/SQLite timezone drift.
			pointsQuery := `
				SELECT substr(time_published, 1, 8) AS day,
				       AVG(ticker_sentiment_score) AS mean_sentiment,
				       COUNT(*) AS article_count
				FROM av_ticker_sentiment
				WHERE ticker = ?
				  AND substr(time_published, 1, 8) >= strftime('%Y%m%d', 'now', ?)
				GROUP BY day
				ORDER BY day`
			rows, err := db.DB().QueryContext(cmd.Context(), pointsQuery, ticker, fmt.Sprintf("-%d days", days))
			if err != nil {
				return fmt.Errorf("querying timeline: %w", err)
			}
			defer rows.Close()
			for rows.Next() {
				var p newsTimelinePoint
				if err := rows.Scan(&p.Day, &p.MeanSentiment, &p.ArticleCount); err == nil {
					result.Points = append(result.Points, p)
				}
			}

			if flagByTopic {
				// Second pass: aggregate by topic via json_each over the
				// articles' topics_json column. Joining articles → ticker_sentiment
				// gives us per-topic per-ticker sentiment.
				topicQuery := `
					SELECT je.value AS topic,
					       AVG(ts.ticker_sentiment_score) AS mean_sentiment,
					       COUNT(*) AS article_count
					FROM av_ticker_sentiment ts
					JOIN av_news_articles a ON a.url = ts.url
					LEFT JOIN json_each(a.topics_json) topics_arr
					LEFT JOIN json_each(topics_arr.value) je ON je.key = 'topic'
					WHERE ts.ticker = ?
					  AND substr(ts.time_published, 1, 8) >= strftime('%Y%m%d', 'now', ?)
					  AND topic IS NOT NULL
					GROUP BY topic
					ORDER BY article_count DESC`
				trows, err := db.DB().QueryContext(cmd.Context(), topicQuery, ticker, fmt.Sprintf("-%d days", days))
				if err == nil {
					defer trows.Close()
					for trows.Next() {
						var t newsTimelineByTopic
						if err := trows.Scan(&t.Topic, &t.MeanSentiment, &t.ArticleCount); err == nil {
							result.TopicAgg = append(result.TopicAgg, t)
						}
					}
				}
				// If the topic query fails (e.g. json_each not available on this
				// SQLite), we just emit empty topic_breakdown — the timeline points
				// are still useful.
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().IntVar(&flagDays, "days", 30, "Window in days (default 30)")
	cmd.Flags().BoolVar(&flagByTopic, "by-topic", false, "Also include a per-topic breakdown")
	return cmd
}
