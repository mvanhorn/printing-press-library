// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Facets are the filter values the careers site embeds in its /en/jobs/ page
// (the self.__next_f RSC payload). They are the only exact source of team,
// subteam, and country spellings for the search filters.
type Facets struct {
	Countries         []string            `json:"countries"`
	Teams             []string            `json:"teams"`
	SubTeams          []string            `json:"sub_teams"`
	TeamSubTeams      map[string][]string `json:"team_sub_teams"`
	ContractTypes     []string            `json:"contract_types"`
	WorkPatterns      []string            `json:"work_patterns"`
	TotalJobs         *int                `json:"total_jobs"`
	UnmappedCountries []string            `json:"unmapped_countries"`
}

var reFacetTotal = regexp.MustCompile(`"totalJobs":(\d+)`)

// ParseFacets extracts the facet lists from the job list page HTML. A page
// without a countries list (a challenge or an unrelated page) is an error.
func ParseFacets(page []byte) (*Facets, error) {
	flat := strings.ReplaceAll(string(page), `\"`, `"`)
	f := &Facets{}
	f.Countries = facetList(flat, "countries")
	f.Teams = facetList(flat, "teams")
	f.SubTeams = facetList(flat, "subTeams")
	f.ContractTypes = facetList(flat, "contractTypes")
	f.WorkPatterns = facetList(flat, "workPatterns")
	f.TeamSubTeams = facetObject(flat, "teamToSubTeamMappings")
	if m := reFacetTotal.FindStringSubmatch(flat); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			f.TotalJobs = &n
		}
	}
	if len(f.Countries) == 0 || len(f.Teams) == 0 {
		return nil, errors.New("page has no embedded country and team facet lists")
	}
	for _, c := range f.Countries {
		if _, ok := CountryNameToISO3(c); !ok {
			f.UnmappedCountries = append(f.UnmappedCountries, c)
		}
	}
	if f.UnmappedCountries == nil {
		f.UnmappedCountries = []string{}
	}
	return f, nil
}

func facetList(flat, key string) []string {
	marker := `"` + key + `":[`
	i := strings.Index(flat, marker)
	if i < 0 {
		return []string{}
	}
	start := i + len(marker) - 1
	end := matchBracket(flat, start, '[', ']')
	if end < 0 {
		return []string{}
	}
	var out []string
	if json.Unmarshal([]byte(flat[start:end+1]), &out) != nil {
		return []string{}
	}
	return out
}

func facetObject(flat, key string) map[string][]string {
	marker := `"` + key + `":{`
	i := strings.Index(flat, marker)
	out := map[string][]string{}
	if i < 0 {
		return out
	}
	start := i + len(marker) - 1
	end := matchBracket(flat, start, '{', '}')
	if end < 0 {
		return out
	}
	_ = json.Unmarshal([]byte(flat[start:end+1]), &out)
	for k := range out {
		sort.Strings(out[k])
	}
	return out
}

// matchBracket finds the index of the bracket closing the one at start,
// skipping brackets inside JSON strings.
func matchBracket(s string, start int, open, close byte) int {
	depth := 0
	inString := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inString {
			if ch == '\\' {
				i++
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case open:
			depth++
		case close:
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}
