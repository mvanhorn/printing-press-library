# The Lancet CLI — Phase 3 Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 gate passed (per-row Cobra resolution + dogfood novel_features_check planned==found==6).

## Data-source pivot
CrossRef carried zero author affiliations for Lancet (0/39 sampled), breaking 4 of 6
transcendence features. User approved switching to OpenAlex (free, no auth, 61% of
authorships carry institutions, 100% of works carry a topic). Spec re-authored and
CLI regenerated against `https://api.openalex.org`.

## Built — foundation (Priority 0)
- `internal/lancet/` hand-authored package: Lancet journal ISSN registry (19 titles),
  OpenAlex cursor-paginated sync that decomposes works into typed SQLite tables
  (`lancet_works`, `lancet_authorships`, `lancet_affiliations`), and the analytical
  query helpers. Table-driven tests in `lancet_test.go` (Lookup, NameForISSN, shortID,
  RankAuthors, CoAuthorMesh, Curate, TopicDrift) — all pass.
- `refresh` command: `thelancet-pp-cli refresh --journal <slug|all> [--years N] [--max-pages N]`.
  Bounded fetch; sorts oldest-first when `--years` is set so window analytics have history.

## Built — absorbed (Priority 1)
- `works search` scoped to the Lancet family by default (injects family ISSN OR-filter
  when caller supplies none). Confirmed: "malaria vaccine" returns only Lancet sources.
- `works get`, `authors search/get`, `sources get` — generated typed endpoints.
- `journals` — lists the Lancet family registry.
- `cited-by <doi-or-id>` — resolves DOI→OpenAlex ID, lists citing works via cites: filter.
- Output behaviors (--json/--csv/--select/--dry-run/--per-page/--page/--all) from framework.

## Built — transcendence (Priority 2)
All read the local store with the missing-mirror guard + dry-run short-circuit:
1. `rank-authors` — authors by total citations, scoped by journal/institution.
2. `mesh` — co-authorship pairs within an institution.
3. `affiliation-growth` — institutions gaining publication velocity (recent vs prior window).
4. `drift` — topic-distribution shift between two year windows.
5. `curate` — topic reading lists, Markdown/BibTeX/JSON export.
6. `visibility-gap` — author citation impact vs journal prestige proxy.

Live-verified against a 400-work flagship sync and a 2000-work lancet-oncology sync.

## Deferred / stubbed (reconciled in manifest, honestly labeled)
- Fetch by PMID — OpenAlex keys works by OpenAlex ID/DOI, not PMID. `(stub)`.
- Batch DOI validation — not built in v1; single-record via `works get`. `(stub)`.
- References / related — available in the `works get` payload (`referenced_works`,
  `related_works`) rather than as standalone commands. `(behavior in works get)`.

## Notes for retro
- Phase 1.9 reachability validated connectivity but not field coverage; the affiliation
  gap should have surfaced there, not at Phase 3. Recommend a field-coverage probe for
  data-layer-critical fields.
- Early artifacts were written to a literal `C:\home\LACI\...` path (Write tool resolved
  `/home/...` to the drive root under git-bash); consolidated into the run research dir.
