# Clinical Trials Intelligence — Absorb Manifest

Sources surveyed: cyanheads/clinicaltrialsgov-mcp-server, JackKuo666/ClinicalTrials-MCP-Server, GSA-TTS/nih-clinicaltrials-mcp-server, BACH-AI-Tools (18 tools), aafjes/mcp-clinicaltrials.gov, R `ctrdata`, `pytrials`. All are single-source CT.gov wrappers.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Full-text study search | cyanheads search | `(generated endpoint) studies list` | Offline FTS5 mirror, `--json`, `--select` dotted paths, typed exit codes |
| 2 | Field/filter search (status, phase, condition, sponsor, location, date) | GSA-TTS, BACH-AI | `clinical-trials-pp-cli search` | Normalized filters across sources, not just CT.gov param passthrough |
| 3 | Fetch single study by NCT ID | all MCP servers | `(generated endpoint) studies get` | Field selection + normalized Trial view + enrichment join |
| 4 | Recruiting-only trials | aafjes filter | `clinical-trials-pp-cli recruiting` | Location/phase facets + recruiting velocity context |
| 5 | Phase III filter | BACH-AI | `clinical-trials-pp-cli phase3` | Phase distribution context, cross-source |
| 6 | Stats: study counts | GSA-TTS stats | `(generated endpoint) stats size` | Cached, composable with `sql` |
| 7 | Stats: field value distributions | GSA-TTS | `(generated endpoint) stats field-values` | Drives emerging/distribution intelligence offline |
| 8 | Enums / metadata / search-areas discovery | CT.gov native | `(generated endpoint) studies enums/metadata/search-areas` | Self-documenting for agents |
| 9 | Pagination + sort | all | `(behavior in clinical-trials-pp-cli search)` `--limit`, `--max-scan-pages`, sort flags | Bounded scans with `scanned_*` accounting |
| 10 | Patient/eligibility matching | cyanheads match | `clinical-trials-pp-cli match` | Eligibility-criteria parse + local filter; honest empty-on-no-match |
| 11 | Local sync / offline mirror | (none have real offline) | `(generated) sync` + SQLite | True offline; nobody else has this |
| 12 | SQL over trials | (none) | `(generated) sql` | Composable analytics over the local mirror |
| 13 | Health/doctor | partial (none multi-source) | `clinical-trials-pp-cli health` | Per-provider status/latency/failure-rate |

## Transcendence (only possible with our multi-source + SQLite approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Emerging therapy/condition detection | `emerging [category]` | hand-code | Requires time-windowed counts of interventions/conditions across `stats/field/values` + snapshots; no single API call returns growth % | Use for "where is the field moving" — fastest-growing categories with % change. Do NOT use for a single trial's status; use `risk` or `studies get`. |
| 2 | Recruitment velocity | `velocity "<term>"` | hand-code | Requires historical SQLite snapshots of recruiting counts/enrollment over time | Use for how fast a disease area's trials/enrollment are growing. Needs ≥2 synced snapshots. |
| 3 | Drug/intervention compare | `compare "<drugA>" "<drugB>"` | hand-code | Requires RxNorm normalization (synonyms) + cross-source trial counts/phases/sponsors merged | Use to compare two drugs head-to-head across trial activity. RxNorm resolves brand↔generic so "Keytruda" == "pembrolizumab". |
| 4 | Trial risk score | `risk "<nct-id>"` | hand-code | Requires composite of termination history, enrollment vs target, single-site, small N, sponsor track record, results-posted compliance — a local join no API exposes | Explainable risk read on one trial. Each factor shown with its contribution. |
| 5 | Watch + change diff | `watch "<term>"` | hand-code | Requires diffing current results against last SQLite snapshot | Use to track a term over time; reports new/changed/terminated since last run. |
| 6 | Geographic hotspots | `(behavior in emerging --by geo)` / `hotspots "<term>"` | hand-code | Requires aggregating `locations[].country` across the merged set | Country/region ranking of recruiting activity for a term. |
| 7 | Sponsor dominance | `sponsors "<term>"` | hand-code | Requires sponsor-name normalization + classification (industry/academic/gov) + aggregation | Ranks who runs the most trials in an area, with sponsor type. |
| 8 | Multi-source merge + dedup | `(behavior in search/recruiting --source all)` | hand-code | Dedup by NCT ID + EUCTR/WHO cross-IDs + title similarity across CT.gov, CTIS, OpenAlex; unify statuses | The core engine: one normalized Trial from many registries. Nobody else does this. |
| 9 | Publication / evidence linkage | `evidence "<nct-id>"` | hand-code | Requires PubMed + OpenAlex lookup by NCT ID / title → did it publish? success signal | Links a trial to its publications and citation context. |
| 10 | Safety signals (FAERS) | `safety "<drug>"` | hand-code | Requires OpenFDA FAERS query by RxNorm-normalized drug | Adverse-event signal for a trial's intervention drug. |
| 11 | Clinician landscape report | `report "<term>" --format md|csv` | hand-code | Requires composing search + emerging + hotspots + sponsors into one artifact | Weekly clinician-facing summary (ties into user's existing report workflow). |
| 12 | Per-provider health | `health` | hand-code | Requires probing each provider for status/latency/failure-rate + cache state | Shows the fallback chain's live state, including WHO ICTRP "bulk-only" honesty. |

Minimum 5 transcendence met (12 listed). Stubs: **none planned as stubs.** WHO ICTRP `--source who` returns an honest "no public API; bulk-import only" message — this is documented behavior, not a stub of a promised feature.

## Hand-code count
- Transcendence rows tagged `hand-code`: **12** (all of P2 + the secondary-source providers behind them).
- `spec-emits` (generator-produced endpoint surface): studies list/get/metadata/enums/search-areas, stats size/field-values/field-sizes, version.
