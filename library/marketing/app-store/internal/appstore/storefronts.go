// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package appstore

import (
	"fmt"
	"sort"
	"strings"
)

// Storefront is one App Store country storefront. ID is the numeric
// storefront id that leads the X-Apple-Store-Front header (us = 143441).
type Storefront struct {
	Code   string `json:"code"`
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
}

// storefrontTable lists Apple's App Store storefronts: the 175 countries in
// Apple Support 118205 (2026-09-30) plus BD and LI, which Apple's 2019
// affiliate appendix lists but the 2026 App Store list does not (Active=false).
// Sources: Apple affiliate "Linking" appendix (archived 2019-12-06), ipatool
// storefront.go, app-store-scraper PR #207 and the App Store Connect territory
// table; no source conflicts on any id.
var storefrontTable = []Storefront{
	{Code: "ae", ID: 143481, Name: "United Arab Emirates", Active: true},
	{Code: "af", ID: 143610, Name: "Afghanistan", Active: true},
	{Code: "ag", ID: 143540, Name: "Antigua and Barbuda", Active: true},
	{Code: "ai", ID: 143538, Name: "Anguilla", Active: true},
	{Code: "al", ID: 143575, Name: "Albania", Active: true},
	{Code: "am", ID: 143524, Name: "Armenia", Active: true},
	{Code: "ao", ID: 143564, Name: "Angola", Active: true},
	{Code: "ar", ID: 143505, Name: "Argentina", Active: true},
	{Code: "at", ID: 143445, Name: "Austria", Active: true},
	{Code: "au", ID: 143460, Name: "Australia", Active: true},
	{Code: "az", ID: 143568, Name: "Azerbaijan", Active: true},
	{Code: "ba", ID: 143612, Name: "Bosnia and Herzegovina", Active: true},
	{Code: "bb", ID: 143541, Name: "Barbados", Active: true},
	{Code: "bd", ID: 143490, Name: "Bangladesh", Active: false},
	{Code: "be", ID: 143446, Name: "Belgium", Active: true},
	{Code: "bf", ID: 143578, Name: "Burkina Faso", Active: true},
	{Code: "bg", ID: 143526, Name: "Bulgaria", Active: true},
	{Code: "bh", ID: 143559, Name: "Bahrain", Active: true},
	{Code: "bj", ID: 143576, Name: "Benin", Active: true},
	{Code: "bm", ID: 143542, Name: "Bermuda", Active: true},
	{Code: "bn", ID: 143560, Name: "Brunei", Active: true},
	{Code: "bo", ID: 143556, Name: "Bolivia", Active: true},
	{Code: "br", ID: 143503, Name: "Brazil", Active: true},
	{Code: "bs", ID: 143539, Name: "Bahamas", Active: true},
	{Code: "bt", ID: 143577, Name: "Bhutan", Active: true},
	{Code: "bw", ID: 143525, Name: "Botswana", Active: true},
	{Code: "by", ID: 143565, Name: "Belarus", Active: true},
	{Code: "bz", ID: 143555, Name: "Belize", Active: true},
	{Code: "ca", ID: 143455, Name: "Canada", Active: true},
	{Code: "cd", ID: 143613, Name: "Congo, Democratic Republic of the", Active: true},
	{Code: "cg", ID: 143582, Name: "Congo, Republic of the", Active: true},
	{Code: "ch", ID: 143459, Name: "Switzerland", Active: true},
	{Code: "ci", ID: 143527, Name: "Côte d'Ivoire", Active: true},
	{Code: "cl", ID: 143483, Name: "Chile", Active: true},
	{Code: "cm", ID: 143574, Name: "Cameroon", Active: true},
	{Code: "cn", ID: 143465, Name: "China mainland", Active: true},
	{Code: "co", ID: 143501, Name: "Colombia", Active: true},
	{Code: "cr", ID: 143495, Name: "Costa Rica", Active: true},
	{Code: "cv", ID: 143580, Name: "Cape Verde", Active: true},
	{Code: "cy", ID: 143557, Name: "Cyprus", Active: true},
	{Code: "cz", ID: 143489, Name: "Czechia", Active: true},
	{Code: "de", ID: 143443, Name: "Germany", Active: true},
	{Code: "dk", ID: 143458, Name: "Denmark", Active: true},
	{Code: "dm", ID: 143545, Name: "Dominica", Active: true},
	{Code: "do", ID: 143508, Name: "Dominican Republic", Active: true},
	{Code: "dz", ID: 143563, Name: "Algeria", Active: true},
	{Code: "ec", ID: 143509, Name: "Ecuador", Active: true},
	{Code: "ee", ID: 143518, Name: "Estonia", Active: true},
	{Code: "eg", ID: 143516, Name: "Egypt", Active: true},
	{Code: "es", ID: 143454, Name: "Spain", Active: true},
	{Code: "fi", ID: 143447, Name: "Finland", Active: true},
	{Code: "fj", ID: 143583, Name: "Fiji", Active: true},
	{Code: "fm", ID: 143591, Name: "Micronesia", Active: true},
	{Code: "fr", ID: 143442, Name: "France", Active: true},
	{Code: "ga", ID: 143614, Name: "Gabon", Active: true},
	{Code: "gb", ID: 143444, Name: "United Kingdom", Active: true},
	{Code: "gd", ID: 143546, Name: "Grenada", Active: true},
	{Code: "ge", ID: 143615, Name: "Georgia", Active: true},
	{Code: "gh", ID: 143573, Name: "Ghana", Active: true},
	{Code: "gm", ID: 143584, Name: "The Gambia", Active: true},
	{Code: "gr", ID: 143448, Name: "Greece", Active: true},
	{Code: "gt", ID: 143504, Name: "Guatemala", Active: true},
	{Code: "gw", ID: 143585, Name: "Guinea-Bissau", Active: true},
	{Code: "gy", ID: 143553, Name: "Guyana", Active: true},
	{Code: "hk", ID: 143463, Name: "Hong Kong", Active: true},
	{Code: "hn", ID: 143510, Name: "Honduras", Active: true},
	{Code: "hr", ID: 143494, Name: "Croatia", Active: true},
	{Code: "hu", ID: 143482, Name: "Hungary", Active: true},
	{Code: "id", ID: 143476, Name: "Indonesia", Active: true},
	{Code: "ie", ID: 143449, Name: "Ireland", Active: true},
	{Code: "il", ID: 143491, Name: "Israel", Active: true},
	{Code: "in", ID: 143467, Name: "India", Active: true},
	{Code: "iq", ID: 143617, Name: "Iraq", Active: true},
	{Code: "is", ID: 143558, Name: "Iceland", Active: true},
	{Code: "it", ID: 143450, Name: "Italy", Active: true},
	{Code: "jm", ID: 143511, Name: "Jamaica", Active: true},
	{Code: "jo", ID: 143528, Name: "Jordan", Active: true},
	{Code: "jp", ID: 143462, Name: "Japan", Active: true},
	{Code: "ke", ID: 143529, Name: "Kenya", Active: true},
	{Code: "kg", ID: 143586, Name: "Kyrgyzstan", Active: true},
	{Code: "kh", ID: 143579, Name: "Cambodia", Active: true},
	{Code: "kn", ID: 143548, Name: "St. Kitts and Nevis", Active: true},
	{Code: "kr", ID: 143466, Name: "South Korea", Active: true},
	{Code: "kw", ID: 143493, Name: "Kuwait", Active: true},
	{Code: "ky", ID: 143544, Name: "Cayman Islands", Active: true},
	{Code: "kz", ID: 143517, Name: "Kazakhstan", Active: true},
	{Code: "la", ID: 143587, Name: "Laos", Active: true},
	{Code: "lb", ID: 143497, Name: "Lebanon", Active: true},
	{Code: "lc", ID: 143549, Name: "St. Lucia", Active: true},
	{Code: "li", ID: 143522, Name: "Liechtenstein", Active: false},
	{Code: "lk", ID: 143486, Name: "Sri Lanka", Active: true},
	{Code: "lr", ID: 143588, Name: "Liberia", Active: true},
	{Code: "lt", ID: 143520, Name: "Lithuania", Active: true},
	{Code: "lu", ID: 143451, Name: "Luxembourg", Active: true},
	{Code: "lv", ID: 143519, Name: "Latvia", Active: true},
	{Code: "ly", ID: 143567, Name: "Libya", Active: true},
	{Code: "ma", ID: 143620, Name: "Morocco", Active: true},
	{Code: "md", ID: 143523, Name: "Moldova", Active: true},
	{Code: "me", ID: 143619, Name: "Montenegro", Active: true},
	{Code: "mg", ID: 143531, Name: "Madagascar", Active: true},
	{Code: "mk", ID: 143530, Name: "North Macedonia", Active: true},
	{Code: "ml", ID: 143532, Name: "Mali", Active: true},
	{Code: "mm", ID: 143570, Name: "Myanmar", Active: true},
	{Code: "mn", ID: 143592, Name: "Mongolia", Active: true},
	{Code: "mo", ID: 143515, Name: "Macao", Active: true},
	{Code: "mr", ID: 143590, Name: "Mauritania", Active: true},
	{Code: "ms", ID: 143547, Name: "Montserrat", Active: true},
	{Code: "mt", ID: 143521, Name: "Malta", Active: true},
	{Code: "mu", ID: 143533, Name: "Mauritius", Active: true},
	{Code: "mv", ID: 143488, Name: "Maldives", Active: true},
	{Code: "mw", ID: 143589, Name: "Malawi", Active: true},
	{Code: "mx", ID: 143468, Name: "Mexico", Active: true},
	{Code: "my", ID: 143473, Name: "Malaysia", Active: true},
	{Code: "mz", ID: 143593, Name: "Mozambique", Active: true},
	{Code: "na", ID: 143594, Name: "Namibia", Active: true},
	{Code: "ne", ID: 143534, Name: "Niger", Active: true},
	{Code: "ng", ID: 143561, Name: "Nigeria", Active: true},
	{Code: "ni", ID: 143512, Name: "Nicaragua", Active: true},
	{Code: "nl", ID: 143452, Name: "Netherlands", Active: true},
	{Code: "no", ID: 143457, Name: "Norway", Active: true},
	{Code: "np", ID: 143484, Name: "Nepal", Active: true},
	{Code: "nr", ID: 143606, Name: "Nauru", Active: true},
	{Code: "nz", ID: 143461, Name: "New Zealand", Active: true},
	{Code: "om", ID: 143562, Name: "Oman", Active: true},
	{Code: "pa", ID: 143485, Name: "Panama", Active: true},
	{Code: "pe", ID: 143507, Name: "Peru", Active: true},
	{Code: "pg", ID: 143597, Name: "Papua New Guinea", Active: true},
	{Code: "ph", ID: 143474, Name: "Philippines", Active: true},
	{Code: "pk", ID: 143477, Name: "Pakistan", Active: true},
	{Code: "pl", ID: 143478, Name: "Poland", Active: true},
	{Code: "pt", ID: 143453, Name: "Portugal", Active: true},
	{Code: "pw", ID: 143595, Name: "Palau", Active: true},
	{Code: "py", ID: 143513, Name: "Paraguay", Active: true},
	{Code: "qa", ID: 143498, Name: "Qatar", Active: true},
	{Code: "ro", ID: 143487, Name: "Romania", Active: true},
	{Code: "rs", ID: 143500, Name: "Serbia", Active: true},
	{Code: "ru", ID: 143469, Name: "Russia", Active: true},
	{Code: "rw", ID: 143621, Name: "Rwanda", Active: true},
	{Code: "sa", ID: 143479, Name: "Saudi Arabia", Active: true},
	{Code: "sb", ID: 143601, Name: "Solomon Islands", Active: true},
	{Code: "sc", ID: 143599, Name: "Seychelles", Active: true},
	{Code: "se", ID: 143456, Name: "Sweden", Active: true},
	{Code: "sg", ID: 143464, Name: "Singapore", Active: true},
	{Code: "si", ID: 143499, Name: "Slovenia", Active: true},
	{Code: "sk", ID: 143496, Name: "Slovakia", Active: true},
	{Code: "sl", ID: 143600, Name: "Sierra Leone", Active: true},
	{Code: "sn", ID: 143535, Name: "Senegal", Active: true},
	{Code: "sr", ID: 143554, Name: "Suriname", Active: true},
	{Code: "st", ID: 143598, Name: "São Tomé and Príncipe", Active: true},
	{Code: "sv", ID: 143506, Name: "El Salvador", Active: true},
	{Code: "sz", ID: 143602, Name: "Eswatini", Active: true},
	{Code: "tc", ID: 143552, Name: "Turks and Caicos", Active: true},
	{Code: "td", ID: 143581, Name: "Chad", Active: true},
	{Code: "th", ID: 143475, Name: "Thailand", Active: true},
	{Code: "tj", ID: 143603, Name: "Tajikistan", Active: true},
	{Code: "tm", ID: 143604, Name: "Turkmenistan", Active: true},
	{Code: "tn", ID: 143536, Name: "Tunisia", Active: true},
	{Code: "to", ID: 143608, Name: "Tonga", Active: true},
	{Code: "tr", ID: 143480, Name: "Türkiye", Active: true},
	{Code: "tt", ID: 143551, Name: "Trinidad and Tobago", Active: true},
	{Code: "tw", ID: 143470, Name: "Taiwan", Active: true},
	{Code: "tz", ID: 143572, Name: "Tanzania", Active: true},
	{Code: "ua", ID: 143492, Name: "Ukraine", Active: true},
	{Code: "ug", ID: 143537, Name: "Uganda", Active: true},
	{Code: "us", ID: 143441, Name: "United States", Active: true},
	{Code: "uy", ID: 143514, Name: "Uruguay", Active: true},
	{Code: "uz", ID: 143566, Name: "Uzbekistan", Active: true},
	{Code: "vc", ID: 143550, Name: "St. Vincent and the Grenadines", Active: true},
	{Code: "ve", ID: 143502, Name: "Venezuela", Active: true},
	{Code: "vg", ID: 143543, Name: "British Virgin Islands", Active: true},
	{Code: "vn", ID: 143471, Name: "Vietnam", Active: true},
	{Code: "vu", ID: 143609, Name: "Vanuatu", Active: true},
	{Code: "xk", ID: 143624, Name: "Kosovo", Active: true},
	{Code: "ye", ID: 143571, Name: "Yemen", Active: true},
	{Code: "za", ID: 143472, Name: "South Africa", Active: true},
	{Code: "zm", ID: 143622, Name: "Zambia", Active: true},
	{Code: "zw", ID: 143605, Name: "Zimbabwe", Active: true},
}

var storefrontByCode = func() map[string]Storefront {
	out := make(map[string]Storefront, len(storefrontTable))
	for _, sf := range storefrontTable {
		out[sf.Code] = sf
	}
	return out
}()

// storefrontAliases maps common country names and codes that differ from the
// ISO alpha-2 storefront code (uk -> gb) to the storefront code. The learn
// seeds in spec.yaml use the same aliases.
var storefrontAliases = map[string]string{
	"uk":                   "gb",
	"united kingdom":       "gb",
	"britain":              "gb",
	"usa":                  "us",
	"america":              "us",
	"united states":        "us",
	"uae":                  "ae",
	"united arab emirates": "ae",
	"ksa":                  "sa",
	"saudi arabia":         "sa",
	"turkey":               "tr",
	"turkiye":              "tr",
	"korea":                "kr",
	"south korea":          "kr",
	"pakistan":             "pk",
	"india":                "in",
	"japan":                "jp",
	"germany":              "de",
	"france":               "fr",
	"canada":               "ca",
	"australia":            "au",
	"brazil":               "br",
	"china":                "cn",
	"indonesia":            "id",
	"mexico":               "mx",
	"spain":                "es",
	"italy":                "it",
}

// Storefronts returns every known storefront (active and legacy) sorted by code.
func Storefronts() []Storefront {
	out := make([]Storefront, len(storefrontTable))
	copy(out, storefrontTable)
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// ActiveStorefrontCodes returns the 175 current App Store storefront codes, sorted.
func ActiveStorefrontCodes() []string {
	out := make([]string, 0, len(storefrontTable))
	for _, sf := range storefrontTable {
		if sf.Active {
			out = append(out, sf.Code)
		}
	}
	sort.Strings(out)
	return out
}

// LookupStorefront resolves a storefront code (case-insensitive) or a known
// alias ("uk", "usa", "pakistan") to its storefront.
func LookupStorefront(code string) (Storefront, bool) {
	key := strings.ToLower(strings.TrimSpace(code))
	if alias, ok := storefrontAliases[key]; ok {
		key = alias
	}
	sf, ok := storefrontByCode[key]
	return sf, ok
}

// StorefrontID returns the numeric storefront id for a two-letter code.
func StorefrontID(code string) (int, bool) {
	sf, ok := LookupStorefront(code)
	if !ok {
		return 0, false
	}
	return sf.ID, true
}

// NormalizeCountry validates a storefront code or alias and returns the
// lowercase two-letter code Apple's URLs use.
func NormalizeCountry(code string) (string, error) {
	if strings.TrimSpace(code) == "" {
		return "", fmt.Errorf("storefront country is empty; use a two-letter code such as us, gb or pk")
	}
	sf, ok := LookupStorefront(code)
	if !ok {
		return "", fmt.Errorf("unknown App Store storefront %q; use a two-letter country code such as us, gb, jp or pk", code)
	}
	return sf.Code, nil
}

// ParseCountries parses a comma-separated storefront list ("us,gb,pk") or
// "all" (the 175 active storefronts). Codes are normalized, validated and
// de-duplicated in input order.
func ParseCountries(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("no storefronts given; use a list such as us,gb,pk or all")
	}
	if strings.EqualFold(raw, "all") {
		return ActiveStorefrontCodes(), nil
	}
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		code, err := NormalizeCountry(part)
		if err != nil {
			return nil, err
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no storefronts given; use a list such as us,gb,pk or all")
	}
	return out, nil
}

// StoreFrontHeader returns the X-Apple-Store-Front value for a storefront and
// client platform id (12 makes the search-hints endpoint answer a plist).
func StoreFrontHeader(code string, platformID int) (string, error) {
	id, ok := StorefrontID(code)
	if !ok {
		return "", fmt.Errorf("unknown App Store storefront %q", code)
	}
	return fmt.Sprintf("%d,%d", id, platformID), nil
}
