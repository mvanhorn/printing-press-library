// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// PATCH(amend-2026-05-20: value-compare) — Program registry for the
// pluggable valuation seam. v1 ships ProgramAtmos only; adding more
// airlines later (United, AAdvantage, etc.) is a single struct-literal
// append in the registry slice below.

// Package valuation looks up cents-per-point values for loyalty programs
// from external sources (currently The Points Guy's monthly valuations
// page), caches them on disk, and computes apples-to-apples cash-vs-
// points comparison math.
package valuation

// Program is a typed slug identifying a loyalty program.
type Program string

const (
	// ProgramAtmos is Alaska Airlines' Atmos Rewards program (formerly Mileage Plan).
	ProgramAtmos Program = "atmos"
)

// ProgramDef carries the metadata needed to scrape TPG's valuations page
// for a given program and to fall back when the scrape fails.
type ProgramDef struct {
	// Slug is the Program identifier (e.g. "atmos").
	Slug Program
	// Display is the human-readable program name used in CLI output.
	Display string
	// TPGRowMatch is the case-insensitive substring used to identify the
	// program's row in TPG's monthly-valuations table. TPG occasionally
	// renames programs (Atmos was "Mileage Plan" before 2025), so this
	// is the controlled point of adaptation.
	TPGRowMatch string
	// FallbackCPP is the cents-per-point value used when both the live
	// TPG scrape AND any on-disk cache are unavailable. Update via PR
	// alongside any meaningful TPG drift.
	FallbackCPP float64
}

// programs is the flat registry. Append a row to add a new program.
var programs = []ProgramDef{
	{
		Slug:        ProgramAtmos,
		Display:     "Alaska Airlines Atmos Rewards",
		TPGRowMatch: "alaska airlines atmos",
		FallbackCPP: 1.4,
	},
}

// BySlug returns the ProgramDef for the given slug.
func BySlug(p Program) (ProgramDef, bool) {
	for _, def := range programs {
		if def.Slug == p {
			return def, true
		}
	}
	return ProgramDef{}, false
}

// Slugs returns every registered program slug (useful for help text and
// validation messages).
func Slugs() []Program {
	out := make([]Program, 0, len(programs))
	for _, def := range programs {
		out = append(out, def.Slug)
	}
	return out
}
