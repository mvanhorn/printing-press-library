---
title: obsidian-pp-cli build summary
date: 2026-05-20
status: built
---

# obsidian-pp-cli — build summary

Read-only V1 wrapping the official Obsidian CLI (v1.12+) as a virtual REST surface, with a local SQLite mirror for offline compound analytics.

## Architecture

1. **Synthetic OpenAPI 3.0 spec** at `obsidian-openapi.yaml` (14 read endpoints).
2. **Printing Press factory** consumes the spec, scaffolds Cobra + MCP + agent-native flags.
3. **Subprocess client** replaces the generated HTTP client: every `Get*` call dispatches to `exec.Command("obsidian", ...)` with stdout shaped into JSON.
4. **Obsidian-specific SQLite schema** (notes / obsidian_tags / obsidian_links / frontmatter_kv / vault_sync_state) populated by a custom `sync` command that walks the vault filesystem and parses frontmatter, wikilinks, embeds, and tags.
5. **Tier-3 compound commands** (health, broken, vault-sql, orphans-enhanced, stale-enhanced, load) query the mirror so they answer instantly even when Obsidian is closed.

## V1 scope (locked)

- 14 live read commands via subprocess
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
