# MCP Market Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Install/inspect a server against a client | Smithery CLI `install`/`inspect` | `mcpmarket-pp-cli server get <slug>` | Shows the same install/config guidance MCP Market's page shows (JSON-LD featureList + description), works offline once synced, `--json`/`--select` for scripting |
| 2 | List available clients | Smithery CLI `list clients` | `mcpmarket-pp-cli client list` | Full 45k catalog vs Smithery's narrower client list, offline after sync, `--csv` export |
| 3 | List servers for a client | Smithery CLI `list servers --client` | `mcpmarket-pp-cli client get <slug>` | Related-server surfacing via `similar-tools`, not just Smithery-registered servers |
| 4 | Keyword search the catalog | mcpmarket.com search box | `mcpmarket-pp-cli search <query>` | Live `/search` mirror plus offline FTS5 fallback across the full synced catalog (works even when the live site is unreachable) |
| 5 | Browse by category | mcpmarket.com category pages | `mcpmarket-pp-cli category list` / `category get <slug>` | SQL-composable, `--json` |
| 6 | All-time top 100 servers/skills | mcpmarket.com leaderboards | `mcpmarket-pp-cli server leaderboard` / `skill leaderboard` | Diffable against prior syncs (see transcendence) |
| 7 | Today's trending servers/skills | mcpmarket.com /daily, /daily/skills | `mcpmarket-pp-cli server daily` / `skill daily` | Persisted locally so "today's" snapshot isn't lost after the day passes |
| 8 | Related/similar tools | mcpmarket.com "Related" sidebar + `/api/similar-tools` | `mcpmarket-pp-cli server similar <slug>` | Direct typed wrapper of the live JSON API |
| 9 | View a skill's SKILL.md content | mcpmarket.com skill page "SKILL.md" tab | `mcpmarket-pp-cli skill content <slug>` | Terminal-native read of frontmatter+body without opening a browser; `--json` for the parsed frontmatter fields |
| 10 | View a skill's FAQ | mcpmarket.com skill page "FAQ" tab | `(behavior in mcpmarket-pp-cli skill get <slug>)` | FAQ included in `skill get --json` output (from `FAQPage` JSON-LD) alongside the rest of the entity |
| 11 | Sync entire catalog locally | (nothing does this today) | `mcpmarket-pp-cli sync` | Full offline mirror in SQLite — no competitor offers this |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Trending delta / rising-vs-stale classifier | `trending --since 7d` | hand-code | Needs like-count velocity computed across ≥2 local snapshots — the live site only shows a static "daily" page, no rate-of-change | none |
| 2 | Sync-to-sync diff (entities added/removed/changed) | `diff --from <ts> --to <ts>` | hand-code | Website has no history endpoint; requires two persisted local snapshots and a row-level diff | none |
| 3 | Author/org portfolio view | `author <org>` | hand-code | No single endpoint unions entity types by author; requires a local cross-table join across server+skill+client catalogs | none |
| 4 | Time-travel leaderboard | `leaderboard --as-of <date>` | hand-code | The site only ever renders current top-100; answering "what was #1 on date X" requires a locally archived snapshot | none |
| 5 | Stack recommendation via similar-tools graph traversal | `stack <server> --depth 2` | hand-code | Chains the `similar-tools` endpoint multiple hops and dedupes/ranks locally; live equivalent would need dozens of sequential API calls | none |
| 6 | New-entrant watch per category | `watch category <name>` | hand-code | Requires comparing category membership across syncs to detect newly-appeared listings | none |
| 7 | Offline full-catalog search | `search <query> --offline` | hand-code | Sub-millisecond FTS5 query across 45k+ synced entities with zero network dependency — the live search has no such guarantee | none |
| 8 | Duplicate/overlap detector | `dedupe --category <name>` | hand-code | Corpus-wide text-similarity scan across thousands of descriptions; requires the full local dataset, not a single lookup | none |
