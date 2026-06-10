// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package autotempest

// Source kinds. inline sources return parsed per-car listings; link sources are
// comparison-link-only because the underlying site blocks scraping / requires
// login, so AutoTempest deep-links to them instead of returning listings.
const (
	KindInline = "inline"
	KindLink   = "link"
)

// Source is one user-facing AutoTempest search source.
type Source struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Country string `json:"country"`
	Kind    string `json:"kind"` // "inline" (parsed listings) or "link" (comparison link only)
}

// Sources is the static registry of the nine user-facing source codes, in the
// order AutoTempest presents them. `extended` is an internal flag, not a user
// source, so it is excluded. te/hem/cs/cv/cm/eb/ot return inline listings; fbm
// and st are comparison-link-only.
var Sources = []Source{
	{Code: "te", Name: "AutoTempest", Country: "US", Kind: KindInline},
	{Code: "hem", Name: "Hemmings", Country: "US", Kind: KindInline},
	{Code: "cs", Name: "CarSoup", Country: "US", Kind: KindInline},
	{Code: "cv", Name: "Carvana", Country: "US", Kind: KindInline},
	{Code: "cm", Name: "Cars.com", Country: "US", Kind: KindInline},
	{Code: "eb", Name: "eBay", Country: "US", Kind: KindInline},
	{Code: "ot", Name: "Others", Country: "US", Kind: KindInline},
	{Code: "fbm", Name: "Facebook Marketplace", Country: "US", Kind: KindLink},
	{Code: "st", Name: "SearchTempest", Country: "US", Kind: KindLink},
}

// DefaultSourceCodes lists every user-facing source code, in registry order.
func DefaultSourceCodes() []string {
	out := make([]string, 0, len(Sources))
	for _, s := range Sources {
		out = append(out, s.Code)
	}
	return out
}

// DeduplicationSites is the full pipe-joined source list AutoTempest sends as
// the deduplicationSites param on every per-source query.
const DeduplicationSites = "te|hem|cs|cv|cm|eb|ot|extended|fbm|st"

// SourceName returns the display name for a source code, or the code itself if
// unknown.
func SourceName(code string) string {
	for _, s := range Sources {
		if s.Code == code {
			return s.Name
		}
	}
	return code
}

// IsKnownSource reports whether code is a user-facing source code.
func IsKnownSource(code string) bool {
	for _, s := range Sources {
		if s.Code == code {
			return true
		}
	}
	return false
}
