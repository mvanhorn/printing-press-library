# Mobbin CLI Absorb Manifest (revised — full discover/collections/images)

## Ecosystem Scan

14 tools found (8 with substantive surfaces). Most material:
1. **Official Mobbin MCP** (api.mobbin.com/mcp) — OAuth, paid-only, 1 tool
2. **pdcolandrea/mobbin-mcp** (TS, 31★) — 7 read tools, HTML flight-chunk extractor
3. **ismailsaleekh/mobbin-agent** — Playwright, 10 tools, batch downloads
4. **solejay/mobbin-cli** — LobeHub skills, multi-profile auth
5. **underthestars-zhy/MobbinAPI** (Swift, 2023) — full collection CRUD via Supabase PostgREST
6. **Batch Collector for Mobbin** (Chrome ext) — flow-page auto-scroll + zip
7. **YonasValentin/design-inspiration-mcp-server** — cross-site Mobbin search via Serper
8. **iamsahebgiri/mobbin-unlocked** — Pro-bypass userscript (ToS-violating, not absorbed)

## Absorbed (match every existing tool)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | List apps by platform | pdcolandrea / Mobbin UI | `mobbin apps list <platform>` | Local SQLite mirror for offline autocomplete |
| 2 | Popular apps + previews | pdcolandrea / Mobbin UI | `mobbin apps popular --platform web` | --json, --select, --agent |
| 3 | Discover page (paginated) | Mobbin UI | `mobbin apps discover --tab latest --platform web --page-index N` | Scriptable pagination |
| 4 | Search apps (auth) | pdcolandrea / official MCP | `mobbin apps search --platform web --app-categories ...` | --dry-run, typed exit codes |
| 5 | Search screens (auth) | pdcolandrea / official MCP `search_screens` | `mobbin screens search --platform web --screen-patterns paywall` | Combinable filters: patterns + elements + OCR keywords + animation |
| 6 | Search flows (auth) | pdcolandrea | `mobbin flows search --platform web --flow-actions creating-account` | Same shape as screens |
| 7 | Full filter taxonomy | pdcolandrea | `mobbin filters list` | Synced to SQLite; queryable offline |
| 8 | List app categories | (new — projected from filters) | `mobbin categories list --platform web` | Browsable first-class category surface |
| 9 | List screen patterns | (new — projected from filters) | `mobbin patterns list --platform web` | Pattern-axis discovery before searching |
| 10 | List UI elements | (new — projected from filters) | `mobbin elements list --platform web` | Element-axis discovery |
| 11 | List flow actions | (new — projected from filters) | `mobbin flow-actions list --platform web` | Flow-axis discovery |
| 12 | Trending {apps,sites,filter-tags,keywords} | Mobbin UI (4 endpoints) | `mobbin trending apps/sites/filter-tags/keywords` | All four exposed |
| 13 | Searchable sites (web) | Mobbin UI | `mobbin sites list` | Web-first |
| 14 | Autocomplete | pdcolandrea / Mobbin UI | `mobbin autocomplete query "paywall"` | Fast ID lookup |
| 15 | List user workspaces | underthestars-zhy/MobbinAPI | `mobbin workspaces list` | Required for collection creation |
| 16 | List user collections | pdcolandrea | `mobbin collections list` | --json |
| 17 | Collection contents (paginated) | pdcolandrea | `mobbin collections contents --collection-id <id>` | Keyset pagination wrapped clean |
| 18 | **Create collection** | underthestars-zhy/MobbinAPI | `mobbin collections create --workspace-id <id> --name "Fintech paywalls"` | First CLI to ship this; PostgREST direct |
| 19 | **Add screen / flow / app to collection** | MobbinAPI | `mobbin collections add-screen --collection-id <c> --screen-id <s>` etc. | First CLI to ship this |
| 20 | **Remove screen / delete collection** | MobbinAPI | `mobbin collections remove-screen ...` / `mobbin collections delete <id>` | First CLI to ship this |
| 21 | Per-app flows + screens (HTML scrape) | pdcolandrea `getAppPage` / ismailsaleekh | `mobbin app <slug>` with `flows` / `screens` / `versions` subs | Next.js __next_f flight-chunk extractor as a stable surface |
| 22 | Auth login --chrome | solejay/mobbin-cli | `mobbin auth login --chrome --profile <name>` | Multi-profile auto-detect (generator-emitted) |
| 23 | Doctor / health | Standard | `mobbin doctor` | Generator-provided |

## Framework Baseline (Priority 0 — generator-provided)
- `mobbin sync` — orchestrate framework store ingester
- `mobbin search "<query>"` — FTS5 cross-entity search
- `mobbin sql "<query>"` — read-only SQL over local store
- `mobbin agent-context`, `mobbin which`, `mobbin api`, `mobbin profile`, `mobbin feedback`, `mobbin import`
- Cobra-tree MCP mirror (every Cobra command becomes an MCP tool)
- `mobbin doctor`, `auth status`, `auth logout`

## Transcendence (only possible with our approach — Priority 2 hand-built)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---------|---------|--------------------------|-------|
| 1 | Pattern deck export (with image download) | `mobbin deck "fintech paywalls" --platform web --limit 20 --export-zip ./deck.zip` | Search + Bytescale URL translation + batch full-res download + manifest CSV. Mobbin UI has no export. | 9/10 |
| 2 | Offline pattern bench | `mobbin bench --pattern paywall --industry fintech --platform web` | Local SQLite aggregate across screens × patterns × apps. No API returns this shape. | 8/10 |
| 3 | Flow audit with delta | `mobbin audit onboarding --platform web --industry b2b-saas --since 60d` | Joins synced flows × apps × industry × captured_at with --since. API has no time filter. | 8/10 |
| 4 | Version drift watch | `mobbin drift stripe-web --since 30d` | Diffs local app_versions snapshots over time. API exposes only current state. | 9/10 |
| 5 | Batch full-res grab | `mobbin grab --pattern empty-state --platform web --industry fintech --out ./refs --rename '{app}_{pattern}_{idx}.png'` | Bytescale translation + filename templating + manifest.json sidecar. | 8/10 |
| 6 | Cross-platform parity | `mobbin cross "paywall" --apps stripe,linear,figma` | Web + iOS fan-out joined on slug locally. Serves dual-platform mandate. | 8/10 |

## First-class capability across every screen-emitting command

**`--save-images` flag**: any command returning screens (search, deck, grab, app, sync, collections contents) accepts `--save-images` to download Bytescale full-res into `~/.cache/mobbin-pp-cli/images/{platform}/{app-slug}/{screen-id}.webp` and emit local paths in the output. `mobbin sync` runs this by default; opt out with `--no-images`. Image translation:
- Supabase storage URL → `bytescale.mobbin.com/FW25bBB/image/mobbin.com/prod/<path>?f=webp&w=1920&q=85&fit=shrink-cover`
- Width tunable via `--width 2560`; format via `--format png`

Cache dedup by screen UUID. `mobbin cache stats` / `mobbin cache prune` round it out (Priority 0 baseline).

## Stubs / Deferred
None. Every row is shipping scope.

## Decision Summary

- **23 absorbed features** match (and beat) every competing Mobbin tool's surface
- **6 transcendence features** ship hand-built
- **7 framework baselines** ship from generator
- **Image caching** is a first-class concern, wired into every screen-emitting command
- Total user-facing commands: **36+**

The two Mobbin tools that come closest:
- Official MCP exposes 1 tool. We expose ~36 + Cobra-tree MCP mirror with HTTP transport.
- pdcolandrea exposes 7 read tools. We expose every one of those PLUS write CRUD PLUS local store PLUS image cache PLUS 6 novel transcendence commands.
