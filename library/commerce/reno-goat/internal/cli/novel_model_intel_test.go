package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/reno-goat/internal/cliutil"
)

func TestSearchResultOffersFromBraveExactModel(t *testing.T) {
	if price := likelySearchSnippetPrice("Price &nbsp; $3,999.00 · JOESC330RM"); price != 3999 {
		t.Fatalf("snippet price parse failed: got %v", price)
	}
	if price := likelySearchSnippetPrice("Retail Price: $5,049.00 · Today's Price :$4,599.00 · Qualified Builders: -$459.90"); price != 4599 {
		t.Fatalf("discount-safe snippet price parse failed: got %v", price)
	}
	body := `title:"JennAir NOIR Single Wall Oven JOESC330RM",url:"https://www.abt.com/JennAir-NOIR-Single-Wall-Oven-With-MultiMode-30-Inch-Wide-in-Stainless-Steel-JOESC330RM/p/220958.html",description:"Price &nbsp; $3,999.00 · JOESC330RM · Stainless Steel"
title:"Jenn Air JOESC330RM | Plesser's Appliances",url:"https://www.plessers.com/jenn-air/joesc330rm",description:"\u003Cstrong>JOESC330RM\u003C/strong> · Retail Price: $5,049.00 · Today&#x27;s Price :$4,599.00 · Qualified Builders: -$459.90"`

	offers := searchResultOffersFromBrave("JOESC330RM", body)
	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d: %#v", len(offers), offers)
	}
	if offers[0].Source != "brave-search:abt.com" || offers[0].Price != 3999 {
		t.Fatalf("unexpected first offer: %#v", offers[0])
	}
	if offers[1].Source != "brave-search:plessers.com" || offers[1].Price != 4599 {
		t.Fatalf("unexpected second offer: %#v", offers[1])
	}
}

func TestSearchResultOffersFromBraveRequiresExactModel(t *testing.T) {
	body := `title:"JennAir NOIR Single Wall Oven JOESC330RL",url:"https://www.plessers.com/jenn-air/joesc330rl",description:"JOESC330RL · Price $3,899.00"`

	offers := searchResultOffersFromBrave("JOESC330RM", body)
	if len(offers) != 0 {
		t.Fatalf("expected no offers for a different model, got %#v", offers)
	}
}

func TestSearchResultOffersFromBraveRejectsSiblingModelURL(t *testing.T) {
	body := `title:"Jenn Air JOESC330RM 30 Inch Single Convection Electric Smart Wall Oven",url:"https://www.plessers.com/jenn-air/joesc330rl",description:"\u003Cstrong>JOESC330RM\u003C/strong> · Today's Price :$4,599.00"`

	offers := searchResultOffersFromBrave("JOESC330RM", body)
	if len(offers) != 0 {
		t.Fatalf("expected sibling model URL to be rejected, got %#v", offers)
	}
}

func TestSearchResultOffersFromBraveProductPriceField(t *testing.T) {
	body := `title:"JennAir NOIR Single Wall Oven JOESC330RM",url:"https://www.abt.com/JennAir-NOIR-Single-Wall-Oven-With-MultiMode-30-Inch-Wide-in-Stainless-Steel-JOESC330RM/p/220958.html",description:"Experience culinary mastery",product:{type:"Product",name:"JennAir NOIR Single Wall Oven JOESC330RM",url:"https://www.abt.com/JennAir-NOIR-Single-Wall-Oven-With-MultiMode-30-Inch-Wide-in-Stainless-Steel-JOESC330RM/p/220958.html",price:"4299.0",offers:[{priceCurrency:"USD",price:"4299.0"}]}
title:"JennAir NOIR JOESC330RM",url:"https://www.ajmadison.com/cgi-bin/ajmadison/JOESC330RM.html",description:"on DCS grill packages over $10,000 · on Bosch kitchen packages"`

	offers := searchResultOffersFromBrave("JOESC330RM", body)
	if len(offers) != 1 {
		t.Fatalf("expected 1 product-price offer, got %#v", offers)
	}
	if offers[0].Source != "brave-search:abt.com" || offers[0].Price != 4299 {
		t.Fatalf("unexpected product-price offer: %#v", offers[0])
	}
}

func TestCleanSearchResultTitleTrimsCapturedMetadata(t *testing.T) {
	title := cleanSearchResultTitle(`Jenn Air JOESC330RM Wall Oven",description:"Save on the JennAir JOESC330RM",page_age:void 0`)
	if title != "Jenn Air JOESC330RM Wall Oven" {
		t.Fatalf("unexpected cleaned title: %q", title)
	}
}

func TestResolveModelIntelSourcesAutoInfersInstalledSelectionCategories(t *testing.T) {
	sources, categories, room, err := resolveModelIntelSources("recessed light trim", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve electrical auto sources: %v", err)
	}
	if room != "" {
		t.Fatalf("expected no room, got %q", room)
	}
	if !containsString(categories, "electrical") {
		t.Fatalf("expected electrical category, got %#v", categories)
	}
	if !containsString(sources, "superbrightleds") || !containsString(sources, "prolighting") || !containsString(sources, "1000bulbs") {
		t.Fatalf("expected electrical sources, got %#v", sources)
	}

	_, categories, _, err = resolveModelIntelSources("ceiling fan", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve ceiling fan auto sources: %v", err)
	}
	if !containsString(categories, "electrical") {
		t.Fatalf("expected ceiling fan to infer electrical category, got %#v", categories)
	}

	sources, categories, _, err = resolveModelIntelSources("mini split heat pump", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve hvac auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "hvac" {
		t.Fatalf("expected hvac category, got %#v", categories)
	}
	if !containsString(sources, "pioneer-mini-split") || !containsString(sources, "iwae") {
		t.Fatalf("expected hvac sources, got %#v", sources)
	}
	if !containsString(sources, "broan-nutone") {
		t.Fatalf("expected Broan-NuTone model discovery source for HVAC ventilation queries, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("thermostat", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve thermostat auto sources: %v", err)
	}
	if !containsString(categories, "hvac") {
		t.Fatalf("expected thermostat to infer hvac category, got %#v", categories)
	}
	if !containsString(sources, "sylvane") || !containsString(sources, "hardware-hut") {
		t.Fatalf("expected HVAC sources for thermostat, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("floor register", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve floor register auto sources: %v", err)
	}
	if !containsString(categories, "hvac") || !containsString(categories, "hardware") {
		t.Fatalf("expected floor register to infer hvac and hardware categories, got %#v", categories)
	}
	if !containsString(sources, "hardware-hut") || !containsString(sources, "rejuvenation") {
		t.Fatalf("expected HVAC/hardware sources for floor register, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("range hood", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve range hood auto sources: %v", err)
	}
	if !containsString(categories, "appliances") || !containsString(sources, "broan-nutone") {
		t.Fatalf("expected appliance range hood routing with Broan-NuTone discovery, categories=%#v sources=%#v", categories, sources)
	}

	_, categories, _, err = resolveModelIntelSources("shower valve", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve shower valve auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") {
		t.Fatalf("expected plumbing shower valve category inference, got %#v", categories)
	}

	sources, categories, _, err = resolveModelIntelSources("floor warming", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve floor warming auto sources: %v", err)
	}
	if !containsString(categories, "flooring") {
		t.Fatalf("expected floor warming to infer flooring category, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") {
		t.Fatalf("expected Floor & Decor for floor warming, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("linear drain", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve linear drain auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "flooring") {
		t.Fatalf("expected linear drain to infer plumbing and flooring categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "faucetdepot") || !containsString(sources, "kbauthority") {
		t.Fatalf("expected plumbing/flooring sources for linear drain, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("medicine cabinet", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve medicine cabinet auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "decor") {
		t.Fatalf("expected medicine cabinet to infer plumbing and decor categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "ikea") || !containsString(sources, "kbauthority") {
		t.Fatalf("expected plumbing/decor sources for medicine cabinet, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("grab bar", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve grab bar auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "hardware") {
		t.Fatalf("expected grab bar to infer plumbing and hardware categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "hardware-hut") {
		t.Fatalf("expected plumbing/hardware sources for grab bar, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("robe hook", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve robe hook auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "hardware") {
		t.Fatalf("expected robe hook to infer plumbing and hardware categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "hardware-hut") {
		t.Fatalf("expected plumbing/hardware sources for robe hook, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("towel bar", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve towel bar auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "hardware") {
		t.Fatalf("expected towel bar to infer plumbing and hardware categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "ikea") {
		t.Fatalf("expected plumbing/hardware sources for towel bar, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("towel ring", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve towel ring auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "plumbing" {
		t.Fatalf("expected towel ring to infer plumbing category, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "faucetdepot") {
		t.Fatalf("expected plumbing sources for towel ring, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("soap dispenser", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve soap dispenser auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "plumbing" {
		t.Fatalf("expected soap dispenser to infer plumbing category, got %#v", categories)
	}
	if !containsString(sources, "plumbtile") || !containsString(sources, "kbauthority") || !containsString(sources, "faucetdepot") {
		t.Fatalf("expected plumbing sources for soap dispenser, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("towel warmer", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve towel warmer auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "electrical") {
		t.Fatalf("expected towel warmer to infer plumbing and electrical categories, got %#v", categories)
	}
	if !containsString(sources, "plumbtile") || !containsString(sources, "kbauthority") || !containsString(sources, "lighting-new-york") {
		t.Fatalf("expected plumbing/electrical sources for towel warmer, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("lighted mirror", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve lighted mirror auto sources: %v", err)
	}
	if !containsString(categories, "electrical") || !containsString(categories, "plumbing") || !containsString(categories, "decor") {
		t.Fatalf("expected lighted mirror to infer electrical, plumbing, and decor categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "prolighting") || !containsString(sources, "ikea") {
		t.Fatalf("expected electrical/plumbing/decor sources for lighted mirror, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("vanity light", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve vanity light auto sources: %v", err)
	}
	if !containsString(categories, "electrical") || !containsString(categories, "plumbing") {
		t.Fatalf("expected vanity light to infer electrical and plumbing categories, got %#v", categories)
	}
	if !containsString(sources, "bees-lighting") || !containsString(sources, "lighting-new-york") || !containsString(sources, "kbauthority") {
		t.Fatalf("expected electrical/plumbing sources for vanity light, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("picture light", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve picture light auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "electrical" {
		t.Fatalf("expected picture light to infer electrical category, got %#v", categories)
	}
	if !containsString(sources, "bees-lighting") || !containsString(sources, "lighting-new-york") {
		t.Fatalf("expected electrical designer-lighting sources for picture light, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("bidet seat", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve bidet seat auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "plumbing" {
		t.Fatalf("expected bidet seat to infer plumbing category, got %#v", categories)
	}
	if !containsString(sources, "plumbtile") || !containsString(sources, "plumbersstock") || !containsString(sources, "kbauthority") {
		t.Fatalf("expected plumbing sources for bidet seat, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("pot filler", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve pot filler auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "plumbing" {
		t.Fatalf("expected pot filler to infer plumbing category, got %#v", categories)
	}
	if !containsString(sources, "plumbtile") || !containsString(sources, "kbauthority") {
		t.Fatalf("expected plumbing sources for pot filler, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("shower head", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve shower head auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "plumbing" {
		t.Fatalf("expected shower head to infer plumbing category, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "faucetdepot") || !containsString(sources, "plumbtile") {
		t.Fatalf("expected plumbing sources for shower head, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("shower panel", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve shower panel auto sources: %v", err)
	}
	if !containsString(categories, "plumbing") || !containsString(categories, "materials") {
		t.Fatalf("expected shower panel to infer plumbing and materials categories, got %#v", categories)
	}
	if !containsString(sources, "floor-and-decor") || !containsString(sources, "ikea") {
		t.Fatalf("expected plumbing/material sources for shower panel, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("cabinet pull", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve cabinet pull auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "hardware" {
		t.Fatalf("expected cabinet pull to infer hardware category, got %#v", categories)
	}
	if !containsString(sources, "hardware-hut") || !containsString(sources, "rejuvenation") || !containsString(sources, "ikea") {
		t.Fatalf("expected hardware sources for cabinet pull, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("door hinge", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve door hinge auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "hardware" {
		t.Fatalf("expected door hinge to infer hardware category, got %#v", categories)
	}
	if !containsString(sources, "hardware-hut") || !containsString(sources, "ikea") {
		t.Fatalf("expected hardware sources for door hinge, got %#v", sources)
	}

	sources, categories, _, err = resolveModelIntelSources("interior door lever", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve interior door lever auto sources: %v", err)
	}
	if len(categories) != 1 || categories[0] != "hardware" {
		t.Fatalf("expected interior door lever to infer hardware category, got %#v", categories)
	}
	if !containsString(sources, "hardware-hut") || !containsString(sources, "rejuvenation") || !containsString(sources, "ikea") {
		t.Fatalf("expected hardware sources for interior door lever, got %#v", sources)
	}
}

func TestResolveModelIntelSourcesAutoFallsBackToApplianceDiscovery(t *testing.T) {
	sources, categories, room, err := resolveModelIntelSources("premium installed selection", "auto", "", "")
	if err != nil {
		t.Fatalf("resolve fallback auto sources: %v", err)
	}
	if len(categories) != 0 || room != "" {
		t.Fatalf("expected no inferred category or room, got categories=%#v room=%q", categories, room)
	}
	if len(sources) != 2 || sources[0] != "ge-appliances" || sources[1] != "bosch" {
		t.Fatalf("expected appliance fallback sources, got %#v", sources)
	}
}

func TestModelIntelFanoutNote(t *testing.T) {
	note := modelIntelFanoutNote(cliutil.FanoutError{Source: "ferguson", Err: errFake("HTTP 403\nbody")})
	if note != "Source warning: ferguson: HTTP 403" {
		t.Fatalf("unexpected fanout note: %q", note)
	}
}

func TestModelRecordFromProductUsesSelectionSKUForFinishSources(t *testing.T) {
	flooring := modelRecordFromProduct(NormalizedProduct{
		Source:   "floor-and-decor",
		ID:       "101317733",
		Title:    "Luxe Sand Matte Porcelain Tile",
		URL:      "https://www.flooranddecor.com/porcelain-tile/luxe-sand-matte-porcelain-tile-101317733.html",
		PriceMin: 2.99,
	})
	if flooring.Model != "101317733" || !modelIntelUsefulRecord(flooring) {
		t.Fatalf("expected Floor & Decor product ID as selection SKU, got %#v", flooring)
	}

	hardware := modelRecordFromProduct(NormalizedProduct{
		Source:   "hardware-hut",
		ID:       "401603",
		Title:    "Deltana Solid Brass Cabinet Hinge",
		URL:      "https://hardwarehut.com/products/deltana-solid-brass-cabinet-hinge-ch2520",
		PriceMin: 15.4,
	})
	if hardware.Model != "CH2520" || !modelIntelUsefulRecord(hardware) {
		t.Fatalf("expected URL model token before product ID, got %#v", hardware)
	}

	modernBathroom := modelRecordFromProduct(NormalizedProduct{
		Source:   "modern-bathroom",
		ID:       "BECKETT-VANITY-30-SINGLE-SINK-SINGLE-HOLE",
		Title:    "Beckett Bathroom Vanity with Countertop 30 inch Single Sink Single hole Faucet Setup",
		URL:      "https://www.modernbathroom.com/products/beckett-bathroom-vanity-with-countertop-30-inch-single-sink-single-hole-faucet-setup",
		PriceMin: 839.2,
	})
	if modernBathroom.Model != "BECKETT-VANITY-30-SINGLE-SINK-SINGLE-HOLE" {
		t.Fatalf("expected Modern Bathroom handle before generic title token, got %#v", modernBathroom)
	}
}

func TestModelIntelProductMatchesCeilingFanQuery(t *testing.T) {
	fan := NormalizedProduct{
		Title:    "52 in. Ceiling Fan - Color Selectable LED Light Kit",
		Category: "Ceiling Fans",
		URL:      "https://www.1000bulbs.com/product/229168/LEDF-CFANWH.html",
	}
	if !modelIntelProductMatchesQuery("ceiling fan", fan) {
		t.Fatalf("expected actual ceiling fan row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "40 Watt Clear Incandescent A15 Appliance Bulb", Category: "Light Bulbs"},
		{Title: "PSX24W LED Fanless Headlight/Fog Light Conversion Kit", Category: "Vehicle Lighting"},
		{Title: "Adjustable Surface Mount for Explosion Proof Light - Wall or Ceiling Mount", Category: "Mounting Accessories"},
	} {
		if modelIntelProductMatchesQuery("ceiling fan", p) {
			t.Fatalf("expected false-positive ceiling fan row to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesThermostatQuery(t *testing.T) {
	thermostat := NormalizedProduct{
		Title:       "Honeywell Home T9 Wi-Fi Smart Thermostat",
		Category:    "Thermostats",
		Description: "Programmable smart thermostat with room sensor.",
	}
	if !modelIntelProductMatchesQuery("thermostat", thermostat) {
		t.Fatalf("expected actual thermostat row to remain eligible")
	}

	servicePart := NormalizedProduct{
		Title:       "Santa Fe Replacement Defrost Thermostat",
		Category:    "Dehumidifiers",
		Description: "Genuine replacement defrost thermostat. Compatible with the following Santa Fe dehumidifiers.",
	}
	if modelIntelProductMatchesQuery("thermostat", servicePart) {
		t.Fatalf("expected replacement defrost service part to be filtered")
	}
}

func TestModelIntelProductMatchesHVACRegisterQuery(t *testing.T) {
	register := NormalizedProduct{
		Title: "Hamilton Decorative Cast Bronze Floor Register - 10x2-1/4in.",
		URL:   "https://hardwarehut.com/products/hamilton-decorative-cast-bronze-floor-register-10x2-1-4in-bronze-patina-hvf-210-bp",
	}
	if !modelIntelProductMatchesQuery("floor register", register) {
		t.Fatalf("expected actual floor register row to remain eligible")
	}

	louver := NormalizedProduct{
		Title: "Register Louver",
		URL:   "https://www.rejuvenation.com/products/register-louver/",
	}
	if !modelIntelProductMatchesQuery("floor register", louver) {
		t.Fatalf("expected Rejuvenation register louver row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "VARIERA Plastic bag dispenser", Category: "Kitchen cabinet & drawer organization"},
		{Title: "RANNILEN Water trap, 1 bowl", Category: "Bathroom sinks"},
		{Title: "ENKOPING Drawer front", Category: "Kitchen doors & drawer fronts"},
		{Title: "METOD Ventilation grill", Category: "Kitchen cabinet frames, rails, legs & toekicks"},
	} {
		if modelIntelProductMatchesQuery("floor register", p) {
			t.Fatalf("expected false-positive floor register row to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesLinearDrainQuery(t *testing.T) {
	linearDrain := NormalizedProduct{
		Title:    "Compotite 24in. Linear Drain Body Black ABS Linear Shower Drain",
		Category: "Linear Drains",
		URL:      "https://www.flooranddecor.com/shower-systems-installation-materials/compotite-24in.-linear-drain-body-black-abs-linear-shower-drain-100195767.html",
	}
	if !modelIntelProductMatchesQuery("linear drain", linearDrain) {
		t.Fatalf("expected actual linear drain row to remain eligible")
	}

	tileableGrate := NormalizedProduct{
		Title:    "Schluter Kerdi-Line Frameless 48in. Tileable Grate",
		Category: "Linear Drains",
		URL:      "https://www.flooranddecor.com/schluter-kerdi-line-frameless-tileable-grate-SKLFTG101.html",
	}
	if !modelIntelProductMatchesQuery("linear drain", tileableGrate) {
		t.Fatalf("expected linear-drain grate row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "PLUG DRAIN LINER", Category: "liner"},
		{Title: "Dishwasher Drain Hose", Category: "Appliance Parts"},
		{Title: "Washer Drain Pump", Category: "Laundry Parts"},
	} {
		if modelIntelProductMatchesQuery("linear drain", p) {
			t.Fatalf("expected appliance drain false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesShowerNicheQuery(t *testing.T) {
	niche := NormalizedProduct{
		Title:    "Schluter Kerdi 12x12 Shower Niche With Frame",
		Category: "Shower Niches and Wall Shelves",
		URL:      "https://www.flooranddecor.com/schluter-installation-materials/schluter-kerdi-12x12-shower-niche-with-frame-101237006.html",
	}
	if !modelIntelProductMatchesQuery("shower niche", niche) {
		t.Fatalf("expected actual shower niche row to remain eligible")
	}

	wallShelf := NormalizedProduct{
		Title:    "EZ-Niches Large Rectangle Niche",
		Category: "Shower Niches and Wall Shelves",
	}
	if !modelIntelProductMatchesQuery("shower niche", wallShelf) {
		t.Fatalf("expected shower niche wall-shelf row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Symmons Origins Shower Trim with PLR Handle", Category: "Volume Control Valves & Trims"},
		{Title: "Pressure Balance Shower Valve Trim", Category: "Shower Trims"},
		{Title: "Tub Spout with Diverter", Category: "Tub & Shower Trim"},
	} {
		if modelIntelProductMatchesQuery("shower niche", p) {
			t.Fatalf("expected shower trim false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesMedicineCabinetQuery(t *testing.T) {
	medicineCabinet := NormalizedProduct{
		Title:    "Hampton 24 in. White Mirror Medicine Cabinet",
		Category: "Medicine Cabinet",
		URL:      "https://www.flooranddecor.com/mirrors/hampton-24-in.-white-mirror-medicine-cabinet-100902113.html",
	}
	if !modelIntelProductMatchesQuery("medicine cabinet", medicineCabinet) {
		t.Fatalf("expected medicine cabinet row to remain eligible")
	}

	mirrorCabinet := NormalizedProduct{
		Title:    "IVOSJON Mirror cabinet with 1 door",
		Category: "Home decor & accessories > Mirrors > Medicine cabinets with mirror",
	}
	if !modelIntelProductMatchesQuery("medicine cabinet", mirrorCabinet) {
		t.Fatalf("expected mirror cabinet row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Fleurco - Medicine Cabinet Long Shelf Kit For 60 Inch Width", Category: "Furniture Parts"},
		{Title: "Yamazaki Medicine Cabinet Risers (Set of 2)", Category: "Bathroom organization"},
		{Title: "KNOXHULT Wall cabinet with door", Category: "Kitchen cabinets & parts"},
		{Title: "BRIMNES Cabinet with doors", Category: "Display & storage cabinets > Cabinets, hutches & cupboards"},
	} {
		if modelIntelProductMatchesQuery("medicine cabinet", p) {
			t.Fatalf("expected medicine cabinet false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesGrabBarQuery(t *testing.T) {
	grabBar := NormalizedProduct{
		Title:    "Brushed Nickel 18 in. Concealed Mount ADA Compliant Grab Bar",
		Category: "Bathroom Hardware",
		URL:      "https://www.flooranddecor.com/bathroom-accessories/brushed-nickel-18-in.-concealed-mount-ada-compliant-grab-bar-101177798.html",
	}
	if !modelIntelProductMatchesQuery("grab bar", grabBar) {
		t.Fatalf("expected actual grab bar row to remain eligible")
	}

	combo := NormalizedProduct{
		Title:    "Brushed Nickel 24 in. Grab Bar with Towel Holder",
		Category: "Bathroom Hardware",
	}
	if !modelIntelProductMatchesQuery("grab bar", combo) {
		t.Fatalf("expected grab-bar combo row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "GE 2.5 Gallon Electric Point of Use Water Heater", Category: "Electric Point Of Use"},
		{Title: "GE ENERGY STAR Top Control Dishwasher", Category: "Built-In"},
		{Title: "DRAGSMARK Clip-on handle", Category: "Kitchen knobs and handles"},
		{Title: "BASINGEN Towel rail", Category: "Bathroom shelves"},
		{Title: "Wall Mount Shower Glass Clamp", Category: "Shower door hardware"},
	} {
		if modelIntelProductMatchesQuery("grab bar", p) {
			t.Fatalf("expected grab bar false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesTowelWarmerQuery(t *testing.T) {
	towelWarmer := NormalizedProduct{
		Title:    "Myson Ecmh3/3 Electric 120V 28 Inch H X 21 Inch W Towel Warmer",
		Category: "Towel Warmers",
		URL:      "https://plumbtile.com/products/myson-ecmh-3-3",
	}
	if !modelIntelProductMatchesQuery("towel warmer", towelWarmer) {
		t.Fatalf("expected actual towel warmer row to remain eligible")
	}

	radiator := NormalizedProduct{
		Title: "ALFI BRAND 31 1/2 INCH LIVE EDGE CEDAR WOOD TOWEL WARMER IN POLISHED CHROME",
	}
	if !modelIntelProductMatchesQuery("heated towel radiator", radiator) {
		t.Fatalf("expected heated towel warmer row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "GE Profile Steam Closet with Fabric Refresh", Category: "Steam Closet"},
		{Title: "Carson 7 inch Brushed Nickel Towel Ring", Category: "Accent & Wall Shelves"},
		{Title: "BASINGEN Towel rail", Category: "Bathroom shelves"},
		{Title: "Myson - Switch, Towel Warmer, Pearl", Category: "Towel Warmer Accessories"},
		{Title: "Myson - Towel Warmer 100W Replacement Element", Category: "Towel Warmer Accessories"},
	} {
		if modelIntelProductMatchesQuery("towel warmer", p) {
			t.Fatalf("expected towel warmer false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesShowerDoorQuery(t *testing.T) {
	showerDoor := NormalizedProduct{
		Title:    "Dade Matte Black Bypass Sliding Shower Door",
		Category: "Shower Doors",
		URL:      "https://www.flooranddecor.com/shower-doors/dade-matte-black-bypass-sliding-shower-door-101464402.html",
	}
	if !modelIntelProductMatchesQuery("shower door", showerDoor) {
		t.Fatalf("expected actual shower door row to remain eligible")
	}

	screen := NormalizedProduct{
		Title: "Linea Satin Black Single Panel Frameless Shower Screen",
	}
	if !modelIntelProductMatchesQuery("shower door", screen) {
		t.Fatalf("expected shower screen row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Kohler - Components 11 Inch Shower Door Handle", Category: "Door Pulls"},
		{Title: "Kallista - Central Park West Shower Door Handle Escutcheons (Pair)", Category: "Shower Components"},
		{Title: "Wall Mount Shower Glass Clamp", Category: "Shower door hardware"},
		{Title: "Wall Mount Glass Shower Door Hinge", Category: "Shower door hardware"},
	} {
		if modelIntelProductMatchesQuery("shower door", p) {
			t.Fatalf("expected shower door false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesLightedMirrorQuery(t *testing.T) {
	ledMirror := NormalizedProduct{
		Title:    "Homewerks 24 in. Silver LED Mirror",
		Category: "Mirror",
		URL:      "https://www.flooranddecor.com/mirrors/homewerks-24-in.-silver-led-mirror-101486066.html",
	}
	if !modelIntelProductMatchesQuery("lighted mirror", ledMirror) {
		t.Fatalf("expected LED mirror row to remain eligible")
	}

	builtInLight := NormalizedProduct{
		Title:    "EKFANN Mirror with built-in light",
		Category: "Magnifying & makeup mirrors",
	}
	if !modelIntelProductMatchesQuery("lighted mirror", builtInLight) {
		t.Fatalf("expected mirror with built-in light row to remain eligible")
	}

	mirrorLight := NormalizedProduct{
		Title:    "RAB MIRA Edge LED Mirror Light - 18 x 32",
		Category: "Illuminated Mirrors > MIRA Lighted Mirrors",
	}
	if !modelIntelProductMatchesQuery("lighted mirror", mirrorLight) {
		t.Fatalf("expected mirror light row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "2 in. Dia. - LED G16.5 Globe - 5 Watt", Category: "LED decorative bulbs"},
		{Title: "MUSIK Wall lamp", Category: "Wall sconces & lamps"},
		{Title: "Tinted Frameless Wall Mirror", Category: "Wall Mirrors"},
		{Title: "NYSJON Mirror with shelf", Category: "Vanity mirrors"},
		{Title: "Mirror Mirror LED 17.88 inch Titanium Bath Vanity & Wall Light", Category: "Bathroom Vanity Lights"},
	} {
		if modelIntelProductMatchesQuery("lighted mirror", p) {
			t.Fatalf("expected lighted mirror false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesRobeHookQuery(t *testing.T) {
	floorDecorHook := NormalizedProduct{
		Title:    "Grant Chrome Robe Hook",
		Category: "Robe Hook",
	}
	if !modelIntelProductMatchesQuery("robe hook", floorDecorHook) {
		t.Fatalf("expected robe hook row to remain eligible")
	}

	wardrobeHook := NormalizedProduct{
		Title: "Ives Double Wardrobe Hook (Polished Brass)",
	}
	if !modelIntelProductMatchesQuery("robe hook", wardrobeHook) {
		t.Fatalf("expected wardrobe hook row to remain eligible")
	}

	bathDoubleHook := NormalizedProduct{
		Title:    "BROFJÄRDEN Double hook",
		Category: "Bathroom accessories",
	}
	if !modelIntelProductMatchesQuery("robe hook", bathDoubleHook) {
		t.Fatalf("expected bathroom double hook row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "BÄSTIS Hook", Category: "Wall & door hooks"},
		{Title: "SKOGSVIKEN Hook for door", Category: "Bathroom shelves"},
		{Title: "TISKEN Hook with suction cup", Category: "Wall & door hooks"},
		{Title: "Ives Ceiling Hook / Undercounter Mount Purse Hook"},
		{Title: "VARIERA Pot lid organizer"},
		{Title: "Bathrobe", Category: "Bathroom textiles"},
	} {
		if modelIntelProductMatchesQuery("robe hook", p) {
			t.Fatalf("expected robe hook false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesTowelBarQuery(t *testing.T) {
	towelBar := NormalizedProduct{
		Title:    "Grant Chrome 18 in. Towel Bar",
		Category: "Towel Bar",
	}
	if !modelIntelProductMatchesQuery("towel bar", towelBar) {
		t.Fatalf("expected towel bar row to remain eligible")
	}

	towelRail := NormalizedProduct{
		Title:    "BÄSINGEN Towel rail",
		Category: "Bathroom shelves",
	}
	if !modelIntelProductMatchesQuery("towel bar", towelRail) {
		t.Fatalf("expected bathroom towel rail row to remain eligible")
	}

	towelHolder := NormalizedProduct{
		Title:    "BROFJÄRDEN Towel holder",
		Category: "Bathroom shelves",
	}
	if !modelIntelProductMatchesQuery("towel bar", towelHolder) {
		t.Fatalf("expected towel holder row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "GE ENERGY STAR Top Control Dishwasher with Dry Boost", Category: "Built-In"},
		{Title: "JORDMÅNEN Hook", Category: "Wall & door hooks"},
		{Title: "BUMERANG Hanger", Category: "Clothes hangers"},
		{Title: "NEREBY Rail", Category: "Kitchen wall organizers"},
		{Title: "HULTARP Rail", Category: "Kitchen organization"},
		{Title: "UTRUSTA Towel rail", Category: "Kitchen systems > SEKTION kitchen cabinets"},
	} {
		if modelIntelProductMatchesQuery("towel bar", p) {
			t.Fatalf("expected towel bar false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesTowelRingQuery(t *testing.T) {
	towelRing := NormalizedProduct{
		Title:    "Grant Chrome Towel Ring",
		Category: "Towel Ring",
	}
	if !modelIntelProductMatchesQuery("towel ring", towelRing) {
		t.Fatalf("expected towel ring row to remain eligible")
	}

	brushedNickel := NormalizedProduct{
		Title:    "Parker Brushed Nickel Towel Ring",
		Category: "Bathroom Accessories",
	}
	if !modelIntelProductMatchesQuery("towel ring", brushedNickel) {
		t.Fatalf("expected alternate towel ring row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "PÅLYCKE Clip-on hook rack", Category: "Kitchen wall organizers"},
		{Title: "BROFJÄRDEN Double hook", Category: "Bathroom shelves"},
		{Title: "SLÅNHÖSTMAL Hand towel", Category: "Bathroom textiles"},
		{Title: "RÖDSYRA Spoonrest", Category: "Kitchen utensils"},
		{Title: "LÄTTHET Hook and clip", Category: "SMÅSTAD interior fittings"},
		{Title: "VALASJÖN Towel hanger, self-adhesive", Category: "Bathroom shelves"},
	} {
		if modelIntelProductMatchesQuery("towel ring", p) {
			t.Fatalf("expected towel ring false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesSoapDispenserQuery(t *testing.T) {
	soapDispenser := NormalizedProduct{
		Title:    "KRAUS KSD-43 Deck Mounted Kitchen Soap and Lotion Dispenser",
		Category: "Kitchen Soap Dispensers",
	}
	if !modelIntelProductMatchesQuery("soap dispenser", soapDispenser) {
		t.Fatalf("expected deck mounted soap/lotion dispenser row to remain eligible")
	}

	bathDispenser := NormalizedProduct{
		Title:    "Trim By Design Economy Soap & Lotion Dispenser",
		Category: "Bathroom Soap Dispensers",
	}
	if !modelIntelProductMatchesQuery("soap dispenser", bathDispenser) {
		t.Fatalf("expected bathroom soap/lotion dispenser row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "GE 3.8 DOE cu. ft. stainless steel capacity washer", Category: "washers"},
		{Title: "RINNIG Dish brush", Category: "Dish cloths, sponges, rags & more"},
		{Title: "NYSKÖLJD Dish drying mat", Category: "Dish racks & drying mats"},
		{Title: "VÄLVÅRDAD Dish-washing brush refills", Category: "Dish cloths, sponges, rags & more"},
		{Title: "CITRONHAJ Oil/vinegar bottle", Category: "Spice jars, shakers & grinders"},
		{Title: "STORAVAN 3-piece bathroom set", Category: "Soap dispensers & soap dishes"},
		{Title: "EKOLN Soap dish", Category: "Soap dispensers & soap dishes"},
	} {
		if modelIntelProductMatchesQuery("soap dispenser", p) {
			t.Fatalf("expected soap dispenser false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesVanityLightQuery(t *testing.T) {
	bathVanityLight := NormalizedProduct{
		Title:    "Enchant 24 LED Bathroom Vanity Light, Brushed Nickel Finish",
		Category: "Bathroom Vanity Lights",
	}
	if !modelIntelProductMatchesQuery("vanity light", bathVanityLight) {
		t.Fatalf("expected bathroom vanity light row to remain eligible")
	}

	kbauthorityLight := NormalizedProduct{
		Title: "DAINOLITE VLD-215-3W 20 INCH 3-LIGHT LED WALL VANITY LIGHTS",
		URL:   "https://www.kbauthority.com/dainolite-vld-215-3w-20-inch-3-light-led-wall-vanity-lights.html",
	}
	if !modelIntelProductMatchesQuery("vanity light", kbauthorityLight) {
		t.Fatalf("expected bath-showroom vanity light row to remain eligible")
	}

	bathBar := NormalizedProduct{
		Title:    "Spec 36 LED Bath Vanity, Black Finish",
		Category: "Bathroom Vanity Lights",
	}
	if !modelIntelProductMatchesQuery("vanity light", bathBar) {
		t.Fatalf("expected bath vanity light row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "DE3175 LED Light Bulb", Category: "Festoon Base LED Bulbs > Cars, Trucks, and SUVs"},
		{Title: "Toman 10.25 in. Matte Black Outdoor Single Sconce", Category: "Sconce"},
		{Title: "Nouveau 2 Small Oxford 1 Light 7.50 inch Wall Sconce", Category: "Wall Sconces"},
		{Title: "Beckett Bathroom Vanity with Countertop", Category: "Bathroom Vanities"},
	} {
		if modelIntelProductMatchesQuery("vanity light", p) {
			t.Fatalf("expected vanity light false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesPictureLightQuery(t *testing.T) {
	kichler := NormalizedProduct{
		Title:    "MIDI-18 Kichler Midi 18 in. 2-Light Picture Light",
		Category: "Picture Lights",
	}
	if !modelIntelProductMatchesQuery("picture light", kichler) {
		t.Fatalf("expected picture light row to remain eligible")
	}

	cabinetMaker := NormalizedProduct{
		Title:    "Chapman & Myers Cabinet Maker 1 Light 8.00 inch Picture Light",
		Category: "Picture Lights",
	}
	if !modelIntelProductMatchesQuery("picture light", cabinetMaker) {
		t.Fatalf("expected cabinet-maker picture light row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "10 Watt - T5.5 Incandescent Light Bulb", Category: "Incandescent Light Bulbs"},
		{Title: "15 Watt - Clear - Incandescent T8 PYGMY Light Bulb", Category: "Light Bulbs"},
		{Title: "Shatter Resistant - 20 Watt - T6.5 Incandescent Light Bulb", Category: "Tubular Bulbs"},
		{Title: "Kitchen Hub", Category: "Kitchen Appliances"},
		{Title: "Designer Wall Mount Hood", Category: "Range Hoods"},
		{Title: "GE Profile Refrigerator", Category: "Refrigerators"},
	} {
		if modelIntelProductMatchesQuery("picture light", p) {
			t.Fatalf("expected picture light false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesBidetSeatQuery(t *testing.T) {
	bidetSeat := NormalizedProduct{
		Title:    "Kohler Puretide Bidet Seat",
		Category: "Bidet Seats",
	}
	if !modelIntelProductMatchesQuery("bidet seat", bidetSeat) {
		t.Fatalf("expected bidet seat row to remain eligible")
	}

	washlet := NormalizedProduct{
		Title:    "TOTO WASHLET S2 Round Electronic Bidet Toilet Seat w/ SoftClose",
		Category: "Bathroom > Toilets",
	}
	if !modelIntelProductMatchesQuery("bidet seat", washlet) {
		t.Fatalf("expected washlet bidet toilet seat row to remain eligible")
	}

	electricBidet := NormalizedProduct{
		Title: "OVE DECORS CALERO ELECTRIC BIDET SEAT FOR ELONGATED SHAPE TOILET IN WHITE",
	}
	if !modelIntelProductMatchesQuery("bidet seat", electricBidet) {
		t.Fatalf("expected electric bidet seat row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Winfield One-Piece Elongated High-Efficiency Toilet", Category: "Bathroom > Toilets"},
		{Title: "Toilet Tank With Left-Hand Trip Lever", Category: "Toilet Tanks"},
		{Title: "Bizet 1 Light 6 inch Black Pearl Wall Bracket Wall Light", Category: "Wall Sconces"},
	} {
		if modelIntelProductMatchesQuery("bidet seat", p) {
			t.Fatalf("expected bidet seat false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesPotFillerQuery(t *testing.T) {
	potFiller := NormalizedProduct{
		Title:    "Kohler Traditional Pot Filler",
		Category: "Kitchen Potfiller Faucets",
	}
	if !modelIntelProductMatchesQuery("pot filler", potFiller) {
		t.Fatalf("expected pot filler row to remain eligible")
	}

	wallMount := NormalizedProduct{
		Title: "Whitehaus Wall Mount Retractable Swing Spout Pot Filler With Lever Handles",
	}
	if !modelIntelProductMatchesQuery("pot filler", wallMount) {
		t.Fatalf("expected wall-mount pot filler row to remain eligible")
	}

	deckMount := NormalizedProduct{
		Title: "Whitehaus Patented Deck Mount Pot Filler With Lever Handles And Swivel Aerator",
	}
	if !modelIntelProductMatchesQuery("pot filler", deckMount) {
		t.Fatalf("expected deck-mount pot filler row with swivel aerator to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Brizo Litze Pot Filler Handle Kit", Category: "Kitchen Parts"},
		{Title: "Pot Filler Replacement Cartridge", Category: "Faucet Parts"},
		{Title: "Pot Filler Spout Kit", Category: "Replacement Parts"},
	} {
		if modelIntelProductMatchesQuery("pot filler", p) {
			t.Fatalf("expected pot filler accessory false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesShowerValveQuery(t *testing.T) {
	valveTrim := NormalizedProduct{
		Title:    "Moen® TL181 Valve Trim Only, 2.5 gpm Shower, Polished Chrome",
		Category: "Pressure Balanced Trims",
	}
	if !modelIntelProductMatchesQuery("moen shower valve", valveTrim) {
		t.Fatalf("expected valve trim row to remain eligible")
	}

	mCoreValve := NormalizedProduct{
		Title: "MOEN UT3691 VOSS 6 1/4 INCH M-CORE 3-SERIES VALVE ONLY",
	}
	if !modelIntelProductMatchesQuery("moen shower valve", mCoreValve) {
		t.Fatalf("expected M-CORE valve row to remain eligible")
	}

	roughIn := NormalizedProduct{
		Title:    "Delta R10000-UNBX MultiChoice Universal Tub and Shower Valve Body",
		Category: "Rough-In Valves",
	}
	if !modelIntelProductMatchesQuery("shower valve", roughIn) {
		t.Fatalf("expected tub and shower valve body row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{
			Title:       "Moon River II Honed Marble Tile",
			Category:    "Floor or Wall Tile",
			Description: "Marble wall and floor tile for shower surrounds",
		},
		{Title: "Rhiver Matte Black Shower Head", Category: "Shower Head"},
		{Title: "Matte Black Frameless Shower Door", Category: "Shower Doors"},
		{Title: "Carrara Waterproof Resilient Shower and Wall Panel", Category: "Shower Wall Panel"},
	} {
		if modelIntelProductMatchesQuery("moen shower valve", p) {
			t.Fatalf("expected shower valve false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesShowerHeadQuery(t *testing.T) {
	showerHead := NormalizedProduct{
		Title:    "Rhiver Matte Black Shower Head",
		Category: "Shower Head",
	}
	if !modelIntelProductMatchesQuery("shower head", showerHead) {
		t.Fatalf("expected shower head row to remain eligible")
	}

	sprayHead := NormalizedProduct{
		Title:    "Brizo Levoir Hydrachoice H2Okinetic Invigorating Spray Head",
		Category: "Shower Heads",
	}
	if !modelIntelProductMatchesQuery("shower head", sprayHead) {
		t.Fatalf("expected spray head row to remain eligible")
	}

	handShower := NormalizedProduct{
		Title: "Modern Hand Shower Wand",
	}
	if !modelIntelProductMatchesQuery("hand shower", handShower) {
		t.Fatalf("expected hand shower row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Jadyn Matte Black Tub and Shower Combination", Category: "Shower & Tub Faucet"},
		{Title: "Pressure Balance Shower Valve Trim", Category: "Shower Trims"},
		{Title: "Shower Arm and Flange", Category: "Shower Accessories"},
		{Title: "Hand Shower Hose", Category: "Shower Accessories"},
		{Title: "Diverter Bathcock with Riser and Shower Head", Category: "Diverter Trims"},
	} {
		if modelIntelProductMatchesQuery("shower head", p) {
			t.Fatalf("expected shower head false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesShowerPanelQuery(t *testing.T) {
	showerWallPanel := NormalizedProduct{
		Title:    "Carrara Forza Waterproof Resilient Shower and Wall Panel",
		Category: "Shower Wall Panel",
	}
	if !modelIntelProductMatchesQuery("shower panel", showerWallPanel) {
		t.Fatalf("expected shower wall panel row to remain eligible")
	}

	wallPanel := NormalizedProduct{
		Title:    "Riva Bianca Waterproof Resilient Shower and Wall Panel",
		Category: "Wall Panel",
	}
	if !modelIntelProductMatchesQuery("shower panel", wallPanel) {
		t.Fatalf("expected shower wall panel row with generic category to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "BROGRUND Hook", Category: "Bathroom shelves"},
		{Title: "RIKTIG Curtain hook", Category: "Curtain hardware"},
		{Title: "PNL TERRA SOLITO LOAD IN CRATE", Category: "Shower Wall Panel"},
		{Title: "Wall Mount Shower Glass Clamp", Category: "Shower door hardware"},
		{Title: "Pressure Balance Shower Valve Trim", Category: "Shower Trims"},
	} {
		if modelIntelProductMatchesQuery("shower panel", p) {
			t.Fatalf("expected shower panel false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesCabinetPullQuery(t *testing.T) {
	cabinetPull := NormalizedProduct{
		Title: "Berenson Advantage Plus 5 inch Center-to-Center Square Cabinet Pull",
	}
	if !modelIntelProductMatchesQuery("cabinet pull", cabinetPull) {
		t.Fatalf("expected cabinet pull row to remain eligible")
	}

	drawerPull := NormalizedProduct{
		Title: "Mission Drawer Pull",
	}
	if !modelIntelProductMatchesQuery("drawer pull", drawerPull) {
		t.Fatalf("expected drawer pull row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "QuickScrews #8-32 Machine Screws - For Cabinet Knobs & Pulls"},
		{Title: "Vernon Cabinet Collection"},
		{Title: "LATMASK Clip-on handle", Category: "SMÅSTAD knobs, handles & accessories"},
		{Title: "KALERUM Knob", Category: "Knobs and handles"},
		{Title: "UTRUSTA Hinge", Category: "Cabinet hinges"},
	} {
		if modelIntelProductMatchesQuery("cabinet pull", p) {
			t.Fatalf("expected cabinet pull false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesDoorHingeQuery(t *testing.T) {
	doorHinge := NormalizedProduct{
		Title: "SGS Residential Duty 4 x 4 Door Hinge - 5/8in Radius Corner",
	}
	if !modelIntelProductMatchesQuery("door hinge", doorHinge) {
		t.Fatalf("expected door hinge row to remain eligible")
	}

	radiusCorner := NormalizedProduct{
		Title: "Ball Bearing Residential Door Hinge Radius Corner",
	}
	if !modelIntelProductMatchesQuery("door hinge", radiusCorner) {
		t.Fatalf("expected residential radius-corner door hinge row to remain eligible")
	}

	for _, p := range []NormalizedProduct{
		{Title: "Ives Hinge Pin Door Stop"},
		{Title: "Knape and Vogt 8092 Series 4x4 Pivot Pocket Door Slides"},
		{Title: "UTRUSTA Hinge", Category: "Cabinet hinges & dampers"},
		{Title: "KOMPLEMENT Soft closing hinge", Category: "PAX wardrobe system"},
		{Title: "Salice Universal Super Hinge", Category: "Cabinet Hardware"},
		{Title: "Wall Mount Glass Shower Door Hinge", Category: "Shower door hardware"},
	} {
		if modelIntelProductMatchesQuery("door hinge", p) {
			t.Fatalf("expected door hinge false positive to be filtered: %#v", p)
		}
	}
}

func TestModelIntelProductMatchesDoorLeverAndLocksetQuery(t *testing.T) {
	for _, p := range []NormalizedProduct{
		{Title: "Schlage Century Door Entrance Lower Handleset w/Latitude Interior Lever", Category: "Electronic & Pushbutton Locks"},
		{Title: "Grandeur Fifth Avenue Door Entry Handle Set", Category: "Door Entry Handlesets"},
		{Title: "Emtek Privacy Door Lever Set", Category: "Door Levers"},
	} {
		if !modelIntelProductMatchesQuery("interior door lever", p) {
			t.Fatalf("expected door lever/handleset row to remain eligible: %#v", p)
		}
	}

	for _, p := range []NormalizedProduct{
		{Title: "UTRUSTA Cabinet Handle", Category: "Cabinet handles"},
		{Title: "Wall Mount Shower Door Handle", Category: "Shower door hardware"},
		{Title: "Ives Hinge Pin Door Stop", Category: "Door accessories"},
		{Title: "Door Lock Replacement Latchbolt", Category: "Door lock parts"},
		{Title: "Kwikset Key Blank", Category: "Key blanks"},
	} {
		if modelIntelProductMatchesQuery("interior door lever", p) {
			t.Fatalf("expected door lever false positive to be filtered: %#v", p)
		}
	}
}

func TestHardwareHutProductPagePriceExtractsSchemaOffer(t *testing.T) {
	body := `<script type="application/ld+json">{"@context":"http://schema.org/","@type":"ProductGroup","hasVariant":[{"@type":"Product","name":"Schlage Century Door Entrance Lower Handleset","offers":{"@type":"Offer","priceCurrency":"USD","price":"226.29","availability":"http://schema.org/InStock"}}]}</script>`
	if price := hardwareHutProductPagePrice(body); price != 226.29 {
		t.Fatalf("expected Hardware Hut schema price, got %v", price)
	}

	if price := hardwareHutProductPagePrice(`<meta property="product:price:amount" content="0.00">`); price != 0 {
		t.Fatalf("expected zero-only metadata to be ignored, got %v", price)
	}
}

func TestShouldProbeAJMadisonIgnoresNonApplianceCategoryText(t *testing.T) {
	hinge := modelIntelRecord{
		Model:      "20204647",
		Title:      "UTRUSTA Hinge",
		Category:   "Kitchen, appliances & supplies > Cabinet hinges & dampers",
		ProductURL: "https://www.ikea.com/us/en/p/utrusta-hinge-20204647/",
	}
	if shouldProbeAJMadison(&hinge) {
		t.Fatalf("non-appliance hinge should not probe AJ Madison")
	}

	cooktop := modelIntelRecord{
		Model:      "PHP9036DJBB",
		Title:      "GE Profile 36 inch induction cooktop",
		ProductURL: "https://www.geappliances.com/appliance/GE-Profile-36-Built-In-Touch-Control-Induction-Cooktop-PHP9036DJBB",
	}
	if !shouldProbeAJMadison(&cooktop) {
		t.Fatalf("appliance model should probe AJ Madison")
	}
}

func TestMoenShopProductProbeHelpers(t *testing.T) {
	if !shouldProbeMoenProductPage(&modelIntelRecord{Model: "UT3691", Brand: "Moen"}) {
		t.Fatalf("expected Moen-branded record to probe Moen product page")
	}
	if shouldProbeMoenProductPage(&modelIntelRecord{Model: "UT3691", Brand: "Kohler"}) {
		t.Fatalf("non-Moen record should not probe Moen product page")
	}
	if url := moenShopProductURL("UT3691ORB"); url != "https://shop.moen.com/products/ut3691" {
		t.Fatalf("unexpected Moen base product URL: %q", url)
	}
	body := `<html><head><meta property="og:title" content="Voss M-CORE 3-Series Valve Trim"></head><body>
		<script>
		const variants = [{"sku":"UT3691","price":31968},{"sku":"UT3691ORB","price":56362}];
		</script>
		<a href="https://assets.moen.com/shared/docs/product-specifications/ut3691ut3692ut3693sp.pdf">Product Specifications</a>
		<a href="https://assets.moen.com/shared/docs/instruction-sheets/ins10945e.pdf">Instruction Sheet/Owners Manual</a>
	</body></html>`
	if !moenPageContainsModel(body, "UT3691ORB") {
		t.Fatalf("expected suffix model to match base Moen product page")
	}
	if price := moenShopPriceForModel(body, "UT3691ORB"); price != 563.62 {
		t.Fatalf("unexpected Moen variant price: %v", price)
	}
	specs := filterModelSpecDocs("UT3691ORB", moenSpecDocsFromHTML(body))
	if len(specs) != 2 {
		t.Fatalf("expected Moen product-page docs, got %#v", specs)
	}
	for _, spec := range specs {
		if !strings.HasPrefix(spec.URL, "https://assets.moen.com/") {
			t.Fatalf("expected canonical Moen asset URL, got %#v", spec)
		}
	}
}

func TestLevitonProductProbeHelpers(t *testing.T) {
	if !shouldProbeLevitonProductPage(&modelIntelRecord{Model: "IPL06-10Z", Brand: "Leviton"}) {
		t.Fatalf("expected Leviton-branded record to probe Leviton product page")
	}
	if shouldProbeLevitonProductPage(&modelIntelRecord{Model: "IPL06-10Z", Brand: "Lutron"}) {
		t.Fatalf("non-Leviton record should not probe Leviton product page")
	}
	if url := levitonProductURL("IPL06-10Z"); url != "https://leviton.com/products/ipl06-10z" {
		t.Fatalf("unexpected Leviton product URL: %q", url)
	}

	body := `<html><body>
		<h2>IPL06-10Z</h2>
		<a href="/content/dam/leviton/residential/product_documents/product_specification/Leviton-IllumaTech-Product-Bulletin.pdf">Product Bulletin</a>
		<a href="/content/dam/leviton/residential/product_documents/instruction_sheet/leviton-ipl06-instruction-sheet-en.pdf">Instruction Sheet - IllumaTech | IPL06 | EN</a>
		<a href="/content/dam/leviton/residential/product_documents/instruction_sheet/leviton-ipl06-instruction-sheet-fr.pdf">Instruction Sheet - IllumaTech | IPL06 | FR</a>
		<a href="/content/dam/leviton/residential/product_documents/instruction_sheet/product-cleaning-instructions.pdf">Generic Cleaning Instructions</a>
		<a href="/content/dam/leviton/residential/product_documents/none/leviton-dimmer-buying-guide.pdf">Dimmers Buying Guide</a>
	</body></html>`

	docs := filterModelSpecDocs("IPL06-10Z", levitonSpecDocsFromHTML(body, "https://leviton.com/products/ipl06-10z"))
	if len(docs) != 2 {
		t.Fatalf("expected Leviton spec and English instruction docs, got %#v", docs)
	}
	if docs[0].Kind != "spec" || docs[1].Kind != "installation" {
		t.Fatalf("unexpected Leviton doc kinds: %#v", docs)
	}
	for _, doc := range docs {
		if !strings.HasPrefix(doc.URL, "https://leviton.com/content/dam/leviton/") {
			t.Fatalf("expected canonical Leviton document URL, got %#v", doc)
		}
	}
}

func TestFilterModelSpecDocsRejectsGenericCreditApplications(t *testing.T) {
	docs := filterModelSpecDocs("IPL06-10Z", []modelSpecDocument{
		{Kind: "spec", URL: "https://assets.1000bulbs.com/VetCLmyPnikhum9fjKVK5oTV?response-content-disposition=inline%3B+filename%3D%22leviton-ipl06-10z-specs.pdf", Source: "1000bulbs"},
		{Kind: "document", URL: "https://assets.1000bulbs.com/static/homepage/creditapplication.pdf", Source: "1000bulbs"},
	})
	if len(docs) != 1 || !strings.Contains(docs[0].URL, "leviton-ipl06-10z-specs.pdf") {
		t.Fatalf("expected only model-specific spec doc, got %#v", docs)
	}
}

func TestBroanNuToneRouteForQuery(t *testing.T) {
	route, category, ok := broanNuToneRouteForQuery("quiet bath fan")
	if !ok || !strings.Contains(route, "bath%20fan") || category != "Ventilation > Bath Fans" {
		t.Fatalf("unexpected bath fan route: route=%q category=%q ok=%v", route, category, ok)
	}
	route, category, ok = broanNuToneRouteForQuery("30 inch range hood")
	if !ok || !strings.Contains(route, "range%20hood") || category != "Ventilation > Range Hoods" {
		t.Fatalf("unexpected range hood route: route=%q category=%q ok=%v", route, category, ok)
	}
}

func TestNormalizeBroanNuToneModelCard(t *testing.T) {
	card := `<div class="productCard productCard--slim">
		<input id="PCPE811" type="checkbox" value="PCPE811">
		<figure class="productCard-figure">
			<a href="/en-us/product/ventilationfans/pcpe811">
				<img class="productCard-img" src="/getmedia/431ad3f9/PCPE811-complete.jpg?width=1800&amp;height=1800&amp;ext=.jpg" alt="Broan-NuTone FlexAir Selectable Bathroom Exhaust Fan, 80/110 CFM">
			</a>
		</figure>
		<div class="productPanel productCard-productPanel">
			<h3 class="productPanel-heading">
				<a href="/en-us/product/ventilationfans/pcpe811" class="productPanel-link">Broan-NuTone FlexAir Selectable Bathroom Exhaust Fan, 80/110 CFM</a>
			</h3>
			<div class="productPanel-infoOption productPanel-infoOption--noSpace">
				<h4 class="productPanel-infoOptionHeading">Model:</h4>
				<p><a href="/en-us/product/ventilationfans/pcpe811" class="productPanel-link">PCPE811</a></p>
			</div>
		</div>
	</div>`

	rec := normalizeBroanNuToneModelCard(card, "https://www.broan-nutone.com/en-us/search/product?q=bath%20fan", "Ventilation > Bath Fans")
	if rec.Model != "PCPE811" || rec.Brand != "Broan-NuTone" {
		t.Fatalf("unexpected model/brand: %#v", rec)
	}
	if rec.Title != "Broan-NuTone FlexAir Selectable Bathroom Exhaust Fan, 80/110 CFM" {
		t.Fatalf("unexpected title: %q", rec.Title)
	}
	if rec.ProductURL != "https://www.broan-nutone.com/en-us/product/ventilationfans/pcpe811" {
		t.Fatalf("unexpected product URL: %q", rec.ProductURL)
	}
	if rec.ImageURL != "https://www.broan-nutone.com/getmedia/431ad3f9/PCPE811-complete.jpg?width=1800&height=1800&ext=.jpg" {
		t.Fatalf("unexpected image URL: %q", rec.ImageURL)
	}
	if len(rec.SourceFields) != 1 || rec.SourceFields[0] != "model_discovery: broan-nutone" {
		t.Fatalf("expected discovery marker, got %#v", rec.SourceFields)
	}
}

func TestBroanNuToneSelectionTitleRejectsReplacementParts(t *testing.T) {
	if !broanNuToneSelectionTitle("Broan-NuTone FlexAir Selectable Bathroom Exhaust Fan, 80/110 CFM") {
		t.Fatalf("expected bath fan selection row to be accepted")
	}
	if broanNuToneSelectionTitle("Broan-NuTone Genuine Replacement Bath Fan Motor Assembly, 80 CFM") {
		t.Fatalf("expected replacement motor row to be rejected")
	}
}

func TestDiscoveryOfferFallbackCandidatesAreDeterministicAndBounded(t *testing.T) {
	records := map[string]*modelIntelRecord{
		"B200": {
			Model:        "B200",
			Title:        "Second model",
			ProductURL:   "https://www.broan-nutone.com/en-us/product/ventilationfans/b200",
			SourceFields: []string{"model_discovery: broan-nutone"},
		},
		"E500": {
			Model:        "E500",
			Title:        "Fifth model",
			ProductURL:   "https://www.broan-nutone.com/en-us/product/ventilationfans/e500",
			SourceFields: []string{"model_discovery: broan-nutone"},
		},
		"A100": {
			Model:        "A100",
			Title:        "First model",
			ProductURL:   "https://www.broan-nutone.com/en-us/product/ventilationfans/a100",
			SourceFields: []string{"model_discovery: broan-nutone"},
		},
		"C300": {
			Model:        "C300",
			Title:        "Already priced",
			ProductURL:   "https://www.broan-nutone.com/en-us/product/ventilationfans/c300",
			SourceFields: []string{"model_discovery: broan-nutone"},
			Offers:       []modelOffer{{Source: "test", Price: 100}},
		},
		"D400": {
			Model:        "D400",
			Title:        "Fourth model",
			ProductURL:   "https://www.broan-nutone.com/en-us/product/ventilationfans/d400",
			SourceFields: []string{"model_discovery: broan-nutone"},
		},
		"F600": {
			Model:        "F600",
			Title:        "Plain source row",
			ProductURL:   "https://example.com/f600",
			SourceFields: []string{"search: plain"},
		},
	}

	candidates := discoveryOfferFallbackCandidates(records, 5)
	if len(candidates) != maxDiscoveryOfferFallbackProbes {
		t.Fatalf("expected capped fallback candidates, got %d: %#v", len(candidates), candidates)
	}
	models := []string{candidates[0].Model, candidates[1].Model, candidates[2].Model, candidates[3].Model}
	if strings.Join(models, ",") != "A100,B200,D400,E500" {
		t.Fatalf("unexpected deterministic candidate order: %#v", models)
	}
}

type errFake string

func (e errFake) Error() string { return string(e) }

func containsString(vals []string, target string) bool {
	for _, val := range vals {
		if val == target {
			return true
		}
	}
	return false
}

func TestSearchResultProductsFromBraveCategoryQuery(t *testing.T) {
	body := `title:"36 induction cooktop",url:"https://search.brave.com/search?q=36+induction+cooktop",description:"shopping",product:{type:"Product",name:"GE Profile PHP6036DWBB 36 Inch Induction Cooktop",url:"https://www.homedepot.com/p/GE-Profile-PHP6036DWBB/330927836",price:"1399.0",offers:[{priceCurrency:"USD",price:"1399.0"}]}
title:"unrelated range",url:"https://example.com/ranges/ABC123",description:"shopping",product:{type:"Product",name:"GE Profile ABC123 30 Inch Electric Range",url:"https://example.com/ranges/ABC123",price:"999.0",offers:[{priceCurrency:"USD",price:"999.0"}]}
title:"36 induction cooktop",url:"https://search.brave.com/search?q=36+induction+cooktop",description:"shopping",product:{type:"Product",name:"Induction Cooktop Virtual Flame | Wi-Fi Flex Zone Burner",url:"https://www.samsung.com/us/cooking-appliances/cooktops/induction-36-built-in-induction-cooktop-with-flex-cookzone-in-stainless-steel-sku-nz36k7880us-aa/",price:"1899.0",offers:[{priceCurrency:"USD",price:"1899.0"}]}
title:"36 induction cooktop",url:"https://search.brave.com/search?q=36+induction+cooktop",description:"shopping",product:{type:"Product",name:"Bosch NIT8661UC 36 Inch Induction Cooktop",url:"https://www.homedepot.com/p/Bosch-NIT8661UC/320451929",price:"2999.0",offers:[{priceCurrency:"USD",price:"2999.0"}]}`

	records := searchResultProductsFromBrave("36 induction cooktop", body, 10)
	if len(records) != 3 {
		t.Fatalf("expected 3 category product records, got %d: %#v", len(records), records)
	}
	if records[0].Model != "PHP6036DWBB" || records[0].BestPrice != 0 || records[0].Offers[0].Price != 1399 {
		t.Fatalf("unexpected first category record: %#v", records[0])
	}
	if records[0].SourceFields != nil {
		t.Fatalf("category search fallback should not invent source fields: %#v", records[0].SourceFields)
	}
	if records[1].Model != "NZ36K7880US-AA" || records[1].Offers[0].Source != "brave-search:samsung.com" {
		t.Fatalf("unexpected second category record: %#v", records[1])
	}
	if records[2].Model != "NIT8661UC" || records[2].Offers[0].Source != "brave-search:homedepot.com" {
		t.Fatalf("unexpected third category record: %#v", records[2])
	}
}

func TestEnrichModelOfferPagesHarvestsSpecsForExactModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><meta name="title" content="JennAir JOESC330RM wall oven"></head><body>
<a href="/docs/joesc330rm-spec-sheet.pdf">Spec Sheet</a>
<a href="/docs/joesc330rm-installation.pdf">Installation Guide</a>
<a href="/docs/joesc330rl-dimensions.pdf">Sibling Dimensions</a>
<a href="/docs/Abt_2026_Review-and-Win-Official-Rules.pdf">Contest Rules</a>
</body></html>`))
	}))
	defer server.Close()

	rec := modelIntelRecord{
		Model: "JOESC330RM",
		Offers: []modelOffer{{
			Source: "brave-search:test-retailer",
			Price:  3999,
			URL:    server.URL + "/product/joesc330rm",
		}},
	}

	enrichModelOfferPages(context.Background(), server.Client(), &rec, 3)
	if rec.ProductURL != server.URL+"/product/joesc330rm" {
		t.Fatalf("expected offer URL to become product URL, got %q", rec.ProductURL)
	}
	if rec.Title != "JennAir JOESC330RM wall oven" {
		t.Fatalf("expected title from offer page, got %q", rec.Title)
	}
	if len(rec.Specs) != 2 {
		t.Fatalf("expected 2 specs from offer page, got %#v", rec.Specs)
	}
	if rec.Specs[0].URL != server.URL+"/docs/joesc330rm-spec-sheet.pdf" {
		t.Fatalf("unexpected first spec URL: %#v", rec.Specs[0])
	}
}

func TestEnrichModelOfferPagesReportsBlockedOfferPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `<title>Just a moment...</title><p>Enable JavaScript and cookies to continue</p>`, http.StatusForbidden)
	}))
	defer server.Close()

	rec := modelIntelRecord{
		Model: "JOESC330RM",
		Offers: []modelOffer{{
			Source: "brave-search:blocked-retailer",
			Title:  "JennAir JOESC330RM Wall Oven",
			Price:  3999,
			URL:    server.URL + "/product/joesc330rm",
		}},
	}

	enrichModelOfferPages(context.Background(), server.Client(), &rec, 3)
	if rec.ProductURL != server.URL+"/product/joesc330rm" {
		t.Fatalf("expected blocked offer URL to remain visible, got %q", rec.ProductURL)
	}
	if rec.Title != "JennAir JOESC330RM Wall Oven" {
		t.Fatalf("expected search result title to remain visible, got %q", rec.Title)
	}
	if len(rec.ProbeStatus) != 1 || rec.ProbeStatus[0].Status != "blocked" {
		t.Fatalf("expected blocked probe status, got %#v", rec.ProbeStatus)
	}
}

func TestShouldUseFinalModelURLKeepsModelSpecificOriginalOnGenericRedirect(t *testing.T) {
	original := "https://www.brayandscarff.com/appliances/cooking/cooktops/cooktops-electric/maytag/mcit8036sb/"
	generic := "https://www.brayandscarff.com/appliances/cooking/cooktops?pnf=1"
	if shouldUseFinalModelURL("MCIT8036SB", original, generic) {
		t.Fatalf("generic redirect should not replace model-specific original URL")
	}

	final := "https://www.example.com/products/mcit8036sb"
	if !shouldUseFinalModelURL("MCIT8036SB", original, final) {
		t.Fatalf("model-specific final URL should be accepted")
	}
}
