package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/marketing/erank/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/erank/internal/store"
)

type keywordSignals struct {
	Keyword     string           `json:"keyword"`
	Source      string           `json:"source"`
	Country     string           `json:"country"`
	Stats       map[string]any   `json:"stats,omitempty"`
	TopListings []map[string]any `json:"top_listings,omitempty"`
	Related     []map[string]any `json:"related_searches,omitempty"`
	NearMatches []map[string]any `json:"near_matches,omitempty"`
	EtsyTags    []map[string]any `json:"etsy_tags,omitempty"`
	Warnings    []string         `json:"warnings,omitempty"`
}

type scoredKeyword struct {
	Keyword           string   `json:"keyword"`
	Source            string   `json:"source,omitempty"`
	Country           string   `json:"country,omitempty"`
	Score             float64  `json:"score"`
	Rating            string   `json:"rating"`
	SearchSignal      float64  `json:"search_signal,omitempty"`
	CompetitionSignal float64  `json:"competition_signal,omitempty"`
	DifficultySignal  float64  `json:"difficulty_signal,omitempty"`
	TopListingCount   int      `json:"top_listing_count"`
	TagCount          int      `json:"tag_count"`
	Warnings          []string `json:"warnings,omitempty"`
}

type consensusTag struct {
	Tag     string   `json:"tag"`
	Count   int      `json:"count"`
	Sources []string `json:"sources"`
}

func fetchKeywordSignals(ctx context.Context, flags *rootFlags, keyword, source, country string) (keywordSignals, error) {
	if isPrintingPressInvalidValue(keyword) {
		return keywordSignals{}, fmt.Errorf("invalid keyword %q", keyword)
	}
	c, err := flags.newClient()
	if err != nil {
		return keywordSignals{}, err
	}
	return fetchKeywordSignalsWithClient(ctx, c, keyword, source, country)
}

func isPrintingPressInvalidValue(value string) bool {
	return strings.Contains(value, "__printing_press_invalid__")
}

func fetchKeywordSignalsWithClient(ctx context.Context, c *client.Client, keyword, source, country string) (keywordSignals, error) {
	params := map[string]string{"keyword": keyword, "marketplace": source, "country": country}
	signals := keywordSignals{Keyword: keyword, Source: source, Country: country}
	var failures []string
	if raw, err := c.Get(ctx, "/api/keyword-tool/stats", params); err == nil {
		signals.Stats = decodeObject(raw)
	} else {
		failures = append(failures, "stats: "+err.Error())
	}
	if raw, err := c.Get(ctx, "/api/keyword-tool/top-listings", params); err == nil {
		signals.TopListings = decodeRows(raw)
	} else {
		failures = append(failures, "top-listings: "+err.Error())
	}
	if raw, err := c.Get(ctx, "/api/keyword-tool/related-searches", withDefaultMatch(params)); err == nil {
		signals.Related = decodeRows(raw)
	} else {
		failures = append(failures, "related-searches: "+err.Error())
	}
	if raw, err := c.Get(ctx, "/api/keyword-tool/near-matches", params); err == nil {
		signals.NearMatches = decodeRows(raw)
	} else {
		failures = append(failures, "near-matches: "+err.Error())
	}
	if raw, err := c.Get(ctx, "/api/keyword-tool/etsy-tags", withDefaultMatch(params)); err == nil {
		signals.EtsyTags = decodeRows(raw)
	} else {
		failures = append(failures, "etsy-tags: "+err.Error())
	}
	signals.Warnings = failures
	if len(failures) >= 5 {
		return signals, fmt.Errorf("all eRank keyword surfaces failed: %s", strings.Join(failures, "; "))
	}
	return signals, nil
}

func withDefaultMatch(params map[string]string) map[string]string {
	next := map[string]string{}
	for key, value := range params {
		next[key] = value
	}
	next["opensearch_filters[default_match]"] = "true"
	return next
}

func scoreKeyword(signals keywordSignals) scoredKeyword {
	searchSignal := bestNumber(signals.Stats, "search", "volume", "avg_searches")
	competitionSignal := bestNumber(signals.Stats, "competition", "competing", "results")
	difficultySignal := bestNumber(signals.Stats, "difficulty")
	tagCount := len(rankConsensusTags(signals, 1))
	score := 50.0
	score += math.Min(searchSignal/100.0, 25)
	score -= math.Min(competitionSignal/10000.0, 20)
	score -= math.Min(difficultySignal/5.0, 20)
	score += math.Min(float64(tagCount), 15)
	score = math.Max(0, math.Min(100, score))
	return scoredKeyword{
		Keyword:           signals.Keyword,
		Source:            signals.Source,
		Country:           signals.Country,
		Score:             math.Round(score*10) / 10,
		Rating:            rating(score),
		SearchSignal:      searchSignal,
		CompetitionSignal: competitionSignal,
		DifficultySignal:  difficultySignal,
		TopListingCount:   len(signals.TopListings),
		TagCount:          tagCount,
		Warnings:          signals.Warnings,
	}
}

func rating(score float64) string {
	switch {
	case score >= 75:
		return "strong"
	case score >= 55:
		return "mixed"
	default:
		return "crowded"
	}
}

func saturationLabel(score scoredKeyword) string {
	if score.DifficultySignal >= 70 || score.CompetitionSignal >= 100000 || score.Score < 45 {
		return "high"
	}
	if score.DifficultySignal >= 40 || score.CompetitionSignal >= 25000 || score.Score < 65 {
		return "medium"
	}
	return "low"
}

const keywordSignalSnapshotTimeout = 250 * time.Millisecond

type keywordSignalSnapshotRecorder struct {
	db *store.Store
}

func newKeywordSignalSnapshotRecorder(ctx context.Context) *keywordSignalSnapshotRecorder {
	recordCtx, cancel := context.WithTimeout(ctx, keywordSignalSnapshotTimeout)
	defer cancel()

	db, err := store.OpenWithContext(recordCtx, defaultDBPath("erank-pp-cli"))
	if err != nil {
		return nil
	}
	return &keywordSignalSnapshotRecorder{db: db}
}

func (r *keywordSignalSnapshotRecorder) Close() {
	if r == nil || r.db == nil {
		return
	}
	_ = r.db.Close()
}

// MCP read-only annotations mean no remote mutation. This local cache write is
// best-effort and intentionally cannot fail live analysis or alter output.
func recordKeywordSignalSnapshot(ctx context.Context, recorder *keywordSignalSnapshotRecorder, signals keywordSignals, score scoredKeyword) {
	if recorder == nil || recorder.db == nil {
		return
	}

	source := signals.Source
	if source == "" {
		source = "live"
	}
	country := signals.Country
	if country == "" {
		country = "us"
	}

	recordCtx, cancel := context.WithTimeout(ctx, keywordSignalSnapshotTimeout)
	defer cancel()

	_ = recorder.db.InsertKeywordSignalSnapshot(recordCtx, store.KeywordSignalSnapshot{
		Keyword:           score.Keyword,
		Source:            source,
		Country:           country,
		Score:             score.Score,
		Rating:            score.Rating,
		SearchSignal:      score.SearchSignal,
		CompetitionSignal: score.CompetitionSignal,
		DifficultySignal:  score.DifficultySignal,
		TagCount:          score.TagCount,
		TopListingCount:   score.TopListingCount,
	})
}

func rankConsensusTags(signals keywordSignals, minCount int) []consensusTag {
	type bucket struct {
		count   int
		sources map[string]bool
	}
	buckets := map[string]*bucket{}
	add := func(source, token string) {
		b, ok := buckets[token]
		if !ok {
			b = &bucket{sources: map[string]bool{}}
			buckets[token] = b
		}
		b.count++
		b.sources[source] = true
	}
	addRow := func(source string, row map[string]any) {
		seen := map[string]bool{}
		for _, value := range collectStrings(row) {
			token := normalizeToken(value)
			if token == "" || len(token) < 3 || seen[token] {
				continue
			}
			seen[token] = true
			add(source, token)
		}
	}
	for _, row := range signals.TopListings {
		addRow("top_listings", row)
	}
	for _, row := range signals.EtsyTags {
		addRow("etsy_tags", row)
	}
	for _, row := range signals.Related {
		addRow("related_searches", row)
	}
	for _, row := range signals.NearMatches {
		addRow("near_matches", row)
	}
	out := make([]consensusTag, 0, len(buckets))
	for tag, b := range buckets {
		if b.count < minCount {
			continue
		}
		sources := make([]string, 0, len(b.sources))
		for source := range b.sources {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		out = append(out, consensusTag{Tag: tag, Count: b.count, Sources: sources})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Tag < out[j].Tag
		}
		return out[i].Count > out[j].Count
	})
	if len(out) > 50 {
		return out[:50]
	}
	return out
}

func buildAngles(signals keywordSignals, limit int) []map[string]any {
	tags := rankConsensusTags(signals, 2)
	if limit <= 0 {
		limit = 10
	}
	if len(tags) > limit {
		tags = tags[:limit]
	}
	angles := make([]map[string]any, 0, len(tags))
	for _, tag := range tags {
		angles = append(angles, map[string]any{
			"angle":    titleLabel(tag.Tag),
			"evidence": tag,
			"prompt":   fmt.Sprintf("Explore %q products for %q using tags from %s.", tag.Tag, signals.Keyword, strings.Join(tag.Sources, ", ")),
		})
	}
	return angles
}

func decodeRows(raw json.RawMessage) []map[string]any {
	var value any
	if len(raw) == 0 || json.Unmarshal(raw, &value) != nil {
		return nil
	}
	return rowsFromAny(value)
}

func decodeObject(raw json.RawMessage) map[string]any {
	var obj map[string]any
	if json.Unmarshal(raw, &obj) == nil {
		return obj
	}
	return map[string]any{}
}

func rowsFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if obj, ok := item.(map[string]any); ok {
				rows = append(rows, obj)
			}
		}
		return rows
	case map[string]any:
		for _, key := range []string{"data", "items", "results", "rows", "listings", "keywords", "tags"} {
			if nested, ok := typed[key]; ok {
				if rows := rowsFromAny(nested); len(rows) > 0 {
					return rows
				}
			}
		}
		return []map[string]any{typed}
	default:
		return nil
	}
}

func collectStrings(value any) []string {
	var out []string
	var walk func(any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			out = append(out, typed)
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for key, item := range typed {
				lower := strings.ToLower(key)
				if strings.Contains(lower, "tag") || strings.Contains(lower, "title") || strings.Contains(lower, "keyword") || strings.Contains(lower, "phrase") {
					walk(item)
				}
			}
		}
	}
	walk(value)
	return out
}

func extractKeywordTerms(raw json.RawMessage) []string {
	return extractKeywordTermsFromRows(decodeRows(raw))
}

func extractKeywordTermsFromRows(rows []map[string]any) []string {
	seen := map[string]bool{}
	var terms []string
	for _, row := range rows {
		for _, value := range collectStrings(row) {
			value = strings.TrimSpace(value)
			key := normalizeToken(value)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			terms = append(terms, value)
		}
	}
	return terms
}

func titleLabel(value string) string {
	words := strings.Fields(strings.ToLower(value))
	for i, word := range words {
		runes := []rune(word)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

func bestNumber(value any, needles ...string) float64 {
	best := 0.0
	var walk func(any, string)
	walk = func(v any, key string) {
		switch typed := v.(type) {
		case map[string]any:
			for k, item := range typed {
				walk(item, k)
			}
		case []any:
			for _, item := range typed {
				walk(item, key)
			}
		case float64:
			if keyMatches(key, needles...) && typed > best {
				best = typed
			}
		case int:
			if keyMatches(key, needles...) && float64(typed) > best {
				best = float64(typed)
			}
		case string:
			if keyMatches(key, needles...) {
				if n, err := strconv.ParseFloat(strings.ReplaceAll(typed, ",", ""), 64); err == nil && n > best {
					best = n
				}
			}
		}
	}
	walk(value, "")
	return best
}

func keyMatches(key string, needles ...string) bool {
	key = strings.ToLower(key)
	for _, needle := range needles {
		if strings.Contains(key, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func normalizeTokenSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		key := normalizeToken(value)
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func normalizeToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.NewReplacer("-", " ", "_", " ", "/", " ").Replace(value)
	value = strings.Join(strings.Fields(value), " ")
	return value
}
