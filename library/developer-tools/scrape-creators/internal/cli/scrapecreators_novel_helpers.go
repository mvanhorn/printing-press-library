// Copyright 2026 Adrian Horning and contributors. Licensed under Apache-2.0. See LICENSE.
// Shared helpers for the hand-written Scrape Creators novel commands.
// Platform registries and tolerant JSON extractors keep the eight novel
// commands (creator find/compare/track, content spikes, trends triangulate,
// ads monitor, transcripts search, account budget) consistent.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/scrape-creators/internal/client"
	"github.com/spf13/cobra"
)

// novelSubRequestTimeout caps a single fan-out sub-request so one slow platform
// cannot stall an aggregating command. context.WithTimeout already clamps to
// the parent boundCtx deadline, so the effective bound is min(8s, remaining).
const novelSubRequestTimeout = 8 * time.Second

// subRequestCtx derives a per-sub-request child context from the command's
// bounded parent. Each goroutine in a parallel fan-out gets its own so a single
// hung call is abandoned without failing its siblings.
func subRequestCtx(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, novelSubRequestTimeout)
}

// isNotFoundErr reports whether a client error is an HTTP 404 — a real
// "handle/resource absent" signal that fan-out commands treat as a negative
// result rather than a fetch failure. It matches on the typed APIError status
// code rather than the error string, so a future change to the client's error
// format cannot silently turn every 404 into a fetch failure.
func isNotFoundErr(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 404
	}
	return false
}

// isErrorEnvelope reports whether a 200 response body is actually a Scrape
// Creators error envelope (`{"success": false, ...}`). The API returns HTTP 200
// with this shape for some "not found"/upstream-down cases, so presence checks
// must not treat a non-empty body as a real profile.
func isErrorEnvelope(data json.RawMessage) bool {
	var env struct {
		Success *bool `json:"success"`
	}
	if err := json.Unmarshal(data, &env); err != nil {
		return false
	}
	return env.Success != nil && !*env.Success
}

// sanitizeFetchErr renders a client error for inclusion in machine output. The
// client already masks the API key in its error text, so the message is safe.
func sanitizeFetchErr(err error) string {
	if err == nil {
		return ""
	}
	return truncate(strings.ReplaceAll(err.Error(), "\n", " "), 200)
}

// warnFetchFailures prints a one-line stderr warning naming how many fan-out
// fetches failed so a partial aggregate is never silently treated as complete.
func warnFetchFailures(cmd *cobra.Command, label string, failures []fetchFailure) {
	if len(failures) == 0 {
		return
	}
	names := make([]string, 0, len(failures))
	for _, f := range failures {
		names = append(names, f.Source)
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s: %d fetch(es) failed (%s); excluded from totals\n",
		label, len(failures), strings.Join(names, ", "))
}

// novelWantsMachine reports whether a novel command should emit JSON rather
// than a human table: explicit --json/--agent, or a non-terminal stdout that
// did not ask for another explicit text format.
func novelWantsMachine(w io.Writer, flags *rootFlags) bool {
	if flags.asJSON || flags.agent {
		return true
	}
	return !isTerminal(w) && !flags.csv && !flags.quiet && !flags.plain
}

// creatorPlatform describes a platform whose public profile endpoint accepts a
// bare handle. Only handle-addressable creator platforms belong here; platforms
// whose profile endpoint requires a full URL (facebook, linkedin) are omitted
// because creator find takes a handle, not a URL.
type creatorPlatform struct {
	name        string
	profilePath string
	handleParam string
}

// creatorPlatforms is the curated registry used by creator find/compare/track.
// Each path + handle param was confirmed against the live API and the spec.
// pinterest uses /v1/pinterest/user/boards (its only handle-addressable
// endpoint); it still carries a follower count, so it doubles as an existence
// probe. facebook and linkedin are intentionally excluded: their profile
// endpoints require a full profile URL, not a handle.
var creatorPlatforms = []creatorPlatform{
	{name: "tiktok", profilePath: "/v1/tiktok/profile", handleParam: "handle"},
	{name: "instagram", profilePath: "/v1/instagram/profile", handleParam: "handle"},
	{name: "youtube", profilePath: "/v1/youtube/channel", handleParam: "handle"},
	{name: "twitter", profilePath: "/v1/twitter/profile", handleParam: "handle"},
	{name: "threads", profilePath: "/v1/threads/profile", handleParam: "handle"},
	{name: "bluesky", profilePath: "/v1/bluesky/profile", handleParam: "handle"},
	{name: "pinterest", profilePath: "/v1/pinterest/user/boards", handleParam: "handle"},
	{name: "twitch", profilePath: "/v1/twitch/profile", handleParam: "handle"},
	{name: "truthsocial", profilePath: "/v1/truthsocial/profile", handleParam: "handle"},
	{name: "snapchat", profilePath: "/v1/snapchat/profile", handleParam: "handle"},
	{name: "kwai", profilePath: "/v1/kwai/profile", handleParam: "handle"},
	{name: "github", profilePath: "/v1/github/user", handleParam: "handle"},
}

// creatorPlatformByName resolves a registry entry by platform name.
func creatorPlatformByName(name string) (creatorPlatform, bool) {
	for _, p := range creatorPlatforms {
		if p.name == name {
			return p, true
		}
	}
	return creatorPlatform{}, false
}

// contentSource maps a platform to its recent-content endpoint for content
// spikes. arrayKey names the response field holding the content list; when it
// is absent the largest top-level array is used as a fallback.
type contentSource struct {
	name        string
	path        string
	handleParam string
	arrayKey    string
}

var contentSources = map[string]contentSource{
	"tiktok":    {name: "tiktok", path: "/v3/tiktok/profile/videos", handleParam: "handle", arrayKey: "aweme_list"},
	"youtube":   {name: "youtube", path: "/v1/youtube/channel-videos", handleParam: "handle", arrayKey: "videos"},
	"instagram": {name: "instagram", path: "/v1/instagram/user/reels", handleParam: "handle", arrayKey: "items"},
}

// searchSource maps a platform to a keyword search endpoint used by trends
// triangulate. resultKey names the primary result array; count of that array
// is the velocity proxy.
type searchSource struct {
	name       string
	path       string
	queryParam string
	resultKey  string
}

var searchSources = []searchSource{
	{name: "tiktok", path: "/v1/tiktok/search/keyword", queryParam: "query", resultKey: "search_item_list"},
	{name: "youtube", path: "/v1/youtube/search", queryParam: "query", resultKey: "videos"},
	{name: "reddit", path: "/v1/reddit/search", queryParam: "query", resultKey: "posts"},
	{name: "threads", path: "/v1/threads/search", queryParam: "query", resultKey: "posts"},
	{name: "rumble", path: "/v1/rumble/search", queryParam: "query", resultKey: "videos"},
}

// adNetwork maps an ad library to its brand search endpoint for ads monitor.
// queryParam carries the brand term; resultKey names the ad list; idField is
// the per-ad identifier used to diff snapshots across runs.
type adNetwork struct {
	name       string
	path       string
	queryParam string
	resultKey  string
	idField    string
}

var adNetworks = []adNetwork{
	{name: "facebook", path: "/v1/facebook/adLibrary/search/ads", queryParam: "query", resultKey: "searchResults", idField: "ad_archive_id"},
	{name: "tiktok", path: "/v1/tiktok/ad-library/search", queryParam: "query", resultKey: "ads", idField: "id"},
	{name: "google", path: "/v1/google/adLibrary/advertisers/search", queryParam: "query", resultKey: "advertisers", idField: "advertiser_id"},
	{name: "linkedin", path: "/v1/linkedin/ads/search", queryParam: "keyword", resultKey: "ads", idField: "id"},
}

// transcriptResourceTypes are the kebab-case resource_type values that
// transcript syncs write into the generic resources table. transcripts search
// restricts its FTS query to these so a search never returns profile or video
// rows.
var transcriptResourceTypes = []string{
	"tiktok-video-transcript",
	"youtube-video-transcript",
	"instagram-media-transcript",
	"facebook-post-transcript",
	"linkedin-post-transcript",
	"rumble-video-transcript",
	"twitter-tweet-transcript",
	"reddit-post-transcript",
	"facebook-ad-library-ad-transcript",
}

// fetchFailure records a single failed fan-out fetch so aggregating commands
// can exclude it from totals and surface it in the JSON envelope rather than
// letting it become a zero-valued phantom row.
type fetchFailure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}

// normalizeKey lowercases a JSON key and strips underscores so follower_count,
// followerCount, and followersCount all compare equal.
func normalizeKey(k string) string {
	return strings.ReplaceAll(strings.ToLower(k), "_", "")
}

// followerKeys are the normalized key names that hold a follower/subscriber
// total across the supported platforms. following/follows keys deliberately
// fall outside this set.
var followerKeys = map[string]bool{
	"followercount":   true, // tiktok followerCount, threads follower_count
	"followerscount":  true, // bluesky followersCount, twitter followers_count
	"subscribercount": true, // youtube subscriberCount
	"fancount":        true, // facebook fan_count
	"followers":       true, // github, twitch
}

var viewKeys = map[string]bool{
	"playcount": true, "viewcount": true, "views": true, "plays": true,
}

var likeKeys = map[string]bool{
	"diggcount": true, "likecount": true, "likes": true, "favoritecount": true, "reactioncount": true,
}

var commentKeys = map[string]bool{
	"commentcount": true, "comments": true,
}

var shareKeys = map[string]bool{
	"sharecount": true, "shares": true, "repostcount": true,
}

// toInt64 coerces a decoded JSON scalar (number or numeric string) to int64.
func toInt64(v any) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return n, true
		}
		if f, err := t.Float64(); err == nil {
			return int64(f), true
		}
	case int64:
		return t, true
	case int:
		return int64(t), true
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return int64(f), true
		}
	}
	return 0, false
}

func sortedKeys(m map[string]any) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

// extractFollowerCount recursively walks decoded API JSON for the first
// plausible follower/subscriber total. It special-cases Instagram's
// edge_followed_by.count nesting, then matches followerKeys, then recurses in
// deterministic key order.
func extractFollowerCount(data json.RawMessage) (int64, bool) {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return 0, false
	}
	return walkFollower(v)
}

func walkFollower(v any) (int64, bool) {
	switch t := v.(type) {
	case map[string]any:
		if efb, ok := t["edge_followed_by"].(map[string]any); ok {
			if n, ok := toInt64(efb["count"]); ok {
				return n, true
			}
		}
		for k, val := range t {
			if followerKeys[normalizeKey(k)] {
				if n, ok := toInt64(val); ok {
					return n, true
				}
			}
		}
		for _, k := range sortedKeys(t) {
			if n, ok := walkFollower(t[k]); ok {
				return n, true
			}
		}
	case []any:
		for _, el := range t {
			if n, ok := walkFollower(el); ok {
				return n, true
			}
		}
	}
	return 0, false
}

// walkNumericKey recursively finds the first numeric value whose key (after
// normalization) is in keyset.
func walkNumericKey(v any, keyset map[string]bool) (int64, bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if keyset[normalizeKey(k)] {
				if n, ok := toInt64(val); ok {
					return n, true
				}
			}
		}
		for _, k := range sortedKeys(t) {
			if n, ok := walkNumericKey(t[k], keyset); ok {
				return n, true
			}
		}
	case []any:
		for _, el := range t {
			if n, ok := walkNumericKey(el, keyset); ok {
				return n, true
			}
		}
	}
	return 0, false
}

// contentMetrics holds the per-item engagement signals content spikes and
// creator compare derive from a single piece of content.
type contentMetrics struct {
	views    int64
	likes    int64
	comments int64
	shares   int64
}

func (m contentMetrics) engagement() int64 { return m.likes + m.comments + m.shares }

// extractContentMetrics pulls view/like/comment/share counts from one content
// item, tolerant of nesting (e.g. TikTok's statistics object).
func extractContentMetrics(item json.RawMessage) contentMetrics {
	var v any
	dec := json.NewDecoder(strings.NewReader(string(item)))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return contentMetrics{}
	}
	views, _ := walkNumericKey(v, viewKeys)
	likes, _ := walkNumericKey(v, likeKeys)
	comments, _ := walkNumericKey(v, commentKeys)
	shares, _ := walkNumericKey(v, shareKeys)
	return contentMetrics{views: views, likes: likes, comments: comments, shares: shares}
}

// walkStringKey recursively finds the first non-empty string whose key (after
// normalization) is in keyset.
func walkStringKey(v any, keyset map[string]bool) (string, bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if keyset[normalizeKey(k)] {
				if s, ok := val.(string); ok && strings.TrimSpace(s) != "" {
					return s, true
				}
			}
		}
		for _, k := range sortedKeys(t) {
			if s, ok := walkStringKey(t[k], keyset); ok {
				return s, true
			}
		}
	case []any:
		for _, el := range t {
			if s, ok := walkStringKey(el, keyset); ok {
				return s, true
			}
		}
	}
	return "", false
}

func extractString(data json.RawMessage, keyset map[string]bool) string {
	var v any
	if json.Unmarshal(data, &v) != nil {
		return ""
	}
	s, _ := walkStringKey(v, keyset)
	return s
}

var creatorNameKeys = map[string]bool{
	"handle": true, "username": true, "author": true, "nickname": true, "screenname": true, "name": true,
}

var transcriptTextKeys = map[string]bool{
	"transcript": true, "text": true, "content": true, "caption": true, "body": true,
}

// platformFromResourceType maps a transcript resource_type to its platform
// label, e.g. "tiktok-video-transcript" -> "tiktok".
func platformFromResourceType(rt string) string {
	if i := strings.IndexByte(rt, '-'); i > 0 {
		return rt[:i]
	}
	return rt
}

// resultArray returns the JSON array under key, falling back to the largest
// top-level array when key is absent, then to a top-level array body. A nil
// return (len 0) means "no results", never an error.
func resultArray(data json.RawMessage, key string) []json.RawMessage {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		if raw, ok := obj[key]; ok {
			var arr []json.RawMessage
			if json.Unmarshal(raw, &arr) == nil {
				return arr
			}
		}
		var best []json.RawMessage
		for _, raw := range obj {
			var arr []json.RawMessage
			if json.Unmarshal(raw, &arr) == nil && len(arr) > len(best) {
				best = arr
			}
		}
		return best
	}
	var arr []json.RawMessage
	if json.Unmarshal(data, &arr) == nil {
		return arr
	}
	return nil
}

// firstStringField returns the first non-empty string value among keys in a
// decoded JSON object, coercing numbers to their string form.
func firstStringField(m map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case string:
			if t != "" {
				return t
			}
		case float64:
			return strconv.FormatInt(int64(t), 10)
		case json.Number:
			return t.String()
		}
	}
	return ""
}

// extractItemID returns a stable identifier for one result item, preferring
// idField then a set of common id/url fallbacks.
func extractItemID(item json.RawMessage, idField string) string {
	var m map[string]any
	if json.Unmarshal(item, &m) != nil {
		return ""
	}
	if id := firstStringField(m, idField); id != "" {
		return id
	}
	return firstStringField(m, "id", "ad_archive_id", "advertiser_id", "aweme_id", "url", "share_url")
}
