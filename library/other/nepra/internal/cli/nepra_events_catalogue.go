// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-events-multipage-feed.json.

package cli

// eventSurface is one page of NEPRA's tariff-determination stream.
//
// The stream is the ONLY current surface on the site — it was last updated the
// day before the survey, while all four Main.htm workbook surfaces froze in
// 2022 — and it is spread over 29 pages rather than the single
// /tariff/Tariff.php the spec named, which returns a 9-byte HTTP 404.
type eventSurface struct {
	// ID is the --surface selector.
	ID string
	// Label is the human name.
	Label string
	// Path is the URL path, with NEPRA's own encoding preserved byte for byte.
	Path string
	// DISCO is set for the eleven ex-WAPDA distribution companies and
	// K-Electric, so --disco can select without the caller knowing paths.
	DISCO string
	// RowsAsOf is the row count MEASURED on AsOfDate. It is a FLOOR, not an
	// equality: the feed is live and grows. Coming back with fewer rows than
	// this is the signature of the documented silent truncation, where the
	// Wind page returned 1,028 of 2,440 rows under an HTTP 200 because gzip
	// was not requested.
	//
	// READ IT WITH RowsMeasured, NEVER ALONE. A bare 0 here is ambiguous and
	// this catalogue contains both meanings of it.
	RowsAsOf int
	// RowsMeasured says whether RowsAsOf is a measurement at all.
	//
	// It exists because ipp-waste publishes a MEASURED zero — the page is
	// reachable and genuinely empty — while thirteen other surfaces were
	// never counted, and an int cannot tell those apart. Collapsing them is
	// the exact blank-is-not-zero error this CLI refuses everywhere else in
	// NEPRA's own data, so it must not commit it in its own catalogue.
	RowsMeasured bool
}

// eventsAsOfDate is when RowsAsOf was measured.
const eventsAsOfDate = "2026-09-08"

// eventSurfaces is the catalogue. Row counts sum to 16,405 across the stream,
// spanning 27 Mar 1999 to 7 Sep 2026.
var eventSurfaces = []eventSurface{
	{ID: "ipp-thermal", Label: "IPP Thermal", Path: "/tariff/Generation%20IPPs%20Thermal.php", RowsAsOf: 5333, RowsMeasured: true},
	{ID: "ipp-wind", Label: "IPP Wind", Path: "/tariff/Generation%20IPPs%20Wind.php", RowsAsOf: 2440, RowsMeasured: true},
	{ID: "ipp-coal", Label: "IPP Coal", Path: "/tariff/Generation%20IPPs%20Coal.php", RowsAsOf: 1318, RowsMeasured: true},
	{ID: "ipp-solar", Label: "IPP Solar", Path: "/tariff/Generation%20IPPs%20Solar.php", RowsAsOf: 751, RowsMeasured: true},
	{ID: "ipp-bio", Label: "IPP Bio Energy", Path: "/tariff/Generation%20IPPs%20Bio%20Energy.php", RowsAsOf: 566, RowsMeasured: true},
	{ID: "ipp-hydel", Label: "IPP Hydel", Path: "/tariff/Generation%20IPPs%20Hydel.php", RowsAsOf: 481, RowsMeasured: true},
	{ID: "ipp-short-term", Label: "IPP Short Term", Path: "/tariff/Generation%20IPPs%20Short%20Term.php", RowsAsOf: 43, RowsMeasured: true},
	// Published, reachable, and genuinely EMPTY. Kept in the catalogue so a
	// zero here is a recorded fact rather than a missing page.
	{ID: "ipp-waste", Label: "IPP Waste to Energy", Path: "/tariff/Generation%20IPPs%20Waste%20to%20Energy.php", RowsAsOf: 0, RowsMeasured: true},

	{ID: "ke-distribution", Label: "K-Electric distribution/supply", Path: "/tariff/Distribution%20K-Electric.php", DISCO: "KE", RowsAsOf: 599, RowsMeasured: true},
	{ID: "ke-generation", Label: "K-Electric generation", Path: "/tariff/Generation%20K-Electric.php", DISCO: "KE", RowsAsOf: 9, RowsMeasured: true},

	{ID: "disco-fesco", Label: "FESCO", Path: "/tariff/Distribution%20FESCO.php", DISCO: "FESCO"},
	{ID: "disco-gepco", Label: "GEPCO", Path: "/tariff/Distribution%20GEPCO.php", DISCO: "GEPCO"},
	{ID: "disco-hazeco", Label: "HAZECO", Path: "/tariff/Distribution%20HAZECO.php", DISCO: "HAZECO"},
	{ID: "disco-hesco", Label: "HESCO", Path: "/tariff/Distribution%20HESCO.php", DISCO: "HESCO"},
	{ID: "disco-iesco", Label: "IESCO", Path: "/tariff/Distribution%20IESCO.php", DISCO: "IESCO"},
	{ID: "disco-lesco", Label: "LESCO", Path: "/tariff/Distribution%20LESCO.php", DISCO: "LESCO"},
	{ID: "disco-mepco", Label: "MEPCO", Path: "/tariff/Distribution%20MEPCO.php", DISCO: "MEPCO"},
	{ID: "disco-pesco", Label: "PESCO", Path: "/tariff/Distribution%20PESCO.php", DISCO: "PESCO"},
	{ID: "disco-qesco", Label: "QESCO", Path: "/tariff/Distribution%20QESCO.php", DISCO: "QESCO"},
	{ID: "disco-sepco", Label: "SEPCO", Path: "/tariff/Distribution%20SEPCO.php", DISCO: "SEPCO"},
	{ID: "disco-tesco", Label: "TESCO", Path: "/tariff/Distribution%20TESCO.php", DISCO: "TESCO"},

	{ID: "wapda-distribution", Label: "WAPDA (pre-unbundling)", Path: "/tariff/Distribution%20WAPDA.php", RowsAsOf: 6, RowsMeasured: true},
	{ID: "wapda-hydro", Label: "WAPDA Hydroelectric", Path: "/tariff/Generation%20WAPDA%20Hydroelectric.php", RowsAsOf: 29, RowsMeasured: true},
	{ID: "upfront", Label: "Upfront tariffs (16 technologies)", Path: "/tariff/Generation%20Upfront.php", RowsAsOf: 79, RowsMeasured: true},
	{ID: "petitions", Label: "Petitions", Path: "/tariff/Petitions.php", RowsAsOf: 698, RowsMeasured: true},
	{ID: "orders", Label: "Orders of the Authority", Path: "/M&E/Orders%20of%20the%20Authority.php", RowsAsOf: 278, RowsMeasured: true},
	{ID: "transmission-ntdc", Label: "Transmission NTDC", Path: "/tariff/Transmission%20NTDC.php"},
	{ID: "market-operators", Label: "Market Operators", Path: "/tariff/Market%20Operators.php"},
}

// eventsDISCORowsAsOf is the MEASURED total across the eleven ex-WAPDA
// distribution pages. The survey reported it as one figure for the group
// rather than per page, so it is asserted at the group level and each
// individual DISCO page carries no floor of its own — recording the number we
// actually have instead of splitting it eleven ways by guesswork.
const eventsDISCORowsAsOf = 3682

// eventSurfaceByID finds a catalogue entry.
func eventSurfaceByID(id string) (eventSurface, bool) {
	for _, s := range eventSurfaces {
		if s.ID == id {
			return s, true
		}
	}
	return eventSurface{}, false
}
