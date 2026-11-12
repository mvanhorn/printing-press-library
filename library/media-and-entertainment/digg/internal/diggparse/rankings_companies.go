// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): library-side new file. Parses
// the ranking Digg embeds in the RSC stream of its rankings page, which
// the generator has no spec for.
//
// https://digg.com/tech/x/rankings ranks people and companies together
// and ships one shape: a direction-less {"entries":[...]} wrapper
// (two copies of it, about 1000 rows). The page used to carry Emerging
// and Movers sections too, each in a wrapper with a `direction` key;
// Digg removed them in October 2026 and ExtractMainRanking still skips
// any wrapper that has one.
//
// Each entry is a CompanyEntry, a strict superset of Roster1000Author
// (same id/rank/categoryRank/bio/vibeDistribution plus follower-velocity
// and "is curated" flags).
//
// Wrapper selection is by *prop semantics*, never by the Next.js
// component ID ($L39 / $L3c / $L3d in one snapshot) — those IDs are
// regenerated on every deploy and would silently break the parser.

package diggparse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// CompanyEntry is one row from the rankings page. Mirrors the
// fields the upstream payload emits today; nullable JSON values are
// typed as pointers so the JSON null vs zero-value distinction is
// preserved for downstream consumers.
type CompanyEntry struct {
	Rank               int                `json:"rank"`
	TargetXID          string             `json:"target_x_id,omitempty"`
	FollowedByCount    int                `json:"followed_by_count"`
	Score              float64            `json:"score"`
	Username           string             `json:"username"`
	DisplayName        string             `json:"display_name,omitempty"`
	ProfileImageURL    string             `json:"profile_image_url,omitempty"`
	FollowersCount     int                `json:"followers_count"`
	Bio                string             `json:"bio,omitempty"`
	Category           string             `json:"category,omitempty"`
	CategoryConfidence *float64           `json:"categoryConfidence"`
	GithubURL          *string            `json:"githubUrl"`
	PreviousRank       *int               `json:"previousRank"`
	RankChange         *int               `json:"rankChange"`
	CategoryRank       int                `json:"categoryRank,omitempty"`
	VibeDistribution   map[string]float64 `json:"vibeDistribution,omitempty"`
	VibeTweetCount     int                `json:"vibeTweetCount,omitempty"`

	// Follower-velocity / curated-list signals — present on the
	// rankings payload, absent from the roster payload.
	FollowCountChange  *int    `json:"followCountChange"`
	IsNewEntrant       bool    `json:"isNewEntrant,omitempty"`
	IsEmergingStartup  bool    `json:"isEmergingStartup,omitempty"`
	EmergingReasoning  *string `json:"emergingReasoning"`
	ClassificationTLDR *string `json:"classificationTldr"`

	// RawJSON keeps the original record substring so callers needing a
	// field we didn't surface can decode it themselves. Not exported as
	// JSON; lives solely for round-trip access.
	RawJSON json.RawMessage `json:"-"`
}

// ErrSectionNotFound signals that the ranking's wrapper
// could not be located in the decoded RSC stream. Callers should treat
// this as "page shape may have changed" (distinct from "section was
// present but had zero entries", which returns an empty slice + nil
// error).
var ErrSectionNotFound = errors.New("section wrapper not found in RSC stream")

// Wrapper shape around an entries array. Direction is set only on the
// section wrappers Digg has since removed.
type sectionWrapper struct {
	Direction *string         `json:"direction"`
	Entries   json.RawMessage `json:"entries"`
}

// ExtractMainRanking returns the full ranking from the page. It picks
// the ranking's wrapper by two anchors:
//
//  1. The wrapper has no `direction` sibling key (the removed Emerging
//     and Movers wrappers always had one).
//  2. The first non-RSC-reference entry shape-matches a CompanyEntry
//     (has both `target_x_id` and `rank` keys). This guards against
//     future direction-less `entries` arrays on the same page —
//     "recommended", pagination caches, navigation chunks — that
//     happen to be larger than the main ranking.
//
// When multiple candidates remain after both anchors, the one with the
// most entries wins. Ties pick the first occurrence (RSC-flatstream
// order); document this as the tie-break behavior so consumers don't
// rely on the larger one when sizes happen to match.
func ExtractMainRanking(decoded string) ([]CompanyEntry, ParseStats, error) {
	objs := scanObjectsContaining(decoded, `"entries":[`)
	var best json.RawMessage
	bestCount := -1
	for _, raw := range objs {
		var w sectionWrapper
		if err := json.Unmarshal(raw, &w); err != nil {
			// Some matches are entries arrays inside larger objects
			// where the smallest enclosing object isn't a wrapper
			// (e.g., a parent React element). Skip silently.
			continue
		}
		if w.Direction != nil {
			continue // A section wrapper, not the ranking.
		}
		if len(w.Entries) == 0 {
			continue
		}
		var arr []json.RawMessage
		if err := json.Unmarshal(w.Entries, &arr); err != nil {
			continue
		}
		if !looksLikeCompanyEntriesArray(arr) {
			continue
		}
		if len(arr) > bestCount {
			bestCount = len(arr)
			best = w.Entries
		}
	}
	if best == nil {
		return nil, ParseStats{}, fmt.Errorf("main: %w", ErrSectionNotFound)
	}
	entries, stats := decodeCompanyEntries(best)
	return entries, stats, nil
}

// looksLikeCompanyEntriesArray heuristically validates that an entries
// array is a company ranking (vs. some other React component's
// `entries` prop). Skips RSC references when probing; requires the
// first non-reference element to be a JSON object that contains both
// `"target_x_id"` and `"rank"` substrings. Cheap byte-level check —
// avoids fully decoding every item just to classify the array.
func looksLikeCompanyEntriesArray(arr []json.RawMessage) bool {
	for _, r := range arr {
		if isRSCReference(r) {
			continue
		}
		probe := bytes.TrimSpace(r)
		if len(probe) == 0 || probe[0] != '{' {
			return false
		}
		return bytes.Contains(probe, []byte(`"target_x_id"`)) &&
			bytes.Contains(probe, []byte(`"rank"`))
	}
	return false
}

// decodeCompanyEntries JSON-decodes an entries array and returns the
// parsed slice plus a ParseStats.
//
// Counting rules (the unifying invariant: every array slot that *could*
// have been a company is either Decoded or Skipped, so SkipRatio is a
// real fraction of "lost entries"):
//
//   - RSC references (Next.js placeholders like
//     `"$3a:props:left:entries:1"`) are skipped silently AND do not
//     count toward Attempted. They are a stable RSC dedup feature: an
//     entry already inlined elsewhere in the stream is referred to
//     instead of repeated.
//
//   - json.Unmarshal failure on the entries array as a whole → 1
//     Skipped attempt (handles "schema renamed the entries key").
//
//   - json.Unmarshal failure on a single entry → 1 Skipped attempt
//     (handles "rank became a string" / "field type changed").
//
//   - Decoded entry with missing username or non-positive rank → 1
//     Skipped attempt. This is the critical case: if upstream renames
//     `username` → `user_handle`, every entry decodes cleanly but with
//     empty Username, and we'd otherwise silently emit nothing.
//     Treating it as Skipped makes SkipRatio surface the drift.
//
//   - Decoded valid entry → 1 Decoded.
func decodeCompanyEntries(raw json.RawMessage) ([]CompanyEntry, ParseStats) {
	var stats ParseStats
	var rawEntries []json.RawMessage
	if err := json.Unmarshal(raw, &rawEntries); err != nil {
		stats.Add(fmt.Errorf("entries array: %w", err))
		return nil, stats
	}
	out := make([]CompanyEntry, 0, len(rawEntries))
	for _, r := range rawEntries {
		if isRSCReference(r) {
			continue
		}
		var entry CompanyEntry
		if err := json.Unmarshal(r, &entry); err != nil {
			stats.Add(err)
			continue
		}
		if entry.Username == "" || entry.Rank <= 0 {
			stats.Add(fmt.Errorf(
				"entry missing required fields (username=%q rank=%d) — possible schema rename",
				entry.Username, entry.Rank))
			continue
		}
		stats.Add(nil)
		entry.RawJSON = append(entry.RawJSON[:0:0], r...)
		out = append(out, entry)
	}
	return out, stats
}

// isRSCReference reports whether a JSON value is a Next.js React Server
// Component reference string (looks like "$ID:path:..." — a quoted
// string whose first non-quote character is '$'). The RSC reference
// system uses this form to point at already-rendered nodes elsewhere
// in the stream; the rankings page emits them inside `entries` arrays
// to reference an entry without duplicating the underlying object.
//
// Trade-off: this WILL also match any legitimate string entry whose
// value starts with '$'. In the company-rankings entries arrays the
// only string-valued elements observed are RSC references; if a
// future schema starts emitting other strings beginning with '$' we'd
// drop them here. The risk is acceptable because the alternative
// (decoding to confirm reference shape) is heavier and the false
// positive surface is narrow.
func isRSCReference(r json.RawMessage) bool {
	trimmed := bytes.TrimSpace(r)
	return len(trimmed) >= 2 && trimmed[0] == '"' && trimmed[1] == '$'
}
