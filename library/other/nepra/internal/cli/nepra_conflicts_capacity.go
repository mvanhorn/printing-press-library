// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The capacity derivation leg: the installed-capacity DEFINITIONS conflict.
//
// The disagreement on this surface is not a typo but a definition: several
// incompatible megawatt figures for 30 June 2024 are each correct for a
// different population, and a consumer that takes one without its definition
// has a wrong number. So the entry's sides are the DEFINITIONS, never a single
// figure.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// capacityMWSide projects an MWSum into a side.
//
// Every megawatt is read through MWSum.Float64()'s SECOND return, and an
// unmeasured sum contributes NO `value` key and renders as "<not measured>".
// That is the whole point of the type: a fiscal year the crosswalk never
// observed cannot answer 0 MW, and a 0 there would read as a measurement of
// nothing rather than nothing measured.
func capacityMWSide(s nepraxwalk.MWSum, variant, basis, url, reportFY string) conflictSide {
	side := conflictSide{
		Raw: s.String(), ValueKind: "not_reported", Unit: "MW", ReportFY: reportFY,
		Variant: variant, URL: url, Basis: basis, Plants: iptr(s.Plants()),
	}
	if mw, ok := s.Float64(); ok {
		side.Value = fptr(mw)
		side.ValueKind = "numeric"
		side.Raw = fmt.Sprintf("%.2f", mw)
		return side
	}
	side.Basis = basis + " [NOT MEASURED: " + s.String() + "]"
	return side
}

// capacityEntry re-derives the definitions conflict from one workbook.
//
// It reports what it MEASURED and nothing else. The absorb manifest's own
// triple — 40,614 MW active, 4,061 MW status, 44,675 MW total — is refuted on
// measurement and is not emitted as a side: an entry citing this project's own
// superseded brief would be the one place the command manufactured a conflict.
func capacityEntry(w *nepraparse.Workbook, fy string) conflictEntry {
	norm := "FY" + conflictBareFY(fy)
	url := conflictGenURL(fy)
	names := make([]string, 0, len(w.Plants))
	for _, p := range w.Plants {
		names = append(names, p.Name)
	}
	rep := nepraxwalk.Coverage(conflictBareFY(fy), names)

	a := capacityMWSide(rep.CapacityReported, "capacity_reported",
		"every numeric installed capacity published for this date, operating or not; the figure that ties "+
			"to the workbook total", url, norm)
	b := capacityMWSide(rep.CapacityOperating, "capacity_operating",
		"the part of the reported total whose monthly generation block is ordinary data. THIS is current "+
			"generating capacity; the reported total is not", url, norm)

	also := []conflictSide{
		capacityMWSide(rep.CapacityNonOperating, "capacity_non_operating_by_sentinel",
			"the part whose monthly block reads DELICENSED or DECOMMISSIONED. A non-operating plant KEEPS a "+
				"valid installed capacity, so this gates how capacity is LABELLED, never whether it is read",
			url, norm),
		capacityMWSide(rep.CapacityStatusUnknown, "capacity_status_unknown",
			"the part whose operating status was not recorded. It belongs to neither of the two above and "+
				"must not be folded into either", url, norm),
		capacityMWSide(rep.CapacityTickered, "capacity_tickered",
			"the part carrying a PSX symbol", url, norm),
		capacityMWSide(rep.CapacityTickeredOperating, "capacity_tickered_operating",
			"the part carrying a PSX symbol that is actually generating; the gap against the line above is "+
				"what an equity analyst would otherwise book as live capacity", url, norm),
	}

	// The third computable definition: non-generating capacity INCLUDING rows
	// whose monthly block is simply blank. A blank block is NOT REPORTED,
	// which is a different fact from a declared DELICENSED, so the two
	// populations are reported separately and never merged.
	blankMW, blankRows, blankNames := capacityBlankBlocks(w)
	nonOpMW, nonOpMeasured := rep.CapacityNonOperating.Float64()
	notGenerating := conflictSide{
		Raw: "<not measured>", ValueKind: "not_reported", Unit: "MW", ReportFY: norm,
		Variant: "capacity_not_generating_including_unreported", URL: url,
		Plants: iptr(rep.CapacityNonOperating.Plants() + blankRows),
	}
	notGenerating.Basis = fmt.Sprintf(
		"the status-sentinel rows plus the %d row(s) whose entire monthly block is blank while a capacity "+
			"is published (%s). Both populations are correct and must NOT be merged: a blank block is NOT "+
			"REPORTED, which is a different fact from a declared DELICENSED", blankRows, blankNames)
	if nonOpMeasured {
		notGenerating.Value = fptr(nonOpMW + blankMW)
		notGenerating.ValueKind = "derived"
		notGenerating.Raw = fmt.Sprintf("%.2f", nonOpMW+blankMW)
	} else {
		notGenerating.Basis += " [NOT MEASURED: the non-operating half has no population behind it]"
	}
	also = append(also, notGenerating)

	// The figures that exist and cannot be recomputed by anything that ships.
	also = append(also, capacityUnavailableSides()...)

	return conflictEntry{
		ID: conflictEntryID(conflictSurfaceCapacity, conflictKindConflict,
			conflictKey{PeriodFY: norm, Metric: "installed_capacity_mw"}, "definitions"),
		Surface:               conflictSurfaceCapacity,
		Kind:                  conflictKindConflict,
		Class:                 "declared_variant",
		SameKey:               true,
		Key:                   conflictKey{PeriodFY: norm, Metric: "installed_capacity_mw"},
		A:                     a,
		B:                     b,
		Also:                  also,
		Ratio:                 conflictRatio(a.Value, b.Value),
		AbsDiff:               conflictAbsDiff(a.Value, b.Value),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            "recomputed from the published workbook by nepraxwalk.Coverage",
		ReproducedByThisBuild: true,
		RecomputeNeeds:        []string{norm},
		Note: "several incompatible megawatt figures for one date, every one of them correct for a " +
			"different population. " + rep.CapacityBasis,
	}
}

// capacityBlankBlocks sums the published capacity of plants whose ENTIRE
// monthly block is blank.
//
// These rows are the reason two different non-generating totals are both
// correct. In FY2023-24 there are two of them and they publish 181.00 MW
// between them; in FY2017-18 there are eleven and NONE publishes a capacity at
// all, so the honest answer there is 0.00 MW over 11 rows — a measured zero
// contribution, not a claim that eleven plants have no capacity.
func capacityBlankBlocks(w *nepraparse.Workbook) (mw float64, rows int, names string) {
	var listed []string
	for _, p := range w.Plants {
		allBlank := true
		for _, o := range p.Months {
			if o.Utilisation.State() != nepraparse.StateNotReported ||
				o.Generation.State() != nepraparse.StateNotReported {
				allBlank = false
				break
			}
		}
		if !allBlank {
			continue
		}
		rows++
		if v, ok := p.InstalledCapacity.Float64(); ok {
			mw += v
			listed = append(listed, fmt.Sprintf("%s %.2f MW", p.Name, v))
			continue
		}
		listed = append(listed, p.Name+" capacity not reported")
	}
	if len(listed) == 0 {
		return 0, 0, "no fully-blank monthly blocks in this year"
	}
	if len(listed) > 4 {
		return mw, rows, fmt.Sprintf("%s and %d more", listed[0], len(listed)-1)
	}
	names = listed[0]
	for _, l := range listed[1:] {
		names += "; " + l
	}
	return mw, rows, names
}

// capacityUnavailableSides are the published capacity figures this build
// cannot recompute. They carry value_kind `unavailable`, NO number, and the
// measured reason — never an approximation.
func capacityUnavailableSides() []conflictSide {
	return []conflictSide{
		{
			Raw: "42,512", ValueKind: "unavailable", Unit: "MW",
			Variant: "sir_2024_cppag_system",
			Basis: "stated by NEPRA's own State of Industry Report 2024 for the CPPA-G system. It lives " +
				"only inside the SIR PDF corpus — 610 MB across ten reports at 68-164 KB/s, and " +
				"sir2023.pdf extracts 0 bytes of text — which this build declares unverifiable. Recorded " +
				"as unavailable and never approximated",
		},
		{
			Raw: "45,888", ValueKind: "unavailable", Unit: "MW",
			Variant: "sir_2024_including_k_electric",
			Basis: "the same SIR 2024 figure including K-Electric, on the same unverifiable surface. Note " +
				"K-Electric's own fleet is absent from the generation workbook entirely, so no computable " +
				"side here includes it",
		},
		{
			Raw: "47,559.97", ValueKind: "unavailable", Unit: "MW",
			Variant: "licence_register_gross_capacity",
			Basis: "Gross Capacity summed over 331 of 335 licence entities, as the research probe published " +
				"it. `licence --all` now reads the register (21 pages, 379 entities) and CANNOT reproduce this " +
				"figure honestly: it is reachable only by reading \"12:00 MW\" as 12 and \"3s 6 MW\" as 3, and " +
				"the latter entity's own plant-detail row reads 1x16MW + 1x20MW, i.e. 36 MW. The shipped command " +
				"reports 361 entities with an MW-convertible capacity and names the other 18 by reason, which is " +
				"why no single total is asserted on either side",
		},
	}
}
