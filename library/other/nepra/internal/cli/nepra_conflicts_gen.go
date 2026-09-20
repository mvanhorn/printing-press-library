// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The generation-workbook derivation leg: rows that fail their own arithmetic.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// conflictsGenYears are the fiscal years whose generation workbook is
// reachable. MEASURED: FY2016-17, FY2024-25 and FY2025-26 all 404.
var conflictsGenYears = []string{
	"FY2017-18", "FY2018-19", "FY2019-20", "FY2020-21", "FY2021-22", "FY2022-23", "FY2023-24",
}

// conflictsGenYearReachable reports whether a fiscal year's workbook is
// published.
func conflictsGenYearReachable(fy string) bool {
	want := strings.ToUpper(strings.TrimSpace(fy))
	if !strings.HasPrefix(want, "FY") {
		want = "FY" + want
	}
	for _, y := range conflictsGenYears {
		if y == want {
			return true
		}
	}
	return false
}

// conflictsWorkbookBytes are the decoded sizes of the workbooks this build has
// actually weighed, MEASURED on 10 Sep 2026. A year absent here has an unknown
// size and is reported as unknown rather than estimated.
//
// They are a cost ESTIMATE, not an identity. The corpus survey weighed the
// same two files at 455,640 and 493,187 bytes and both are now 361 bytes
// larger, so the size of a NEPRA page drifts without its data changing —
// which is the same reason the artifact provenance carries a content hash
// alongside the raw one.
var conflictsWorkbookBytes = map[string]int{
	"FY2020-21": 456001,
	"FY2023-24": 493548,
}

// conflictsFetchWorkbook downloads one generation workbook.
//
// The bytes are returned UNDECODED, because ParseWorkbook does the decoding:
// the files are windows-1252 and the HTTP response declares NO charset, so a
// naive UTF-8 read of a 493,187-byte file reports zero table rows.
func conflictsFetchWorkbook(ctx context.Context, c *client.Client, fy string) (raw []byte, url, contentType string, err error) {
	if !conflictsGenYearReachable(fy) {
		return nil, "", "", notFoundErr(fmt.Errorf(
			"no generation workbook is published for %s; the reachable years are %s",
			fy, strings.Join(conflictsGenYears, ", ")))
	}
	path := conflictGenPath(fy)
	body, err := c.GetWithHeaders(ctx, path, nil,
		map[string]string{client.HTMLResponseHeader: "true"})
	if err != nil {
		return nil, c.BaseURL + path, "", err
	}
	return body, c.BaseURL + path, c.LastContentType(), nil
}

// conflictsColumnOrderGuard refuses the whole arithmetic leg when the
// workbook's column order is not provable from the data.
//
// The pair order is "% age" then "GWh", and the proof is that Sum ==
// sum(12 months) is an identity for energy and not for a utilisation ratio. If
// a future file swapped the two, every GWh value read would actually be a
// utilisation and every number would still look plausible — so the leg
// REFUSES rather than reports.
func conflictsColumnOrderGuard(w *nepraparse.Workbook) error {
	v := w.ColumnOrder(nepraparse.DefaultSumTolerance)
	if v.PctFirst {
		return nil
	}
	return fmt.Errorf(
		"the workbook's month-pair column order is not provable from its own numbers, so no arithmetic "+
			"entry is emitted: %s", v.Explanation)
}

// genSumEntries turns each published-Sum failure into an entry.
//
// The derived side is LABELLED derived and never replaces the published Sum:
// which side is wrong is not determinable from the document. Rows carrying a
// blank, a status sentinel or unmodelled text are SKIPPED by CheckSum and are
// never treated as zeros; the eligible count travels with the finding so a
// regression that widened the tolerance is visible.
func genSumEntries(w *nepraparse.Workbook, fy string) []conflictEntry {
	rep := w.CheckSum(nepraparse.FieldGWh, nepraparse.DefaultSumTolerance)
	url := conflictGenURL(fy)
	norm := "FY" + conflictBareFY(fy)
	out := make([]conflictEntry, 0, len(rep.Mismatches))
	for _, m := range rep.Mismatches {
		key := conflictKey{PeriodFY: norm, Entity: m.Plant, Metric: "generation_gwh"}
		pub, der := fptr(m.Reported), fptr(m.MonthlySum)
		out = append(out, conflictEntry{
			ID:      conflictEntryID(conflictSurfaceGen, conflictKindArithmetic, key, "sum"),
			Surface: conflictSurfaceGen,
			Kind:    conflictKindArithmetic,
			SameKey: false,
			Key:     key,
			A: conflictSide{
				Raw: fmt.Sprintf("%.2f", m.Reported), Value: pub, ValueKind: "numeric", Unit: "GWh",
				ReportFY: norm, Table: "published Sum column", Variant: "as_published", URL: url,
			},
			B: conflictSide{
				Raw: "", Value: der, ValueKind: "derived", Unit: "GWh", ReportFY: norm,
				Table: "sum of the row's own twelve monthly GWh cells", Variant: "derived", URL: url,
				Basis: fmt.Sprintf("twelve monthly GWh cells of source row %d", m.RowIndex),
			},
			Ratio:                 conflictRatio(pub, der),
			AbsDiff:               conflictAbsDiff(pub, der),
			Direction:             "unknowable",
			Reconciled:            false,
			Computable:            conflictVerified,
			Derivation:            "recomputed from the published workbook by nepraparse.CheckSum",
			ReproducedByThisBuild: true,
			RecomputeNeeds:        []string{norm},
			Population: &conflictPopulation{
				FiscalYear:  norm,
				Basis:       "plant rows whose thirteen GWh cells are all numeric (Sum plus twelve months)",
				Denominator: rep.Eligible,
				Flagged:     rep.Failed,
			},
			Note: fmt.Sprintf("the row's own twelve monthly cells do not add up to its own annual Sum "+
				"(delta %+.2f GWh at tolerance %.2f); %d of %d eligible rows reconcile and %d were skipped "+
				"as not-reported, status-sentinel or unmodelled text rather than summed as zeros",
				m.Delta, rep.Tolerance, rep.Passed, rep.Eligible, len(rep.Skipped)),
		})
	}
	return out
}

// genZeroLoadFactorEntries finds cells publishing generation against a
// utilisation of exactly 0.00, and gives each one its own verdict.
//
// A bare count here would be a wrong answer. Reporting all of them as
// contradictions would be wrong too: one of the ten measured rows publishes an
// implied utilisation that genuinely ROUNDS to 0.00 at the workbook's own two
// decimals, and one has an installed capacity of 0 so no utilisation exists to
// compare against. So each entry carries its implied utilisation, the
// denominator it was drawn from, and its own verdict.
//
// The scan reads only cells where BOTH members of the pair are numeric. An
// NBSP blank is never treated as zero: the twin count of blank-utilisation
// rows with positive generation is reported so that a regression coercing
// StateNotReported to 0.0 becomes visible instead of inventing findings.
func genZeroLoadFactorEntries(w *nepraparse.Workbook, fy string) ([]conflictEntry, genScanCensus) {
	norm := "FY" + conflictBareFY(fy)
	url := conflictGenURL(fy)
	census := genScanCensus{FiscalYear: norm}

	var out []conflictEntry
	for _, p := range w.Plants {
		obs := make([]nepraparse.MonthlyObservation, 0, nepraparse.PairCount)
		obs = append(obs, p.Months[:]...)
		obs = append(obs, p.Total)
		for idx, o := range obs {
			gen, genOK := o.Generation.Float64()
			util, utilOK := o.Utilisation.Float64()
			if o.Utilisation.State() == nepraparse.StateNotReported && genOK && gen > 0 {
				// The negative control. A non-zero count here means a
				// regression coerced a blank into a zero.
				census.BlankUtilisationWithGeneration++
			}
			if !genOK || !utilOK {
				continue
			}
			if o.IsTotal {
				census.SumPairsBothNumeric++
			} else {
				census.PlantMonthsBothNumeric++
			}
			if gen <= 0 || util != 0 {
				continue
			}
			if o.IsTotal {
				census.SumPairsFlagged++
			} else {
				census.PlantMonthsFlagged++
			}
			out = append(out, genZeroLoadFactorEntry(p, o, idx, norm, fy, url, gen))
		}
	}
	// Attach the denominators only after the whole year has been counted, so
	// no entry quotes a partial population.
	for i := range out {
		if out[i].Population == nil {
			continue
		}
		if out[i].Population.Basis == zlfSumBasis {
			out[i].Population.Denominator = census.SumPairsBothNumeric
			out[i].Population.Flagged = census.SumPairsFlagged
			continue
		}
		out[i].Population.Denominator = census.PlantMonthsBothNumeric
		out[i].Population.Flagged = census.PlantMonthsFlagged
	}
	return out, census
}

// genScanCensus is what the zero-load-factor scan actually looked at. It is
// reported so a count is never quotable without its denominator.
type genScanCensus struct {
	FiscalYear string `json:"fiscal_year"`
	// PlantMonthsBothNumeric is the population: cells where the "% age" and
	// GWh members of one month pair are both numbers.
	PlantMonthsBothNumeric int `json:"plant_months_both_numeric"`
	PlantMonthsFlagged     int `json:"plant_months_flagged"`
	// The annual Sum pair, counted separately because a Sum is not a month.
	SumPairsBothNumeric int `json:"sum_pairs_both_numeric"`
	SumPairsFlagged     int `json:"sum_pairs_flagged"`
	// BlankUtilisationWithGeneration is the negative control that proves
	// blank was never read as zero. It must be 0.
	BlankUtilisationWithGeneration int `json:"blank_utilisation_with_generation"`
}

func genZeroLoadFactorEntry(p nepraparse.Plant, o nepraparse.MonthlyObservation, idx int,
	norm, fy, url string, gen float64) conflictEntry {
	key := conflictKey{PeriodFY: norm, Entity: p.Name, Metric: "utilisation_pct"}
	published := fptr(0)

	capMW, capKnown := p.InstalledCapacity.Float64()
	implied, impliedOK := zlfImplied(gen, capMW, capKnown, fy, idx, o.IsTotal)
	verdict := zlfVerdictFor(implied, impliedOK)
	basis := zlfBasisFor(gen, capMW, capKnown, p.InstalledCapacity.State().String(), fy, idx, o.IsTotal)
	var derived *float64
	derivedKind := "not_reported"
	if impliedOK {
		derived = fptr(implied)
		derivedKind = "derived"
	}

	pop := &conflictPopulation{FiscalYear: norm, Basis: zlfBasis}
	if o.IsTotal {
		pop = &conflictPopulation{FiscalYear: norm, Basis: zlfSumBasis}
	}

	return conflictEntry{
		ID:      conflictEntryID(conflictSurfaceGen, conflictKindArithmetic, key, "zlf-"+strings.ToLower(o.Label())),
		Surface: conflictSurfaceGen,
		Kind:    conflictKindArithmetic,
		SameKey: false,
		Key:     key,
		A: conflictSide{
			Raw: o.Utilisation.Raw(), Value: published, ValueKind: "numeric", Unit: "%", ReportFY: norm,
			Table: "published \"% age\" cell, " + o.Label(), Variant: "as_published", URL: url,
			Basis: fmt.Sprintf("published alongside %.2f GWh in the same month pair", gen),
		},
		B: conflictSide{
			Raw: "", Value: derived, ValueKind: derivedKind, Unit: "%", ReportFY: norm,
			Table:   "utilisation implied by the row's own GWh, installed capacity and day count",
			Variant: "derived", URL: url, Basis: basis,
		},
		Ratio:                 conflictRatio(published, derived),
		AbsDiff:               conflictAbsDiff(published, derived),
		Direction:             "unknowable",
		Reconciled:            false,
		Computable:            conflictVerified,
		Derivation:            "recomputed from the published workbook's own cells",
		ReproducedByThisBuild: true,
		RecomputeNeeds:        []string{norm},
		Population:            pop,
		Note:                  zlfNoteFor(verdict),
	}
}

// zlfImplied returns the utilisation the row's OWN cells imply, and whether it
// exists at all.
//
// A false second return means there is no denominator — installed capacity is
// 0 MW, or was never published — so no utilisation exists to compare the
// published 0.00 against. It does NOT mean the implied utilisation is zero,
// and the entry that carries it emits no `value` key on that side.
//
// The shipped ledger and this derivation leg both call it, so the ledger's
// numbers are literally this code's output rather than transcriptions of it.
func zlfImplied(gwh, capMW float64, capKnown bool, fy string, idx int, isTotal bool) (float64, bool) {
	if !capKnown || capMW <= 0 {
		return 0, false
	}
	// THE ANNUAL DENOMINATOR IS SUMMED FROM THE REAL MONTHS, not assumed to
	// be 365 days.
	//
	// It used to be hardcoded 24*365 while the monthly legs called
	// daysInFiscalMonth. Two of the seven reachable fiscal years span a leap
	// day, so on those the annual denominator was a full day short and the
	// derived utilisation came out 1/365 — about 0.274% — too high. Worse,
	// the annual figure then disagreed with the sum of the twelve monthly
	// denominators this same function computes, so one command contradicted
	// itself depending on which leg you read.
	days := 0
	if isTotal {
		for i := 0; i < genMonthsPerPlant; i++ {
			days += daysInFiscalMonth(fy, i)
		}
	} else {
		days = daysInFiscalMonth(fy, idx)
	}
	if days == 0 {
		return 0, false
	}
	hours := 24.0 * float64(days)
	return 100 * gwh * 1000 / (capMW * hours), true
}

// zlfBasisFor states the arithmetic, or the reason there is none.
func zlfBasisFor(gwh, capMW float64, capKnown bool, capState, fy string, idx int, isTotal bool) string {
	if !capKnown {
		return "installed capacity is not a published number for this plant (" + capState +
			"), so there is no denominator and no utilisation exists to compare the published 0.00 against"
	}
	if capMW <= 0 {
		return "installed capacity is published as 0 MW, so there is no denominator and no utilisation " +
			"exists to compare the published 0.00 against"
	}
	// The basis must state the denominator ACTUALLY used, or a reader cannot
	// reproduce the figure. It previously printed the literal "365 days" for
	// every year, which was false on the two leap-spanning fiscal years.
	days := 0
	if isTotal {
		for i := 0; i < genMonthsPerPlant; i++ {
			days += daysInFiscalMonth(fy, i)
		}
	} else {
		days = daysInFiscalMonth(fy, idx)
	}
	dayBasis := fmt.Sprintf("%d days", days)
	return fmt.Sprintf("100 * %.2f GWh * 1000 / (%.0f MW * %s * 24 h)", gwh, capMW, dayBasis)
}

// zlfPublishedDecimals is the precision the workbooks publish utilisation to.
// It is what separates a genuine contradiction from a rounding: an implied
// utilisation below half of the last published digit ROUNDS to the published
// 0.00 and is CONSISTENT with it.
const zlfPublishedDecimals = 0.005

// zlfVerdictFor classifies one zero-load-factor cell. Reporting every flagged
// cell as a contradiction would be wrong, and reporting a bare count would
// hide that.
func zlfVerdictFor(implied float64, impliedOK bool) string {
	switch {
	case !impliedOK:
		return zlfUncomputable
	case implied < zlfPublishedDecimals:
		return zlfConsistent
	default:
		return zlfContradiction
	}
}

// daysInFiscalMonth returns the real length of a fiscal month.
//
// Fiscal index 0 is July of the start year and 11 is June of the end year, so
// February's length depends on the year: FY2023-24's February has 29 days. A
// hard-coded 30 would put every implied utilisation out by up to 3.4%.
func daysInFiscalMonth(fy string, idx int) int {
	f, err := nepraparse.ParseFiscalYear(conflictBareFY(fy))
	if err != nil || idx < 0 || idx >= len(nepraparse.MonthsInFiscalOrder) {
		return 0
	}
	y, cm, ok := f.Period(nepraparse.MonthsInFiscalOrder[idx])
	if !ok {
		return 0
	}
	return time.Date(y, cm, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1).Day()
}

// genContentHash hashes the EXTRACTED workbook rows rather than the markup, so
// the hash is comparable across fetches.
func genContentHash(w *nepraparse.Workbook) ([]byte, error) {
	type wire struct {
		SNo                 int
		Name                string
		InstalledCapacity   string
		Status              string
		Months, Utilisation []string
	}
	rows := make([]wire, 0, len(w.Plants))
	for _, p := range w.Plants {
		r := wire{SNo: p.SNo, Name: p.Name, InstalledCapacity: p.InstalledCapacity.Raw(),
			Status: p.Status.String()}
		for _, o := range p.Months {
			r.Months = append(r.Months, o.Generation.Raw())
			r.Utilisation = append(r.Utilisation, o.Utilisation.Raw())
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return json.Marshal(rows)
}
