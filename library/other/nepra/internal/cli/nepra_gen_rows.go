// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-gen-plant-month-extract.json.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// The five row classes. Four are NEPRA's own vocabulary, read out of the
// cells; genRowClassListedNoData is THIS TOOL'S NAME for a shape NEPRA
// publishes without explaining.
const (
	// genRowClassActive means the monthly block carries measurements.
	genRowClassActive = "active"
	// genRowClassListedNoData means every one of the plant's 26 monthly cells
	// is blank. Its capacity may or may not be published: FY2023-24's two
	// such rows carry 97 and 84 MW, FY2017-18's eleven carry nothing.
	//
	// WHAT THIS MEANS IS NOT STATED ANYWHERE IN THE SOURCE. There is no
	// legend, glossary or footnote in any of the seven files. It could be
	// idle, newly licensed and unbuilt, delicensed before the status
	// vocabulary existed, or an omission. The name says only what was
	// observed — listed, no data — and must not be glossed as "idle".
	genRowClassListedNoData = "listed_no_data"
	// genRowClassUnmodelledBlock means all 26 monthly cells carry the SAME
	// text that this build cannot model.
	//
	// IT EXISTS BECAUSE OF A MEASURED TYPO, and it is the reason this rule
	// has five steps rather than the four originally specified. FY2022-23
	// publishes the bare word "DELICENSE" — no trailing D — in a
	// <td colspan=26> for two plants, 52 cells in all, against 11 correctly
	// spelled DELICENSED rows and 1 DECOMMISSIONED in the same file.
	// nepraparse recognises DELICENSED only, so those 52 cells are
	// StateUnknownText, and WITHOUT THIS CLASS the two rows come out
	// "active" — reading as two plants that reported, with a null everywhere
	// a number should be.
	//
	// The text is NOT normalised to DELICENSED here. That would be this
	// command guessing at NEPRA's intent on the strength of a missing
	// letter, and the vocabulary belongs to nepraparse, which is not this
	// command's to change. The verbatim text travels in every affected
	// row's `note`, and meta.warnings names it.
	genRowClassUnmodelledBlock = "unmodelled_block"
)

// genRow is one plant-month at the default grain.
//
// EVERY ONE OF THE 25 FIELDS IS ALWAYS PRESENT, and no numeric field carries
// `omitempty`. That is deliberate and load-bearing twice over:
//
//  1. A *float64 that is nil marshals as `null`, which a consumer cannot
//     mistake for 0. Omitting the key instead would make an absent month
//     indistinguishable from a schema change.
//  2. The generated compactListFields keeps any key present in >= 80% of
//     rows and drops the rest. A row model that omitted `gwh` when it was
//     unreported would have the entire generation column silently stripped
//     in a year where more than a fifth of plant-months are unreported —
//     FY2017-18 is 11 of 108 rows, which is close enough to the line to be
//     a real hazard. A fixed key set makes that unreachable.
type genRow struct {
	FY              string `json:"fy"`
	SNo             int    `json:"sno"`
	Plant           string `json:"plant"`
	Technology      string `json:"technology"`
	TechnologyKnown bool   `json:"technology_known"`
	Fuel            string `json:"fuel"`
	FuelKnown       bool   `json:"fuel_known"`
	// InstalledMW and DependableMW are nil unless the capacity cell is a
	// number. A status sentinel in the MONTHLY block does NOT null them:
	// Kotri keeps 174/120 MW while all 26 of its monthly cells read
	// DELICENSED. Capacity and generation nullability are independent.
	InstalledMW       *float64 `json:"installed_mw"`
	InstalledMWState  string   `json:"installed_mw_state"`
	DependableMW      *float64 `json:"dependable_mw"`
	DependableMWState string   `json:"dependable_mw_state"`
	// RowClass is this tool's four-step derivation over the whole row.
	RowClass string `json:"row_class"`
	// BlockStatus is nepraparse's own verdict on the monthly block, kept
	// separately because "numeric" here does NOT mean the plant reported:
	// Reshma and Gulf Powergen both have block_status numeric with all 26
	// monthly cells blank.
	BlockStatus string `json:"block_status"`
	Month       string `json:"month"`
	MonthIndex  int    `json:"month_index"`
	Period      string `json:"period"`
	// UtilisationPct is the published "% age" cell for this month. It is
	// carried per row and NEVER aggregated: see genRollup.
	UtilisationPct   *float64 `json:"utilisation_pct"`
	UtilisationState string   `json:"utilisation_state"`
	GWh              *float64 `json:"gwh"`
	GWhState         string   `json:"gwh_state"`
	// SumGWhReported is the plant's published annual Sum, repeated on each
	// of its twelve rows. It is the PUBLISHED value, never recomputed.
	SumGWhReported      *float64 `json:"sum_gwh_reported"`
	SumGWhReportedState string   `json:"sum_gwh_reported_state"`
	// SumReconciles is whether sum(12 months) matches the published Sum
	// within nepraparse.DefaultSumTolerance. It is nil when the row was not
	// eligible for the check — a blank, a status sentinel or unmodelled text
	// anywhere in the thirteen cells — because an ineligible row is not a
	// failing row.
	SumReconciles *bool `json:"sum_reconciles"`
	// Note carries the verbatim text of any cell in this row that nepraparse
	// could not model. It is "" when there is none, never a placeholder.
	Note      string `json:"note"`
	SourceURL string `json:"source_url"`
}

// genRowHeader is the csv/tsv column order, and it is the SAME order as the
// struct fields above.
//
// It is written out literally because the shipped printCSV sorts keys with
// sort.Strings, which alphabetises these 25 columns into an order no consumer
// asked for, and printAutoTable cuts a >6-column table to its first six
// headers without saying so.
var genRowHeader = []string{
	"fy", "sno", "plant", "technology", "technology_known", "fuel", "fuel_known",
	"installed_mw", "installed_mw_state", "dependable_mw", "dependable_mw_state",
	"row_class", "block_status", "month", "month_index", "period",
	"utilisation_pct", "utilisation_state", "gwh", "gwh_state",
	"sum_gwh_reported", "sum_gwh_reported_state", "sum_reconciles", "note", "source_url",
}

// Cells renders the row in genRowHeader order.
//
// An absent number is the EMPTY STRING, never "0" and never "null": the
// adjacent *_state column carries the reason. A csv consumer that reads the
// numeric column alone gets a blank it has to handle, which is the intended
// friction.
func (r genRow) Cells() []string {
	return []string{
		r.FY,
		genFormatInt(r.SNo),
		r.Plant,
		r.Technology,
		genFormatBool(r.TechnologyKnown),
		r.Fuel,
		genFormatBool(r.FuelKnown),
		genFormatFloatPtr(r.InstalledMW),
		r.InstalledMWState,
		genFormatFloatPtr(r.DependableMW),
		r.DependableMWState,
		r.RowClass,
		r.BlockStatus,
		r.Month,
		genFormatInt(r.MonthIndex),
		r.Period,
		genFormatFloatPtr(r.UtilisationPct),
		r.UtilisationState,
		genFormatFloatPtr(r.GWh),
		r.GWhState,
		genFormatFloatPtr(r.SumGWhReported),
		r.SumGWhReportedState,
		genFormatBoolPtr(r.SumReconciles),
		r.Note,
		r.SourceURL,
	}
}

// genRowClass derives the row's class. THE ORDER OF THE FOUR STEPS IS THE
// CORRECTNESS REQUIREMENT.
//
//  1. a status sentinel covering the monthly block wins, and keeps NEPRA's
//     own word for it;
//  2. then "Export to K.Electric" in the INSTALLED CAPACITY column, because
//     FY2020-21's three such rows also have an all-blank monthly block and
//     would otherwise be misreported as listed_no_data — losing the one
//     thing NEPRA did say about them;
//  3. then an all-blank monthly block;
//  4. then a monthly block that is uniformly one unmodelled text — see
//     genRowClassUnmodelledBlock for the FY2022-23 "DELICENSE" typo that put
//     this step here;
//  5. otherwise active.
//
// Step 3 cannot be replaced by Plant.Status: Reshma (97 MW) and Gulf Powergen
// (84 MW) both have Status == StateNumeric and no monthly data at all in
// FY2023-24. Reading
// a numeric block status as "operating" is the exact defect nepraxwalk's
// Operating() (operating, known) pair was introduced to fix.
func genRowClass(p nepraparse.Plant) string {
	if p.Status.Status() {
		return p.Status.String()
	}
	if p.InstalledCapacity.State() == nepraparse.StateExportToKElectric {
		return nepraparse.StateExportToKElectric.String()
	}
	if genUniformBlock(p, nepraparse.StateNotReported) != nil {
		return genRowClassListedNoData
	}
	if genUniformBlock(p, nepraparse.StateUnknownText) != nil {
		return genRowClassUnmodelledBlock
	}
	return genRowClassActive
}

// genUniformBlock reports the single raw text every one of the plant's 26
// monthly cells carries in the given state, or nil when the block is not
// uniformly that state.
//
// Uniformity is required rather than assumed: the source writes a status row
// as ONE <td colspan=26>, so a genuine sentinel row IS uniform, and demanding
// it means a partial or shifted read cannot masquerade as a clean status row.
func genUniformBlock(p nepraparse.Plant, want nepraparse.CellState) *string {
	raw := p.Months[0].Utilisation.Raw()
	for _, obs := range genObservations(p) {
		for _, v := range [2]nepraparse.Value{obs.Utilisation, obs.Generation} {
			if v.State() != want || v.Raw() != raw {
				return nil
			}
		}
	}
	return &raw
}

// genObservations returns the thirteen pairs — twelve months then Sum.
func genObservations(p nepraparse.Plant) []nepraparse.MonthlyObservation {
	out := make([]nepraparse.MonthlyObservation, 0, nepraparse.PairCount)
	out = append(out, p.Months[:]...)
	return append(out, p.Total)
}

// genRows shapes one parsed workbook into plant-month rows.
//
// It emits TWELVE ROWS FOR EVERY PLANT, including delicensed plants and plants
// with no monthly data, because a plant that did not report is a finding and
// dropping it would turn 133 plants into 118 without saying so. The row count
// is therefore plants x 12 — 1,596 for FY2023-24 — and NOT the "reporting rows
// x 12" figure that a reporting-rows-only reading produces.
func genRows(w *nepraparse.Workbook, sourceURL string) []genRow {
	reconciles := genReconcileByRow(w)
	fy := w.FiscalYear.Label()
	out := make([]genRow, 0, len(w.Plants)*12)
	for _, p := range w.Plants {
		class := genRowClass(p)
		base := genRow{
			FY:                  fy,
			SNo:                 p.SNo,
			Plant:               p.Name,
			Technology:          p.Technology,
			TechnologyKnown:     p.TechnologyKnown,
			Fuel:                p.Fuel,
			FuelKnown:           p.FuelKnown,
			InstalledMW:         genFloatPtr(p.InstalledCapacity),
			InstalledMWState:    p.InstalledCapacity.State().String(),
			DependableMW:        genFloatPtr(p.DependableCapacity),
			DependableMWState:   p.DependableCapacity.State().String(),
			RowClass:            class,
			BlockStatus:         p.Status.String(),
			SumGWhReported:      genFloatPtr(p.Total.Generation),
			SumGWhReportedState: p.Total.Generation.State().String(),
			SumReconciles:       reconciles[p.RowIndex],
			SourceURL:           sourceURL,
		}
		for i, m := range nepraparse.MonthsInFiscalOrder {
			obs := p.Months[i]
			row := base
			row.Month = m.String()
			row.MonthIndex = m.FiscalIndex()
			// PeriodKey is the only correct way to date these columns: the
			// months run in FISCAL order, so column 7 is January of the
			// SECOND calendar year, not July.
			if pk, ok := w.FiscalYear.PeriodKey(m); ok {
				row.Period = pk
			}
			row.UtilisationPct = genFloatPtr(obs.Utilisation)
			row.UtilisationState = obs.Utilisation.State().String()
			row.GWh = genFloatPtr(obs.Generation)
			row.GWhState = obs.Generation.State().String()
			row.Note = genRowNote(p, obs)
			out = append(out, row)
		}
	}
	return out
}

// genRowNote reports any unmodelled cell text touching this row, verbatim.
// Unmodelled text is neither dropped nor coerced to a number; it travels.
func genRowNote(p nepraparse.Plant, obs nepraparse.MonthlyObservation) string {
	var parts []string
	for _, c := range []struct {
		label string
		v     nepraparse.Value
	}{
		{"installed_mw", p.InstalledCapacity},
		{"dependable_mw", p.DependableCapacity},
		{"utilisation_pct", obs.Utilisation},
		{"gwh", obs.Generation},
		{"sum_gwh_reported", p.Total.Generation},
	} {
		if c.v.State() == nepraparse.StateUnknownText {
			parts = append(parts, fmt.Sprintf("unmodelled %s text %q", c.label, c.v.Raw()))
		}
	}
	if len(p.Residue) > 0 {
		parts = append(parts, fmt.Sprintf("%d cell(s) past logical column %d: %q",
			len(p.Residue), nepraparse.LogicalColumns-1, p.Residue))
	}
	return joinNotes(parts)
}

func joinNotes(parts []string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += "; "
		}
		out += p
	}
	return out
}

// genReconcileByRow maps each plant's source row index to its Sum verdict, or
// to nil when the row was not eligible for the check.
func genReconcileByRow(w *nepraparse.Workbook) map[int]*bool {
	rep := w.CheckSum(nepraparse.FieldGWh, 0)
	out := make(map[int]*bool, len(w.Plants))
	for _, c := range rep.Checks {
		ok := c.OK
		out[c.RowIndex] = &ok
	}
	return out
}

// genMW is a summed capacity that knows whether it is a measurement, in the
// shape nepraxwalk.MWSum marshals to.
//
// The `mw` key is OMITTED on an unmeasured sum. A bare float64 here is the
// single most dangerous field this command could emit: nepraxwalk shipped
// exactly that once and reported 0 MW for a fiscal year in which the workbook
// publishes 36,902 MW, while simultaneously reporting that nothing had gone
// unreported.
type genMW struct {
	Measured bool     `json:"measured"`
	MW       *float64 `json:"mw,omitempty"`
	// Plants is how many plants CONTRIBUTED a number.
	Plants int `json:"plants"`
	// PlantsNotNumeric is how many plants were in the population but whose
	// capacity cell was not a number, so the sum is partial by exactly that
	// many rows. A sum with a non-zero value here is still a real
	// measurement — over a smaller population, which is why the shortfall is
	// a field rather than a footnote.
	PlantsNotNumeric int `json:"plants_not_numeric"`
}

// genSumMW sums the numeric capacities of a population.
//
// An EMPTY population is a measured 0 over 0 plants — a real finding ("this
// year had no non-operating capacity"). A population whose every member is
// non-numeric is UNMEASURED, and its `mw` key does not appear.
func genSumMW(vs []nepraparse.Value) genMW {
	out := genMW{}
	total := 0.0
	for _, v := range vs {
		if n, ok := v.Float64(); ok {
			total += n
			out.Plants++
			continue
		}
		out.PlantsNotNumeric++
	}
	if out.Plants > 0 || out.PlantsNotNumeric == 0 {
		out.Measured = true
		mw := total
		out.MW = &mw
	}
	return out
}

// genRollupRow is one group-month.
//
// THERE IS NO UTILISATION FIELD, not even a null one. Sum == sum(12 months)
// holds on 118/118 GWh rows and only 3/118 "% age" rows in FY2023-24: that
// asymmetry is the proof the columns are percent-then-GWh, and it is also the
// proof that a utilisation is a ratio rather than an additive quantity. A
// summed percentage would be arithmetic nonsense and an unweighted mean would
// be a different plant-weighting than NEPRA's; a capacity-weighted load factor
// needs hours-per-month, which this corpus does not publish anywhere. So the
// honest answer is no field at all, and meta.refusals says so in words.
type genRollupRow struct {
	FY     string `json:"fy"`
	Rollup string `json:"rollup"`
	// Group is the published Technology or Fuel string VERBATIM, including
	// "THERMAL- COAL" with its upstream space and "Coal" as the only
	// mixed-case entry. A group outside the measured vocabulary is kept as
	// its own group and flagged in meta.warnings, never folded into a
	// neighbour and never re-spelled.
	Group      string `json:"group"`
	Month      string `json:"month"`
	MonthIndex int    `json:"month_index"`
	Period     string `json:"period"`
	// GWh is the sum over the cells in this group-month that carried a
	// number, and the key is ABSENT when there were none.
	GWh *float64 `json:"gwh,omitempty"`
	// GWhMeasured reports whether any cell was summable at all. False with
	// gwh absent is the correct answer for technology "Coal" in FY2023-24 —
	// one plant, Lakhra, DELICENSED — and 0 would be a fabrication.
	GWhMeasured bool `json:"gwh_measured"`
	// GWhCellsSummed with the five Cells* counts below says exactly how
	// complete the sum is. Their total is the group's plant count.
	GWhCellsSummed         int `json:"gwh_cells_summed"`
	CellsNotReported       int `json:"cells_not_reported"`
	CellsDelicensed        int `json:"cells_delicensed"`
	CellsDecommissioned    int `json:"cells_decommissioned"`
	CellsExportToKElectric int `json:"cells_export_to_k_electric"`
	CellsUnknownText       int `json:"cells_unknown_text"`

	Plants                  int `json:"plants"`
	PlantsActive            int `json:"plants_active"`
	PlantsDelicensed        int `json:"plants_delicensed"`
	PlantsDecommissioned    int `json:"plants_decommissioned"`
	PlantsListedNoData      int `json:"plants_listed_no_data"`
	PlantsExportToKElectric int `json:"plants_export_to_k_electric"`
	PlantsUnmodelledBlock   int `json:"plants_unmodelled_block"`

	// The four capacity sums each carry their own measured flag, because a
	// class whose members publish a sentinel instead of a number must not
	// report 0 MW. unmodelled_block gets its own bucket rather than being
	// folded into non_operating: a plant whose block reads "DELICENSE" is
	// PROBABLY non-operating, and probably is not a measurement.
	InstalledMWActive                  *float64 `json:"installed_mw_active,omitempty"`
	InstalledMWActiveMeasured          bool     `json:"installed_mw_active_measured"`
	InstalledMWNonOperating            *float64 `json:"installed_mw_non_operating,omitempty"`
	InstalledMWNonOperatingMeasured    bool     `json:"installed_mw_non_operating_measured"`
	InstalledMWListedNoData            *float64 `json:"installed_mw_listed_no_data,omitempty"`
	InstalledMWListedNoDataMeasured    bool     `json:"installed_mw_listed_no_data_measured"`
	InstalledMWUnmodelledBlock         *float64 `json:"installed_mw_unmodelled_block,omitempty"`
	InstalledMWUnmodelledBlockMeasured bool     `json:"installed_mw_unmodelled_block_measured"`

	SourceURL string `json:"source_url"`
}

// genRollupHeader is the rollup csv/tsv column order. There is no utilisation
// column in it.
var genRollupHeader = []string{
	"fy", "rollup", "group", "month", "month_index", "period",
	"gwh", "gwh_measured", "gwh_cells_summed",
	"cells_not_reported", "cells_delicensed", "cells_decommissioned",
	"cells_export_to_k_electric", "cells_unknown_text",
	"plants", "plants_active", "plants_delicensed", "plants_decommissioned",
	"plants_listed_no_data", "plants_export_to_k_electric", "plants_unmodelled_block",
	"installed_mw_active", "installed_mw_active_measured",
	"installed_mw_non_operating", "installed_mw_non_operating_measured",
	"installed_mw_listed_no_data", "installed_mw_listed_no_data_measured",
	"installed_mw_unmodelled_block", "installed_mw_unmodelled_block_measured",
	"source_url",
}

// Cells renders the rollup row in genRollupHeader order.
func (r genRollupRow) Cells() []string {
	return []string{
		r.FY, r.Rollup, r.Group, r.Month, genFormatInt(r.MonthIndex), r.Period,
		genFormatFloatPtr(r.GWh), genFormatBool(r.GWhMeasured), genFormatInt(r.GWhCellsSummed),
		genFormatInt(r.CellsNotReported), genFormatInt(r.CellsDelicensed), genFormatInt(r.CellsDecommissioned),
		genFormatInt(r.CellsExportToKElectric), genFormatInt(r.CellsUnknownText),
		genFormatInt(r.Plants), genFormatInt(r.PlantsActive), genFormatInt(r.PlantsDelicensed),
		genFormatInt(r.PlantsDecommissioned), genFormatInt(r.PlantsListedNoData),
		genFormatInt(r.PlantsExportToKElectric), genFormatInt(r.PlantsUnmodelledBlock),
		genFormatFloatPtr(r.InstalledMWActive), genFormatBool(r.InstalledMWActiveMeasured),
		genFormatFloatPtr(r.InstalledMWNonOperating), genFormatBool(r.InstalledMWNonOperatingMeasured),
		genFormatFloatPtr(r.InstalledMWListedNoData), genFormatBool(r.InstalledMWListedNoDataMeasured),
		genFormatFloatPtr(r.InstalledMWUnmodelledBlock), genFormatBool(r.InstalledMWUnmodelledBlockMeasured),
		r.SourceURL,
	}
}

// The two rollup kinds.
const (
	genRollupTechnology = "technology"
	genRollupFuel       = "fuel"
)

// genRollup aggregates the year by technology or fuel into group-month rows.
//
// It sums ONLY cells whose Value.Float64 second return is true, and counts
// every cell it could not sum by the reason it could not. A group whose cells
// were all non-numeric reports gwh_measured false with no number, which is the
// difference between "this group generated nothing" and "this group published
// nothing".
func genRollup(w *nepraparse.Workbook, kind, sourceURL string) []genRollupRow {
	key := func(p nepraparse.Plant) string { return p.Technology }
	if kind == genRollupFuel {
		key = func(p nepraparse.Plant) string { return p.Fuel }
	}

	members := map[string][]nepraparse.Plant{}
	for _, p := range w.Plants {
		members[key(p)] = append(members[key(p)], p)
	}
	groups := make([]string, 0, len(members))
	for g := range members {
		groups = append(groups, g)
	}
	sort.Strings(groups)

	fy := w.FiscalYear.Label()
	out := make([]genRollupRow, 0, len(groups)*12)
	for _, g := range groups {
		plants := members[g]
		// The per-class capacity sums are annual, not monthly, so they are
		// computed once per group and repeated across its twelve rows.
		var active, nonOperating, listed, unmodelled []nepraparse.Value
		counts := map[string]int{}
		for _, p := range plants {
			class := genRowClass(p)
			counts[class]++
			switch class {
			case genRowClassActive:
				active = append(active, p.InstalledCapacity)
			case genRowClassListedNoData:
				listed = append(listed, p.InstalledCapacity)
			case genRowClassUnmodelledBlock:
				unmodelled = append(unmodelled, p.InstalledCapacity)
			default:
				nonOperating = append(nonOperating, p.InstalledCapacity)
			}
		}
		activeMW := genSumMW(active)
		nonOpMW := genSumMW(nonOperating)
		listedMW := genSumMW(listed)
		unmodelledMW := genSumMW(unmodelled)

		for i, m := range nepraparse.MonthsInFiscalOrder {
			row := genRollupRow{
				FY: fy, Rollup: kind, Group: g,
				Month: m.String(), MonthIndex: m.FiscalIndex(),
				Plants:                             len(plants),
				PlantsActive:                       counts[genRowClassActive],
				PlantsDelicensed:                   counts[nepraparse.StateDelicensed.String()],
				PlantsDecommissioned:               counts[nepraparse.StateDecommissioned.String()],
				PlantsListedNoData:                 counts[genRowClassListedNoData],
				PlantsExportToKElectric:            counts[nepraparse.StateExportToKElectric.String()],
				PlantsUnmodelledBlock:              counts[genRowClassUnmodelledBlock],
				InstalledMWActive:                  activeMW.MW,
				InstalledMWActiveMeasured:          activeMW.Measured,
				InstalledMWNonOperating:            nonOpMW.MW,
				InstalledMWNonOperatingMeasured:    nonOpMW.Measured,
				InstalledMWListedNoData:            listedMW.MW,
				InstalledMWListedNoDataMeasured:    listedMW.Measured,
				InstalledMWUnmodelledBlock:         unmodelledMW.MW,
				InstalledMWUnmodelledBlockMeasured: unmodelledMW.Measured,
				SourceURL:                          sourceURL,
			}
			if pk, ok := w.FiscalYear.PeriodKey(m); ok {
				row.Period = pk
			}
			total := 0.0
			for _, p := range plants {
				v := p.Months[i].Generation
				if n, ok := v.Float64(); ok {
					total += n
					row.GWhCellsSummed++
					continue
				}
				switch v.State() {
				case nepraparse.StateNotReported:
					row.CellsNotReported++
				case nepraparse.StateDelicensed:
					row.CellsDelicensed++
				case nepraparse.StateDecommissioned:
					row.CellsDecommissioned++
				case nepraparse.StateExportToKElectric:
					row.CellsExportToKElectric++
				default:
					row.CellsUnknownText++
				}
			}
			if row.GWhCellsSummed > 0 {
				row.GWhMeasured = true
				gwh := total
				row.GWh = &gwh
			}
			out = append(out, row)
		}
	}
	return out
}

// genRowsJSON marshals rows for the artifact's content hash. Hashing the
// EXTRACTED rows rather than the markup is what makes the hash comparable
// across fetches of a site whose markup carries per-response tokens.
func genRowsJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		// A []genRow of plain scalars cannot fail to marshal; if it somehow
		// did, an empty content hash is better than a panic in a read-only
		// command.
		return nil
	}
	return b
}

// genFloatPtr converts a workbook cell to a nullable number.
//
// It returns nil for EVERY non-numeric state, and a pointer to a real 0 for a
// measured 0.00 — the whole point of the exercise. KAPCO publishes 0.00 in all
// twelve FY2023-24 months against 1,601 MW installed; Reshma publishes nothing
// at all against 97 MW installed. Those two must not render the same way.
func genFloatPtr(v nepraparse.Value) *float64 {
	n, ok := v.Float64()
	if !ok {
		return nil
	}
	return &n
}

// genFormatFloatPtr renders a nullable number for csv/tsv. An absent
// measurement is the empty string; the row's *_state column carries the
// reason.
//
// 'f' with precision -1 emits the shortest form that round-trips, so no
// precision is invented. The published thousands separators and trailing
// zeros are NOT reproduced: "1,601.00" becomes "1601". That is a rendering
// choice, not a value change, and it is documented in the flag help.
func genFormatFloatPtr(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func genFormatInt(v int) string { return strconv.Itoa(v) }

func genFormatBool(v bool) string { return strconv.FormatBool(v) }

// genFormatBoolPtr renders a nullable boolean. An ineligible Sum check is the
// empty string, never "false": "not checked" and "checked and failed" are
// different findings.
func genFormatBoolPtr(v *bool) string {
	if v == nil {
		return ""
	}
	return strconv.FormatBool(*v)
}
