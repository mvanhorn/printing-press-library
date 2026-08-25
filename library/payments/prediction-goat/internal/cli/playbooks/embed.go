// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// Package playbooks ships the prediction-goat-specific playbook +
// notes content as an embedded filesystem. The auto-install path in
// internal/cli/playbook_init.go reads from FS at first DB open and
// seeds the learning_playbooks table.
//
// Convention (designed to copy cleanly to every CLI):
//   - <family>.json holds the steps + entity_slots + expected_tool_calls
//   - <family>_notes.md holds gotchas / workarounds (read verbatim
//     by the agent at recall time)
//   - MANIFEST.md keeps //go:embed *.md matching at least one file
//     even when no playbook content exists
//
// Bump SeedVersion when the embedded content changes so existing
// installs re-seed on the next CLI invocation.
//
// PATCH(learn-loop-backport U9): ported from ESPN PR #851 HEAD
// 9bb0a40a (library/media-and-entertainment/espn/internal/cli/
// playbooks/embed.go). SeedVersion flavored for prediction-goat;
// hand-authored content lands in U10.
// PATCH(learn-loop-backport U10): embed pattern now includes *.json
// alongside *.md to pick up hand-authored playbook content; SeedVersion
// bumped to 002 so existing installs re-seed the new content. Three
// shipped playbooks cover the highest-value repeat-query shapes
// surfaced by the 2026-05-22 dogfood-gaps plan: odds_for_team,
// event_markets, series_summary.

package playbooks

import "embed"

// The *.md pattern matches MANIFEST.md so the embed declaration is
// well-formed even when no playbook content has shipped. The
// *.json pattern picks up hand-authored playbook JSONs shipped in
// U10. The pair becomes a JSON+notes bundle for the install path
// in internal/cli/playbook_init.go to walk.
//
//go:embed *.json *.md
var FS embed.FS

// SeedVersion identifies the playbook content version. Embedded by
// the install path as a sentinel row; mismatch triggers re-seed.
// Format: <iso-date>-<cli-name>-<content-rev>.
var SeedVersion = "2026-05-26-prediction-goat-002"
