// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// Rendering and the no-request catalogue for `fleet`.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// ---------------------------------------------------------------------------
// Meta
// ---------------------------------------------------------------------------

func fleetBuildMeta() fleetMeta {
	rows := nepraxwalk.Rows()
	obs := 0
	for _, r := range rows {
		obs += len(r.Observed)
	}
	m := fleetMeta{
		Source: "computed",
		Basis: fmt.Sprintf("embedded crosswalk internal/nepraxwalk/crosswalk.json (%d rows, %d observations); no request made",
			len(rows), obs),
		Crosswalk: fleetCrosswalkMeta{
			Rows:             len(rows),
			Observations:     obs,
			ObservedFYs:      nepraxwalk.ObservedFYs(),
			HeldOutFYs:       nepraxwalk.HeldOutFYs(),
			LatestObservedFY: nepraxwalk.LatestObservedFY(),
			PublishedFYs:     nepraFleetPublishedFYs,
			SNoNote:          nepraFleetSNoNote,
		},
	}
	if err := nepraxwalk.Validate(); err != nil {
		m.Crosswalk.SchemaValid = false
		m.Crosswalk.SchemaError = err.Error()
	} else {
		m.Crosswalk.SchemaValid = true
	}
	return m
}

// ---------------------------------------------------------------------------
// Main render
// ---------------------------------------------------------------------------

func fleetRender(cmd *cobra.Command, flags *rootFlags, res fleetResults, strict bool) error {
	// Failed assertions go to stderr in BOTH modes, always.
	var failed []fleetAssertion
	for _, a := range res.Assertions {
		if !a.OK {
			failed = append(failed, a)
			fmt.Fprintf(cmd.ErrOrStderr(), "ASSERTION %s FAILED: %s\n", a.Name, a.Detail)
		}
	}
	for _, col := range res.FiscalYears {
		if col.CoverageClass != fleetCoverageObserved {
			fmt.Fprintf(cmd.ErrOrStderr(), "UNMEASURED: FY%s is %s — every capacity figure for it is <not measured>, which is NOT a zero.\n",
				col.FY, col.CoverageClass)
		}
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fleetWriteHuman(cmd, res)
	} else {
		out, err := json.MarshalIndent(fleetEnvelope{Meta: fleetBuildMeta(), Results: res}, "", "  ")
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(out, flags, "results", true)
		if err != nil {
			return err
		}
		if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
			return err
		}
	}

	if strict && len(failed) > 0 {
		names := make([]string, 0, len(failed))
		for _, a := range failed {
			names = append(names, a.Name)
		}
		return fmt.Errorf("%d of %d assertions failed under --strict: %s",
			len(failed), len(res.Assertions), strings.Join(names, ", "))
	}
	return nil
}

// fleetRenderAbsent emits the absent-parent answer. It is an ANSWER, not a
// miss, so it exits 0 — with `plants` null rather than [] and every capacity
// sum unmeasured, so no consumer can read it as "generated nothing".
func fleetRenderAbsent(cmd *cobra.Command, flags *rootFlags, res fleetResults, pp nepraxwalk.ParentPlants) error {
	res.AsOf = fleetAsOf{
		AsOfFY: nepraxwalk.LatestObservedFY(),
		Basis: fmt.Sprintf("nepraxwalk.PlantsForParent(%q) reports this operator's fleet as absent from these workbooks; both capacity sums are unmeasured by construction",
			pp.PSXTicker),
		CapacityOperating:    pp.CapacityOperating,
		CapacityNonOperating: pp.CapacityNonOperating,
		DoNotAdd: "there is nothing to add: no plant of this operator appears in these workbooks, so both sums are unmeasured. " +
			"An unmeasured sum is NOT a measurement of zero capacity.",
	}
	res.Plants = nil
	res.FiscalYears = []fleetFiscalYear{}
	res.Assertions = []fleetAssertion{fleetAbsentSchemaAssertion(), fleetAbsentSumsAssertion(pp)}

	fmt.Fprintf(cmd.ErrOrStderr(), "ABSENT FROM DATASET: %s (PSX %s). %s\n",
		pp.ParentName, pp.PSXTicker, pp.AbsenceNote)

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "%s (PSX %s)\n  status: %s\n", pp.ParentName, pp.PSXTicker, pp.Status)
		fmt.Fprintf(w, "  capacity_operating:     %s\n", pp.CapacityOperating)
		fmt.Fprintf(w, "  capacity_non_operating: %s\n", pp.CapacityNonOperating)
		if len(res.Resolution.ProbeTokens) > 0 {
			fmt.Fprintf(w, "  probe tokens searched (0 hits over 241 published names): %s\n",
				strings.Join(res.Resolution.ProbeTokens, ", "))
		}
		if res.Resolution.Caveat != "" {
			fmt.Fprintf(cmd.ErrOrStderr(), "CAVEAT: %s\n", res.Resolution.Caveat)
		}
		return nil
	}
	out, err := json.MarshalIndent(fleetEnvelope{Meta: fleetBuildMeta(), Results: res}, "", "  ")
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(out, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

func fleetAbsentSchemaAssertion() fleetAssertion {
	rows := nepraxwalk.Rows()
	obs := 0
	for _, r := range rows {
		obs += len(r.Observed)
	}
	if err := nepraxwalk.Validate(); err != nil {
		return fleetAssertion{Name: "crosswalk_schema", OK: false,
			Detail: fmt.Sprintf("nepraxwalk.Validate() failed over %d rows / %d observations: %v", len(rows), obs, err)}
	}
	return fleetAssertion{Name: "crosswalk_schema", OK: true,
		Detail: fmt.Sprintf("nepraxwalk.Validate() clean over %d rows / %d observations", len(rows), obs)}
}

// fleetAbsentSumsAssertion is the guard that keeps the absence from ever
// hardening into a zero.
func fleetAbsentSumsAssertion(pp nepraxwalk.ParentPlants) fleetAssertion {
	okOp := !pp.CapacityOperating.Measured()
	okNon := !pp.CapacityNonOperating.Measured()
	okPlants := len(pp.Plants) == 0
	return fleetAssertion{
		Name: "absence_is_not_zero", OK: okOp && okNon && okPlants,
		Detail: fmt.Sprintf("capacity_operating measured=%v, capacity_non_operating measured=%v, plants=%d; all three must be false/false/0 so the absence cannot read as 0 MW",
			pp.CapacityOperating.Measured(), pp.CapacityNonOperating.Measured(), len(pp.Plants)),
	}
}

// ---------------------------------------------------------------------------
// Human rendering
// ---------------------------------------------------------------------------

func fleetWriteHuman(cmd *cobra.Command, res fleetResults) {
	w := cmd.OutOrStdout()
	e := cmd.ErrOrStderr()

	label := res.Resolution.ParentName
	if label == "" {
		label = res.Selector.Value
	}
	fmt.Fprintf(w, "%s", label)
	if res.Resolution.PSXTicker != "" {
		fmt.Fprintf(w, "  (PSX %s, %s)", res.Resolution.PSXTicker, res.Resolution.ListedStatus)
	}
	fmt.Fprintf(w, "\n%d plant(s) reached via %s %q", res.Resolution.PlantsReached, res.Selector.Kind, res.Selector.Value)
	if res.Resolution.MatchKind != "" {
		fmt.Fprintf(w, " [match: %s -> %q]", res.Resolution.MatchKind, res.Resolution.MatchedName)
	}
	fmt.Fprintln(w)

	// The as-of block: two lines, never one.
	fmt.Fprintf(w, "\nAS OF FY%s\n", res.AsOf.AsOfFY)
	fmt.Fprintf(w, "  operating       %2d plant(s)   %s\n", res.AsOf.OperatingPlants, res.AsOf.CapacityOperating)
	fmt.Fprintf(w, "  NON-operating   %2d plant(s)   %s\n", res.AsOf.NonOperatingPlants, res.AsOf.CapacityNonOperating)
	if res.AsOf.StatusUnknownPlants > 0 {
		fmt.Fprintf(w, "  status unknown  %2d plant(s)   (belongs to neither line above)\n", res.AsOf.StatusUnknownPlants)
	}
	if res.AsOf.NotObservedPlants > 0 {
		fmt.Fprintf(w, "  not observed    %2d plant(s)   (absent from the FY%s workbook)\n", res.AsOf.NotObservedPlants, res.AsOf.AsOfFY)
	}
	fmt.Fprintf(e, "DO NOT ADD: %s\n", res.AsOf.DoNotAdd)

	// The plant x fiscal-year grid.
	fys := make([]string, 0, len(res.FiscalYears))
	for _, c := range res.FiscalYears {
		fys = append(fys, c.FY)
	}
	if len(res.Plants) > 0 && len(fys) > 0 {
		fmt.Fprintf(w, "\nPLANT x FISCAL YEAR (installed capacity MW / S.No / block status)\n")
		nameW := 24
		for _, p := range res.Plants {
			if n := len(p.CanonicalName); n > nameW {
				nameW = n
			}
		}
		if nameW > 52 {
			nameW = 52
		}
		fmt.Fprintf(w, "%-*s", nameW, "PLANT")
		for _, fy := range fys {
			fmt.Fprintf(w, "  %-30s", "FY"+fy)
		}
		fmt.Fprintln(w)
		for _, p := range res.Plants {
			fmt.Fprintf(w, "%-*s", nameW, fleetTrunc(p.CanonicalName, nameW))
			for _, cell := range p.ByFY {
				fmt.Fprintf(w, "  %-30s", fleetCellText(cell, fleetClassifyFY(cell.FY)))
			}
			fmt.Fprintln(w)
		}
	}

	// Fiscal-year columns.
	fmt.Fprintf(w, "\nFISCAL YEARS\n")
	for _, c := range res.FiscalYears {
		fmt.Fprintf(w, "  FY%-8s %-22s observed %d/%d\n", c.FY, c.CoverageClass, c.ObservedInFY, c.PlantsInFleet)
		fmt.Fprintf(w, "      reported      %s\n", c.CapacityReported)
		fmt.Fprintf(w, "      operating     %s\n", c.CapacityOperating)
		fmt.Fprintf(w, "      NON-operating %s\n", c.CapacityNonOperating)
		// Printed only when it is non-zero: a +0.0% line is noise, and the
		// measured zero is still in the JSON for anyone who wants it.
		if c.OverstatementPct != nil && *c.OverstatementPct > 0 {
			fmt.Fprintf(w, "      summing every published capacity overstates live capacity by %+.1f%%\n", *c.OverstatementPct)
		}
		if c.CapacityNotReported > 0 || c.CapacityStatusCell > 0 || c.CapacityUnknownText > 0 {
			fmt.Fprintf(w, "      cells with no published number: %d blank, %d status sentinel, %d unmodelled text\n",
				c.CapacityNotReported, c.CapacityStatusCell, c.CapacityUnknownText)
		}
		if c.ClassNote != "" {
			fmt.Fprintf(e, "FY%s: %s\n", c.FY, c.ClassNote)
		}
	}

	if res.Resolution.ScopeNote != "" {
		fmt.Fprintf(e, "SCOPE: %s\n", res.Resolution.ScopeNote)
	}
	if res.Resolution.Caveat != "" {
		fmt.Fprintf(e, "CAVEAT: %s\n", res.Resolution.Caveat)
	}
	fmt.Fprintf(e, "S.No: %s\n", nepraFleetSNoNote)
	for _, g := range res.DeclaredGaps {
		fmt.Fprintf(e, "CANNOT ANSWER (%s): %s — %s\n", g.Verdict, g.Question, g.Reason)
	}
}

// fleetCellText renders one plant-year cell. An unobserved cell distinguishes
// "this fiscal year was never indexed" from "this plant is absent from a
// workbook that WAS indexed" — two different facts that a single
// "<not observed>" would merge.
func fleetCellText(c fleetPlantFY, class string) string {
	if !c.Observed {
		if class == fleetCoverageObserved {
			return "<not in FY workbook>"
		}
		return "<FY not indexed>"
	}
	capText := "<no value>"
	if c.InstalledCapacity != nil {
		capText = c.InstalledCapacity.String()
	}
	status := c.Status
	if c.StatusKnown != nil && !*c.StatusKnown {
		status = "status unknown"
	}
	sno := 0
	if c.SNo != nil {
		sno = *c.SNo
	}
	return fmt.Sprintf("%s  #%d %s", capText, sno, status)
}

func fleetTrunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}

func fleetWriteRefusalHuman(w io.Writer, r fleetRefusal) {
	b := r.Error
	fmt.Fprintf(w, "REFUSED (%s)\n", b.Kind)
	if b.Token != "" {
		fmt.Fprintf(w, "  token: %s\n", b.Token)
	}
	if b.Query != "" {
		fmt.Fprintf(w, "  query: %q\n", b.Query)
	}
	if b.DeclinedName != "" {
		fmt.Fprintf(w, "  declined_name: %q\n", b.DeclinedName)
	}
	fmt.Fprintf(w, "  reason: %s\n", b.Reason)
	if b.Hazard != "" {
		fmt.Fprintf(w, "  hazard: %s\n", b.Hazard)
	}
	if b.FuzzyMatching != "" {
		fmt.Fprintf(w, "  fuzzy_matching: %s\n", b.FuzzyMatching)
	}
	for _, c := range b.Candidates {
		fmt.Fprintf(w, "  candidate [%s] %s", c.Kind, c.CanonicalName)
		if c.ParentName != "" {
			fmt.Fprintf(w, " parent=%s", c.ParentName)
		}
		if c.PSXTicker != "" {
			fmt.Fprintf(w, " psx=%s", c.PSXTicker)
		}
		fmt.Fprintf(w, "\n      %s\n", c.Note)
	}
	if b.AbsentParentHint != "" {
		fmt.Fprintf(w, "  absent_parent_hint: %s\n", b.AbsentParentHint)
	}
	if b.CounterFact != "" {
		fmt.Fprintf(w, "  counter_fact: %s\n", b.CounterFact)
	}
	if len(b.NearDuplicateGroup) > 0 {
		fmt.Fprintf(w, "  reviewed near-duplicate group: %s\n", strings.Join(b.NearDuplicateGroup, " | "))
	}
	if len(b.KnownSymbols) > 0 {
		fmt.Fprintf(w, "  known PSX symbols (%d): %s\n", len(b.KnownSymbols), strings.Join(b.KnownSymbols, ", "))
	}
	for _, rec := range b.Recovery {
		fmt.Fprintf(w, "  try: %s\n", rec)
	}
}

// ---------------------------------------------------------------------------
// Catalogue (no request, no selector)
// ---------------------------------------------------------------------------

type fleetCatalogue struct {
	Operators      []fleetCatalogueOperator     `json:"operators"`
	RefusedTokens  []fleetCatalogueRefusedToken `json:"refused_tokens"`
	AbsentParents  []fleetCatalogueAbsentParent `json:"absent_parents"`
	DeclaredGaps   []fleetGap                   `json:"declared_gaps"`
	Usage          []string                     `json:"usage"`
	DuplicateNotes []fleetCatalogueDuplicate    `json:"reviewed_near_duplicates"`
}

type fleetCatalogueOperator struct {
	PSXTicker           string           `json:"psx_ticker"`
	ParentName          string           `json:"parent_name"`
	Plants              int              `json:"plants"`
	AsOfFY              string           `json:"as_of_fy"`
	OperatingPlants     int              `json:"operating_plants"`
	NonOperatingPlants  int              `json:"non_operating_plants"`
	StatusUnknownPlants int              `json:"status_unknown_plants"`
	NotObservedPlants   int              `json:"not_observed_plants"`
	CapacityOperating   nepraxwalk.MWSum `json:"capacity_operating"`
	// CapacityNonOperating is published capacity that is NOT generating. It is
	// never added to the line above.
	CapacityNonOperating nepraxwalk.MWSum `json:"capacity_non_operating"`
}

type fleetCatalogueRefusedToken struct {
	Token      string                      `json:"token"`
	Reason     string                      `json:"reason"`
	Hazard     string                      `json:"hazard"`
	Candidates []nepraxwalk.TokenCandidate `json:"candidates"`
}

type fleetCatalogueAbsentParent struct {
	PSXTicker   string   `json:"psx_ticker"`
	ParentName  string   `json:"parent_name"`
	Status      string   `json:"status"`
	AbsenceNote string   `json:"absence_note"`
	ProbeTokens []string `json:"probe_tokens"`
	Caveat      string   `json:"caveat"`
	Reach       string   `json:"reach"`
}

type fleetCatalogueDuplicate struct {
	Names     []string `json:"names"`
	Verdict   string   `json:"verdict"`
	Suspicion string   `json:"suspicion"`
}

// fleetPrintCatalogue lists what `fleet` can be pointed at. It touches only the
// embedded crosswalk: no client is constructed and no request is made.
func fleetPrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	cat := fleetCatalogue{
		DeclaredGaps: fleetDeclaredGaps(),
		Usage: []string{
			"nepra-pp-cli fleet --parent HUBC",
			"nepra-pp-cli fleet --parent HUBC --fy-range 2017-18:2023-24 --agent",
			"nepra-pp-cli fleet --parent KEL --parent-kind psx-symbol",
			"nepra-pp-cli fleet --plant \"Kotri Power Station\"",
		},
	}
	for _, t := range nepraxwalk.Tickers() {
		pp, ok := nepraxwalk.PlantsForParent(t)
		if !ok {
			continue
		}
		cat.Operators = append(cat.Operators, fleetCatalogueOperator{
			PSXTicker:            t,
			ParentName:           pp.ParentName,
			Plants:               len(pp.Plants),
			AsOfFY:               pp.AsOfFY,
			OperatingPlants:      pp.OperatingPlants,
			NonOperatingPlants:   pp.NonOperatingPlants,
			StatusUnknownPlants:  pp.StatusUnknownPlants,
			NotObservedPlants:    pp.NotObservedPlants,
			CapacityOperating:    pp.CapacityOperating,
			CapacityNonOperating: pp.CapacityNonOperating,
		})
	}
	for _, a := range nepraxwalk.AmbiguousTokens() {
		cat.RefusedTokens = append(cat.RefusedTokens, fleetCatalogueRefusedToken{
			Token: a.Token, Reason: a.Reason, Hazard: a.Hazard, Candidates: a.Candidates,
		})
	}
	for _, ap := range nepraxwalk.AbsentParents() {
		cat.AbsentParents = append(cat.AbsentParents, fleetCatalogueAbsentParent{
			PSXTicker: ap.PSXTicker, ParentName: ap.ParentName, Status: string(ap.Status),
			AbsenceNote: ap.AbsenceNote, ProbeTokens: ap.ProbeTokens, Caveat: ap.Caveat,
			Reach: fmt.Sprintf("nepra-pp-cli fleet --parent %s --parent-kind %s", ap.PSXTicker, fleetParentKindPSX),
		})
	}
	for _, pd := range nepraxwalk.PossibleDuplicates() {
		cat.DuplicateNotes = append(cat.DuplicateNotes, fleetCatalogueDuplicate{
			Names: pd.Names, Verdict: pd.Verdict, Suspicion: pd.Suspicion,
		})
	}

	meta := fleetBuildMeta()
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.OutOrStdout()
		fmt.Fprintf(w, "%d PSX-listed operators with plants in this crosswalk (as of FY%s)\n",
			len(cat.Operators), meta.Crosswalk.LatestObservedFY)
		fmt.Fprintf(w, "%-7s %-42s %6s  %-28s %-28s\n", "SYMBOL", "PARENT", "PLANTS", "OPERATING", "NON-OPERATING")
		for _, o := range cat.Operators {
			fmt.Fprintf(w, "%-7s %-42s %6d  %-28s %-28s\n",
				o.PSXTicker, fleetTrunc(o.ParentName, 42), o.Plants,
				o.CapacityOperating.String(), o.CapacityNonOperating.String())
		}
		fmt.Fprintf(w, "\n%d REFUSED tokens (both readings are returned; none is picked):\n", len(cat.RefusedTokens))
		for _, t := range cat.RefusedTokens {
			fmt.Fprintf(w, "  %-5s %s\n", t.Token, t.Reason)
			for _, c := range t.Candidates {
				fmt.Fprintf(w, "        [%s] %s %s\n", c.Kind, c.CanonicalName, c.PSXTicker)
			}
		}
		fmt.Fprintf(w, "\n%d operator(s) known ABSENT from these workbooks:\n", len(cat.AbsentParents))
		for _, a := range cat.AbsentParents {
			fmt.Fprintf(w, "  %-5s %s -> %s\n", a.PSXTicker, a.ParentName, a.Reach)
		}
		e := cmd.ErrOrStderr()
		fmt.Fprintf(e, "crosswalk: %d rows / %d observations, schema_valid=%v. observed %v, held out %v, NEPRA publishes %v.\n",
			meta.Crosswalk.Rows, meta.Crosswalk.Observations, meta.Crosswalk.SchemaValid,
			meta.Crosswalk.ObservedFYs, meta.Crosswalk.HeldOutFYs, meta.Crosswalk.PublishedFYs)
		fmt.Fprintf(e, "S.No: %s\n", nepraFleetSNoNote)
		for _, g := range cat.DeclaredGaps {
			fmt.Fprintf(e, "CANNOT ANSWER (%s): %s — %s\n", g.Verdict, g.Question, g.Reason)
		}
		fmt.Fprintln(e, "No request was made. Pass --parent <SYMBOL> or --plant \"<published name>\" for a panel.")
		return nil
	}

	payload, err := json.MarshalIndent(map[string]any{"meta": meta, "results": cat}, "", "  ")
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}
