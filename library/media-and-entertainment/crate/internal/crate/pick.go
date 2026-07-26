// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

package crate

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Filter narrows a shelf before picking or counting.
type Filter struct {
	Genre  string
	Style  string
	Label  string
	Artist string
	Format string
	// Decade is a bare decade like "1970" or "1970s".
	Decade string
	// YearFrom and YearTo bound the release year; zero means unbounded.
	YearFrom int
	YearTo   int
	// Unrated keeps only records with no rating, which is the closest
	// available stand-in for "not listened to lately".
	Unrated bool
}

func containsFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(strings.TrimSpace(v), want) {
			return true
		}
		// Discogs styles are multi-word and users type prefixes.
		if strings.Contains(strings.ToLower(v), strings.ToLower(want)) {
			return true
		}
	}
	return false
}

// decadeStart parses "1970" or "1970s" into 1970. ok is false if neither.
func decadeStart(s string) (int, bool) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(s), "s"))
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1000 || n > 3000 {
		return 0, false
	}
	return (n / 10) * 10, true
}

// Matches reports whether a record satisfies every set field of the filter.
func (f Filter) Matches(r Record) bool {
	if f.Genre != "" && !containsFold(r.Genres, f.Genre) {
		return false
	}
	if f.Style != "" && !containsFold(r.Styles, f.Style) {
		return false
	}
	if f.Label != "" && !containsFold(r.Labels, f.Label) {
		return false
	}
	if f.Artist != "" && !containsFold(r.Artists, f.Artist) {
		return false
	}
	if f.Format != "" && !containsFold(r.Formats, f.Format) {
		return false
	}
	if f.Decade != "" {
		start, ok := decadeStart(f.Decade)
		if !ok {
			return false
		}
		if r.Year < start || r.Year > start+9 {
			return false
		}
	}
	if f.YearFrom > 0 && r.Year < f.YearFrom {
		return false
	}
	if f.YearTo > 0 && r.Year > f.YearTo {
		return false
	}
	if f.Unrated && r.Rating != 0 {
		return false
	}
	return true
}

// Apply returns the records matching the filter.
func (f Filter) Apply(recs []Record) []Record {
	out := make([]Record, 0, len(recs))
	for _, r := range recs {
		if f.Matches(r) {
			out = append(out, r)
		}
	}
	return out
}

// Pick chooses one record and explains the choice.
//
// Selection is deterministic given a seed so a run can be reproduced, and the
// candidate pool is returned alongside so the caller can say how much of the
// shelf was in play. When preferUnrated is set, records with no rating are
// drawn from first: an unrated record is the best signal Discogs offers for
// "you have not sat with this one". When it is not set the whole matching pool
// is used, which is what --any means and must actually do.
func Pick(recs []Record, f Filter, seed int64, preferUnrated bool) (Record, string, int, bool) {
	pool := f.Apply(recs)
	if len(pool) == 0 {
		return Record{}, "", 0, false
	}

	var unrated []Record
	if preferUnrated {
		for _, r := range pool {
			if r.Rating == 0 {
				unrated = append(unrated, r)
			}
		}
	}

	chooseFrom, reason := pool, "from the whole shelf"
	if len(unrated) > 0 {
		chooseFrom = unrated
		reason = fmt.Sprintf("unrated, so probably unplayed (%d of %d matching records are unrated)",
			len(unrated), len(pool))
	}

	// Stable ordering before indexing, so the same seed always yields the
	// same record regardless of the order rows came back from SQLite.
	sort.SliceStable(chooseFrom, func(i, j int) bool { return chooseFrom[i].ReleaseID < chooseFrom[j].ReleaseID })

	idx := int(seed % int64(len(chooseFrom)))
	if idx < 0 {
		idx += len(chooseFrom)
	}
	return chooseFrom[idx], reason, len(pool), true
}

// Tally is one row of a shelf breakdown.
type Tally struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	// Share is the fraction of records in this bucket, 0..1.
	Share float64 `json:"share"`
}

// Dimension names a way to slice a collection.
type Dimension string

const (
	ByDecade Dimension = "decade"
	ByGenre  Dimension = "genre"
	ByStyle  Dimension = "style"
	ByLabel  Dimension = "label"
	ByFormat Dimension = "format"
	ByArtist Dimension = "artist"
	ByYear   Dimension = "year"
)

// ValidDimensions lists every accepted --by value.
func ValidDimensions() []string {
	return []string{"decade", "genre", "style", "label", "format", "artist", "year"}
}

// Breakdown counts a collection along one dimension, most common first.
//
// Multi-valued dimensions (a record can carry several genres, styles, labels,
// or artists) count once per distinct value, so the counts deliberately sum to
// more than the number of records. Share is therefore expressed against the
// record count, not against the sum of counts, and the caller says so.
func Breakdown(recs []Record, dim Dimension) ([]Tally, error) {
	switch dim {
	case ByDecade, ByGenre, ByStyle, ByLabel, ByFormat, ByArtist, ByYear:
	default:
		// Validate up front. Inside the loop this never fires for an empty
		// collection, so Breakdown(nil, "bogus") would return no error.
		return nil, fmt.Errorf("unknown breakdown %q: use one of %s", dim, strings.Join(ValidDimensions(), ", "))
	}

	counts := map[string]int{}
	add := func(k string) {
		k = strings.TrimSpace(k)
		if k == "" {
			return
		}
		counts[k]++
	}

	for _, r := range recs {
		switch dim {
		case ByDecade:
			add(r.Decade())
		case ByGenre:
			for _, v := range r.Genres {
				add(v)
			}
		case ByStyle:
			for _, v := range r.Styles {
				add(v)
			}
		case ByLabel:
			for _, v := range r.Labels {
				add(v)
			}
		case ByFormat:
			for _, v := range r.Formats {
				add(v)
			}
		case ByArtist:
			for _, v := range r.Artists {
				add(v)
			}
		case ByYear:
			if r.Year >= 1000 {
				add(strconv.Itoa(r.Year))
			}
		}
	}

	out := make([]Tally, 0, len(counts))
	for k, n := range counts {
		share := 0.0
		if len(recs) > 0 {
			share = float64(n) / float64(len(recs))
		}
		out = append(out, Tally{Key: k, Count: n, Share: share})
	}
	// Count descending, then key ascending, so equal counts render stably.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// IsMultiValued reports whether a dimension can count a record more than once.
func IsMultiValued(dim Dimension) bool {
	switch dim {
	case ByGenre, ByStyle, ByLabel, ByArtist, ByFormat:
		return true
	default:
		return false
	}
}
