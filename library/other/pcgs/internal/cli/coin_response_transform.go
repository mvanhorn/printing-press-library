// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-18: coin-response-transforms) — Single helper that
// post-processes raw PCGS coin records before they leave the CLI:
//
//  1. Converts PriceGuideValue: 0 to PriceGuideValue: null so downstream
//     consumers can distinguish "PCGS hasn't priced this" (common on modern
//     David Hall FDI bullion) from "this coin is genuinely worth zero". See
//     AGENTS.md P3.
//
//  2. Surfaces year/name disagreement as a top-level year_mismatch object
//     when the year prefix parsed from Name (regex ^\d{4}, allowing optional
//     (P) prefix) differs from the integer Year field. PCGS returns a
//     handful of certs per inventory pass where Name="2022-S ..." but
//     Year=2021; without this, the agent silently picks one and the conflict
//     stays hidden. See AGENTS.md P4.
//
//  3. PATCH(amend-2026-05-19: images-field-strip) — Drops the noisy
//     Images: [{}, {}] array PCGS returns on CoinFactsModel responses. The
//     two stub items have no URL fields (image URLs live on a separate
//     endpoint exposed as `coin images <cert>`) and read as "the image
//     fetch failed" to a first-time consumer. Removing the field entirely
//     leaves the HasObverseImage / HasReverseImage / HasTrueViewImage /
//     ImageReady booleans intact so callers can still tell images exist
//     before paying the second quota call. See AGENTS.md P6.
//
//     Safe because the envelope classifier (classifyPCGSEnvelope in
//     pcgs_envelope.go) already runs inside resolveRead — well before this
//     transform — and consumes the Images=[] + all-Has*Image-false
//     heuristic on the GetImagesByCertNo endpoint. Stripping Images at
//     the response-transform layer cannot affect bogus-cert detection.
//
// Both transforms are idempotent and safe on non-coin JSON (the function
// passes through invalid/non-object payloads unchanged), so wiring it into
// every coin response path is non-breaking.

package cli

import (
	"encoding/json"
	"regexp"
	"strconv"
)

// nameYearPattern matches the year prefix at the start of a PCGS coin Name,
// allowing an optional "(P)" mint-prefix (used on some Philadelphia issues).
// Examples that match:
//
//	"2022-S $1 Silver Eagle First Strike"            → 2022
//	"(P) 1965 50C SMS SP67"                          → 1965
//	"1881-S $1"                                      → 1881
//
// Examples that don't match (returns "", caller treats as "no parsable year"):
//
//	"No Date Cent VF20"                              → ""
//	"Half Cent 1796"                                 → ""  (year not at start)
var nameYearPattern = regexp.MustCompile(`^\s*(?:\(P\)\s*)?(\d{4})`)

// applyCoinResponseTransforms post-processes a single PCGS coin record (the
// CoinFactsModel / CoinFactsByGradeModel / CoinFactsByBarcodeModel shape) to
// apply two compatible transforms:
//
//   - PriceGuideValue: 0 → PriceGuideValue: null
//   - year_mismatch field added when Name year != Year field
//
// The function unmarshals into map[string]json.RawMessage so unrelated
// fields pass through verbatim (no float-precision loss). Keys are
// re-emitted in alphabetical order — Go's encoding/json marshals
// map[string]X with sorted keys, which differs from the original PCGS
// response order. Any byte-for-byte diff against a previously-cached
// payload will flag every field as moved on first read after upgrade.
// When the payload is not a JSON object — e.g. an array, null, or
// non-JSON garbage — the original bytes are returned unchanged.
func applyCoinResponseTransforms(data json.RawMessage) json.RawMessage {
	if len(data) == 0 {
		return data
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return data
	}
	if obj == nil {
		return data
	}

	// P3 — PriceGuideValue: 0 → null
	if raw, ok := obj["PriceGuideValue"]; ok && isZeroNumber(raw) {
		obj["PriceGuideValue"] = json.RawMessage("null")
	}

	// P4 — year_mismatch detection
	if _, alreadyPresent := obj["year_mismatch"]; !alreadyPresent {
		if mismatch := computeYearMismatch(obj["Name"], obj["Year"]); mismatch != nil {
			encoded, encErr := json.Marshal(mismatch)
			if encErr == nil {
				obj["year_mismatch"] = encoded
			}
		}
	}

	// PATCH(amend-2026-05-19: images-field-strip) — P6: drop the noisy
	// Images stub array. Idempotent (delete on absent map key is a no-op).
	// Has*Image / ImageReady booleans elsewhere in the object are left
	// untouched so callers can still tell images exist before paying the
	// second quota call to `coin images <cert>`.
	delete(obj, "Images")

	out, err := json.Marshal(obj)
	if err != nil {
		// Re-marshal failure (vanishingly unlikely after a successful
		// Unmarshal) — return original to avoid silent payload corruption.
		return data
	}
	return out
}

// isZeroNumber reports whether the raw JSON value represents the numeric
// zero in any of the textual forms json.Marshal might produce: "0", "0.0",
// "0e0", etc. A simple strconv.ParseFloat handles all of them; the explicit
// check that the parse succeeded AND the result equals zero prevents bare
// "null"/"\"\"" from being mistreated as zero.
func isZeroNumber(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	v, err := strconv.ParseFloat(string(raw), 64)
	if err != nil {
		return false
	}
	return v == 0
}

// computeYearMismatch parses the year prefix from Name and compares it to
// the Year field. Returns nil (no mismatch field emitted) when:
//
//   - Name is missing or carries no parsable 4-digit year prefix
//   - Year is missing, null, or zero (zero is PCGS's "unknown" sentinel)
//   - The two values agree
//
// Returns a map with the two integer values when they disagree.
func computeYearMismatch(nameRaw, yearRaw json.RawMessage) map[string]int {
	var name string
	if err := json.Unmarshal(nameRaw, &name); err != nil || name == "" {
		return nil
	}
	m := nameYearPattern.FindStringSubmatch(name)
	if len(m) < 2 {
		return nil
	}
	nameYear, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}

	// Year is typed int32 in the swagger but PCGS occasionally returns it
	// as a JSON string in older payloads — accept either shape.
	var yearField int
	if len(yearRaw) > 0 {
		if err := json.Unmarshal(yearRaw, &yearField); err != nil {
			var s string
			if err2 := json.Unmarshal(yearRaw, &s); err2 != nil {
				return nil
			}
			y, parseErr := strconv.Atoi(s)
			if parseErr != nil {
				return nil
			}
			yearField = y
		}
	}
	if yearField == 0 {
		return nil
	}
	if nameYear == yearField {
		return nil
	}
	return map[string]int{
		"name_year":  nameYear,
		"year_field": yearField,
	}
}
