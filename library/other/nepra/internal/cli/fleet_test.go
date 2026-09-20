// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// HAND-AUTHORED behaviour tests for the `fleet` panel. Every number asserted
// here was MEASURED by running this command against the embedded crosswalk, not
// copied from a document.
//
// All offline: the crosswalk is embedded, no fixture and no request is needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

// runFleet drives the command in isolation with machine output, which is what a
// piped caller and an agent both get.
func runFleet(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	flags := &rootFlags{asJSON: true}
	cmd := newNepraFleetCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	// root.go:287 sets SilenceUsage on the real root, and cobra makes every
	// subcommand respect it. Mirrored here so a standalone command does not
	// dump its usage text onto stdout and make a refusal envelope unparseable
	// in the test but fine in the shipped binary.
	silenceFleetUsage(cmd)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

func silenceFleetUsage(cmd *cobra.Command) {
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
}

// runFleetHuman drives the command with human output forced on.
func runFleetHuman(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	old := humanFriendly
	humanFriendly = true
	t.Cleanup(func() { humanFriendly = old })
	flags := &rootFlags{}
	cmd := newNepraFleetCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs(args)
	silenceFleetUsage(cmd)
	err = cmd.Execute()
	return out.String(), errBuf.String(), err
}

type fleetTestEnvelope struct {
	Meta struct {
		Source    string `json:"source"`
		Crosswalk struct {
			SchemaValid      bool     `json:"schema_valid"`
			Rows             int      `json:"rows"`
			Observations     int      `json:"observations"`
			ObservedFYs      []string `json:"observed_fys"`
			HeldOutFYs       []string `json:"held_out_fys"`
			LatestObservedFY string   `json:"latest_observed_fy"`
			PublishedFYs     []string `json:"published_fys"`
			SNoNote          string   `json:"sno_note"`
		} `json:"crosswalk"`
	} `json:"meta"`
	Results struct {
		Selector struct {
			Kind       string `json:"kind"`
			Value      string `json:"value"`
			ParentKind string `json:"parent_kind"`
		} `json:"selector"`
		Resolution struct {
			Token         string   `json:"token"`
			TokenKind     string   `json:"token_kind"`
			ParentName    string   `json:"parent_name"`
			PSXTicker     string   `json:"psx_ticker"`
			ListedStatus  string   `json:"listed_status"`
			Status        string   `json:"status"`
			PlantsReached int      `json:"plants_reached"`
			MatchKind     string   `json:"match_kind"`
			MatchedName   string   `json:"matched_name"`
			ScopeNote     string   `json:"scope_note"`
			AbsenceNote   string   `json:"absence_note"`
			ProbeTokens   []string `json:"probe_tokens"`
			Caveat        string   `json:"caveat"`
		} `json:"resolution"`
		AsOf struct {
			AsOfFY               string      `json:"as_of_fy"`
			OperatingPlants      int         `json:"operating_plants"`
			NonOperatingPlants   int         `json:"non_operating_plants"`
			StatusUnknownPlants  int         `json:"status_unknown_plants"`
			NotObservedPlants    int         `json:"not_observed_plants"`
			CapacityOperating    fleetTestMW `json:"capacity_operating"`
			CapacityNonOperating fleetTestMW `json:"capacity_non_operating"`
			DoNotAdd             string      `json:"do_not_add"`
		} `json:"as_of"`
		Plants      []fleetTestPlant `json:"plants"`
		FiscalYears []fleetTestFY    `json:"fiscal_years"`
		Assertions  []struct {
			Name   string `json:"name"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail"`
		} `json:"assertions"`
		DeclaredGaps []struct {
			Question string `json:"question"`
			Verdict  string `json:"verdict"`
			Reason   string `json:"reason"`
		} `json:"declared_gaps"`
	} `json:"results"`
}

// fleetTestMW mirrors nepraxwalk.MWSum's wire shape. MW is a POINTER so the
// test can tell an omitted `mw` key from a zero one — that distinction is the
// whole point of the type.
type fleetTestMW struct {
	Measured bool     `json:"measured"`
	MW       *float64 `json:"mw"`
	Plants   int      `json:"plants"`
}

type fleetTestPlant struct {
	CanonicalName string          `json:"canonical_name"`
	Aliases       []string        `json:"aliases"`
	PSXTicker     string          `json:"psx_ticker"`
	Confidence    string          `json:"confidence"`
	ValidFromFY   string          `json:"valid_from_fy"`
	ValidToFY     string          `json:"valid_to_fy"`
	ByFY          []fleetTestCell `json:"by_fy"`
}

type fleetTestCell struct {
	FY                string `json:"fy"`
	Observed          bool   `json:"observed"`
	Reason            string `json:"reason"`
	SNo               *int   `json:"sno"`
	Status            string `json:"status"`
	Operating         *bool  `json:"operating"`
	StatusKnown       *bool  `json:"status_known"`
	InstalledCapacity *struct {
		State string   `json:"state"`
		Value *float64 `json:"value"`
		Raw   string   `json:"raw"`
	} `json:"installed_capacity"`
}

type fleetTestFY struct {
	FY                    string      `json:"fy"`
	FYObserved            bool        `json:"fy_observed"`
	CoverageClass         string      `json:"coverage_class"`
	PlantsInFleet         int         `json:"plants_in_fleet"`
	ObservedInFY          int         `json:"observed_in_fy"`
	NoObservationInFY     int         `json:"no_observation_in_fy"`
	CapacityReported      fleetTestMW `json:"capacity_reported"`
	CapacityOperating     fleetTestMW `json:"capacity_operating"`
	CapacityNonOperating  fleetTestMW `json:"capacity_non_operating"`
	CapacityStatusUnknown fleetTestMW `json:"capacity_status_unknown"`
	CapacityBasis         string      `json:"capacity_basis"`
	ClassNote             string      `json:"class_note"`
	OverstatementPct      *float64    `json:"published_vs_operating_overstatement_pct"`
}

func decodeFleet(t *testing.T, stdout string) fleetTestEnvelope {
	t.Helper()
	var env fleetTestEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not the fleet envelope: %v\n%s", err, stdout)
	}
	return env
}

func fleetPlantByName(t *testing.T, env fleetTestEnvelope, name string) fleetTestPlant {
	t.Helper()
	for _, p := range env.Results.Plants {
		if p.CanonicalName == name {
			return p
		}
	}
	t.Fatalf("plant %q not in panel; got %v", name, fleetPlantNames(env))
	return fleetTestPlant{}
}

func fleetPlantNames(env fleetTestEnvelope) []string {
	out := make([]string, 0, len(env.Results.Plants))
	for _, p := range env.Results.Plants {
		out = append(out, p.CanonicalName)
	}
	return out
}

func fleetCellByFY(t *testing.T, p fleetTestPlant, fy string) fleetTestCell {
	t.Helper()
	for _, c := range p.ByFY {
		if c.FY == fy {
			return c
		}
	}
	t.Fatalf("plant %q has no FY%s cell", p.CanonicalName, fy)
	return fleetTestCell{}
}

func fleetFYByLabel(t *testing.T, env fleetTestEnvelope, fy string) fleetTestFY {
	t.Helper()
	for _, c := range env.Results.FiscalYears {
		if c.FY == fy {
			return c
		}
	}
	t.Fatalf("no FY%s column in the panel", fy)
	return fleetTestFY{}
}

// wantMeasured asserts a sum is a measurement of exactly mw over plants plants.
func wantMeasured(t *testing.T, label string, got fleetTestMW, mw float64, plants int) {
	t.Helper()
	if !got.Measured {
		t.Errorf("%s: measured=false, want a measurement of %.2f MW over %d plants", label, mw, plants)
		return
	}
	if got.MW == nil {
		t.Errorf("%s: measured=true but the mw key is absent", label)
		return
	}
	if *got.MW != mw || got.Plants != plants {
		t.Errorf("%s = %.2f MW over %d plants, want %.2f MW over %d plants", label, *got.MW, got.Plants, mw, plants)
	}
}

// wantUnmeasured asserts a sum carries NO number at all. A `mw` key present on
// an unmeasured sum is the fabricated zero this command exists to prevent.
func wantUnmeasured(t *testing.T, label string, got fleetTestMW) {
	t.Helper()
	if got.Measured {
		t.Errorf("%s: measured=true, want unmeasured", label)
	}
	if got.MW != nil {
		t.Errorf("%s: unmeasured sum carries mw=%v; the key must be absent", label, *got.MW)
	}
	if got.Plants != 0 {
		t.Errorf("%s: unmeasured sum claims %d plants, want 0", label, got.Plants)
	}
}

// ---------------------------------------------------------------------------
// Wiring (the scaffold's original smoke test, kept and extended)
// ---------------------------------------------------------------------------

// TestNovelFleetHelpWires smoke-tests that the fleet command resolves at
// runtime and renders useful --help output. Catches wiring regressions (missing
// AddCommand, panicking RunE on --help, etc.).
func TestNovelFleetHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"fleet", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("fleet --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "fleet", "--parent", "--plant", "--parent-kind", "--fy-range", "--strict"} {
		if !strings.Contains(help, want) {
			t.Fatalf("fleet --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestFleetAnnotations pins the annotations the publish gate reads, and asserts
// the command is no longer a scaffold so root.go's preferImplementedNovelCommands
// cannot drop it.
func TestFleetAnnotations(t *testing.T) {
	cmd := newNepraFleetCmd(&rootFlags{})
	if isNovelScaffoldCommand(cmd) {
		t.Error("fleet still carries pp:novel-scaffold; an implemented command must not")
	}
	for k, want := range map[string]string{
		"mcp:read-only":       "true",
		"pp:happy-args":       "--parent=HUBC",
		"pp:typed-exit-codes": "true",
		"pp:novel-hand-coded": "true",
	} {
		if got := cmd.Annotations[k]; got != want {
			t.Errorf("annotation %q = %q, want %q", k, got, want)
		}
	}
}

// ---------------------------------------------------------------------------
// The HUBC panel
// ---------------------------------------------------------------------------

// TestFleetHUBCPanel pins every measured number in the worked example.
func TestFleetHUBCPanel(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "HUBC")
	if err != nil {
		t.Fatalf("fleet --parent HUBC returned %v", err)
	}
	env := decodeFleet(t, stdout)

	if env.Meta.Source != "computed" {
		t.Errorf("meta.source = %q, want %q", env.Meta.Source, "computed")
	}
	if !env.Meta.Crosswalk.SchemaValid {
		t.Error("meta.crosswalk.schema_valid = false")
	}
	if env.Meta.Crosswalk.Rows != 133 || env.Meta.Crosswalk.Observations != 241 {
		t.Errorf("crosswalk scale = %d rows / %d observations, want 133 / 241",
			env.Meta.Crosswalk.Rows, env.Meta.Crosswalk.Observations)
	}
	if env.Meta.Crosswalk.LatestObservedFY != "2023-24" {
		t.Errorf("latest_observed_fy = %q, want 2023-24", env.Meta.Crosswalk.LatestObservedFY)
	}
	if len(env.Meta.Crosswalk.PublishedFYs) != 7 {
		t.Errorf("published_fys has %d entries, want 7", len(env.Meta.Crosswalk.PublishedFYs))
	}

	r := env.Results.Resolution
	if r.Token != "HUBC" || r.TokenKind != "psx_ticker" {
		t.Errorf("resolution token/kind = %q/%q, want HUBC/psx_ticker", r.Token, r.TokenKind)
	}
	if r.ParentName != "The Hub Power Company Limited" {
		t.Errorf("parent_name = %q", r.ParentName)
	}
	if r.ListedStatus != "listed" || r.Status != "has_plants" {
		t.Errorf("listed_status/status = %q/%q, want listed/has_plants", r.ListedStatus, r.Status)
	}
	if r.PlantsReached != 6 {
		t.Errorf("plants_reached = %d, want 6", r.PlantsReached)
	}

	want := []string{
		"China Power Hub Generation Company (Private) Limited. (CPHGCL)",
		"Hub Power Company (HUBCO)",
		"Narowal Energy Ltd.",
		"New Bong Escape Hydropower Project",
		"ThalNova Power Thar (Pvt.) Limited (TPTPL)",
		"Thar Energy Limited (TEL)",
	}
	got := fleetPlantNames(env)
	if len(got) != len(want) {
		t.Fatalf("panel has %d plants, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("plants[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	as := env.Results.AsOf
	if as.AsOfFY != "2023-24" {
		t.Errorf("as_of.as_of_fy = %q, want 2023-24", as.AsOfFY)
	}
	if as.OperatingPlants != 5 || as.NonOperatingPlants != 1 ||
		as.StatusUnknownPlants != 0 || as.NotObservedPlants != 0 {
		t.Errorf("as_of partition = %d/%d/%d/%d, want 5/1/0/0",
			as.OperatingPlants, as.NonOperatingPlants, as.StatusUnknownPlants, as.NotObservedPlants)
	}
	wantMeasured(t, "as_of.capacity_operating", as.CapacityOperating, 2289, 5)
	wantMeasured(t, "as_of.capacity_non_operating", as.CapacityNonOperating, 1292, 1)

	// The delicensed plant: published, real, generating nothing, and it KEEPS
	// its 1,292 MW installed capacity.
	hub := fleetPlantByName(t, env, "Hub Power Company (HUBCO)")
	cell := fleetCellByFY(t, hub, "2023-24")
	if !cell.Observed {
		t.Fatal("Hub Power FY2023-24 is not observed")
	}
	if cell.Status != "delicensed" {
		t.Errorf("Hub Power FY2023-24 status = %q, want delicensed", cell.Status)
	}
	if cell.Operating == nil || *cell.Operating {
		t.Errorf("Hub Power FY2023-24 operating = %v, want false", cell.Operating)
	}
	if cell.StatusKnown == nil || !*cell.StatusKnown {
		t.Errorf("Hub Power FY2023-24 status_known = %v, want true", cell.StatusKnown)
	}
	if cell.InstalledCapacity == nil || cell.InstalledCapacity.Value == nil || *cell.InstalledCapacity.Value != 1292 {
		t.Errorf("Hub Power FY2023-24 installed_capacity = %+v, want a numeric 1292", cell.InstalledCapacity)
	}
	if cell.InstalledCapacity != nil && cell.InstalledCapacity.State != "numeric" {
		t.Errorf("Hub Power FY2023-24 capacity state = %q, want numeric", cell.InstalledCapacity.State)
	}
	// valid_to_fy is empty even for the delicensed plant: a publication
	// interval, never an operating claim.
	if hub.ValidToFY != "" {
		t.Errorf("Hub Power valid_to_fy = %q, want empty even though the plant is delicensed", hub.ValidToFY)
	}
	if len(hub.Aliases) == 0 || !strings.Contains(hub.Aliases[0], "Hub Power Company (HUBCO)") {
		t.Errorf("Hub Power aliases = %q, want the NBSP-padded published variant", hub.Aliases)
	}
}

// TestFleetNeverSumsOperatingAndNonOperating is the guard on the one
// arithmetic this command exists to prevent.
func TestFleetNeverSumsOperatingAndNonOperating(t *testing.T) {
	stdout, stderr, err := runFleet(t, "--parent", "HUBC")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)

	// 3,581 MW is the sum of HUBC's six published capacities. It is a real
	// figure — capacity_reported — and it must appear NOWHERE else.
	fy := fleetFYByLabel(t, env, "2023-24")
	wantMeasured(t, "FY2023-24 capacity_reported", fy.CapacityReported, 3581, 6)
	wantMeasured(t, "FY2023-24 capacity_operating", fy.CapacityOperating, 2289, 5)
	wantMeasured(t, "FY2023-24 capacity_non_operating", fy.CapacityNonOperating, 1292, 1)

	// Blank the one legitimate occurrence, then no 3581 may remain.
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	results := raw["results"].(map[string]any)
	for _, col := range results["fiscal_years"].([]any) {
		delete(col.(map[string]any), "capacity_reported")
	}
	scrubbed, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"3581", "3,581"} {
		if strings.Contains(string(scrubbed), forbidden) {
			t.Errorf("payload leaks the combined total %q outside fiscal_years[].capacity_reported", forbidden)
		}
	}
	if strings.Contains(stderr, "3581") || strings.Contains(stderr, "3,581") {
		t.Errorf("stderr leaks the combined total:\n%s", stderr)
	}

	// The do-not-add sentence must name the delicensed plant, and must not
	// carry a combined figure.
	if !strings.Contains(env.Results.AsOf.DoNotAdd, "Hub Power Company (HUBCO)") {
		t.Errorf("do_not_add does not name the delicensed plant: %q", env.Results.AsOf.DoNotAdd)
	}
	if !strings.Contains(env.Results.AsOf.DoNotAdd, "1292.00 MW") {
		t.Errorf("do_not_add does not quote the non-operating megawatts: %q", env.Results.AsOf.DoNotAdd)
	}

	// Every capacity object in the payload obeys the measured/mw contract.
	for _, c := range env.Results.FiscalYears {
		for label, s := range map[string]fleetTestMW{
			"capacity_reported":       c.CapacityReported,
			"capacity_operating":      c.CapacityOperating,
			"capacity_non_operating":  c.CapacityNonOperating,
			"capacity_status_unknown": c.CapacityStatusUnknown,
		} {
			if s.Measured && s.MW == nil {
				t.Errorf("FY%s %s: measured with no mw key", c.FY, label)
			}
			if !s.Measured && s.MW != nil {
				t.Errorf("FY%s %s: unmeasured but carries mw=%v", c.FY, label, *s.MW)
			}
		}
	}
}

// TestFleetOverstatementIsMeasured pins the +56.4% this panel exists to warn
// about. 3581/2289 - 1 = 56.443861948449104%.
func TestFleetOverstatementIsMeasured(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "HUBC")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	fy := fleetFYByLabel(t, env, "2023-24")
	if fy.OverstatementPct == nil {
		t.Fatal("FY2023-24 carries no published_vs_operating_overstatement_pct")
	}
	if got := *fy.OverstatementPct; got < 56.443 || got > 56.444 {
		t.Errorf("FY2023-24 overstatement = %v%%, want 56.4438...%% (3581/2289)", got)
	}
	// FY2017-18 has no delicensed plant, so the overstatement is a MEASURED
	// zero — present, not omitted.
	f18 := fleetFYByLabel(t, env, "2017-18")
	if f18.OverstatementPct == nil || *f18.OverstatementPct != 0 {
		t.Errorf("FY2017-18 overstatement = %v, want a measured 0", f18.OverstatementPct)
	}
}

// TestFleetFY2017_18Column pins the earlier year, including CPHGCL's MEASURED
// zero and a measured (not unmeasured) zero for non-operating capacity.
func TestFleetFY2017_18Column(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "HUBC", "--fy-range", "2017-18:2017-18")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	if len(env.Results.FiscalYears) != 1 {
		t.Fatalf("got %d fiscal-year columns, want 1", len(env.Results.FiscalYears))
	}
	c := env.Results.FiscalYears[0]
	if c.FY != "2017-18" || !c.FYObserved || c.CoverageClass != "observed" {
		t.Errorf("column = %q observed=%v class=%q, want 2017-18 true observed", c.FY, c.FYObserved, c.CoverageClass)
	}
	if c.ObservedInFY != 4 || c.NoObservationInFY != 2 || c.PlantsInFleet != 6 {
		t.Errorf("population = %d observed / %d unobserved of %d, want 4 / 2 of 6",
			c.ObservedInFY, c.NoObservationInFY, c.PlantsInFleet)
	}
	wantMeasured(t, "FY2017-18 capacity_reported", c.CapacityReported, 1601, 4)
	wantMeasured(t, "FY2017-18 capacity_operating", c.CapacityOperating, 1601, 4)
	// A MEASURED zero, not an absence of measurement: FY2017-18 carries no
	// status sentinels at all.
	wantMeasured(t, "FY2017-18 capacity_non_operating", c.CapacityNonOperating, 0, 0)

	cph := fleetPlantByName(t, env, "China Power Hub Generation Company (Private) Limited. (CPHGCL)")
	cell := fleetCellByFY(t, cph, "2017-18")
	if cell.InstalledCapacity == nil {
		t.Fatal("CPHGCL FY2017-18 carries no installed_capacity")
	}
	if cell.InstalledCapacity.State != "numeric" {
		t.Errorf("CPHGCL FY2017-18 state = %q, want numeric (a published zero is DATA)", cell.InstalledCapacity.State)
	}
	if cell.InstalledCapacity.Value == nil || *cell.InstalledCapacity.Value != 0 {
		t.Errorf("CPHGCL FY2017-18 value = %v, want a measured 0", cell.InstalledCapacity.Value)
	}
	if cell.InstalledCapacity.Raw != "0" {
		t.Errorf("CPHGCL FY2017-18 raw = %q, want %q", cell.InstalledCapacity.Raw, "0")
	}

	// TEL and TPTPL are absent from the FY2017-18 workbook and must carry NO
	// capacity key at all.
	for _, name := range []string{"Thar Energy Limited (TEL)", "ThalNova Power Thar (Pvt.) Limited (TPTPL)"} {
		p := fleetPlantByName(t, env, name)
		cell := fleetCellByFY(t, p, "2017-18")
		if cell.Observed {
			t.Errorf("%s: FY2017-18 observed=true, want false", name)
		}
		if cell.InstalledCapacity != nil {
			t.Errorf("%s: unobserved FY2017-18 carries installed_capacity %+v", name, cell.InstalledCapacity)
		}
		if cell.Operating != nil || cell.StatusKnown != nil || cell.SNo != nil {
			t.Errorf("%s: unobserved FY2017-18 carries status fields", name)
		}
		if !strings.Contains(cell.Reason, "not in the FY2017-18 workbook") {
			t.Errorf("%s: FY2017-18 reason = %q", name, cell.Reason)
		}
	}
}

// TestFleetHeldOutYearIsUnmeasured is the fabricated-zero guard. Replacing
// Coverage's MWSum with a hand-rolled float64 fails this test.
func TestFleetHeldOutYearIsUnmeasured(t *testing.T) {
	stdout, stderr, err := runFleet(t, "--parent", "HUBC", "--fy-range", "2020-21:2020-21")
	if err != nil {
		t.Fatalf("a legitimately unmeasurable window must still exit 0; got %v", err)
	}
	env := decodeFleet(t, stdout)
	c := fleetFYByLabel(t, env, "2020-21")
	if c.FYObserved {
		t.Error("FY2020-21 reports fy_observed=true")
	}
	if c.CoverageClass != "held_out" {
		t.Errorf("coverage_class = %q, want held_out", c.CoverageClass)
	}
	if c.NoObservationInFY != 6 || c.ObservedInFY != 0 {
		t.Errorf("population = %d observed / %d unobserved, want 0 / 6", c.ObservedInFY, c.NoObservationInFY)
	}
	wantUnmeasured(t, "FY2020-21 capacity_reported", c.CapacityReported)
	wantUnmeasured(t, "FY2020-21 capacity_operating", c.CapacityOperating)
	wantUnmeasured(t, "FY2020-21 capacity_non_operating", c.CapacityNonOperating)
	wantUnmeasured(t, "FY2020-21 capacity_status_unknown", c.CapacityStatusUnknown)
	if !strings.Contains(c.CapacityBasis, "NOT a finding of zero capacity") {
		t.Errorf("capacity_basis does not disclaim the zero reading: %q", c.CapacityBasis)
	}
	// The class note must carry the workbook's real published capacity, so a
	// reader cannot mistake the unmeasured column for an empty year.
	if !strings.Contains(c.ClassNote, "36,902.00 MW") || !strings.Contains(c.ClassNote, "(NPPCL) - Balloki") {
		t.Errorf("held-out class_note is missing its measured counter-evidence: %q", c.ClassNote)
	}
	if !strings.Contains(stderr, "UNMEASURED: FY2020-21 is held_out") {
		t.Errorf("stderr does not warn about the unmeasured year:\n%s", stderr)
	}
	// No fiscal-year capacity object may carry an mw key. (The as_of block is
	// deliberately still FY2023-24 — the crosswalk's latest observed year,
	// stated in as_of_fy — so it is measured regardless of --fy-range.)
	var raw map[string]any
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		t.Fatal(err)
	}
	cols, err := json.Marshal(raw["results"].(map[string]any)["fiscal_years"])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(cols), "\"mw\"") {
		t.Errorf("a fully unmeasured fiscal-year column emitted an mw key:\n%s", cols)
	}
	if env.Results.AsOf.AsOfFY != "2023-24" {
		t.Errorf("as_of.as_of_fy = %q; it must stay the latest OBSERVED year and say so, not follow --fy-range", env.Results.AsOf.AsOfFY)
	}

	// Human mode must print the same non-answer as "<not measured>".
	hOut, _, err := runFleetHuman(t, "--parent", "HUBC", "--fy-range", "2020-21:2020-21")
	if err != nil {
		t.Fatalf("human mode returned %v", err)
	}
	if !strings.Contains(hOut, "<not measured>") {
		t.Errorf("human output does not print <not measured>:\n%s", hOut)
	}
	if strings.Contains(hOut, "0.00 MW over 0 plants") {
		t.Errorf("human output printed a zero for the held-out year:\n%s", hOut)
	}
}

// TestFleetStrictFailsWhenNoYearObserved: --strict turns a fully unmeasurable
// window into a non-zero exit; without it the panel still prints.
func TestFleetStrictFailsWhenNoYearObserved(t *testing.T) {
	_, stderr, err := runFleet(t, "--parent", "HUBC", "--fy-range", "2018-19:2019-20", "--strict")
	if err == nil {
		t.Fatal("--strict must fail when no requested fiscal year was observed")
	}
	if got := ExitCode(err); got != 1 {
		t.Errorf("ExitCode = %d, want 1 (a plain error under --strict)", got)
	}
	for _, want := range []string{"FY2018-19 (published_not_indexed)", "FY2019-20 (published_not_indexed)"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr does not name %s:\n%s", want, stderr)
		}
	}

	stdout, stderr2, err := runFleet(t, "--parent", "HUBC", "--fy-range", "2018-19:2019-20")
	if err != nil {
		t.Fatalf("without --strict the same window must exit 0; got %v", err)
	}
	env := decodeFleet(t, stdout)
	if len(env.Results.FiscalYears) != 2 {
		t.Fatalf("got %d columns, want 2", len(env.Results.FiscalYears))
	}
	for _, c := range env.Results.FiscalYears {
		if c.CoverageClass != "published_not_indexed" {
			t.Errorf("FY%s class = %q, want published_not_indexed", c.FY, c.CoverageClass)
		}
		wantUnmeasured(t, "FY"+c.FY+" capacity_reported", c.CapacityReported)
	}
	// The assertion still fires on stderr with --strict off.
	if !strings.Contains(stderr2, "ASSERTION some_fy_observed FAILED") {
		t.Errorf("assertion text must reach stderr in BOTH modes:\n%s", stderr2)
	}
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

type fleetTestRefusal struct {
	Error struct {
		Kind       string `json:"kind"`
		Token      string `json:"token"`
		Query      string `json:"query"`
		Reason     string `json:"reason"`
		Hazard     string `json:"hazard"`
		Candidates []struct {
			Kind          string `json:"kind"`
			CanonicalName string `json:"canonical_name"`
			ParentName    string `json:"parent_name"`
			PSXTicker     string `json:"psx_ticker"`
			Note          string `json:"note"`
		} `json:"candidates"`
		Recovery           []string `json:"recovery"`
		DeclinedName       string   `json:"declined_name"`
		FuzzyMatching      string   `json:"fuzzy_matching"`
		AbsentParentHint   string   `json:"absent_parent_hint"`
		CounterFact        string   `json:"counter_fact"`
		NearDuplicateGroup []string `json:"near_duplicate_group"`
		KnownSymbols       []string `json:"known_symbols"`
	} `json:"error"`
}

func decodeRefusal(t *testing.T, stdout string) fleetTestRefusal {
	t.Helper()
	var r fleetTestRefusal
	if err := json.Unmarshal([]byte(stdout), &r); err != nil {
		t.Fatalf("stdout is not a refusal envelope: %v\n%s", err, stdout)
	}
	return r
}

// TestFleetRefusesBareKEL: both readings reach the caller, neither is picked.
func TestFleetRefusesBareKEL(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "KEL")
	if err == nil {
		t.Fatal("bare KEL must be refused")
	}
	if got := ExitCode(err); got != 2 {
		t.Errorf("ExitCode = %d, want 2", got)
	}
	r := decodeRefusal(t, stdout)
	if r.Error.Kind != "ambiguous_token" || r.Error.Token != "KEL" {
		t.Errorf("envelope kind/token = %q/%q", r.Error.Kind, r.Error.Token)
	}
	if len(r.Error.Candidates) != 2 {
		t.Fatalf("got %d candidates, want exactly 2", len(r.Error.Candidates))
	}
	if !strings.Contains(r.Error.Hazard, "131 MW RFO plant") {
		t.Errorf("hazard sentence missing: %q", r.Error.Hazard)
	}
	joined := stdout
	for _, want := range []string{"Kohinoor", "KOHE", "K-Electric"} {
		if !strings.Contains(joined, want) {
			t.Errorf("envelope does not mention %q", want)
		}
	}
	for _, want := range []string{
		"nepra-pp-cli fleet --parent KOHE",
		`nepra-pp-cli fleet --plant "Kohinoor Energy Limited. (KEL)"`,
		"nepra-pp-cli fleet --parent KEL --parent-kind psx-symbol",
	} {
		var found bool
		for _, rec := range r.Error.Recovery {
			if rec == want {
				found = true
			}
		}
		if !found {
			t.Errorf("recovery does not offer %q; got %v", want, r.Error.Recovery)
		}
	}
	// A refusal carries no fleet and no capacity.
	if strings.Contains(stdout, "\"plants\"") || strings.Contains(stdout, "capacity_") {
		t.Errorf("refusal leaked a fleet or a capacity figure:\n%s", stdout)
	}
}

// TestFleetRefusesAllSixCuratedTokens: the whole curated refusal set.
func TestFleetRefusesAllSixCuratedTokens(t *testing.T) {
	tokens := []string{"KEL", "AGL", "APL", "SPL", "AEL", "HEPL"}
	for _, tok := range tokens {
		stdout, _, err := runFleet(t, "--parent", tok)
		if err == nil {
			t.Errorf("%s: must be refused, got a result", tok)
			continue
		}
		if got := ExitCode(err); got != 2 {
			t.Errorf("%s: ExitCode = %d, want 2", tok, got)
		}
		r := decodeRefusal(t, stdout)
		if r.Error.Kind != "ambiguous_token" {
			t.Errorf("%s: kind = %q, want ambiguous_token", tok, r.Error.Kind)
		}
		if len(r.Error.Candidates) < 2 {
			t.Errorf("%s: %d candidates, want at least 2", tok, len(r.Error.Candidates))
		}
		if r.Error.Hazard == "" {
			t.Errorf("%s: refusal carries no hazard", tok)
		}
	}
}

// TestFleetKELAsPSXSymbolIsAbsentNotEmpty: an absence is an ANSWER.
func TestFleetKELAsPSXSymbolIsAbsentNotEmpty(t *testing.T) {
	stdout, stderr, err := runFleet(t, "--parent", "KEL", "--parent-kind", "psx-symbol")
	if err != nil {
		t.Fatalf("absent_from_dataset is an answer, not a miss; got %v", err)
	}
	env := decodeFleet(t, stdout)
	r := env.Results.Resolution
	if r.Status != "absent_from_dataset" {
		t.Errorf("status = %q, want absent_from_dataset", r.Status)
	}
	for _, want := range []string{"BQPS", "Korangi", "SITE", "241 published names", "NOT a measurement of zero"} {
		if !strings.Contains(r.AbsenceNote, want) {
			t.Errorf("absence_note missing %q: %q", want, r.AbsenceNote)
		}
	}
	if len(r.ProbeTokens) != 7 {
		t.Errorf("probe_tokens has %d entries, want 7: %v", len(r.ProbeTokens), r.ProbeTokens)
	}
	if !strings.Contains(r.Caveat, "KOHE") {
		t.Errorf("caveat does not point at the reachable Kohinoor plant: %q", r.Caveat)
	}
	// `plants` must be absent or null, NEVER [].
	if strings.Contains(stdout, "\"plants\": []") {
		t.Errorf("emitted an empty plants array, which reads as \"generated nothing\":\n%s", stdout)
	}
	if env.Results.Plants != nil {
		t.Errorf("plants = %v, want nil", env.Results.Plants)
	}
	wantUnmeasured(t, "as_of.capacity_operating", env.Results.AsOf.CapacityOperating)
	wantUnmeasured(t, "as_of.capacity_non_operating", env.Results.AsOf.CapacityNonOperating)
	if strings.Contains(stdout, "\"mw\"") {
		t.Errorf("the absence emitted an mw key:\n%s", stdout)
	}
	if len(env.Results.FiscalYears) != 0 {
		t.Errorf("fiscal_years has %d entries, want 0", len(env.Results.FiscalYears))
	}
	var sawGuard bool
	for _, a := range env.Results.Assertions {
		if a.Name == "absence_is_not_zero" && a.OK {
			sawGuard = true
		}
	}
	if !sawGuard {
		t.Errorf("no passing absence_is_not_zero assertion: %+v", env.Results.Assertions)
	}
	if !strings.Contains(stderr, "ABSENT FROM DATASET") {
		t.Errorf("stderr does not announce the absence:\n%s", stderr)
	}
}

// TestFleetKohinoorReachableByNameAndSymbol: refusing the token must not make
// the plant unreachable.
func TestFleetKohinoorReachableByNameAndSymbol(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"by full published name", []string{"--plant", "Kohinoor Energy Limited. (KEL)"}},
		{"by real PSX symbol", []string{"--parent", "KOHE"}},
	}
	for _, tc := range cases {
		stdout, _, err := runFleet(t, tc.args...)
		if err != nil {
			t.Fatalf("%s: returned %v", tc.name, err)
		}
		env := decodeFleet(t, stdout)
		if len(env.Results.Plants) != 1 {
			t.Fatalf("%s: got %d plants, want 1: %v", tc.name, len(env.Results.Plants), fleetPlantNames(env))
		}
		p := env.Results.Plants[0]
		if p.CanonicalName != "Kohinoor Energy Limited. (KEL)" {
			t.Errorf("%s: canonical_name = %q", tc.name, p.CanonicalName)
		}
		if p.PSXTicker != "KOHE" {
			t.Errorf("%s: psx_ticker = %q, want KOHE", tc.name, p.PSXTicker)
		}
		for fy, wantSNo := range map[string]int{"2017-18": 12, "2023-24": 35} {
			c := fleetCellByFY(t, p, fy)
			if c.SNo == nil || *c.SNo != wantSNo {
				t.Errorf("%s: FY%s sno = %v, want %d", tc.name, fy, c.SNo, wantSNo)
			}
			if c.InstalledCapacity == nil || c.InstalledCapacity.Value == nil || *c.InstalledCapacity.Value != 131 {
				t.Errorf("%s: FY%s capacity = %+v, want 131 MW", tc.name, fy, c.InstalledCapacity)
			}
		}
	}
}

// TestFleetPlantAliasResolvesToOnePlant: the Narowal rename is the join a
// name-only join drops.
func TestFleetPlantAliasResolvesToOnePlant(t *testing.T) {
	cases := []struct {
		query    string
		wantKind string
	}{
		{"Narowal Energy Ltd. (HUBCO)", "alias"},
		{"Narowal Energy Ltd.", "canonical"},
		{"  narowal energy ltd  ", "canonical"},
		{"Narowal\n  Energy Ltd. (HUBCO)", "alias"},
	}
	for _, tc := range cases {
		stdout, _, err := runFleet(t, "--plant", tc.query)
		if err != nil {
			t.Fatalf("%q: returned %v", tc.query, err)
		}
		env := decodeFleet(t, stdout)
		if len(env.Results.Plants) != 1 || env.Results.Plants[0].CanonicalName != "Narowal Energy Ltd." {
			t.Fatalf("%q resolved to %v, want [Narowal Energy Ltd.]", tc.query, fleetPlantNames(env))
		}
		if got := env.Results.Resolution.MatchKind; got != tc.wantKind {
			t.Errorf("%q: match_kind = %q, want %q", tc.query, got, tc.wantKind)
		}
		if got := env.Results.Plants[0].PSXTicker; got != "HUBC" {
			t.Errorf("%q: psx_ticker = %q, want HUBC", tc.query, got)
		}
		if !strings.Contains(env.Results.Resolution.ScopeNote, "--parent HUBC") {
			t.Errorf("%q: scope_note does not point at the wider fleet: %q", tc.query, env.Results.Resolution.ScopeNote)
		}
	}
}

// TestFleetRefusesUnknownAndAmbiguousPlantNames: exit 3, no fuzzy suggestion.
func TestFleetRefusesUnknownAndAmbiguousPlantNames(t *testing.T) {
	for _, q := range []string{
		"Narowul Energy Ltd.",
		"Narowal",
		"(NPPCL)",
		"The Hub Power Company Limited",
		"Kot Addu Power Company Block-I",
		"Foundation Wind Energy-I Ltd.",
	} {
		stdout, _, err := runFleet(t, "--plant", q)
		if err == nil {
			t.Errorf("%q: must be declined", q)
			continue
		}
		if got := ExitCode(err); got != 3 {
			t.Errorf("%q: ExitCode = %d, want 3", q, got)
		}
		r := decodeRefusal(t, stdout)
		if r.Error.Kind != "unknown_plant_name" {
			t.Errorf("%q: kind = %q", q, r.Error.Kind)
		}
		if r.Error.FuzzyMatching != "disabled" {
			t.Errorf("%q: fuzzy_matching = %q, want disabled", q, r.Error.FuzzyMatching)
		}
		if r.Error.DeclinedName == "" {
			t.Errorf("%q: no declined_name echoed", q)
		}
	}

	// The FWEL case names the reviewed near-duplicate group.
	stdout, _, err := runFleet(t, "--plant", "Foundation Wind Energy-I Ltd.")
	if err == nil {
		t.Fatal("FWEL must be declined")
	}
	r := decodeRefusal(t, stdout)
	if len(r.Error.NearDuplicateGroup) != 2 {
		t.Fatalf("FWEL near_duplicate_group = %v, want both Foundation Wind rows", r.Error.NearDuplicateGroup)
	}
	for _, want := range []string{"Foundation Wind Energy-I Ltd. (FWEL-I)", "Foundation Wind Energy-I Ltd. (FWEL-II)"} {
		var found bool
		for _, n := range r.Error.NearDuplicateGroup {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("near_duplicate_group missing %q; got %v", want, r.Error.NearDuplicateGroup)
		}
	}

	// The K-Electric case adds the absence hint AND the Lucky Electric
	// counter-fact, so the hint is not read as "nothing at Bin Qasim".
	stdout, _, err = runFleet(t, "--plant", "Bin Qasim Power Station BQPS-1")
	if err == nil {
		t.Fatal("BQPS must be declined")
	}
	if got := ExitCode(err); got != 3 {
		t.Errorf("BQPS ExitCode = %d, want 3", got)
	}
	r = decodeRefusal(t, stdout)
	if !strings.Contains(r.Error.AbsentParentHint, "K-Electric Limited") {
		t.Errorf("BQPS: no absent-parent hint: %q", r.Error.AbsentParentHint)
	}
	if !strings.Contains(r.Error.AbsentParentHint, "--parent-kind psx-symbol") {
		t.Errorf("BQPS: hint does not name the reachable route: %q", r.Error.AbsentParentHint)
	}
	if !strings.Contains(r.Error.CounterFact, "Lucky Electric Power Company Limited (LEPCL)") {
		t.Errorf("BQPS: missing the Lucky Electric counter-fact: %q", r.Error.CounterFact)
	}
}

// TestFleetUnknownTokenExit3 lists the 15 known symbols rather than guessing.
func TestFleetUnknownTokenExit3(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "OGDC")
	if err == nil {
		t.Fatal("OGDC must be declined")
	}
	if got := ExitCode(err); got != 3 {
		t.Errorf("ExitCode = %d, want 3", got)
	}
	r := decodeRefusal(t, stdout)
	if r.Error.Kind != "unknown_token" {
		t.Errorf("kind = %q, want unknown_token", r.Error.Kind)
	}
	if len(r.Error.KnownSymbols) != 15 {
		t.Errorf("known_symbols has %d entries, want 15: %v", len(r.Error.KnownSymbols), r.Error.KnownSymbols)
	}
	for _, want := range []string{"HUBC", "KAPCO", "KOHE", "TICL"} {
		if !strings.Contains(strings.Join(r.Error.KnownSymbols, ","), want) {
			t.Errorf("known_symbols missing %q", want)
		}
	}
	if !strings.Contains(r.Error.Reason, "not a claim the token is invalid on the PSX") {
		t.Errorf("reason overclaims: %q", r.Error.Reason)
	}
}

// TestFleetAcronymScopeIsNotTheFleet: an acronym reaches its plants, not the
// parent's fleet, and says so.
func TestFleetAcronymScopeIsNotTheFleet(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "TEL")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	r := env.Results.Resolution
	if r.TokenKind != "nepra_acronym" {
		t.Errorf("token_kind = %q, want nepra_acronym", r.TokenKind)
	}
	if r.PSXTicker != "HUBC" {
		t.Errorf("psx_ticker = %q, want HUBC", r.PSXTicker)
	}
	if len(env.Results.Plants) != 1 || env.Results.Plants[0].CanonicalName != "Thar Energy Limited (TEL)" {
		t.Fatalf("plants = %v, want exactly [Thar Energy Limited (TEL)]", fleetPlantNames(env))
	}
	if !strings.Contains(r.ScopeNote, "has 6 in this crosswalk") {
		t.Errorf("scope_note does not say the parent fleet is 6: %q", r.ScopeNote)
	}
	if !strings.Contains(r.ScopeNote, "--parent HUBC") {
		t.Errorf("scope_note does not name the wider command: %q", r.ScopeNote)
	}
	// The acronym panel's as-of is drawn from Coverage, not PlantsForParent,
	// so it must describe only this one plant.
	wantMeasured(t, "as_of.capacity_operating", env.Results.AsOf.CapacityOperating, 330, 1)
}

// ---------------------------------------------------------------------------
// S.No, assertions, catalogue, modes
// ---------------------------------------------------------------------------

// TestFleetSNoIsNeverAKey: S.No is reported per fiscal year and moves.
func TestFleetSNoIsNeverAKey(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "HUBC")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	if !strings.Contains(env.Meta.Crosswalk.SNoNote, "ZERO keep the same S.No") {
		t.Errorf("meta.sno_note missing: %q", env.Meta.Crosswalk.SNoNote)
	}
	if !strings.Contains(env.Meta.Crosswalk.SNoNote, "107 plant names common") {
		t.Errorf("meta.sno_note does not carry the measured 107: %q", env.Meta.Crosswalk.SNoNote)
	}
	// Measured moves for two named plants.
	for name, want := range map[string][2]int{
		"Hub Power Company (HUBCO)": {10, 34},
		"Narowal Energy Ltd.":       {11, 58},
	} {
		p := fleetPlantByName(t, env, name)
		a := fleetCellByFY(t, p, "2017-18")
		b := fleetCellByFY(t, p, "2023-24")
		if a.SNo == nil || b.SNo == nil {
			t.Fatalf("%s: a per-fiscal-year sno is missing", name)
		}
		if *a.SNo != want[0] || *b.SNo != want[1] {
			t.Errorf("%s: S.No %d->%d, want %d->%d", name, *a.SNo, *b.SNo, want[0], want[1])
		}
	}
	// And the panel's own runtime recount agrees that nothing is stable.
	var sno struct{ ok bool }
	for _, a := range env.Results.Assertions {
		if a.Name != "sno_is_not_a_key" {
			continue
		}
		sno.ok = a.OK
		if !strings.Contains(a.Detail, "0 keep their S.No") {
			t.Errorf("sno_is_not_a_key detail = %q, want a recount showing 0 stable", a.Detail)
		}
		if !strings.Contains(a.Detail, "Hub Power Company (HUBCO) 10->34") {
			t.Errorf("sno_is_not_a_key detail does not name the measured move: %q", a.Detail)
		}
	}
	if !sno.ok {
		t.Error("no passing sno_is_not_a_key assertion")
	}
}

// TestFleetAssertionsPass pins the assertion set and its cross-check detail.
func TestFleetAssertionsPass(t *testing.T) {
	stdout, stderr, err := runFleet(t, "--parent", "HUBC", "--strict")
	if err != nil {
		t.Fatalf("--strict on a clean panel must exit 0; got %v", err)
	}
	env := decodeFleet(t, stdout)
	want := map[string]bool{
		"crosswalk_schema":    false,
		"as_of_cross_check":   false,
		"fy_population_floor": false,
		"some_fy_observed":    false,
		"sno_is_not_a_key":    false,
	}
	for _, a := range env.Results.Assertions {
		if _, ok := want[a.Name]; !ok {
			t.Errorf("unexpected assertion %q", a.Name)
			continue
		}
		want[a.Name] = true
		if !a.OK {
			t.Errorf("assertion %q failed: %s", a.Name, a.Detail)
		}
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("assertion %q is missing", name)
		}
	}
	if stderr != "" {
		t.Errorf("a clean panel wrote to stderr:\n%s", stderr)
	}
	// The cross-check must actually name both independent computations.
	for _, a := range env.Results.Assertions {
		if a.Name != "as_of_cross_check" {
			continue
		}
		for _, want := range []string{"PlantsForParent(\"HUBC\")", "Coverage(\"2023-24\", fleet)", "2289.00 MW over 5 plants", "1292.00 MW over 1 plants"} {
			if !strings.Contains(a.Detail, want) {
				t.Errorf("as_of_cross_check detail missing %q: %q", want, a.Detail)
			}
		}
	}
	if len(env.Results.DeclaredGaps) != 4 {
		t.Errorf("declared_gaps has %d entries, want 4", len(env.Results.DeclaredGaps))
	}
	var sawLoadFactor bool
	for _, g := range env.Results.DeclaredGaps {
		if strings.Contains(g.Question, "load factor") {
			sawLoadFactor = true
			if g.Verdict != "out_of_scope" {
				t.Errorf("load-factor gap verdict = %q", g.Verdict)
			}
		}
	}
	if !sawLoadFactor {
		t.Error("declared_gaps does not declare that load factor is not computable")
	}
}

// TestFleetCatalogueMakesNoRequest: the no-selector branch is offline and
// enumerates everything the command can be pointed at.
func TestFleetCatalogueMakesNoRequest(t *testing.T) {
	// A closed-port base URL proves no HTTP call is attempted: any request
	// would fail loudly instead of returning the catalogue.
	t.Setenv("NEPRA_BASE_URL", "http://127.0.0.1:1")
	stdout, _, err := runFleet(t)
	if err != nil {
		t.Fatalf("bare fleet must exit 0; got %v", err)
	}
	var env struct {
		Meta struct {
			Source    string `json:"source"`
			Crosswalk struct {
				Rows         int      `json:"rows"`
				ObservedFYs  []string `json:"observed_fys"`
				HeldOutFYs   []string `json:"held_out_fys"`
				PublishedFYs []string `json:"published_fys"`
			} `json:"crosswalk"`
		} `json:"meta"`
		Results struct {
			Operators []struct {
				PSXTicker            string      `json:"psx_ticker"`
				ParentName           string      `json:"parent_name"`
				Plants               int         `json:"plants"`
				CapacityOperating    fleetTestMW `json:"capacity_operating"`
				CapacityNonOperating fleetTestMW `json:"capacity_non_operating"`
			} `json:"operators"`
			RefusedTokens []struct {
				Token  string `json:"token"`
				Hazard string `json:"hazard"`
			} `json:"refused_tokens"`
			AbsentParents []struct {
				PSXTicker string `json:"psx_ticker"`
				Reach     string `json:"reach"`
			} `json:"absent_parents"`
			DeclaredGaps []struct {
				Question string `json:"question"`
			} `json:"declared_gaps"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("catalogue is not JSON: %v\n%s", err, stdout)
	}
	if env.Meta.Source != "computed" {
		t.Errorf("meta.source = %q, want computed", env.Meta.Source)
	}
	if env.Meta.Crosswalk.Rows != 133 {
		t.Errorf("rows = %d, want 133", env.Meta.Crosswalk.Rows)
	}
	if len(env.Meta.Crosswalk.ObservedFYs) != 2 || len(env.Meta.Crosswalk.HeldOutFYs) != 1 || len(env.Meta.Crosswalk.PublishedFYs) != 7 {
		t.Errorf("fiscal-year lists = observed %v, held out %v, published %v",
			env.Meta.Crosswalk.ObservedFYs, env.Meta.Crosswalk.HeldOutFYs, env.Meta.Crosswalk.PublishedFYs)
	}
	if len(env.Results.Operators) != 15 {
		t.Errorf("catalogue lists %d operators, want 15", len(env.Results.Operators))
	}
	if len(env.Results.RefusedTokens) != 6 {
		t.Errorf("catalogue lists %d refused tokens, want 6", len(env.Results.RefusedTokens))
	}
	if len(env.Results.AbsentParents) != 1 {
		t.Errorf("catalogue lists %d absent parents, want 1", len(env.Results.AbsentParents))
	}
	if len(env.Results.DeclaredGaps) != 4 {
		t.Errorf("catalogue declares %d gaps, want 4", len(env.Results.DeclaredGaps))
	}
	// HUBC's row in the catalogue keeps the two sums apart.
	var sawHUBC bool
	for _, o := range env.Results.Operators {
		if o.PSXTicker != "HUBC" {
			continue
		}
		sawHUBC = true
		if o.Plants != 6 {
			t.Errorf("catalogue HUBC plants = %d, want 6", o.Plants)
		}
		wantMeasured(t, "catalogue HUBC capacity_operating", o.CapacityOperating, 2289, 5)
		wantMeasured(t, "catalogue HUBC capacity_non_operating", o.CapacityNonOperating, 1292, 1)
	}
	if !sawHUBC {
		t.Error("catalogue has no HUBC row")
	}
	for _, a := range env.Results.AbsentParents {
		if a.PSXTicker == "KEL" && !strings.Contains(a.Reach, "--parent-kind psx-symbol") {
			t.Errorf("KEL's catalogue entry does not name the reachable route: %q", a.Reach)
		}
	}
}

// TestFleetDryRunAndAgentModes.
func TestFleetDryRunAndAgentModes(t *testing.T) {
	flags := &rootFlags{asJSON: true, dryRun: true}
	cmd := newNepraFleetCmd(flags)
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--parent", "HUBC"})
	silenceFleetUsage(cmd)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--dry-run returned %v", err)
	}
	var dr dryRunResult
	if err := json.Unmarshal(out.Bytes(), &dr); err != nil {
		t.Fatalf("--dry-run stdout is not the dryRunResult envelope: %v\n%s", err, out.String())
	}
	if !dr.DryRun || dr.Action != "fleet" {
		t.Errorf("dry-run envelope = %+v, want dry_run=true action=fleet", dr)
	}
	if strings.Contains(out.String(), "capacity") {
		t.Errorf("--dry-run emitted panel data:\n%s", out.String())
	}

	// --agent (asJSON) keeps stdout parseable and confines notes to stderr.
	stdout, _, err := runFleet(t, "--parent", "HUBC", "--fy-range", "2020-21:2023-24")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	if !json.Valid([]byte(stdout)) {
		t.Fatalf("stdout is not valid JSON:\n%s", stdout)
	}
	if strings.Contains(stdout, "UNMEASURED:") || strings.Contains(stdout, "DO NOT ADD:") {
		t.Errorf("human notes leaked onto stdout:\n%s", stdout)
	}
}

// TestFleetUsageErrors: every bad input exits 2.
func TestFleetUsageErrors(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		flags *rootFlags
		want  string
	}{
		{"parent and plant together", []string{"--parent", "HUBC", "--plant", "Kotri Power Station"}, nil, "mutually exclusive"},
		{"parent-kind without parent", []string{"--parent-kind", "psx-symbol"}, nil, "only applies to --parent"},
		{"parent-kind bogus", []string{"--parent", "HUBC", "--parent-kind", "nepra-acronym"}, nil, "is not valid"},
		{"fy-range no colon", []string{"--parent", "HUBC", "--fy-range", "2023-24"}, nil, "must be exactly <FY>:<FY>"},
		{"fy-range inverted", []string{"--parent", "HUBC", "--fy-range", "2023-24:2017-18"}, nil, "inverted"},
		{"fy-range two-year half", []string{"--parent", "HUBC", "--fy-range", "2017-2019:2023-24"}, nil, "spans 2 years"},
		{"fy-range too wide", []string{"--parent", "HUBC", "--fy-range", "2000-01:2050-51"}, nil, "maximum is 20"},
		{"fy-range only", []string{"--fy-range", "2017-18:2023-24"}, nil, "no selector"},
		{"data-source live", []string{"--parent", "HUBC"}, &rootFlags{asJSON: true, dataSource: "live"}, "no live equivalent"},
	}
	for _, tc := range cases {
		flags := tc.flags
		if flags == nil {
			flags = &rootFlags{asJSON: true}
		}
		cmd := newNepraFleetCmd(flags)
		var out, errBuf bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		cmd.SetArgs(tc.args)
		silenceFleetUsage(cmd)
		err := cmd.Execute()
		if err == nil {
			t.Errorf("%s: expected a usage error", tc.name)
			continue
		}
		if got := ExitCode(err); got != 2 {
			t.Errorf("%s: ExitCode = %d, want 2 (%v)", tc.name, got, err)
		}
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error = %q, want it to mention %q", tc.name, err.Error(), tc.want)
		}
	}
}

// TestFleetSinglePlantKeepsCapacityUnderAStatusSentinel: Kotri keeps 174 MW in
// both years while its FY2023-24 block reads DELICENSED.
func TestFleetSinglePlantKeepsCapacityUnderAStatusSentinel(t *testing.T) {
	stdout, _, err := runFleet(t, "--plant", "Kotri Power Station")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	if len(env.Results.Plants) != 1 {
		t.Fatalf("got %d plants, want 1", len(env.Results.Plants))
	}
	p := env.Results.Plants[0]
	if p.PSXTicker != "" {
		t.Errorf("psx_ticker = %q, want empty", p.PSXTicker)
	}
	for fy, want := range map[string]struct {
		sno    int
		status string
	}{
		"2017-18": {99, "numeric"},
		"2023-24": {14, "delicensed"},
	} {
		c := fleetCellByFY(t, p, fy)
		if c.SNo == nil || *c.SNo != want.sno {
			t.Errorf("Kotri FY%s sno = %v, want %d", fy, c.SNo, want.sno)
		}
		if c.Status != want.status {
			t.Errorf("Kotri FY%s status = %q, want %q", fy, c.Status, want.status)
		}
		// A non-operating plant KEEPS its published capacity.
		if c.InstalledCapacity == nil || c.InstalledCapacity.Value == nil || *c.InstalledCapacity.Value != 174 {
			t.Errorf("Kotri FY%s capacity = %+v, want 174 MW", fy, c.InstalledCapacity)
		}
	}
	// As of FY2023-24 the whole selection is non-operating, so live capacity is
	// a MEASURED zero over zero plants and the 174 MW sits in the other line.
	wantMeasured(t, "as_of.capacity_operating", env.Results.AsOf.CapacityOperating, 0, 0)
	wantMeasured(t, "as_of.capacity_non_operating", env.Results.AsOf.CapacityNonOperating, 174, 1)
	if env.Results.AsOf.OperatingPlants != 0 || env.Results.AsOf.NonOperatingPlants != 1 {
		t.Errorf("as_of partition = %d operating / %d non-operating, want 0 / 1",
			env.Results.AsOf.OperatingPlants, env.Results.AsOf.NonOperatingPlants)
	}
}
