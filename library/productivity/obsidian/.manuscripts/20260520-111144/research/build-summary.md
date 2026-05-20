---
title: obsidian-pp-cli build summary
date: 2026-05-20
status: built
---

# obsidian-pp-cli — build summary

Read-only V1 wrapping the official Obsidian CLI (v1.12+) as a virtual REST surface, with a local SQLite mirror for offline compound analytics.

## Architecture

1. **Synthetic OpenAPI 3.0 spec** at `obsidian-openapi.yaml` (13 read endpoints; `/orphans` is owned by the hand-authored Tier-3 command and intentionally absent from the spec).
2. **Printing Press factory** consumes the spec, scaffolds Cobra + MCP + agent-native flags.
3. **Subprocess client** replaces the generated HTTP client: every `Get*` call dispatches to `exec.Command("obsidian", ...)` with stdout shaped into JSON.
4. **Obsidian-specific SQLite schema** (notes / obsidian_tags / obsidian_links / frontmatter_kv / vault_sync_state) populated by a custom `sync` command that walks the vault filesystem and parses frontmatter, wikilinks, embeds, and tags.
5. **Tier-3 compound commands** (health, broken, vault-sql, orphans-enhanced, stale-enhanced, load) query the mirror so they answer instantly even when Obsidian is closed.

## V1 scope (locked)

- 13 live read commands via subprocess (12 Tier-1 endpoints; `/search` split into `live-search` and `live-search/context`)
- 6 Tier-3 analytics commands (health, broken, vault-sql, orphans, stale, load) backed by the SQLite mirror
- ZERO write operations — deferred to V2 pending upstream `markdown-patch` frontmatter-corruption fix
- macOS only, single vault, no multi-vault

## V2 deferred

- 6 write commands (create / delete / append / prepend / move / property:set)
- decay / hotspots / reconcile Tier-3 analytics (designed, not implemented in V1)
- multi-vault, Windows, Linux

## Source provenance

- Spec author: hand-authored synthetic OpenAPI 3.0 spec
- Command surface: derived from `obsidian help` output (v1.12.7, captured 2026-05-19)
- Upstream binary: official Obsidian CLI shipped in Obsidian desktop v1.12+

## Polish pass — 2026-05-20

Regenerated against the same spec with three Greptile review fixes folded in:

- `sync.go syncOneNote` — wrap DELETE + re-INSERT of child rows in a transaction so a mid-flight error rolls back instead of leaving the note record with empty child tables while `vault_sync_state` advances (P1 — silent permanent breakage).
- `health.go` stale-count — compute the cutoff in Go as RFC3339 instead of SQLite's `datetime('now', '-N days')`; the space-vs-T separator made notes modified on the cutoff date compare as newer-than-cutoff (P2).
- `pm_orphans.go` — drop the redundant in-memory `sort.SliceStable` pass; SQL `ORDER BY` already sorts honoring `--oldest` (P2).

Spec cleanup: dropped `/orphans` (the Tier-3 hand-authored `orphans` command supersedes the raw pass-through) and renamed `/search` → `/live-search` so the Obsidian-process search command is distinct from the framework `search` (local mirror, fast/offline).

License + attribution: standardized hand-authored Go file headers on Apache-2.0 (matching LICENSE + generator templates); set `owner_name="LARGE FORMAT"` and `printer_name="Angelo Pullen"` in the manifest.

Phase-5 acceptance: replaced the hand-written proof with one produced by `printing-press dogfood --live --level quick` (5 tests passed, 3 skipped — same matrix the autonomous runner would have exercised).
