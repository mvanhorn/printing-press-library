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

// verifyCrosscheckSources are the values --crosscheck accepts.
var verifyCrosscheckSources = []string{"iea"}

// verifyIEAURL is the only external series anyone in this project has ever
// observed answering: https://api.iea.org/stats/indicator/ElecGenByFuel with
// countries=PAKISTAN returned 200 / 44,893 B / application/json / 204 rows
// covering 1990-2023 in GWh. IEA's HTML country pages return 403.
const verifyIEAURL = "https://api.iea.org/stats/indicator/ElecGenByFuel?countries=PAKISTAN"

// verifySigmaResult is the NEPRA side of any external comparison.
type verifySigmaResult struct {
	// TotalGWh sums the published "Sum GWh" column over rows whose value is
	// a real measurement.
	TotalGWh float64 `json:"sigma_sum_gwh"`
	// RowsSummed and RowsExcluded partition the plant rows.
	RowsSummed   int `json:"rows_summed"`
	RowsExcluded int `json:"rows_excluded"`
	// ExcludedByState counts the excluded rows by the cell state that
	// excluded them, so an exclusion can never be mistaken for a zero.
	ExcludedByState map[string]int `json:"excluded_by_state"`
}

// verifySumGWh totals the annual Sum-GWh column.
//
// It sums ONLY cells whose CellState is StateNumeric. A blank month or a
// DELICENSED row is an absence of measurement, not a zero: conflating the two
// would invent 11 plants x 12 months of false zero generation in FY2017-18
// alone. The excluded rows are counted and broken down by state alongside the
// total, so the total always travels with what it left out.
func verifySumGWh(w *nepraparse.Workbook) verifySigmaResult {
	out := verifySigmaResult{ExcludedByState: map[string]int{}}
	for _, p := range w.Plants {
		v := p.Total.Generation
		if n, ok := v.Float64(); ok && v.State() == nepraparse.StateNumeric {
			out.TotalGWh += n
			out.RowsSummed++
			continue
		}
		out.RowsExcluded++
		out.ExcludedByState[v.State().String()]++
	}
	// Two decimals is the published precision; carrying float noise past it
	// would imply a precision the source does not have.
	out.TotalGWh = float64(int64(out.TotalGWh*100+0.5)) / 100
	return out
}

// verifyDelta is deliberately incapable of carrying a reconciled number.
type verifyDelta struct {
	// BasisAligned is false whenever the two series do not share a period,
	// a population and a unit. It is false for every NEPRA/IEA pair that
	// exists.
	BasisAligned bool `json:"basis_aligned"`
	// ValueGWh and ValuePct are pointers so they can be, and are, null. A
	// number here would be a fabrication: subtracting a Jul-Jun fiscal total
	// from a calendar-year national total is not a reconciliation.
	ValueGWh *float64 `json:"value_gwh"`
	ValuePct *float64 `json:"value_pct"`
	Reason   string   `json:"reason"`
}

// verifyIEASide is what was observed of the external series. It is null when
// the series was not read, and this build never reads it — see DeclaredGap.
type verifyIEASide struct {
	Rows      int    `json:"rows"`
	Years     int    `json:"years"`
	FirstYear int    `json:"first_year"`
	LastYear  int    `json:"last_year"`
	Units     string `json:"units"`
}

// verifyCrosscheck reports both sides of an external comparison and the
// reasons they cannot be subtracted.
type verifyCrosscheck struct {
	Source string `json:"source"`
	URL    string `json:"url"`
	// Fetched says whether this run read the external series.
	Fetched bool `json:"fetched"`
	// DeclaredGap names what this command cannot answer today and why. Null
	// when the external series was read.
	DeclaredGap *verifyGap             `json:"declared_gap"`
	IEA         *verifyIEASide         `json:"iea"`
	NEPRA       *verifyCrosscheckNEPRA `json:"nepra"`
	Delta       verifyDelta            `json:"delta"`
	GapReasons  []string               `json:"gap_reasons"`
	// Verdict is always "reported". A documented, structural gap between two
	// honest publications is not a gate failure, so this leg never changes
	// the exit code.
	Verdict string `json:"verdict"`
}

type verifyCrosscheckNEPRA struct {
	FY string `json:"fy"`
	verifySigmaResult
}

// verifyGap is something the command declines to answer, with the reason.
type verifyGap struct {
	Gap    string `json:"gap"`
	Reason string `json:"reason"`
}

// verifyCrosscheckGapReasons are the four measured, structural reasons the two
// series cannot be reconciled. They are the answer to "why is there no
// number here", and they are the deliverable — surfacing the gap means
// printing both sides and these reasons, not subtracting them.
func verifyCrosscheckGapReasons(fy string) []string {
	return []string{
		fmt.Sprintf("period basis: NEPRA FY%s is Jul-Jun; IEA's series is calendar-year and stops at 2023. "+
			"No NEPRA year shares a full period with any IEA year, in either direction.", fy),
		"coverage: K-Electric's own fleet is ABSENT from the NEPRA generation dataset entirely — no BQPS, " +
			"no Korangi, no SITE — while IEA's series is national. The NEPRA total is therefore " +
			"structurally below any national figure by an unknown amount.",
		"excluded rows: rows with no numeric Sum are excluded from the NEPRA total and counted by state, " +
			"never read as zero. In FY2023-24 that is 15 of 133 rows (12 DELICENSED, 1 DECOMMISSIONED, " +
			"2 with an empty <td colspan=26>).",
		"ceiling: IEA publishes annual national generation by fuel only — no plant, no DISCO, no tariff, " +
			"no losses — so even a shared period would compare one column against a panel.",
	}
}

// verifyBuildCrosscheck assembles the crosscheck block.
//
// THE EXTERNAL SERIES IS NOT READ BY THIS BUILD, AND NEVER WILL BE. What began
// as a declared gap pending a governance review is now a settled refusal:
// IEA's own published policy forbids it, measured 11 Sep 2026.
//
//	https://api.iea.org/robots.txt  ->  "User-agent: *" / "Disallow: /"
//
// That is a blanket disallow for EVERY agent across the ENTIRE API host — not a
// named-bot block of the kind nepra.org.pk carries and which this CLI was
// cleared against by using an honest product UA. There is no compliant way to
// read it. Independently, IEA's Terms of Use exclude "Standalone datasets, data
// explorers and databases" from their CC BY 4.0 Open Use Terms, so the series
// is Non-CC Material even where a transport were permitted. Transport and
// licence each forbid it on their own.
//
// AND IT WOULD STILL BE REFUSED WITH THE DATA IN HAND, which is the more
// interesting reason: the comparison is not analytically valid as the feature
// was specified. NEPRA publishes Jul-Jun FISCAL years; IEA's series is
// CALENDAR-year and stops at 2023, so no NEPRA year shares a full period with
// any IEA year in either direction. NEPRA's generation dataset also excludes
// K-Electric's own fleet (no BQPS, no Korangi, no SITE) while IEA's series is
// national. Subtracting the two and calling the difference a "gap" would
// manufacture a finding out of a basis mismatch.
//
// What IS computable ships: the NEPRA side is real, measured from the file
// under inspection, with its exclusions counted by state — and the four
// reasons the two series cannot be subtracted. delta.value_gwh is null in
// every case, including the case where the IEA series IS read, because the
// basis mismatch is structural and not a data-availability problem.
func verifyBuildCrosscheck(fy string, sigma verifySigmaResult) *verifyCrosscheck {
	return &verifyCrosscheck{
		Source:  "iea",
		URL:     verifyIEAURL,
		Fetched: false,
		DeclaredGap: &verifyGap{
			Gap: "the IEA side of --crosscheck iea is not read by this build",
			Reason: "IEA's own policy forbids it, measured 11 Sep 2026: api.iea.org/robots.txt " +
				"is \"User-agent: *\" / \"Disallow: /\", a blanket disallow for every agent across " +
				"the whole API host, and IEA's Terms of Use exclude standalone datasets and " +
				"databases from their CC BY 4.0 Open Use Terms, so the series is Non-CC " +
				"Material. Transport and licence each forbid it alone. The comparison would " +
				"also be refused with the data in hand: NEPRA is Jul-Jun fiscal, IEA is " +
				"calendar-year ending 2023, and NEPRA's dataset excludes K-Electric's own " +
				"fleet while IEA's series is national. The NEPRA side below is real and " +
				"measured. Reporting one side with the reasons is honest; fetching a series " +
				"whose publisher disallows it, or printing a remembered figure as if it had " +
				"been fetched, would not be.",
		},
		IEA:   nil,
		NEPRA: &verifyCrosscheckNEPRA{FY: fy, verifySigmaResult: sigma},
		Delta: verifyDelta{
			BasisAligned: false,
			ValueGWh:     nil,
			ValuePct:     nil,
			Reason: "Refused, and it would still be refused with the IEA series in hand. NEPRA " +
				"publishes Jul-Jun fiscal years and IEA publishes calendar years ending 2023, " +
				"so no pair shares a period; K-Electric's fleet is absent from the NEPRA " +
				"dataset, so no pair shares a population. A number here would be a fabrication.",
		},
		GapReasons: verifyCrosscheckGapReasons(fy),
		Verdict:    verifyReported,
	}
}
