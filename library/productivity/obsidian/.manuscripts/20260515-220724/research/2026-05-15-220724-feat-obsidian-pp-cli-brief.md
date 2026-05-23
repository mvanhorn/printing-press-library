# Obsidian CLI Brief

## API Identity
- **Domain:** Personal knowledge management. Obsidian is a local-first markdown editor over a folder of `.md` files ("vault"). There is no canonical HTTP API — the filesystem **is** the API. A community plugin (coddingtonbear/obsidian-local-rest-api) exposes one when the app is running, but the vast majority of automation tools operate directly on `.md` files.
- **Users:** Damien specifically — operating the UCE three-layer protocol vault (`/Users/dstevens/Documents/Dev/UCE`) as a Knowledge-Graph + Events + Patterns memory system. The CLI is a **local enforcer** of the protocol that the **cm (Compounding Memory) extraction pipeline** can pick up downstream via Tuck filesystem sync. CM treats Obsidian vaults as a source (`SourceType::ObsidianImport`), so the upstream contract is: write protocol-compliant frontmatter, cm extracts the rest.
- **Data profile:** Folders of `.md` files with YAML frontmatter, optional `_facts/*.toml` sidecars, wiki-style `[[links]]`, `#tags`. Typical vault: 100s–10,000s of notes, 10s of MB of text.

## Reachability Risk
- **None.** Local filesystem. No network, no auth, no rate limits, no bot protection. The Local REST API plugin (if used) requires the app running + an API key, but that's optional — we default to filesystem.

## Top Workflows
1. **Write a new protocol-compliant note** — `obsidian note new --type meeting --title "..."` emits frontmatter that passes UCE's `frontmatter_parser.py` validator on first save.
2. **Find context fast** — `obsidian search "buttermilk"` returns matching notes in <100ms via local FTS5 index (vs Obsidian's in-app search which is slower and not scriptable).
3. **Audit the vault for protocol violations** — `obsidian lint` finds notes missing required fields, bad enum values, dates not in ISO format, descriptions >150 chars.
4. **Add a fact to an entity** — `obsidian fact add "[[Jeff Smith]]" --category preference --fact "Prefers async over meetings"` writes to inline `facts:` (<20) or graduates to `_facts/Jeff Smith.toml` (≥20).
5. **Token-efficient progressive disclosure** — `obsidian context list People/ --layer description` returns `path: description` pairs without note bodies, so agents can decide what to read without burning context.
6. **Link-graph queries** — `obsidian links to "[[Servosity]]"` finds backlinks; `obsidian orphans` finds notes with no incoming links.

## Data Layer
- **Primary entities:** `notes` (one row per `.md` file: path, type, date, description, status, body_text_hash, mtime), `frontmatter_fields` (sparse key-value for type-specific fields), `facts` (id, parent_note_path, fact, category, timestamp, status, source, decision_trace_id), `tags`, `links` (from_note, to_note, link_type), `validation_findings` (path, rule, severity, message).
- **Sync cursor:** mtime on `.md` files. Incremental sync = diff vs `notes.mtime`; full sync = walk vault.
- **FTS/search:** SQLite FTS5 over (title, body, frontmatter_text). Tag-faceted via inverted index. Link-graph queries via recursive CTE on `links` table.

## Codebase Intelligence
- UCE has an existing Python validator at `UCE/src/vault/frontmatter_parser.py:45-191` that defines required fields, type-specific field requirements, and ISO date validation. The Obsidian CLI's `lint` command MUST match its rule set so the two tools never disagree.
- cm (Rust) has `SourceType::ObsidianImport` (cm-core `provenance/source.rs:57`) — confidence 0.95. cm does NOT have an ingest endpoint; it reads from Tuck via EventBridge. So the Obsidian CLI's job ends at "write a valid `.md` file to disk"; tuck-fs and cm-extraction-lambda take it from there.

## User Vision
- Wraps the existing Obsidian ecosystem (CLIs, REST plugin, MCP servers) and beats them by being the **only one that enforces the UCE three-layer protocol**.
- Instant search via FTS5 (sub-100ms on 10k notes).
- Token-efficient: every command supports `--select`, `--json`, `--compact`, and a `--layer description` shorthand for progressive disclosure.
- Designed to be the local frontend that cm picks up downstream (no conflicts with cm's entity resolution).

## Source Priority
- Single source. Skip — no inversion risk.

## Product Thesis
- **Name:** `obsidian-pp-cli` (binary), Obsidian CLI (display).
- **Why it should exist:** Every existing Obsidian CLI treats frontmatter as opaque YAML — they let you set arbitrary fields. None enforce a protocol. Damien runs UCE's three-layer protocol; without enforcement, the vault drifts (missing `description`, bad date format, type enum violations) and the cm extraction pipeline silently degrades. This CLI is the **enforcement layer**: every `note new` / `fact add` / `update` writes spec-compliant frontmatter, and `lint` finds violations introduced by hand-edits or other tools. The instant FTS5 search and token-efficient agent output are necessary because Damien uses this from agents (Claude, Codex) constantly and the alternatives (open Obsidian, search, copy, paste) are slow and context-heavy.

## Build Priorities
1. **Vault store + sync** (Priority 0): SQLite schema for notes/frontmatter_fields/facts/tags/links; `sync` walks vault.
2. **CRUD on notes with protocol enforcement** (Priority 1): `note new`, `note get`, `note set`, `note rm`, `note mv`. Every write validates against the UCE protocol before touching disk.
3. **Frontmatter operations** (Priority 1): `frontmatter get/set/del`, `lint`, `migrate` (fix common violations).
4. **Fact operations** (Priority 1): `fact add/list/supersede`, inline-to-TOML graduation at 20 facts.
5. **Search + link graph** (Priority 1): `search`, `links to/from`, `orphans`, `tags list`.
6. **Optional REST passthrough** (Priority 1 deferred): `--rest` flag on `open`-style commands to invoke the Local REST API plugin when the app is running.
7. **Transcendence** (Priority 2): protocol-aware lint with severity tiers, three-layer dashboard, dead-link finder, description-only listing, context bundles for agents, decision-trace tracker. (Subagent will refine.)

