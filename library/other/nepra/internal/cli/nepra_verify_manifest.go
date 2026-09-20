// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Patch record to be written at .printing-press-patches/nepra-verify-fetch-gate.json;
// it is not in this change because the writable set for this task is source only.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// verifyManifestAsOf is when the expectations in this file were last measured
// against the live site. Every byte floor and every decoy below was re-fetched
// from nepra.org.pk on this date; the per-surface fields carry their own
// measurement dates where they differ.
//
// This is the date `--check-stale` measures the manifest's own age against.
const verifyManifestAsOf = "2026-09-10"

// verifyStaleAfterDefault is the staleness window in days.
//
// 90 is the parity target: ebillpakistan.pk's scripts/tariff-staleness.mjs
// uses STALE_AFTER_DAYS=90 and exits 1 past it. Off unless --check-stale is
// passed, so a cron opts into the gate deliberately.
const verifyStaleAfterDefault = 90

// verifyNewestPublishedFY is the newest fiscal year the Detail-of-Generation
// series publishes, and verifyNewestFYCoverageEnd is the last day that year
// covers. The series is frozen here: FY2024-25 and FY2025-26 are both a 9-byte
// HTTP 404 (re-measured live on verifyManifestAsOf), and on 19 Jan 2026 the
// Federal Minister for Power called the State of Industry Report five months
// late and "based on incomplete and inaccurate data".
const (
	verifyNewestPublishedFY    = "2023-24"
	verifyNewestFYCoverageEnd  = "2024-06-30"
	verifyCanonicalColumnCount = nepraparse.LogicalColumns
)

// verifySurfaceKind values.
const (
	verifyKindWorkbook = "generation-workbook"
	verifyKindSheet    = "excel-sheet"
)

// verifyInternals are a surface's structural numbers as MEASURED, and they
// exist only for surfaces whose internals were actually observed.
//
// The struct is reached through a POINTER on verifySurface precisely so that
// "never measured" is distinguishable from "measured as zero". Four of the
// seven reachable generation years — gen-2018-19, gen-2019-20, gen-2021-22
// and gen-2022-23 — had their INTERNALS measured only once and never on a
// second date, so their pointer is nil and `verify` reports them under
// measured_but_not_asserted instead of gating on them.
//
// NOTE THE DISTINCTION, because an earlier version of this comment blurred
// it: all seven years DO now carry a committed workbook fixture under
// internal/cli/testdata/workbook-fy*.htm.gz. Having the bytes is not the same
// as having observed the internals twice, and it is the second observation
// that licenses gating on them.
//
// That is not caution for its own sake. Re-measuring those four live on
// verifyManifestAsOf showed their internals genuinely DIFFER from the three
// fixtured years, which is exactly why they must not inherit a floor:
// FY2018-19 and FY2019-20 have a physical width of 32, not the 39/40 the
// settled facts record, and their raw <td> counts are 3,536 and 3,567 against
// 4,479 for the same 108 plants; FY2021-22 fails the Sum==sum(12 months)
// identity on 9 of 125 rows and FY2019-20 on 1 of 107 by -142.45 GWh, so a
// gate that asserted the invariant everywhere would fail on published data.
type verifyInternals struct {
	// InternalsAsOf is when these numbers were measured.
	InternalsAsOf string
	// TableCount and HeaderCellCount are 1 and 0 in every year sampled, and
	// they are asserted for EVERY surface rather than only the fixtured
	// ones: zero <th> is why header detection here is text-matched, and a
	// second table would mean the export changed shape.
	TableRows     int
	RawCells      int
	Plants        int
	PhysicalWidth int
	// NBSPBytes is the count of 0xA0 bytes, the only non-ASCII byte in these
	// files. It is positive proof from the payload itself that windows-1252
	// is load-bearing rather than a guess.
	NBSPBytes int
	// SumPassed/SumEligible are Workbook.CheckSum(FieldGWh, 0).
	SumPassed   int
	SumEligible int
	// Ineligible rows and their state breakdown. Ineligible rows are
	// EXCLUDED from the sum and counted; they are never read as zero.
	Ineligible               int
	IneligibleDelicensed     int
	IneligibleDecommissioned int
	IneligibleNotReported    int
	IneligibleUnknownText    int
	// SigmaSumGWh is the total of the Sum-GWh column over numeric rows only.
	SigmaSumGWh float64
	// GridRows and GridRawCells are the pre-shaping grid numbers for an
	// Excel sheet, and ExtractedRows is what extractNepraTable yields.
	GridRows      int
	GridRawCells  int
	ExtractedRows int
}

// verifySurface is one catalogued surface and the frozen expectation for it.
type verifySurface struct {
	// ID is the --surface selector, or "gen-<fy>" for a generation year.
	ID string
	// Kind is verifyKindWorkbook or verifyKindSheet.
	Kind string
	// Label is the human name.
	Label string
	// Path is the request path. For a generation workbook it is built from
	// FiscalYear.Label() through the shared resource template, NEVER from a
	// caller's raw --fy argument: the path token is "2023-24", and the
	// FY-prefixed form "FY2023-24" is an HTTP 404 with a 9-byte body on
	// every year.
	Path string
	// FY is the fiscal year for a workbook surface, "" for a sheet.
	FY string
	// ExpectedBandLabel is the in-document <td colspan=26> band label, the
	// ONLY place the file states which year it holds. The filename is an
	// untrusted hint: "SIR Data 2025.htm" returns 200 and frames FY2023-24.
	ExpectedBandLabel string
	// ByteFloor is a FLOOR on the decoded body length, not an equality:
	// NEPRA may add rows. It is per-surface because the frameset decoy is
	// byte-identical across years, so one global floor would let a shell
	// served for an unfloored year through.
	ByteFloor int
	// ByteFloorAsOf is when ByteFloor was measured.
	ByteFloorAsOf string
	// RowFloor is the extracted-row floor for a sheet. 0 asserts nothing.
	RowFloor int
	// Asserted says whether this surface's INTERNALS may fail the gate.
	// False means report-only. See verifyInternals for the measured reason.
	Asserted bool
	// Internals is nil when the internals were never measured.
	Internals *verifyInternals
}

// verifyDecoy is a body that returns like data and is not data. Each was
// re-fetched live on verifyManifestAsOf; the recorded lengths are what came
// back after Content-Encoding was removed.
type verifyDecoy struct {
	ID string `json:"decoy"`
	// Bytes is the decoded length. 0 means the length varies.
	Bytes int `json:"bytes"`
	// HTTPStatus is what the site actually answers with.
	HTTPStatus int    `json:"http_status"`
	Evidence   string `json:"evidence"`
}

// verifyDecoys is the complete set of decoys this gate knows how to refuse.
var verifyDecoys = []verifyDecoy{
	{
		ID: "404-stub", Bytes: 9, HTTPStatus: 404,
		Evidence: "Body is literally \"Not Found\" (hex 4e6f7420466f756e64) served as " +
			"Content-Type text/html; charset=iso-8859-1. That iso-8859-1 default is the " +
			"origin of the whole encoding trap: the data files declare windows-1252 " +
			"in-document and nothing in the HTTP header.",
	},
	{
		ID: "frameset-shell", Bytes: 9838, HTTPStatus: 200,
		Evidence: "The per-year parent .htm (no _files/sheet001.htm) is a <frameset rows=\"*,18\"> " +
			"shell with frames frSheet/frScroll/frTabs and no numbers in it, and it is " +
			"byte-identical across FY2017-18, FY2020-21 and FY2023-24 — three years of " +
			"\"data\" that is one file. It returns HTTP 200, and the grid built from it has " +
			"0 tables and 0 cells.",
	},
	{
		ID: "sir-stub", Bytes: 374, HTTPStatus: 200,
		Evidence: "SIR Data 2024.htm and SIR Data 2025.htm are both 200, both exactly 374 bytes, " +
			"both md5 37c27f4decfbad45c80078a997f13c19, both titled <title>SIR Data 2024</title>, " +
			"and both frame List%20of%20Companies%20Genenration%20wise%202023-24.htm. " +
			"Enumerating years by filename therefore double-counts FY2023-24 as FY2024-25 " +
			"behind a real HTTP 200.",
	},
}

// verifyUnpublishedYear is a well-formed fiscal year the series does not
// publish, with the evidence for saying so. A --fy naming one of these is
// answered from this record and exits 3; it is NOT probed live and presented
// as a discovery. Pass --check-stale to probe whether that has changed.
type verifyUnpublishedYear struct {
	FY       string `json:"fy"`
	Evidence string `json:"evidence"`
	AsOf     string `json:"measured_on"`
}

var verifyUnpublishedYears = []verifyUnpublishedYear{
	{FY: "2015-16", Evidence: "HTTP 404, 9-byte body. Before the series starts.", AsOf: "2026-09-08"},
	{FY: "2016-17", Evidence: "HTTP 404, 9-byte body. Before the series starts.", AsOf: "2026-09-08"},
	{FY: "2024-25", Evidence: "HTTP 404, 9-byte body (re-measured live). The series is frozen at FY2023-24.", AsOf: verifyManifestAsOf},
	{FY: "2025-26", Evidence: "HTTP 404, 9-byte body. The series is frozen at FY2023-24.", AsOf: "2026-09-08"},
}

func verifyUnpublishedYearFor(label string) (verifyUnpublishedYear, bool) {
	for _, y := range verifyUnpublishedYears {
		if y.FY == label {
			return y, true
		}
	}
	return verifyUnpublishedYear{}, false
}

// verifyGenerationPath builds the request path for one fiscal year from the
// SHARED resource template rather than a second copy of it, so the path (and
// NEPRA's own upstream typo "Genenration") cannot drift from the promoted
// `generation year` command.
func verifyGenerationPath(fy nepraparse.FiscalYear) (string, error) {
	return resourceDetailPath("generation", fy.Label())
}

// verifyGenerationSurface builds the manifest entry for one fiscal year.
func verifyGenerationSurface(label string, floor int, floorAsOf string, internals *verifyInternals) verifySurface {
	fy, err := nepraparse.ParseFiscalYear(label)
	if err != nil {
		// Unreachable: the labels below are literals in this file, and the
		// manifest consistency test parses every one of them.
		panic(fmt.Sprintf("verify manifest: unparseable fiscal year %q: %v", label, err))
	}
	path, err := verifyGenerationPath(fy)
	if err != nil {
		panic(fmt.Sprintf("verify manifest: no generation path template: %v", err))
	}
	return verifySurface{
		ID:                "gen-" + fy.Label(),
		Kind:              verifyKindWorkbook,
		Label:             "Detail of Generation, FY" + fy.Label(),
		Path:              path,
		FY:                fy.Label(),
		ExpectedBandLabel: fy.BandLabel(),
		ByteFloor:         floor,
		ByteFloorAsOf:     floorAsOf,
		Asserted:          internals != nil,
		Internals:         internals,
	}
}

// verifySurfaces is the frozen expectation manifest: 7 generation workbooks
// and 4 Excel data sheets.
//
// The three fixtured years carry internals; the other four carry a byte floor
// measured live on verifyManifestAsOf and nothing else. All seven byte floors
// were re-fetched live on that date and every one reproduced exactly.
var verifySurfaces = buildVerifySurfaces()

func buildVerifySurfaces() []verifySurface {
	out := []verifySurface{
		verifyGenerationSurface("2017-18", 426282, "2026-09-10", &verifyInternals{
			InternalsAsOf: "2026-09-08",
			TableRows:     114, RawCells: 4479, Plants: 108, PhysicalWidth: 40, NBSPBytes: 128,
			SumPassed: 97, SumEligible: 97,
			Ineligible: 11, IneligibleNotReported: 11,
			SigmaSumGWh: 121125.65,
		}),
		verifyGenerationSurface("2018-19", 418887, "2026-09-10", nil),
		verifyGenerationSurface("2019-20", 421591, "2026-09-10", nil),
		verifyGenerationSurface("2020-21", 455640, "2026-09-10", &verifyInternals{
			InternalsAsOf: "2026-09-08",
			TableRows:     114, RawCells: 4479, Plants: 108, PhysicalWidth: 40, NBSPBytes: 128,
			// 104/105, NOT 105/105. The one failure is real published
			// arithmetic — (NPPCL) - Balloki, months total 5,945.21 against
			// a reported Sum of 5,905.65, delta +39.56 GWh — and it is
			// reported rather than hidden or tolerated away.
			SumPassed: 104, SumEligible: 105,
			Ineligible: 3, IneligibleNotReported: 3,
			SigmaSumGWh: 129580.38,
		}),
		verifyGenerationSurface("2021-22", 516219, "2026-09-10", nil),
		verifyGenerationSurface("2022-23", 490461, "2026-09-10", nil),
		verifyGenerationSurface("2023-24", 493187, "2026-09-10", &verifyInternals{
			InternalsAsOf: "2026-09-08",
			TableRows:     139, RawCells: 4965, Plants: 133, PhysicalWidth: 39, NBSPBytes: 132,
			SumPassed: 118, SumEligible: 118,
			// 15 of 133 rows carry no numeric Sum: 12 DELICENSED, 1
			// DECOMMISSIONED and 2 with an entirely empty <td colspan=26>.
			// The 13 status rows are the 3,880.00 MW non-generating
			// population; adding the 2 blank-block rows (Reshma 97.00,
			// Gulf Powergen 84.00) gives 4,061.00 MW over 15 rows. Both
			// figures are correct for different populations and must not
			// be merged.
			Ineligible: 15, IneligibleDelicensed: 12, IneligibleDecommissioned: 1, IneligibleNotReported: 2,
			SigmaSumGWh: 126765.10,
		}),
	}

	sheets := []struct {
		id, label string
		floor     int
		internals *verifyInternals
	}{
		{"fca", "Fuel Cost Adjustment (2018-2022)", 79843, &verifyInternals{
			InternalsAsOf: "2026-09-08", GridRows: 53, GridRawCells: 572, ExtractedRows: 48,
		}},
		{"quarterly", "Quarterly Data (XWD & KE)", 79672, &verifyInternals{
			InternalsAsOf: "2026-09-08", GridRows: 168, GridRawCells: 582, ExtractedRows: 163,
		}},
		{"sro", "S.R.O. notifications", 40402, &verifyInternals{
			InternalsAsOf: "2026-09-08", GridRows: 23, GridRawCells: 245, ExtractedRows: 19,
		}},
		{"hydel", "Hydel data", 14783, &verifyInternals{
			InternalsAsOf: "2026-09-08", GridRows: 33, GridRawCells: 126, ExtractedRows: 30,
		}},
	}
	for _, s := range sheets {
		path, err := resourceReadPath(s.id)
		if err != nil {
			panic(fmt.Sprintf("verify manifest: no read path for %q: %v", s.id, err))
		}
		out = append(out, verifySurface{
			ID:            s.id,
			Kind:          verifyKindSheet,
			Label:         s.label,
			Path:          path,
			ByteFloor:     s.floor,
			ByteFloorAsOf: "2026-09-08",
			RowFloor:      s.internals.ExtractedRows,
			Asserted:      true,
			Internals:     s.internals,
		})
	}
	return out
}

// verifySheetIDs lists the non-generation selectors --surface accepts.
func verifySheetIDs() []string {
	var ids []string
	for _, s := range verifySurfaces {
		if s.Kind == verifyKindSheet {
			ids = append(ids, s.ID)
		}
	}
	return ids
}

// verifyGenerationYears lists the reachable fiscal-year labels in order.
func verifyGenerationYears() []string {
	var out []string
	for _, s := range verifySurfaces {
		if s.Kind == verifyKindWorkbook {
			out = append(out, s.FY)
		}
	}
	return out
}

func verifySurfaceByID(id string) (verifySurface, bool) {
	for _, s := range verifySurfaces {
		if s.ID == id {
			return s, true
		}
	}
	return verifySurface{}, false
}

func verifySurfaceForFY(label string) (verifySurface, bool) {
	return verifySurfaceByID("gen-" + label)
}
