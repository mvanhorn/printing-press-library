# Retraction Checker CLI Brief

## API Identity
- Domain: scholarly metadata / research integrity
- Primary source: Crossref REST API (https://api.crossref.org) — free, keyless, polite pool via `mailto`
- Secondary source: OpenAlex (https://api.openalex.org) — free, keyless; citation-ranked related works
- Users: researchers, librarians, meta-scientists, and AI agents doing citation-integrity checks
- Data profile: 73,126+ retraction notices in Crossref (embedded Retraction Watch data); each work carries `update-to`/`update-by` records with `type: retraction`, date, and source

## Reachability Risk
- None. Live probe: `GET /works?filter=update-type:retraction&rows=1` → HTTP 200, 73,126 results.
- Single work: `GET /works/10.1016/j.micpro.2020.103768` → HTTP 200 with `update-to[].type=retraction` and title `RETRACTED:` prefix.

## Retraction detection signals (Crossref)
- `message.update-to[]` with `type` in {retraction, correction, withdrawal, expression_of_concern, removal} → this record is/points to a retraction notice.
- `message.update-by[]` → this work has been retracted by the listed notice(s).
- Title prefix `RETRACTED:` / `WITHDRAWN:` → corroborating signal.
- `filter=update-type:retraction` lists retraction notices; each item's `update-to[].DOI` is the retracted paper.

## Top Workflows
1. Check one DOI/PMID for retraction status, date, reason source, notice.
2. Batch-scan a reading list / .bib for retracted entries.
3. Find current superseding research for a retracted/older paper (OpenAlex, citation-ranked).
4. Watch a topic or reading list for newly-announced retractions.

## Data Layer
- Local SQLite cache of checked works (DOI → retraction verdict + fetched-at) via modernc.org/sqlite.
- Watch state files (baseline of seen retraction notices per topic/list) via os.UserConfigDir, snapshot/first-run/diff pattern.

## Source Priority
- Primary: crossref — internal OpenAPI spec authored from docs + live probe — auth: free
- Secondary: openalex — aggregator-pattern hand-built source client — auth: free
- Economics: both keyless; no paid tier. No inversion risk (Crossref leads all headline commands).

## User Vision
- Fully keyless by default; every command works with zero configuration.
- Exactly four commands in the first PR: check, scan, superseded, watch. Do NOT build more.
- Optional AI synthesis on `superseded` is DEFERRED (keyless-only first PR).
- Go-only, no subprocess. Follow scientific-consensus conventions (cobra rootFlags, printJSONFiltered, isTerminal, boundCtx, classifyAPIError, modernc.org/sqlite, internal/{cli,source,client,store,cliutil,mcp}).

## Product Thesis
- Name: Retraction Checker (retraction-checker-pp-cli)
- Why it should exist: no keyless CLI turns Crossref's embedded Retraction Watch data into a one-shot retraction verdict, batch bibliography audit, superseding-research finder, and retraction watch.

## Build Priorities
1. Crossref source client + DOI/PMID resolution + retraction verdict parser.
2. check + scan (Crossref-only, keyless).
3. superseded (Crossref date context + OpenAlex citation-ranked related works).
4. watch (snapshot/baseline/diff).
