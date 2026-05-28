// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/alaska-airlines/internal/valuation"
)

func TestBuildCashSearchParams_AlwaysSetsCorePieces(t *testing.T) {
	p := buildCashSearchParams(cashSearchInput{
		Origin:      "FCO",
		Destination: "SEA",
		Depart:      "2026-08-30",
		Adults:      "1",
		RoundTrip:   "false",
		Locale:      "en-us",
	})
	if p["O"] != "FCO" || p["D"] != "SEA" || p["OD"] != "2026-08-30" {
		t.Errorf("core fields missing: %+v", p)
	}
	if p["A"] != "1" {
		t.Errorf("adults missing: %+v", p)
	}
	if p["RT"] != "false" {
		t.Errorf("RT missing: %+v", p)
	}
	if p["locale"] != "en-us" {
		t.Errorf("locale missing: %+v", p)
	}
}

func TestBuildCashSearchParams_OmitsAwardKeys(t *testing.T) {
	// Cash search must NOT carry the award toggles. If these leak, the
	// SvelteKit endpoint flips into award mode and the cash fare is
	// reported as taxes-on-an-award.
	p := buildCashSearchParams(cashSearchInput{Origin: "FCO", Destination: "SEA", Depart: "2026-08-30"})
	forbidden := []string{"ShoppingMethod", "UPG", "OT", "DT", "SpecFare"}
	for _, k := range forbidden {
		if _, ok := p[k]; ok {
			t.Errorf("cash params unexpectedly carries %q = %q", k, p[k])
		}
	}
}

func TestBuildCashSearchParams_RoundTripOmitsReturn(t *testing.T) {
	p := buildCashSearchParams(cashSearchInput{Origin: "FCO", Destination: "SEA", Depart: "2026-08-30"})
	if _, ok := p["DD"]; ok {
		t.Errorf("one-way should not set DD; got %q", p["DD"])
	}
}

func TestBuildCashSearchParams_RoundTripSetsReturn(t *testing.T) {
	p := buildCashSearchParams(cashSearchInput{
		Origin:      "FCO",
		Destination: "SEA",
		Depart:      "2026-08-30",
		Return:      "2026-09-06",
		RoundTrip:   "true",
	})
	if p["DD"] != "2026-09-06" {
		t.Errorf("DD = %q; want 2026-09-06", p["DD"])
	}
	if p["RT"] != "true" {
		t.Errorf("RT = %q; want true", p["RT"])
	}
}

func TestBuildValueCompareEnvelope_HappyPath(t *testing.T) {
	// FCO -> SEA Aug 30: cash $1766.23, award 30000 + $64.53, cpp 1.4.
	// effective_cpp ~= 5.67, multiple ~= 4.05, tpg_valued_usd ~= 484.53.
	miles := 30000
	in := valueCompareInputs{
		Origin: "FCO", Destination: "SEA", Depart: "2026-08-30", Cabin: "economy",
		Adults: "1", Children: "0", MaxStops: -1, RoundTrip: false,
		Program:   valuation.ProgramAtmos,
		CashFare:  lowestFare{CashUSD: 1766.23, Carrier: "AS", Cabin: "MAIN", Stops: 0},
		AwardFare: lowestFare{Miles: &miles, CashUSD: 64.53, Carrier: "AS", Cabin: "REFUNDABLE_MAIN", Stops: 0},
		Valuation: valuation.Result{CPPCents: 1.4, Source: valuation.SourceTPGLive, FetchedAt: time.Date(2026, 5, 20, 0, 0, 0, 0, time.UTC)},
	}
	env := buildValueCompareEnvelope(in)

	meta := env["meta"].(map[string]any)
	if meta["cpp_baseline_cents"].(float64) != 1.4 {
		t.Errorf("cpp_baseline_cents = %v; want 1.4", meta["cpp_baseline_cents"])
	}
	if meta["cpp_baseline_source"].(string) != valuation.SourceTPGLive {
		t.Errorf("cpp_baseline_source = %q; want %q", meta["cpp_baseline_source"], valuation.SourceTPGLive)
	}

	results := env["results"].(map[string]any)
	cash := results["cash"].(map[string]any)
	if cash["price_usd"].(float64) != 1766.23 {
		t.Errorf("cash.price_usd = %v; want 1766.23", cash["price_usd"])
	}
	award := results["award"].(map[string]any)
	if award["miles"].(int) != 30000 {
		t.Errorf("award.miles = %v; want 30000", award["miles"])
	}
	if award["taxes_usd"].(float64) != 64.53 {
		t.Errorf("award.taxes_usd = %v; want 64.53", award["taxes_usd"])
	}

	comp := results["comparison"].(map[string]any)
	eff := comp["effective_cpp_cents"].(float64)
	if eff < 5.66 || eff > 5.68 {
		t.Errorf("effective_cpp_cents = %v; want ~5.67", eff)
	}
	mult := comp["multiple"].(float64)
	if mult < 4.04 || mult > 4.06 {
		t.Errorf("multiple = %v; want ~4.05", mult)
	}
	tpgUSD := comp["tpg_valued_usd"].(float64)
	if tpgUSD < 484.50 || tpgUSD > 484.55 {
		t.Errorf("tpg_valued_usd = %v; want ~484.53", tpgUSD)
	}
}

func TestBuildValueCompareEnvelope_NoAwardInventory(t *testing.T) {
	in := valueCompareInputs{
		Origin: "FCO", Destination: "SEA", Depart: "2026-08-30",
		Program:   valuation.ProgramAtmos,
		CashFare:  lowestFare{CashUSD: 1766.23, Cabin: "MAIN", Carrier: "AS"},
		AwardFare: lowestFare{}, // no inventory
		Valuation: valuation.Result{CPPCents: 1.4, Source: valuation.SourceTPGCached},
	}
	env := buildValueCompareEnvelope(in)
	results := env["results"].(map[string]any)
	if results["award"] != nil {
		t.Errorf("award = %v; want nil when no inventory", results["award"])
	}
	if results["comparison"] != nil {
		t.Errorf("comparison = %v; want nil when award missing", results["comparison"])
	}
	meta := env["meta"].(map[string]any)
	notes, _ := meta["notes"].([]string)
	if len(notes) == 0 || !strings.Contains(strings.Join(notes, " "), "award inventory") {
		t.Errorf("expected a notes entry about award inventory; got %v", notes)
	}
}

func TestBuildValueCompareEnvelope_NoCash(t *testing.T) {
	miles := 30000
	in := valueCompareInputs{
		Origin: "FCO", Destination: "SEA", Depart: "2026-08-30",
		Program:   valuation.ProgramAtmos,
		CashFare:  lowestFare{}, // no cash
		AwardFare: lowestFare{Miles: &miles, CashUSD: 64.53},
		Valuation: valuation.Result{CPPCents: 1.4, Source: valuation.SourceTPGLive},
	}
	env := buildValueCompareEnvelope(in)
	results := env["results"].(map[string]any)
	if results["cash"] != nil {
		t.Errorf("cash = %v; want nil", results["cash"])
	}
	if results["comparison"] != nil {
		t.Errorf("comparison = %v; want nil when cash missing", results["comparison"])
	}
}

func TestBuildValueCompareEnvelope_OverrideSourceSurfaces(t *testing.T) {
	miles := 30000
	in := valueCompareInputs{
		Program:   valuation.ProgramAtmos,
		CashFare:  lowestFare{CashUSD: 1000},
		AwardFare: lowestFare{Miles: &miles, CashUSD: 50},
		Valuation: valuation.Result{CPPCents: 2.0, Source: valuation.SourceOverride, FetchedAt: time.Now()},
	}
	env := buildValueCompareEnvelope(in)
	meta := env["meta"].(map[string]any)
	if meta["cpp_baseline_source"].(string) != valuation.SourceOverride {
		t.Errorf("source = %q; want %q", meta["cpp_baseline_source"], valuation.SourceOverride)
	}
	if meta["cpp_baseline_cents"].(float64) != 2.0 {
		t.Errorf("cpp_baseline_cents = %v; want 2.0", meta["cpp_baseline_cents"])
	}
}

func TestBuildValueCompareEnvelope_WarningSurfaces(t *testing.T) {
	miles := 30000
	in := valueCompareInputs{
		Program:   valuation.ProgramAtmos,
		CashFare:  lowestFare{CashUSD: 1000},
		AwardFare: lowestFare{Miles: &miles, CashUSD: 50},
		Valuation: valuation.Result{
			CPPCents:  1.4,
			Source:    valuation.SourceFallbackConst,
			FetchedAt: time.Now(),
			Warning:   errors.New("simulated cloudflare block"),
		},
	}
	env := buildValueCompareEnvelope(in)
	meta := env["meta"].(map[string]any)
	if meta["valuation_warning"].(string) != "simulated cloudflare block" {
		t.Errorf("valuation_warning = %q; want %q", meta["valuation_warning"], "simulated cloudflare block")
	}
}

func TestPrintValueCompareDryRun_StructureValid(t *testing.T) {
	var buf bytes.Buffer
	err := printValueCompareDryRunTo(&buf, "FCO", "SEA", "2026-08-30", "", "economy", valuation.ProgramAtmos)
	if err != nil {
		t.Fatalf("dry-run err = %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal(buf.Bytes(), &preview); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, buf.String())
	}
	if preview["dry_run"] != true {
		t.Errorf("dry_run = %v; want true", preview["dry_run"])
	}
	if preview["program"] != string(valuation.ProgramAtmos) {
		t.Errorf("program = %v; want %v", preview["program"], valuation.ProgramAtmos)
	}
	calls, _ := preview["calls_to_make"].([]any)
	if len(calls) != 3 {
		t.Errorf("calls_to_make len = %d; want 3", len(calls))
	}
}
