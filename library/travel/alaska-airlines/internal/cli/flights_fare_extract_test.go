// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// buildSvelteKitFixture wraps an arbitrary root JSON object in the
// SvelteKit `{"type":"chunk","id":"_","data":[...]}` envelope that the
// alaskaair.com endpoint emits. The data array is built by walking the
// supplied root and replacing every container with a positional
// pointer; primitives are inlined.
//
// This lets tests author payloads in normal Go-map form and have them
// re-encoded as if they came off the wire.
func buildSvelteKitFixture(root any) string {
	chunk := []any{}
	var encode func(v any) int
	encode = func(v any) int {
		switch x := v.(type) {
		case map[string]any:
			node := make(map[string]any, len(x))
			idx := len(chunk)
			chunk = append(chunk, node) // reserve slot first to allow self-refs
			for k, vv := range x {
				switch vt := vv.(type) {
				case map[string]any, []any:
					node[k] = encode(vt)
				default:
					node[k] = vt
				}
			}
			chunk[idx] = node
			return idx
		case []any:
			arr := make([]any, len(x))
			idx := len(chunk)
			chunk = append(chunk, arr)
			for i, vv := range x {
				switch vt := vv.(type) {
				case map[string]any, []any:
					arr[i] = encode(vt)
				default:
					arr[i] = vt
				}
			}
			chunk[idx] = arr
			return idx
		default:
			idx := len(chunk)
			chunk = append(chunk, x)
			return idx
		}
	}
	encode(root)
	envelope := map[string]any{
		"type": "chunk",
		"id":   "test",
		"data": chunk,
	}
	out, _ := json.Marshal(envelope)
	return string(out)
}

func segments(carriers ...string) []any {
	segs := make([]any, len(carriers))
	for i, c := range carriers {
		segs[i] = map[string]any{
			"displayCarrier": map[string]any{
				"carrierCode":  c,
				"flightNumber": float64(100 + i),
			},
		}
	}
	return segs
}

// makeRow builds a row map with the given segments and a solutions
// map. The solutions arg is keyed by raw cabin slot name
// ("MAIN", "BUSINESS", "REFUNDABLE_MAIN", etc.).
func makeRow(segs []any, solutions map[string]any) map[string]any {
	return map[string]any{
		"segments":  segs,
		"solutions": solutions,
	}
}

func TestExtractLowestFare_Award_FindsLowest(t *testing.T) {
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":     map[string]any{"atmosPoints": float64(70000), "grandTotal": float64(5.6)},
				"REFUNDABLE_BUSINESS": map[string]any{"atmosPoints": float64(150000), "grandTotal": float64(5.6)},
			}),
			makeRow(segments("JX", "JX"), map[string]any{
				"REFUNDABLE_MAIN":     map[string]any{"atmosPoints": float64(45000), "grandTotal": float64(33.9)},
				"REFUNDABLE_BUSINESS": map[string]any{"atmosPoints": float64(300000), "grandTotal": float64(33.9)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeAward, "", -1, 0)
	if got.Miles == nil {
		t.Fatalf("expected miles extracted, got %+v", got)
	}
	if *got.Miles != 45000 {
		t.Errorf("Miles = %d; want 45000", *got.Miles)
	}
	if got.CashUSD != 33.9 {
		t.Errorf("CashUSD (taxes) = %v; want 33.9", got.CashUSD)
	}
}

func TestExtractLowestFare_Award_MaxStopsFilter(t *testing.T) {
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN": map[string]any{"atmosPoints": float64(70000), "grandTotal": float64(5.6)},
			}),
			makeRow(segments("JX", "JX"), map[string]any{
				"REFUNDABLE_MAIN": map[string]any{"atmosPoints": float64(45000), "grandTotal": float64(33.9)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeAward, "", 0, 0) // nonstop only
	if got.Miles == nil {
		t.Fatalf("expected miles, got nil")
	}
	if *got.Miles != 70000 {
		t.Errorf("max-stops=0 should pick nonstop 70000, got %d", *got.Miles)
	}
	if got.Stops != 0 {
		t.Errorf("Stops = %d; want 0", got.Stops)
	}
}

func TestExtractLowestFare_Award_CabinFilter(t *testing.T) {
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":     map[string]any{"atmosPoints": float64(70000), "grandTotal": float64(5.6)},
				"REFUNDABLE_BUSINESS": map[string]any{"atmosPoints": float64(150000), "grandTotal": float64(5.6)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeAward, "business", -1, 0)
	if got.Miles == nil {
		t.Fatalf("expected miles, got nil")
	}
	if *got.Miles != 150000 {
		t.Errorf("cabin=business should pick 150000 BUSINESS row, got %d", *got.Miles)
	}
	if !strings.Contains(strings.ToLower(got.Cabin), "business") {
		t.Errorf("Cabin = %q; want a business slot", got.Cabin)
	}
}

func TestExtractLowestFare_Cash_FindsLowest(t *testing.T) {
	// Cash-mode rows carry atmosPoints = -1 (sentinel) and grandTotal
	// is the cash fare.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"BUSINESS": map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(5145.43)},
				"MAIN":     map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(1766.23)},
				"PREMIUM":  map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(2083.23)},
				"SAVER":    map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(1626.23)},
			}),
			makeRow(segments("BA", "AS"), map[string]any{
				"MAIN":  map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(2237.23)},
				"SAVER": map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(1875.23)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeCash, "economy", -1, 0)
	// Economy filter accepts MAIN and SAVER. Lowest is SAVER on row 0 = 1626.23.
	if got.Miles != nil {
		t.Errorf("cash mode should return nil Miles, got %v", got.Miles)
	}
	if got.CashUSD != 1626.23 {
		t.Errorf("CashUSD = %v; want 1626.23", got.CashUSD)
	}
}

func TestExtractLowestFare_Cash_NonstopOnly(t *testing.T) {
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"MAIN": map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(1766.23)},
			}),
			makeRow(segments("BA", "AS"), map[string]any{
				"MAIN": map[string]any{"atmosPoints": float64(-1), "grandTotal": float64(900.00)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeCash, "economy", 0, 0)
	if got.CashUSD != 1766.23 {
		t.Errorf("nonstop cash = %v; want 1766.23 (the cheaper 900 has 1 stop)", got.CashUSD)
	}
	if got.Stops != 0 {
		t.Errorf("Stops = %d; want 0", got.Stops)
	}
}

func TestExtractLowestFare_Cash_SkipsAwardRows(t *testing.T) {
	// Award rows in the response (atmosPoints > 0) must NOT be picked
	// up in cash mode.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"MAIN": map[string]any{"atmosPoints": float64(45000), "grandTotal": float64(5.6)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeCash, "", -1, 0)
	if got.CashUSD != 0 {
		t.Errorf("cash mode against award-only data should yield 0; got %v", got.CashUSD)
	}
}

func TestExtractLowestFare_Carrier_FromFirstSegment(t *testing.T) {
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS", "JX"), map[string]any{
				"MAIN": map[string]any{"atmosPoints": float64(50000), "grandTotal": float64(20)},
			}),
		},
	}
	fix := buildSvelteKitFixture(root)
	got := extractLowestFare(json.RawMessage(fix), fareModeAward, "", -1, 0)
	if got.Carrier != "AS" {
		t.Errorf("Carrier = %q; want AS", got.Carrier)
	}
}

func TestExtractLowestFare_Award_RankingByMilesOnly(t *testing.T) {
	// Two options: 25k+$500 (low miles, high taxes) and 30k+$5 (more
	// miles, lower taxes). With rankingCPP=0, the legacy "minimum
	// miles" ranking picks 25k. award-cheapest needs this behavior.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":  map[string]any{"atmosPoints": float64(25000), "grandTotal": float64(500)},
				"REFUNDABLE_SAVER": map[string]any{"atmosPoints": float64(30000), "grandTotal": float64(5)},
			}),
		},
	}
	got := extractLowestFare(json.RawMessage(buildSvelteKitFixture(root)), fareModeAward, "", -1, 0)
	if got.Miles == nil || *got.Miles != 25000 {
		t.Errorf("with rankingCPP=0, want 25000-mile option; got %+v", got)
	}
}

func TestExtractLowestFare_Award_RankingByTotalCost(t *testing.T) {
	// Same options, but rankingCPP=1.4 means the 30k+$5 option's
	// total ($420 + $5 = $425) is dramatically cheaper than the
	// 25k+$500 option ($350 + $500 = $850). value-compare needs this
	// behavior so the comparison reflects the best real redemption.
	root := map[string]any{
		"rows": []any{
			makeRow(segments("AS"), map[string]any{
				"REFUNDABLE_MAIN":  map[string]any{"atmosPoints": float64(25000), "grandTotal": float64(500)},
				"REFUNDABLE_SAVER": map[string]any{"atmosPoints": float64(30000), "grandTotal": float64(5)},
			}),
		},
	}
	got := extractLowestFare(json.RawMessage(buildSvelteKitFixture(root)), fareModeAward, "", -1, 1.4)
	if got.Miles == nil || *got.Miles != 30000 {
		t.Errorf("with rankingCPP=1.4, want 30000-mile option (lower total cost); got %+v", got)
	}
}

func TestExtractLowestFare_Empty(t *testing.T) {
	got := extractLowestFare(json.RawMessage(`{}`), fareModeAward, "", -1, 0)
	if got.Miles != nil || got.CashUSD != 0 {
		t.Errorf("empty response should yield zero-value fare; got %+v", got)
	}
}

func TestExtractLowestFare_Cabin_Normalization(t *testing.T) {
	cases := []struct {
		filter   string
		cabinKey string
		matches  bool
	}{
		{"economy", "MAIN", true},
		{"economy", "REFUNDABLE_MAIN", true},
		{"economy", "SAVER", true},
		{"economy", "BUSINESS", false},
		{"coach", "MAIN", true},
		{"business", "BUSINESS", true},
		{"business", "REFUNDABLE_BUSINESS", true},
		{"premium", "PREMIUM", true},
		{"premium", "REFUNDABLE_PARTNER_PREMIUM", true},
		{"", "MAIN", true}, // empty filter matches all
		{"any", "BUSINESS", true},
	}
	for _, c := range cases {
		norm := normalizeCabinFilter(c.filter)
		got := norm == "" || cabinMatches(c.cabinKey, norm)
		if got != c.matches {
			t.Errorf("filter=%q cabinKey=%q: matched=%v want=%v (norm=%q)", c.filter, c.cabinKey, got, c.matches, norm)
		}
	}
}

// Sanity check that the SvelteKit fixture helper round-trips through
// the hydrator: build a payload, hydrate it, walk back to the leaves.
func TestSvelteKitFixture_RoundTrip(t *testing.T) {
	root := map[string]any{
		"hello": "world",
		"nest":  map[string]any{"deep": float64(42)},
	}
	fix := buildSvelteKitFixture(root)
	hydrated, ok := hydrateSearchResponse(json.RawMessage(fix))
	if !ok {
		t.Fatalf("hydrate returned ok=false; payload:\n%s", fix)
	}
	if hydrated["hello"] != "world" {
		t.Errorf("hello = %v; want world", hydrated["hello"])
	}
	nest, ok := hydrated["nest"].(map[string]any)
	if !ok {
		t.Fatalf("nest is %T; want map[string]any", hydrated["nest"])
	}
	if fmt.Sprint(nest["deep"]) != "42" {
		t.Errorf("nest.deep = %v; want 42", nest["deep"])
	}
}
