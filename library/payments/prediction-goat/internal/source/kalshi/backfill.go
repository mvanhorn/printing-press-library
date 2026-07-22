package kalshi

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// DefaultBackfillMinVolume is the volume floor under which the price
// backfill skips Kalshi markets. Picked so the markets users actually ask
// about (NBA series-winners, World Cup teams, headline celebrity markets)
// all clear the floor while long-tail thin markets stay one-call. Override
// via PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME.
const DefaultBackfillMinVolume = 1000.0

// BackfillReader is the subset of the local store the price backfill
// needs: read tickers and their stored payloads. Separating from
// KalshiStore (which only writes) keeps the source-package surface
// minimal.
type BackfillReader interface {
	BackfillCandidates(ctx context.Context, minVolume float64) ([]BackfillCandidate, error)
}

// BackfillCandidate is one row the backfill pass will GET /markets/{ticker}
// for. Carries the ticker and a coarse `traded` flag so the caller can
// log how many of the candidate rows actually needed enriching.
type BackfillCandidate struct {
	Ticker  string
	Untraded bool
}

// BackfillStats records the outcome of a backfill pass for the calling
// command to surface as the run summary.
type BackfillStats struct {
	Considered int
	Updated    int
	Skipped    int
	Errors     int
}

// BackfillMarketPrices walks every active Kalshi market in the local
// store above `minVolume` and re-fetches the detail payload from
// /markets/{ticker} so the cached row carries real price fields instead
// of the list-endpoint's null bid/ask. Best-effort: per-market errors
// log and continue. Pass minVolume <= 0 to backfill every active market.
func BackfillMarketPrices(ctx context.Context, c *Client, st KalshiStore, reader BackfillReader, minVolume float64) (BackfillStats, error) {
	stats := BackfillStats{}
	if reader == nil {
		return stats, fmt.Errorf("kalshi backfill: nil reader")
	}
	if st == nil {
		return stats, fmt.Errorf("kalshi backfill: nil store")
	}
	if c == nil {
		c = New()
	}
	candidates, err := reader.BackfillCandidates(ctx, minVolume)
	if err != nil {
		return stats, fmt.Errorf("kalshi backfill: list candidates: %w", err)
	}
	stats.Considered = len(candidates)
	for _, cand := range candidates {
		if err := ctx.Err(); err != nil {
			return stats, err
		}
		body, err := c.GetMarket(ctx, cand.Ticker)
		if err != nil {
			stats.Errors++
			fmt.Fprintf(os.Stderr, "kalshi backfill: %s: %v\n", cand.Ticker, err)
			continue
		}
		// Kalshi wraps single-market responses in a {"market": {...}}
		// envelope. Unwrap so the stored row matches the list payload
		// shape (object with ticker/title/yes_ask_dollars/etc at top).
		var env map[string]json.RawMessage
		if err := json.Unmarshal(body, &env); err != nil {
			stats.Errors++
			fmt.Fprintf(os.Stderr, "kalshi backfill: %s: decode envelope: %v\n", cand.Ticker, err)
			continue
		}
		raw, ok := env["market"]
		if !ok {
			raw = body
		}
		if err := st.Upsert("kalshi_markets", cand.Ticker, raw); err != nil {
			stats.Errors++
			fmt.Fprintf(os.Stderr, "kalshi backfill: %s: upsert: %v\n", cand.Ticker, err)
			continue
		}
		stats.Updated++
	}
	stats.Skipped = stats.Considered - stats.Updated - stats.Errors
	return stats, nil
}

// ResolveBackfillMinVolume reads the env override for the volume floor.
// Invalid values fall back to DefaultBackfillMinVolume so a typo in CI
// doesn't accidentally backfill every market on the platform.
func ResolveBackfillMinVolume() float64 {
	raw := os.Getenv("PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME")
	if raw == "" {
		return DefaultBackfillMinVolume
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		fmt.Fprintf(os.Stderr, "kalshi backfill: invalid PREDICTION_GOAT_KALSHI_BACKFILL_MIN_VOLUME=%q, using default %.0f\n", raw, DefaultBackfillMinVolume)
		return DefaultBackfillMinVolume
	}
	return v
}

// SQLiteBackfillReader is a default BackfillReader implementation backed
// by the standard SQLite store layout. Tests can substitute a fake.
type SQLiteBackfillReader struct {
	DB *sql.DB
}

// BackfillCandidates selects active Kalshi markets with 24h volume above
// the floor. Markets whose list payload already has prices (yes_ask_dollars
// > 0 OR last_price_dollars > 0) are still candidates so the backfill
// refreshes their state on each sync — the wire-cost is bounded by the
// volume floor regardless.
func (r SQLiteBackfillReader) BackfillCandidates(ctx context.Context, minVolume float64) ([]BackfillCandidate, error) {
	if r.DB == nil {
		return nil, fmt.Errorf("SQLiteBackfillReader: nil DB")
	}
	rows, err := r.DB.QueryContext(ctx, `SELECT id, data FROM resources
WHERE resource_type='kalshi_markets'
AND json_extract(data,'$.status') = 'active'
AND CAST(COALESCE(json_extract(data,'$.volume_fp'), json_extract(data,'$.volume')) AS REAL) >= ?
ORDER BY CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) DESC`, minVolume)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BackfillCandidate, 0)
	for rows.Next() {
		var ticker sql.NullString
		var data sql.NullString
		if err := rows.Scan(&ticker, &data); err != nil {
			return nil, err
		}
		if !ticker.Valid || ticker.String == "" {
			continue
		}
		untraded := false
		if data.Valid {
			var obj map[string]any
			if err := json.Unmarshal([]byte(data.String), &obj); err == nil {
				yesAsk, _ := obj["yes_ask_dollars"].(float64)
				noAsk, _ := obj["no_ask_dollars"].(float64)
				lastPrice, _ := obj["last_price_dollars"].(float64)
				volume24h, _ := obj["volume_24h_fp"].(float64)
				if isUntradedKalshi(yesAsk, noAsk, lastPrice, volume24h) {
					untraded = true
				}
			}
		}
		out = append(out, BackfillCandidate{Ticker: ticker.String, Untraded: untraded})
	}
	return out, rows.Err()
}

// isUntradedKalshi mirrors the cli-package helper so the source layer
// can detect untraded-default rows without importing back into cli.
func isUntradedKalshi(yesAsk, noAsk, lastPrice, volume24h float64) bool {
	if lastPrice > 0 || volume24h > 0 {
		return false
	}
	if yesAsk <= 0 && noAsk <= 0 {
		return false
	}
	return (yesAsk+noAsk)-1.0 > 0.10
}
