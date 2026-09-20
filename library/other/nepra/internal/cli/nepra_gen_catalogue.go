// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-gen-plant-month-extract.json.

package cli

import (
	"strings"
)

// genAsOfDate is when every floor in genYears was MEASURED, by fetching the
// seven live URLs and the two recorded 404s from nepra.org.pk.
//
// It is not a publication date and not a guess. A floor without a date is an
// assertion nobody can re-check.
const genAsOfDate = "2026-09-10"

// genSheetPathTemplate is the generation workbook path.
//
// TWO THINGS HERE ARE LOAD-BEARING AND BOTH LOOK LIKE TYPOS.
//
//  1. "Genenration" is NEPRA's own misspelling. The correctly spelled path
//     404s with a 9-byte "Not Found" body.
//  2. The year token is bare ("2023-24"), never "FY2023-24". The FY-prefixed
//     form 404s the same way.
//
// The %20s are ALREADY percent-encoded, and that is why the year token is
// substituted with strings.ReplaceAll rather than either of the two obvious
// alternatives:
//
//   - fmt.Sprintf is a TRAP HERE, not a style choice. Six of the seven %20
//     sequences in this template parse as format verbs (%20o, %20I, %20R,
//     %20C, %20G, %20w), so a Sprintf call silently rewrites NEPRA's own
//     percent-encoding into padded octal and hexadecimal. `go vet` catches
//     the first one; it does not catch what the rest would do to the URL.
//   - the generated replacePathParam routes the value through url.PathEscape.
//     That is a no-op on "2023-24" today, but it is the same re-encoding of an
//     already-encoded path that turned the reliability resource into a 404
//     (patch nepra-multi-segment-path-param).
const genSheetPathTemplate = "/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation/" +
	"List%20of%20Companies%20Genenration%20wise%20{fy}_files/sheet001.htm"

// genSheetPath builds the path for one fiscal year from its bare label.
func genSheetPath(label string) string {
	return strings.ReplaceAll(genSheetPathTemplate, "{fy}", label)
}

// genYear is one fiscal year of the generation panel, with the floors that
// were measured for it.
type genYear struct {
	// Label is the URL token and the fiscal year as published, "2023-24".
	Label string
	// Reachable records whether the URL returned a workbook on genAsOfDate.
	// An unreachable year is UNPUBLISHED AT THIS PATH, which is not the same
	// as a year that published no plants.
	Reachable bool
	// DecodedBytes is the decoded body length measured on genAsOfDate with
	// Accept: */*. All seven reachable years reproduce their byte count
	// exactly on re-fetch.
	//
	// It is asserted as a FLOOR, not an equality, and the reason is
	// measured: this CLI's client sends Accept: text/html, and Cloudflare
	// then injects a ~361-byte RUM beacon <script> after </html>. FY2023-24
	// arrives as 493,548 bytes through the client against 493,187 bytes
	// through Accept: */*, byte-identical up to that injection. The floor
	// catches the failure that matters — a truncated body under an HTTP 200,
	// which is how this site's silent truncation presents — without failing
	// on the CDN's own footer.
	DecodedBytes int
	// PlantRows is the plant-row count measured on genAsOfDate. 0 means NOT
	// MEASURED and asserts nothing — never "no plants".
	PlantRows int
	// PlantRowsPinned reports whether PlantRows is backed by a committed
	// fixture in internal/cli/testdata, so a test re-derives it on every
	// run. Three of the seven years are; the other four were measured once
	// against the live bytes and are not pinned.
	PlantRowsPinned bool
	// StatusRows is the number of plants whose whole monthly block is a
	// status sentinel, measured on genAsOfDate. DELICENSED and
	// DECOMMISSIONED appear ONLY in the last two published years, so a
	// non-zero count in an earlier year means the parse or the source has
	// changed. -1 means not measured.
	StatusRows int
	// Note carries anything a caller must know before quoting the year.
	Note string
}

// genYears is the catalogue.
//
// Byte counts and the two 404s were re-measured live on genAsOfDate; the seven
// reachable years returned exactly these lengths and FY2016-17 and FY2024-25
// returned HTTP 404 with a 9-byte "Not Found" body. FY2015-16 and FY2025-26
// are recorded as unreachable on the same evidence pattern but were NOT probed
// on that date, which is why their note says so.
var genYears = []genYear{
	{Label: "2015-16", Reachable: false, StatusRows: -1,
		Note: "not published at this path; NOT probed on " + genAsOfDate + " (the two adjacent years that were probed both 404)"},
	{Label: "2016-17", Reachable: false, StatusRows: -1,
		Note: "HTTP 404, 9-byte body, measured on " + genAsOfDate},
	{Label: "2017-18", Reachable: true, DecodedBytes: 426282, PlantRows: 108, PlantRowsPinned: true, StatusRows: 0,
		Note: "11 plants carry an all-blank monthly block AND a blank Installed Capacity, so this year's " +
			"listed_no_data capacity is UNMEASURED rather than 0 MW"},
	{Label: "2018-19", Reachable: true, DecodedBytes: 418887, PlantRows: 108, StatusRows: 0,
		Note: "6 plants carry an all-blank monthly block; 1 published Sum disagrees with its own twelve months"},
	{Label: "2019-20", Reachable: true, DecodedBytes: 421591, PlantRows: 108, StatusRows: 0,
		Note: "1 plant carries an all-blank monthly block; 1 published Sum disagrees with its own twelve months by -142.45 GWh"},
	{Label: "2020-21", Reachable: true, DecodedBytes: 455640, PlantRows: 108, PlantRowsPinned: true, StatusRows: 0,
		Note: "3 plants carry \"Export to K.Electric\" in the Installed Capacity column, not in the monthly block"},
	{Label: "2021-22", Reachable: true, DecodedBytes: 516219, PlantRows: 125, StatusRows: 0,
		Note: "NINE published Sums disagree with their own twelve months, from -33.78 to +1.01 GWh; " +
			"every one is reported and none is corrected"},
	{Label: "2022-23", Reachable: true, DecodedBytes: 490461, PlantRows: 130, StatusRows: 12,
		Note: "first year to publish the status vocabulary: 11 DELICENSED + 1 DECOMMISSIONED, PLUS 2 rows " +
			"whose 26 monthly cells read the bare word \"DELICENSE\" (no trailing D). Those 52 cells are " +
			"unmodelled text and their rows are classed unmodelled_block, not delicensed"},
	{Label: "2023-24", Reachable: true, DecodedBytes: 493187, PlantRows: 133, PlantRowsPinned: true, StatusRows: 13,
		Note: "12 DELICENSED + 1 DECOMMISSIONED status rows, plus 2 rows listed WITH capacity (97 + 84 MW) and no monthly data"},
	{Label: "2024-25", Reachable: false, StatusRows: -1,
		Note: "HTTP 404, 9-byte body, measured on " + genAsOfDate},
	{Label: "2025-26", Reachable: false, StatusRows: -1,
		Note: "not published at this path; NOT probed on " + genAsOfDate + " (the fiscal year had not ended)"},
}

// genYearByLabel looks a fiscal year up in the catalogue.
func genYearByLabel(label string) (genYear, bool) {
	for _, y := range genYears {
		if y.Label == label {
			return y, true
		}
	}
	return genYear{}, false
}

// genReachableLabels lists the years that returned a workbook on genAsOfDate.
func genReachableLabels() []string {
	var out []string
	for _, y := range genYears {
		if y.Reachable {
			out = append(out, y.Label)
		}
	}
	return out
}

// genReachableList renders the reachable years for an error message.
func genReachableList() string {
	return strings.Join(genReachableLabels(), ", ")
}
