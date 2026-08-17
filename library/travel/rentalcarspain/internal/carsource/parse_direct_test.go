// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

// Drivalia's request must carry meta.language, or the API returns a wrong fleet
// with fallback model names (NBMR -> the literal "acriss", EBMR -> "Lancia Y").
// This guards against the meta block being dropped from the request envelope.
func TestDrivaliaMetaLanguage(t *testing.T) {
	m := drivaliaMeta()
	if m["language"] != "es-ES" {
		t.Errorf("drivaliaMeta language = %v, want es-ES", m["language"])
	}
	if m["frontendID"] == "" || m["frontendID"] == nil {
		t.Error("drivaliaMeta must set frontendID")
	}
}

// Drivalia's enriched-offer lists the zero-excess C1 cover plus a mandatory
// young/senior-driver surcharge (charged in the online total). Ancillaries repeat
// per productCode, so drivaliaPricing must dedupe by code before summing.
// Verified: FIAT 500, 7 days, under-24 → C1 €174.93 + YOUNG DRIVER €83.65.
func TestDrivaliaPricing(t *testing.T) {
	anc := func(code, typ string, mandatory bool, cents int64) drivaliaAncillary {
		a := drivaliaAncillary{Code: code, Type: typ, IsMandatory: mandatory}
		a.Price.Value = cents
		return a
	}
	// Young driver: C1 + mandatory D2, each duplicated per productCode.
	young := []drivaliaAncillary{
		anc("C1", "Insurance", false, 17493),
		anc("C1", "Insurance", false, 17493),
		anc("D2", "Service", true, 8365),  // YOUNG DRIVER
		anc("D2", "Service", true, 8365),  // duplicate productCode
		anc("D3", "Service", false, 8365), // SENIOR DRIVER, not applicable
		anc("D23", "Service", false, 7700),
	}
	c1, extra := drivaliaPricing(young)
	if c1 != 17493 {
		t.Errorf("C1 = %d, want 17493 (deduped)", c1)
	}
	if extra != 8365 {
		t.Errorf("mandatory extra = %d, want 8365 (D2 counted once, not doubled)", extra)
	}

	// Standard age: nothing mandatory → only C1, no extra.
	standard := []drivaliaAncillary{
		anc("C1", "Insurance", false, 17493),
		anc("D2", "Service", false, 8365),
		anc("D3", "Service", false, 8365),
	}
	if c1, extra := drivaliaPricing(standard); c1 != 17493 || extra != 0 {
		t.Errorf("standard pricing = (c1 %d, extra %d), want (17493, 0)", c1, extra)
	}

	// No C1 present → c1 = -1 so the caller drops the offer.
	if c1, _ := drivaliaPricing([]drivaliaAncillary{anc("D2", "Service", true, 8365)}); c1 != -1 {
		t.Errorf("missing C1 should yield c1 = -1, got %d", c1)
	}
}

// Delpaso does not price driver age online, so its young-driver surcharge is
// computed from the published rule: €12/day, min €36, max €100, ages 21–24.
// Standard-age and unspecified drivers pay nothing extra.
func TestDelpasoYoungDriverSurcharge(t *testing.T) {
	cases := []struct {
		name       string
		age, days  int
		want       float64
	}{
		{"standard age, no surcharge", 35, 7, 0},
		{"unspecified age, no surcharge", 0, 7, 0},
		{"exactly 25 is standard", 25, 7, 0},
		{"young, 7 days = 12x7 = 84", 22, 7, 84},
		{"young, 2 days hits the 36 floor", 23, 2, 36},
		{"young, 3 days = 36 (at floor)", 24, 3, 36},
		{"young, 30 days hits the 100 cap", 21, 30, 100},
		{"young, zero days floors to 1 day then min", 22, 0, 36},
	}
	for _, c := range cases {
		if got := delpasoYoungDriverSurcharge(c.age, c.days); got != c.want {
			t.Errorf("%s: delpasoYoungDriverSurcharge(%d,%d) = %v, want %v", c.name, c.age, c.days, got, c.want)
		}
	}
}

func TestClickrentContainsItem(t *testing.T) {
	// A "full" rate: pack (id 262) nesting the zero-excess item (291).
	full := []clickrentItem{
		{ID: 290},
		{ID: 262, IsPack: true, ItemsPack: []clickrentItem{
			{ID: 241},
			{ID: 291}, // ¡Sin Franquicia!
		}},
	}
	economy := []clickrentItem{{ID: 540}, {ID: 290}}

	if !clickrentContainsItem(full, clickrentZeroExcessItem) {
		t.Errorf("full rate should contain zero-excess item %d", clickrentZeroExcessItem)
	}
	if clickrentContainsItem(economy, clickrentZeroExcessItem) {
		t.Error("economy rate should not contain the zero-excess item")
	}
}

func TestGoldcarZeroExcess(t *testing.T) {
	tarifas := map[string]goldcarTarifa{
		"FullFuelSDC":    {PrecioTotalf: 156.14},
		"TNRFullFuelSDC": {PrecioTotalf: 150.00},
		"PackKeyngo":     {PrecioTotalf: 335.56, TieneCobertura: true},
		"PackPrime":      {PrecioTotalf: 277.00, TieneCobertura: true},
	}
	price, rate := goldcarZeroExcess(tarifas)
	if rate != "PackPrime" {
		t.Errorf("expected cheapest Pack* tariff PackPrime, got %q", rate)
	}
	if price != 277.00 {
		t.Errorf("expected 277.00, got %v", price)
	}

	// No zero-excess pack → 0.
	only := map[string]goldcarTarifa{"FullFuelSDC": {PrecioTotalf: 156.14}}
	if p, _ := goldcarZeroExcess(only); p != 0 {
		t.Errorf("expected 0 when no Pack* tariff, got %v", p)
	}
}

func TestCicarCarRe(t *testing.T) {
	html := `
    <p class='precioModelo grupo_A MLTP0'></p><span class="precioSinFormato" style="display:none">182,17</span>
    <span class="idModeloSeleccionado" style="display:none">F54RDBA</span>
    <span class="grupoSeleccionado" style="display:none">A</span>
    <span class="nombreModeloSeleccionado" style="display:none">Fiat 500</span>
    ...next card...
    <span class="precioSinFormato" style="display:none">259,77</span>
    <span class="idModeloSeleccionado" style="display:none">P20085GA</span>
    <span class="grupoSeleccionado" style="display:none">F</span>
    <span class="nombreModeloSeleccionado" style="display:none">Peugeot 2008 AUT.</span>`
	m := cicarCarRe.FindAllStringSubmatch(html, -1)
	if len(m) != 2 {
		t.Fatalf("expected 2 cars, got %d", len(m))
	}
	if got := parsePrice(m[0][1]); got != 182.17 {
		t.Errorf("price[0] = %v, want 182.17", got)
	}
	if m[0][4] != "Fiat 500" {
		t.Errorf("name[0] = %q, want Fiat 500", m[0][4])
	}
	if m[1][3] != "F" || m[1][4] != "Peugeot 2008 AUT." {
		t.Errorf("car[1] parsed wrong: group=%q name=%q", m[1][3], m[1][4])
	}
}
