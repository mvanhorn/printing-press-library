# NCBI Entrez CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | ESearch: text query against any database | EDirect `esearch` | `entrez search --db pubmed --query "term"` | --json/--csv output, --snapshot for drift detection, --tag for PRISMA |
| 2 | EFetch: download records in multiple formats | EDirect `efetch` | `entrez fetch --db pubmed --id 12345 --format abstract` | FASTA/XML/JSON/text, auto-index to local store, --limit pagination |
| 3 | ESummary: document summaries | EDirect `esummary` / Biopython Entrez.esummary | `entrez summary --db pubmed --id 12345` | --json, --select field filtering, --compact for agent use |
| 4 | ELink: cross-database linking | EDirect `elink` | `entrez link --from pubmed --to gene --id 12345` | --materialize to SQLite, composable with other commands |
| 5 | EInfo: database metadata | EDirect `einfo` | `entrez info [db]` | List all databases or show fields/links for a specific db, --json |
| 6 | EPost: upload UID list to History server | EDirect `epost` | `entrez post --db pubmed --id 1,2,3` | stdin support for piped UIDs, batch from file |
| 7 | EGQuery: counts across all databases | None (underused) | `entrez gquery --query "term"` | --json, --store for temporal tracking |
| 8 | ESpell: spelling suggestions | Biopython Entrez.espell | `entrez spell --db pubmed --query "brest cancr"` | --auto-expand, thesaurus persistence |
| 9 | ECitMatch: citation to PMID resolution | None (barely used) | `entrez cite-match --input refs.bib` | BibTeX/CSV input, batch resolution, retraction check |
| 10 | Date filtering | EDirect `efilter` / E-utils params | `entrez search ... --since 2024-01 --until 2025-12` | Human-readable dates, relative dates (--since 7d) |
| 11 | History server support | EDirect pipes, Biopython | WebEnv/query_key auto-managed internally | Transparent to user, used for large result sets |
| 12 | Pagination | All tools (retstart/retmax) | `entrez search ... --limit 100 --offset 0` | Auto-paginate with --all, streaming for large sets |
| 13 | FASTA sequence retrieval | EDirect `efetch -format fasta` | `entrez fetch --db protein --id NP_001 --format fasta` | Direct file output with --output, multi-ID batch |
| 14 | GenBank/GenPept format | EDirect `efetch -format gb/gp` | `entrez fetch --format genbank` | Multiple format support per database |
| 15 | API key support | EDirect env var, Biopython | `NCBI_API_KEY` env var or config.toml | Doctor checks key validity, shows rate limit tier |
| 16 | Rate limiting | biogo/ncbi (3 req/s built-in) | Adaptive limiter, 3 req/s default, 10 req/s with key | User-visible rate info, --rate-limit override |
| 17 | Local data sync | None (no existing tool has this) | `entrez sync --db pubmed --query "term"` | Incremental date-based sync to SQLite |
| 18 | Offline FTS search | None | `entrez local-search "term" --rank bm25` | FTS5 with proximity, regex, BM25 ranking |
| 19 | SQL queries | None | `entrez sql "SELECT * FROM pubmed WHERE ..."` | Direct SQL against synced data |
| 20 | Pipe-based composition | EDirect (core feature) | Unix pipe support + --json streaming | EDirect compatibility + structured output |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Citation Snowball Tracker | `snowball --seed PMIDs --depth N` | 10 | Recursive cited-by graph via ELink with persistent frontier tracking across sessions. EDirect has no state between runs. |
| 2 | Cross-Database Link Materializer | `link-map "query" --from db --through db,db` | 10 | Materializes the full ELink cross-database graph into SQLite JOINable tables. EDirect loses intermediate link state. |
| 3 | Temporal Publication Velocity | `watch "query" --interval weekly` | 9 | Time-series PubMed counts stored locally with sigma-based alerting. No tool tracks historical query counts. |
| 4 | PRISMA-Ready Deduplication | `prisma --strategies a,b,c` | 9 | Cross-strategy dedup with provenance tracking for systematic review PRISMA reporting. Currently done in Excel. |
| 5 | FTS5 Abstract Corpus Search | `local-search "proximity NEAR/3 query"` | 9 | Offline proximity search, regex, BM25 ranking over cached abstracts. PubMed search cannot do proximity queries. |
| 6 | Saved Search Drift Detector | `drift my-search --since date` | 9 | Symmetric set-difference across snapshots: new, removed, retracted PMIDs. PubMed Alerts only track additions. |
| 7 | Drug-Gene-Literature Triangle | `triangle --drug name` | 9 | Chains ESearch+ELink across PubChem/Gene/PubMed with "unseen" tracking. Three API hops stored as one local graph. |
| 8 | Batch CitMatch with Retraction Check | `cite-match --input refs.bib --check-retractions` | 8 | ECitMatch + EFetch retraction status in one batch. Two API calls per ref, prohibitively slow without caching. |
| 9 | Agent Pipeline Compositor | `pipeline create name "search | link | fetch"` | 8 | Named, replayable, resumable pipelines with SQLite as intermediate buffer. EDirect pipes are stateless and one-shot. |
| 10 | Multi-DB Count Heatmap | `gquery trend --query term --top-movers` | 7 | Historical EGQuery counts stored locally to detect which databases are gaining records for your topic. |
| 11 | ESpell Thesaurus with Audit Trail | `thesaurus list` / `thesaurus reject` | 7 | Persistent accept/reject of spelling suggestions builds a personal controlled vocabulary across sessions. |
| 12 | MeSH Hierarchy Explorer | `mesh explode "term" --diff` | 8 | Local MeSH tree storage with annual diff detection. Alerts when MeSH changes alter your saved search results. |
