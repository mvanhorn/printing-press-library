// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// movers brief — TOP_GAINERS_LOSERS snapshot, optionally enriched with the
// 7-day mean sentiment from the local store for each ticker.
//
// One AV call per invocation. Snapshots persist to av_movers_snapshots so
// daily diff queries can answer "who moved into/out of the top 20 today".

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

type moversBriefResult struct {
	SnapshotDate string             `json:"snapshot_date"`
	Side         string             `json:"side"`
	Results      []moversBriefEntry `json:"results"`
	APICallsUsed int                `json:"api_calls_used"`
}

type moversBriefEntry struct {
	Ticker                  string   `json:"ticker"`
	Price                   string   `json:"price"`
	ChangeAmount            string   `json:"change_amount,omitempty"`
	ChangePercentage        string   `json:"change_percentage"`
	Volume                  string   `json:"volume,omitempty"`
	Sentiment7DZScore       *float64 `json:"sentiment_7d_z_score,omitempty"`
	Sentiment7DArticleCount *int     `json:"sentiment_7d_article_count,omitempty"`
}

func newMoversBriefCmd(flags *rootFlags) *cobra.Command {
	var flagSide string
	var flagEnrich string

	cmd := &cobra.Command{
		Use:         "brief",
		Short:       "Today's top movers with optional local sentiment overlay",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Run TOP_GAINERS_LOSERS once and emit a compact list of movers for one side
(gainers / losers / active). Persists the snapshot to av_movers_snapshots so
follow-on queries can compute daily diffs.

When --enrich sentiment is passed, each row is joined against
av_ticker_sentiment over the trailing 7 days and the per-ticker z-score is
attached so an agent can distinguish "top gainer with strong positive
sentiment" from "top gainer with sentiment-free move".

Cost: 1 AV call.`,
		Example: strings.Trim(`
  alphavantage-pp-cli movers brief --side gainers --json
  alphavantage-pp-cli movers brief --side gainers --enrich sentiment --json
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

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Get("/query", map[string]string{"function": "TOP_GAINERS_LOSERS"})
			if err != nil {
				logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", false, err.Error())
				return classifyAPIError(err, flags)
			}
			logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", true, "")

			today := time.Now().UTC().Format("2006-01-02")
			side := strings.ToLower(flagSide)
			if side == "" {
				side = "gainers"
			}

			entries, err := extractMoversSide(data, side)
			if err != nil {
				return err
			}

			// Persist the snapshot rows so diff queries can run later.
			persistMoversSnapshot(cmd, db, today, side, entries)

			result := moversBriefResult{
				SnapshotDate: today,
				Side:         side,
				Results:      entries,
				APICallsUsed: 1,
			}

			if strings.EqualFold(flagEnrich, "sentiment") {
				enrichMoversWithSentiment(cmd, db, result.Results)
			}

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagSide, "side", "gainers", "Which side: gainers | losers | active")
	cmd.Flags().StringVar(&flagEnrich, "enrich", "", "Enrichment: sentiment (joins local av_ticker_sentiment)")
	return cmd
}

// extractMoversSide pulls one of the three arrays out of the TOP_GAINERS_LOSERS
// response: "top_gainers", "top_losers", or "most_actively_traded".
func extractMoversSide(data json.RawMessage, side string) ([]moversBriefEntry, error) {
	var resp map[string]json.RawMessage
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing TOP_GAINERS_LOSERS: %w", err)
	}
	var key string
	switch side {
	case "gainers":
		key = "top_gainers"
	case "losers":
		key = "top_losers"
	case "active":
		key = "most_actively_traded"
	default:
		return nil, fmt.Errorf("invalid --side %q: use gainers, losers, or active", side)
	}
	raw, ok := resp[key]
	if !ok {
		return nil, fmt.Errorf("TOP_GAINERS_LOSERS response missing key %q", key)
	}
	var rows []map[string]any
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, fmt.Errorf("parsing %q array: %w", key, err)
	}
	entries := make([]moversBriefEntry, 0, len(rows))
	for _, r := range rows {
		e := moversBriefEntry{
			Ticker:           stringField(r, "ticker"),
			Price:            stringField(r, "price"),
			ChangeAmount:     stringField(r, "change_amount"),
			ChangePercentage: stringField(r, "change_percentage"),
			Volume:           stringField(r, "volume"),
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// stringField returns the value at key as a string, or "" if absent or nil.
// Avoids the "<nil>" rendering that fmt.Sprintf("%v", nil) produces when the
// API response is missing a column for a given row.
func stringField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// persistMoversSnapshot writes a snapshot row per ticker so diff queries can
// run later. Failures are silently swallowed — movers brief still succeeds
// even if the snapshot can't be persisted.
//
// Numeric fields (price, change_amount, volume) arrive from the AV response
// as strings ("213.94", "+2.81", "10342000"). We parse them into the REAL /
// INTEGER columns of av_movers_snapshots; un-parsable values fall back to
// SQL NULL rather than 0 so downstream readers can distinguish "missing" from
// "actually zero".
func persistMoversSnapshot(cmd *cobra.Command, db *store.Store, date, side string, entries []moversBriefEntry) {
	tx, err := db.DB().Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	for i, e := range entries {
		_, _ = tx.Exec(
			`INSERT OR REPLACE INTO av_movers_snapshots
			 (snapshot_date, side, ticker, price, change_amount, change_percentage, volume, rank)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			date, side, e.Ticker,
			nullableFloat(e.Price),
			nullableFloat(e.ChangeAmount),
			e.ChangePercentage,
			nullableInt(e.Volume),
			i+1,
		)
	}
	_ = tx.Commit()
}

// nullableFloat parses a numeric string into a float64 or returns nil when
// the value is empty or unparseable. AV gainer/loser rows ship prices as
// plain decimal strings; change_amount is signed ("+2.81", "-1.04") and
// strconv.ParseFloat handles both.
func nullableFloat(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Strip a leading "+" because strconv.ParseFloat accepts "-" but rejects "+"
	// in Go < 1.13 modes that match AV's signed strings.
	s = strings.TrimPrefix(s, "+")
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return v
}

// nullableInt parses a numeric string into an int64 or returns nil. Used for
// the volume column where the AV response always emits a plain integer string
// but we still want NULL on a parse miss rather than a silent 0.
func nullableInt(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return v
}

// enrichMoversWithSentiment joins each row against av_ticker_sentiment over
// the trailing 7 days and attaches a z-score (computed across the population
// of tickers in this snapshot). Mutates result rows in place.
func enrichMoversWithSentiment(cmd *cobra.Command, db *store.Store, rows []moversBriefEntry) {
	if len(rows) == 0 {
		return
	}
	// Collect mean sentiment + count per ticker.
	meanByTicker := make(map[string]float64, len(rows))
	countByTicker := make(map[string]int, len(rows))
	for _, r := range rows {
		var m float64
		var c int
		err := db.DB().QueryRowContext(cmd.Context(),
			`SELECT COALESCE(AVG(ticker_sentiment_score), 0), COUNT(*)
			 FROM av_ticker_sentiment
			 WHERE ticker = ?
			   AND substr(time_published, 1, 8) >= strftime('%Y%m%d', 'now', '-7 days')`,
			r.Ticker,
		).Scan(&m, &c)
		if err == nil {
			meanByTicker[r.Ticker] = m
			countByTicker[r.Ticker] = c
		}
	}

	// Compute z-scores across only tickers that have at least 1 article.
	var values []float64
	for _, r := range rows {
		if countByTicker[r.Ticker] > 0 {
			values = append(values, meanByTicker[r.Ticker])
		}
	}
	mean := 0.0
	stddev := 0.0
	if len(values) > 0 {
		for _, v := range values {
			mean += v
		}
		mean /= float64(len(values))
		for _, v := range values {
			d := v - mean
			stddev += d * d
		}
		stddev = math.Sqrt(stddev / float64(len(values)))
	}

	for i := range rows {
		t := rows[i].Ticker
		if countByTicker[t] > 0 {
			cnt := countByTicker[t]
			rows[i].Sentiment7DArticleCount = &cnt
			z := 0.0
			if stddev > 0 {
				z = (meanByTicker[t] - mean) / stddev
			}
			rows[i].Sentiment7DZScore = &z
		}
	}
}
