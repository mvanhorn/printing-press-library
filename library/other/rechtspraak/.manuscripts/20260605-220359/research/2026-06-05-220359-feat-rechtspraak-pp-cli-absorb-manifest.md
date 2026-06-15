# Rechtspraak CLI Absorb Manifest

## Source tools (catalog)

| Tool | Lang | URL | Notes |
|------|------|-----|-------|
| basm92/rechtspraak_cli | Shell | https://github.com/basm92/rechtspraak_cli | Closest direct competitor. Two scripts: get_rechtspraak_id, get_documents. No caching, no search. |
| maastrichtlawtech/rechtspraak-extractor | Python (PyPI) | https://github.com/maastrichtlawtech/rechtspraak-extractor | `get_rechtspraak`, `get_rechtstraak_metadata`. Ships pre-built `lido.db` SQLite. Closest local-cache pattern. |
| openstate/open-rechtspraak | Python+PG | https://github.com/openstate/open-rechtspraak | Server-grade pipeline. `make import_verdicts --start_date --end_date`. Also imports judge namenlijst. |
| digitalheir/rechtspraak-js | JS (npm) | https://github.com/digitalheir/rechtspraak-js | JSON-LD normalization. Surfaces real data-quality warts. |
| digitalheir/java-rechtspraak-library | Java | https://github.com/digitalheir/java-rechtspraak-library | Java client + citation graph features. |
| Nikki91D/rechtspraak | Python | https://github.com/Nikki91D/rechtspraak | Basic API client. |
| Spijkervet/dutch_jurisdiction_elastic_search | Python | https://github.com/Spijkervet/dutch_jurisdiction_elastic_search | Proves bulk-FTS demand. |
| maastrichtlawtech/extraction_libraries | Python | https://github.com/maastrichtlawtech/extraction_libraries | Academic legal data extraction monorepo. |
| **No MCP server exists for rechtspraak today.** | — | — | Clear seat at the table. |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | Search by decision-date range | basm92, rechtspraak-extractor | `search --from 2024-01-01 --to 2024-01-31` | Translates to repeated `date=` upstream; SQLite-persisted, agent-native --json, paginated, --since shortcuts | shipping |
| 1b | Search by publication-date range | (none) | `search --published-from --published-to` | Repeated `modified=` upstream; lets users filter when decisions HIT the public site vs when they were decided | shipping |
| 2 | Multi-court UNION | rechtspraak-extractor (single-court only) | `search --court HR --court RBAMS` (repeatable, comma-separated) | Repeated `creator=` upstream; offline name/code/PSI-URI resolution via Instanties cache | shipping |
| 3 | Multi-subject UNION | rechtspraak-extractor (single-subject) | `search --subject strafrecht --subject bestuursrecht_belastingrecht` (repeatable) | Repeated `subject=` upstream; friendly-name → PSI URI offline resolution | shipping |
| 4 | Procedure-type filter | java-rechtspraak-library | `search --procedure cassatie` | **API ignores `procedure=` — filtered locally against synced Proceduresoorten metadata**; multi/repeatable | shipping |
| 5 | Search by document type | basm92 | `search --type Uitspraak\|Conclusie` | spec-emits | shipping |
| 5b | **Local keyword AND filter** | (none — no upstream API for it either) | `search --keyword "huurprijs" --keyword "appartement"` | **Upstream has no free-text search — filters locally via FTS5 over title + inhoudsindicatie + uitspraak body**; repeatable flags are AND-combined | shipping |
| 5c | **Local exclude filter** | (none) | `search --exclude "kort geding"` | NOT-match against local FTS index; repeatable; addresses the long-tail keyword-overlap problem in Dutch legal search | shipping |
| 5d | **Phrase / regex filter** | (none) | `search --phrase "huurprijswijziging" --regex "/art\\.\\s*7:\\d+/"` | Quoted phrase via FTS5; regex over body text in SQLite | shipping |
| 5e | **FTS scope control** | (none) | `--summary-only` / `--full-only` / `--require-summary` / `--require-full` | Restrict where keyword/phrase/regex match — saves Joris's NLP corpus from no-summary noise | shipping |
| 5f | **Annotate narrowing steps** | (none) | `--annotate-count` | Print result count after each filter pass so users can iterate confidently | shipping |
| 6 | Get decision metadata by ECLI | all | `get ECLI:NL:HR:2024:1` | Local cache, compact default, --json for full | shipping |
| 7 | Get decision with full text | basm92 get_documents.sh | `get ECLI:... --full` | Cached locally, FTS-indexed | shipping |
| 8 | Get inhoudsindicatie summary | rechtspraak-extractor | `get ECLI:... --summary` | spec-emits via return=DOC | shipping |
| 9 | List courts (instanties) | java-rechtspraak-library | `courts list`, `courts get RBNHO` | Fully offline after first sync | shipping |
| 10 | List subject areas | basm92, rechtspraak-extractor | `subjects list`, `subjects tree` | Hierarchical tree view | shipping |
| 11 | List procedure types | java-rechtspraak-library | `procedures list` | Fully offline | shipping |
| 12 | Date-range bulk extract | basm92 get_documents.sh | `sync --from --to --court ...` | Incremental cursor, dedup | shipping |
| 13 | CSV output | rechtspraak-extractor | `--csv` everywhere | spec-emits | shipping |
| 14 | JSON output | rechtspraak-extractor | `--json` everywhere | spec-emits | shipping |
| 15 | Local SQLite cache | rechtspraak-extractor | First-party SQLite, FTS5 over summaries | Structured & queryable, not flat dump | shipping |
| 16 | JSON-LD normalized output | rechtspraak-js | `get ECLI:... --jsonld` | Reuses normalization patterns | shipping |
| 17 | Bulk FTS search | Spijkervet ES indexer | Local FTS in SQLite | No external server needed | shipping |
| 18 | Field projection | (none) | `--select` (generator built-in) | spec-emits | shipping |
| 19 | Health / doctor | (none) | `doctor` | spec-emits | shipping |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|------------------------|
| 1 | Appeal-chain walker | `chain ECLI:NL:HR:2024:1 [--depth N] [--json]` | hand-code | 10/10 | Walks the cassation/conclusie/eerdereAanleg/replaces graph in both directions; tree or JSON output. No competitor does this from a CLI. Mira's defining workflow. |
| 2 | Vindplaatsen / citations extractor | `citations ECLI:... [--format=bibtex\|csv\|json]` | hand-code | 9/10 | Parses the `psi:vindplaats` list into structured {journal, year, page, annotator}; emits BibTeX for Mira's voetnoten. rechtspraak-js flags vindplaatsen as a data-quality problem nobody else has surfaced as a command. |
| 3 | Dossier by zaaknummer | `dossier 22/00155 [--court ...] [--json]` | hand-code | 9/10 | Cross-instance case-file tracking. Local SQLite lookup by `zaaknummer`; falls back to live filtered scan when uncached. Brief workflow #4 — no competitor command. |
| 4 | Watch / incremental poll | `watch [--court HR] [--subject ...] [--since 7d] [--quiet] [--json]` | hand-code | 9/10 | Diffs the filter result against local SQLite; only prints new ECLIs; `--quiet` for cron. Sanne's ritual. openstate has the server-grade version, no CLI does. |
| 5 | Conclusie pairing | `conclusie ECLI:NL:HR:2024:1` | hand-code | 8/10 | Bidirectional HR-uitspraak ↔ A-G conclusie pairing via `psi:conclusie` relation. Mira's daily move. |
| 6 | Bulk archive ingest | `sync --archive <yyyy-mm-dd>` | hand-code | 8/10 | Downloads the official weekly full-corpus archive ZIP; bulk inserts to SQLite. Order of magnitude faster than paging the Atom feed for backfills. Joris's backfill workflow. |
| 7 | Court-code dictionary | `code RBAMS` / `code "Rechtbank Amsterdam"` | hand-code | 7/10 | Bidirectional offline lookup against the Instanties cache; emits code, full name, parent body, PSI URI. Agent-friction reducer for Anouk. |
| 8 | MCP server mode | `mcp serve` | spec-emits | 9/10 | First MCP server for Dutch case law. Generator auto-emits the wrapping; every absorbed + novel command becomes an MCP tool. Anouk-defining. |
| 9 | ECLI parse/validate | `ecli parse ECLI:NL:HR:2024:1` | hand-code | 6/10 | Pure-local regex parse into country/court/year/sequence with validation; bonus `ecli url`. Load-bearing for chain/citations command output. |
| 10 | **Narrowing pipe** | `narrow [--keyword X] [--exclude Y] [--phrase "..."] [--regex /.../] < eclis.txt` | hand-code | 9/10 | Reads ECLI list from stdin/file (or piped from search/watch/sync), applies local filters against synced FTS5 index plus on-demand content fetches, emits the narrowed ECLI list. Enables iterative legal-research workflows: `search ... \| narrow --keyword X \| narrow --exclude Y`. The defining workflow for legal-keyword overlap problems. |

**Hand-code count: 9.** Each ~50-150 LoC plus `root.go` wiring. `mcp serve` is auto-emitted (spec-emits). No stubs in this manifest.

**Note on absorbed search command:** the local-narrowing flags (#5b–#5f) are absorbed but are the defining capability — every competing tool exposes only the upstream metadata filters. Surfacing rich local narrowing as composable repeatable flags is what makes this CLI usable for the persona problem the user named: "100s of cases for almost every topic." The transcendence row `narrow` (#10) makes that same vocabulary chain-composable across commands.
