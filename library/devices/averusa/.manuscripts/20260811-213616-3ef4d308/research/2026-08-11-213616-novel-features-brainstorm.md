# AVer USA Novel-Features Brainstorm (first print)

## Customer model

Four personas, grounded in the brief's Users (AV integrators/installers, IT/AV admins, education technology coordinators, procurement/RFP writers, field techs) and Top Workflows.

**1. Dana Reyes — AV design engineer / RFP writer** (procurement + integrator)
- **Today:** For every bid she opens the AVer product page for each candidate model in a browser, follows the "Specifications" link to the datasheet PDF, and transcribes fields (zoom, FoV, sensor, mount, presets, certifications) by hand into her spec-compliance table. Tabs open: AVer product page, three datasheet PDFs, her RFP template. She re-copies the same spec values every single bid. She cannot answer "does the CAM570 and CAM550 both do 12x optical?" without opening both PDFs side by side.
- **Weekly ritual:** At least one RFP response or recommended-configuration email per week, each requiring a per-model spec-compliance table verified against vendor datasheets.
- **Frustration:** Spec values live inside N separate PDFs. There is no side-by-side view, and transcription errors get caught by buyers.

**2. Marcus Bell — field installation technician** (field techs)
- **Today:** At the job site, phone or tablet in hand, he fights the Salesforce Experience-Cloud SPA, hunts the model, downloads a PDF and hopes it is the right revision of the manual. In a comms closet or basement with no signal, he has nothing — he waits, or installs from memory.
- **Weekly ritual:** 3-6 installs a week; each needs the exact model's user manual or quick-start guide; he also pre-stages a "job bag" of manuals before driving out.
- **Frustration:** The portal maze, wrong-revision risk (portal filenames are garbage like `AVer PTZ Link User Manual EN v1.02_2021.07.12.pdf`), and zero offline access in the field.

**3. Priya Natarajan — education technology coordinator** (ed-tech coordinators)
- **Today:** During budget cycles she evaluates document cameras and classroom audio for rollout proposals, reading white papers and checking which AVer models are current so she never specs a discontinued unit into a school bid. She manually re-checks the catalog every month because nothing tells her what changed.
- **Weekly ritual:** Reviewing the current catalog and support page, gathering evaluation material for one active proposal, matching models to classroom requirements.
- **Frustration:** Product freshness and white-paper material are scattered with no change signal and no structured current-vs-discontinued data.

**4. Elena Voss — IT/AV fleet administrator** (IT/AV admins)
- **Today:** Manages a deployed fleet of AVer PTZ/conference/document cameras. She periodically clicks through product pages and the /support/ download hub to see if AVer released anything new, downloads updated manuals to the shared drive, and maintains a bookmark list of PDF links she trusts — some silently 404.
- **Weekly ritual:** Checking for new/updated docs across the fleet's models, refreshing the shared manual drive, auditing her team's link list.
- **Frustration:** Nothing tells her when a document changed or a PDF link went dead. The catalog is unenumerable (no directory listing, no changelog), so stale and broken links are invisible until someone clicks them.

## Candidates (pre-cut)

| # | Feature | Command | One-liner | Persona | Source | Long Description | Verdict |
|---|---------|---------|-----------|---------|--------|------------------|---------|
| 1 | Model spec comparison | `compare CAM570 CAM550` | Side-by-side spec fields for 2+ models | Dana | (c) + (a) | See below | **KEEP** — local join, no LLM, no external service |
| 2 | Single-model spec dump | `specs CAM570 --json --agent` | One model's full spec fields as structured/agent output for RFP tables | Dana | (c) + (a) | See below | **KEEP** — overlaps `compare`; needs redirect |
| 3 | Doc reachability audit | `doctor --full` | HEAD-check every doc URL; flag 404s and soft-404 shells | Elena | (a) + (b) | none | **KEEP** — live probe already found a mislinked datasheet |
| 4 | What's-new diff | `whats-new` | Docs/products added or updated since last sync | Elena, Priya | (a) + (c) | none | **KEEP** — local sync-cursor diff, no API equivalent |
| 5 | Doc coverage matrix | `coverage conference-camera` | Per-model doc-type availability table, flags missing manual/spec | Dana, Elena | (c) | See below | **KEEP** — products⋈documents join |
| 6 | Offline job bag | `docs pack CAM570 --out ./site/` | Batch-download all docs for a model with stable names | Marcus | (a) + workflow 5 | See below | **KEEP** — offline-library workflow is explicit in brief |
| 7 | Current vs discontinued | `products status --category conference-camera` | Flag models AVer lists as discontinued | Priya, Elena | (b) | none | **KEEP** — observed live on /support/ page |
| 8 | Smart quick-start pick | `docs quickstart CAM570` | Auto-resolve + fetch the QSG for a model | Marcus | (a) | none | **KILL** — subsumed by `docs pack` (QSG included in the batch) |
| 9 | Newest-manual resolver | `docs latest CAM570` | Resolve the highest-revision manual for a model | Marcus, Elena | (a) | none | **KILL** — version pick folds into `docs pack` naming; per-model freshness folds into `doctor` |
| 10 | White paper reading list | `docs search --type white-paper --limit 20` | Curated evaluation-paper list | Priya | (b) | none | **KILL** — re-proposes absorbed feature #6 (type filter already ships) |
| 11 | PDF full-text search | `docs fts "auto framing"` | Content search inside PDF bodies | Dana | (b) | none | **KILL** — needs a PDF text-extraction dependency (external infra) + >200 LoC |
| 12 | Spec field diff | `specs --diff CAM570 CAM550` | Diff spec fields between two models | Dana | (a) | none | **KILL** — scope creep; `compare` renders the same side-by-side |
| 13 | Comparison-chart mode | `compare --docs CAM570 CAM550` | Compare doc *coverage* side by side | Elena | (c) | none | **KILL** — `coverage` already answers doc-availability; avoid two output shapes on `compare` |
| 14 | Firmware/software update tracker | `docs audit --firmware` | Track firmware/software doc updates per model | Elena | (b) | none | **KILL** — unverifiable (support-portal entityId gap covers ~500 articles) + software-manual type is already absorbed |

**Long Description for #1 `compare`:** Use this command to compare spec fields side-by-side across two or more AVer models. Do NOT use this command for a single model's full spec dump; use 'specs <model>' instead. Do NOT use this command to check which documents exist for a model; use 'coverage <category>' instead.

**Long Description for #2 `specs`:** Use this command to dump one model's structured spec fields as text/JSON for RFP compliance tables or agent pipelines. Do NOT use this command to compare models; use 'compare' instead.

**Long Description for #5 `coverage`:** Use this command to see which doc types (user-manual, spec-sheet, quick-start, white-paper, …) exist for every model in a category and which are missing. Do NOT use this command to compare spec values; use 'compare' instead.

**Long Description for #6 `docs pack`:** Use this command to batch-download every document for a model into one offline folder with stable `<model>-<type>` names. Do NOT use this command to fetch a single named document; use 'docs download <doc-id>' instead.

**Live-verification note (bash probes, 2026-08-11):** averusa.com product pages are static HTML with **zero `<table>` elements** (specs live only inside datasheet PDFs). The CAM570 page links BOTH `cam570-datasheet.pdf` AND the wrong `cam520pro3-datasheet.pdf` (mislink — evidence for `doctor`). The datasheet URL pattern `/business/downloads/datasheet-brochure/cam570-datasheet.pdf` returns 200 `application/pdf` (last-modified May 2026). The /support/ page contains 6× "Discontinued Devices" sections (evidence for `products status`).

## Survivors and kills

### Survivors

**1. `compare` — Model spec comparison (8/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes — Dana answers one cross-model "which model fits" question per bid, and she has an active bid weekly (brief workflow #3 is explicit).
2. **Wrapper vs leverage:** Not a wrapper — no AVer endpoint returns specs at all; spec fields exist only inside per-model datasheet PDFs. This is a cross-model local join nothing else provides.
3. **Transcendence proof:** Local SQLite join of `products` ⋈ `spec_fields` (extracted from each model's datasheet at sync time), rendered as a transposed matrix; agent/JSON shaped via global flags.
4. **Sibling kill:** `specs --diff` (#12) — killed because `compare` already renders the same side-by-side view with one command; a diff mode just adds a second output shape to the same join.
5. **Buildability:** `hand-code` — spec extraction + matrix renderer (~100-150 LoC) + root.go wiring; not generator-emittable. SQLite note: drain the per-model spec rows into plain structs (check `rows.Err()`, close) before running the follow-up row queries, per the open-rows constraint. Data source: `local` — must reject `--data-source live` with "no live equivalent — spec fields are only populated after sync".
6. **Long-description validity:** `specs` and `coverage` both survive with those exact names — redirect holds.

**2. `specs` — Single-model spec dump (8/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes — Dana builds a spec-compliance table for every quoted model; the brief's workflow #2 asks for "extractable spec fields" verbatim.
2. **Wrapper vs leverage:** Not a wrapper — fields are extracted from PDFs at sync time, never exposed by any AVer API.
3. **Transcendence proof:** Local SQLite `spec_fields` table, emitted as JSON/agent-shaped output for RFP auto-fill; the one-flag-ahead-of-compare shape.
4. **Sibling kill:** `specs --diff` (#12) — killed; cross-model field comparison is `compare`'s job.
5. **Buildability:** `hand-code` — shares the sync-time spec extraction with `compare`; single-model read path is simpler (~60-100 LoC). Data source: `local` — reject `--data-source live`.
6. **Long-description validity:** `compare` survives — redirect holds.

**3. `doctor` — Doc reachability audit (8/10, hand-code, strategy: `auto`)**
1. **Weekly use:** Yes — Elena audits the fleet's doc links weekly before updating the shared drive; soft-404s and mislinks are silent killers of installer trust.
2. **Wrapper vs leverage:** Not a thin rename — it is an audit over the entire local catalog, and it catches a real defect I verified live (the CAM570 product page links `cam520pro3-datasheet.pdf` as its datasheet).
3. **Transcendence proof:** Local `documents` table (URLs) driven through live HTTP HEAD checks; persists `last_checked`/`last_status` back into SQLite. No API endpoint offers this.
4. **Sibling kill:** `docs latest` (#9) — killed; per-model freshness is a per-URL health question, folded into `doctor`'s status output.
5. **Buildability:** `hand-code` — net/http HEAD + soft-404 shell detection (~80 LoC). Data source: `auto` — live HEAD checks with local cached results as fallback; write last-check status outside any open BeginTx (WAL single-writer; treat upsert errors as command failures).
6. **Long-description validity:** `none`.

**4. `whats-new` — What changed since last sync (7/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes — Elena and Priya both re-check the catalog weekly and currently have no signal (no changelog exists on averusa.com).
2. **Wrapper vs leverage:** Not a wrapper of an API — there is no "what changed" endpoint; the value is the local sync-cursor delta. (Framework `tail --resource documents --since` only replays a timestamp window; `whats-new` diffs against the user's own last-sync state, a semantic no generated command has.)
3. **Transcendence proof:** Local SQLite `sync_state` cursor compared against `documents`/`products` `updated_at` to list added/updated rows since the previous `docs sync`.
4. **Sibling kill:** A `sync report --verbose` candidate was considered and killed in Pass 2 trimming — same delta, but duplicating `whats-new`'s output; `whats-new` owns the diff.
5. **Buildability:** `hand-code` — sync-state table read + diff (~60 LoC). Data source: `local`; calls `hintIfUnsynced(cmd, db, "")` then `hintIfStale` before returning.
6. **Long-description validity:** `none`.

**5. `coverage` — Doc-type availability matrix (7/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes — Dana checks per-category doc readiness for a recommendation and Elena audits fleet doc coverage weekly.
2. **Wrapper vs leverage:** Not a wrapper — a LEFT JOIN of `products` ⋈ `documents` pivoted on the 8-value doc-type enum, producing missing-doc flags no single catalog call yields.
3. **Transcendence proof:** Pure local SQLite aggregate across the two synced tables, rendered as a model×doc-type matrix with MISSING markers.
4. **Sibling kill:** `compare --docs` (#13) — killed; doc availability is `coverage`'s shape, not a second mode on `compare`.
5. **Buildability:** `hand-code` — pivot query + table renderer (~70 LoC). Data source: `local`; drain-first pattern for the join, reject `--data-source live`.
6. **Long-description validity:** `compare` survives — redirect holds.

**6. `docs pack` — Offline job bag (8/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes — Marcus pre-stages manuals for 3-6 installs a week, and the brief's "offline library" workflow is explicit.
2. **Wrapper vs leverage:** More than a batch rename — model-scoped grouping, type-priority ordering, and stable `<model>-<type>.<ext>` naming that neutralizes the portal's garbage filenames.
3. **Transcendence proof:** Groups the local `documents` table by model and drives the existing `docs download` flow per member with deterministic naming; `--dry-run` lists the bag without fetching.
4. **Sibling kill:** `docs quickstart` (#8) and `docs latest` (#9) — killed; QSG+newest-manual resolution are exactly the type-priority/version rules `pack` applies when assembling the bag.
5. **Buildability:** `hand-code` — grouping + name-mangling + delegation (~80 LoC). Data source: `local`; sync hints on `documents`.
6. **Long-description validity:** `docs download` survives as an absorbed generated command — redirect holds.

**7. `products status` — Current vs discontinued (6/10, hand-code, strategy: `local`)**
1. **Weekly use:** Yes (during procurement cycles) — Priya must never spec a discontinued model; Elena plans fleet refreshes; both re-check the catalog weekly.
2. **Wrapper vs leverage:** Not a wrapper — current/discontinued status is not structured anywhere in the API surface; the /support/ page's "Discontinued Devices" lists are HTML text.
3. **Transcendence proof:** Local `products.discontinued` flag (populated at sync from the /support/ Discontinued Devices sections) filtered and grouped by category — a service-specific content pattern.
4. **Sibling kill:** No close sibling among candidates because I generated one — a `products current` alias was killed as a duplicate; `products status` is the single command.
5. **Buildability:** `hand-code` — one boolean column (minor data-model addition) + /support/ page section parse (~50 LoC). Data source: `local`; sync hints on `products`.
6. **Long-description validity:** `none`.

**Survivor transcendence table**

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Model spec comparison | `compare CAM570 CAM550` | 8/10 | hand-code | Joins local `products` ⋈ `spec_fields` (fields extracted from each model's datasheet PDF at sync time) and renders a transposed spec matrix, no external dependencies | Brief workflow #3 "Compare models"; live probe: `cam570-datasheet.pdf` returns 200 application/pdf; product page has zero `<table>` elements, so specs exist only in PDFs | Use this command to compare spec fields side-by-side across AVer models. Do NOT use it for a single-model spec dump; use 'specs <model>' instead. Do NOT use it to check document availability; use 'coverage <category>' instead. |
| 2 | Single-model spec dump | `specs CAM570 --json --agent` | 8/10 | hand-code | Reads local `spec_fields` (populated at sync from the datasheet) and emits structured text/JSON for RFP tables, no external dependencies | Brief workflow #2 explicitly asks for "extractable spec fields"; live probe confirms Specifications links resolve only to PDFs | Use this command to dump one model's structured spec fields for RFP compliance or agents. Do NOT use it to compare models; use 'compare' instead. |
| 3 | Doc reachability audit | `doctor --full` | 8/10 | hand-code | Reads PDF URLs from the local `documents` table and issues HTTP HEAD/GET checks to flag 404s and 61301-byte soft-404 shells, caching results to SQLite, no external dependencies | Brief Reachability Risk documents the soft-404 shell; live probe found the CAM570 page linking `cam520pro3-datasheet.pdf` alongside `cam570-datasheet.pdf` (mislink) — dead/mislinked links are real | none |
| 4 | What's-new diff | `whats-new --since 7d` | 7/10 | hand-code | Compares the local `sync_state` cursor against `documents`/`products` `updated_at` to list rows added or updated since last sync, no external dependencies | No changelog or diff surface exists on averusa.com (homepage/support pages show none); offline-library workflow implies churn tracking | none |
| 5 | Doc coverage matrix | `coverage conference-camera` | 7/10 | hand-code | LEFT JOINs local `products` ⋈ `documents` and pivots on the 8-value doc-type enum to render a model×type matrix with MISSING flags, no external dependencies | Brief data layer defines categories→models→documents with the doc-type enum; live /support/ page shows per-model doc coverage is non-uniform | Use this command to see which doc types exist per model in a category and what is missing. Do NOT use it to compare spec values; use 'compare' instead. |
| 6 | Offline job bag | `docs pack CAM570 --out ./site/ --dry-run` | 8/10 | hand-code | Groups the local `documents` table by model and runs the existing `docs download` flow per member with deterministic `<model>-<type>` naming, no external dependencies | Brief workflow #1 "Get the manual" (field ritual) + offline-library workflow; portal filenames are unstructured ("AVer PTZ Link User Manual EN v1.02_2021.07.12.pdf") so stable naming is required | Use this command to batch-download every document for a model into one offline folder. Do NOT use it to fetch a single named document; use 'docs download <doc-id>' instead. |
| 7 | Current vs discontinued | `products status --category conference-camera` | 6/10 | hand-code | Reads a local `products.discontinued` flag (set at sync from the /support/ page's "Discontinued Devices" sections) and filters/group by category, no external dependencies | Live probe: /support/ page contains 6 "Discontinued Devices" sections (e.g., CAM130, VC520, PTZ330N) — the only current/discontinued signal, not structured in the brief's data layer | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `docs quickstart <model>` | Smart-pick logic is exactly the type-priority rule `docs pack` applies when assembling the offline bag | `docs pack` |
| `docs latest <model>` | Version/revision resolution folds into `docs pack` naming; per-doc freshness is a `doctor` health question | `docs pack`, `doctor` |
| White paper reading list | Re-proposes the absorbed `--type white-paper` filter; nothing new beyond a preset search | `docs search` (absorbed) |
| PDF full-text search (`docs fts`) | Requires a PDF text-extraction dependency and >200 LoC content pipeline — scope creep with an external infra cost | `specs` |
| `specs --diff A B` | Scope creep — `compare` already renders the same side-by-side from the same join | `compare` |
| `compare --docs` | Second output shape on `compare`; doc availability is already `coverage`'s job | `coverage` |
| Firmware/software update tracker | Unverifiable: the support-portal entityId gap covers ~500 articles, and the software-manual doc type is already absorbed | `whats-new` |
