# art-goat Absorb Manifest

art-goat is a multi-source contemplative aggregator. MVP scope = AIC + NASA APOD. The "absorb" portion is what AIC's 103 endpoints and APOD's single endpoint give us; the "transcendence" portion is the contemplative spine plus cross-source unified commands — none of which exist in any other CLI today.

## Source tools (competing prior art)

| Tool | URL | Coverage | Stars | Why it's not enough |
|---|---|---|---|---|
| `art-institute-of-chicago/api` (AIC reference) | https://github.com/art-institute-of-chicago/api | Official AIC API; no CLI ships | n/a | Endpoint reference only, no client/CLI |
| `metmuseum/openaccess` | https://github.com/metmuseum/openaccess | CSV bulk export of Met objects | n/a | Static CSV; no live API, no Met CLI; Met not in MVP |
| `nasa/apod-api` | https://github.com/nasa/apod-api | The APOD upstream service | n/a | API reference; many client wrappers exist (none contemplative) |
| `randyrossi/nasa-apod-rust` | example community wrapper | Rust APOD CLI | low | One-shot fetch; no journal, no aggregation, no contemplative spine |

None of these wrap two-source-or-more contemplatively. No existing CLI has a `sit` command, a journal-aware `today` picker, or persistent reflection storage.

## Absorbed (match or beat everything that exists)

### From AIC's 103-endpoint OpenAPI surface

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | List/get artworks | AIC `/artworks`, `/artworks/{id}` | Generated typed client + Cobra mirror | Offline via local store, `--json`, `--select`, `--csv` |
| 2 | Search artworks | AIC `/artworks/search` | Generated client + FTS5 over local store | Works offline, regex, SQL composable |
| 3 | List/get artists | AIC `/artists`, `/artists/{id}` | Generated client | Same as above |
| 4 | List/get agents | AIC `/agents`, `/agents/{id}`, `/agents/search` | Generated client | Same |
| 5 | List/get articles | AIC `/articles`, `/articles/{id}`, `/articles/search` | Generated client | Same |
| 6 | List/get exhibitions | AIC `/exhibitions`, `/exhibitions/{id}`, `/exhibitions/search` | Generated client | Same |
| 7 | List/get places | AIC `/places`, `/places/{id}`, `/places/search` | Generated client | Same |
| 8 | List/get categories + category-terms | AIC `/categories`, `/category-terms` | Generated client | Same |
| 9 | List/get artwork-types, date-qualifiers, place-qualifiers, image-types | AIC reference endpoints | Generated client | Reference data normalized into store |
| 10 | List/get galleries, mobile sounds, tours | AIC `/galleries`, `/sounds`, `/tours` | Generated client | Same |
| 11 | List/get educational resources | AIC `/educational-resources` | Generated client | Same |
| 12 | List/get digital publications + articles | AIC `/digital-publications`, `/digital-publication-articles` | Generated client | Same |
| 13 | List/get products | AIC `/products` | Generated client | Same |
| 14 | Search across resources (federated AIC) | AIC `/search` | Generated client | Reused as substrate for unified `search` |

### From NASA APOD

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 15 | Fetch APOD by date | NASA APOD endpoint | Hand-authored `internal/source/apod/client.go` | DEMO_KEY default; lazy upgrade to user key |
| 16 | Fetch APOD by date range | NASA APOD `?start_date=&end_date=` | Hand-authored | Bulk sync into store |
| 17 | Random APOD | NASA APOD `?count=N` | Hand-authored | Reused as substrate for unified `random` |

### CLI quality table-stakes (every command)

| # | Feature | Our Implementation | Added Value |
|---|---|---|---|
| 18 | `--json` structured output | All commands | Agent-friendly |
| 19 | `--select <dotted.path>` field filtering | All read commands | Bounded context cost |
| 20 | `--csv` tabular output | All list commands | Spreadsheet-friendly |
| 21 | `--compact` high-gravity fields only | All commands | Default reasonable for terminal |
| 22 | `--limit` for list commands | Generated mirrors | Pagination control |
| 23 | `--dry-run` for any mutation | All applicable | Safe retries (verify-mode floor) |
| 24 | Typed exit codes (0/2/3/4/5/7/10) | All commands | Composable shell pipelines |
| 25 | `doctor` health check | Generated | Auth + store + sync diagnostics |
| 26 | `sync` populates local store from APIs | Generated + hand-authored multi-source | All read commands work offline after first sync |
| 27 | `auth wizard / set / status / test` | Hand-authored lazy-on-demand | No setup wall for anonymous use |
| 28 | `sql <SELECT>` over local store | Generated | Ad-hoc analytics |
| 29 | `context` for agent introspection | Generated | MCP context tool |

## Transcendence (only possible with our approach)

These are the commands that justify the print. None exist in any other CLI.

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| T1 | Contemplative timer with HTML emit and journal capture | `sit [id]` | Requires hand-authored Cobra command with browser emit, terminal timer, and journal SQLite roundtrip. No museum API gives you a "sit" endpoint. | 10/10 |
| T2 | Opinionated daily pick using anti-repeat + journal-aware diversity | `today` | Requires local join across `works` and `sits` to compute "haven't seen recently" and "different from last 3 sits" — only possible with both stores local. | 10/10 |
| T3 | Random piece + reflection prompt without timer | `presence` | Lighter-weight contemplative invocation; reads from store, writes a no-timer journal entry. Novel category. | 7/10 |
| T4 | Persistent reflection log with Markdown mirror | `journal write` | Captures user prose into SQLite; one-way regenerates `~/.art-goat/journal/<date>-<source>-<id>.md` for data-ownership posture. Configurable path via `ART_GOAT_JOURNAL_PATH`. | 8/10 |
| T5 | Reframed practice metrics (breadth/variety/region/mood drift) | `journal stats` | Headlines source breadth, medium variety, period coverage, mood-by-region — *not* streak-as-headline. Streak appears at bottom labeled "if you want to know." | 8/10 |
| T6 | FTS over your own reflection history | `journal search 'rain'` | Searches sits_fts virtual table; surfaces years-old reflections by token. No other tool stores reflections at all. | 6/10 |
| T7 | One-way SQLite → Markdown mirror export | `journal export` | Writes `~/.art-goat/journal/*.md` from SQLite. User-owned data persists if art-goat dies. | 7/10 |
| T8 | Cross-source similarity over unified schema | `similar <id>` | Match on `culture_region` OR overlapping `period` OR shared `creator_canonical`, ranked by FTS overlap. AIC alone can't show you a similar work from APOD. | 7/10 |
| T9 | Federated metadata browse across sources | `browse --culture=japan --medium=woodblock` | Filters span AIC + APOD via the unified `works` table. | 6/10 |
| T10 | Side-by-side compare from any sources | `compare <id1> <id2>` | Two works from different sources rendered as parallel metadata table. | 5/10 |
| T11 | Federated artist index | `artist <name>` | Aggregates works by `creator_canonical` across sources; tells you what AIC has *and* whether APOD has anything by the name. | 5/10 |
| T12 | Unified random across all sources | `random` | Single command picks one piece across the entire local corpus, weighted optionally by source. | 5/10 |
| T13 | Configured sources status | `sources` | Lists configured sources with sync state, count, last sync. Includes "auth required" hints lazily. | 5/10 |

**Total: 13 transcendence features, all scored ≥ 5/10. None ship as stubs.**

## What is intentionally deferred to v2 (post-MVP)

- The remaining 6 sources from the locked design (Met Open Access, Cleveland Museum, Te Papa Tongarewa, National Palace Museum Taiwan, Rijksmuseum, Smithsonian Open Access). Each is a mechanical addition: new `internal/source/<slug>/client.go` + registry wire-up. The schema and unified command surface require zero changes.
- `path --theme=X` multi-piece curated walk. Not in MVP; would re-open the "is this hand-authored static curation?" question.
- `--inline` opt-in for terminal-graphics displays (iTerm/Kitty). MVP ships HTML emit + description-mode fallback only; `--inline` ships in v1.1 after the HTML emit path is proven.
- Streak opt-in display (`art-goat journal opt-in --show-streak`). Streak is computed and queryable in MVP via `journal stats` (bottom of output, "if you want to know"), but the per-sit greeting line opt-in flag is v1.1.
- Anti-repeat full implementation with persistent state. MVP: anti-repeat last 30 days. v1.1: configurable window per user preference.

These are explicit, non-blocking, and tagged with their next-step path. The MVP ships a complete contemplative tool, not a stubbed-out preview.

## Implementation map (Phase 3 build plan)

Files to hand-author in Phase 3 (everything not in this list is generator-emitted):

```
internal/source/registry.go         — Source interface + dispatcher
internal/source/apod/client.go      — NASA APOD client (Sync method)
internal/source/aic/client.go       — AIC source-shaped adapter wrapping generated client
internal/store/works_schema.go      — Unified `works` table migration
internal/store/sits_schema.go       — Journal `sits` table migration
internal/cli/sit.go                 — `sit` command + HTML emit + journal capture
internal/cli/today.go               — `today` command + algorithmic pick
internal/cli/presence.go            — `presence` command
internal/cli/journal.go             — `journal {write,stats,search,export}`
internal/cli/prompts.go             — ~25 prompt library, `// pp:novel-static-reference`
internal/cli/browse.go              — federated browse over `works`
internal/cli/random.go              — unified random
internal/cli/compare.go             — side-by-side compare
internal/cli/artist.go              — federated artist index
internal/cli/similar.go             — cross-source similarity
internal/cli/sources.go             — `sources` status command
internal/cli/journal_mirror.go      — SQLite → Markdown mirror emit
internal/cli/html_emit.go           — Browser HTML + image embed
internal/cli/sit_phases.go          — `--phased` mode helpers
```

All hidden interactive commands (`sit`, `today` interactive form, `presence`, `journal write`, `journal export`) check `cliutil.IsVerifyEnv()` and short-circuit; carry `mcp:hidden=true`. Read-only commands carry `mcp:read-only=true`.
