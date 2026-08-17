// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "strings"

// SpainAirport maps a Spanish airport across the sources that use different
// location identifiers: IATA (Rentalcars, and the human-facing code) and the
// DoYouSpain internal destination code. Codes verified via DoYouSpain
// autocomplete.
type SpainAirport struct {
	IATA           string // e.g. AGP
	Name           string // e.g. Málaga Airport
	DoYouSpainCode string // e.g. MAL02
}

// spainAirports is the built-in table of Spain's commercial airports — every
// AENA field with scheduled passenger service and car-rental desks, across the
// mainland, the Balearics, the Canaries and Melilla.
//
// Order matters for the name-substring fallback in ResolveAirport: the busier
// airport of a same-named pair is listed first, so "palma" resolves to Palma de
// Mallorca (PMI) rather than La Palma (SPC), and "tenerife" to Tenerife South
// (TFS) rather than Tenerife North (TFN). Málaga stays at index 0 as the
// default location.
var spainAirports = []SpainAirport{
	// Mainland — major
	{IATA: "AGP", Name: "Málaga Airport", DoYouSpainCode: "MAL02"},
	{IATA: "ALC", Name: "Alicante Airport", DoYouSpainCode: "ALC01"},
	{IATA: "BCN", Name: "Barcelona Airport", DoYouSpainCode: "BNA02"},
	{IATA: "MAD", Name: "Madrid Airport", DoYouSpainCode: "MAD02"},
	{IATA: "VLC", Name: "Valencia Airport", DoYouSpainCode: "VAL01"},
	{IATA: "SVQ", Name: "Seville Airport", DoYouSpainCode: "SVI02"},
	{IATA: "BIO", Name: "Bilbao Airport", DoYouSpainCode: "BIL02"},
	{IATA: "GRO", Name: "Girona Airport", DoYouSpainCode: "GIR01"},
	{IATA: "RMU", Name: "Murcia (Corvera) Airport", DoYouSpainCode: "MUR01"},
	// Mainland — regional
	{IATA: "REU", Name: "Reus Airport", DoYouSpainCode: "REU01"},
	{IATA: "LEI", Name: "Almería Airport", DoYouSpainCode: "ALM01"},
	{IATA: "GRX", Name: "Granada Airport", DoYouSpainCode: "GRA02"},
	{IATA: "XRY", Name: "Jerez Airport", DoYouSpainCode: "JRZ01"},
	{IATA: "ZAZ", Name: "Zaragoza Airport", DoYouSpainCode: "ZRG02"},
	{IATA: "SCQ", Name: "Santiago de Compostela Airport", DoYouSpainCode: "SGO01"},
	{IATA: "OVD", Name: "Asturias Airport", DoYouSpainCode: "AVI02"},
	{IATA: "VGO", Name: "Vigo Airport", DoYouSpainCode: "VGO02"},
	{IATA: "LCG", Name: "A Coruña Airport", DoYouSpainCode: "COR02"},
	{IATA: "SDR", Name: "Santander Airport", DoYouSpainCode: "SNT02"},
	{IATA: "EAS", Name: "San Sebastián Airport", DoYouSpainCode: "SSB01"},
	{IATA: "VLL", Name: "Valladolid Airport", DoYouSpainCode: "VLL01"},
	{IATA: "PNA", Name: "Pamplona Airport", DoYouSpainCode: "PMP02"},
	// Balearic Islands
	{IATA: "PMI", Name: "Palma de Mallorca Airport", DoYouSpainCode: "PMA02@2"},
	{IATA: "IBZ", Name: "Ibiza Airport", DoYouSpainCode: "IBI01"},
	{IATA: "MAH", Name: "Menorca Airport", DoYouSpainCode: "MNC01"},
	// Canary Islands
	{IATA: "TFS", Name: "Tenerife South Airport", DoYouSpainCode: "TNF02"},
	{IATA: "TFN", Name: "Tenerife North Airport", DoYouSpainCode: "TNF01"},
	{IATA: "LPA", Name: "Gran Canaria (Las Palmas) Airport", DoYouSpainCode: "GCA01@1"},
	{IATA: "ACE", Name: "Lanzarote Airport", DoYouSpainCode: "LNZ02"},
	{IATA: "FUE", Name: "Fuerteventura Airport", DoYouSpainCode: "FUE02"},
	{IATA: "SPC", Name: "La Palma Airport", DoYouSpainCode: "LPA02"},
	// Autonomous city
	{IATA: "MLN", Name: "Melilla Airport", DoYouSpainCode: "MLN01"},
}

// SpainAirports returns the built-in airport table.
func SpainAirports() []SpainAirport { return spainAirports }

// accentFolds maps the accented runes used in Spanish place names to their
// plain ASCII equivalent, so a query typed without accents ("malaga",
// "almeria", "coruna") still matches "Málaga", "Almería", "A Coruña".
var accentFolds = strings.NewReplacer(
	"á", "a", "à", "a", "ä", "a", "â", "a",
	"é", "e", "è", "e", "ë", "e", "ê", "e",
	"í", "i", "ì", "i", "ï", "i", "î", "i",
	"ó", "o", "ò", "o", "ö", "o", "ô", "o",
	"ú", "u", "ù", "u", "ü", "u", "û", "u",
	"ñ", "n", "ç", "c",
)

// foldName lowercases a string and strips Spanish accents for comparison.
func foldName(s string) string {
	return accentFolds.Replace(strings.ToLower(strings.TrimSpace(s)))
}

// ResolveAirport matches a user query against the airport table by IATA code,
// DoYouSpain code, or an accent-insensitive name/city substring. Returns the
// airport and true on a match; an empty query defaults to Málaga.
func ResolveAirport(query string) (SpainAirport, bool) {
	q := strings.ToUpper(strings.TrimSpace(query))
	if q == "" {
		return spainAirports[0], true // default: Málaga
	}
	// Exact IATA or DoYouSpain code.
	for _, a := range spainAirports {
		if q == a.IATA || q == strings.ToUpper(a.DoYouSpainCode) {
			return a, true
		}
	}
	// Name/city substring, accent- and case-insensitive. The table is ordered
	// so the busier airport of a same-named pair wins.
	ql := foldName(query)
	for _, a := range spainAirports {
		if strings.Contains(foldName(a.Name), ql) {
			return a, true
		}
	}
	return SpainAirport{}, false
}
