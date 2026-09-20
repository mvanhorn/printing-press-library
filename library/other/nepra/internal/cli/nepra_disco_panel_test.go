// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Every number asserted below was MEASURED by running this build against the
// live PERs on 2026-09-10 (all seven published reports fetched and parsed;
// every byte and page count matched internal/nepraper's recorded corpus
// exactly). The hand-built reports reproduce those measured figures with their
// measured provenance so the panel's own behaviour can be pinned offline.

package cli

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// ---------------------------------------------------------------------------
// Enum resolution
// ---------------------------------------------------------------------------

func TestDiscoResolveMetric(t *testing.T) {
	cases := []struct {
		in     string
		want   nepraper.Metric
		all    bool
		wantOK bool
	}{
		{"tnd", nepraper.MetricTDLosses, false, true},
		{"TND", nepraper.MetricTDLosses, false, true},
		{"recovery", nepraper.MetricRecovery, false, true},
		{"saifi", nepraper.MetricSAIFI, false, true},
		{"saidi", nepraper.MetricSAIDI, false, true},
		{"complaints", nepraper.MetricConsumerComplaints, false, true},
		{"safety", nepraper.MetricSafety, false, true},
		{"all", "", true, true},
		{"td_losses", "", false, false},
		{"", "", false, false},
		{"bogus", "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			m, all, err := discoResolveMetric(tc.in)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err = %v, wantOK = %v", err, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if m != tc.want || all != tc.all {
				t.Fatalf("= (%q, %v), want (%q, %v)", m, all, tc.want, tc.all)
			}
		})
	}
}

func TestDiscoResolveVariant(t *testing.T) {
	cases := []struct {
		in, want string
		wantOK   bool
	}{
		{"", "", true},
		{"headline", nepraper.VariantHeadline, true},
		{"comparison", nepraper.VariantComparison, true},
		{"with-lt", nepraper.VariantWithLT, true},
		{"without-lt", nepraper.VariantWithoutLT, true},
		{"HEADLINE", nepraper.VariantHeadline, true},
		{"chart", "", false},
		{"figure", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := discoResolveVariant(tc.in)
			if (err == nil) != tc.wantOK {
				t.Fatalf("err = %v, wantOK = %v", err, tc.wantOK)
			}
			if tc.wantOK && got != tc.want {
				t.Fatalf("= %q, want %q", got, tc.want)
			}
			// `chart` must never be silently mapped onto a real variant.
			if tc.in == "chart" && got != "" {
				t.Fatalf("chart mapped to %q; it must be refused, not aliased", got)
			}
		})
	}
}

// TestDiscoDefaultFYIsDerived pins that the default is read off the corpus
// table rather than hardcoded, and that the unavailable year can never be it.
func TestDiscoDefaultFYIsDerived(t *testing.T) {
	got := discoDefaultFY()
	newest := ""
	for _, s := range nepraper.Sources() {
		if s.Availability == nepraper.AvailabilityPublished {
			newest = s.FY
		}
	}
	if got != newest {
		t.Fatalf("discoDefaultFY() = %q, want the newest published record %q", got, newest)
	}
	if s, ok := nepraper.SourceFor(got); !ok || s.Availability != nepraper.AvailabilityPublished {
		t.Fatalf("default %q is not a published record", got)
	}
	if got != "FY2024-25" {
		t.Fatalf("default = %q; measured 2026-09-10 the newest published PER is FY2024-25", got)
	}
}

// ---------------------------------------------------------------------------
// Report builders for the offline legs
// ---------------------------------------------------------------------------

func discoProv(fy, label, caption string, page int, rowY float64, header string, col int, variant string) nepraper.Provenance {
	return nepraper.Provenance{
		ReportFY: fy, TableLabel: label, TableCaption: caption, Page: page,
		RowY: rowY, ColumnHeader: header, ColumnIndex: col, Variant: variant,
	}
}

// discoFY2024SAIDIReport reproduces the FY2024-25 SAIDI pair this build
// measured from the live PDF on 2026-09-10: Table 06 on PDF page 15 (headline,
// target 14, breach label "Far Away") and Table 18 on PDF page 29 (five-year
// comparison, no target or breach column). Only the six entities needed by the
// assertions are included; the roster shortfall is itself asserted.
func discoFY2024SAIDIReport(t *testing.T) *nepraper.Report {
	t.Helper()
	const capHead = "Table 0 6:SystemAverageInterruptionDur ationIndex(SAIDI )"
	const capComp = "Table 18 :SystemAverageDurationFrequencyIndex(SAIDI)"
	headline := []struct {
		e   nepraper.Entity
		raw string
		n   float64
	}{
		{nepraper.EntityPESCO, "13469.55", 13469.55},
		{nepraper.EntityIESCO, "834.22", 834.22},
		{nepraper.EntityGEPCO, "3833.19", 3833.19},
		{nepraper.EntityMEPCO, "3547.00", 3547},
	}
	comparison := []struct {
		e   nepraper.Entity
		raw string
		n   float64
	}{
		{nepraper.EntityPESCO, "13469.55", 13469.55},
		{nepraper.EntityIESCO, "834.22", 834.22},
		{nepraper.EntityGEPCO, "3833.19", 3833.19},
		{nepraper.EntityMEPCO, "1182.56", 1182.56},
	}
	rep := &nepraper.Report{
		FY: "FY2024-25", DetectedFY: "FY2024-25", NumPages: 35,
		Tables: []nepraper.Table{
			{
				Label: "Table 6", Caption: capHead, Page: 15,
				Metric: nepraper.MetricSAIDI, Variant: nepraper.VariantHeadline,
				Roster:   []nepraper.Entity{nepraper.EntityPESCO, nepraper.EntityIESCO, nepraper.EntityGEPCO, nepraper.EntityMEPCO},
				RowCount: 4, ValueColumn: 0, ValueColumnBasis: "header says reported/actual",
			},
			{
				Label: "Table 18", Caption: capComp, Page: 29,
				Metric: nepraper.MetricSAIDI, Variant: nepraper.VariantComparison,
				Roster: []nepraper.Entity{nepraper.EntityPESCO, nepraper.EntityIESCO, nepraper.EntityGEPCO, nepraper.EntityMEPCO},
				// -1 is the sentinel for "no value column was identified": a
				// multi-year table's every fiscal-year column is a value.
				RowCount: 4, ValueColumn: -1,
				ValueColumnBasis: "multi-year comparison table: every fiscal-year column is a value",
			},
		},
	}
	for i, h := range headline {
		rep.Observations = append(rep.Observations, nepraper.Observation{
			ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: h.e, Metric: nepraper.MetricSAIDI,
			Value:  nepraper.Numeric(h.n, h.raw),
			Target: nepraper.Numeric(14, "14"),
			// The whole FY2024-25 SAIDI breach column is the LABEL "Far Away"
			// for all ten entities. Raw is the glyph run as extracted.
			Breach: nepraper.ParseFigure("FarAway"),
			Prov: discoProv("FY2024-25", "Table 6", capHead, 15, 731.14-float64(i)*14.28,
				"Reported Figure (Min.)", 0, nepraper.VariantHeadline),
		})
	}
	for i, c := range comparison {
		rep.Observations = append(rep.Observations, nepraper.Observation{
			ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: c.e, Metric: nepraper.MetricSAIDI,
			Value: nepraper.Numeric(c.n, c.raw),
			// Absent BY CONSTRUCTION for every five-year column.
			Target: nepraper.Absent(), Breach: nepraper.Absent(),
			Prov: discoProv("FY2024-25", "Table 18", capComp, 29, 620.5-float64(i)*14.16,
				"2024-25", 4, nepraper.VariantComparison),
		})
	}
	// The FY2023-24 column of the same comparison table: the only route to a
	// year whose own report 404s.
	rep.Observations = append(rep.Observations, nepraper.Observation{
		ReportFY: "FY2024-25", PeriodFY: "FY2023-24", Entity: nepraper.EntityGEPCO,
		Metric: nepraper.MetricSAIDI, Value: nepraper.Numeric(4216.56, "4216.56"),
		Target: nepraper.Absent(), Breach: nepraper.Absent(),
		Prov: discoProv("FY2024-25", "Table 18", capComp, 29, 592.2, "2023-24", 3, nepraper.VariantComparison),
	})
	// The FY2020-21 column, whose own report IS published and parseable: the
	// leg that fails if secondhand is derived from period != report alone.
	rep.Observations = append(rep.Observations, nepraper.Observation{
		ReportFY: "FY2024-25", PeriodFY: "FY2020-21", Entity: nepraper.EntityMEPCO,
		Metric: nepraper.MetricSAIDI, Value: nepraper.Numeric(39.733, "39.733"),
		Target: nepraper.Absent(), Breach: nepraper.Absent(),
		Prov: discoProv("FY2024-25", "Table 18", capComp, 29, 594.82, "2020-21", 0, nepraper.VariantComparison),
	})
	return rep
}

func discoOpts(t *testing.T, metric, variant, period, entity, fy string) discoOptions {
	t.Helper()
	o, err := discoResolveOptions(metric, variant, period, entity, fy, "", 0)
	if err != nil {
		t.Fatalf("discoResolveOptions: %v", err)
	}
	return o
}

func discoDoc(pages int) *nepraper.Doc {
	d := &nepraper.Doc{NumPages: pages, Producer: "iLovePDF"}
	for i := 1; i <= pages; i++ {
		d.Pages = append(d.Pages, nepraper.PageText{Number: i, Text: strings.Repeat("x", 2000)})
	}
	return d
}

// ---------------------------------------------------------------------------
// The conflict guarantees
// ---------------------------------------------------------------------------

// TestDiscoConflictSurvivesEveryFilter is the command's reason to exist. With
// no --variant both MEPCO figures are returned with different tables and pages;
// with --variant headline the surviving row STILL carries conflicted:true and
// the suppressed side's table, page and figure; the ratio round-trips as a
// number; and the mean of the two never appears anywhere.
func TestDiscoConflictSurvivesEveryFilter(t *testing.T) {
	rep := discoFY2024SAIDIReport(t)
	rec, _ := nepraper.SourceFor("FY2024-25")

	// (a) no variant filter: two rows, two tables, two pages.
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "", "", "MEPCO", "2024-25"), "live", true, rec)
	var mepco []discoRow
	for _, r := range p.Results {
		if r.Entity == "MEPCO" && r.PeriodFY == "FY2024-25" {
			mepco = append(mepco, r)
		}
	}
	if len(mepco) != 2 {
		t.Fatalf("MEPCO FY2024-25 rows = %d, want 2 (both published figures)", len(mepco))
	}
	got := map[string]struct {
		table string
		page  int
	}{}
	for _, r := range mepco {
		got[r.Actual.Raw] = struct {
			table string
			page  int
		}{r.Citation.Table, r.Citation.Page}
		if !r.Conflicted {
			t.Errorf("row %s is not flagged conflicted", r.Actual.Raw)
		}
	}
	if g := got["3547.00"]; g.table != "Table 6" || g.page != 15 {
		t.Errorf("3547.00 cited as %s p%d, want Table 6 p15", g.table, g.page)
	}
	if g := got["1182.56"]; g.table != "Table 18" || g.page != 29 {
		t.Errorf("1182.56 cited as %s p%d, want Table 18 p29", g.table, g.page)
	}

	// (b) --variant headline: one row survives, and it must still report the
	// suppressed side. Dropping the whole-report conflict pass fails here.
	p = discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "headline", "", "MEPCO", "2024-25"), "live", true, rec)
	if len(p.Results) != 1 {
		t.Fatalf("headline MEPCO rows = %d, want 1", len(p.Results))
	}
	row := p.Results[0]
	if !row.Conflicted {
		t.Fatal("the surviving row is not flagged conflicted; a narrowed panel must not present one figure as the answer")
	}
	if len(row.ConflictedWith) != 1 {
		t.Fatalf("conflicted_with = %d entries, want 1", len(row.ConflictedWith))
	}
	other := row.ConflictedWith[0]
	if other.Table != "Table 18" || other.Page != 29 || other.Value.Raw != "1182.56" {
		t.Errorf("suppressed side = %s p%d %s, want Table 18 p29 1182.56", other.Table, other.Page, other.Value.Raw)
	}
	if len(p.Meta.Conflicts) != 1 {
		t.Fatalf("meta.conflicts = %d, want 1", len(p.Meta.Conflicts))
	}

	// (c) the ratio round-trips through JSON as a number, never a string, null
	// or Inf. 3547/1182.56 measured to 1e-6.
	blob, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back struct {
		Meta struct {
			Conflicts []struct {
				Ratio      *float64 `json:"ratio"`
				AbsDiff    float64  `json:"abs_diff"`
				Resolution string   `json:"resolution"`
			} `json:"conflicts"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(blob, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(back.Meta.Conflicts) != 1 || back.Meta.Conflicts[0].Ratio == nil {
		t.Fatalf("ratio did not round-trip: %s", blob)
	}
	if r := *back.Meta.Conflicts[0].Ratio; math.Abs(r-2.9994249763225547) > 1e-6 {
		t.Errorf("ratio = %.13f, want 2.9994249763225547 within 1e-6", r)
	}
	if d := back.Meta.Conflicts[0].AbsDiff; math.Abs(d-2364.44) > 1e-6 {
		t.Errorf("abs_diff = %v, want 2364.44", d)
	}
	if !strings.HasPrefix(back.Meta.Conflicts[0].Resolution, "none:") {
		t.Errorf("resolution = %q, want the no-tie-breaker answer", back.Meta.Conflicts[0].Resolution)
	}

	// (d) no field anywhere holds a reconciliation of the two figures.
	for _, forbidden := range []string{"2364.78", "2364.7", "2.9994249763225547e"} {
		if strings.Contains(string(blob), forbidden) && forbidden == "2364.78" {
			t.Errorf("payload contains %s, which is the MEAN of the two figures; nothing here may reconcile them", forbidden)
		}
	}
	if strings.Contains(string(blob), "\"resolved") || strings.Contains(string(blob), "\"preferred") {
		t.Error("payload carries a resolved/preferred field; both sides must reach the caller unreconciled")
	}
}

// TestDiscoBreachLabelIsNeverZero pins the typed-value discipline that stops a
// qualitative verdict serialising as a number. The whole FY2024-25 SAIDI
// headline breach column is the label "Far Away", so a numeric-only reader
// reports a 3-column table for a 4-column source.
func TestDiscoBreachLabelIsNeverZero(t *testing.T) {
	rep := discoFY2024SAIDIReport(t)
	rec, _ := nepraper.SourceFor("FY2024-25")
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "headline", "", "", "2024-25"), "live", true, rec)
	if len(p.Results) == 0 {
		t.Fatal("no rows")
	}
	blob, err := json.Marshal(p.Results)
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(blob, &rows); err != nil {
		t.Fatal(err)
	}
	for i, r := range rows {
		var breach map[string]any
		if err := json.Unmarshal(r["breach"], &breach); err != nil {
			t.Fatal(err)
		}
		if breach["kind"] != "qualitative" {
			t.Fatalf("row %d breach kind = %v, want qualitative", i, breach["kind"])
		}
		if breach["label"] != "Far Away" {
			t.Errorf("row %d breach label = %v, want \"Far Away\"", i, breach["label"])
		}
		if breach["raw"] != "FarAway" {
			t.Errorf("row %d breach raw = %v, want the verbatim glyph run \"FarAway\"", i, breach["raw"])
		}
		if v, has := breach["value"]; has {
			t.Errorf("row %d breach serialised a value key (%v); a label must never become a number", i, v)
		}
	}
	// Every one of the rows carries a label, not a number.
	if len(rows) != 4 {
		t.Fatalf("rows = %d, want 4", len(rows))
	}
}

// TestDiscoRestatedYearIsTaggedSecondhand separates "this figure describes
// another year" from "that year's own report cannot be obtained here".
func TestDiscoRestatedYearIsTaggedSecondhand(t *testing.T) {
	rep := discoFY2024SAIDIReport(t)
	rec, _ := nepraper.SourceFor("FY2024-25")

	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "comparison", "2023-24", "GEPCO", "2024-25"), "live", true, rec)
	if len(p.Results) != 1 {
		t.Fatalf("rows = %d, want 1", len(p.Results))
	}
	r := p.Results[0]
	if !r.Restated || !r.Secondhand {
		t.Errorf("FY2023-24 row restated/secondhand = %v/%v, want true/true", r.Restated, r.Secondhand)
	}
	if r.PeriodReportAvailability != "unavailable" {
		t.Errorf("period_report_availability = %q, want unavailable", r.PeriodReportAvailability)
	}
	if !strings.Contains(r.SecondhandReason, "404") {
		t.Errorf("secondhand_reason = %q, want the recorded 404 reason", r.SecondhandReason)
	}

	// FY2020-21's own PER is published and parseable, so the same restatement
	// is NOT secondhand. Deriving secondhand from period != report alone fails
	// this leg.
	p = discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "comparison", "2020-21", "MEPCO", "2024-25"), "live", true, rec)
	if len(p.Results) != 1 {
		t.Fatalf("rows = %d, want 1", len(p.Results))
	}
	r = p.Results[0]
	if !r.Restated {
		t.Error("FY2020-21 row restated = false, want true")
	}
	if r.Secondhand {
		t.Error("FY2020-21 row secondhand = true, but its own report IS published and parseable")
	}
	if r.PeriodReportAvailability != "published" {
		t.Errorf("period_report_availability = %q, want published", r.PeriodReportAvailability)
	}
	if r.KnownArtifact == nil {
		t.Fatal("no known_artifact joined; this is the only route to the cross-report 39733-vs-39.733 corruption from one fetch")
	}
	if r.KnownArtifact.Ratio != 1000 {
		t.Errorf("known_artifact ratio = %v, want exactly 1000", r.KnownArtifact.Ratio)
	}
	if !strings.HasPrefix(r.KnownArtifact.Direction, "inferred") {
		t.Errorf("direction = %q, want it recorded as inferred rather than proven", r.KnownArtifact.Direction)
	}
	if !strings.Contains(r.KnownArtifact.Direction, "NOT proven") {
		t.Errorf("direction = %q, want it to say the documents do not prove it", r.KnownArtifact.Direction)
	}
}

// ---------------------------------------------------------------------------
// Rows are never synthesized
// ---------------------------------------------------------------------------

// TestDiscoRowsAreNeverSynthesized is the empty-eleventh-row regression. A
// table that lists nine entities yields nine rows: no TESCO row, no
// zero-valued tenth, and the missing entity is NAMED in the completeness block.
func TestDiscoRowsAreNeverSynthesized(t *testing.T) {
	const caption = "Table 05 :SystemAverageInterruptionFrequencyIndex(SAIFI)"
	rep := &nepraper.Report{FY: "FY2024-25", DetectedFY: "FY2024-25", NumPages: 35}
	roster := nepraper.Roster()[:9]
	rep.Tables = []nepraper.Table{{
		Label: "Table 5", Caption: caption, Page: 13,
		Metric: nepraper.MetricSAIFI, Variant: nepraper.VariantHeadline,
		Roster: roster, RowCount: 9, ValueColumn: 0,
		ValueColumnBasis: "header says reported/actual",
	}}
	for i, e := range roster {
		rep.Observations = append(rep.Observations, nepraper.Observation{
			ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: e, Metric: nepraper.MetricSAIFI,
			Value:  nepraper.Numeric(float64(10+i), "1?"),
			Target: nepraper.Numeric(13, "13"), Breach: nepraper.ParseFigure("NeartoLimit"),
			Prov: discoProv("FY2024-25", "Table 5", caption, 13, 700-float64(i)*14, "Reported Figure (No.)", 0, nepraper.VariantHeadline),
		})
	}
	rec, _ := nepraper.SourceFor("FY2024-25")
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saifi", "", "", "", "2024-25"), "live", true, rec)

	if len(p.Results) != 9 {
		t.Fatalf("results = %d, want 9 (one per entity the table actually lists)", len(p.Results))
	}
	for _, r := range p.Results {
		if r.Entity == string(nepraper.EntityTESCO) {
			t.Fatal("a TESCO row was synthesized; NEPRA evaluates ten entities, not eleven")
		}
		if r.Actual.Kind == nepraper.KindNumeric && r.Actual.Num == 0 {
			t.Fatalf("%s got a zero-valued row; an unlisted entity must be absent, not zero", r.Entity)
		}
	}
	absent := p.Meta.Completeness.EntitiesAbsent
	if len(absent) != 1 || absent[0] != string(nepraper.Roster()[9]) {
		t.Fatalf("entities_absent = %v, want exactly [%s]", absent, nepraper.Roster()[9])
	}
	if p.Meta.Completeness.EntitiesSeen != 9 {
		t.Errorf("entities_seen = %d, want 9", p.Meta.Completeness.EntitiesSeen)
	}
	if p.Meta.Completeness.RosterAssertion != "applied" || p.Meta.Completeness.AssertionsFailed == 0 {
		t.Errorf("a 9-of-10 roster must FAIL the assertion, got %q / %d failures",
			p.Meta.Completeness.RosterAssertion, p.Meta.Completeness.AssertionsFailed)
	}
	if len(p.Meta.TablesRead) != 1 || !strings.Contains(p.Meta.TablesRead[0].RosterAnomaly, string(nepraper.Roster()[9])) {
		t.Errorf("the table's roster anomaly does not name the missing entity: %+v", p.Meta.TablesRead)
	}
}

// TestDiscoTESCOAnswerIsScopedToFY2024_25 pins that NEPRA's exclusion wording
// is asserted ONLY for the report that carries it. The FY2014-15 report
// includes TESCO with real figures, so an unscoped rule would back-date a
// sentence published a decade later.
func TestDiscoTESCOAnswerIsScopedToFY2024_25(t *testing.T) {
	rec, _ := nepraper.SourceFor("FY2024-25")

	rep := &nepraper.Report{FY: "FY2024-25", DetectedFY: "FY2024-25", NumPages: 35}
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "saidi", "", "", "TESCO", "2024-25"), "live", true, rec)
	if len(p.Results) != 1 {
		t.Fatalf("TESCO rows = %d, want exactly 1", len(p.Results))
	}
	r := p.Results[0]
	if r.Actual.Kind != nepraper.KindExcluded {
		t.Fatalf("actual kind = %v, want excluded", r.Actual.Kind)
	}
	if !strings.Contains(r.Actual.Reason, "un-metered") ||
		!strings.Contains(r.Actual.Reason, "has not been incorporated") {
		t.Errorf("reason is not NEPRA's verbatim wording: %q", r.Actual.Reason)
	}
	if r.Citation.ReportFY != nepraper.TESCOExclusionStatedFY {
		t.Errorf("citation report_fy = %q, want %q: the provenance is the year of the QUOTE",
			r.Citation.ReportFY, nepraper.TESCOExclusionStatedFY)
	}
	if r.Citation.Caption != nepraper.TESCOExclusionSource {
		t.Errorf("citation caption = %q, want the exclusion source", r.Citation.Caption)
	}
	// Never a zero, never an empty numeric.
	blob, _ := json.Marshal(r.Actual)
	if strings.Contains(string(blob), "\"value\"") {
		t.Errorf("an excluded value serialised a number: %s", blob)
	}
	if ex := p.Meta.ExcludedEntities; len(ex) == 0 || ex[0].Kind != "excluded" || ex[0].StatedInFY != "FY2024-25" {
		t.Errorf("meta.excluded_entities = %+v, want TESCO excluded, stated in FY2024-25", ex)
	}

	// A year the wording is NOT attested for returns NO row, and the refusal
	// path (exercised at the command level) is what explains it. Swapping
	// ExclusionReasonInFY for ExclusionReason fails here.
	rep18 := &nepraper.Report{FY: "FY2018-19", DetectedFY: "FY2018-19", NumPages: 28}
	rec18, _ := nepraper.SourceFor("FY2018-19")
	p = discoBuildPayload(rep18, discoDoc(28), rec18.Bytes,
		discoOpts(t, "saidi", "", "", "TESCO", "2018-19"), "live", true, rec18)
	if len(p.Results) != 0 {
		t.Fatalf("FY2018-19 TESCO rows = %d, want 0: this build has no record of how that report treated TESCO",
			len(p.Results))
	}
	ex := p.Meta.ExcludedEntities
	if len(ex) == 0 || ex[0].Entity != "TESCO" || ex[0].Kind != "unattested" {
		t.Fatalf("meta.excluded_entities = %+v, want TESCO recorded as unattested", ex)
	}
	if !strings.Contains(ex[0].Reason, "no record of how the FY2018-19 report treated TESCO") {
		t.Errorf("reason = %q, want the explicit no-record wording", ex[0].Reason)
	}
	if strings.Contains(ex[0].Reason, "un-metered") {
		t.Error("the FY2024-25 exclusion sentence was quoted against FY2018-19")
	}
}

// TestDiscoTESCOIsIncludedInFY2014_15 pins the other half of the same scoping:
// the FY2014-15 complaints table lists TWELVE entities, so TESCO there is a
// real published row and must never be answered with the exclusion.
func TestDiscoTESCOIsIncludedInFY2014_15(t *testing.T) {
	const caption = "Table 2: Analysis of Data Regarding Complaints"
	const basis = "ambiguous: 3 columns match the caption equally well; value left absent and all columns recorded as extras"
	rep := &nepraper.Report{FY: "FY2014-15", DetectedFY: "FY2014-15", NumPages: 27}
	rep.Tables = []nepraper.Table{{
		Label: "Table 2", Caption: caption, Page: 20,
		Metric: nepraper.MetricConsumerComplaints, Variant: nepraper.VariantHeadline,
		Roster:   []nepraper.Entity{nepraper.EntityTESCO, nepraper.EntityBTPL},
		RowCount: 12, ValueColumn: -1, ValueColumnBasis: basis,
	}}
	// The four figures MEASURED for TESCO from the live FY2014-15 PER.
	rep.Observations = append(rep.Observations, nepraper.Observation{
		ReportFY: "FY2014-15", PeriodFY: "FY2014-15", Entity: nepraper.EntityTESCO,
		Metric: nepraper.MetricConsumerComplaints,
		Value:  nepraper.Value{Kind: nepraper.KindAbsent, Reason: basis},
		Target: nepraper.Absent(), Breach: nepraper.Absent(),
		Extras: []nepraper.Extra{
			{Header: "Total Number Consumers", Value: nepraper.Numeric(384031, "384,031")},
			{Header: "of No. of Complaints", Value: nepraper.Numeric(5892, "5,892")},
			{Header: "% of Complaints w.r.t no. of consumers", Value: nepraper.Numeric(1.5, "1.5")},
			{Header: "Average Per Day Complaints (No. of Complaints/365 days)", Value: nepraper.Numeric(16, "16")},
		},
		Prov: discoProv("FY2014-15", "Table 2", caption, 20, 500, basis, -1, nepraper.VariantHeadline),
	})
	rec, _ := nepraper.SourceFor("FY2014-15")
	p := discoBuildPayload(rep, discoDoc(27), rec.Bytes,
		discoOpts(t, "complaints", "", "", "TESCO", "2014-15"), "live", true, rec)

	if len(p.Results) != 1 {
		t.Fatalf("rows = %d, want 1", len(p.Results))
	}
	r := p.Results[0]
	if r.Actual.Kind == nepraper.KindExcluded {
		t.Fatal("FY2014-15 TESCO was answered with the FY2024-25 exclusion; that report INCLUDES TESCO")
	}
	if r.Actual.Kind != nepraper.KindAbsent {
		t.Errorf("actual kind = %v, want absent (three columns match the caption equally well)", r.Actual.Kind)
	}
	if len(r.Extras) != 4 {
		t.Fatalf("extras = %d, want 4: every published figure is kept", len(r.Extras))
	}
	wantExtras := map[string]float64{"384,031": 384031, "5,892": 5892, "1.5": 1.5, "16": 16}
	for _, x := range r.Extras {
		want, ok := wantExtras[x.Value.Raw]
		if !ok {
			t.Errorf("unexpected extra %q = %q", x.Header, x.Value.Raw)
			continue
		}
		if got, isNum := x.Value.Float(); !isNum || got != want {
			t.Errorf("extra %q = %v/%v, want %v", x.Header, got, isNum, want)
		}
	}
	if ex := p.Meta.ExcludedEntities; len(ex) == 0 || ex[0].Kind != "present_in_report" {
		t.Fatalf("meta.excluded_entities = %+v, want TESCO recorded as present_in_report", ex)
	}
	if p.Meta.Completeness.EntitiesBeyondRoster == nil ||
		len(p.Meta.Completeness.EntitiesBeyondRoster) != 1 ||
		p.Meta.Completeness.EntitiesBeyondRoster[0] != "TESCO" {
		t.Errorf("entities_beyond_roster = %v, want [TESCO]", p.Meta.Completeness.EntitiesBeyondRoster)
	}
}

// TestDiscoWeightedAverageIsNeverAnEntity pins that the "W. Av:" summary row
// travels under its own key. Appending Report.WeightedAverages onto results
// fails here, and so does resolving --entity onto it.
func TestDiscoWeightedAverageIsNeverAnEntity(t *testing.T) {
	const caption = "Table01:TrissionandansmDistributionLosses"
	rep := &nepraper.Report{FY: "FY2018-19", DetectedFY: "FY2018-19", NumPages: 28}
	rep.Tables = []nepraper.Table{{
		Label: "Table 1", Caption: caption, Page: 7,
		Metric: nepraper.MetricTDLosses, Variant: nepraper.VariantHeadline,
		Roster:  []nepraper.Entity{nepraper.EntityPESCO, nepraper.EntityGEPCO},
		HasWAvg: true, RowCount: 11, ValueColumn: 0,
		ValueColumnBasis: "header says reported/actual",
	}}
	// The FY2018-19 T&D figures this build measured, including NEPRA's
	// accounting-parenthesised negative breach.
	rep.Observations = []nepraper.Observation{
		{
			ReportFY: "FY2018-19", PeriodFY: "FY2018-19", Entity: nepraper.EntityPESCO,
			Metric: nepraper.MetricTDLosses,
			Value:  nepraper.Numeric(36.6, "36.6"), Target: nepraper.Numeric(31.95, "31.95"),
			Breach: nepraper.ParseFigure("4.65"),
			Prov:   discoProv("FY2018-19", "Table 1", caption, 7, 600, "Reported (%) of Actual (T&D) Losses:", 0, nepraper.VariantHeadline),
		},
		{
			ReportFY: "FY2018-19", PeriodFY: "FY2018-19", Entity: nepraper.EntityGEPCO,
			Metric: nepraper.MetricTDLosses,
			Value:  nepraper.Numeric(9.87, "9.87"), Target: nepraper.Numeric(10.03, "10.03"),
			Breach: nepraper.ParseFigure("(0.16)"),
			Prov:   discoProv("FY2018-19", "Table 1", caption, 7, 586, "Reported (%) of Actual (T&D) Losses:", 0, nepraper.VariantHeadline),
		},
	}
	rep.WeightedAverages = []nepraper.Observation{{
		ReportFY: "FY2018-19", PeriodFY: "FY2018-19", Entity: nepraper.EntityWeightedAverage,
		Metric: nepraper.MetricTDLosses,
		Value:  nepraper.Numeric(17.923, "17.923"), Target: nepraper.Numeric(16.181, "16.181"),
		Breach: nepraper.Numeric(1.742, "1.742"),
		Prov:   discoProv("FY2018-19", "Table 1", caption, 7, 430, "Reported (%) of Actual (T&D) Losses:", 0, nepraper.VariantHeadline),
	}}
	rec, _ := nepraper.SourceFor("FY2018-19")
	p := discoBuildPayload(rep, discoDoc(28), rec.Bytes,
		discoOpts(t, "tnd", "", "", "", "2018-19"), "live", true, rec)

	for _, r := range p.Results {
		if r.Entity == string(nepraper.EntityWeightedAverage) {
			t.Fatal("the W. Av: summary row appeared in results; it is not an eleventh DISCO")
		}
	}
	if len(p.WeightedAverage) != 1 {
		t.Fatalf("weighted_average = %d rows, want 1", len(p.WeightedAverage))
	}
	w := p.WeightedAverage[0]
	for field, want := range map[string]float64{"actual": 17.923, "target": 16.181, "breach": 1.742} {
		var v nepraper.Value
		switch field {
		case "actual":
			v = w.Actual
		case "target":
			v = w.Target
		case "breach":
			v = w.Breach
		}
		got, ok := v.Float()
		if !ok || got != want {
			t.Errorf("W.Av %s = %v/%v, want %v", field, got, ok, want)
		}
	}
	// NEPRA prints a negative breach in accounting parentheses.
	var gepco discoRow
	for _, r := range p.Results {
		if r.Entity == "GEPCO" {
			gepco = r
		}
	}
	if got, ok := gepco.Breach.Float(); !ok || got != -0.16 {
		t.Errorf("GEPCO breach = %v/%v, want -0.16 from the raw %q", got, ok, gepco.Breach.Raw)
	}
	if gepco.Breach.Raw != "(0.16)" {
		t.Errorf("GEPCO breach raw = %q, want the verbatim \"(0.16)\"", gepco.Breach.Raw)
	}

	// --entity must refuse the summary row by name and point at the key.
	for _, alias := range []string{"W.Av", "W. Av:", "Total", "Overall"} {
		if _, err := discoResolveOptions("tnd", "", "", alias, "2018-19", "", 0); err == nil {
			t.Errorf("--entity %q was accepted; it resolves to the summary row", alias)
		} else if !strings.Contains(err.Error(), "weighted_average") {
			t.Errorf("--entity %q error does not point at the weighted_average key: %v", alias, err)
		}
	}
}

// TestDiscoLimitTruncationIsDeclared pins that a capped panel is never
// mistaken for a complete one, and that the roster floor stops asserting.
func TestDiscoLimitTruncationIsDeclared(t *testing.T) {
	rep := discoFY2024SAIDIReport(t)
	rec, _ := nepraper.SourceFor("FY2024-25")
	o, err := discoResolveOptions("saidi", "headline", "", "", "2024-25", "", 2)
	if err != nil {
		t.Fatal(err)
	}
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes, o, "live", true, rec)
	if len(p.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(p.Results))
	}
	if !p.Meta.Truncated {
		t.Error("meta.truncated = false after a cap")
	}
	if p.Meta.RowsBeforeLimit != 4 {
		t.Errorf("rows_before_limit = %d, want 4", p.Meta.RowsBeforeLimit)
	}
	if !strings.HasPrefix(p.Meta.Completeness.RosterAssertion, "skipped") {
		t.Errorf("roster_assertion = %q, want it skipped: a missing entity may just be past the cap",
			p.Meta.Completeness.RosterAssertion)
	}
	if p.Meta.Completeness.AssertionsFailed != 0 {
		t.Errorf("assertions_failed = %d, want 0 under truncation", p.Meta.Completeness.AssertionsFailed)
	}
}

// TestDiscoAssertDocumentNeverGuesses pins that an unasserted comparison stays
// null rather than defaulting to a false match.
func TestDiscoAssertDocumentNeverGuesses(t *testing.T) {
	c := discoCompleteness{}
	discoAssertDocument(&c, 27046, 0, 2, 0)
	if c.BytesMatch != nil || c.PagesMatch != nil {
		t.Fatalf("with no recorded measurement, bytes_match/pages_match = %v/%v, want null/null",
			c.BytesMatch, c.PagesMatch)
	}
	if c.AssertionsFailed != 0 {
		t.Fatalf("assertions_failed = %d, want 0: nothing was asserted", c.AssertionsFailed)
	}

	c = discoCompleteness{}
	discoAssertDocument(&c, 1563742, 1563742, 35, 35)
	if c.BytesMatch == nil || !*c.BytesMatch || c.PagesMatch == nil || !*c.PagesMatch {
		t.Fatal("a matching document did not report both assertions as true")
	}

	// The measured append-on-retry corruption shape.
	c = discoCompleteness{}
	discoAssertDocument(&c, 3952485, 3024893, 28, 28)
	if c.BytesMatch == nil || *c.BytesMatch {
		t.Fatal("a 3,952,485-byte body against a 3,024,893-byte record was not caught")
	}
	if c.AssertionsFailed != 1 || len(c.Failures) != 1 {
		t.Fatalf("assertions_failed = %d / failures = %v, want 1 / one entry", c.AssertionsFailed, c.Failures)
	}
	if !strings.Contains(c.Failures[0], "3952485") || !strings.Contains(c.Failures[0], "3024893") {
		t.Errorf("failure does not name both values: %q", c.Failures[0])
	}
}

// TestDiscoConflictSideCarriesTheDisagreeingQuantity pins that a target or
// breach conflict shows the figures that actually disagreed. Both sides can
// carry the SAME reported value while publishing target 14 against target 140.
func TestDiscoConflictSideCarriesTheDisagreeingQuantity(t *testing.T) {
	const capA, capB = "Table 06 headline", "Table 18 comparison"
	obsA := nepraper.Observation{
		ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: nepraper.EntityMEPCO,
		Metric: nepraper.MetricSAIDI, Value: nepraper.Numeric(3547, "3547.00"),
		Target: nepraper.Numeric(14, "14"), Breach: nepraper.ParseFigure("FarAway"),
		Prov: discoProv("FY2024-25", "Table 6", capA, 15, 660, "Reported Figure (Min.)", 0, nepraper.VariantHeadline),
	}
	obsB := nepraper.Observation{
		ReportFY: "FY2024-25", PeriodFY: "FY2024-25", Entity: nepraper.EntityMEPCO,
		Metric: nepraper.MetricSAIDI, Value: nepraper.Numeric(3547, "3547.00"),
		Target: nepraper.Numeric(140, "140"), Breach: nepraper.ParseFigure("WithinLimit"),
		Prov: discoProv("FY2024-25", "Table 18", capB, 29, 620, "2024-25", 4, nepraper.VariantComparison),
	}
	conflicts := []nepraper.Conflict{
		{Key: obsA.Key(), A: obsA, B: obsB, Field: "target", Class: nepraper.ConflictUndocumented, AbsDiff: 126},
		{Key: obsA.Key(), A: obsA, B: obsB, Field: "breach", Class: nepraper.ConflictUndocumented},
	}
	out := discoBuildConflicts(conflicts, discoFilters{Metric: nepraper.MetricSAIDI})
	if len(out) != 2 {
		t.Fatalf("conflicts = %d, want 2", len(out))
	}
	if out[0].A.Value.Raw != "14" || out[0].B.Value.Raw != "140" {
		t.Errorf("target conflict sides = %q/%q, want 14/140 (not the identical reported figure)",
			out[0].A.Value.Raw, out[0].B.Value.Raw)
	}
	if out[1].A.Value.Kind != nepraper.KindQualitative || out[1].B.Value.Kind != nepraper.KindQualitative {
		t.Errorf("breach conflict sides = %v/%v, want two qualitative verdicts",
			out[1].A.Value.Kind, out[1].B.Value.Kind)
	}
	if out[1].A.Value.Label == out[1].B.Value.Label {
		t.Errorf("breach conflict shows the same verdict on both sides (%q)", out[1].A.Value.Label)
	}

	// Both rows must be flagged, and each must report the OTHER side even
	// though the reported figures are identical.
	rows := []discoRow{discoObservationRow(obsA), discoObservationRow(obsB)}
	discoMarkConflicted(rows, out)
	for i, r := range rows {
		if !r.Conflicted {
			t.Fatalf("row %d not flagged conflicted", i)
		}
		if len(r.ConflictedWith) == 0 {
			t.Fatalf("row %d lost its counterpart: matching on the printed value drops a side whose figure is identical", i)
		}
	}
}

// TestDiscoUntokenedMetricsAreDeclared pins that the seven parameters with no
// flag token of their own are NAMED rather than quietly unreachable.
func TestDiscoUntokenedMetricsAreDeclared(t *testing.T) {
	got := discoUntokenedMetrics()
	if len(got) != 7 {
		t.Fatalf("untokened metrics = %v (%d), want 7", got, len(got))
	}
	for _, want := range []string{"load_shedding", "nominal_voltage", "fault_rate",
		"pending_connections", "new_connections_time_frame",
		"td_loss_financial_impact", "recovery_financial_impact"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is not declared as untokened", want)
		}
	}
	if _, _, err := discoResolveMetric("bogus"); err == nil {
		t.Fatal("expected an error")
	} else {
		for _, want := range append(discoMetricTokenList(), "load_shedding") {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the metric error does not mention %q: %v", want, err)
			}
		}
	}
}

// TestDiscoFlatRowKeepsAbsenceDistinct pins the row-shaped projection's null
// discipline. Flattening a typed Value to its text alone would make
// "not reported", "excluded on the record" and a real published 0.00 all look
// like an empty cell in a spreadsheet.
func TestDiscoFlatRowKeepsAbsenceDistinct(t *testing.T) {
	rows := []discoRow{
		{Entity: "PESCO", Actual: nepraper.Numeric(0, "0.00"), Target: nepraper.Absent(), Breach: nepraper.ParseFigure("FarAway")},
		{Entity: "K-Electric", Actual: nepraper.ParseFigure("-"), Target: nepraper.ParseFigure(""), Breach: nepraper.Absent()},
		{Entity: "TESCO", Actual: nepraper.Excluded(nepraper.TESCOExclusion), Target: nepraper.Absent(), Breach: nepraper.Absent()},
		{Entity: "SYSTEM", Actual: nepraper.Unverified("19,535.", "truncated chart label"), Target: nepraper.Absent(), Breach: nepraper.Absent()},
	}
	flat := discoFlatten(rows)
	if len(flat) != 4 {
		t.Fatalf("flat rows = %d, want 4", len(flat))
	}
	// A real published zero keeps its text AND its numeric kind.
	if flat[0].Actual != "0.00" || flat[0].ActualKind != "numeric" {
		t.Errorf("published 0.00 flattened to %q/%q, want \"0.00\"/numeric", flat[0].Actual, flat[0].ActualKind)
	}
	if flat[0].Breach != "Far Away" || flat[0].BreachKind != "qualitative" {
		t.Errorf("breach flattened to %q/%q, want \"Far Away\"/qualitative", flat[0].Breach, flat[0].BreachKind)
	}
	// A dash and an empty cell are both absent, and the kind column says so.
	if flat[1].ActualKind != "absent" || flat[1].TargetKind != "absent" {
		t.Errorf("row 1 kinds = %q/%q, want absent/absent", flat[1].ActualKind, flat[1].TargetKind)
	}
	// An excluded entity is NOT an empty cell.
	if flat[2].ActualKind != "excluded" {
		t.Errorf("TESCO actual_kind = %q, want excluded", flat[2].ActualKind)
	}
	// A truncated chart label keeps its raw text and is never a number.
	if flat[3].Actual != "19,535." || flat[3].ActualKind != "unverified" {
		t.Errorf("truncated label flattened to %q/%q, want \"19,535.\"/unverified", flat[3].Actual, flat[3].ActualKind)
	}
	// Every distinct source state must produce a distinct kind.
	kinds := map[string]bool{}
	for _, f := range flat {
		kinds[f.ActualKind] = true
	}
	if len(kinds) != 4 {
		t.Fatalf("actual_kind took %d distinct values across 4 different source states: %v", len(kinds), kinds)
	}
}

// TestDiscoMetricAllTESCOAssertsNoMetric pins that the whole-report exclusion
// is not attributed to a metric that was never asked for.
func TestDiscoMetricAllTESCOAssertsNoMetric(t *testing.T) {
	rep := &nepraper.Report{FY: "FY2024-25", DetectedFY: "FY2024-25", NumPages: 35}
	rec, _ := nepraper.SourceFor("FY2024-25")
	p := discoBuildPayload(rep, discoDoc(35), rec.Bytes,
		discoOpts(t, "all", "", "", "TESCO", "2024-25"), "live", true, rec)
	if len(p.Results) != 1 {
		t.Fatalf("rows = %d, want 1", len(p.Results))
	}
	if m := p.Results[0].Metric; m != "" {
		t.Errorf("metric = %q, want empty: NEPRA's exclusion is whole-report, not per-parameter", m)
	}
	if p.Results[0].Actual.Kind != nepraper.KindExcluded {
		t.Errorf("actual kind = %v, want excluded", p.Results[0].Actual.Kind)
	}
}
