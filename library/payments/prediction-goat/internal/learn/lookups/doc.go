// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package lookups provides a generic canonical-to-value mapping table
// (entity_lookups) plus computed-kind short-circuits (lowercase,
// uppercase, kebab-case, capitalize-first, slug). It is the
// substitution backbone for the recipe engine in
// internal/learn/recipes: a recipe like
//
//	"odds {country} wins world cup" -> "KXMENWORLDCUP-26-{country:country_iso2}"
//
// resolves at apply time by calling Lookup("country_iso2", "England")
// and getting back "GB". The recipe inference engine reads the table
// the same way to bind variable positions across two or more teaches.
//
// Why hard-coded SQLite seeds are OK:
//
//   - The seeded payload is *canonical reference data* (ISO 3166-1
//     country codes, current-season major-league sports team
//     abbreviations), not *domain logic*. The data does not know
//     anything about Polymarket, Kalshi, or prediction markets in
//     general. A podcast CLI, a stocks CLI, or a recipe CLI can ship
//     the same `country_iso2` and `nfl_team_lowercase` rows verbatim
//     and the lookup surface keeps working.
//
//   - Shipping the seeds inside SQLite (instead of a JSON file or an
//     embed.FS scan) lets a per-user `teach-lookup` row coexist with
//     the seeded row in the same table, queried through the same
//     surface, with no two-tier resolution logic. Source is tracked
//     via the `source` column ('seeded' | 'taught' | 'inferred') so
//     diagnostics can tell them apart.
//
//   - The data is small (~500 rows total, ~30KB compiled into the
//     binary as Go literals). Re-running the seed inserts via
//     INSERT OR IGNORE is idempotent and runs once per fresh DB.
//
// Computed kinds bypass the DB entirely. Asking for
// `Lookup("lowercase", "Portugal")` returns ("portugal", true) without
// touching SQLite — these are pure string transforms registered as
// kinds so recipes can name them in the same syntax used for
// table-backed kinds. The set of computed kinds is closed (see
// store.go computedKind/computedLookup); adding a new one is a
// source-code change.
//
// Extension points for a new CLI:
//
//   - Reuse the package as-is. The Lookup / LookupAll / Upsert API
//     is domain-agnostic and the table is already in SQLite via the
//     v4->v5 migration.
//
//   - Add domain-specific kinds either by extending the seeds
//     subpackage (preferred for canonical reference data that ships
//     with the binary; copy `seeds/sports.go` as a template) or by
//     calling Upsert at runtime (preferred for per-user additions or
//     dynamically discovered mappings).
//
//   - For per-user runtime additions, expose a `teach-lookup` CLI
//     command in your CLI's internal/cli/ package; prediction-goat's
//     teach_lookup.go is the reference shape.
//
// See docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md
// section U9 ("Entity-lookup table + seeded data + teach-lookup
// command") for the full design rationale.
package lookups
