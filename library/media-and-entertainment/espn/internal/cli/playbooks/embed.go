// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

// Package playbooks ships the ESPN-specific playbook + notes content
// as an embedded filesystem. The auto-install path in
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

package playbooks

import "embed"

//go:embed *.json *.md
var FS embed.FS

// SeedVersion identifies the playbook content version. Embedded by
// the install path as a sentinel row; mismatch triggers re-seed.
// Format: <iso-date>-<cli-name>-<content-rev>.
var SeedVersion = "2026-05-25-espn-005"
