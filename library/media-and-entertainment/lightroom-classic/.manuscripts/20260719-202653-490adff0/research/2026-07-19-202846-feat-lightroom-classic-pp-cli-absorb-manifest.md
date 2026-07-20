## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Criteria photo search (date range, rating op, pick/reject, color label, keyword, collection, camera, lens, ISO/aperture/shutter/focal) | Lightroom-SQL-tools lrselect | lightroom-classic-pp-cli photos (alias: find) | Agent-native --json/--select/--csv, typed exits, no Python env, human EXIF units |
| 2 | List collections with image counts | lrcat-extractor (source) | lightroom-classic-pp-cli collections | --json, sorted, excludes systemOnly noise |
| 3 | List keywords with image counts | lrcat-extractor (source) | lightroom-classic-pp-cli keywords | --json, hierarchy via genealogy, zero-count visibility |
| 4 | List cameras/lenses with counts | Lightroom-SQL-tools | lightroom-classic-pp-cli cameras | first/last-seen dates per body (also: lenses) |
| 5 | Lenses listing | Lightroom-SQL-tools | lightroom-classic-pp-cli lenses | same as cameras, for glass |
| 6 | Resolve image to absolute path on disk | LightroomClassicCatalogReader | lightroom-classic-pp-cli path | works by id or filename; exists-on-disk check |
| 7 | CSV/JSON export of result sets | ExportLRCatalog | (behavior in lightroom-classic-pp-cli photos (alias: find)) --json/--csv/--select on every listing command | pipes to jq; stable field names for build scripts |
| 8 | APEX aperture/shutter conversion to f-stop and fraction | Lightroom-SQL-tools | (behavior in lightroom-classic-pp-cli photos (alias: find)) all EXIF output human-readable + raw values in --json | agents get both machine and human units |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Streak & gap tracker | streaks | hand-code | 10 | Day-level coverage (current/longest streak, missing dates) over 10 years of captureTime — no Lightroom feature or tool computes this | Reports day-level coverage. Use 'project' for fixed-length targets; 'on-this-day' for cross-year same-date lookup. |
| 2 | Daily pick resolver | pick-of-day | hand-code | 9 | Selection ladder (pick=1 → highest rating → latest touch) returning exactly one image per day with resolved path — the record a static-site build needs | Guarantees ≤1 result per day via tiebreak ladder; --range emits one per day. Differs from find, which returns all matches. |
| 3 | On-this-day | on-this-day | hand-code | 9 | Calendar-position retrieval across all years via dateMonth/dateDay — backbone of month/day browse pages | Groups by year for a fixed month/day (defaults today). find's date filters are range-based; this is calendar-based. |
| 4 | Project progress | project | hand-code | 8 | "Day 63 of 100, 2 missed, on pace for DATE" accounting for collection-backed finite projects | Takes --collection and --target N; reports completion %, missed days, projected end. streaks is open-ended catalog-wide. |
| 5 | Shooting habits stats | stats | hand-code | 8 | Histograms by focal/hour/weekday/month/camera/lens/iso over harvested EXIF — Lightroom filters one facet, never aggregates | Returns bucketed counts only, not image rows. --by camera/lens includes first/last-seen dates. |
| 6 | Catalog doctor | doctor | hand-code | 8 | Stats every resolved path on disk to find missing masters + images without captureTime, orphan keywords, empty collections | Read-only health report; only command touching the filesystem beyond the .lrcat. Never writes. |
| 7 | Keeper funnel | funnel | hand-code | 7 | Shot → pick → rated → developed → collected conversion rates, optionally per year | Chains quality signals into ratios; stats is single-facet histograms. |
| 8 | Unedited backlog | backlog | hand-code | 6 | Picked/high-rated images with no develop adjustments — workflow debt Lightroom smart collections cannot express | Requires develop-settings join; not an absorbed find filter. |
