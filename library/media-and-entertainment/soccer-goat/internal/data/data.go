// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package data embeds the bundled potential dataset so the CLI ships it in the
// binary. See SOURCE.md for provenance and the refresh procedure.
package data

import _ "embed"

// PotentialSofifa2025GZ is the gzip-compressed CSV (ea_id,name,overall,potential),
// one row per player from a June 2025 sofifa snapshot (FC25 era).
//
//go:embed potential-sofifa-2025.csv.gz
var PotentialSofifa2025GZ []byte
