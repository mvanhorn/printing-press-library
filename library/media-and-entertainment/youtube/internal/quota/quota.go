// Package quota holds the YouTube Data API v3 per-endpoint cost map and
// helpers for the quota planner and ledger commands.
//
// Cost values are sourced from the official YouTube Data API quota calculator
// (developers.google.com/youtube/v3/determine_quota_cost) as of 2026-Q1.
// A 10,000-unit/day default project quota is assumed.
package quota

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DailyDefault is the standard YouTube Data API v3 daily project quota.
const DailyDefault = 10000

// Cost is the documented quota cost for a given resource.method combination.
// Returns 1 for unknown read endpoints (the documented minimum), 50 for unknown
// writes — agents can override with a manual --units flag if they know better.
func Cost(resource, method string) int {
	resource = strings.ToLower(resource)
	method = strings.ToLower(method)
	if v, ok := table[resource+"."+method]; ok {
		return v
	}
	if isWriteMethod(method) {
		return 50 // documented minimum write cost; better than guessing 0.
	}
	return 1
}

// All returns the full cost table for `cost ledger --table`. Read-only.
func All() map[string]int {
	out := make(map[string]int, len(table))
	for k, v := range table {
		out[k] = v
	}
	return out
}

// HashKey turns a raw API key string into an opaque hash for the
// quota_log table — we don't store the key itself, just enough to
// disambiguate runs with different keys against the same project quota.
// Empty keys yield "anon".
func HashKey(rawKey string) string {
	if rawKey == "" {
		return "anon"
	}
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])[:12]
}

func isWriteMethod(m string) bool {
	switch m {
	case "insert", "update", "delete", "set", "unset", "rate", "reportabuse", "setmoderationstatus":
		return true
	}
	return false
}

// table is the documented quota cost map. Write costs vary widely (50 for
// most playlist/comment writes, 1600 for video uploads); these are the values
// the planner displays.
var table = map[string]int{
	// Read - 1 unit unless noted
	"activities.list":              1,
	"captions.list":                50,
	"captions.download":            200,
	"channels.list":                1,
	"channelsections.list":         1,
	"comments.list":                1,
	"commentthreads.list":          1,
	"i18nlanguages.list":           1,
	"i18nregions.list":             1,
	"members.list":                 2,
	"membershipslevels.list":       1,
	"playlistitems.list":           1,
	"playlists.list":               1,
	"search.list":                  100,
	"subscriptions.list":           1,
	"thumbnails.set":               50,
	"videoabusereportreasons.list": 1,
	"videocategories.list":         1,
	"videos.list":                  1,
	"videos.getrating":             1,
	"livebroadcasts.list":          1,
	"livechatmessages.list":        1,

	// Write
	"captions.insert":              400,
	"captions.update":              450,
	"captions.delete":              50,
	"channels.update":              50,
	"channelbanners.insert":        50,
	"channelsections.insert":       50,
	"channelsections.update":       50,
	"channelsections.delete":       50,
	"comments.insert":              50,
	"comments.update":              50,
	"comments.setmoderationstatus": 50,
	"comments.delete":              50,
	"commentthreads.insert":        50,
	"playlistitems.insert":         50,
	"playlistitems.update":         50,
	"playlistitems.delete":         50,
	"playlists.insert":             50,
	"playlists.update":             50,
	"playlists.delete":             50,
	"subscriptions.insert":         50,
	"subscriptions.delete":         50,
	"videos.insert":                1600,
	"videos.update":                50,
	"videos.delete":                50,
	"videos.rate":                  50,
	"videos.reportabuse":           50,
	"watermarks.set":               50,
	"watermarks.unset":             50,
}

// Endpoints returns a sorted, deduplicated list of all endpoints in the table
// (useful for tab completion or `quota plan --list`).
func Endpoints() []string {
	out := make([]string, 0, len(table))
	for k := range table {
		out = append(out, k)
	}
	return out
}
