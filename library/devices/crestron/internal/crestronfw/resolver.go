// Package crestronfw resolves Crestron models to the firmware releases that
// cover them.
//
// Crestron scopes one firmware release to an entire model family: a single
// release titled "TSW-570/TSW-770/TSW-1070/TSS-770/TSS-1070/TS-770/TS-1070
// 3.0.x" is the current firmware for seven distinct models. Searching the site
// for one of those models can therefore miss the release that actually governs
// it, because the release is indexed under the family title. Resolving that
// many-to-many mapping is the reason this package exists.
package crestronfw

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/crestron/internal/crestronparse"
)

// Searcher fetches firmware search results for a query. It is an interface so
// the resolver can be tested without network access.
type Searcher interface {
	SearchFirmware(ctx context.Context, query string, limit int) ([]crestronparse.SearchResult, error)
}

// Release is a firmware release with its covered models resolved.
type Release struct {
	Title    string    `json:"title"`
	Version  string    `json:"version,omitempty"`
	Models   []string  `json:"models,omitempty"`
	Date     string    `json:"date,omitempty"`
	Released time.Time `json:"-"`
	URL      string    `json:"url,omitempty"`
}

// Status is the currency verdict for one fleet model.
type Status struct {
	Model     string `json:"model"`
	Installed string `json:"installed,omitempty"`
	Latest    string `json:"latest,omitempty"`
	Date      string `json:"latest_date,omitempty"`
	URL       string `json:"release_url,omitempty"`
	// CoveredBy is the release title, which names the whole family. It is
	// surfaced because it is often not the model the user searched for.
	CoveredBy string `json:"covered_by,omitempty"`
	State     string `json:"state"`
	Note      string `json:"note,omitempty"`
}

// Fleet status states.
const (
	StateCurrent   = "current"
	StateOutdated  = "outdated"
	StateUnknown   = "unknown"
	StateNoRelease = "no-release"
	StateError     = "error"
)

// ParseFleetFile reads a fleet list. Each line is a model, optionally followed
// by the installed version after whitespace, a comma, or an equals sign.
// Blank lines and # comments are ignored.
func ParseFleetFile(content string) []Status {
	out := make([]Status, 0)
	for _, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "#"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		model, installed := line, ""
		for _, sep := range []string{",", "=", "\t", " "} {
			if i := strings.Index(line, sep); i > 0 {
				model = strings.TrimSpace(line[:i])
				installed = strings.TrimSpace(line[i+len(sep):])
				break
			}
		}
		installed = strings.TrimLeft(installed, ",= \t")
		if model == "" {
			continue
		}
		out = append(out, Status{Model: model, Installed: installed})
	}
	return out
}

// releaseDateLayouts covers the date shapes Crestron renders in search results.
var releaseDateLayouts = []string{"Jan 2, 2006", "January 2, 2006", "1/2/2006", "2006-01-02"}

func parseReleaseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, l := range releaseDateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// ReleasesFrom converts raw search rows into releases with their covered model
// set expanded.
func ReleasesFrom(rows []crestronparse.SearchResult) []Release {
	out := make([]Release, 0, len(rows))
	for _, r := range rows {
		parts, version := crestronparse.SplitReleaseTitle(r.Title)
		models := crestronparse.ExpandModelFamily(parts)
		out = append(out, Release{
			Title:    r.Title,
			Version:  version,
			Models:   models,
			Date:     r.Date,
			Released: parseReleaseDate(r.Date),
			URL:      r.URL,
		})
	}
	return out
}

// normalizeModel makes model comparison tolerant of the punctuation and casing
// differences between a user's fleet list and Crestron's release titles.
// "DM-NVX-384(C)" and "dm nvx 384c" both normalize to "DMNVX384C".
func normalizeModel(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CoveringReleases returns the releases that cover a model, newest first.
//
// A release covers a model when the model appears in the release's expanded
// model list. Exact normalized equality is preferred; a release whose model
// list contains an entry that is a normalized prefix of the requested model
// (or vice versa) is accepted as a family match so that "DM-NVX-384" matches a
// release listing "DM-NVX-384(C)".
func CoveringReleases(releases []Release, model string) []Release {
	want := normalizeModel(model)
	if want == "" {
		return nil
	}
	var exact, family []Release
	for _, rel := range releases {
		var isExact, isFamily bool
		for _, m := range rel.Models {
			got := normalizeModel(m)
			if got == "" {
				continue
			}
			switch {
			case got == want:
				isExact = true
			case strings.HasPrefix(got, want) || strings.HasPrefix(want, got):
				isFamily = true
			}
		}
		if isExact {
			exact = append(exact, rel)
		} else if isFamily {
			family = append(family, rel)
		}
	}
	hits := exact
	if len(hits) == 0 {
		hits = family
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if !hits[i].Released.Equal(hits[j].Released) {
			return hits[i].Released.After(hits[j].Released)
		}
		return compareVersions(hits[i].Version, hits[j].Version) > 0
	})
	return hits
}

// compareVersions orders dotted numeric versions. Non-numeric segments fall
// back to string comparison so unusual version strings still order stably.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) || i < len(bs); i++ {
		var av, bv string
		if i < len(as) {
			av = as[i]
		}
		if i < len(bs) {
			bv = bs[i]
		}
		ai, aerr := strconv.Atoi(av)
		bi, berr := strconv.Atoi(bv)
		if aerr == nil && berr == nil {
			if ai != bi {
				if ai > bi {
					return 1
				}
				return -1
			}
			continue
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	return 0
}

// Resolve fills in the currency verdict for one model.
func Resolve(ctx context.Context, s Searcher, st Status, limit int) Status {
	rows, err := s.SearchFirmware(ctx, st.Model, limit)
	if err != nil {
		st.State = StateError
		st.Note = fmt.Sprintf("firmware search failed: %v", err)
		return st
	}
	hits := CoveringReleases(ReleasesFrom(rows), st.Model)
	if len(hits) == 0 {
		st.State = StateNoRelease
		st.Note = "no firmware release found covering this model"
		return st
	}
	latest := hits[0]
	st.Latest = latest.Version
	st.Date = latest.Date
	st.URL = latest.URL
	st.CoveredBy = latest.Title

	switch {
	case st.Installed == "":
		st.State = StateUnknown
		st.Note = "no installed version supplied; add it after the model in the fleet file to get a currency verdict"
	case compareVersions(st.Installed, latest.Version) >= 0:
		st.State = StateCurrent
	default:
		st.State = StateOutdated
	}
	return st
}
