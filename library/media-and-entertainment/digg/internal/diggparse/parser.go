// Package diggparse decodes the React Server Component (RSC) stream that
// Digg AI embeds in its /ai and /ai/<clusterUrlId> HTML pages, extracts
// the structured cluster/author/event objects, and exposes them as Go
// types for the local store.
//
// Why this exists: Digg AI is a Next.js 15 SPA on Vercel. The page's
// data is shipped to the browser as a series of self.__next_f.push([1,
// "<escaped-json>"]) calls embedded inline in the HTML. There is no
// public REST endpoint for the feed; the only JSON API is
// /api/trending/status. To turn /ai into a CLI-friendly structured
// surface we parse the RSC stream from the same HTML the browser
// renders.
package diggparse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Cluster is a Digg AI story cluster as embedded in the /ai page.
// Fields mirror the upstream payload field names; unset fields are
// zero-valued. RawJSON keeps the original object for forward-compat.
type Cluster struct {
	ClusterID            string          `json:"clusterId"`
	ClusterURLID         string          `json:"clusterUrlId"`
	ShortID              string          `json:"shortId,omitempty"`
	Label                string          `json:"label,omitempty"`
	Title                string          `json:"title,omitempty"`
	TLDR                 string          `json:"tldr,omitempty"`
	URL                  string          `json:"url,omitempty"`
	Permalink            string          `json:"permalink,omitempty"`
	Topic                string          `json:"topic,omitempty"`
	CurrentRank          int             `json:"currentRank"`
	PeakRank             int             `json:"peakRank,omitempty"`
	PreviousRank         int             `json:"previousRank,omitempty"`
	Rank                 int             `json:"rank,omitempty"`
	Delta                int             `json:"delta"`
	GravityScore         float64         `json:"gravityScore,omitempty"`
	ScoreComponents      json.RawMessage `json:"scoreComponents,omitempty"`
	Evidence             json.RawMessage `json:"evidence,omitempty"`
	NumeratorCount       int             `json:"numeratorCount,omitempty"`
	NumeratorLabel       string          `json:"numeratorLabel,omitempty"`
	PercentAboveAverage  float64         `json:"percentAboveAverage,omitempty"`
	ReplacementRationale string          `json:"replacementRationale,omitempty"`
	Pos6h                float64         `json:"pos6h,omitempty"`
	Pos12h               float64         `json:"pos12h,omitempty"`
	Pos24h               float64         `json:"pos24h,omitempty"`
	PosLast              float64         `json:"posLast,omitempty"`
	Bookmarks            int             `json:"bookmarks,omitempty"`
	Likes                int             `json:"likes,omitempty"`
	Comments             int             `json:"comments,omitempty"`
	Replies              int             `json:"replies,omitempty"`
	Quotes               int             `json:"quotes,omitempty"`
	Views                int             `json:"views,omitempty"`
	ViewCount            int             `json:"viewCount,omitempty"`
	Impressions          int             `json:"impressions,omitempty"`
	Retweets             int             `json:"retweets,omitempty"`
	QuoteTweets          int             `json:"quoteTweets,omitempty"`
	SourceTitle          string          `json:"sourceTitle,omitempty"`
	HackerNews           json.RawMessage `json:"hackerNews,omitempty"`
	Techmeme             json.RawMessage `json:"techmeme,omitempty"`
	ExternalFeeds        json.RawMessage `json:"externalFeeds,omitempty"`
	Authors              []ClusterAuthor `json:"authors,omitempty"`
	TopAuthors           []ClusterAuthor `json:"topAuthors,omitempty"`
	ActivityAt           string          `json:"activityAt,omitempty"`
	ComputedAt           string          `json:"computedAt,omitempty"`
	FirstPostAt          string          `json:"firstPostAt,omitempty"`
	RawJSON              json.RawMessage `json:"-"`
}

// ClusterAuthor is one X account that contributed to a cluster.
type ClusterAuthor struct {
	Username      string  `json:"username,omitempty"`
	DisplayName   string  `json:"displayName,omitempty"`
	XID           string  `json:"xId,omitempty"`
	AvatarURL     string  `json:"avatarUrl,omitempty"`
	Influence     float64 `json:"influence,omitempty"`
	Podist        float64 `json:"podist,omitempty"`
	PostType      string  `json:"postType,omitempty"`
	PostXID       string  `json:"postXId,omitempty"`
	PostPermalink string  `json:"permalink,omitempty"`
}

// Event is one entry from /api/trending/status events[].
type Event struct {
	ID            string          `json:"id"`
	Type          string          `json:"type"`
	RunID         string          `json:"runId,omitempty"`
	ClusterID     string          `json:"clusterId,omitempty"`
	Label         string          `json:"label,omitempty"`
	Username      string          `json:"username,omitempty"`
	PostType      string          `json:"postType,omitempty"`
	PostXID       string          `json:"postXId,omitempty"`
	Permalink     string          `json:"permalink,omitempty"`
	Delta         int             `json:"delta,omitempty"`
	CurrentRank   int             `json:"currentRank,omitempty"`
	PreviousRank  int             `json:"previousRank,omitempty"`
	Count         int             `json:"count,omitempty"`
	Total         int             `json:"total,omitempty"`
	OriginalPosts int             `json:"originalPosts,omitempty"`
	Retweets      int             `json:"retweets,omitempty"`
	QuoteTweets   int             `json:"quoteTweets,omitempty"`
	Replies       int             `json:"replies,omitempty"`
	Links         int             `json:"links,omitempty"`
	Videos        int             `json:"videos,omitempty"`
	Images        int             `json:"images,omitempty"`
	EmbeddedCount int             `json:"embeddedCount,omitempty"`
	TotalCount    int             `json:"totalCount,omitempty"`
	At            string          `json:"at,omitempty"`
	CreatedAt     string          `json:"createdAt,omitempty"`
	DedupeKey     string          `json:"dedupeKey,omitempty"`
	RawJSON       json.RawMessage `json:"-"`
}

// TrendingStatus mirrors GET /api/trending/status.
type TrendingStatus struct {
	ComputedAt           string  `json:"computedAt,omitempty"`
	NextFetchAt          string  `json:"nextFetchAt,omitempty"`
	LastFetchCompletedAt string  `json:"lastFetchCompletedAt,omitempty"`
	IsFetching           bool    `json:"isFetching"`
	StoriesToday         int     `json:"storiesToday,omitempty"`
	ClustersToday        int     `json:"clustersToday,omitempty"`
	Events               []Event `json:"events,omitempty"`
}

// pushPattern matches: self.__next_f.push([1, "<js-string-literal>"])
// The captured group is the raw JS string literal contents (still escaped).
var pushPattern = regexp.MustCompile(`self\.__next_f\.push\(\[\s*\d+\s*,\s*"((?:[^"\\]|\\.)*)"\s*\]\)`)

// jsUnescape decodes a JS string literal body (without surrounding quotes).
// Handles \" \\ \/ \n \t \r \b \f \uXXXX. Unknown escapes preserve the
// following character (matches V8 behavior in non-strict mode well enough
// for Next.js's emitted RSC streams).
func jsUnescape(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		case '\\':
			b.WriteByte('\\')
		case '/':
			b.WriteByte('/')
		case 'u':
			if i+4 < len(s) {
				if r, err := strconv.ParseUint(s[i+1:i+5], 16, 32); err == nil {
					b.WriteRune(rune(r))
					i += 4
					continue
				}
			}
			b.WriteByte(s[i])
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// DecodeRSC reassembles the RSC stream from an HTML body and returns the
// concatenated unescaped payload. Callers then scan it with field-aware
// regexes.
func DecodeRSC(html []byte) string {
	matches := pushPattern.FindAllSubmatch(html, -1)
	var b strings.Builder
	for _, m := range matches {
		if len(m) > 1 {
			b.WriteString(jsUnescape(string(m[1])))
		}
	}
	return b.String()
}

// ExtractClusters walks the decoded RSC stream and returns every distinct
// cluster object found (deduplicated by clusterId). Earlier occurrences
// win when the same clusterId appears twice — Digg sometimes emits a
// cluster with sparse fields then a later occurrence with full data;
// we merge by preferring whichever object has more populated fields.
//
// Filters out non-cluster objects that happen to carry a clusterId
// (e.g. cluster_detected events): a real cluster must have either a
// clusterUrlId or a non-zero rank/title/tldr.
func ExtractClusters(decoded string) ([]Cluster, error) {
	objs := scanObjectsContaining(decoded, `"clusterId":"`)
	merged := make(map[string]Cluster)
	for _, raw := range objs {
		var c Cluster
		if err := json.Unmarshal(raw, &c); err != nil {
			continue
		}
		if c.ClusterID == "" {
			continue
		}
		// Normalize: Digg emits "rank" not "currentRank" in /ai feed objects.
		if c.CurrentRank == 0 && c.Rank > 0 {
			c.CurrentRank = c.Rank
		}
		// PATCH: Treat PeakRank=9999 as absent (Digg's sentinel for newly-
		// detected clusters with no tracked all-time peak). Normalize at
		// parse time so a sentinel occurrence does not block a later
		// occurrence's legitimate PeakRank via the first-non-zero-wins
		// merge guard. Mirrors the Rank → CurrentRank normalization above.
		if c.PeakRank == 9999 {
			c.PeakRank = 0
		}
		// Keep only objects that look like real clusters. cluster_detected
		// events also carry a clusterId, but they're events, not clusters.
		// Fold them into existing clusters if the cluster is already there;
		// otherwise drop.
		isRealCluster := c.ClusterURLID != "" || c.Title != "" || c.TLDR != "" || c.CurrentRank > 0 || len(c.Authors) > 0 || len(c.ScoreComponents) > 0
		c.RawJSON = append(c.RawJSON[:0:0], raw...)
		if existing, ok := merged[c.ClusterID]; ok {
			merged[c.ClusterID] = mergeClusters(existing, c)
		} else if isRealCluster {
			merged[c.ClusterID] = c
		}
	}
	out := make([]Cluster, 0, len(merged))
	for _, c := range merged {
		out = append(out, c)
	}
	return out, nil
}

// ExtractEvents finds every event-shaped object in the stream. Used for
// /api/trending/status JSON OR for cluster_detected/fast_climb events
// embedded in the /ai HTML stream.
func ExtractEvents(decoded string) ([]Event, error) {
	objs := scanObjectsContaining(decoded, `"dedupeKey":"`)
	out := make([]Event, 0, len(objs))
	seen := make(map[string]bool)
	for _, raw := range objs {
		var e Event
		if err := json.Unmarshal(raw, &e); err != nil {
			continue
		}
		key := e.ID
		if key == "" {
			key = e.DedupeKey
		}
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		e.RawJSON = append(e.RawJSON[:0:0], raw...)
		out = append(out, e)
	}
	return out, nil
}

// scanObjectsContaining walks s and returns the SMALLEST JSON object
// substring around each occurrence of `needle`. The clusters/events we
// care about are flat leaves nested inside an envelope (e.g.
// storiesByFilter.top.items[]); finding the outer envelope and skipping
// past it would lose every leaf, so we anchor by the needle position
// and walk left to find the enclosing object's opening brace.
func scanObjectsContaining(s, needle string) [][]byte {
	var out [][]byte
	pos := 0
	for {
		idx := strings.Index(s[pos:], needle)
		if idx < 0 {
			break
		}
		abs := pos + idx
		start := findEnclosingObjectStart(s, abs)
		if start < 0 {
			pos = abs + len(needle)
			continue
		}
		end := matchBalancedObject(s, start)
		if end < 0 {
			pos = abs + len(needle)
			continue
		}
		obj := s[start:end]
		out = append(out, []byte(obj))
		pos = end
	}
	return out
}

// findEnclosingObjectStart returns the index of the '{' that opens the
// innermost JSON object containing pos. Forward-scan with a stack of
// open-brace positions; aware of strings, nested objects, and escapes.
// Returns -1 if no enclosing object exists.
func findEnclosingObjectStart(s string, pos int) int {
	var stack []int
	inStr := false
	for i := 0; i < pos && i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			stack = append(stack, i)
		case '}':
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(stack) == 0 {
		return -1
	}
	return stack[len(stack)-1]
}

// matchBalancedObject returns the index AFTER the closing brace of the
// JSON object starting at s[start] (must be '{'). Returns -1 if no
// balanced match is found. Aware of strings, escapes, and nesting.
func matchBalancedObject(s string, start int) int {
	depth := 0
	inStr := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// mergeClusters returns the union of two cluster records observed for
// the same clusterId. Most fields use later-wins-when-nonzero, so b
// fills in or enriches values on a. The four rank-bearing fields
// (CurrentRank, PeakRank, PreviousRank, Delta) use first-non-zero-wins
// instead: a non-zero accumulator is preserved, and b only fills in
// when a is still zero. This keeps the main /ai leaderboard rank
// (emitted first in the RSC stream) from being clobbered by a later
// featured / weekly / pinned / sevenDaysStories section that assigns
// the same cluster a different rank.
func mergeClusters(a, b Cluster) Cluster {
	if b.ClusterURLID != "" {
		a.ClusterURLID = b.ClusterURLID
	}
	if b.Label != "" {
		a.Label = b.Label
	}
	if b.Title != "" {
		a.Title = b.Title
	}
	if b.TLDR != "" {
		a.TLDR = b.TLDR
	}
	if b.URL != "" {
		a.URL = b.URL
	}
	if b.Permalink != "" {
		a.Permalink = b.Permalink
	}
	if b.Topic != "" {
		a.Topic = b.Topic
	}
	// PATCH: rank-bearing fields use first-non-zero-wins precedence so later
	// cross-section occurrences (featured / weekly / pinned / sevenDaysStories
	// blocks in the RSC stream) cannot overwrite the main /ai leaderboard rank
	// for the same cluster. Zero accumulator still accepts a later non-zero
	// value so sparse-then-full enrichment is preserved.
	if a.CurrentRank == 0 && b.CurrentRank != 0 {
		a.CurrentRank = b.CurrentRank
	}
	if a.PeakRank == 0 && b.PeakRank != 0 {
		a.PeakRank = b.PeakRank
	}
	if a.PreviousRank == 0 && b.PreviousRank != 0 {
		a.PreviousRank = b.PreviousRank
	}
	if a.Delta == 0 && b.Delta != 0 {
		a.Delta = b.Delta
	}
	if b.GravityScore != 0 {
		a.GravityScore = b.GravityScore
	}
	if len(b.ScoreComponents) > 0 {
		a.ScoreComponents = b.ScoreComponents
	}
	if len(b.Evidence) > 0 {
		a.Evidence = b.Evidence
	}
	if b.ReplacementRationale != "" {
		a.ReplacementRationale = b.ReplacementRationale
	}
	if b.Pos6h != 0 {
		a.Pos6h = b.Pos6h
	}
	if b.Pos12h != 0 {
		a.Pos12h = b.Pos12h
	}
	if b.Pos24h != 0 {
		a.Pos24h = b.Pos24h
	}
	if b.PosLast != 0 {
		a.PosLast = b.PosLast
	}
	if b.Bookmarks != 0 {
		a.Bookmarks = b.Bookmarks
	}
	if b.Likes != 0 {
		a.Likes = b.Likes
	}
	if b.Views != 0 {
		a.Views = b.Views
	}
	if b.Retweets != 0 {
		a.Retweets = b.Retweets
	}
	if b.QuoteTweets != 0 {
		a.QuoteTweets = b.QuoteTweets
	}
	if b.SourceTitle != "" {
		a.SourceTitle = b.SourceTitle
	}
	if len(b.HackerNews) > 0 {
		a.HackerNews = b.HackerNews
	}
	if len(b.Techmeme) > 0 {
		a.Techmeme = b.Techmeme
	}
	if len(b.Authors) > 0 && len(a.Authors) == 0 {
		a.Authors = b.Authors
	}
	if len(b.TopAuthors) > 0 && len(a.TopAuthors) == 0 {
		a.TopAuthors = b.TopAuthors
	}
	if b.NumeratorLabel != "" {
		a.NumeratorLabel = b.NumeratorLabel
	}
	if b.PercentAboveAverage != 0 {
		a.PercentAboveAverage = b.PercentAboveAverage
	}
	if b.ActivityAt != "" {
		a.ActivityAt = b.ActivityAt
	}
	return a
}

// ParseHomeFeed is the convenience entry for the /ai page: decode RSC,
// extract clusters, and return them with the full RSC text for callers
// that want to extract more (e.g., events, top author records).
func ParseHomeFeed(html []byte) (clusters []Cluster, events []Event, decoded string, err error) {
	decoded = DecodeRSC(html)
	if decoded == "" {
		return nil, nil, "", fmt.Errorf("no RSC pushes found in HTML (%d bytes)", len(html))
	}
	clusters, err = ExtractClusters(decoded)
	if err != nil {
		return nil, nil, decoded, err
	}
	events, _ = ExtractEvents(decoded)
	return clusters, events, decoded, nil
}

// ParseTrendingStatus parses the /api/trending/status JSON response.
func ParseTrendingStatus(body []byte) (*TrendingStatus, error) {
	body = bytes.TrimSpace(body)
	var t TrendingStatus
	if err := json.Unmarshal(body, &t); err != nil {
		return nil, fmt.Errorf("parsing trending/status: %w", err)
	}
	return &t, nil
}

// ParseTime is a forgiving timestamp parser. Digg ships a mix of
// RFC3339 with and without sub-second precision and timezone variants.
// Returns zero time on parse failure rather than erroring.
func ParseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05.000000+00:00",
		"2006-01-02T15:04:05+00:00",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
