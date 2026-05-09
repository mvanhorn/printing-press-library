// Roster1000 parser: extracts the structured AI 1000 author records from
// the /ai/1000 page's RSC payload.
//
// The /ai/1000 page is a Next.js 15 SPA. The roster — every ranked
// author, with rank/score/category/bio/vibeDistribution — ships embedded
// in one of the page's self.__next_f.push([1, "..."]) calls as an
// `entries` array shaped like:
//
//	{"entries":[{"rank":1,"target_x_id":"1605","username":"sama",
//	  "display_name":"Sam Altman", ...,
//	  "previousRank":1,"rankChange":0,"categoryRank":1,
//	  "vibeDistribution":{"troll":0,"banter":3.1,...},
//	  "vibeTweetCount":200},{"rank":2,...}], "maxScore":...}
//
// We reuse the existing DecodeRSC stream walker, scan for objects
// containing the `target_x_id` key (the most distinctive marker; clusters
// don't have it), and JSON-decode each.
//
// Tolerances:
//   - previousRank and rankChange are commonly null (newly-listed accounts);
//     we keep the JSON null distinction by typing them as *int.
//   - githubUrl is null for most entries; *string.
//   - vibeDistribution is decoded into a map[string]float64 so future keys
//     don't break the parser.
//   - Malformed object substrings are skipped and reported as a partial
//     error wrapping the bad index, never panicked.
package diggparse

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Roster1000Author is one entry from /ai/1000.
type Roster1000Author struct {
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
	CategoryConfidence float64            `json:"categoryConfidence,omitempty"`
	GithubURL          *string            `json:"githubUrl"`
	PreviousRank       *int               `json:"previousRank"`
	RankChange         *int               `json:"rankChange"`
	CategoryRank       int                `json:"categoryRank,omitempty"`
	VibeDistribution   map[string]float64 `json:"vibeDistribution,omitempty"`
	VibeTweetCount     int                `json:"vibeTweetCount,omitempty"`

	// RawJSON keeps the original record substring so callers that need a
	// field we didn't surface can decode it themselves. Not exported as
	// JSON; lives solely for round-trip access.
	RawJSON json.RawMessage `json:"-"`
}

// ExtractRoster1000Authors walks the decoded RSC stream and returns every
// distinct author record found. Authors are deduplicated by username
// (lowercased) — earlier occurrences win. Returns the slice and, when at
// least one chunk failed to decode, an error wrapping the bad chunk
// indexes; valid records are still returned alongside the error so
// callers can tolerate partial parses.
func ExtractRoster1000Authors(decoded string) ([]Roster1000Author, error) {
	objs := scanObjectsContaining(decoded, `"target_x_id":`)
	out := make([]Roster1000Author, 0, len(objs))
	seen := make(map[string]bool, len(objs))
	var badIdxs []int
	for i, raw := range objs {
		var a Roster1000Author
		if err := json.Unmarshal(raw, &a); err != nil {
			badIdxs = append(badIdxs, i)
			continue
		}
		if a.Username == "" || a.Rank <= 0 {
			// Object had target_x_id but isn't a roster entry (defensive —
			// a future schema may carry the field elsewhere).
			continue
		}
		key := strings.ToLower(a.Username)
		if seen[key] {
			continue
		}
		seen[key] = true
		a.RawJSON = append(a.RawJSON[:0:0], raw...)
		out = append(out, a)
	}
	if len(badIdxs) > 0 {
		return out, fmt.Errorf("roster /ai/1000: %d malformed RSC chunk(s) at indexes %v", len(badIdxs), badIdxs)
	}
	return out, nil
}

// ParseRoster1000 is the convenience entry for the /ai/1000 page: decode
// RSC, extract author records, return them. If decoded RSC is empty or
// no records are found, returns a typed error so callers can distinguish
// "page changed shape" from a genuine empty roster.
func ParseRoster1000(html []byte) ([]Roster1000Author, error) {
	decoded := DecodeRSC(html)
	if decoded == "" {
		return nil, fmt.Errorf("no RSC pushes found in /ai/1000 HTML (%d bytes); page shape may have changed", len(html))
	}
	authors, err := ExtractRoster1000Authors(decoded)
	if len(authors) == 0 {
		if err != nil {
			return nil, fmt.Errorf("/ai/1000 parse produced 0 authors: %w", err)
		}
		return nil, errors.New("/ai/1000 parse produced 0 authors; page shape may have changed")
	}
	return authors, err
}
