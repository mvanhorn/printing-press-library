// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// briefing earnings — multi-call orchestration that composes a single
// pre-earnings card from EARNINGS_CALENDAR + EARNINGS + NEWS_SENTIMENT +
// GLOBAL_QUOTE. Uses local cache where fresh; skips endpoints that hit a
// rate-limit error and emits a partial-result warning rather than failing.
//
// Up to 4 AV calls per invocation.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type briefingEarningsResult struct {
	Symbol               string   `json:"symbol"`
	UpcomingEarningsDate string   `json:"upcoming_earnings_date,omitempty"`
	UpcomingEstimate     string   `json:"upcoming_estimate,omitempty"`
	RecentArticlesCount  int      `json:"recent_articles_count"`
	Sentiment7DMean      float64  `json:"sentiment_7d_mean"`
	LastEPSReported      string   `json:"last_eps_reported,omitempty"`
	LastEPSEstimated     string   `json:"last_eps_estimated,omitempty"`
	LastReportedDate     string   `json:"last_reported_date,omitempty"`
	CurrentQuote         any      `json:"current_quote,omitempty"`
	APICallsAttempted    int      `json:"api_calls_attempted"`
	APICallsSucceeded    int      `json:"api_calls_succeeded"`
	Warnings             []string `json:"warnings,omitempty"`
}

func newBriefingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "briefing",
		Short: "Pre-event multi-call briefings (earnings, etc.)",
		Long: `Composite briefings that orchestrate multiple Alpha Vantage endpoints
into a single card. Designed to replace 4-5 separate API calls with one
agent-friendly invocation.`,
	}
	cmd.AddCommand(newBriefingEarningsCmd(flags))
	return cmd
}

func newBriefingEarningsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "earnings <symbol>",
		Short:       "Pre-earnings card: upcoming date + recent news + last EPS + quote",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `One-shot pre-earnings card composed from up to 4 AV endpoints:
  1. EARNINGS_CALENDAR  (horizon 3month, filtered to symbol)
  2. EARNINGS           (last reported quarter)
  3. NEWS_SENTIMENT     (topic=earnings, ticker=symbol, limit=10)
  4. GLOBAL_QUOTE       (current price)

GLOBAL_QUOTE is cached at the client level for 5 minutes, so repeated calls
within the same session don't burn extra quota. Endpoints that hit a
rate-limit error are skipped with a partial-result warning rather than
failing the whole briefing.`,
		Example: strings.Trim(`
  alphavantage-pp-cli briefing earnings AAPL --json
  alphavantage-pp-cli briefing earnings NVDA --json --select symbol,upcoming_earnings_date,sentiment_7d_mean
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			symbol := strings.ToUpper(args[0])

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := briefingEarningsResult{Symbol: symbol}

			// 1. EARNINGS_CALENDAR (CSV)
			result.APICallsAttempted++
			calData, ecErr := c.Get("/query", map[string]string{
				"function": "EARNINGS_CALENDAR",
				"symbol":   symbol,
				"horizon":  "3month",
			})
			if ecErr != nil {
				logQuotaCall(cmd, db, "EARNINGS_CALENDAR", symbol, false, ecErr.Error())
				result.Warnings = append(result.Warnings, fmt.Sprintf("EARNINGS_CALENDAR skipped: %v", trimErrorMessage(ecErr)))
			} else {
				logQuotaCall(cmd, db, "EARNINGS_CALENDAR", symbol, true, "")
				result.APICallsSucceeded++
				if date, est := extractFirstEarningsRow(calData); date != "" {
					result.UpcomingEarningsDate = date
					result.UpcomingEstimate = est
				}
			}

			// 2. EARNINGS (historical)
			result.APICallsAttempted++
			earnData, eErr := c.Get("/query", map[string]string{
				"function": "EARNINGS",
				"symbol":   symbol,
			})
			if eErr != nil {
				logQuotaCall(cmd, db, "EARNINGS", symbol, false, eErr.Error())
				result.Warnings = append(result.Warnings, fmt.Sprintf("EARNINGS skipped: %v", trimErrorMessage(eErr)))
			} else {
				logQuotaCall(cmd, db, "EARNINGS", symbol, true, "")
				result.APICallsSucceeded++
				if date, rep, estVal := extractLastQuarterlyEarnings(earnData); date != "" {
					result.LastReportedDate = date
					result.LastEPSReported = rep
					result.LastEPSEstimated = estVal
				}
			}

			// 3. NEWS_SENTIMENT (earnings topic) — persist via same path as sweep
			//    so news timeline/search/screen pick it up.
			result.APICallsAttempted++
			newsData, nErr := c.Get("/query", map[string]string{
				"function": "NEWS_SENTIMENT",
				"tickers":  symbol,
				"topics":   "earnings",
				"limit":    "10",
			})
			if nErr != nil {
				logQuotaCall(cmd, db, "NEWS_SENTIMENT", symbol, false, nErr.Error())
				result.Warnings = append(result.Warnings, fmt.Sprintf("NEWS_SENTIMENT skipped: %v", trimErrorMessage(nErr)))
			} else {
				logQuotaCall(cmd, db, "NEWS_SENTIMENT", symbol, true, "")
				result.APICallsSucceeded++
				added, _, _, perr := persistNewsResponse(db, newsData)
				if perr == nil {
					result.RecentArticlesCount = added
				}
				// Compute 7d mean sentiment for this ticker from the local store.
				if mean, ok := computeSentimentMean(cmd, db, symbol, 7); ok {
					result.Sentiment7DMean = mean
				}
			}

			// 4. GLOBAL_QUOTE
			result.APICallsAttempted++
			quoteData, qErr := c.Get("/query", map[string]string{
				"function": "GLOBAL_QUOTE",
				"symbol":   symbol,
			})
			if qErr != nil {
				logQuotaCall(cmd, db, "GLOBAL_QUOTE", symbol, false, qErr.Error())
				result.Warnings = append(result.Warnings, fmt.Sprintf("GLOBAL_QUOTE skipped: %v", trimErrorMessage(qErr)))
			} else {
				logQuotaCall(cmd, db, "GLOBAL_QUOTE", symbol, true, "")
				result.APICallsSucceeded++
				var quoteWrapper map[string]json.RawMessage
				if json.Unmarshal(quoteData, &quoteWrapper) == nil {
					if raw, ok := quoteWrapper["Global Quote"]; ok {
						var gq map[string]any
						if json.Unmarshal(raw, &gq) == nil {
							result.CurrentQuote = gq
						}
					}
				}
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// extractFirstEarningsRow parses the EARNINGS_CALENDAR CSV body and returns
// the first (symbol, name, reportDate, fiscalDateEnding, estimate, currency)
// row's reportDate + estimate. Returns empty strings on miss.
func extractFirstEarningsRow(data json.RawMessage) (string, string) {
	// EARNINGS_CALENDAR returns CSV (not JSON). The body may arrive as a JSON
	// string or as raw text — handle both.
	body := string(data)
	if len(body) > 0 && body[0] == '"' {
		var unquoted string
		if err := json.Unmarshal(data, &unquoted); err == nil {
			body = unquoted
		}
	}
	// AV soft-failures arrive as 200-with-CSV bodies that the csv parser would
	// happily decompose into single-character cells (e.g. "Error Message: ..."
	// becomes ["E","r","r","o","r","M",...]). The client middleware catches
	// JSON-shaped soft-failures upstream, but CSV variants slip through; sniff
	// the body before parsing so callers see "" instead of a fake "r" date.
	lower := strings.ToLower(body)
	if strings.HasPrefix(strings.TrimSpace(lower), "error message") ||
		strings.HasPrefix(strings.TrimSpace(lower), "information") ||
		strings.HasPrefix(strings.TrimSpace(lower), "note") {
		return "", ""
	}
	reader := csv.NewReader(strings.NewReader(body))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return "", ""
	}
	// Header: symbol,name,reportDate,fiscalDateEnding,estimate,currency
	header := rows[0]
	dateIdx, estIdx := -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "reportdate":
			dateIdx = i
		case "estimate":
			estIdx = i
		}
	}
	if dateIdx < 0 {
		return "", ""
	}
	row := rows[1]
	if dateIdx >= len(row) {
		return "", ""
	}
	date := row[dateIdx]
	// Sanity: reportDate must look like YYYY-MM-DD; if not, the CSV is
	// almost certainly an error payload masquerading as data.
	if !looksLikeISODate(date) {
		return "", ""
	}
	est := ""
	if estIdx >= 0 && estIdx < len(row) {
		est = row[estIdx]
	}
	return date, est
}

// extractLastQuarterlyEarnings reads the EARNINGS JSON and returns the most
// recent quarterly row's (date, reportedEPS, estimatedEPS).
func extractLastQuarterlyEarnings(data json.RawMessage) (string, string, string) {
	var resp struct {
		Quarterly []map[string]any `json:"quarterlyEarnings"`
	}
	if err := json.Unmarshal(data, &resp); err != nil || len(resp.Quarterly) == 0 {
		return "", "", ""
	}
	q := resp.Quarterly[0] // AV emits newest first
	date := fmt.Sprintf("%v", q["reportedDate"])
	rep := fmt.Sprintf("%v", q["reportedEPS"])
	est := fmt.Sprintf("%v", q["estimatedEPS"])
	if date == "<nil>" {
		date = ""
	}
	if rep == "<nil>" {
		rep = ""
	}
	if est == "<nil>" {
		est = ""
	}
	return date, rep, est
}

// computeSentimentMean averages av_ticker_sentiment.ticker_sentiment_score
// for a single ticker over the last N days. Returns (0, false) when no rows.
func computeSentimentMean(cmd *cobra.Command, db *store.Store, ticker string, days int) (float64, bool) {
	var mean float64
	var count int
	err := db.DB().QueryRowContext(cmd.Context(),
		`SELECT COALESCE(AVG(ticker_sentiment_score), 0), COUNT(*)
		 FROM av_ticker_sentiment
		 WHERE ticker = ?
		   AND substr(time_published, 1, 8) >= strftime('%Y%m%d', 'now', ?)`,
		ticker, fmt.Sprintf("-%d days", days),
	).Scan(&mean, &count)
	if err != nil || count == 0 {
		return 0, false
	}
	return mean, true
}

// trimErrorMessage shortens long error messages so they fit in the briefing
// warnings array without leaking 200-char API error bodies. Also unwraps
// *client.APIError so the truncated AV message replaces the verbose
// "method path returned HTTP N: ..." prefix.
func trimErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		// apiErr.Body is the truncated AV note/info/error message.
		msg := apiErr.Body
		if len(msg) > 120 {
			msg = msg[:120] + "..."
		}
		return msg
	}
	msg := err.Error()
	if len(msg) > 120 {
		msg = msg[:120] + "..."
	}
	return msg
}

// looksLikeISODate returns true if s matches YYYY-MM-DD shape (10 chars,
// digits at positions 0-3,5-6,8-9, dashes at 4 and 7). Cheaper than a regex
// for a hot path that only needs structural validation.
func looksLikeISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range s {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
