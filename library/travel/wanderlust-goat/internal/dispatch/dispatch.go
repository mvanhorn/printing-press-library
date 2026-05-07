// Package dispatch implements country→source-set and intent→source-set
// mapping for the `research-plan` and fanout commands. Pure data; no I/O.
package dispatch

import (
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/sources"
)

// Plan is the typed call graph emitted by `research-plan`. Agents loop
// over the entries and execute each through the matching CLI subcommand.
type Plan struct {
	Anchor   AnchorRef `json:"anchor"`
	Country  string    `json:"country"`
	Intent   string    `json:"intent"`
	Calls    []Call    `json:"calls"`
	Sources  []string  `json:"sources"`
	Notes    []string  `json:"notes"`
}

// AnchorRef is the resolved anchor location echoed back so agents have
// canonical lat/lng without re-running geocode.
type AnchorRef struct {
	Query   string  `json:"query"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Country string  `json:"country"`
	Display string  `json:"display"`
}

// Call is one source query the agent should run.
type Call struct {
	Client        string            `json:"client"`         // matches sources.Slug (e.g. "tabelog")
	Method        string            `json:"method"`         // "search", "geosearch", "category"
	Params        map[string]string `json:"params"`         // criteria, anchor lat/lng, radius
	Locale        string            `json:"locale"`         // en, ja, ko, fr
	ExpectedTrust float64           `json:"expected_trust"` // pre-boost trust for ranking
	Tier          string            `json:"tier"`           // editorial, regional, foundation, hidden, crowd
	Stub          bool              `json:"stub,omitempty"` // true if the source ships as v1 stub
	StubReason    string            `json:"stub_reason,omitempty"`
}

// Build constructs a research plan for the given anchor and parsed criteria.
// Anchor must have lat/lng and a country code resolved.
//
// Default ordering: foundation (geocode/routing) → editorial → regional
// (with country boost) → hidden → crowd.
func Build(anchor AnchorRef, intent sources.Intent, redditKW []string, anchorRadiusMeters int) Plan {
	plan := Plan{
		Anchor:  anchor,
		Country: string(anchor.Country),
		Intent:  string(intent),
	}
	for _, src := range sources.ForCountry(sources.Country(anchor.Country)) {
		// Skip OSRM in research-plan — it's a per-pair lookup, not a search source.
		if src.Slug == "osrm" {
			continue
		}
		// Filter by intent compatibility: include the source only if it
		// claims the intent OR has no intent constraint listed.
		matches := len(src.Intents) == 0
		for _, allowed := range src.Intents {
			if allowed == intent {
				matches = true
				break
			}
		}
		if !matches {
			continue
		}
		method, params := methodFor(src, anchor, intent, redditKW, anchorRadiusMeters)
		plan.Calls = append(plan.Calls, Call{
			Client:        src.Slug,
			Method:        method,
			Params:        params,
			Locale:        src.Locale,
			ExpectedTrust: src.Trust + boostFor(src, anchor.Country),
			Tier:          tierName(src.Tier),
			Stub:          src.Stub,
			StubReason:    src.StubReason,
		})
		plan.Sources = append(plan.Sources, src.Slug)
	}
	if len(redditKW) > 0 {
		plan.Notes = append(plan.Notes, "Reddit calls filter body keyword: "+joinComma(redditKW))
	}
	if anchor.Country != string(sources.CountryUniversal) && anchor.Country != "" {
		plan.Notes = append(plan.Notes, "Local-language regional sources for "+anchor.Country+" get a +0.05 trust boost.")
	}
	return plan
}

func boostFor(src sources.Source, country string) float64 {
	if src.CountryMatchBoost > 0 && string(src.Country) == country {
		return src.CountryMatchBoost
	}
	return 0
}

func methodFor(src sources.Source, anchor AnchorRef, intent sources.Intent, kw []string, radius int) (string, map[string]string) {
	params := map[string]string{
		"lat":            ftoa(anchor.Lat),
		"lng":            ftoa(anchor.Lng),
		"intent":         string(intent),
		"radius_meters":  itoa(radius),
	}
	switch src.Slug {
	case "nominatim":
		return "search", map[string]string{"query": anchor.Query}
	case "overpass":
		return "category", params
	case "wikipedia":
		params["locale"] = src.Locale
		return "geosearch", params
	case "wikivoyage":
		params["locale"] = src.Locale
		return "page", params
	case "atlasobscura":
		return "near", params
	case "reddit":
		params["subreddits"] = subredditsForCountry(anchor.Country)
		params["min_score"] = "10"
		params["min_comments"] = "3"
		if len(kw) > 0 {
			params["keyword_filter"] = joinComma(kw)
		}
		return "search", params
	case "tabelog", "naver_map", "navermap", "mangoplate", "kakaomap", "lefooding", "michelin", "eater", "timeout", "nyt36hours", "naverblog", "fourtravel", "retty", "hotpepper", "notecom", "pudlo":
		return "search", params
	}
	return "search", params
}

func tierName(t sources.Tier) string {
	switch t {
	case sources.TierFoundation:
		return "foundation"
	case sources.TierEditorial:
		return "editorial"
	case sources.TierRegional:
		return "regional"
	case sources.TierHidden:
		return "hidden"
	case sources.TierCrowd:
		return "crowd"
	}
	return "unknown"
}

func subredditsForCountry(country string) string {
	switch country {
	case "JP":
		return "japan,JapanTravel,Tokyo,osaka,Kyoto"
	case "KR":
		return "korea,seoul,KoreaTravel"
	case "FR":
		return "Paris,France,francetravel"
	case "GB", "UK":
		return "london,unitedkingdom"
	case "IT":
		return "italy,Rome,Milan"
	case "DE":
		return "berlin,germany,Munich"
	}
	// Universal fallback
	return "travel,solotravel,backpacking"
}

func ftoa(f float64) string {
	// 5 decimal degrees ≈ 1m precision; enough for source dispatch.
	return strconv.FormatFloat(f, 'f', 5, 64)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}
