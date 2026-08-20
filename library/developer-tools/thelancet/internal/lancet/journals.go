// Package lancet holds the Lancet journal registry and the OpenAlex-backed
// local data layer that powers the analytics commands. Hand-authored; not
// generated.
package lancet

import (
	"sort"
	"strings"
)

// Journal is one member of The Lancet family of journals.
type Journal struct {
	Slug    string `json:"slug"`    // CLI-facing slug, e.g. "lancet-oncology"
	ISSN    string `json:"issn"`    // ISSN-L used to scope OpenAlex queries
	Display string `json:"display"` // canonical prose name
}

// journals is the curated Lancet family registry. ISSNs are ISSN-L values as
// indexed by OpenAlex's primary_location.source.issn filter.
var journals = []Journal{
	{"lancet", "0140-6736", "The Lancet"},
	{"lancet-oncology", "1470-2045", "The Lancet Oncology"},
	{"lancet-neurology", "1474-4422", "The Lancet Neurology"},
	{"lancet-infectious-diseases", "1473-3099", "The Lancet Infectious Diseases"},
	{"lancet-respiratory", "2213-2600", "The Lancet Respiratory Medicine"},
	{"lancet-diabetes-endocrinology", "2213-8587", "The Lancet Diabetes & Endocrinology"},
	{"lancet-psychiatry", "2215-0366", "The Lancet Psychiatry"},
	{"lancet-public-health", "2468-2667", "The Lancet Public Health"},
	{"lancet-global-health", "2214-109X", "The Lancet Global Health"},
	{"lancet-gastroenterology-hepatology", "2468-1253", "The Lancet Gastroenterology & Hepatology"},
	{"lancet-haematology", "2352-3026", "The Lancet Haematology"},
	{"lancet-hiv", "2352-3018", "The Lancet HIV"},
	{"lancet-planetary-health", "2542-5196", "The Lancet Planetary Health"},
	{"lancet-child-adolescent", "2352-4642", "The Lancet Child & Adolescent Health"},
	{"lancet-rheumatology", "2665-9913", "The Lancet Rheumatology"},
	{"lancet-digital-health", "2589-7500", "The Lancet Digital Health"},
	{"lancet-healthy-longevity", "2666-7568", "The Lancet Healthy Longevity"},
	{"ebiomedicine", "2352-3964", "eBioMedicine"},
	{"eclinicalmedicine", "2589-5370", "eClinicalMedicine"},
}

// Journals returns the full registry, sorted by slug.
func Journals() []Journal {
	out := make([]Journal, len(journals))
	copy(out, journals)
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out
}

// Lookup resolves a journal by slug, ISSN, or "all". The special slug "all"
// returns every journal; an empty slug defaults to the flagship Lancet.
// Returns the matched journals and true, or nil and false when unknown.
func Lookup(slugOrISSN string) ([]Journal, bool) {
	key := strings.ToLower(strings.TrimSpace(slugOrISSN))
	if key == "" || key == "lancet" {
		// Bare "lancet" means the flagship journal only.
		return []Journal{journals[0]}, true
	}
	if key == "all" || key == "family" {
		return Journals(), true
	}
	for _, j := range journals {
		if strings.ToLower(j.Slug) == key || j.ISSN == slugOrISSN {
			return []Journal{j}, true
		}
	}
	return nil, false
}

// NameForISSN returns the display name for an ISSN, or the ISSN itself when
// not in the registry.
func NameForISSN(issn string) string {
	for _, j := range journals {
		if j.ISSN == issn {
			return j.Display
		}
	}
	return issn
}

// FamilyISSNFilter returns an OpenAlex filter clause scoping a query to the
// whole Lancet family, e.g. "primary_location.source.issn:0140-6736|1470-2045|...".
// The pipe separator is OpenAlex's OR operator.
func FamilyISSNFilter() string {
	issns := make([]string, 0, len(journals))
	for _, j := range journals {
		issns = append(issns, j.ISSN)
	}
	return "primary_location.source.issn:" + strings.Join(issns, "|")
}

// Slugs returns all known journal slugs plus the "all" alias, for help text.
func Slugs() []string {
	out := []string{"all"}
	for _, j := range Journals() {
		out = append(out, j.Slug)
	}
	return out
}
