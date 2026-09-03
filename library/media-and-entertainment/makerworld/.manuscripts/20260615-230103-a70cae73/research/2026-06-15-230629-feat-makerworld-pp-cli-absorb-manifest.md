# MakerWorld CLI — Absorb Manifest

## Ecosystem scanned
- **bambu-mcp** (`schwarztim/bambu-mcp`) — MCP server; read `docs/cloud-api-reference.md`, `docs/design-service-api.md`, `src/makerworld.ts` (ground-truth endpoints + download resolution).
- **Apify actors** — MakerWorld Models Details Scraper, Models Search Scraper, Maker Projects Extractor (feature parity targets).
- **3D GO** — multi-platform mobile app (search/collections/preview across Thingiverse/Printables/Cults3D/MakerWorld).
- No existing CLI for MakerWorld (or any peer platform) — greenfield.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword search models | Apify search scraper; web `select/design2` | makerworld-pp-cli search <kw> | offline FTS over mirror, --json/--select/--csv, orderBy score/hotScore |
| 2 | Model (design) details | Apify details scraper; bambu-mcp | (generated endpoint) designs get | --select deep fields, --compact high-gravity |
| 3 | Browse trending / category | web UI `select/design/nav` | (generated endpoint) designs list | navKey enum (`--nav`), pagination, agent output |
| 4 | List categories | web UI `homepage/nav` | (generated endpoint) categories list | offline, stable keys |
| 5 | Designer's models | Apify maker scraper | (generated endpoint) designers models | local mirror, counts |
| 6 | Related designs | bambu-mcp `/design/{id}/relate` | (generated endpoint) designs related | composable IDs |
| 7 | Remixes of a design | bambu-mcp `/design/{id}/remixed` | (generated endpoint) designs remixes | lineage-ready |
| 8 | Comments & ratings | bambu-mcp `comment-service/commentandrating` | (generated endpoint) designs ratings | aggregate star stats |
| 9 | Download 3MF | bambu-mcp `makerworld_download` | makerworld-pp-cli download <designId> | auth tier (bearer); auto-resolve default instance |
| 10 | Favorites / liked (account) | bambu-mcp; web UI | makerworld-pp-cli favorites | auth tier (MAKERWORLD_TOKEN); graceful no-token state |
| 11 | Recommended-for-you | web UI `recommand/youlike` | (generated endpoint) recommend list | agent output |

## Transcendence (only possible with our local-SQLite + agent-native approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Quality-blend discovery | discover | hand-code | Joins per-instance ratingScoreTotal/ratingCount against printCount/downloadCount in local SQLite and ranks by a composite quality signal (`--sort quality\|staff-picks`); no scraper or the web UI exposes a combined quality filter | Use this to find models that are both well-rated AND popular from the local mirror. Do NOT use it for what is newly rising; use 'movers' instead. |
| 2 | Designer-watch deltas | designers deltas | hand-code | Set-diffs each watched designer's mirrored catalog/counts between the two most recent syncs to report new uploads and rank rises — "new since last visit" exists nowhere upstream | Use this for the roll-up of new uploads/rank rises across all synced designers since the last sync. For absolute current catalog use 'designers models'. |
| 3 | Trend movers | movers | hand-code | Ranks designs by largest positive delta in like/download/print between the two latest syncs — a reproducible time-delta the platform's opaque server-side Trending never shows | Use this for what is rising between syncs. For absolute current popularity use 'designs list --nav Trending'; for quality-vs-popularity blend use 'discover'. |
| 4 | Multi-tag discovery | tags | hand-code | Tag-cloud aggregation + case-insensitive multi-tag intersection over the local mirror; the MakerWorld UI filters by only one tag at a time. (Replaced `designs lineage` after build-time discovery that MakerWorld's /remixed endpoint returns empty for all designs — user re-approved the swap.) | none |
| 5 | Printer/AMS-fit filter | discover --printable | hand-code | Filters mirrored instances[] by needAms/materialCnt/weight/instanceFilaments so only models matching the user's printer/material setup surface — a buyer-side need scrapers ignore | none |

### Transcendence implementation notes
- New top-level hand-coded commands: `discover` (quality blend + printer-fit modes), `movers`, `tags`. New hand-wired child: `designers deltas`.
- `--printable` (+ `--no-ams`, `--material`, `--max-weight`) are filter flags on `discover` (printer-fit row folded into `discover`).
- Hand-code scope = 4 command files: `discover`, `movers`, `designers deltas`, `tags`.
- `discover`, `movers`, `designers deltas`, `tags` read the local SQLite mirror (`pp:data-source local`; require `sync` first; emit unsynced hint). `discover` additionally enriches the top candidates live when `--no-ams`/`--max-weight` are set.

## Data layer (Priority 0)
- Resources synced to SQLite: `designs` (full detail + counts + instances + tags + contest), `designers` (creators), `ratings` (per-design comment-service rollup). Sync stores snapshots so movers/deltas can diff across runs.
- Offline FTS over title/summary/tags/creator.

## Auth tiers
- **free (default):** all reads (search, browse, categories, designs get/related/remixes/ratings, designers models, recommend) — no auth, `api.bambulab.com/v1`.
- **bearer (optional):** download 3MF, favorites/liked — `MAKERWORLD_TOKEN` (Bambu Cloud JWT).
