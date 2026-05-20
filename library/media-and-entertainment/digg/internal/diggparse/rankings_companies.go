// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.
//
// PATCH(digg-rankings-and-min-starrers): library-side new file. The
// /ai/x/rankings/companies page ships three distinct ranking slices in
// its embedded RSC stream and the generator has no spec for it yet.
//
// Sections, identified by the JSON wrapper around their `entries` array:
//   - Emerging: {"direction":"emerging","entries":[...]}
//   - Movers:   {"left":{"direction":"up","entries":[...]},
//                "right":{"direction":"down","entries":[...]}}
//   - Main:     {"entries":[...]}    (no `direction` sibling key)
//
// Each entry is a CompanyEntry, a strict superset of Roster1000Author
// (same id/rank/categoryRank/bio/vibeDistribution plus follower-velocity
// and "is curated" flags). For Movers entries the extractor stamps a
// Direction field so callers can distinguish up/down without re-parsing.
//
// Wrapper selection is by *prop semantics*, never by the Next.js
// component ID ($L39 / $L3c / $L3d in today's snapshot) — those IDs are
// regenerated on every deploy and would silently break the parser.

package diggparse

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// CompanyEntry is one row from /ai/x/rankings/companies. Mirrors the
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
	// rankings/companies payload, absent from /ai/1000.
	FollowCountChange  *int    `json:"followCountChange"`
	IsNewEntrant       bool    `json:"isNewEntrant,omitempty"`
	IsEmergingStartup  bool    `json:"isEmergingStartup,omitempty"`
	EmergingReasoning  *string `json:"emergingReasoning"`
	ClassificationTLDR *string `json:"classificationTldr"`

	// Direction is stamped by ExtractMovers ("up" or "down"). Empty for
	// entries returned by ExtractEmerging or ExtractMainRanking; the
	// upstream payload doesn't carry a per-entry direction.
	Direction string `json:"direction,omitempty"`

	// RawJSON keeps the original record substring so callers needing a
	// field we didn't surface can decode it themselves. Not exported as
	// JSON; lives solely for round-trip access.
	RawJSON json.RawMessage `json:"-"`
}

// RankingsCompanies bundles the three sections plus per-section
// ParseStats so a single ParseRankingsCompanies call returns everything
// the rankings commands need.
type RankingsCompanies struct {
	Emerging      []CompanyEntry
	MoversUp      []CompanyEntry
	MoversDown    []CompanyEntry
	Main          []CompanyEntry
	EmergingStats ParseStats
	MoversStats   ParseStats
	MainStats     ParseStats
}

// ErrSectionNotFound signals that a wrapper for the requested section
// could not be located in the decoded RSC stream. Callers should treat
// this as "page shape may have changed" (distinct from "section was
// present but had zero entries", which returns an empty slice + nil
// error).
var ErrSectionNotFound = errors.New("section wrapper not found in RSC stream")

// Wrapper shape for Emerging and per-direction Movers entries.
type sectionWrapper struct {
	Direction *string         `json:"direction"`
	Entries   json.RawMessage `json:"entries"`
}

// Wrapper shape for the Movers component prop object.
type moversWrapper struct {
	Left  *sectionWrapper `json:"left"`
	Right *sectionWrapper `json:"right"`
}

// ExtractEmerging returns the entries from the curated "EMERGING
// STARTUPS — CURATED THIS SNAPSHOT" section. The section is identified
// by a wrapper whose `direction` value is "emerging".
//
// The returned slice may contain entries with `IsEmergingStartup: false`
// — the curated list mixes flagged-emerging accounts with notable
// new-entrant adjacents. Callers wanting only the AI-judge-flagged
// emerging subset can filter on the IsEmergingStartup field downstream.
func ExtractEmerging(decoded string) ([]CompanyEntry, ParseStats, error) {
	wrapper, err := findDirectionWrapper(decoded, "emerging")
	if err != nil {
		return nil, ParseStats{}, err
	}
	entries, stats := decodeCompanyEntries(wrapper.Entries, "")
	return entries, stats, nil
}

// ExtractMovers returns both Movers entries lists. Up-movers carry
// `Direction: "up"`; down-movers carry `Direction: "down"`. ParseStats
// is aggregated across both halves so callers get a single drift
// signal for the section.
//
// A missing side (e.g., the page has `left` but no `right`) is not
// fatal — the present side is returned with the present-side
// ParseStats, and the missing side is returned as a nil slice. Only
// the case where *both* sides are missing returns ErrSectionNotFound.
func ExtractMovers(decoded string) ([]CompanyEntry, []CompanyEntry, ParseStats, error) {
	objs := scanObjectsContaining(decoded, `"left":{"direction":"up"`)
	if len(objs) == 0 {
		return nil, nil, ParseStats{}, fmt.Errorf("movers: %w", ErrSectionNotFound)
	}
	// Take the smallest enclosing — there should be exactly one such
	// wrapper on a healthy page; defensively use the first.
	var mw moversWrapper
	if err := json.Unmarshal(objs[0], &mw); err != nil {
		return nil, nil, ParseStats{}, fmt.Errorf("movers: decode wrapper: %w", err)
	}
	if mw.Left == nil && mw.Right == nil {
		return nil, nil, ParseStats{}, fmt.Errorf("movers: %w (both sides absent)", ErrSectionNotFound)
	}
	var up, down []CompanyEntry
	var upStats, downStats ParseStats
	if mw.Left != nil {
		up, upStats = decodeCompanyEntries(mw.Left.Entries, "up")
	}
	if mw.Right != nil {
		down, downStats = decodeCompanyEntries(mw.Right.Entries, "down")
	}
	return up, down, mergeStats(upStats, downStats), nil
}

// ExtractMainRanking returns the full company ranking from the page.
// Disambiguates the main wrapper from the Emerging/Movers wrappers by
// two anchors:
//
//  1. The wrapper has no `direction` sibling key (Emerging and Movers
//     wrappers always do).
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
			continue // Emerging or Movers wrapper.
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
	entries, stats := decodeCompanyEntries(best, "")
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

// ParseRankingsCompanies is the convenience entry for the rankings/
// companies page: decode RSC once, extract all three sections, return
// them together. Returns a typed error if the RSC stream is empty
// (page shape changed) — but a section-not-found mid-extract is
// recorded in the per-section ParseStats so callers can render
// whatever sections did parse.
func ParseRankingsCompanies(html []byte) (*RankingsCompanies, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, fmt.Errorf(
			"no RSC pushes found in /ai/x/rankings/companies HTML (%d bytes); page shape may have changed",
			len(html))
	}
	out := &RankingsCompanies{}

	if entries, stats, err := ExtractEmerging(decoded); err == nil {
		out.Emerging = entries
		out.EmergingStats = stats
	} else if !errors.Is(err, ErrSectionNotFound) {
		return nil, fmt.Errorf("emerging: %w", err)
	}

	if up, down, stats, err := ExtractMovers(decoded); err == nil {
		out.MoversUp = up
		out.MoversDown = down
		out.MoversStats = stats
	} else if !errors.Is(err, ErrSectionNotFound) {
		return nil, fmt.Errorf("movers: %w", err)
	}

	if entries, stats, err := ExtractMainRanking(decoded); err == nil {
		out.Main = entries
		out.MainStats = stats
	} else if !errors.Is(err, ErrSectionNotFound) {
		return nil, fmt.Errorf("main: %w", err)
	}

	return out, nil
}

// findDirectionWrapper locates a section wrapper with the named
// direction value (e.g. "emerging", "up", "down"). Returns
// ErrSectionNotFound if no candidate is found, or a structured
// decode error if a candidate is malformed.
func findDirectionWrapper(decoded, direction string) (*sectionWrapper, error) {
	needle := fmt.Sprintf(`"direction":%q`, direction)
	objs := scanObjectsContaining(decoded, needle)
	for _, raw := range objs {
		var w sectionWrapper
		if err := json.Unmarshal(raw, &w); err != nil {
			continue
		}
		if w.Direction == nil || *w.Direction != direction {
			// Defensive: smallest enclosing object may include an
			// adjacent direction value (rare, but possible if the
			// page lists multiple directions).
			continue
		}
		return &w, nil
	}
	return nil, fmt.Errorf("%s: %w", direction, ErrSectionNotFound)
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
//     count toward Attempted. They are a stable RSC dedup feature; the
//     main ranking refers to entries already inlined in Emerging or
//     Movers. The CLI command unions across sections if total coverage
//     is needed.
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
//
// When direction is non-empty it is stamped onto each returned entry.
func decodeCompanyEntries(raw json.RawMessage, direction string) ([]CompanyEntry, ParseStats) {
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
		if direction != "" {
			entry.Direction = direction
		}
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
// for the main ranking to reference up-/down-movers without
// duplicating the underlying object.
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

// mergeStats sums two ParseStats (used when an extractor combines
// multiple sub-arrays, e.g. Movers up + down). Errors are concatenated
// up to ParseStatsMaxErrors.
func mergeStats(a, b ParseStats) ParseStats {
	out := ParseStats{
		Attempted: a.Attempted + b.Attempted,
		Decoded:   a.Decoded + b.Decoded,
		Skipped:   a.Skipped + b.Skipped,
	}
	out.Errors = append(out.Errors, a.Errors...)
	for _, e := range b.Errors {
		if len(out.Errors) >= ParseStatsMaxErrors {
			break
		}
		out.Errors = append(out.Errors, e)
	}
	return out
}
