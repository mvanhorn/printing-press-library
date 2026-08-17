// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import (
	"regexp"
	"strings"
)

// supplierNames maps DoYouSpain data-prv codes to human supplier names.
// Codes verified against a live Málaga results page (2026-07-13); a few are
// best-effort and fall through to the raw code when unknown.
var supplierNames = map[string]string{
	"PAS":  "Delpaso",
	"REC":  "Record Go",
	"RBC1": "Record Go",
	"WIB":  "Wiber",
	"SXT":  "Sixt",
	"EUK":  "Europcar",
	"EU2":  "Europcar",
	"GOB":  "Goldcar",
	"GOB1": "Goldcar",
	"OKR1": "OK Mobility",
	"NIZ1": "Niza Cars",
	"CRX":  "Centauro",
	"DRV1": "Drivalia",
	"ENT":  "Enterprise",
	"FLW":  "Firefly",
	"FLS":  "Firefly",
	"ALM":  "Alamo",
	"CYR1": "Centauro",
	"BGX":  "Budget",
	"SUR":  "Surprice",
	"WHE":  "Wheego",
	"LID":  "Lidl Cars",
	"SLO":  "Solmar",
	"ECR":  "Europcar",
	"MAL":  "Malco",
	"ALQ":  "Alquiber",
}

// SupplierName returns the human name for a DoYouSpain supplier code, or the
// code itself when unknown.
func SupplierName(code string) string {
	if n, ok := supplierNames[strings.ToUpper(strings.TrimSpace(code))]; ok {
		return n
	}
	return code
}

// canonicalAliases folds source-specific spellings of the same real company to
// one canonical display name, so ratings and prices join across sources. Keys
// are the normalized form produced by canonicalKey (lowercase alnum, trailing
// car/cars/rentacar stripped, "NR"/"NL" no-reserve markers removed).
var canonicalAliases = map[string]string{
	"niza":            "Niza",
	"nizacars":        "Niza",
	"recordgo":        "Record Go",
	"record":          "Record Go",
	"okmobility":      "OK Mobility",
	"ok":              "OK Mobility",
	"keddybyeuropcar": "Keddy",
	"keddy":           "Keddy",
	"goldcar":         "Goldcar",
	"centauro":        "Centauro",
	"drivalia":        "Drivalia",
	"delpaso":         "Delpaso",
	"europcar":        "Europcar",
	"gobycar":         "Goby Car",
	"rentbycar":       "Rent By Car",
	"clickrent":       "Clickrent",
	"click":           "Clickrent",
	"flizzr":          "Flizzr",
	"flizzrbysixt":    "Flizzr",
	"national":        "National",
	"alamo":           "Alamo",
	"avis":            "Avis",
	"budget":          "Budget",
	"sixt":            "Sixt",
	"enterprise":      "Enterprise",
	"hertz":           "Hertz",
	"dollar":          "Dollar",
	"thrifty":         "Thrifty",
	"firefly":         "Firefly",
	"wiber":           "Wiber",
}

var nrMarkerRe = regexp.MustCompile(`(?i)\s+n[rl]$`)

// canonicalKey normalizes a supplier name to a lookup key: strip a trailing
// " NR"/" NL" no-reserve marker, then lowercase and keep letters/digits only.
func canonicalKey(name string) string {
	name = nrMarkerRe.ReplaceAllString(strings.TrimSpace(name), "")
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CanonicalSupplier normalizes a free-text supplier name from any source to a
// stable canonical display name so the same real company matches across
// DoYouSpain, Rentalcars, and the direct clients (e.g. "ALAMO"/"Alamo" → "Alamo",
// "Niza Cars"/"Nizacars" → "Niza", "OK MOBILITY NR" → "OK Mobility"). Unknown
// names are title-cased so casing differences still merge.
func CanonicalSupplier(name string) string {
	trimmed := nrMarkerRe.ReplaceAllString(strings.TrimSpace(name), "")
	if trimmed == "" {
		return ""
	}
	if canon, ok := canonicalAliases[canonicalKey(name)]; ok {
		return canon
	}
	return titleCase(trimmed)
}

// titleCase renders "ALAMO"/"alamo" → "Alamo", preserving multi-word names.
func titleCase(s string) string {
	fields := strings.Fields(strings.ToLower(s))
	for i, f := range fields {
		r := []rune(f)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		fields[i] = string(r)
	}
	return strings.Join(fields, " ")
}

// SupplierAliases maps user-facing supplier keywords (as passed to
// --supplier) to the set of human names they should match. Keys are
// lowercase; matching is done case-insensitively against Offer.Supplier.
var supplierAliases = map[string][]string{
	"delpaso":   {"Delpaso"},
	"recordgo":  {"Record Go"},
	"record":    {"Record Go"},
	"wiber":     {"Wiber"},
	"sixt":      {"Sixt"},
	"europcar":  {"Europcar"},
	"goldcar":   {"Goldcar"},
	"okmobility": {"OK Mobility"},
	"ok":        {"OK Mobility"},
	"centauro":  {"Centauro"},
	"enterprise": {"Enterprise"},
	"firefly":   {"Firefly"},
}

// MatchesSupplier reports whether an offer's supplier matches a user-supplied
// keyword. It accepts both known aliases and free substrings so
// "--supplier drivalia" still works for suppliers not in the alias table.
func MatchesSupplier(offerSupplier, keyword string) bool {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	offer := strings.ToLower(strings.TrimSpace(offerSupplier))
	if keyword == "" {
		return true
	}
	if names, ok := supplierAliases[keyword]; ok {
		for _, n := range names {
			if strings.EqualFold(offerSupplier, n) {
				return true
			}
		}
		return false
	}
	return strings.Contains(offer, keyword)
}
