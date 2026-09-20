// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-gen-plant-month-extract.json.

package cli

import (
	"fmt"
	"math"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// genRatio is a passed-of-eligible pair. Both halves travel, because "3
// passed" without "of 118" is not a finding.
type genRatio struct {
	Passed   int `json:"passed"`
	Eligible int `json:"eligible"`
}

type genSchemaMeta struct {
	Matches     bool     `json:"matches"`
	Fingerprint string   `json:"fingerprint"`
	Diff        []string `json:"diff"`
	BandLabel   string   `json:"band_label"`
	// PctLeafCount is how many leaf cells read exactly "% age". Every
	// sampled year has 13.
	PctLeafCount int `json:"pct_leaf_count"`
}

// genCensusMeta is nepraparse's own cell-state census, passed through
// unchanged plus its own balance verdict.
//
// It is in the output because a null-discipline claim nobody can check is
// worth nothing: these counts let a consumer re-derive that the six monthly
// states add up to plants x 26, that a blank was never counted as a zero, and
// that 471 measured zeros survived as measurements.
type genCensusMeta struct {
	MonthlyCellsTotal    int  `json:"monthly_cells_total"`
	Present              int  `json:"present"`
	NotReported          int  `json:"not_reported"`
	Delicensed           int  `json:"delicensed"`
	Decommissioned       int  `json:"decommissioned"`
	ExportToKElectric    int  `json:"export_to_k_electric"`
	UnknownText          int  `json:"unknown_text"`
	MeasuredZeros        int  `json:"measured_zeros"`
	ThousandsSeparated   int  `json:"thousands_separated"`
	NotReportedRows      int  `json:"not_reported_rows"`
	FullyNotReportedRows int  `json:"fully_not_reported_rows"`
	MixedBlankZeroRows   int  `json:"mixed_blank_zero_rows"`
	StatusRows           int  `json:"status_rows"`
	CapacityCells        int  `json:"capacity_cells"`
	CapacityPresent      int  `json:"capacity_present"`
	CapacityNotReported  int  `json:"capacity_not_reported"`
	CapacityStatus       int  `json:"capacity_status"`
	ResidueCells         int  `json:"residue_cells"`
	Balanced             bool `json:"balanced"`
}

func genCensus(c nepraparse.Census) genCensusMeta {
	return genCensusMeta{
		MonthlyCellsTotal:    c.MonthlyCellsTotal,
		Present:              c.Present,
		NotReported:          c.NotReported,
		Delicensed:           c.Delicensed,
		Decommissioned:       c.Decommissioned,
		ExportToKElectric:    c.ExportToKElectric,
		UnknownText:          c.UnknownText,
		MeasuredZeros:        c.MeasuredZeros,
		ThousandsSeparated:   c.ThousandsSeparated,
		NotReportedRows:      c.NotReportedRows,
		FullyNotReportedRows: c.FullyNotReportedRows,
		MixedBlankZeroRows:   c.MixedBlankZeroRows,
		StatusRows:           c.StatusRows,
		CapacityCells:        c.CapacityCells,
		CapacityPresent:      c.CapacityPresent,
		CapacityNotReported:  c.CapacityNotReported,
		CapacityStatus:       c.CapacityStatus,
		ResidueCells:         c.ResidueCells,
		Balanced:             c.Balanced(),
	}
}

// genColumnOrderMeta is the percent-then-GWh proof, re-derived from the
// published numbers on every run rather than trusted from the schema.
type genColumnOrderMeta struct {
	PctFirst    bool     `json:"pct_first"`
	GWh         genRatio `json:"gwh"`
	Pct         genRatio `json:"pct"`
	Explanation string   `json:"explanation"`
}

type genSumMismatch struct {
	SNo        int     `json:"sno"`
	Plant      string  `json:"plant"`
	MonthlySum float64 `json:"monthly_sum"`
	Reported   float64 `json:"sum_reported"`
	Delta      float64 `json:"delta"`
}

// genSumCheckMeta reports the Sum == sum(12 months) identity on GWh.
//
// A mismatch is REPORTED, never corrected. FY2020-21's "(NPPCL) - Balloki"
// publishes twelve months totalling 5,945.21 against a Sum of 5,905.65 — a
// +39.56 GWh discrepancy in NEPRA's own arithmetic. Both numbers reach the
// caller: the monthly cells in the rows, the published Sum in
// sum_gwh_reported, and the disagreement here.
type genSumCheckMeta struct {
	Field           string           `json:"field"`
	Tolerance       float64          `json:"tolerance"`
	Eligible        int              `json:"eligible"`
	Passed          int              `json:"passed"`
	Failed          int              `json:"failed"`
	Skipped         int              `json:"skipped"`
	SkippedByReason map[string]int   `json:"skipped_by_reason"`
	Mismatches      []genSumMismatch `json:"mismatches"`
}

// genCapacityMeta splits the year's published capacity by row class so every
// megawatt is attributable to a population.
//
// PublishedTotal is the sum over the plants whose Installed Capacity cell is a
// NUMBER, with the count of plants that could not contribute alongside it. It
// is deliberately not "the year's capacity": that phrasing would hide
// FY2020-21's three "Export to K.Electric" capacity cells, which are a
// statement rather than a figure.
type genCapacityMeta struct {
	PublishedTotal    genMW `json:"published_total"`
	Active            genMW `json:"active"`
	NonOperating      genMW `json:"non_operating"`
	ListedNoData      genMW `json:"listed_no_data"`
	ExportToKElectric genMW `json:"export_to_k_electric"`
	UnmodelledBlock   genMW `json:"unmodelled_block"`
}

// genAssertion is one completeness check with the evidence to re-check it.
type genAssertion struct {
	Name string `json:"name"`
	OK   bool   `json:"ok"`
	// Asserts is false when there was nothing to check against — an
	// unmeasured floor. A check that asserts nothing is reported as such
	// rather than reported as passing.
	Asserts  bool   `json:"asserts"`
	Expected *int   `json:"expected,omitempty"`
	Actual   *int   `json:"actual,omitempty"`
	AsOf     string `json:"as_of,omitempty"`
	Detail   string `json:"detail"`
}

// genMeta is everything the parse measured about the bytes it read.
type genMeta struct {
	Source          string `json:"source"`
	Grain           string `json:"grain"`
	FY              string `json:"fy"`
	BandLabel       string `json:"band_label"`
	Title           string `json:"title"`
	Charset         string `json:"charset"`
	CharsetDeclared bool   `json:"charset_declared"`
	Rows            int    `json:"rows"`
	Plants          int    `json:"plants"`
	Rollup          string `json:"rollup,omitempty"`
	Groups          int    `json:"groups,omitempty"`
	// FilterTechnology and FilterFuel record a `generation plants` /
	// `generation monthly` selection. They are omitempty because an absent
	// key is the honest shape for "no filter"; PlantsMatched is NOT, because
	// it is a measured count that must stay readable even when it equals
	// Plants.
	FilterTechnology string `json:"filter_technology,omitempty"`
	FilterFuel       string `json:"filter_fuel,omitempty"`
	// PlantsMatched is how many plants the filter selected, against Plants
	// for the whole year. Both are needed: every measurement in this block
	// except Rows is computed over the WHOLE workbook, so quoting Rows
	// against Plants alone reads as a contradiction under a filter.
	PlantsMatched   int  `json:"plants_matched"`
	LogicalColumns  int  `json:"logical_columns"`
	PhysicalWidth   int  `json:"physical_width"`
	TableCount      int  `json:"table_count"`
	HeaderCellCount int  `json:"header_cell_count"`
	TableRows       int  `json:"table_rows"`
	RawCells        int  `json:"raw_cells"`
	SeparatorRows   int  `json:"separator_rows"`
	SNoContiguous   bool `json:"sno_contiguous"`
	NamesUnique     bool `json:"names_unique"`

	Schema     genSchemaMeta     `json:"schema"`
	RowClasses genRowClassCounts `json:"row_classes"`
	// Reconcilability says how the active class and the Sum identity's
	// eligible set overlap, because in two of the seven published years they
	// are different sets.
	Reconcilability genReconcileSplit  `json:"reconcilability"`
	Census          genCensusMeta      `json:"census"`
	ColumnOrder     genColumnOrderMeta `json:"column_order"`
	SumCheck        genSumCheckMeta    `json:"sum_check"`
	CapacityMW      genCapacityMeta    `json:"capacity_mw"`

	Assertions []genAssertion `json:"assertions"`
	// Refusals are the questions this command will NOT answer, each with the
	// reason. A refusal in the payload is what stops a consumer inferring
	// that an absent field was an oversight.
	Refusals []string      `json:"refusals"`
	Warnings []string      `json:"warnings"`
	Artifact nepraArtifact `json:"artifact"`
}

// genRowClassCounts is a struct rather than a map so all five classes are
// always present. A class that is genuinely 0 in a year — export_to_k_electric
// in FY2023-24 — is a finding, and a map would have dropped the key.
type genRowClassCounts struct {
	Active            int `json:"active"`
	Delicensed        int `json:"delicensed"`
	Decommissioned    int `json:"decommissioned"`
	ExportToKElectric int `json:"export_to_k_electric"`
	ListedNoData      int `json:"listed_no_data"`
	// UnmodelledBlock is 2 in FY2022-23 and 0 in every other published year.
	// See genRowClassUnmodelledBlock: those two rows read "DELICENSE".
	UnmodelledBlock int `json:"unmodelled_block"`
}

// Total is the plant count the five classes account for.
func (c genRowClassCounts) Total() int {
	return c.Active + c.Delicensed + c.Decommissioned + c.ExportToKElectric +
		c.ListedNoData + c.UnmodelledBlock
}

func genCountRowClasses(w *nepraparse.Workbook) genRowClassCounts {
	var out genRowClassCounts
	for _, p := range w.Plants {
		switch genRowClass(p) {
		case genRowClassActive:
			out.Active++
		case nepraparse.StateDelicensed.String():
			out.Delicensed++
		case nepraparse.StateDecommissioned.String():
			out.Decommissioned++
		case nepraparse.StateExportToKElectric.String():
			out.ExportToKElectric++
		case genRowClassListedNoData:
			out.ListedNoData++
		case genRowClassUnmodelledBlock:
			out.UnmodelledBlock++
		}
	}
	return out
}

func genCapacity(w *nepraparse.Workbook) genCapacityMeta {
	var all, active, nonOperating, listed, export, unmodelled []nepraparse.Value
	for _, p := range w.Plants {
		all = append(all, p.InstalledCapacity)
		switch genRowClass(p) {
		case genRowClassActive:
			active = append(active, p.InstalledCapacity)
		case genRowClassListedNoData:
			listed = append(listed, p.InstalledCapacity)
		case genRowClassUnmodelledBlock:
			unmodelled = append(unmodelled, p.InstalledCapacity)
		case nepraparse.StateExportToKElectric.String():
			export = append(export, p.InstalledCapacity)
		default:
			nonOperating = append(nonOperating, p.InstalledCapacity)
		}
	}
	return genCapacityMeta{
		PublishedTotal:    genSumMW(all),
		Active:            genSumMW(active),
		NonOperating:      genSumMW(nonOperating),
		ListedNoData:      genSumMW(listed),
		ExportToKElectric: genSumMW(export),
		UnmodelledBlock:   genSumMW(unmodelled),
	}
}

// genBuildMeta assembles the meta block from what the parse measured.
func genBuildMeta(w *nepraparse.Workbook, y genYear, grain string, rows int, body []byte, sourceURL, contentType string, content []byte) genMeta {
	order := w.ColumnOrder(0)
	gwh := order.GWh
	skipped := map[string]int{}
	for _, s := range gwh.Skipped {
		skipped[s.Reason.String()]++
	}
	mismatches := make([]genSumMismatch, 0, len(gwh.Mismatches))
	for _, m := range gwh.Mismatches {
		mismatches = append(mismatches, genSumMismatch{
			SNo: m.SNo, Plant: m.Plant,
			MonthlySum: m.MonthlySum, Reported: m.Reported, Delta: m.Delta,
		})
	}

	meta := genMeta{
		Source:          "live",
		Grain:           grain,
		FY:              w.FiscalYear.Label(),
		BandLabel:       w.Header.BandLabel,
		Title:           w.Title,
		Charset:         w.Charset,
		CharsetDeclared: w.CharsetDeclared,
		Rows:            rows,
		Plants:          len(w.Plants),
		// Default to the whole workbook: unfiltered callers (`gen`) have no
		// matcher, so every plant is "matched". The filtered `generation
		// plants`/`monthly` path overwrites this with the selected count.
		// Leaving it 0 beside a non-zero Plants reads as a failed join.
		PlantsMatched:   len(w.Plants),
		LogicalColumns:  w.LogicalWidth,
		PhysicalWidth:   w.PhysicalWidth,
		TableCount:      w.TableCount,
		HeaderCellCount: w.HeaderCellCount,
		TableRows:       w.TableRows,
		RawCells:        w.RawCells,
		SeparatorRows:   w.SeparatorRows,
		SNoContiguous:   w.SNoContiguous,
		NamesUnique:     w.NamesUnique,
		Schema: genSchemaMeta{
			Matches:      w.Header.Matches(),
			Fingerprint:  w.Header.Fingerprint(),
			Diff:         w.Header.Diff(),
			BandLabel:    w.Header.BandLabel,
			PctLeafCount: w.Header.PctLeafCount,
		},
		RowClasses:      genCountRowClasses(w),
		Reconcilability: genReconcilability(w),
		Census:          genCensus(w.Census),
		ColumnOrder: genColumnOrderMeta{
			PctFirst:    order.PctFirst,
			GWh:         genRatio{Passed: order.GWh.Passed, Eligible: order.GWh.Eligible},
			Pct:         genRatio{Passed: order.Pct.Passed, Eligible: order.Pct.Eligible},
			Explanation: order.Explanation,
		},
		SumCheck: genSumCheckMeta{
			Field:           gwh.Field.String(),
			Tolerance:       gwh.Tolerance,
			Eligible:        gwh.Eligible,
			Passed:          gwh.Passed,
			Failed:          gwh.Failed,
			Skipped:         len(gwh.Skipped),
			SkippedByReason: skipped,
			Mismatches:      mismatches,
		},
		CapacityMW: genCapacity(w),
		Warnings:   w.Warnings,
		Artifact:   newNepraArtifact(sourceURL, body, content, contentType, rows, 0),
	}
	if meta.Schema.Diff == nil {
		meta.Schema.Diff = []string{}
	}
	if meta.Warnings == nil {
		meta.Warnings = []string{}
	}
	meta.Warnings = append(genUnmodelledBlockWarnings(w), meta.Warnings...)
	meta.Refusals = genRefusals(w, order, meta.CapacityMW)
	meta.Assertions = genAssertions(w, y, grain, rows, len(body), order)
	return meta
}

// genUnmodelledBlockWarnings names every row whose whole monthly block is one
// text this build cannot model, with the text verbatim.
//
// nepraparse already warns per CELL, which for FY2022-23 is 52 lines saying
// the same thing twice per month. This is the one line per ROW that says what
// it means, and it goes FIRST so it is not lost in that list.
func genUnmodelledBlockWarnings(w *nepraparse.Workbook) []string {
	var out []string
	for _, p := range w.Plants {
		if genRowClass(p) != genRowClassUnmodelledBlock {
			continue
		}
		text := ""
		if raw := genUniformBlock(p, nepraparse.StateUnknownText); raw != nil {
			text = *raw
		}
		out = append(out, fmt.Sprintf("UNMODELLED BLOCK: %s has all %d monthly cells reading %q, which this "+
			"build does not model. The row is classed %q and every cell stays unknown_text — the text is NOT "+
			"normalised to a known sentinel, because a missing letter is not evidence of intent.",
			p.Name, nepraparse.MonthlyCells, text, genRowClassUnmodelledBlock))
	}
	return out
}

// genRefusals states what this command will not answer, and why.
func genRefusals(w *nepraparse.Workbook, order nepraparse.ColumnOrderVerdict, capMW genCapacityMeta) []string {
	return []string{
		fmt.Sprintf("NO AGGREGATE UTILISATION IS EMITTED, in any grain. Sum == sum(12 months) holds %d/%d "+
			"on GWh and only %d/%d on \"%s\" in this very file: utilisation is a ratio, not an additive "+
			"quantity. A summed percentage would be arithmetic nonsense, an unweighted mean would impose a "+
			"plant-weighting NEPRA never published, and a capacity-weighted load factor needs hours-per-month, "+
			"which this corpus does not publish anywhere. The per-plant-month utilisation_pct column carries "+
			"every published figure verbatim; combining them is the caller's decision to document.",
			order.GWh.Passed, order.GWh.Eligible, order.Pct.Passed, order.Pct.Eligible, nepraparse.MetricPct),

		"NO CROSS-YEAR PANEL. S.No is an in-year ordinal only — 0 of 106 plants kept their serial into " +
			"FY2023-24 because the file was re-sorted by technology — and plant names drift " +
			"(\"Narowal Energy Ltd. (HUBCO)\" became \"Narowal Energy Ltd.\"). Joining years on either key " +
			"silently shuffles the panel, so this command reads ONE year per invocation.",

		"NO LOCAL STORE, so no offline or historical read. There is no sync path for the generation panel " +
			"(the store carries candidates, events, extras, learnings and playbooks only), which is why " +
			"--data-source local is refused rather than quietly served from the live URL.",

		fmt.Sprintf("NO RAW-CELL HISTOGRAM AND NO SPAN-COLLISION COUNT. The %d raw <td> and the %d-column "+
			"physical width are reported, but the per-row <td> distribution and the colspan/rowspan collision "+
			"count live on nepraparse.Grid, which ParseWorkbook does not return. Recovering them would mean "+
			"decoding and re-gridding the whole workbook a second time; that cost was not paid, so those two "+
			"numbers are ABSENT rather than estimated.", w.RawCells, w.PhysicalWidth),

		fmt.Sprintf("NO RECONCILIATION TO ANY OTHER PARSER'S TOTAL. An independent parse of FY2023-24 "+
			"reported 44,675 MW (40,614 active + 4,061 non-generating); THIS parse measures %s for the year "+
			"in hand. For FY2023-24 the 181 MW of that gap is explained — the two rows listed with capacity "+
			"and no monthly data, Reshma 97 and Gulf Powergen 84, which the other parse folded into its "+
			"non-generating bucket — but 11.5 MW sits inside the reporting rows and neither side accounts "+
			"for it. capacity_mw prints what THIS parse measured with the population behind each figure, "+
			"and is not adjusted to agree with anything.", genMWString(capMW.PublishedTotal)),

		"DELICENSED AND DECOMMISSIONED ARE NOT GLOSSED. NEPRA publishes both tokens with no legend, glossary " +
			"or footnote anywhere in the seven files. They travel verbatim; what distinguishes them is not " +
			"stated by the source and is not inferred here.",
	}
}

// genAssertions is the completeness block.
//
// Every entry either checks a MEASURED floor or says plainly that it checks
// nothing. A floor of 0 is not a floor.
func genAssertions(w *nepraparse.Workbook, y genYear, grain string, rows, bodyBytes int, order nepraparse.ColumnOrderVerdict) []genAssertion {
	classes := genCountRowClasses(w)
	plants := len(w.Plants)
	out := []genAssertion{}

	if y.PlantRows > 0 {
		pinned := "measured live, not pinned by a committed fixture"
		if y.PlantRowsPinned {
			pinned = "pinned by a committed fixture"
		}
		out = append(out, genAssertion{
			Name: "plant_row_floor", Asserts: true, OK: plants >= y.PlantRows,
			Expected: genIntPtr(y.PlantRows), Actual: genIntPtr(plants), AsOf: genAsOfDate,
			Detail: fmt.Sprintf("FY%s published %d plant rows on %s (%s); fewer than that is a truncated read, not a smaller year",
				y.Label, y.PlantRows, genAsOfDate, pinned),
		})
	} else {
		out = append(out, genAssertion{
			Name: "plant_row_floor", Asserts: false, OK: true,
			Actual: genIntPtr(plants), AsOf: genAsOfDate,
			Detail: fmt.Sprintf("NO plant-row floor was measured for FY%s, so %d rows is reported and asserted against nothing. "+
				"Only FY2017-18, FY2020-21 and FY2023-24 have a committed fixture behind their count.", y.Label, plants),
		})
	}

	if y.DecodedBytes > 0 {
		out = append(out, genAssertion{
			Name: "decoded_bytes_floor", Asserts: true, OK: bodyBytes >= y.DecodedBytes,
			Expected: genIntPtr(y.DecodedBytes), Actual: genIntPtr(bodyBytes), AsOf: genAsOfDate,
			Detail: fmt.Sprintf("FY%s decoded to exactly %d bytes on %s with Accept: */* and reproduces that "+
				"length on re-fetch; a short body is the shape a truncated response takes under an HTTP 200. "+
				"It is a FLOOR and not an equality because this client sends Accept: text/html, which makes "+
				"Cloudflare inject a ~361-byte RUM beacon <script> after </html> — measured, 493,548 against "+
				"493,187 for FY2023-24. The beacon carries a per-deploy integrity hash, so it also moves "+
				"sha256_raw without the document changing; sha256_content is over the extracted rows and does not.",
				y.Label, y.DecodedBytes, genAsOfDate),
		})
	} else {
		out = append(out, genAssertion{
			Name: "decoded_bytes_floor", Asserts: false, OK: true, Actual: genIntPtr(bodyBytes),
			Detail: fmt.Sprintf("NO byte floor was measured for FY%s; %d bytes is reported and asserted against nothing", y.Label, bodyBytes),
		})
	}

	out = append(out, genAssertion{
		Name: "schema_matches", Asserts: true, OK: w.Header.Matches(),
		Detail: fmt.Sprintf("the 32-column header band matched the measured schema; %d leaf cells read exactly %q", w.Header.PctLeafCount, nepraparse.MetricPct),
	})

	out = append(out, genAssertion{
		Name: "census_balanced", Asserts: true, OK: w.Census.Balanced(),
		Expected: genIntPtr(w.Census.MonthlyCellsTotal),
		Actual: genIntPtr(w.Census.Present + w.Census.NotReported + w.Census.Delicensed +
			w.Census.Decommissioned + w.Census.ExportToKElectric + w.Census.UnknownText),
		Detail: fmt.Sprintf("the six monthly cell states must account for every one of %d plants x %d cells; a shortfall means a cell escaped classification",
			len(w.Plants), nepraparse.MonthlyCells),
	})

	out = append(out, genAssertion{
		Name: "pct_then_gwh_proved", Asserts: true, OK: order.PctFirst,
		Detail: order.Explanation,
	})

	out = append(out, genAssertion{
		Name: "rows_equal_plants_times_12", Asserts: grain == "plant-month", OK: grain != "plant-month" || rows == plants*12,
		Expected: genIntPtr(plants * 12), Actual: genIntPtr(rows),
		Detail: "at the plant-month grain every plant gets twelve rows whatever its class, so a delicensed or unreported plant cannot vanish from the extract",
	})

	out = append(out, genAssertion{
		Name: "row_classes_account_for_every_plant", Asserts: true, OK: classes.Total() == plants,
		Expected: genIntPtr(plants), Actual: genIntPtr(classes.Total()),
		Detail: "the five row classes are a partition; a plant in none of them would be a silent drop",
	})

	// THE SPEC FOR THIS COMMAND ASSERTED active == SumReport.Eligible. That
	// EQUALITY IS FALSE, and only the three fixture years hide it: measured
	// live on 2026-09-10, FY2021-22 has 125 active plants against 116
	// eligible and FY2022-23 has 118 against 116, because a plant whose
	// monthly block is PARTLY blank is active (it reported something) and
	// ineligible (it cannot be summed). Asserting the equality would have
	// made two of the seven published years fail a completeness gate for
	// being published the way NEPRA published them.
	//
	// What IS an invariant is the containment: a plant whose thirteen GWh
	// cells are all numeric cannot be delicensed, decommissioned or
	// all-blank, so it must be active. The residual is reported as a count
	// rather than treated as a fault.
	reach := genReconcilability(w)
	out = append(out, genAssertion{
		Name: "every_reconcilable_plant_is_active", Asserts: true, OK: reach.EligibleNotActive == 0,
		Expected: genIntPtr(0), Actual: genIntPtr(reach.EligibleNotActive),
		Detail: fmt.Sprintf("a plant whose thirteen GWh cells are all numeric must be classed active; %d of %d active plants are NOT eligible for the Sum identity, which is a partly-blank published block and not an error",
			reach.ActiveNotEligible, classes.Active),
	})

	if y.StatusRows >= 0 {
		out = append(out, genAssertion{
			Name: "status_rows_year_bounded", Asserts: true, OK: w.Census.StatusRows == y.StatusRows,
			Expected: genIntPtr(y.StatusRows), Actual: genIntPtr(w.Census.StatusRows), AsOf: genAsOfDate,
			Detail: fmt.Sprintf("DELICENSED/DECOMMISSIONED appear only in FY2022-23 and FY2023-24 across all seven published years; FY%s measured %d status rows on %s",
				y.Label, y.StatusRows, genAsOfDate),
		})
	} else {
		out = append(out, genAssertion{
			Name: "status_rows_year_bounded", Asserts: false, OK: true, Actual: genIntPtr(w.Census.StatusRows),
			Detail: fmt.Sprintf("NO status-row count was measured for FY%s; %d is reported and asserted against nothing", y.Label, w.Census.StatusRows),
		})
	}

	capMW := genCapacity(w)
	if exhausts, checkable := genCapacityIdentity(capMW); checkable {
		out = append(out, genAssertion{
			Name: "capacity_classes_exhaust_published_total", Asserts: true, OK: exhausts,
			Detail: "active + non_operating + listed_no_data + export_to_k_electric + unmodelled_block must equal published_total to 1e-9, so every published megawatt is attributable to a class",
		})
	} else {
		out = append(out, genAssertion{
			Name: "capacity_classes_exhaust_published_total", Asserts: false, OK: true,
			Detail: "at least one capacity class is unmeasured — a plant in it publishes a sentinel rather than a number — so the classes cannot be checked against the total, and neither side was rounded to make them agree",
		})
	}
	return out
}

// genReconcileSplit counts how the active class and the Sum-identity's
// eligible set overlap. They are NOT the same set in every year.
type genReconcileSplit struct {
	// ActiveAndEligible is a plant that reported and can be summed.
	ActiveAndEligible int `json:"active_and_eligible"`
	// ActiveNotEligible is a plant that reported SOMETHING but whose
	// thirteen GWh cells are not all numeric — a partly-blank block. It
	// keeps its twelve rows and its sum_reconciles is null, not false.
	ActiveNotEligible int `json:"active_not_eligible"`
	// EligibleNotActive must be 0: an all-numeric block cannot be
	// delicensed, decommissioned or all-blank. A non-zero value means the
	// row_class rule and the parser disagree about who reported.
	EligibleNotActive int `json:"eligible_not_active"`
}

// genReconcilability computes that overlap.
func genReconcilability(w *nepraparse.Workbook) genReconcileSplit {
	eligible := map[int]bool{}
	for _, c := range w.CheckSum(nepraparse.FieldGWh, 0).Checks {
		eligible[c.RowIndex] = true
	}
	var out genReconcileSplit
	for _, p := range w.Plants {
		active := genRowClass(p) == genRowClassActive
		switch {
		case active && eligible[p.RowIndex]:
			out.ActiveAndEligible++
		case active:
			out.ActiveNotEligible++
		case eligible[p.RowIndex]:
			out.EligibleNotActive++
		}
	}
	return out
}

// genCapacityIdentity reports whether the four class sums add to the published
// total, and whether the check was possible at all.
func genCapacityIdentity(c genCapacityMeta) (ok bool, checkable bool) {
	parts := []genMW{c.Active, c.NonOperating, c.ListedNoData, c.ExportToKElectric, c.UnmodelledBlock}
	if !c.PublishedTotal.Measured || c.PublishedTotal.MW == nil {
		return false, false
	}
	sum := 0.0
	for _, p := range parts {
		if !p.Measured || p.MW == nil {
			return false, false
		}
		sum += *p.MW
	}
	return math.Abs(sum-*c.PublishedTotal.MW) <= 1e-9, true
}

func genIntPtr(v int) *int { return &v }

// genShortfalls returns the assertions that actually failed. An assertion that
// asserts nothing can never appear here.
func genShortfalls(as []genAssertion) []genAssertion {
	var out []genAssertion
	for _, a := range as {
		if a.Asserts && !a.OK {
			out = append(out, a)
		}
	}
	return out
}
