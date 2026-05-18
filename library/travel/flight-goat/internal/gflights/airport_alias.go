// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(library): airport alias table for IATA codes that have been retired
// or replaced. Google's GetShoppingResults silently returns an empty payload
// for decommissioned codes, so a CLI that passes the raw user input gets
// zero results with no signal that anything is wrong. This table is the
// fix at the request edge: substitute the current code before the call,
// surface the substitution in the response envelope (see SearchResult.AirportRemapped).
//
// Entries are added when a route stops returning results because its IATA
// code changed. Keep the map sorted by key for review readability.
//
//   PNH -> KTI: Phnom Penh International closed September 2025, replaced by
//               Techo International Airport. Google migrated to KTI.
//   REP -> SAI: Siem Reap International replaced by Siem Reap-Angkor
//               International (SAI) in 2024.

package gflights

import "strings"

var airportAliases = map[string]string{
	"PNH": "KTI",
	"REP": "SAI",
}

// AirportRemap records a single airport-code substitution. Callers inspect
// Changed to know whether the substitution actually happened.
type AirportRemap struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Changed bool   `json:"-"`
}

// AirportRemapNote is the response-envelope echo. Origin and Destination
// are populated only when the corresponding code was remapped; both are
// omitted from JSON when nil.
type AirportRemapNote struct {
	Origin      *AirportRemap `json:"origin,omitempty"`
	Destination *AirportRemap `json:"destination,omitempty"`
}

// remapAirport normalizes a single airport code. If the input matches a
// retired IATA code, the returned remap carries the current code and
// Changed=true. Otherwise the input passes through (uppercased) unchanged.
func remapAirport(code string) AirportRemap {
	normalized := strings.ToUpper(strings.TrimSpace(code))
	if normalized == "" {
		return AirportRemap{}
	}
	if current, ok := airportAliases[normalized]; ok {
		return AirportRemap{From: normalized, To: current, Changed: true}
	}
	return AirportRemap{From: normalized, To: normalized, Changed: false}
}

// remapAirportPair applies remapAirport to both ends of an origin/destination
// pair. The returned AirportRemapNote is nil when neither code was remapped,
// so callers can attach it directly to a result envelope with omitempty.
func remapAirportPair(origin, destination string) (AirportRemap, AirportRemap, *AirportRemapNote) {
	o := remapAirport(origin)
	d := remapAirport(destination)
	if !o.Changed && !d.Changed {
		return o, d, nil
	}
	note := &AirportRemapNote{}
	if o.Changed {
		oc := o
		note.Origin = &oc
	}
	if d.Changed {
		dc := d
		note.Destination = &dc
	}
	return o, d, note
}
