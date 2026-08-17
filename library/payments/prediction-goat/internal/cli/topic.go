// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/polymarket"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type topicHit struct {
	Source         string  `json:"source"`
	Kind           string  `json:"kind"`
	ID             string  `json:"id"`
	Title          string  `json:"title"`
	Status         string  `json:"status,omitempty"`
	YesProbability float64 `json:"yesProbability,omitempty"`
	YesPercent     float64 `json:"yesPercent,omitempty"`
	Volume24h      float64 `json:"volume24h,omitempty"`
	EndDate        string  `json:"endDate,omitempty"`
	URL            string  `json:"url,omitempty"`
	Untraded       bool    `json:"untraded,omitempty"`
	ExpandedFrom   string  `json:"expandedFrom,omitempty"`
	// PriceSource is set when the row carries a price/probability that
	// went through the live refresh pipeline ("live" or "stale"). Rows
	// with no price-bearing fields (tags, series) leave this empty so
	// agents can tell a tag-row apart from a price-row whose refresh
	// genuinely failed.
	PriceSource string `json:"price_source,omitempty"`
	// rankScore is the per-hit ranking score (BM25 plus volume weighting)
	// used by the cross-venue re-rank. Not emitted in JSON output.
	rankScore float64
}

type topicResult struct {
	Topic     string         `json:"topic"`
	Count     int            `json:"count"`
	Truncated bool           `json:"truncated,omitempty"`
	Hits      []topicHit     `json:"hits"`
	Meta      *freshnessMeta `json:"meta,omitempty"`
}

// topicSearchWindow is the over-fetch multiplier the vol-weighted re-rank
// uses. The SQL LIMIT pulls min(window*limit, maxWindow) rows by BM25; the
// Go re-rank then sorts them by a weighted combination of BM25 rank and
// log-volume so high-volume series-winner markets (e.g. KXNBAWEST at $29M)
// surface above unrelated FTS matches with higher term frequency.
const (
	topicSearchWindow    = 4
	topicSearchMaxWindow = 200
	topicVolWeight       = 0.20
)

func newTopicCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var dbPath string
	var activeOnly bool
	var withPrices bool
	var expand bool
	var vf venueFlags
	cmd := &cobra.Command{
		Use:   "topic <name>",
		Short: "Cross-venue topic bundle (slim ranked markets/events/tags from Polymarket and Kalshi)",
		Example: `  prediction-goat-pp-cli topic kanye --json
  prediction-goat-pp-cli topic 'arizona basketball' --limit 20 --with-prices
  prediction-goat-pp-cli topic 'world cup' --kalshi   # skip Polymarket
  prediction-goat-pp-cli topic 'world cup' --polymarket --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			venue, err := resolveVenue(vf)
			if err != nil {
				return err
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("topic open database: %w", err)
			}
			defer db.Close()
			topic := strings.Join(args, " ")
			searchLimit := topicSearchWindow * limit
			if searchLimit > topicSearchMaxWindow {
				searchLimit = topicSearchMaxWindow
			}
			if searchLimit < limit {
				searchLimit = limit
			}
			polyTypes := []string{"markets", "events", "tags"}
			kalshiTypes := []string{"kalshi_markets", "kalshi_events", "kalshi_series"}
			var polyHits, kalshiHits []topicHit
			if venue == "all" || venue == "polymarket" {
				polyHits, err = topicSearchByTypes(cmd.Context(), db.DB(), topicFTSQuery(topic), polyTypes, searchLimit)
				if err != nil {
					return fmt.Errorf("topic search polymarket: %w", err)
				}
			}
			if venue == "all" || venue == "kalshi" {
				kalshiHits, err = topicSearchByTypes(cmd.Context(), db.DB(), topicFTSQuery(topic), kalshiTypes, searchLimit)
				if err != nil {
					return fmt.Errorf("topic search kalshi: %w", err)
				}
			}
			// Vol-weighted re-rank: each side already arrives BM25-sorted
			// (lower-rank-better in SQLite FTS5). Re-score by combining
			// BM25-position with log-volume so high-volume mainline markets
			// surface above thin BM25-favored siblings.
			sortHitsByScore(polyHits)
			sortHitsByScore(kalshiHits)
			if activeOnly {
				kalshiHits = filterActiveOnly(cmd.Context(), db.DB(), kalshiHits)
				polyHits = filterPolyActiveOnly(polyHits)
			}
			truncated := len(polyHits)+len(kalshiHits) > limit
			results := interleaveTopicHits(polyHits, kalshiHits, limit)
			// Force-include: if a query token names an outcome that did
			// not make the truncated set, append it. This catches multi-
			// outcome events where one participant's market gets buried by
			// the BM25 + volume rerank (e.g. USA in a 48-team World Cup).
			results = forceIncludeNamedOutcomes(results, polyHits, kalshiHits, topicQueryTokens(topic), limit)
			if withPrices {
				results = expandWithPrices(cmd.Context(), db.DB(), results)
			}
			if expand {
				results = expandPolymarketSiblings(cmd, results, limit)
			}
			// Live-on-read freshness: refresh price-bearing fields from
			// the upstream APIs so the cached discovery index never serves
			// stale prices. Runs AFTER expand/with-prices so synthetic rows
			// inserted by those steps also get fresh values. See freshness.go.
			outcome := refreshTopicHits(cmd.Context(), nil, results)
			// Rerank layer: apply taught learnings AFTER freshness refresh so
			// boost/hide/alias act on the live prices. See teach.go for the
			// LLM contract.
			var applied int
			var hasHigh bool
			if !noLearnActive(flags) {
				results, applied, hasHigh = applyLearningsForTopic(cmd.Context(), db, topic, results)
			}
			for i := range results {
				results[i].YesPercent = yesPercent(results[i].YesProbability)
			}
			meta := buildFreshnessMeta(outcome, indexSyncedAt(db))
			if meta != nil {
				meta.LearningsApplied = applied
				meta.TeachHint = teachHintFor(topic, applied, hasHigh, len(results))
			}
			result := topicResult{Topic: topic, Count: len(results), Truncated: truncated, Hits: results, Meta: meta}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), result, flags); err != nil {
					return err
				}
			} else {
				if err := printSimpleTable(cmd.OutOrStdout(), []string{"Source", "Kind", "Title", "%Yes", "Volume24h", "EndDate"}, topicRows(results)); err != nil {
					return err
				}
				if footer := freshnessFooterLine(meta); footer != "" {
					fmt.Fprintln(cmd.OutOrStdout(), footer)
				}
			}
			if len(results) == 0 {
				return notFoundErr(fmt.Errorf("no markets, events, or tags matched topic %q (try a broader query, or run `prediction-goat-pp-cli sync` and `kalshi sync` first)", topic))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "Max results")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().BoolVar(&activeOnly, "active-only", true, "Suppress series whose events are all closed and Polymarket markets marked closed")
	cmd.Flags().BoolVar(&withPrices, "with-prices", false, "Resolve series shells to the top active market under them so prices appear inline")
	cmd.Flags().BoolVar(&expand, "expand", true, "Walk Polymarket multi-outcome event families so siblings (e.g. all World Cup teams) surface from a single seed market")
	addVenueFlags(cmd, &vf)
	return cmd
}

// expandPolymarketSiblings walks any Polymarket market hits whose parent
// event has additional siblings and appends those siblings to the result
// set. Caps live API calls at 2 events per topic call so the cost stays
// bounded even when many seed markets hit the same multi-outcome family.
// Live-fetch errors fall back silently (existing results are unchanged).
func expandPolymarketSiblings(cmd *cobra.Command, hits []topicHit, limit int) []topicHit {
	const maxEventExpansions = 2
	if len(hits) == 0 {
		return hits
	}
	client := polymarket.New()
	expanded := 0
	seen := make(map[string]struct{}, len(hits)*2)
	for _, h := range hits {
		seen[h.Source+"|"+h.ID] = struct{}{}
	}
	seenEvents := make(map[string]struct{}, maxEventExpansions)
	out := append([]topicHit{}, hits...)
	for _, h := range hits {
		if expanded >= maxEventExpansions {
			break
		}
		if h.Source != "polymarket" || h.Kind != "market" {
			continue
		}
		ev, siblings, err := client.SiblingsForMarket(cmd.Context(), h.ID, false)
		if err != nil || ev.Slug == "" || len(siblings) <= 1 {
			continue
		}
		if _, dup := seenEvents[ev.Slug]; dup {
			continue
		}
		seenEvents[ev.Slug] = struct{}{}
		expanded++
		for _, sib := range siblings {
			key := "polymarket|" + sib.Slug
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, topicHit{
				Source:         "polymarket",
				Kind:           "market",
				ID:             sib.Slug,
				Title:          sib.Question,
				YesProbability: sib.YesProbability,
				Volume24h:      sib.Volume,
				EndDate:        sib.EndDate,
				URL:            sib.URL,
				ExpandedFrom:   "event:" + ev.Slug,
			})
			if len(out) >= limit*2 {
				return out
			}
		}
	}
	return out
}

// topicSearchByTypes runs an FTS5 search restricted to a fixed set of
// resource types and returns up to `limit` decoded topicHit rows. It is
// the per-venue half of the cross-venue interleave the topic command does.
func topicSearchByTypes(ctx context.Context, db *sql.DB, ftsQuery string, types []string, limit int) ([]topicHit, error) {
	if len(types) == 0 || limit <= 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(types)), ",")
	q := `SELECT r.resource_type, r.id, r.data, rank FROM resources r
JOIN resources_fts f ON r.id = f.id AND r.resource_type = f.resource_type
WHERE resources_fts MATCH ?
AND r.resource_type IN (` + placeholders + `)
ORDER BY rank LIMIT ?`
	args := make([]any, 0, len(types)+2)
	args = append(args, ftsQuery)
	for _, t := range types {
		args = append(args, t)
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	hits := make([]topicHit, 0)
	position := 0
	for rows.Next() {
		var typ, id, data sql.NullString
		var rank sql.NullFloat64
		if err := rows.Scan(&typ, &id, &data, &rank); err != nil {
			return nil, err
		}
		if !typ.Valid || !data.Valid {
			continue
		}
		hit, ok := topicHitFromJSON(typ.String, id.String, data.String)
		if ok {
			// Score = BM25 (more negative = better in FTS5) combined with
			// log(1 + volume24h). Negate BM25 so larger is better, then
			// add a volume bonus. Empty volume contributes zero so pure
			// FTS rank dominates when no volume signal exists.
			bm25 := -rank.Float64
			volBonus := 0.0
			if hit.Volume24h > 0 {
				volBonus = math.Log1p(hit.Volume24h) * topicVolWeight
			}
			hit.rankScore = bm25 + volBonus
			hits = append(hits, hit)
		}
		position++
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hits, nil
}

// sortHitsByScore orders hits with the highest rankScore first (BM25 plus
// volume bonus). Stable sort preserves SQL order on ties.
func sortHitsByScore(hits []topicHit) {
	sort.SliceStable(hits, func(i, j int) bool {
		return hits[i].rankScore > hits[j].rankScore
	})
}

// filterActiveOnly drops Kalshi series whose events are all closed or
// settled. The store joins kalshi_events to kalshi_series via the
// series_ticker JSON field, so existence of any UNRESOLVED event under
// the series ticker is the cheap inclusion check. Both event-status and
// market-status are checked: Kalshi events use status values "open"
// (event is accepting bets, listed but not started) and "settled" (event
// has resolved). Some series ship only markets without parent events;
// in that case the kalshi_markets fallback looks for any market with
// status='active'.
func filterActiveOnly(ctx context.Context, db *sql.DB, hits []topicHit) []topicHit {
	out := make([]topicHit, 0, len(hits))
	for _, h := range hits {
		if h.Source != "kalshi" || h.Kind != "series" {
			out = append(out, h)
			continue
		}
		var count int
		row := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM resources
WHERE resource_type='kalshi_events'
AND json_extract(data,'$.series_ticker') = ?
AND COALESCE(json_extract(data,'$.status'), 'open') NOT IN ('settled','closed','resolved','finalized')
LIMIT 1`, h.ID)
		if err := row.Scan(&count); err != nil || count == 0 {
			// Fall back to kalshi_markets check when no live event row
			// exists for the series. Some series ship only markets, not
			// parent events; an active market under the series ticker
			// counts as active.
			marketRow := db.QueryRowContext(ctx, `SELECT COUNT(1) FROM resources
WHERE resource_type='kalshi_markets'
AND json_extract(data,'$.series_ticker') = ?
AND json_extract(data,'$.status') = 'active'
LIMIT 1`, h.ID)
			var marketCount int
			if mErr := marketRow.Scan(&marketCount); mErr != nil || marketCount == 0 {
				continue
			}
		}
		out = append(out, h)
	}
	return out
}

// filterPolyActiveOnly drops Polymarket hits whose stored status indicates
// closed/inactive. Status is populated by topicHitFromJSON's pmStatus.
func filterPolyActiveOnly(hits []topicHit) []topicHit {
	out := make([]topicHit, 0, len(hits))
	for _, h := range hits {
		if h.Source == "polymarket" && (h.Status == "closed" || h.Status == "inactive") {
			continue
		}
		out = append(out, h)
	}
	return out
}

// forceIncludeNamedOutcomes appends hits whose Title contains a query token
// as a whole word but did not make the truncated result set. Catches the
// case where a query names an outcome buried below the BM25+vol cap, e.g.
// "USA" in a 48-team World Cup event.
func forceIncludeNamedOutcomes(results, polyHits, kalshiHits []topicHit, tokens []string, limit int) []topicHit {
	if len(tokens) == 0 {
		return results
	}
	seen := make(map[string]struct{}, len(results))
	for _, h := range results {
		seen[h.Source+"|"+h.ID] = struct{}{}
	}
	pool := make([]topicHit, 0, len(polyHits)+len(kalshiHits))
	pool = append(pool, polyHits...)
	pool = append(pool, kalshiHits...)
	for _, h := range pool {
		if _, dup := seen[h.Source+"|"+h.ID]; dup {
			continue
		}
		lower := strings.ToLower(h.Title)
		matched := false
		for _, tok := range tokens {
			if containsWord(lower, tok) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		results = append(results, h)
		seen[h.Source+"|"+h.ID] = struct{}{}
	}
	if len(results) > limit*2 {
		// Hard cap on force-include expansion; never inflate beyond 2x
		// the user's requested limit even if many tokens match.
		results = results[:limit*2]
	}
	return results
}

// containsWord returns true if `tok` appears as a whole word inside `s`.
// Used by force-include so "USA" matches "Will USA win" but not "USA-
// branded foo" or "Caused by it". s is expected to be lowercased.
func containsWord(s, tok string) bool {
	if tok == "" {
		return false
	}
	for {
		idx := strings.Index(s, tok)
		if idx < 0 {
			return false
		}
		left := idx == 0 || !isWordRune(s[idx-1])
		right := idx+len(tok) == len(s) || !isWordRune(s[idx+len(tok)])
		if left && right {
			return true
		}
		s = s[idx+1:]
	}
}

func isWordRune(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9')
}

// expandWithPrices replaces Kalshi series shells and Polymarket event
// shells with their highest-volume active market sibling so a single
// `topic` call surfaces actionable prices instead of empty title rows.
// Pure-local: queries the synced store, no live API calls.
func expandWithPrices(ctx context.Context, db *sql.DB, hits []topicHit) []topicHit {
	for i := range hits {
		switch {
		case hits[i].Source == "kalshi" && hits[i].Kind == "series":
			if priced, ok := kalshiTopMarketForSeries(ctx, db, hits[i].ID); ok {
				priced.ExpandedFrom = "series:" + hits[i].ID
				hits[i] = priced
			}
		case hits[i].Source == "polymarket" && hits[i].Kind == "event":
			if priced, ok := polymarketTopMarketForEvent(ctx, db, hits[i].ID); ok {
				priced.ExpandedFrom = "event:" + hits[i].ID
				hits[i] = priced
			}
		}
	}
	return hits
}

// kalshiTopMarketForSeries returns the highest-volume active market under
// a Kalshi series ticker, formatted as a topicHit. Returns false when no
// active market exists.
func kalshiTopMarketForSeries(ctx context.Context, db *sql.DB, seriesTicker string) (topicHit, bool) {
	row := db.QueryRowContext(ctx, `SELECT id, data FROM resources
WHERE resource_type='kalshi_markets'
AND json_extract(data,'$.series_ticker') = ?
AND json_extract(data,'$.status') = 'active'
ORDER BY CAST(COALESCE(json_extract(data,'$.volume_24h_fp'),0) AS REAL) DESC
LIMIT 1`, seriesTicker)
	var id, data sql.NullString
	if err := row.Scan(&id, &data); err != nil || !data.Valid {
		return topicHit{}, false
	}
	return topicHitFromJSON("kalshi_markets", id.String, data.String)
}

// polymarketTopMarketForEvent returns the highest-volume open market under
// a Polymarket event slug. Polymarket events embed their child markets in
// the events table's data column as a `markets` array; the helper expands
// the highest-volumed entry. Returns false if no open child market exists.
func polymarketTopMarketForEvent(ctx context.Context, db *sql.DB, eventSlug string) (topicHit, bool) {
	row := db.QueryRowContext(ctx, `SELECT data FROM resources
WHERE resource_type='events'
AND json_extract(data,'$.slug') = ?
LIMIT 1`, eventSlug)
	var data sql.NullString
	if err := row.Scan(&data); err != nil || !data.Valid {
		return topicHit{}, false
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(data.String), &event); err != nil {
		return topicHit{}, false
	}
	markets, _ := event["markets"].([]any)
	var best map[string]any
	bestVol := -1.0
	for _, m := range markets {
		mObj, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if jsonString(mObj, "closed") == "true" {
			continue
		}
		vol := firstFloat(mObj, "volume24h", "volume24hr", "volumeNum")
		if vol > bestVol {
			bestVol = vol
			best = mObj
		}
	}
	if best == nil {
		return topicHit{}, false
	}
	raw, err := json.Marshal(best)
	if err != nil {
		return topicHit{}, false
	}
	return topicHitFromJSON("markets", jsonString(best, "slug"), string(raw))
}

// interleaveTopicHits round-robins two ranked venue slices into one bundle
// of at most `limit` rows, deduplicating by (source,id) and by case-folded
// title so v1/v2 Kalshi series with identical titles fold into one row.
func interleaveTopicHits(a, b []topicHit, limit int) []topicHit {
	if limit <= 0 {
		return nil
	}
	out := make([]topicHit, 0, limit)
	seenKey := make(map[string]struct{}, limit)
	seenTitle := make(map[string]struct{}, limit)
	add := func(h topicHit) bool {
		k := h.Source + "|" + h.ID
		if _, dup := seenKey[k]; dup {
			return false
		}
		tk := strings.ToLower(strings.TrimSpace(h.Title))
		if tk != "" {
			if _, dup := seenTitle[tk]; dup {
				return false
			}
			seenTitle[tk] = struct{}{}
		}
		seenKey[k] = struct{}{}
		out = append(out, h)
		return true
	}
	ai, bi := 0, 0
	for len(out) < limit && (ai < len(a) || bi < len(b)) {
		if ai < len(a) {
			add(a[ai])
			ai++
			if len(out) >= limit {
				break
			}
		}
		if bi < len(b) {
			add(b[bi])
			bi++
		}
	}
	return out
}

func topicHitFromJSON(resourceType, fallbackID, raw string) (topicHit, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return topicHit{}, false
	}
	id := fallbackID
	h := topicHit{ID: id}
	switch resourceType {
	case "markets":
		id = firstNonEmpty(jsonString(obj, "slug"), id)
		h = topicHit{Source: "polymarket", Kind: "market", ID: id, Title: firstNonEmpty(jsonString(obj, "question"), jsonString(obj, "title")), Status: pmStatus(obj), YesProbability: jsonFloat(obj, "lastTradePrice"), Volume24h: firstFloat(obj, "volume24hr", "volumeNum"), EndDate: jsonString(obj, "endDate"), URL: "https://polymarket.com/market/" + id}
	case "events":
		id = firstNonEmpty(jsonString(obj, "slug"), id)
		h = topicHit{Source: "polymarket", Kind: "event", ID: id, Title: jsonString(obj, "title"), Status: pmStatus(obj), Volume24h: jsonFloat(obj, "volume"), EndDate: jsonString(obj, "endDate"), URL: "https://polymarket.com/event/" + id}
	case "tags":
		id = firstNonEmpty(jsonString(obj, "slug"), id)
		h = topicHit{Source: "polymarket", Kind: "tag", ID: id, Title: firstNonEmpty(jsonString(obj, "label"), jsonString(obj, "title")), URL: "https://polymarket.com/tag/" + id}
	case "kalshi_markets":
		id = firstNonEmpty(jsonString(obj, "ticker"), id)
		eventTicker := jsonString(obj, "event_ticker")
		yesAsk := jsonFloat(obj, "yes_ask_dollars")
		noAsk := jsonFloat(obj, "no_ask_dollars")
		lastPrice := jsonFloat(obj, "last_price_dollars")
		volume24h := jsonFloat(obj, "volume_24h_fp")
		untraded := isUntradedKalshi(yesAsk, noAsk, lastPrice, volume24h)
		h = topicHit{Source: "kalshi", Kind: "market", ID: id, Title: jsonString(obj, "title"), Status: jsonString(obj, "status"), YesProbability: lastPrice, Volume24h: volume24h, EndDate: jsonString(obj, "expiration_time"), URL: "https://kalshi.com/markets/" + eventTicker + "/" + id, Untraded: untraded}
	case "kalshi_events":
		id = firstNonEmpty(jsonString(obj, "event_ticker"), id)
		h = topicHit{Source: "kalshi", Kind: "event", ID: id, Title: jsonString(obj, "title"), EndDate: jsonString(obj, "strike_period"), URL: "https://kalshi.com/markets/" + id}
	case "kalshi_series":
		id = firstNonEmpty(jsonString(obj, "ticker"), id)
		h = topicHit{Source: "kalshi", Kind: "series", ID: id, Title: jsonString(obj, "title"), URL: "https://kalshi.com/markets?series=" + id}
	}
	return h, h.Source != "" && h.ID != ""
}

func topicRows(items []topicHit) [][]string {
	rows := make([][]string, 0, len(items))
	for _, it := range items {
		probCell := formatProb(it.YesProbability)
		if it.Untraded {
			probCell = "untraded"
		}
		rows = append(rows, []string{it.Source, it.Kind, it.Title, probCell, formatNumber(it.Volume24h), it.EndDate})
	}
	return rows
}

func printSimpleTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func jsonString(obj map[string]any, key string) string {
	if v, ok := obj[key]; ok && v != nil {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func jsonFloat(obj map[string]any, key string) float64 {
	if v, ok := obj[key]; ok && v != nil {
		switch n := v.(type) {
		case float64:
			return n
		case json.Number:
			f, _ := n.Float64()
			return f
		case string:
			var f float64
			_, _ = fmt.Sscanf(n, "%f", &f)
			return f
		}
	}
	return 0
}

func firstFloat(obj map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if f := jsonFloat(obj, k); f != 0 {
			return f
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// refreshTopicHits batches the topic command's hits by venue, fires
// one live API call per venue, and overwrites the cached
// price-bearing fields on the in-memory slice. Returns the per-venue
// outcome so the caller can populate the envelope's price_source.
//
// Hits with Kind=="market" carry prices; other kinds (event/tag/
// series) intentionally have no PriceSource set so an agent can tell
// a tag-row apart from a market-row whose refresh failed.
func refreshTopicHits(ctx context.Context, fc freshnessClient, hits []topicHit) refreshOutcome {
	polySlugs := make([]string, 0, len(hits))
	kalshiTickers := make([]string, 0, len(hits))
	for _, h := range hits {
		if h.Kind != "market" {
			continue
		}
		switch h.Source {
		case "polymarket":
			polySlugs = append(polySlugs, h.ID)
		case "kalshi":
			kalshiTickers = append(kalshiTickers, h.ID)
		}
	}
	outcome := refreshVenues(ctx, fc, polySlugs, kalshiTickers)
	for i := range hits {
		if hits[i].Kind != "market" {
			continue
		}
		switch hits[i].Source {
		case "polymarket":
			if !outcome.PolymarketAsked {
				continue
			}
			if !outcome.PolymarketOK {
				hits[i].PriceSource = priceSourceStale
				continue
			}
			if v, ok := outcome.Polymarket[hits[i].ID]; ok {
				applyLiveValuesIfPresent(v, &hits[i].YesProbability, &hits[i].Volume24h, &hits[i].Status)
			}
			hits[i].PriceSource = priceSourceLive
		case "kalshi":
			if !outcome.KalshiAsked {
				continue
			}
			if !outcome.KalshiOK {
				hits[i].PriceSource = priceSourceStale
				continue
			}
			if v, ok := outcome.Kalshi[hits[i].ID]; ok {
				applyLiveValuesIfPresent(v, &hits[i].YesProbability, &hits[i].Volume24h, &hits[i].Status)
			}
			hits[i].PriceSource = priceSourceLive
		}
	}
	return outcome
}

func pmStatus(obj map[string]any) string {
	if jsonString(obj, "closed") == "true" {
		return "closed"
	}
	if jsonString(obj, "active") == "false" {
		return "inactive"
	}
	return "active"
}
