// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The completeness assertion, and the reason this command's numbers can be
// trusted. Every identity below is checked against nepraparse.Census, which
// is an INDEPENDENT set of counters computed by the parser over the same file
// without any knowledge of this command's grouping. A grouping bug that drops
// or double-counts a plant cannot pass all of them.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// capacityAssertions is the machine-readable assertion block. Each named
// field is true when the identity held. Failed carries one sentence per
// failure, and --strict turns a non-empty Failed into a non-zero exit AFTER
// the payload has been written.
type capacityAssertions struct {
	CensusBalanced                  *bool `json:"census_balanced,omitempty"`
	StatusRowsMatchCensus           *bool `json:"status_rows_match_census,omitempty"`
	DelicensedRowsMatchCensus       *bool `json:"delicensed_rows_match_census,omitempty"`
	DecommissionedRowsMatchCensus   *bool `json:"decommissioned_rows_match_census,omitempty"`
	FullyNotReportedRowsMatchCensus *bool `json:"fully_not_reported_rows_match_census,omitempty"`
	BucketsPartitionPlantRows       *bool `json:"buckets_partition_plant_rows,omitempty"`
	// NoUnmodelledStatusSentinels is false when the source put text in a
	// monthly block that this build cannot classify. Census CANNOT catch
	// this: nepraparse and this command share the blind spot, so both agree
	// on a wrong answer. It is checked separately for that reason.
	NoUnmodelledStatusSentinels *bool `json:"no_unmodelled_status_sentinels,omitempty"`
	CapacityCellsAccounted      *bool `json:"installed_plus_dependable_cells_accounted,omitempty"`
	PlantRowFloor               *int  `json:"plant_row_floor,omitempty"`
	PlantRowFloorMet            *bool `json:"plant_row_floor_met,omitempty"`
	ByteFloor                   *int  `json:"byte_floor,omitempty"`
	ByteFloorMet                *bool `json:"byte_floor_met,omitempty"`
	// Failed is ALWAYS emitted, as an empty array when nothing failed, so a
	// consumer never has to distinguish "no failures" from "the key is
	// missing because this build forgot to check".
	Failed []string `json:"failed"`
}

// capacityAssertLive checks the live report against the parser's own census.
//
// The per-member identities matter as much as the total: asserting only
// "delicensed + decommissioned == Census.StatusRows" passes unchanged if the
// two members are swapped, so each is checked against its own cell count.
// Census.Delicensed counts CELLS (12 rows x 26 = 312) and MonthlyCells is 26,
// which makes the per-member row count recoverable from an independent
// counter.
func capacityAssertLive(w *nepraparse.Workbook, bodyLen int, surface capacityAsOfSurface, rep capacityReport) capacityAssertions {
	a := capacityAssertions{Failed: []string{}}
	if w == nil {
		a.Failed = append(a.Failed, "no workbook was parsed, so no completeness identity could be checked")
		return a
	}
	c := w.Census
	fail := func(format string, args ...any) {
		a.Failed = append(a.Failed, fmt.Sprintf(format, args...))
	}

	balanced := c.Balanced()
	a.CensusBalanced = &balanced
	if !balanced {
		fail("census does not balance: %d classified monthly cells against %d in the block, so a cell escaped classification",
			c.Present+c.NotReported+c.Delicensed+c.Decommissioned+c.ExportToKElectric+c.UnknownText, c.MonthlyCellsTotal)
	}

	// Recount from the plants, independently of whatever --by produced, so
	// the identities hold for every grouping.
	perStatus := map[capacityStatus]int{}
	numericCells, capNotReported, capStatusCells, capUnknown := 0, 0, 0, 0
	for _, p := range w.Plants {
		perStatus[capacityStatusOf(p)]++
		for _, v := range [2]nepraparse.Value{p.InstalledCapacity, p.DependableCapacity} {
			switch st := v.State(); {
			case st == nepraparse.StateNumeric:
				numericCells++
			case st == nepraparse.StateNotReported:
				capNotReported++
			case st.Status():
				capStatusCells++
			default:
				capUnknown++
			}
		}
	}

	nonOperating := perStatus[capStatusDelicensed] + perStatus[capStatusDecommissioned]
	ok := nonOperating == c.StatusRows
	a.StatusRowsMatchCensus = &ok
	if !ok {
		fail("delicensed+decommissioned rows = %d but the parser's independent Census.StatusRows = %d", nonOperating, c.StatusRows)
	}

	wantDelicensed := c.Delicensed / nepraparse.MonthlyCells
	okD := perStatus[capStatusDelicensed] == wantDelicensed && c.Delicensed%nepraparse.MonthlyCells == 0
	a.DelicensedRowsMatchCensus = &okD
	if !okD {
		fail("delicensed rows = %d but Census.Delicensed = %d cells, which is %d rows of %d",
			perStatus[capStatusDelicensed], c.Delicensed, wantDelicensed, nepraparse.MonthlyCells)
	}

	wantDecommissioned := c.Decommissioned / nepraparse.MonthlyCells
	okX := perStatus[capStatusDecommissioned] == wantDecommissioned && c.Decommissioned%nepraparse.MonthlyCells == 0
	a.DecommissionedRowsMatchCensus = &okX
	if !okX {
		fail("decommissioned rows = %d but Census.Decommissioned = %d cells, which is %d rows of %d",
			perStatus[capStatusDecommissioned], c.Decommissioned, wantDecommissioned, nepraparse.MonthlyCells)
	}

	// BOTH members are compared against Census.FullyNotReportedRows, and the
	// reason is worth keeping: nepraparse's Census counts a row as
	// fully-not-reported on the MONTHLY BLOCK alone, so it cannot separate a
	// plant that published a capacity number from one whose capacity cell
	// holds the "Export to K.Electric" sentinel. This command CAN separate
	// them and does, so the identity is against the sum.
	//
	// Before the export member existed both sides were wrong together —
	// capacity booked those three FY2020-21 plants as listed_no_data and
	// Census counted them the same way — so this assertion PASSED while the
	// bucket's own basis text asserted the opposite of what was true.
	okF := perStatus[capStatusListedNoData]+perStatus[capStatusExportToKElectric] == c.FullyNotReportedRows
	a.FullyNotReportedRowsMatchCensus = &okF
	if !okF {
		fail("listed_no_data rows = %d plus export_to_k_electric rows = %d do not sum to the parser's "+
			"independent Census.FullyNotReportedRows = %d; these are the rows whose monthly block reports "+
			"nothing, split by whether a capacity number was published",
			perStatus[capStatusListedNoData], perStatus[capStatusExportToKElectric], c.FullyNotReportedRows)
	}

	summed := 0
	for _, n := range perStatus {
		summed += n
	}
	okP := summed == c.Rows && c.Rows == rep.Totals.PlantRows
	a.BucketsPartitionPlantRows = &okP
	if !okP {
		fail("the status members sum to %d plants against Census.Rows = %d and %d reported plant rows: "+
			"a plant was dropped or double-counted", summed, c.Rows, rep.Totals.PlantRows)
	}

	// The one identity the Census cannot supply. Census.StatusRows counts
	// only the sentinels nepraparse RECOGNISES, so in FY2022-23 both it and
	// this command say 11 delicensed rows and agree — while two further rows
	// spell the sentinel "DELICENSE" and are classified by neither. Checking
	// it here is what stops that agreement reading as confirmation.
	undetermined := perStatus[capStatusUndetermined]
	okU := undetermined == 0
	a.NoUnmodelledStatusSentinels = &okU
	if !okU {
		// The megawatts are recounted FROM THE PLANTS, not looked up in the
		// report's buckets, for the same reason perStatus above is: an
		// assertion is a statement about the WORKBOOK and must read the same
		// under every --by. Reading rep.bucket(capStatusUndetermined) made
		// this sentence grouping-dependent and it reported "no capacity
		// number" over 181.00 MW of published capacity — under --by
		// technology because the member is nested inside a parent's ByStatus,
		// and under --by system because that grouping carries no status split
		// at all, so the member does not exist there to find.
		mw := "no capacity number"
		var undeterminedMW float64
		var anyMeasured bool
		for _, p := range w.Plants {
			if capacityStatusOf(p) != capStatusUndetermined {
				continue
			}
			if n, measured := p.InstalledCapacity.Float64(); measured {
				undeterminedMW += n
				anyMeasured = true
			}
		}
		if anyMeasured {
			mw = fmt.Sprintf("%.2f MW installed", undeterminedMW)
		}
		fail("%d plant row(s) publish a monthly block this build cannot classify (%s): %s. The verbatim text is %q. "+
			"They are reported as status_undetermined and are in NO operating total; nepraparse's Census agrees "+
			"with this command's other counts because both share the blind spot, so this is the only check that "+
			"catches it",
			undetermined, mw, strings.Join(rep.UndeterminedPlants, "; "), strings.Join(rep.UnmodelledText, "\", \""))
	}

	okC := numericCells == c.CapacityPresent && capNotReported == c.CapacityNotReported &&
		capStatusCells == c.CapacityStatus && numericCells+capNotReported+capStatusCells+capUnknown == c.CapacityCells
	a.CapacityCellsAccounted = &okC
	if !okC {
		fail("capacity cells do not reconcile: %d numeric / %d not-reported / %d status / %d unknown against "+
			"Census %d present / %d not-reported / %d status over %d cells",
			numericCells, capNotReported, capStatusCells, capUnknown,
			c.CapacityPresent, c.CapacityNotReported, c.CapacityStatus, c.CapacityCells)
	}

	// The measured floors. A year with no fixture has NO floor, and a floor
	// of 0 would assert something, so nothing is emitted for those years.
	if surface.FloorsMeasured {
		if surface.PlantRowFloor > 0 {
			floor := surface.PlantRowFloor
			a.PlantRowFloor = &floor
			met := len(w.Plants) >= floor
			a.PlantRowFloorMet = &met
			if !met {
				fail("the sheet returned %d plant rows, below the %d measured for FY%s. NEPRA silently truncates "+
					"under some conditions, so do not treat this result as complete", len(w.Plants), floor, surface.FiscalYear)
			}
		}
		if surface.DecodedByteFloor > 0 && bodyLen > 0 {
			floor := surface.DecodedByteFloor
			a.ByteFloor = &floor
			met := bodyLen >= floor
			a.ByteFloorMet = &met
			if !met {
				fail("the response decoded to %d bytes, below the %d measured for FY%s", bodyLen, floor, surface.FiscalYear)
			}
		}
	}
	return a
}

// capacityAssertLocal is the offline path's much smaller assertion set. There
// is no Census offline, so census_balanced, the per-member census identities
// and byte_floor are ABSENT rather than asserted as true.
func capacityAssertLocal(surface capacityAsOfSurface, rep capacityReport) capacityAssertions {
	a := capacityAssertions{Failed: []string{}}
	summed := 0
	for _, b := range rep.Buckets {
		if b.Plants != nil {
			summed += *b.Plants
		}
	}
	// Only the status grouping partitions the rows; system deliberately does
	// not (export_to_k_electric is a subset of cppag) and technology sums to
	// the same total by a different route.
	if rep.By == "status" {
		ok := summed == rep.Totals.PlantRows
		a.BucketsPartitionPlantRows = &ok
		if !ok {
			a.Failed = append(a.Failed, fmt.Sprintf(
				"the status members sum to %d plants against %d reported plant rows", summed, rep.Totals.PlantRows))
		}
	}
	if surface.FloorsMeasured && surface.PlantRowFloor > 0 {
		floor := surface.PlantRowFloor
		a.PlantRowFloor = &floor
		met := rep.Totals.PlantRows >= floor
		a.PlantRowFloorMet = &met
		if !met {
			a.Failed = append(a.Failed, fmt.Sprintf(
				"the crosswalk carries %d observations for FY%s, below the %d plant rows measured in the published sheet",
				rep.Totals.PlantRows, surface.FiscalYear, floor))
		}
	}
	return a
}
