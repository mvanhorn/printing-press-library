// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type marketScreenItem struct {
	Source         string  `json:"source"`
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	YesProbability float64 `json:"yesProbability,omitempty"`
	YesPercent     float64 `json:"yesPercent,omitempty"`
	Volume24h      float64 `json:"volume24h,omitempty"`
	Liquidity      float64 `json:"liquidity,omitempty"`
	EndDate        string  `json:"endDate,omitempty"`
	Untraded bool `json:"untraded,omitempty"`
	// PriceSource is set after the live-on-read refresh fires: "live"
	// when the upstream venue answered, "stale" when the refresh
	// failed for the row's venue. See freshness.go.
	PriceSource string `json:"price_source,omitempty"`
}

type trendingItem = marketScreenItem
type trendingResult struct {
	Items []trendingItem `json:"items"`
	Meta  *freshnessMeta `json:"meta,omitempty"`
}

func newTrendingCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	var vf venueFlags
	cmd := &cobra.Command{
		Use:   "trending",
		Short: "Top 24h volume markets across Polymarket and Kalshi",
		Example: `  prediction-goat-pp-cli trending --json
  prediction-goat-pp-cli trending --polymarket --limit 10`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			venue, err := resolveVenue(vf)
			if err != nil {
				return err
			}
			items, err := runMarketScreen(cmd, "trending", dbPath, venue, limit, 0, "", "")
			if err != nil {
				return err
			}
			outcome := refreshMarketScreenItems(cmd.Context(), nil, items)
			meta := buildFreshnessMeta(outcome, indexSyncedAtFromPath(cmd.Context(), dbPath))
			if renderErr := renderTrending(cmd, flags, trendingResult{Items: items, Meta: meta}); renderErr != nil {
				return renderErr
			}
			if len(items) == 0 {
				if hint := emptyStoreHint(cmd, dbPath, "trending", venue); hint != nil {
					return hint
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	addVenueFlags(cmd, &vf)
	return cmd
}

func runMarketScreen(cmd *cobra.Command, screen, dbPath, venue string, limit int, value float64, text, text2 string) ([]marketScreenItem, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("prediction-goat-pp-cli")
	}
	if venue != "all" && venue != "polymarket" && venue != "kalshi" {
		return nil, fmt.Errorf("invalid --venue %q: must be all, polymarket, or kalshi", venue)
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("%s open database: %w", screen, err)
	}
	defer db.Close()
	// For venue=all, fetch per-venue ranked slices independently so a
	// heavier-volume venue cannot crowd the other out under a global
	// ORDER BY sort_value. Each side gets up to `limit` rows; we then
	// interleave them round-robin and trim to `limit`.
	if venue == "all" {
		pmItems, err := runMarketScreenOneVenue(cmd, db, screen, "polymarket", limit, value, text, text2)
		if err != nil {
			return nil, err
		}
		ksItems, err := runMarketScreenOneVenue(cmd, db, screen, "kalshi", limit, value, text, text2)
		if err != nil {
			return nil, err
		}
		return interleaveMarketScreenItems(pmItems, ksItems, limit), nil
	}
	return runMarketScreenOneVenue(cmd, db, screen, venue, limit, value, text, text2)
}

// emptyStoreHint returns a typed not-found error with a "run sync first"
// message when a market screen returns zero rows because the underlying
// resource tables are empty. Returns nil when the tables are populated
// (the empty result is then a legitimate "no matches" answer). Callers
// should render the (empty) result first and return this error last so
// agents see structured output and a non-zero exit code.
func emptyStoreHint(cmd *cobra.Command, dbPath, screen, venue string) error {
	if dbPath == "" {
		dbPath = defaultDBPath("prediction-goat-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()
	pmCount, ksCount := 0, 0
	if venue == "all" || venue == "polymarket" {
		_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM resources WHERE resource_type='markets'`).Scan(&pmCount)
	}
	if venue == "all" || venue == "kalshi" {
		_ = db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM resources WHERE resource_type='kalshi_markets'`).Scan(&ksCount)
	}
	switch venue {
	case "polymarket":
		if pmCount == 0 {
			return notFoundErr(fmt.Errorf("%s: no Polymarket markets in local store. Run `prediction-goat-pp-cli sync` first", screen))
		}
	case "kalshi":
		if ksCount == 0 {
			return notFoundErr(fmt.Errorf("%s: no Kalshi markets in local store. Run `prediction-goat-pp-cli kalshi sync` first", screen))
		}
	default:
		if pmCount == 0 && ksCount == 0 {
			return notFoundErr(fmt.Errorf("%s: local store is empty. Run `prediction-goat-pp-cli sync` and `prediction-goat-pp-cli kalshi sync` first", screen))
		}
	}
	return nil
}

func runMarketScreenOneVenue(cmd *cobra.Command, db *store.Store, screen, venue string, limit int, value float64, text, text2 string) ([]marketScreenItem, error) {
	sqlText, args := marketScreenSQL(screen, venue, limit, value, text, text2)
	rows, err := db.DB().QueryContext(cmd.Context(), sqlText, args...)
	if err != nil {
		return nil, fmt.Errorf("%s query: %w", screen, err)
	}
	defer rows.Close()
	items := make([]marketScreenItem, 0)
	for rows.Next() {
		var source, id, title, endDate sql.NullString
		var yes, volume, liquidity sql.NullFloat64
		if err := rows.Scan(&source, &id, &title, &yes, &volume, &liquidity, &endDate); err != nil {
			return nil, fmt.Errorf("%s scan: %w", screen, err)
		}
		items = append(items, marketScreenItem{Source: source.String, ID: id.String, Title: title.String, YesProbability: yes.Float64, YesPercent: yesPercent(yes.Float64), Volume24h: volume.Float64, Liquidity: liquidity.Float64, EndDate: endDate.String})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", screen, err)
	}
	if venue == "kalshi" || venue == "all" {
		enrichKalshiTitles(cmd, db, items)
	}
	return items, nil
}

// kalshiMultiLegTitleRE matches Kalshi market titles that are
// comma-concatenated YES/NO outcome legs (the Multi-Variable Event
// pattern), as opposed to natural-language event titles. Markets whose
// "title" field matches this shape get their displayed title replaced
// with the parent event's title via a lookup against kalshi_events.
var kalshiMultiLegTitleRE = regexp.MustCompile(`^(?i)(yes|no)\s.+,(yes|no)\s`)

// looksLikeMultiLegTitle returns true when the title is a CSV of YES/NO
// outcome legs rather than a human-readable event title.
func looksLikeMultiLegTitle(title string) bool {
	t := strings.TrimSpace(title)
	if t == "" {
		return false
	}
	return kalshiMultiLegTitleRE.MatchString(t)
}

// enrichKalshiTitles rewrites Kalshi market titles that are
// comma-concatenated outcome legs (the Multi-Variable Event shape) to
// a more readable form: the parent event's natural-language title when
// kalshi_events has been synced, otherwise a summarized "<first leg>
// (+N more)" form so the title stays scannable. Bulk-loads event titles
// once per result set rather than per-row.
func enrichKalshiTitles(cmd *cobra.Command, db *store.Store, items []marketScreenItem) {
	pending := make(map[string][]int)
	for i, it := range items {
		if it.Source != "kalshi" || !looksLikeMultiLegTitle(it.Title) {
			continue
		}
		eventTicker := kalshiEventTickerFromMarketID(it.ID)
		if eventTicker != "" {
			pending[eventTicker] = append(pending[eventTicker], i)
		}
	}
	resolved := make(map[int]bool)
	if len(pending) > 0 {
		placeholders := make([]string, 0, len(pending))
		args := make([]any, 0, len(pending))
		for ticker := range pending {
			placeholders = append(placeholders, "?")
			args = append(args, ticker)
		}
		query := `SELECT id, COALESCE(json_extract(data,'$.title'), json_extract(data,'$.event_title'), '') AS event_title
FROM resources WHERE resource_type='kalshi_events' AND id IN (` + strings.Join(placeholders, ",") + `)`
		rows, err := db.DB().QueryContext(cmd.Context(), query, args...)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id, eventTitle sql.NullString
				if err := rows.Scan(&id, &eventTitle); err != nil {
					break
				}
				if !id.Valid || strings.TrimSpace(eventTitle.String) == "" {
					continue
				}
				for _, idx := range pending[id.String] {
					items[idx].Title = eventTitle.String
					resolved[idx] = true
				}
			}
			// Surface mid-iteration row errors (context cancellation, WAL
			// write contention) on stderr so the fallback summary path
			// isn't silently masking real DB trouble. Best-effort
			// enrichment, so a warning is enough; the summarized
			// fallback below still runs.
			if rerr := rows.Err(); rerr != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: enrichKalshiTitles: %v\n", rerr)
			}
		}
	}
	// Fallback: any remaining multi-leg titles get summarized so they
	// stay scannable rather than dumping the full CSV of legs.
	for i, it := range items {
		if it.Source != "kalshi" || resolved[i] || !looksLikeMultiLegTitle(it.Title) {
			continue
		}
		items[i].Title = summarizeMultiLegTitle(it.Title)
	}
}

// summarizeMultiLegTitle turns "yes A,yes B,yes C,yes D" into
// "yes A (+3 more legs)". Used as a last-resort fallback when no parent
// event title is available locally.
func summarizeMultiLegTitle(title string) string {
	parts := strings.Split(title, ",")
	if len(parts) <= 1 {
		return title
	}
	first := strings.TrimSpace(parts[0])
	rest := len(parts) - 1
	if rest == 1 {
		return first + " (+1 more leg)"
	}
	return fmt.Sprintf("%s (+%d more legs)", first, rest)
}

// kalshiEventTickerFromMarketID extracts the event ticker from a Kalshi
// market ticker. Kalshi market tickers follow the convention
// `<SERIES>-<EVENT>-<MARKET>` (e.g. KXMVESPORTS-S20269-A47C); the event
// ticker is the prefix up to the final hyphen segment. Returns the
// empty string when the shape doesn't match.
func kalshiEventTickerFromMarketID(marketID string) string {
	idx := strings.LastIndex(marketID, "-")
	if idx <= 0 {
		return ""
	}
	return marketID[:idx]
}

// interleaveMarketScreenItems round-robins two ranked venue slices to keep
// both venues visible under venue=all. Returns at most `limit` rows.
func interleaveMarketScreenItems(a, b []marketScreenItem, limit int) []marketScreenItem {
	if limit <= 0 {
		return nil
	}
	out := make([]marketScreenItem, 0, limit)
	ai, bi := 0, 0
	for len(out) < limit && (ai < len(a) || bi < len(b)) {
		if ai < len(a) {
			out = append(out, a[ai])
			ai++
			if len(out) >= limit {
				break
			}
		}
		if bi < len(b) {
			out = append(out, b[bi])
			bi++
		}
	}
	return out
}

func marketScreenSQL(screen, venue string, limit int, value float64, text, text2 string) (string, []any) {
	pmVolume := `CAST(COALESCE(json_extract(data,'$.volume24hr'), json_extract(data,'$.volumeNum'), 0) AS REAL)`
	pmEnd := `COALESCE(json_extract(data,'$.endDate'), '')`
	pm := fmt.Sprintf(`SELECT 'polymarket' source, id, COALESCE(json_extract(data,'$.question'), json_extract(data,'$.title'), id) title,
CAST(COALESCE(json_extract(data,'$.lastTradePrice'),0) AS REAL) yes, %s volume,
CAST(COALESCE(json_extract(data,'$.liquidityNum'),0) AS REAL) liquidity, %s end_date, %s sort_value FROM resources WHERE resource_type='markets'`, pmVolume, pmEnd, pmVolume)
	ks := `SELECT 'kalshi' source, id, COALESCE(json_extract(data,'$.title'), id) title,
CAST(COALESCE(json_extract(data,'$.last_price_dollars'),0) AS REAL) yes,
CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) volume,
CAST(COALESCE(json_extract(data,'$.liquidity_dollars'),0) AS REAL) liquidity,
COALESCE(json_extract(data,'$.expiration_time'), json_extract(data,'$.close_time'), '') end_date,
CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) sort_value FROM resources WHERE resource_type='kalshi_markets'`
	args := make([]any, 0)
	switch screen {
	case "trending":
	case "liquid":
		// PATCH(liquid-24h-volume-symmetric): both venues filter on 24h rolling
		// volume so --min-volume has consistent meaning across the result set.
		// PM rows that lack 'volume24hr' (stale markets with no recent trading)
		// coalesce to 0 and fail the floor, which is intended behavior for a
		// "liquid markets" screen. Greptile P1 on PR #780.
		pm += " AND CAST(COALESCE(json_extract(data,'$.volume24hr'),0) AS REAL) > ?"
		ks += " AND CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) > ?"
	case "new":
		pm = strings.Replace(pm, pmVolume+" sort_value", "COALESCE(json_extract(data,'$.createdAt'),'') sort_value", 1)
		ks = strings.Replace(ks, "CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) sort_value", "COALESCE(json_extract(data,'$.open_time'),'') sort_value", 1)
		pm += " AND COALESCE(json_extract(data,'$.createdAt'),'') > ?"
		ks += " AND COALESCE(json_extract(data,'$.open_time'),'') > ?"
	case "resolving":
		pm = strings.Replace(pm, pmVolume+" sort_value", "CAST(COALESCE(json_extract(data,'$.liquidityNum'),0) AS REAL) sort_value", 1)
		ks = strings.Replace(ks, "CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) sort_value", "CAST(COALESCE(json_extract(data,'$.liquidity_dollars'),0) AS REAL) sort_value", 1)
		// json_extract returns integer 1 for JSON boolean true (and 0 for
		// false), so a string comparison against 'true' never filters
		// anything. Match both the boolean and string encodings.
		pm += " AND COALESCE(json_extract(data,'$.closed'), 0) NOT IN (1, 'true')"
		pm += " AND " + pmEnd + " BETWEEN ? AND ?"
		ks += " AND COALESCE(json_extract(data,'$.status'),'active') NOT IN ('settled','closed','resolved','finalized')"
		ks += " AND COALESCE(json_extract(data,'$.close_time'), json_extract(data,'$.expiration_time'), '') BETWEEN ? AND ?"
	}
	parts := make([]string, 0, 2)
	if venue == "all" || venue == "polymarket" {
		parts = append(parts, pm)
		if screen == "liquid" {
			args = append(args, value)
		} else if screen == "new" {
			args = append(args, text)
		} else if screen == "resolving" {
			args = append(args, text, text2)
		}
	}
	if venue == "all" || venue == "kalshi" {
		parts = append(parts, ks)
		if screen == "liquid" {
			args = append(args, value)
		} else if screen == "new" {
			args = append(args, text)
		} else if screen == "resolving" {
			args = append(args, text, text2)
		}
	}
	return "SELECT source, id, title, yes, volume, liquidity, end_date FROM (" + strings.Join(parts, " UNION ALL ") + ") ORDER BY " + screenOrder(screen) + " LIMIT ?", append(args, limit)
}

// screenOrder used to switch sort order per screen but every branch now
// returns the same expression. Keep the function for callsite clarity
// and so any future per-screen ordering lands in one place.
func screenOrder(_ string) string {
	return "sort_value DESC"
}

func renderTrending(cmd *cobra.Command, flags *rootFlags, result trendingResult) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	if err := printSimpleTable(cmd.OutOrStdout(), []string{"Source", "ID", "Title", "%Yes", "Volume24h", "Liquidity", "EndDate"}, marketScreenRows(result.Items)); err != nil {
		return err
	}
	if footer := freshnessFooterLine(result.Meta); footer != "" {
		fmt.Fprintln(cmd.OutOrStdout(), footer)
	}
	return nil
}

// refreshMarketScreenItems batches the slice by venue, fires a live
// refresh per venue, and applies the refreshed price-bearing fields
// to each item. Returns the per-venue outcome so the caller can
// populate the envelope's price_source.
func refreshMarketScreenItems(ctx context.Context, fc freshnessClient, items []marketScreenItem) refreshOutcome {
	polySlugs := make([]string, 0, len(items))
	kalshiTickers := make([]string, 0, len(items))
	for _, it := range items {
		switch it.Source {
		case "polymarket":
			polySlugs = append(polySlugs, it.ID)
		case "kalshi":
			kalshiTickers = append(kalshiTickers, it.ID)
		}
	}
	outcome := refreshVenues(ctx, fc, polySlugs, kalshiTickers)
	var dummyStatus string
	for i := range items {
		switch items[i].Source {
		case "polymarket":
			if !outcome.PolymarketAsked {
				continue
			}
			if !outcome.PolymarketOK {
				items[i].PriceSource = priceSourceStale
				continue
			}
			if v, ok := outcome.Polymarket[items[i].ID]; ok {
				applyLiveValuesIfPresent(v, &items[i].YesProbability, &items[i].Volume24h, &dummyStatus)
				// PATCH(refresh-market-screen-items-yespercent): keep
				// the derived YesPercent field in sync with the just-
				// refreshed YesProbability so agent-facing JSON never
				// shows yesProbability=0.60 alongside yesPercent=55.0
				// (the pre-refresh value). Mirrors compare.go and
				// mispriced.go post-refresh recomputes. Greptile P1 on
				// PR #780.
				items[i].YesPercent = yesPercent(items[i].YesProbability)
			}
			items[i].PriceSource = priceSourceLive
		case "kalshi":
			if !outcome.KalshiAsked {
				continue
			}
			if !outcome.KalshiOK {
				items[i].PriceSource = priceSourceStale
				continue
			}
			if v, ok := outcome.Kalshi[items[i].ID]; ok {
				applyLiveValuesIfPresent(v, &items[i].YesProbability, &items[i].Volume24h, &dummyStatus)
				// PATCH(refresh-market-screen-items-yespercent): mirrors
				// the Polymarket branch above; recompute the derived
				// YesPercent so it stays in sync with the refreshed
				// YesProbability after the live-on-read pass.
				items[i].YesPercent = yesPercent(items[i].YesProbability)
			}
			items[i].PriceSource = priceSourceLive
		}
	}
	return outcome
}

func marketScreenRows(items []marketScreenItem) [][]string {
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		rows = append(rows, []string{it.Source, it.ID, it.Title, formatProb(it.YesProbability), formatNumber(it.Volume24h), formatNumber(it.Liquidity), it.EndDate})
	}
	return rows
}

func formatProb(v float64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

// yesPercent returns the 0-100 percent companion of a 0-1 yesProbability,
// rounded to one decimal place. JSON output carries both fields:
// `yesProbability` is the canonical machine representation; `yesPercent`
// is the apples-to-apples display value agents can surface directly. A
// zero input yields zero so the omitempty tag suppresses it in JSON, the
// same convention formatProb uses for human-readable text.
func yesPercent(v float64) float64 {
	if v == 0 {
		return 0
	}
	rounded := float64(int(v*1000+0.5)) / 10
	return rounded
}

// isUntradedKalshi flags Kalshi markets whose displayed YES ask is the
// platform-default 17c rather than a real implied probability. The three
// signals together identify untraded markets: no last trade, no 24h
// volume, and a YES ask + NO ask that overshoots $1.00 by more than 10c
// (a tight book sums to roughly $1.00 plus the maker fee). Polymarket
// markets without volume are not flagged here; the threshold logic only
// fires on the Kalshi-specific default-ask pattern.
func isUntradedKalshi(yesAsk, noAsk, lastPrice, volume24h float64) bool {
	if lastPrice > 0 || volume24h > 0 {
		return false
	}
	if yesAsk <= 0 && noAsk <= 0 {
		return false
	}
	return (yesAsk+noAsk)-1.0 > 0.10
}

func formatNumber(v float64) string {
	if v == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f", v)
}
