// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — Structured fare extractor.
//
// Alaska's /search/results/__data.json returns a SvelteKit-encoded
// payload: a multi-part stream where the largest "chunk" carries a
// positionally-encoded array. Each cell is either a scalar or a small
// object/array whose values may be integer pointers back into the same
// array. To read fare data, we first hydrate the array (resolve int
// pointers to their pointed-to nodes) and then walk the resulting tree
// for the well-known shape:
//
//   root.rows[i].solutions.<CABIN>.{ atmosPoints, grandTotal, seatsRemaining }
//   root.rows[i].segments[j].displayCarrier.{ carrierCode, flightNumber }
//
// `atmosPoints` is -1 for cash-mode rows and a positive integer for
// award-mode rows. `grandTotal` carries the cash $ for cash mode and
// taxes-and-fees for award mode.
//
// The earlier "tolerant tree walker" in flights_award_cheapest.go used
// field names (milesAmount, cashAmount) that the live API does not emit
// — so it returned no extractions in production. This file replaces
// that path with a structured extractor that knows the actual shape
// observed during the 2026-05-19 amend sniff and the 2026-05-20
// value-compare planning sniff.

package cli

import (
	"encoding/json"
	"strings"
)

// fareMode discriminates which mode the caller wants to extract.
type fareMode int

const (
	fareModeAward fareMode = iota
	fareModeCash
)

// lowestFare is the unified shape returned by the structured extractor.
// For award mode, Miles is non-nil and CashUSD carries taxes. For cash
// mode, Miles is nil and CashUSD carries the full fare.
type lowestFare struct {
	Miles   *int
	CashUSD float64
	Carrier string
	Cabin   string // raw cabin slot key, e.g. "MAIN", "BUSINESS"
	Stops   int
}

// extractLowestFare hydrates the SvelteKit payload and returns the
// lowest-priced fare matching the requested mode, cabin filter, and
// maxStops limit. When cabinFilter is empty, all cabins are eligible.
// When maxStops < 0, the stops constraint is ignored.
//
// rankingCPP controls how award candidates are compared (cash mode
// always ranks by total cash). When rankingCPP > 0, award candidates
// are ranked by their TPG-valued total cost (miles*cpp/100 + taxes);
// this is the right criterion for value-compare, where a 30k+$5
// option strictly beats a 25k+$500 option at any realistic cpp. When
// rankingCPP == 0 (e.g. award-cheapest, where no cpp is known at
// extract time), candidates are ranked by miles only — the legacy
// behavior. Callers that know the cpp before extracting should pass
// it; callers that don't can pass 0 and the ranking falls back to
// the miles minimum.
func extractLowestFare(data json.RawMessage, mode fareMode, cabinFilter string, maxStops int, rankingCPP float64) lowestFare {
	root, ok := hydrateSearchResponse(data)
	if !ok {
		return lowestFare{}
	}
	rows, _ := root["rows"].([]any)
	if len(rows) == 0 {
		return lowestFare{}
	}

	wantCabin := normalizeCabinFilter(cabinFilter)

	var best *lowestFare
	for _, rowAny := range rows {
		row, ok := rowAny.(map[string]any)
		if !ok {
			continue
		}
		stops := countStops(row)
		if maxStops >= 0 && stops > maxStops {
			continue
		}
		sols, ok := row["solutions"].(map[string]any)
		if !ok {
			continue
		}
		carrier, _ := primaryCarrier(row)
		for cabinKey, solAny := range sols {
			sol, ok := solAny.(map[string]any)
			if !ok {
				continue
			}
			if wantCabin != "" && !cabinMatches(cabinKey, wantCabin) {
				continue
			}
			points, hasPoints := readSolutionInt(sol, "atmosPoints", "allPaxPoints")
			cash, hasCash := readSolutionFloat(sol, "grandTotal", "allPaxTotal")

			switch mode {
			case fareModeAward:
				// Award rows have atmosPoints > 0; cash-mode rows
				// carry the sentinel -1.
				if !hasPoints || points <= 0 {
					continue
				}
				miles := points
				candidate := lowestFare{
					Miles:   &miles,
					CashUSD: cash, // taxes for award mode
					Carrier: carrier,
					Cabin:   cabinKey,
					Stops:   stops,
				}
				if best == nil || awardCandidateBetter(candidate, *best, rankingCPP) {
					best = &candidate
				}
			case fareModeCash:
				// Cash rows have atmosPoints == -1 (or missing); a
				// positive atmosPoints means this is an award fare
				// dressed as cash — skip.
				if hasPoints && points > 0 {
					continue
				}
				if !hasCash || cash <= 0 {
					continue
				}
				candidate := lowestFare{
					CashUSD: cash,
					Carrier: carrier,
					Cabin:   cabinKey,
					Stops:   stops,
				}
				if best == nil || candidate.CashUSD < best.CashUSD {
					best = &candidate
				}
			}
		}
	}

	if best == nil {
		return lowestFare{}
	}
	return *best
}

// hydrateSearchResponse parses the SvelteKit multi-part response,
// finds the largest data chunk, and resolves positional int pointers
// into a normal Go map tree. Returns (root, true) on success.
func hydrateSearchResponse(data json.RawMessage) (map[string]any, bool) {
	// The response is one or more concatenated JSON values. The biggest
	// `data` array holds the encoded fare tree. Decode multiple values
	// from the stream.
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.UseNumber()
	var chunk []any
	for {
		var part any
		if err := dec.Decode(&part); err != nil {
			break
		}
		obj, ok := part.(map[string]any)
		if !ok {
			continue
		}
		raw, ok := obj["data"]
		if !ok {
			continue
		}
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		if len(arr) > len(chunk) {
			chunk = arr
		}
	}
	if len(chunk) == 0 {
		return nil, false
	}

	// Resolve positional pointers. Element 0 is the root.
	seen := make(map[int]any, len(chunk))
	var hyd func(idx int, depth int) any
	hyd = func(idx int, depth int) any {
		if depth > 200 {
			return nil
		}
		if v, ok := seen[idx]; ok {
			return v
		}
		if idx < 0 || idx >= len(chunk) {
			return nil
		}
		raw := chunk[idx]
		seen[idx] = nil // mark in-progress to break cycles
		resolved := resolveNode(raw, chunk, hyd, depth)
		seen[idx] = resolved
		return resolved
	}
	root, ok := hyd(0, 0).(map[string]any)
	return root, ok
}

// resolveNode resolves int-pointer values inside a node by recursing
// through hyd; non-int values are kept as-is.
func resolveNode(v any, chunk []any, hyd func(int, int) any, depth int) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, vv := range x {
			out[k] = resolveValue(vv, chunk, hyd, depth+1)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, vv := range x {
			out[i] = resolveValue(vv, chunk, hyd, depth+1)
		}
		return out
	default:
		return x
	}
}

// resolveValue resolves a single value: if it looks like a positional
// pointer (a small non-negative integer within chunk bounds), recurse;
// otherwise return as-is. json.Number is used to detect numeric values
// in the decoded tree.
func resolveValue(v any, chunk []any, hyd func(int, int) any, depth int) any {
	if n, ok := v.(json.Number); ok {
		if i, err := n.Int64(); err == nil {
			idx := int(i)
			if idx >= 0 && idx < len(chunk) {
				return hyd(idx, depth+1)
			}
		}
		// Non-int or out-of-range: treat as a literal numeric value.
		if f, err := n.Float64(); err == nil {
			return f
		}
		return v
	}
	if m, ok := v.(map[string]any); ok {
		return resolveNode(m, chunk, hyd, depth)
	}
	if a, ok := v.([]any); ok {
		return resolveNode(a, chunk, hyd, depth)
	}
	return v
}

// awardCandidateBetter ranks two award candidates. When cpp > 0, the
// ranking is by total TPG-valued cost (miles*cpp/100 + taxes); this is
// what value-compare wants since a 30k+$5 option strictly beats a
// 25k+$500 option at any realistic cpp. When cpp == 0, fall back to
// minimum miles — the legacy behavior used by award-cheapest where no
// baseline cpp is known at extract time.
func awardCandidateBetter(candidate, best lowestFare, cpp float64) bool {
	if cpp > 0 {
		candTotal := float64(*candidate.Miles)*cpp/100 + candidate.CashUSD
		bestTotal := float64(*best.Miles)*cpp/100 + best.CashUSD
		return candTotal < bestTotal
	}
	return *candidate.Miles < *best.Miles
}

// primaryCarrier reads the first segment's carrier code. Returns ""
// when the structure is unexpected.
func primaryCarrier(row map[string]any) (string, bool) {
	segs, ok := row["segments"].([]any)
	if !ok || len(segs) == 0 {
		return "", false
	}
	first, ok := segs[0].(map[string]any)
	if !ok {
		return "", false
	}
	disp, _ := first["displayCarrier"].(map[string]any)
	if disp == nil {
		disp, _ = first["publishingCarrier"].(map[string]any)
	}
	if disp == nil {
		return "", false
	}
	if code, ok := disp["carrierCode"].(string); ok {
		return code, true
	}
	return "", false
}

// countStops returns len(segments) - 1, clamped to zero.
func countStops(row map[string]any) int {
	segs, ok := row["segments"].([]any)
	if !ok {
		return 0
	}
	n := len(segs) - 1
	if n < 0 {
		return 0
	}
	return n
}

// readSolutionInt extracts the first integer-shaped field from a
// solution map.
func readSolutionInt(sol map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := sol[k]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case json.Number:
			if i, err := x.Int64(); err == nil {
				return int(i), true
			}
		case float64:
			return int(x), true
		case int:
			return x, true
		}
	}
	return 0, false
}

// readSolutionFloat extracts the first float-shaped field from a
// solution map.
func readSolutionFloat(sol map[string]any, keys ...string) (float64, bool) {
	for _, k := range keys {
		v, ok := sol[k]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case json.Number:
			if f, err := x.Float64(); err == nil {
				return f, true
			}
		case float64:
			return x, true
		case int:
			return float64(x), true
		}
	}
	return 0, false
}

// normalizeCabinFilter maps human cabin names to lowercase tokens used
// in the cabin matcher.
func normalizeCabinFilter(cabin string) string {
	switch strings.ToLower(strings.TrimSpace(cabin)) {
	case "", "any":
		return ""
	case "economy", "coach", "main":
		return "economy"
	case "premium", "premium economy":
		return "premium"
	case "business":
		return "business"
	case "first":
		return "first"
	default:
		return strings.ToLower(cabin)
	}
}

// cabinMatches returns true when the raw cabin key (e.g. "MAIN",
// "REFUNDABLE_BUSINESS", "REFUNDABLE_PARTNER_PREMIUM") belongs to the
// requested normalized cabin family.
func cabinMatches(rawCabinKey, normalized string) bool {
	k := strings.ToLower(rawCabinKey)
	switch normalized {
	case "economy":
		// MAIN, REFUNDABLE_MAIN, SAVER (Alaska's cheap economy bucket).
		return strings.Contains(k, "main") || strings.Contains(k, "saver")
	case "premium":
		return strings.Contains(k, "premium")
	case "business":
		return strings.Contains(k, "business")
	case "first":
		return strings.Contains(k, "first")
	default:
		return strings.Contains(k, normalized)
	}
}
