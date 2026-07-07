# Rechtspraak CLI Brief

## API Identity
- Domain: Dutch court decisions (`uitspraken`) - search, retrieve, analyze
- Users: Lawyers, paralegals, legal researchers, civic-tech devs, journalists, academic researchers
- Data profile: 3.7M+ ECLIs, ~20 years of decisions. Rich RDF/XML metadata: court (`creator`), publication & decision dates, subject area (`rechtsgebied`), procedure type, case number (`zaaknummer`), formal relations to related cases, citations (`vindplaatsen`), `inhoudsindicatie` (summary), full decision text
- Source of truth: `https://data.rechtspraak.nl/` (Atom feed for search, RDF/XML for content). Open government data under Dutch Open Data policy. No auth.

## Reachability Risk
- **None.** `printing-press probe-reachability` returns `mode: standard_http`, both stdlib and Surf-Chrome return 200 with `application/atom+xml`. Live samples of `/uitspraken/zoeken` and `/uitspraken/content` confirm the API is open and stable.
- Note: `www.rechtspraak.nl` (the marketing/portal site) does block plain WebFetch. The data API at `data.rechtspraak.nl` does not. CLI must hit `data.rechtspraak.nl` only.

## Top Workflows
1. **Recent-decision monitoring** — "what did Hoge Raad publish on tax law this week?" (date range + court + subject filters)
2. **Decision lookup** — "fetch ECLI:NL:HR:2024:1, give me the summary and the full text"
3. **Appeal-chain navigation** — "show me the trial-court decision and cassation chain for this ECLI"
4. **Case-file (dossier) tracking** — "all decisions for zaaknummer 22/00155"
5. **Iterative narrowing of an over-broad result set** — "of those 12,000 bestuursrecht decisions, the ones mentioning 'huurprijswijziging' but not 'kort geding'" — combine include/exclude/phrase/regex over the local FTS index
6. **Bulk research export** — pull a corpus of decisions for academic/NLP work

## Upstream filter shape (probed live)
- **API supports server-side:** date range (repeated `date=`), modified/publication range (repeated `modified=`), multi-court UNION (repeated `creator=`), multi-subject UNION (repeated `subject=`), `type=Uitspraak|Conclusie`, `replaces=ECLI`, `max`/`from` pagination, `return=META|DOC`.
- **API does NOT support:** free-text keyword search (`q`/`keyword`/`search`/`text` all return the full 3.7M corpus, parameter ignored), procedure filter (`procedure=` ignored despite being in the official vocab), zaaknummer filter, sort direction.
- **Implication:** every keyword/phrase/regex narrowing — and even procedure-type narrowing — MUST happen locally against the synced corpus. This makes a rich local-narrowing flag set (`--keyword`, `--exclude`, `--phrase`, `--regex`, `--procedure`) on `search` the defining capability of the CLI, not a nice-to-have. The competitor tools all expose the upstream filter set verbatim; none expose local narrowing as a composable language.

## Data Layer
- **Primary entities:**
  - `uitspraak` (decision) — ECLI, court, dates, type, subject, procedure, case number, summary, full text, relations
  - `instantie` (court) — code, full name, parent body (e.g., Rechtbank vs Hof vs Hoge Raad)
  - `rechtsgebied` (subject area) — hierarchical (bestuursrecht > belastingrecht)
  - `proceduresoort` (procedure type) — cassatie, hoger beroep, kort geding, etc.
  - `relation` (case-to-case link) — type=cassatie/conclusie/aanleg, points to another ECLI
  - `vindplaats` (citation) — where a decision is cited in legal journals
- **Sync cursor:** publication date (`date`) plus high-water ECLI per court. Atom feed supports `from=` pagination and `date=` exact-date filters; a full-corpus daily archive also exists per the official PDF.
- **FTS:** over decision titles + `inhoudsindicatie` + full `uitspraak` text. Dutch language - default tokenizer is fine; can layer Snowball Dutch stemmer later.

## Codebase Intelligence
- **basm92/rechtspraak_cli** — shell scripts. `get_rechtspraak_id.sh <sd> <ed> <civil-only> <out>` and `get_documents.sh <id-file> <out-dir>`. No caching, no search, no metadata enrichment. Closest competing CLI.
- **maastrichtlawtech/rechtspraak-extractor** (Python, PyPI) — `get_rechtspraak(max_ecli, sd, ed, save_file)` + `get_rechtstraak_metadata(method='api'|'sqlite', sqlite_db_path, fallback_to_api, multi_threading)`. They ship a pre-built SQLite (`lido.db`) with `ecli`, `document_type`, `date_decision`, `instance`, `full_text`. This is the closest local-cache pattern.
- **openstate/open-rechtspraak** — Python + PostgreSQL pipeline. `make import_verdicts --start_date --end_date`, two-step enrich pattern, also imports the `namenlijst` (judge name list). Server-grade, not CLI-grade.
- **digitalheir/rechtspraak-js** (npm `rechtspraak-nl`) — normalizes raw RDF/XML into clean JSON-LD; fixes known data-quality issues (URI encoding, untyped dates, case-number arrays). Useful intelligence: the upstream XML has real warts we should handle.
- **digitalheir/java-rechtspraak-library** — Java client. Same surface as the Python ones plus citation graph features.
- **Spijkervet/dutch_jurisdiction_elastic_search** — Elasticsearch indexer of the full archive. Proof that bulk-FTS-over-decisions is a real use case.
- **No MCP server exists for rechtspraak today** — clear seat at the table.

## Product Thesis
- Name: `rechtspraak-pp-cli` (binary: `rechtspraak-pp-cli`, slug: `rechtspraak`)
- Why it should exist:
  - Every existing tool is a *library* (Python, JS, Java) — you write code to use them. basm92's shell scripts are command-line but don't cache, don't search, and don't enrich. No tool gives you `rechtspraak get ECLI:... --json` or `rechtspraak search "huurprijs" --court HR --since 2024-01-01`.
  - No MCP server exists, so AI agents reach for the API the hard way (raw XML parsing).
  - Pairs with `tenderned` and `pdok-location` — same Dutch open-data stack, same offline-first, agent-native shape. Civic-tech researchers and AI legal-research workflows benefit directly.
  - Local SQLite + FTS over Dutch decision summaries is a real differentiator the community has only partially solved (rechtspraak-extractor ships a pre-built DB but no query CLI).

## Build Priorities
1. **Foundation (P0):** Atom feed client + RDF content fetcher; local SQLite for `uitspraken`, `instanties`, `rechtsgebieden`, `proceduresoorten`, `relations`; sync command with date-window incremental mode; FTS over title + `inhoudsindicatie`.
2. **Absorbed (P1):** search by date / court / subject / procedure / type, get by ECLI, content download, all vocab list commands, ECLI parser, dossier (by zaaknummer), JSON / CSV / `--select` output.
3. **Transcendence (P2):** `chain <ECLI>` (walk appeal/cassation graph), `citations <ECLI>` (extract `vindplaatsen`), `watch --court HR --subject strafrecht --since 7d` (incremental poll), `drift` (snapshot diff), `code <court-code>` (offline court-code dictionary).
