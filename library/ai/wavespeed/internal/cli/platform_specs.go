// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"
	"strings"
)

// platformFormat is one renderable format for a social platform (feed image,
// story, reel/video). DurationCapSec is 0 for still images. CaptionHint and
// HashtagSlots feed the per-platform manifest a downstream posting tool reads.
type platformFormat struct {
	Format         string `json:"format"` // feed | story | reel | video
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	AspectRatio    string `json:"aspect_ratio"`
	DurationCapSec int    `json:"duration_cap_sec"`
	IsVideo        bool   `json:"is_video"`
	CaptionHint    string `json:"caption_hint"`
	HashtagSlots   int    `json:"hashtag_slots"`
}

// platformSpec is the full format table for one platform plus its default
// format (what `pack` produces when no explicit format/aspect is requested).
type platformSpec struct {
	Platform      string           `json:"platform"`
	DefaultFormat string           `json:"default_format"`
	Formats       []platformFormat `json:"formats"`
}

// platformSpecs is the source of truth for per-platform request shapes. It is
// consumed by pack (output sizing + manifest), qa preflight (request-shape
// validation), and the inspection-light dimension validator.
var platformSpecs = map[string]platformSpec{
	"instagram": {
		Platform:      "instagram",
		DefaultFormat: "feed",
		Formats: []platformFormat{
			{Format: "feed", Width: 1080, Height: 1350, AspectRatio: "4:5", CaptionHint: "<=2200 chars, hook in first line", HashtagSlots: 30},
			{Format: "story", Width: 1080, Height: 1920, AspectRatio: "9:16", DurationCapSec: 60, IsVideo: true, CaptionHint: "sticker-friendly, minimal text", HashtagSlots: 10},
			{Format: "reel", Width: 1080, Height: 1920, AspectRatio: "9:16", DurationCapSec: 90, IsVideo: true, CaptionHint: "<=2200 chars, strong hook", HashtagSlots: 30},
		},
	},
	"tiktok": {
		Platform:      "tiktok",
		DefaultFormat: "video",
		Formats: []platformFormat{
			{Format: "video", Width: 1080, Height: 1920, AspectRatio: "9:16", DurationCapSec: 180, IsVideo: true, CaptionHint: "<=2200 chars, trend-aware hook", HashtagSlots: 5},
		},
	},
	"facebook": {
		Platform:      "facebook",
		DefaultFormat: "feed",
		Formats: []platformFormat{
			{Format: "feed", Width: 1080, Height: 1080, AspectRatio: "1:1", CaptionHint: "<=63206 chars, front-load value", HashtagSlots: 5},
			{Format: "story", Width: 1080, Height: 1920, AspectRatio: "9:16", DurationCapSec: 60, IsVideo: true, CaptionHint: "minimal text overlay", HashtagSlots: 5},
		},
	},
	"x": {
		Platform:      "x",
		DefaultFormat: "feed",
		Formats: []platformFormat{
			{Format: "feed", Width: 1600, Height: 900, AspectRatio: "16:9", CaptionHint: "<=280 chars", HashtagSlots: 3},
			{Format: "video", Width: 1280, Height: 720, AspectRatio: "16:9", DurationCapSec: 140, IsVideo: true, CaptionHint: "<=280 chars", HashtagSlots: 3},
		},
	},
}

// lookupPlatform returns the spec for a platform name (case-insensitive).
func lookupPlatform(name string) (platformSpec, bool) {
	spec, ok := platformSpecs[strings.ToLower(strings.TrimSpace(name))]
	return spec, ok
}

// knownPlatforms returns the sorted list of supported platform names.
func knownPlatforms() []string {
	out := make([]string, 0, len(platformSpecs))
	for k := range platformSpecs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// defaultFormatFor returns a platform's default format descriptor.
func defaultFormatFor(platform string) (platformFormat, bool) {
	spec, ok := lookupPlatform(platform)
	if !ok {
		return platformFormat{}, false
	}
	return formatFor(spec, spec.DefaultFormat)
}

// formatFor finds a named format within a spec.
func formatFor(spec platformSpec, format string) (platformFormat, bool) {
	format = strings.ToLower(strings.TrimSpace(format))
	if format == "" {
		format = spec.DefaultFormat
	}
	for _, f := range spec.Formats {
		if f.Format == format {
			return f, true
		}
	}
	return platformFormat{}, false
}
