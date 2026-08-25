// Copyright 2026 Kerry Morrison and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// Local-store record shapes for the internal/gtrends-backed novel commands
// (trends interest/region/related/trending, and the 7 novel read commands
// built on top of them). Each type is used for BOTH writing (the Priority-1
// wrapper commands persist these via store.Upsert) and reading (the
// Priority-2 commands decode resources.data back into the same struct) so a
// field-name drift between writer and reader is a compile error, not a
// silent runtime data-loss bug.

// gtInterestPointRecord is the gt_interest_point resource: one
// (keyword, geo, date) interest-over-time value. Written by `trends
// interest`; read by `trends changes`, `trends opportunities`, and `trends
// seasonality`.
type gtInterestPointRecord struct {
	Keyword      string `json:"keyword"`
	Geo          string `json:"geo"`
	Timeframe    string `json:"timeframe"`
	Date         string `json:"date"`
	Value        int    `json:"value"`
	Category     int    `json:"category"`
	Property     string `json:"property"`
	CompareScope string `json:"compare_scope"`
	SyncedAt     string `json:"synced_at"`
}

// gtRegionInterestRecord is the gt_region_interest resource. Written by
// `trends region`.
type gtRegionInterestRecord struct {
	Keyword   string `json:"keyword"`
	GeoCode   string `json:"geo_code"`
	GeoName   string `json:"geo_name"`
	Timeframe string `json:"timeframe"`
	Value     int    `json:"value"`
	SyncedAt  string `json:"synced_at"`
}

// gtRelatedTermRecord is the gt_related_term resource. Written by `trends
// related`; read by `trends changes`, `trends opportunities`, and `trends
// history search`.
type gtRelatedTermRecord struct {
	Keyword    string `json:"keyword"`
	Term       string `json:"term"`
	Kind       string `json:"kind"`  // "top" | "rising"
	Facet      string `json:"facet"` // "query" | "topic"
	Value      int    `json:"value"`
	IsBreakout bool   `json:"is_breakout"`
	Geo        string `json:"geo"`
	Timeframe  string `json:"timeframe"`
	Category   int    `json:"category"`
	SyncedAt   string `json:"synced_at"`
}

// gtKeywordQueryRecord is the gt_keyword_query resource: one row per tracked
// (keyword, geo, category, property, compare_scope) pair, updated on every
// `trends interest` call. Read by `trends stale`.
type gtKeywordQueryRecord struct {
	Keyword      string `json:"keyword"`
	Geo          string `json:"geo"`
	Timeframe    string `json:"timeframe"`
	Category     int    `json:"category"`
	Property     string `json:"property"`
	CompareScope string `json:"compare_scope"`
	LastSyncedAt string `json:"last_synced_at"`
}

// gtTrendingTopicRecord is the gt_trending_topic resource. Written by
// `trends trending`; read by `trends trending at` and `trends history
// search`.
type gtTrendingTopicRecord struct {
	Date     string `json:"date"`
	Geo      string `json:"geo"`
	Term     string `json:"term"`
	Rank     int    `json:"rank"`
	SyncedAt string `json:"synced_at"`
}
