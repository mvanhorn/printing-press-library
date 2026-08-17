# Pexels CLI — Absorb Manifest

Binary: `pexels-pp-cli`. API is strictly read-only.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search photos (orientation/size/color/locale) | official pexels SDK | (generated endpoint) photos search | offline mirror, `--json`/`--select`/`--csv`, typed exits |
| 2 | Curated photos feed | official SDK | (generated endpoint) photos curated | agent-native, scriptable |
| 3 | Get photo by id | official SDK | (generated endpoint) photos get | structured envelope |
| 4 | Search videos (orientation/size/locale) | official SDK | (generated endpoint) videos search | same |
| 5 | Popular videos (min_width/min_height/min_duration/max_duration) | official SDK | (generated endpoint) videos popular | same |
| 6 | Get video by id | official SDK | (generated endpoint) videos get | same |
| 7 | Featured collections | official SDK | (generated endpoint) collections featured | same |
| 8 | My collections (user's own) | developer-ishan MCP / houseme CLI | (generated endpoint) collections list | only 2 ecosystem tools cover this |
| 9 | Collection media (type=photos/videos, sort=asc/desc) | official SDK | (generated endpoint) collections media | same |
| 10 | Download a photo at a chosen src size | CaullenOmdahl MCP | (behavior in pexels-pp-cli download) `--type photo --size <ladder>` | resolution-aware + ledger-tracked |
| 11 | Download a video at a chosen quality | CaullenOmdahl MCP | (behavior in pexels-pp-cli download) `--type video --quality hd|sd` | best-file fallback |
| 12 | Bulk download all results for a query | pexel-downloader / AguilarLagunas | (behavior in pexels-pp-cli download) `--limit --max-pages` | rate-aware checkpointing |
| 13 | Filename + folder templating | AguilarLagunas downloader | (behavior in pexels-pp-cli download) `--output --name-template` | id/photographer/alt tokens |
| 14 | Field projection (dot-paths, @sets) | agynio CLI | (behavior in pexels-pp-cli photos search) `--select`/`--compact` | global on every command |
| 15 | `{data, meta}` structured JSON envelope | agynio CLI | (behavior in pexels-pp-cli photos search) `--json` | framework envelope w/ pagination meta |
| 16 | auth login / auth status | agynio CLI | pexels-pp-cli auth | raw-key (no Bearer), doctor-integrated |
| 17 | Attribution string + attribution_html | pypexels / pexels-media skill | (behavior in pexels-pp-cli attribution) | ledger-wide, not one-off |
| 18 | Per-download .meta.json attribution sidecar | pexels-media skill | (behavior in pexels-pp-cli download) `--sidecar` | auto on every download |
| 19 | Best video-file selector | afshinator MCP | (behavior in pexels-pp-cli resolve) | target-dimension driven |
| 20 | Surface X-Ratelimit-Limit/Remaining/Reset | developer-ishan/afshinator MCPs | (behavior in pexels-pp-cli doctor) + persisted on every call | written to local rate ledger |
| 21 | Random photo | pypexels / official SDK helper | (behavior in pexels-pp-cli photos curated) `--random` | curated + random-page trick |
| 22 | Agent-native renamed/markdown output | afshinator MCP | (behavior in pexels-pp-cli photos search) `--agent` | enforced attribution in agent output |

Every generated endpoint command also sets the raw `Authorization` header (no Bearer) and a `User-Agent` (Cloudflare 403 guard), and surfaces rate-limit headers.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Quota forecaster | `quota forecast --resources photos,videos --max-pages N` | hand-code | Pexels hides rate headers on 429; we persist X-Ratelimit-* to a local ledger and forecast affordable pages + reset ETA — no single API call can | Use to check BEFORE a bulk pull whether it fits remaining quota. Do NOT use to show usage of a single call; that's `doctor`. |
| 2 | Best-fit resolution picker | `resolve <id> --target-width W --target-height H` | hand-code | Encodes Pexels' crop-vs-scale semantics across the 8-size photo ladder and video_files into a target-driven choice; requires the cached media record | Use to pick one resolution for a known id against a pixel target. Do NOT use to list all sizes (that's `--select`). |
| 3 | Dedup + rate-aware checkpointed download | `download "term" --type photo --limit N --max-pages M` | hand-code | Joins results against the local downloads ledger by id (skips dupes), watches surfaced X-Ratelimit-Remaining, checkpoints each page, halts before 429 | Use for repeated/large harvests that must not re-download and risk quota. Do NOT use to predict feasibility; use `quota forecast`. |
| 4 | Attribution ledger + SOURCES export | `attribution export --resources photos,videos --csv` | hand-code | Reads the local downloads ledger and emits SOURCES.md + per-file .meta.json (id, URLs, photographer, avg_color, alt, license, attribution + attribution_html) | none |
| 5 | Offline re-search of synced media | `search "term" --type photo` | hand-code | Local FTS over synced photo alt/photographer/query + video user.name, stable ordering, live fallback via sync-hint helper | Use to re-find media already synced. Do NOT use to discover NEW media not yet synced. |
| 6 | Photographer / attribution roll-up | `analytics --type photos --group-by photographer` | spec-emits | Local GROUP BY over synced photos + ledger for credit-balance / licensing review | none |

**Hand-code commitment:** 5 features require hand-written Go after generate (quota forecast, resolve, download, attribution export, offline-search FTS wiring). 1 (analytics group-by) is emitted by the framework data layer. Stubs: none.

## Data Layer
- Tables: photos, videos, collections, collection_media (synced) + downloads ledger + rate ledger (local-only).
- FTS over photo alt/photographer/query, video user.name.
- next_page full-URL normalized to int cursor for sync.
