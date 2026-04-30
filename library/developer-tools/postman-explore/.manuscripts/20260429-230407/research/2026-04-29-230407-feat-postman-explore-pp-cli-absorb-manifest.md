# Postman Explore — Absorb Manifest

## Ecosystem Scan

A thorough sweep across npm, PyPI, and GitHub for tools that touch the public Postman API Network found:

| Tool | Surface | Relevance |
|------|---------|-----------|
| postmanlabs/newman | Run a collection from a local file | Different surface — execution, not discovery |
| postmanlabs/postman-cli | Run private workspaces, auth-required | Different scope — authenticated Postman product |
| postmanlabs/postman-mcp-server | Manage authenticated workspaces, collections, environments via MCP | Different scope — auth'd product API |
| PostmanV3/postman-mcp-server | Same — auth'd Postman API CRUD via MCP | Different scope |
| delano/postman-mcp-server | Same — auth'd Postman API + private network management | Different scope |
| public-apis/public-apis | Curated markdown list of free APIs | Different surface — not Postman, not searchable as data |
| APIs-guru/openapi-directory | Wikipedia for Web APIs (OpenAPI defs) | Different surface — OpenAPI specs, not Postman |

**Conclusion:** No CLI today targets the public discovery surface at postman.com/explore. Every Postman CLI tool ships against the authenticated Postman product API; none of them surface community collection lookup as a first-class capability. The absorb table below contains only spec endpoints — there is nothing to outdo. The value of this Printed CLI is concentrated in the transcendence table.

## Absorbed (match what the spec offers, beat with agent-native ergonomics)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List collections, workspaces, APIs, flows | postman.com/explore web UI | `browse <type>` with `--limit --offset --category --sort popular --min-monthly-forks` | `--json --select`, paginated, agent-native, offline-resyncable from local store |
| 2 | Search the public network | postman.com search bar | `search <query>` with `--type` for narrowing | Object-keyed JSON, `--json --select` filter, results split by entity type |
| 3 | Get full detail for an entity | Click-through on web UI | `get <numericId>` | Single-record JSON with all fields, no proxy envelope plumbing exposed |
| 4 | List categories | Web UI side strip | `categories list` | Spotlighted order preserved; JSON for joins |
| 5 | Get category detail | `/category/<slug>` page | `categories get <slug>` | Includes hero image, icon, full description |
| 6 | Get team workspaces by handle | Team page on web | `teams workspaces <handle>` | Full workspace array, sortable client-side |
| 7 | Get team profile | Team header | `teams get <id>` | Small profile for joins |
| 8 | Network-wide entity counts | Footer stats hint on web | `stats` | Single command, JSON, machine-readable |

Every absorbed row gets `--json`, `--select`, `--dry-run`, structured exit codes, and an FTS-indexed local mirror so commands work offline after `sync`.

## Transcendence (only possible with local sync + agent-native plumbing)

User personas drove the transcendence selection — these features come from rituals and frustrations the website's UI cannot serve.

### Personas

1. **API integrator** ("I need a Postman Collection for the third-party API I'm wiring up") — wants quick canonical lookup; frustrated that searching `stripe` returns dozens of forks of varying quality.
2. **API curator / DevRel** ("I'm comparing payment processors") — wants comparative discovery across publishers and categories; frustrated that comparison requires clicking through each team page.
3. **AI agent** ("User asked about Twilio's SMS API; I need to point them at a canonical collection") — wants programmatic, offline-searchable, structured discovery; today the web UI is unscriptable.
4. **API watcher** ("I track Twilio's collection updates") — wants change detection over time; the website has no "what changed" view.

### Transcendence Table

| # | Feature | Command | Source | Score | How It Works | Evidence |
|---|---------|---------|--------|-------|--------------|----------|
| 1 | Canonical collection lookup | `canonical <vendor>` | new | 9/10 | Searches local FTS + ranks by (verified publisher) × (max forkCount) × (recency). Returns one best candidate plus runner-ups. Powered by `searchAll` + locally-cached publisher verification flag. | Persona 1 + 3 primary need; absent on web (search returns ungrouped, unranked) |
| 2 | Trend ranking by custom metric | `top --metric weekForks --type collection --category payments` | prior (reframed from `trending`) | 8/10 | Local SQLite query against synced metrics array. Aggregates the rich `weekForkCount`/`monthViewCount` set per entity. | Persona 2 + 4; site only sorts by `popular` (which isn't time-windowed); prior planned `trending --days 7` |
| 3 | Publisher gravitas (cross-team comparison) | `publishers top --category devtools` | prior (reframed from `teams rank`) | 8/10 | Aggregates fork totals across every entity per publisher; ranks within a category. SQL JOIN across entities + categories. | Persona 2; web shows team pages individually but no comparison view; prior planned `teams rank --by forks` |
| 4 | Drift / changelog since last sync | `drift --since 30d --type collection` | prior (reframed from `watch diff`) | 8/10 | Compares two synced snapshots; reports new entities, removed entities, and entities whose `updatedAt` advanced. | Persona 4; no equivalent on web; prior planned `watch add <id>` / `watch diff` |
| 5 | Similar collection finder (more-like-this) | `similar <id>` | prior (kept) | 7/10 | FTS5 more-like-this over name+summary+description+tags, restricted to same entityType. | Persona 1; web has no comparable view; prior planned `similar <entity-id>` |
| 6 | Fork velocity (accelerating collections) | `velocity --type collection --top 10` | prior (kept) | 7/10 | Local computation of `weekForkCount × 4 / monthForkCount` ratio; ranks by acceleration. | Persona 2 + 4; no equivalent on web; prior planned `velocity <entity-id>` |
| 7 | Verified publishers only | `browse <type> --verified-only` | new | 6/10 | Local filter on `publisherInfo.team[*].isVerified`. | Persona 1; signal exists in API response but not as a web-UI filter |
| 8 | Category landscape report | `category landscape <slug>` | prior (reframed from `landscape`) | 5/10 | Combines per-type entity counts, top 5 publishers by aggregate fork count, and top 5 entities by viewCount for a category. | Persona 2; web shows category page but no aggregate stats; prior planned `landscape --category payments` |

Total novel features ≥ 5/10: **8**.

### Dropped prior features (with one-line justifications)

- **`stale --days 180`** — Below 5/10 (Domain 1 + Pain 1 + Build 2 + Research 0 = 4/10). Niche; `drift` covers detecting changes more generally.
- **`search "graphql" --all-categories`** — Foundation; the base `search` command already searches across all entity types and categories, so this isn't a separate feature.
- **`sync` / `search --offline`** — Foundation, not transcendence; lives in Priority 0.

### Buildability proofs

1. **canonical** — `searchAll` returns publisher metadata; `publisherInfo` from browse responses provides verification flag; rank in-process. No external service.
2. **top** — Local SQL: `SELECT … FROM entities JOIN metrics ON entities.id = metrics.entity_id WHERE metric_name = ? AND entity_type = ? ORDER BY metric_value DESC`. Pure local data.
3. **publishers top** — Aggregate query across cached entities and category memberships. Pure local data.
4. **drift** — Two SQLite snapshots compared via JOIN on `id`. Pure local data.
5. **browse --verified-only** — Filter on cached publisher verification flag. Pure local data.
6. **velocity** — Ratio computation in SQL. Pure local data.

All six pass the kill/keep checks: no LLM dependency, no external service, no auth, no scope creep, all verifiable in dogfood by checking output against known fork counts, no reimplementation (every feature reads from the local store populated by `sync`).

## Reprint Reconciliation

A prior CLI exists at `~/printing-press/library/postman-explore/` from `printing-press v0.4.0`. No archived `research.json` from that run was kept (older binary). The prior CLI shipped these transcendence-shaped commands:

| Prior command | Today's verdict | Reason |
|---------------|-----------------|--------|
| `analytics` (generic local SQL) | **Reframed → `sql`** | Generic SQL access is now a Phase 0 foundation feature provided by the v3 generator; no longer needs its own command surface |
| `workflow archive` | **Dropped (subsumed by `sync --all-types`)** | Sync covers this in v3 |
| `workflow status` | **Dropped (subsumed by `sync status`)** | Same as above |

No prior-feature drops below the 5/10 threshold are owed re-justification — the prior commands were generic scaffolding, not domain features.

## Stubs

None. Every feature in the absorbed and transcendence tables is fully buildable from the spec + local store.

## Phase Gate 1.5 Showcase

I cataloged **8 absorbed features** + **8 novel features** = **16 total**. Every absorbed feature is reachable through the proxy with HTTP 200 verified. The novel features are:

1. **`canonical <vendor>`** (9/10) — Best community Postman Collection for a vendor, ranked by verification × fork count × recency. *new*
2. **`top --metric weekForks --type ... --category ...`** (8/10) — Trend ranking across any of 10 metric dimensions. *prior, reframed from `trending`*
3. **`publishers top --category devtools`** (8/10) — Cross-publisher gravitas ranking. *prior, reframed from `teams rank`*
4. **`drift --since 30d --type collection`** (8/10) — What changed on the network. *prior, reframed from `watch diff`*
5. **`similar <id>`** (7/10) — More-like-this over the synced FTS index. *prior, kept*
6. **`velocity --type collection --top 10`** (7/10) — Accelerating collections (week-rate × 4 / month-rate). *prior, kept*
7. **`browse <type> --verified-only`** (6/10) — Filter to verified publishers. *new*
8. **`category landscape <slug>`** (5/10) — Per-category aggregate of counts, top publishers, top entities. *prior, reframed from `landscape`*

Six of the eight novel features absorb prior planned ideas (the v0.4.0 CLI shipped none of them; my v3 shipping scope reverses that). Two are new this run (`canonical` and `--verified-only`). Two prior features were dropped with justifications above (`stale`, cross-category search). Total feature count: 16. The web UI exposes maybe 4 of these directly (browse, search, category navigation, team pages). Every novel feature is local-store-powered and works offline.
