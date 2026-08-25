# Clinical Trials Intelligence CLI — Research Brief

## API Identity
- **Domain:** Clinical trials registries + biomedical enrichment. Multi-source aggregator.
- **Users:** Clinical researchers, physicians, medical/biotech analysts, evidence-synthesis teams, AI agents doing literature/trial review.
- **Data profile:** ~570k+ studies on ClinicalTrials.gov alone; each study is a deeply nested protocol object (status, phase, conditions, interventions, sponsors, locations, dates, eligibility, outcomes, results). Enrichment data: drug names/relations (RxNorm), approvals + adverse events (OpenFDA), publications (PubMed/OpenAlex), disease ontology (MeSH).

## Source Priority (combo CLI — see source-priority.json)
- **Primary:** ClinicalTrials.gov — official OpenAPI v2.0.5 (`https://clinicaltrials.gov/api/oas/v2`), base `https://clinicaltrials.gov/api/v2`. No auth. Headline commands + README top.
- **Secondary (fallback):** EU CTIS (`POST https://euclinicaltrials.eu/ctis-public-api/search`, JSON) → OpenAlex (context) → SQLite cache (last resort).
- **Enrichment:** OpenFDA (`api.fda.gov`), RxNorm/RxNav (`rxnav.nlm.nih.gov`), PubMed (NCBI E-utilities), MeSH (`id.nlm.nih.gov` + E-utilities `db=mesh`).
- **Economics:** All free. OpenFDA + PubMed accept an OPTIONAL key for rate limits only. No paid gating of any command.
- **Inversion risk:** Primary (CT.gov) has the richest spec AND is the intended headline — no inversion pressure. WHO ICTRP has no spec but is NOT promoted (see Reachability Risk); do not let any cleaner secondary spec displace CT.gov.

## Reachability Risk
- **CT.gov:** None. Live JSON 200; OpenAPI spec resolves.
- **EU CTIS:** Low. Public POST search API returns JSON (confirmed by community libs: JulHeg/euclinicaltrials.py, parseforge scraper). Portal-fed API, schema can drift — isolate in adapter.
- **WHO ICTRP:** **HIGH / no live API.** Crawling service "currently not available" (2025); access is web portal + on-request SharePoint bulk download only. **Decision: do NOT ship a live WHO provider.** Cover WHO trials indirectly through CT.gov `secondaryIdInfos[].domain == "WHO"` cross-IDs, and leave a documented `--source who` hook that returns a clear "bulk-import only" message + pointer. This is an honest gap, surfaced in README Known Gaps and `health`.
- **OpenAlex / OpenFDA / RxNorm / PubMed / MeSH:** None. Stable public REST.

## Top Workflows
1. **"What's running for <condition>?"** — search/recruiting trials by disease, filter by phase/status/location.
2. **"Is this drug advancing?"** — compare two drugs/interventions across trial counts, phases, sponsors, recruiting status.
3. **"Where is the field moving?"** — emerging therapies/conditions: fastest-growing categories by trials started per time window.
4. **"Should I trust this trial?"** — risk read on a single NCT ID (termination history, slow/zero enrollment, single-site, tiny N, sponsor track record, results-posted compliance).
5. **"Tell me when something changes"** — watch a term; diff against last SQLite snapshot; report new/changed/terminated trials.
6. **"Give me the weekly landscape"** — clinician-facing markdown/CSV report (ties into the user's existing weekly-report workflow).

## Table Stakes (from competitor CT.gov MCP servers: cyanheads, JackKuo666, GSA-TTS, BACH-AI-Tools 18 tools, aafjes; R `ctrdata`; `pytrials`)
- Full-text + field search of studies; filters (status, phase, condition, intervention, sponsor, location, date).
- Fetch single study by NCT ID with field selection.
- Pagination, sort by relevance/date.
- Stats endpoints (study counts, field value distributions).
- Patient-to-trial eligibility matching (cyanheads).
- Enums / metadata / search-areas discovery.

## Data Layer
- **Primary entities:** `study` (normalized Trial), `sponsor`, `location`, `intervention`, `condition`, `snapshot` (time-series for velocity/watch), `publication` (PubMed/OpenAlex links), `drug` (RxNorm normalized), `adverse_event` (OpenFDA FAERS).
- **Sync cursor:** CT.gov `lastUpdatePostDate` + `nextPageToken`; per-source TTL (CT.gov 7d, CTIS 14d).
- **FTS/search:** SQLite FTS5 over title/condition/intervention/sponsor for offline search.
- **Time-series:** `snapshot(query_hash, captured_at, trial_id, status, enrollment)` enables real recruitment velocity + watch diffing — the feature no single API call provides.

## Codebase Intelligence
- CT.gov data already embeds `secondaryIdInfos` with `domain: "EU CTR"` and `domain: "WHO"` → free cross-registry dedup keys for the merge engine.
- Essie expression syntax for `query.*` params; `filter.*`/`postFilter.*` for facets. Field selection via `fields` param (dotted paths into protocolSection) — critical for bounded agent output.
- Response is deeply nested (`protocolSection.identificationModule.nctId`, etc.) — normalization layer must flatten to the unified Trial.

## User Vision (USER_BRIEFING_CONTEXT)
Full multi-API intelligence system, NOT an API wrapper. 5 layers (providers → normalize → merge → intelligence → output), 8 sources, graceful degradation down a fallback chain, SQLite cache, never crash. Commands: search, recruiting, phase3, compare, emerging, watch, risk, health. Refinements requested: RxNorm drug-name normalization, MeSH disease normalization, sponsor classification, phase-transition tracking, result/publication linkage, FAERS safety signals, markdown/CSV clinician report.

## Product Thesis
- **Name:** Clinical Trials Intelligence (slug `clinical-trials`, binary `clinical-trials-pp-cli`, primary brand ClinicalTrials.gov).
- **Why it should exist:** Every existing tool is a single-source ClinicalTrials.gov pass-through (MCP or Python). None aggregates across registries, normalizes to one model, dedups, or computes intelligence (velocity, emerging therapies, abandonment, hotspots, sponsor dominance, risk). This is the first **clinical intelligence system**, not a database viewer — with offline SQLite, agent-native `--json/--select`, and graceful multi-source degradation.

## Build Priorities
1. **P0 foundation:** generate CT.gov endpoint surface from OpenAPI; normalization layer → unified Trial; SQLite store + FTS + snapshots; sync.
2. **P1 absorb:** match every competitor feature (search/recruiting/phase filter/single-study/stats/eligibility match) with offline + `--json` + typed exit codes.
3. **P2 transcend:** intelligence engine (`emerging`, `compare`, `risk`, recruitment velocity, geographic hotspots, sponsor dominance, `watch` with diff) + secondary-source providers (CTIS, OpenAlex, OpenFDA, RxNorm, PubMed, MeSH) behind the merge engine + `health` + clinician report output.
