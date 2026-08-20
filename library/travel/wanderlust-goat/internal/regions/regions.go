// Package regions is the single source of truth for country -> Stage-2
// research-source mapping. v1 scattered switch statements across the
// orchestrator and per-source clients; v2 inverts that — adding a country
// means adding one row to the table below.
//
// Slugs in LocalReviewSites and LocalForums are kebab-case package names
// under internal/<source>/. The wiring test in internal/cli/wiring_test.go
// uses this table to assert every named source is reachable from cli/.
package regions

import "strings"

// Region describes one country (or country group) and the locale-aware
// research-stage sources the dispatcher should fan out to within it.
type Region struct {
	// Codes is the set of ISO 3166-1 alpha-2 country codes this region
	// applies to. Most regions cover one country; DE-AT-CH and UK-IE share.
	Codes []string

	// PrimaryLanguage is the dominant local language code (ja, ko, fr...).
	PrimaryLanguage string

	// Languages is every language the dispatcher should query in. Always
	// includes "en" for cross-checks against international sources.
	Languages []string

	// LocalReviewSites are slugs (= package names under internal/<source>/)
	// for editorial / curated review aggregators in this country. Order is
	// trust-descending: the first-listed source has the highest trust weight.
	LocalReviewSites []string

	// LocalForums are subreddits (or other community forums) to search for
	// "<name> <city>" mentions. Plain strings, no /r/ prefix.
	LocalForums []string

	// GoogleTLD is the country-code TLD for google.<TLD> queries. Empty
	// string for countries where Google is blocked (CN); the dispatcher
	// falls through silently for those.
	GoogleTLD string

	// Description is a one-line summary used by the `coverage` command.
	Description string
}

// regions is the static table. Order matters only for fallback resolution
// (Lookup walks linearly and returns the first match).
var regions = []Region{
	{
		Codes:            []string{"JP"},
		PrimaryLanguage:  "ja",
		Languages:        []string{"ja", "en"},
		LocalReviewSites: []string{"tabelog", "retty", "hotpepper", "notecom", "hatena"},
		LocalForums:      []string{"japan", "JapanTravel", "japanfood"},
		GoogleTLD:        "co.jp",
		Description:      "Japan: Tabelog/Retty/Hot Pepper editorial layer; note.com + hatena.ne.jp blogs; Reddit JP subs.",
	},
	{
		Codes:            []string{"KR"},
		PrimaryLanguage:  "ko",
		Languages:        []string{"ko", "en"},
		LocalReviewSites: []string{"navermap", "naverblog", "kakaomap", "mangoplate"},
		LocalForums:      []string{"korea", "seoul", "KoreaTravel"},
		GoogleTLD:        "co.kr",
		Description:      "Korea: Naver map/blog primary; Kakao map + MangoPlate secondary; Korean-language Reddit.",
	},
	{
		Codes:            []string{"CN"},
		PrimaryLanguage:  "zh",
		Languages:        []string{"zh", "en"},
		LocalReviewSites: []string{"dianping", "mafengwo", "xiaohongshu"},
		LocalForums:      []string{"China_irl", "travelchina"},
		GoogleTLD:        "", // Google blocked in CN; fall through silently.
		Description:      "China: Dianping/Mafengwo/Xiaohongshu where scrapable; no Google search path.",
	},
	{
		Codes:            []string{"FR"},
		PrimaryLanguage:  "fr",
		Languages:        []string{"fr", "en"},
		LocalReviewSites: []string{"lefooding", "pudlo", "lafourchette"},
		LocalForums:      []string{"Paris", "france", "francetravel"},
		GoogleTLD:        "fr",
		Description:      "France: Le Fooding editorial; Pudlo/LaFourchette aggregators; Paris/France subreddits.",
	},
	{
		Codes:            []string{"IT"},
		PrimaryLanguage:  "it",
		Languages:        []string{"it", "en"},
		LocalReviewSites: []string{"gamberorosso", "slowfood", "dissapore"},
		LocalForums:      []string{"italy", "Rome", "Milan"},
		GoogleTLD:        "it",
		Description:      "Italy: Gambero Rosso/Slow Food/Dissapore editorial; Italian-language city subs.",
	},
	{
		Codes:            []string{"DE", "AT", "CH"},
		PrimaryLanguage:  "de",
		Languages:        []string{"de", "en"},
		LocalReviewSites: []string{"falstaff", "derfeinschmecker"},
		LocalForums:      []string{"germany", "wien", "Munich"},
		GoogleTLD:        "de",
		Description:      "DACH: Falstaff/Der Feinschmecker editorial; German-language city subs.",
	},
	{
		Codes:            []string{"ES"},
		PrimaryLanguage:  "es",
		Languages:        []string{"es", "en"},
		LocalReviewSites: []string{"verema", "eltenedor"},
		LocalForums:      []string{"spain", "madrid"},
		GoogleTLD:        "es",
		Description:      "Spain: Verema/El Tenedor aggregators; Spanish-language city subs.",
	},
	{
		Codes:            []string{"GB", "UK", "IE"},
		PrimaryLanguage:  "en",
		Languages:        []string{"en"},
		LocalReviewSites: []string{"squaremeal", "hotdinners", "observerfood"},
		LocalForums:      []string{"london", "unitedkingdom"},
		GoogleTLD:        "co.uk",
		Description:      "UK/Ireland: SquareMeal/Hot Dinners/Observer Food; UK city subs.",
	},
}

// fallback is returned by Lookup when no region matches. English Reddit
// travel subs only — no review sites.
var fallback = Region{
	Codes:            []string{"*"},
	PrimaryLanguage:  "en",
	Languages:        []string{"en"},
	LocalReviewSites: nil,
	LocalForums:      []string{"travel", "solotravel"},
	GoogleTLD:        "",
	Description:      "Fallback: English travel subreddits only; no curated regional review sites.",
}

// Lookup returns the Region for a country code. The lookup is case-
// insensitive and accepts both ISO alpha-2 codes ("jp") and lower-case
// alphas. Unknown codes return the fallback Region.
func Lookup(countryCode string) Region {
	cc := strings.ToUpper(strings.TrimSpace(countryCode))
	if cc == "" || cc == "*" {
		return fallback
	}
	for _, r := range regions {
		for _, code := range r.Codes {
			if code == cc {
				return r
			}
		}
	}
	return fallback
}

// All returns every region (excluding the fallback). The wiring test and
// `coverage` command iterate this to enumerate every Stage-2 source slug
// the dispatcher MAY route to.
func All() []Region {
	out := make([]Region, len(regions))
	copy(out, regions)
	return out
}

// Fallback returns the fallback region used when no country matches.
func Fallback() Region {
	return fallback
}

// AllSourceSlugs returns the unique set of every LocalReviewSites slug
// across every region. Used by the wiring test and `coverage` to know
// which internal/<source>/ packages MUST exist.
func AllSourceSlugs() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range regions {
		for _, s := range r.LocalReviewSites {
			if !seen[s] {
				seen[s] = true
				out = append(out, s)
			}
		}
	}
	return out
}
