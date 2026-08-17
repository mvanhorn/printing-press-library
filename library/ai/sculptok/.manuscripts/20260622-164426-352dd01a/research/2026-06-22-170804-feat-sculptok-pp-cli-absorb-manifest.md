# SculptOK CLI — Absorb Manifest

Greenfield: no existing CLI / MCP / Claude skill / npm / PyPI for SculptOK (1 tiny personal GitHub repo only). Relief/depth-map competitors (Reliefmod, ImageToStl, DepthR, VOXELASE DepthGen Pro) are web-only with no CLI surface to absorb; their conceptual features (depth strength, invert, STL export, thickness) are already covered by SculptOK's own documented parameters. So "Absorbed" = full coverage of SculptOK's official 9-endpoint `api-open` surface, each matched and beaten with offline store, `--json`/`--select`, `--dry-run`, typed exit codes, and credit-cost awareness.

## Absorbed (match or beat the full official API surface)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Check credit balance | SculptOK GET /point/info | `(generated endpoint) account credits` | --json/--select, typed exit, free read |
| 2 | Credit history (paged) | GET /point/page | `(generated endpoint) account history` | offline SQLite mirror + framework search/analytics |
| 3 | Upload image | POST /image/upload (multipart) | `(behavior in sculptok-pp-cli generate <kind>)` | local-path upload handled inside the workflow commands (multipart not in scalar param set) |
| 4 | Depth-map draw (raw submit) | POST /draw/prompt | `(generated endpoint) draw depthmap` | all documented params as flags: style/hd_fix/optimal_size/extInfo/version/draw_hd with enums + defaults |
| 5 | Background removal / HD restore (raw submit) | POST /draw/hd/prompt | `(generated endpoint) draw restore` | hdFix/removeBack flags (2 cr) |
| 6 | 3D draw (raw submit) | POST /draw/3d/prompt | `(generated endpoint) draw threed` | hd_fix precision flag (basic/standard/high) |
| 7 | Image-to-STL (raw submit) | POST /draw/stl/prompt | `(generated endpoint) draw stl` | width_mm/min_thickness/max_thickness/invert/scale_image flags w/ documented ranges |
| 8 | Drawing status (poll) | GET /draw/prompt?uuid | `(generated endpoint) draw status` | parses imgRecords[3]/queue position/currentStep |
| 9 | Drawing history (paged) | GET /image/page | `(generated endpoint) account drawings` | offline SQLite mirror |

Notes: raw `draw <kind>` submit commands return a promptId only (async). They exist as the absorbed 1:1 surface; the headline value is the hand-built `generate` family (transcendence #1) that chains upload+submit+poll+download. The `{code,msg,data}` envelope is unwrapped via `response_path: data`; the hand-built workflow commands additionally treat `code != 0` as a typed error.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | One-command generate (image -> result) | `generate depthmap\|stl\|threed\|restore <local-image>` | hand-code | Chains /image/upload -> /draw/*/prompt -> polls GET /draw/prompt?uuid -> downloads imgRecords/STL/model, persists the job to SQLite, with a /point/info credit preflight. No single API call does this; the web app forces manual submit/refresh/download. | none |
| 2 | Batch generate over a folder | `generate <kind> --batch <dir>` | hand-code | Loops generate over a directory, sums credit preflight vs live balance, writes one job row per image, emits a --json summary. Headless asset-pipeline use no web UI offers. | Use for many images of the SAME draw kind; for a single image use `generate`. |
| 3 | Credit-cost preflight estimator | `cost <kind> [--version pro4k] [--batch <dir>]` | hand-code | Joins the static per-kind/version cost table (10/15/30/2/10/3) with live /point/info balance to report would-spend vs remaining before any credit is spent. The API has no estimate endpoint. | For a historical breakdown use `analytics`; this is a forward estimate. |
| 4 | Pre-process then draw | `generate depthmap <img> --restore-first` | hand-code | Submits /draw/hd/prompt (bg-removal+HD, 2 cr), polls it, feeds the cleaned src into /draw/prompt, persists both jobs — Priya's exact manual ritual in one command. | Adds a restore pass before the draw; for restore alone use `generate restore`. |
| 5 | Spend analytics by kind/day | `analytics --type credit_events --group-by kind` | spec-emits (framework) | Local aggregation over the credit_events store grouped by draw kind/day; the API exposes only raw paged history, no aggregation. Requires the custom local store wired by generate/sync. | Reports WHERE credits went; for a forward estimate use `cost`. |
| 6 | Reconcile credits vs jobs | `reconcile --db <path>` | spec-emits (framework) | Local join of credit_events against jobs to flag credits charged with no produced job (and vice versa) — impossible via the API's two separate paged endpoints. | Audits spend vs produced jobs; for grouped totals use `analytics`. |
| 7 | Offline job search | `search --type jobs --limit N` | spec-emits (framework) | FTS over locally stored jobs (kind/status/params/remarks); SculptOK has no search endpoint and relief competitors are web-only. | none |
| 8 | Sync local store from history | `sync --resources jobs,credit_events,images --since 7d` | spec-emits (framework) | Pages /image/page and /point/page into SQLite so search/analytics/reconcile work on a fresh machine. | none |

Hand-code scope commitment (genuine new Go, ~50-150 LoC each + root.go wiring): rows 1-4 (the `generate` workflow family incl. --batch and --restore-first, and `cost`), plus a sibling `internal/sculptok/` client for multipart upload + submit + poll + envelope-code handling, and the custom `jobs` local store schema. Rows 5-8 ride the generator's framework commands (sync/search/analytics/reconcile) once the local store + syncable history resources are declared in the spec.
