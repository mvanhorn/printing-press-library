// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package seeds carries the canonical reference data the
// entity_lookups table ships with. The data is split by domain into
// per-file slices so a future generator template can lift this
// directory wholesale (or per-file selectively) into a new CLI.
//
// Files in this package:
//
//   - countries.go  ISO 3166-1 alpha-2, alpha-3, and lowercase-name
//                   variants. ~750 rows. Public-domain data.
//   - sports.go     Current-season major-league sports team
//                   abbreviations + lowercase-mascot variants for
//                   NFL, NBA, MLB, MLS. ~240 rows. Public-domain
//                   data (team names + their official short forms).
//   - generic.go    Computed-kind placeholder + cross-domain alias
//                   rows that don't fit one domain. Computed kinds
//                   themselves are implemented in
//                   ../store.go::computedLookup; this file documents
//                   their existence and reserves table-shaped seed
//                   data for them in case a future plan needs to
//                   materialize one.
//   - init.go       Seeds() concatenates every domain slice into the
//                   single batch the migration consumes.
//
// Extension protocol for a future CLI lifting this package:
//
//  1. Copy the directory into your CLI's internal/learn/lookups/seeds.
//
//  2. Add a new file (e.g. ratings.go for stock-rating mappings, or
//     guests.go for podcast-show-to-guest aliases). Export a single
//     []lookups.LookupRow variable.
//
//  3. Add that slice to the concatenation in init.go's Seeds().
//
//  4. Keep the existing country and sports rows even if you don't
//     plan to use them — the data is small enough that the cost of
//     keeping them is zero, and a recipe template that names
//     country_iso2 will still resolve if a user pastes one in.
//
// Why not embed.FS over a JSON file: Go literals compile-time-check
// the schema (the LookupRow struct catches typos) and let `go vet`
// flag duplicate keys. A JSON file moves those errors to runtime.
// The binary-size cost is identical either way (the data ends up in
// the rodata segment).
//
// See ../doc.go for the broader package design and
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// section U9 for the rationale on hard-coded SQLite seeds.
package seeds
