# The Lancet CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

Reconciled to the actual OpenAlex-backed command surface after the data-source
pivot (see brief). Dispositions: `(generated endpoint)` = generator-emitted typed
command; `(behavior in ...)` = flag/mode/output behavior inside a resolving
command; `(stub)` = intentionally not built, reason given.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search articles by keyword | pubmed-cli search, OpenAlex | thelancet-pp-cli works search | Scoped to the Lancet family by default; offline-capable, agent-native output |
| 2 | Search articles by author | pubmed-cli search, OpenAlex | (generated endpoint) authors search | Author disambiguation via OpenAlex; finds all Lancet publications by a person |
| 3 | Fetch article by DOI/ID | pubmed-cli fetch, OpenAlex | (generated endpoint) works get | Structured metadata (title, authors, institutions, topic, citations, links) |
| 4 | Fetch article by PMID | pubmed-cli fetch (PubMed-specific) | (stub) OpenAlex keys works by OpenAlex ID or DOI, not PMID; use works get with a DOI | DOI is the primary identifier in OpenAlex |
| 5 | Find citing papers | pubmed-cli cited-by | thelancet-pp-cli cited-by | Resolves DOI/ID then lists citing works via OpenAlex cites: filter |
| 6 | Find cited references | pubmed-cli references | (behavior in thelancet-pp-cli works get) referenced_works[] is in the work payload (use --select referenced_works) | Reference list straight from the work record |
| 7 | Related articles | pubmed-cli related | (behavior in thelancet-pp-cli works get) related_works[] is in the work payload | OpenAlex-computed relatedness, no extra call |
| 8 | Topic classification | pubmed-cli mesh | (behavior in thelancet-pp-cli works get) primary_topic in payload; also powers drift and curate | OpenAlex topic taxonomy replaces MeSH for Lancet |
| 9 | Batch DOI validation | pubmed-cli refcheck | (stub) not built in v1; single-record resolution via works get | Deferred; OpenAlex resolves one DOI/ID per works get call |
| 10 | Browse journals | manual navigation | thelancet-pp-cli journals | Lists the curated Lancet family (slug, ISSN, name) for scoping |
| 11 | Current / latest articles | RSS feeds (blocked) | (behavior in thelancet-pp-cli works search) --sort publication_date:desc returns the most recent Lancet articles | No bot-blocked RSS; works against OpenAlex |
| 12 | Publication date filtering | pubmed-cli implicit | (behavior in thelancet-pp-cli works search) --filter from_publication_date:...,to_publication_date:... | Temporal filtering via OpenAlex filter syntax |
| 13 | Open access filtering | pubmed-cli implicit | (behavior in thelancet-pp-cli works search) --filter is_oa:true (also curate --open-access) | Filter to open-access works |
| 14 | Pagination/limits | pubmed-cli implicit | (behavior in thelancet-pp-cli works search) --per-page / --page / --all | Cursor/page pagination through large result sets |
| 15 | JSON output | pubmed-cli implicit | (behavior in thelancet-pp-cli works search) --json | Structured machine-readable output for agents |
| 16 | CSV export | pubmed-cli implicit | (behavior in thelancet-pp-cli works search) --csv | Bulk export for spreadsheets/downstream analysis |
| 17 | Dry-run support | n/a | (behavior in thelancet-pp-cli works search) --dry-run | Shows the request without sending (verify-friendly) |
| 18 | Field selection | n/a | (behavior in thelancet-pp-cli works search) --select results.title,results.cited_by_count | Narrow deeply-nested OpenAlex payloads; reduces agent context |
| 19 | Journal metadata | OpenAlex sources | (generated endpoint) sources get | Per-journal works_count, citations, host org via ISSN |
| 20 | Local analytics store | (novel) | thelancet-pp-cli refresh | Syncs Lancet works into local SQLite for the analytics commands |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Lab citation impact rank | rank-authors --institution "<name>" --journal "<title>" --trend | hand-code | Requires full author-citation graph pre-materialized locally. External APIs serve single-article queries; local store enables instant cross-tabs by journal and impact factor. | Rank researchers at your institution by citation impact within a specific Lancet journal or topic. Shows who is dominating this field at your org. Do NOT use this for hiring alone; use alongside peer evaluation. |
| 2 | Institutional citation mesh | mesh --org "<name>" --department "<dept>" --theme "<keyword>" | hand-code | Requires author identity resolution + cross-referencing citations against local article store. Impossible without pre-synced author deduplication and citation graph. | Find researchers at your institution who cite each other's work; quantify co-citation frequency; visualize how research themes overlap between departments. |
| 3 | Affiliation growth tracking | affiliation-growth --journal "<title>" --years 5 --threshold 5 | hand-code | Requires historical snapshots per article stored in SQLite. Time-series institutional growth trends are invisible to point-in-time API queries. | Track which institutions published zero articles in a Lancet journal 3+ years ago but now publish 5+ articles/year. Surface rising research centers in your competitive space. |
| 4 | Editorial drift detector | drift --journal "<title>" --window1 "2019-01-01:2020-12-31" --window2 "2023-01-01:2024-12-31" --top-n 10 | hand-code | Requires stored historical catalogs of journal articles. No API serves "all articles in journal X from 2019-2020" for comparative analysis. | Compare publication distribution across topics between two time windows. Identify emerging specialties (publishing more) and fading ones (publishing less) in any Lancet journal. |
| 5 | Reading list curation | curate --topic "<string>" --sort date\|citations\|impact --output markdown\|bibtex | hand-code | Fast local ranking and filtering on 20k+ articles with multi-dimensional sort. No external API can do this without multiple round-trips and aggregation. | Select research topic/keywords; CLI generates ranked reading list by publication date, citation count, open-access status. Export as Markdown or BibTeX. Librarian workflow enabler. |
| 6 | Funding visibility gap | visibility-gap --metric citations-vs-prestige --institution "<name>" | hand-code | Requires cross-tabulation of journal prestige proxy (or impact factor) with author citation counts plus Lancet-internal citation frequency. Three aggregates impossible as a single API call. | Find authors publishing in lower-tier Lancet journals but getting high citation counts, or vice versa. Surface mis-matched authors for internal promotion or collaboration opportunities. |

## Approved Stubs

(None identified at this stage. All transcendence features were approved and are planned for full implementation.)
