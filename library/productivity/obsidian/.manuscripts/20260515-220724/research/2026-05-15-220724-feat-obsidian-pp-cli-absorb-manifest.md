# Obsidian CLI Absorb Manifest

## Sources Surveyed
| # | Tool | URL | Stars (approx) | Lang | Role |
|---|------|-----|----------------|------|------|
| 1 | Yakitrak/obsidian-cli | github.com/Yakitrak/obsidian-cli | ~600 | Go | Most-popular CLI (open, search, create, frontmatter edit) |
| 2 | jwhonce/obsidian-cli | github.com/jwhonce/obsidian-cli | ~150 | Python | CLI + MCP server, frontmatter, MCP integration |
| 3 | mpfilbin/obsidian-vault-manager | github.com/mpfilbin/obsidian-vault-manager | ~50 | Ruby | Tags, images, properties, indexing |
| 4 | rjzxui/obsidian-vault-cli | github.com/rjzxui/obsidian-vault-cli | ~30 | TS | 100+ commands, tasks, links |
| 5 | mattjoyce/obsave | github.com/mattjoyce/obsave | ~80 | Python | Pipe content in, frontmatter auto-add |
| 6 | davidpp/obsidian-cli | github.com/davidpp/obsidian-cli | ~100 | TS | AI-optimized, Omnisearch, REST |
| 7 | coddingtonbear/obsidian-local-rest-api | github.com/coddingtonbear/obsidian-local-rest-api | ~1.5k | TS | Official REST plugin — 14 endpoint groups |
| 8 | cyanheads/obsidian-mcp-server | github.com/cyanheads/obsidian-mcp-server | ~500 | TS | MCP via REST plugin — read/write/search/surgical-patch |
| 9 | StevenStavrakis/obsidian-mcp | github.com/StevenStavrakis/obsidian-mcp | ~400 | TS | Direct-vault MCP |
| 10 | aaronsb/obsidian-mcp-plugin | github.com/aaronsb/obsidian-mcp-plugin | ~250 | TS | Graph-aware MCP, semantic ops |
| 11 | bitbonsai/mcpvault | github.com/bitbonsai/mcpvault | ~200 | Go | 14 token-optimized MCP methods |
| 12 | dp-veritas/mcp-obsidian-tools | github.com/dp-veritas/mcp-obsidian-tools | ~80 | TS | Read-only MCP (metadata-rich) |
| 13 | MarkusPfundstein/mcp-obsidian | github.com/MarkusPfundstein/mcp-obsidian | ~300 | TS | Original MCP-Obsidian |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List notes (whole vault) | Yakitrak `ls` | `obsidian note list` reads from SQLite store | Offline, paginated, filterable by `--type`, `--tag`, `--status`, `--folder`, returns `--json` |
| 2 | List notes in folder | Yakitrak `ls <folder>` | `obsidian note list --folder People/` | Same plus FTS ranking when combined with `--match` |
| 3 | Get note (raw) | Yakitrak `cat`, REST `GET /vault/{file}` | `obsidian note get <path>` | `--frontmatter-only`, `--body-only`, `--select <fields>` for token-efficient agent reads |
| 4 | Create note | Yakitrak `create`, obsave | `obsidian note new --type meeting --title "..."` | **Three-layer protocol validation** on write — refuses to create notes that won't pass UCE validator |
| 5 | Edit note body | Yakitrak `edit` | `obsidian note set <path> --body @file` or `--body-stdin` | Atomic write via temp file; preserves frontmatter unless explicitly changed |
| 6 | Delete note | Yakitrak `rm` | `obsidian note rm <path>` | `--dry-run`, refuses to delete notes with incoming links unless `--force` |
| 7 | Move/rename note | Yakitrak `mv` | `obsidian note mv <from> <to>` | **Auto-rewrites all `[[wikilinks]]` to the renamed note across the vault** |
| 8 | Open note in app | Yakitrak `open`, REST `POST /open/` | `obsidian note open <path>` | Default prints `obsidian://` URI; `--launch` opens in app (verify-env-safe) |
| 9 | Print frontmatter | Yakitrak `frontmatter --print` | `obsidian frontmatter get <path>` | `--key <field>` for single-field read; `--json` |
| 10 | Edit frontmatter field | Yakitrak `frontmatter --edit --key x --value y` | `obsidian frontmatter set <path> --key x --value y` | **Validates new value against UCE enum/type rules** before write |
| 11 | Delete frontmatter field | Yakitrak `frontmatter --delete --key x` | `obsidian frontmatter del <path> --key x` | Refuses to delete required fields (`type`, `date`, `description`, `status`) unless `--force` |
| 12 | List tags | Vault-Manager, REST `GET /tags` | `obsidian tag list` | Counts, sorted; `--match <prefix>` filter |
| 13 | Find notes by tag | Vault-CLI | `obsidian tag get <tag>` | Returns matching note paths with descriptions (token-efficient) |
| 14 | Add tag to note | Vault-CLI | `obsidian tag add <path> <tag>` | Validates that tag goes in frontmatter (`tags: []`) per UCE schema, not inline `#tag` |
| 15 | Remove tag | Vault-CLI | `obsidian tag rm <path> <tag>` | |
| 16 | Search full-text | Yakitrak `search`, REST `POST /search/simple` | `obsidian search "<query>"` | **FTS5 ranking**, <100ms on 10k notes, supports `AND/OR/NOT`, regex via `--regex`, `--select description` for token efficiency |
| 17 | Advanced query (dataview-like) | REST `POST /search` (JsonLogic), davidpp Omnisearch | `obsidian sql "SELECT path, description FROM notes WHERE type='meeting' AND date > '2026-01-01'"` | Read-only SQLite — composable, scriptable; richer than JsonLogic |
| 18 | List backlinks | Vault-CLI, MCP-vault | `obsidian links to <path>` | Returns `path: link_text` pairs |
| 19 | List outgoing links | Vault-CLI | `obsidian links from <path>` | Distinguishes wiki, markdown, embed |
| 20 | List orphan notes | mpfilbin Vault-Manager | `obsidian orphans` | Notes with zero incoming links |
| 21 | List broken links | mcp-obsidian-tools | `obsidian links broken` | `[[targets]]` that resolve to no note |
| 22 | Read daily / periodic note | REST `/periodic/{period}/` | `obsidian periodic today` (also `yesterday`, `--date 2026-05-15`, `--period weekly`) | Creates from template if missing |
| 23 | List commands (REST plugin) | REST `/commands/`, Yakitrak | `obsidian rest commands` (requires `--rest`) | Only available with REST plugin running; clear error if not |
| 24 | Execute Obsidian command | REST `POST /commands/{id}` | `obsidian rest exec <id> --rest` | Same |
| 25 | Append to active file | REST `PATCH /active/` | `obsidian active append "<text>" --rest` | |
| 26 | Read active file | REST `GET /active/` | `obsidian active get --rest` | |
| 27 | Sync vault to local index | (none — we're the only one with a store) | `obsidian sync` (full), `obsidian sync --incremental` | Foundation for instant search and SQL queries; no competitor has this |
| 28 | Pipe content into note | obsave | `cat foo.txt \| obsidian note new --title "..." --type idea --body-stdin` | Auto-fills required frontmatter with sane defaults; refuses on validation failure |
| 29 | Detect duplicate notes | (none) | `obsidian dupes` | Notes with identical body hash; `--by title` finds same-title across folders |
| 30 | List facts on a note | (none) | `obsidian fact list <path>` | Inline + TOML facts merged |
| 31 | Add a fact | (none — most CLIs treat frontmatter as opaque) | `obsidian fact add <path> --fact "..." --category preference` | **Auto-graduates inline `facts:` to `_facts/<name>.toml` at threshold (20 facts) per UCE protocol** |
| 32 | Supersede a fact | (none) | `obsidian fact supersede <path> --id jeff-001` | Marks `status: superseded`, preserves history |
| 33 | Get note (token-efficient) | bitbonsai mcpvault (claims "token-optimized") | `obsidian note get <path> --layer description` returns 1 line | We win because we have `notes.description` indexed in SQLite, not parsed at read time |
| 34 | Doctor / health check | (none — competitors skip) | `obsidian doctor` | Checks vault path readable, store fresh, count of validation errors |

## Transcendence (only possible with our approach)

These are the verified survivors from the novel-features subagent (full audit trail in `2026-05-15-220724-novel-features-brainstorm.md`).

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Protocol lint (with severity tiers) | `obsidian lint [--severity error\|warn\|info] [--rule <id>] [--exit-nonzero-on error]` | 10/10 | Walks every `.md` row in local SQLite, applies the rule set ported from UCE's `frontmatter_parser.py`. No other Obsidian CLI knows about the three-layer protocol. **Headline feature.** |
| 2 | Auto-migrate common violations | `obsidian migrate [--rule date-iso\|type-enum\|missing-description] [--dry-run]` | 9/10 | Reads `validation_findings` from the store and applies mechanical fixers (ISO date coercion, type enum normalization, fill missing fields from mtime) via the same enforcement path as `note new`. |
| 3 | Three-layer dashboard | `obsidian layers stats` | 8/10 | Aggregates `notes.type` against the layer map (KG/Events/Patterns) from local SQLite; reports per-layer counts, average age, recent-write velocity. |
| 4 | Fact graduation candidates | `obsidian facts graduation-candidates [--threshold 20]` | 8/10 | SQL on local facts table joining count vs threshold; surfaces entities approaching the 20-fact inline→TOML graduation rule before they trip it. |
| 5 | Decision-trace tracker | `obsidian facts decision-trace <trace-id>` | 8/10 | SQL across inline + TOML facts for one `decision_trace_id`; reconstructs decision history with sources, ordered by timestamp. |
| 6 | Entity dossier (agent-shaped) | `obsidian entity dossier "[[Entity]]" [--layer description]` | 9/10 | Joins notes + frontmatter + facts (inline + TOML) + backlinks + tags for one entity into one structured block. `--layer description` returns the token-efficient shape. |
| 7 | cm extraction-readiness audit | `obsidian readiness [--source-tag <tag>] [--since <date>]` | 8/10 | Filters `validation_findings` to the rule subset that cm's `ObsidianImport` extractor depends on. Fix upstream before Tuck syncs. |
| 8 | Stale notes by type | `obsidian stale [--type meeting] [--older-than 90d]` | 7/10 | SQL on `notes.mtime + type` — find journals/meetings never promoted, active entities going cold. |
| 9 | Provenance audit | `obsidian provenance <note-path>` | 7/10 | Walks `source` field on note + every fact (inline + TOML); prints the chain (transcript → fact → trace). |
| 10 | Daily note append (protocol-enforced) | `obsidian daily append "<text>" [--section "## Notes"]` | 7/10 | Resolves today's daily-note path from periodic-note settings; creates from template (with protocol-compliant frontmatter) if missing; appends under named section. |

## Stubs / Deferred
None. All 34 absorbed + 10 transcendence features are shipping scope. No paid APIs, no headless Chrome — pure filesystem.

## Total Feature Count
- Absorbed: 34
- Transcendence: 10
- **Total: 44**

Best competitor (Yakitrak/obsidian-cli) ships ~12 commands. mcpvault claims 14 token-optimized methods. This CLI absorbs all of them, plus matches the Local REST API's full surface (via `--rest`), plus 10 protocol-aware features no Obsidian CLI has.
