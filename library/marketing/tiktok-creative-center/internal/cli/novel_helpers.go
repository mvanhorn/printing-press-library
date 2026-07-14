// Copyright 2026 Jon and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-implemented transcendence support for the TikTok Creative Center CLI.
// Pure parsing/synthesis helpers shared by the novel commands. Kept side-effect
// free so they can be unit-tested without a SQLite store.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/tiktok-creative-center/internal/store"
)

// cliName is the canonical binary/config name used for store + config paths.
const novelCLIIName = "tiktok-creative-center-pp-cli"

// syncFirstHint is the actionable error returned when a transcendence command
// is invoked before any data has been mirrored into the local store.
const syncFirstHint = "no local data — run 'tiktok-creative-center-pp-cli sync --resources hashtags --param countryCode=US --param timeRange=7' first"

// novelOpenStore opens the local store read-only, returning a friendly error
// when no sync has happened yet.
func novelOpenStore(ctx context.Context) (*store.Store, error) {
	db, err := openStoreForRead(ctx, novelCLIIName)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if db == nil {
		return nil, fmt.Errorf("%s", syncFirstHint)
	}
	return db, nil
}

// hashtagRow is the decoded projection of a synced hashtag used by every
// transcendence command. Fields are normalized from their raw (often stringly
// typed) upstream shapes.
type hashtagRow struct {
	ID             string   `json:"hashtagID"`
	Name           string   `json:"hashtagName"`
	PublishCnt     float64  `json:"publishCnt"`
	RankIndex      float64  `json:"rankIndex"`
	Popularity     float64  `json:"popularity"`
	PopularityLast float64  `json:"popularityLast"`
	PopularityPrev float64  `json:"popularityPrev,omitempty"`
	IndustryIDs    []string `json:"industryIDs"`
	TopCreators    []string `json:"topCreators"`
	CountryCode    string   `json:"countryCode,omitempty"`
	TimeRange      int      `json:"timeRange,omitempty"`
	SyncedAt       string   `json:"syncedAt,omitempty"`
	raw            map[string]any
}

// loadHashtagRows reads every synced hashtag, filters by region, and returns
// decoded rows. region=="" means all regions.
func loadHashtagRows(ctx context.Context, db *store.Store, region string) ([]hashtagRow, error) {
	rows, err := db.Query(`SELECT data, synced_at FROM "hashtags" ORDER BY synced_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("querying hashtags: %w", err)
	}
	defer rows.Close()

	seen := map[string]bool{}
	out := make([]hashtagRow, 0, 64)
	for rows.Next() {
		var dataJSON, syncedAt string
		if err := rows.Scan(&dataJSON, &syncedAt); err != nil {
			return nil, fmt.Errorf("scanning hashtag row: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &obj); err != nil {
			continue
		}
		row := decodeHashtag(obj)
		row.SyncedAt = syncedAt
		// The hashtag list response does not include countryCode (it is a
		// request param, not a response field), so synced rows carry no
		// region. Only filter when a row actually has one; otherwise the
		// region was already applied at sync time and the row passes through.
		if region != "" && row.CountryCode != "" && !strings.EqualFold(row.CountryCode, region) {
			continue
		}
		// Keep the freshest row per hashtag ID (rows ordered DESC by synced_at).
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out = append(out, row)
	}
	return out, rows.Err()
}

// decodeHashtag normalizes a raw hashtag object into a hashtagRow.
func decodeHashtag(obj map[string]any) hashtagRow {
	row := hashtagRow{
		ID:          strAny(obj, "hashtagID", "hashtag_id", "id"),
		Name:        strAny(obj, "hashtagName", "hashtag_name", "name"),
		PublishCnt:  parseFloat(obj, "publishCnt", "publish_cnt"),
		RankIndex:   parseFloat(obj, "rankIndex", "rank_index"),
		IndustryIDs: toStringSlice(obj["industryIDs"]),
		TopCreators: decodeTopCreators(obj["topCreators"]),
		CountryCode: strAny(obj, "countryCode", "country_code"),
		TimeRange:   int(parseFloat(obj, "timeRange", "time_range")),
		raw:         obj,
	}
	curve := obj["popularityCurve"]
	row.Popularity, row.PopularityLast = curveMaxAndLast(curve)
	return row
}

// decodeTopCreators extracts creator identifiers from the topCreators field,
// which may be an array of objects or strings.
func decodeTopCreators(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		switch val := item.(type) {
		case string:
			if val != "" {
				out = append(out, val)
			}
		case map[string]any:
			if id := strAny(val, "creatorID", "userId", "uniqueID", "nickname", "name"); id != "" {
				out = append(out, id)
			}
		}
	}
	return out
}

// curveMaxAndLast returns (max, last) numeric values from a popularity curve.
// Defensive across shapes: [{date,value}], [{time,count}], [[x,y]], or a bare
// number array. Returns (0,0) when nothing parseable is found.
func curveMaxAndLast(curve any) (max, last float64) {
	arr, ok := curve.([]any)
	if !ok || len(arr) == 0 {
		return 0, 0
	}
	for _, point := range arr {
		v := curvePointValue(point)
		if math.IsNaN(v) {
			continue
		}
		if v > max {
			max = v
		}
		last = v
	}
	return max, last
}

// curvePointValue extracts the numeric "y" from one curve point.
func curvePointValue(point any) float64 {
	switch p := point.(type) {
	case float64:
		return p
	case int:
		return float64(p)
	case string:
		return parseFloat(map[string]any{"_": p}, "_")
	case map[string]any:
		// Prefer value/count/popularity; fall back to the second element of an
		// implicit [x,y] pair by scanning numeric fields in insertion order.
		for _, key := range []string{"value", "count", "popularity", "vv", "y"} {
			if v, ok := p[key]; ok {
				if f := asFloat(v); !math.IsNaN(f) {
					return f
				}
			}
		}
		return mapSecondNumeric(p)
	case []any:
		if len(p) >= 2 {
			return asFloat(p[1])
		}
	}
	return math.NaN()
}

// mapSecondNumeric returns the second numeric value encountered in a map,
// approximating [x,y] when keys are non-standard.
func mapSecondNumeric(m map[string]any) float64 {
	type kv struct {
		k string
		v float64
	}
	nums := make([]kv, 0, len(m))
	for k, v := range m {
		if f := asFloat(v); !math.IsNaN(f) {
			nums = append(nums, kv{k, f})
		}
	}
	if len(nums) < 2 {
		return math.NaN()
	}
	sort.Slice(nums, func(i, j int) bool { return nums[i].k < nums[j].k })
	return nums[1].v
}

// asFloat coerces any JSON-decoded scalar to float64; NaN if not numeric.
func asFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(n), 64)
		if err != nil {
			return math.NaN()
		}
		return f
	case bool:
		if n {
			return 1
		}
		return 0
	}
	return math.NaN()
}

// parseFloat reads a (possibly string-typed) numeric field by any of its keys.
func parseFloat(obj map[string]any, keys ...string) float64 {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if f := asFloat(v); !math.IsNaN(f) {
				return f
			}
		}
	}
	return 0
}

// strAny reads the first non-empty string field by any of its keys.
func strAny(obj map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := obj[k]; ok {
			if s := toStr(v); s != "" {
				return s
			}
		}
	}
	return ""
}

// toStr coerces a JSON-decoded scalar to a trimmed string.
func toStr(v any) string {
	switch s := v.(type) {
	case string:
		return strings.TrimSpace(s)
	case float64:
		if s == math.Trunc(s) {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		if s {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		b, _ := json.Marshal(s)
		return strings.Trim(string(b), `"`)
	}
}

// toStringSlice flattens a JSON array of strings/numbers into []string.
func toStringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s := toStr(item); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// matchNiche reports whether a hashtag is relevant to a niche keyword. The
// keyword is tokenized (split on whitespace/punctuation) and a row matches if
// ANY token is a substring of the hashtag name or any industry — or the full
// keyword is. This handles TikTok's spaceless hashtag names: "marvel rivals"
// matches "marvelrivalss9" via the "marvel" token. Empty keyword matches all.
func matchNiche(row hashtagRow, niche string) bool {
	if niche == "" {
		return true
	}
	name := strings.ToLower(row.Name)
	full := strings.ToLower(niche)
	if strings.Contains(name, full) {
		return true
	}
	for _, ind := range row.IndustryIDs {
		if strings.Contains(strings.ToLower(ind), full) {
			return true
		}
	}
	for _, tok := range nicheTokens(niche) {
		if tok == "" {
			continue
		}
		if strings.Contains(name, tok) {
			return true
		}
		for _, ind := range row.IndustryIDs {
			if strings.Contains(strings.ToLower(ind), tok) {
				return true
			}
		}
	}
	return false
}

// nicheTokens splits a niche keyword into lowercase match tokens on any
// non-alphanumeric rune, so "marvel rivals" -> ["marvel","rivals"] and
// "marvel-rivals s9" -> ["marvel","rivals","s9"].
func nicheTokens(niche string) []string {
	fields := strings.FieldsFunc(strings.ToLower(niche), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	return fields
}

// opportunityScore ranks hashtags by "high popularity, low publish count" —
// the underserved-but-rising signal. Score = popularity / max(publishCnt, 1).
func opportunityScore(row hashtagRow) float64 {
	return row.Popularity / math.Max(row.PublishCnt, 1)
}

// viralRank returns hashtags sorted by opportunity score (desc), truncated.
func viralRank(rows []hashtagRow, top int) []hashtagRow {
	sort.SliceStable(rows, func(i, j int) bool {
		return opportunityScore(rows[i]) > opportunityScore(rows[j])
	})
	if top > 0 && len(rows) > top {
		return rows[:top]
	}
	return rows
}

// slopeTrend classifies a popularity curve's direction from its first→last
// delta. Returns the delta and a human label.
func slopeTrend(row hashtagRow) (delta float64, label string) {
	delta = row.PopularityLast - row.PopularityPrev
	switch {
	case delta > 0:
		label = "rising"
	case delta < 0:
		label = "falling"
	default:
		label = "flat"
	}
	return
}

// sharedIndustries returns industries shared between two hashtags.
func sharedIndustries(a, b hashtagRow) []string {
	set := map[string]bool{}
	for _, x := range a.IndustryIDs {
		set[x] = true
	}
	var out []string
	for _, y := range b.IndustryIDs {
		if set[y] {
			out = append(out, y)
		}
	}
	return out
}

// sharedCreators returns creator identifiers shared between two hashtags.
func sharedCreators(a, b hashtagRow) []string {
	set := map[string]bool{}
	for _, x := range a.TopCreators {
		set[x] = true
	}
	var out []string
	for _, y := range b.TopCreators {
		if set[y] {
			out = append(out, y)
		}
	}
	return out
}

// parseDurationArg parses a human duration like "24h", "7d", "2w", "3m".
func parseDurationArg(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	unit := s[len(s)-1]
	num := s[:len(s)-1]
	n, err := strconv.Atoi(num)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid duration %q: expected like 24h, 7d, 2w", s)
	}
	switch unit {
	case 'h':
		return time.Duration(n) * time.Hour, nil
	case 'd':
		return time.Duration(n) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(n) * 7 * 24 * time.Hour, nil
	case 'm':
		return time.Duration(n) * 30 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("invalid duration unit %q in %q: use h/d/w/m", unit, s)
	}
}

// parseIntFlag parses a string flag as a positive int with a default fallback.
func parseIntFlag(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

// storeHasMultiSync reports whether the hashtags table holds rows from more
// than one distinct sync timestamp, enabling a true cross-sync diff.
func storeHasMultiSync(db *store.Store) (bool, error) {
	rows, err := db.Query(`SELECT COUNT(DISTINCT synced_at) FROM "hashtags"`)
	if err != nil {
		return false, fmt.Errorf("counting sync snapshots: %w", err)
	}
	defer rows.Close()
	var n int
	if !rows.Next() {
		return false, nil
	}
	if err := rows.Scan(&n); err != nil {
		return false, fmt.Errorf("scanning sync count: %w", err)
	}
	return n > 1, rows.Err()
}

// storeHashtagIDsSince returns hashtag IDs whose synced_at is newer than cutoff.
func storeHashtagIDsSince(db *store.Store, cutoff time.Time) ([]hashtagRow, error) {
	rows, err := db.Query(`SELECT data FROM "hashtags" WHERE synced_at > ? ORDER BY synced_at DESC`, cutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("querying new hashtags: %w", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	out := make([]hashtagRow, 0, 32)
	for rows.Next() {
		var dataJSON string
		if err := rows.Scan(&dataJSON); err != nil {
			return nil, fmt.Errorf("scanning hashtag row: %w", err)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(dataJSON), &obj); err != nil {
			continue
		}
		row := decodeHashtag(obj)
		if seen[row.ID] {
			continue
		}
		seen[row.ID] = true
		out = append(out, row)
	}
	return out, rows.Err()
}

// ensure os import retained for future stderr hints without churn.
var _ = os.Stderr

// topAdRow is a decoded Top Ads / Top Content item used by the content and
// competitor commands. Extracted defensively from the itemInfo/itemAuthorInfo
// nested objects.
type topAdRow struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Author     string   `json:"author"`
	Handle     string   `json:"handle,omitempty"`
	AuthorID   string   `json:"authorID"`
	Popularity float64  `json:"popularity"`
	Region     string   `json:"region,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	AdText     string   `json:"adText,omitempty"`
}

// loadTopAdRows reads every synced top-ads row and decodes it. region==""
// means all regions.
func loadTopAdRows(ctx context.Context, db *store.Store, region string) ([]topAdRow, error) {
	raw, err := db.List("top-ads", 0)
	if err != nil {
		return nil, fmt.Errorf("querying top-ads: %w", err)
	}
	out := make([]topAdRow, 0, len(raw))
	for _, r := range raw {
		trimmed := strings.TrimSpace(string(r))
		if trimmed == "" || trimmed == "null" || trimmed == "[]" || trimmed == "{}" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(r, &obj); err != nil {
			continue
		}
		row := decodeTopAd(obj)
		if region != "" && row.Region != "" && !strings.EqualFold(row.Region, region) {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

// decodeTopAd normalizes a raw TopContent object into a topAdRow.
func decodeTopAd(obj map[string]any) topAdRow {
	info := asMap(obj["itemInfo"])
	authorInfo := asMap(obj["itemAuthorInfo"])
	authorMetrics := asMap(obj["itemAuthorMetrics"])
	itemMetrics := asMap(obj["itemMetrics"])
	row := topAdRow{
		ID:       strAny(info, "itemID", "id", "itemId", "videoID"),
		Title:    strAny(info, "title", "itemTitle", "name", "desc"),
		Author:   strAny(authorInfo, "nickName", "handlerName", "displayname", "displayName", "nickname", "name", "uniqueID"),
		Handle:   strAny(authorInfo, "handlerName", "uniqueID", "username"),
		AuthorID: strAny(info, "authorID", "creatorID", "authorId"),
		Region:   strAny(info, "countryCode", "region", "country"),
		AdText:   strAny(info, "adText", "desc", "description"),
	}
	if row.AuthorID == "" {
		row.AuthorID = strAny(authorInfo, "userID", "userId", "uniqueID", "id")
	}
	// Popularity: prefer video views (the Creative Center ranks top content by
	// views). Fall back through organic views, then author followers, then any
	// like/play count the API exposes.
	row.Popularity = asFloat(itemMetrics["videoViews"])
	if row.Popularity == 0 {
		row.Popularity = asFloat(itemMetrics["organicVideoViews"])
	}
	if row.Popularity == 0 {
		row.Popularity = asFloat(authorMetrics["followers"])
	}
	if row.Popularity == 0 {
		row.Popularity = asFloat(itemMetrics["likeCount"])
	}
	if row.Popularity == 0 {
		row.Popularity = asFloat(info["playCount"])
	}
	if keywords := toStringSlice(info["keywords"]); len(keywords) > 0 {
		row.Keywords = keywords
	}
	if row.Title != "" {
		row.Keywords = append(row.Keywords, strings.Fields(row.Title)...)
	}
	return row
}

// asMap coerces a JSON-decoded value to map[string]any, or nil.
func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// contentItem is a unified ranked content entry across the three sources the
// content/niche commands join together.
type contentItem struct {
	Source     string  `json:"source"`
	Title      string  `json:"title"`
	Hashtag    string  `json:"hashtag,omitempty"`
	Author     string  `json:"author,omitempty"`
	Popularity float64 `json:"popularity"`
	ID         string  `json:"id,omitempty"`
}

// hashtagVideoItems extracts representative videoList entries from a hashtag
// row's raw object as contentItems.
func hashtagVideoItems(row hashtagRow) []contentItem {
	curve := row.raw["videoList"]
	arr, ok := curve.([]any)
	if !ok {
		return nil
	}
	out := make([]contentItem, 0, len(arr))
	for _, v := range arr {
		m := asMap(v)
		if m == nil {
			continue
		}
		out = append(out, contentItem{
			Source:     "representative_video",
			Title:      strAny(m, "title", "desc", "name"),
			Hashtag:    row.Name,
			Author:     strAny(m, "author", "nickname", "displayname"),
			Popularity: asFloat(m["playCount"]),
			ID:         strAny(m, "itemID", "id", "videoID"),
		})
	}
	return out
}

// topAdContentItem converts a topAdRow to a contentItem.
func topAdContentItem(ad topAdRow) contentItem {
	return contentItem{
		Source:     "top_ad",
		Title:      ad.Title,
		Author:     ad.Author,
		Popularity: ad.Popularity,
		ID:         ad.ID,
	}
}

// rankContentByPopularity sorts content items by popularity desc and truncates.
func rankContentByPopularity(items []contentItem, top int) []contentItem {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Popularity > items[j].Popularity
	})
	if top > 0 && len(items) > top {
		return items[:top]
	}
	return items
}

// sortByPopularity sorts hashtag rows by popularity descending (in place).
func sortByPopularity(rows []hashtagRow) {
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].Popularity > rows[j].Popularity
	})
}

// sortByPopularityAds sorts competitor ads by popularity descending (in place).
func sortByPopularityAds(ads []competitorAd) {
	sort.SliceStable(ads, func(i, j int) bool {
		return ads[i].Popularity > ads[j].Popularity
	})
}
