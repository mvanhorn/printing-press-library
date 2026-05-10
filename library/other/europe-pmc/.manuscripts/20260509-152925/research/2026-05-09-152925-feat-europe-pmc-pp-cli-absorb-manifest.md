# Europe PMC CLI — Absorb Manifest

## Absorbed (20 features from europepmc R package, pyEuropePMC, API docs)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Article search with field operators | europepmc R / pyEuropePMC | `epmc articles query --query "term"` | --json/--csv, cursor pagination, MeSH synonym expansion |
| 2 | Article lookup by source+ID | europepmc R | `epmc articles lookup --source MED --id 12345` | Multi-format output, --compact for agents |
| 3 | Profile/breakdown by source | europepmc R epmc_profile | `epmc profile breakdown --query "term"` | Source + pub_type breakdown in one call |
| 4 | List searchable fields | API docs | `epmc fields list` | 143 fields with descriptions |
| 5 | Citations (who cites this) | europepmc R epmc_citations | `epmc citations list --source MED --id 12345` | Paginated, stored locally |
| 6 | References (what this cites) | europepmc R epmc_refs | `epmc references list --source MED --id 12345` | Paginated, stored locally |
| 7 | Database cross-links | europepmc R epmc_db | `epmc database-links list` | Filter by specific database |
| 8 | Data links (unified) | API docs | `epmc datalinks list` | Provenance tracking (text-mined vs submitted) |
| 9 | Labs links (external providers) | API docs | `epmc labs-links list` | Altmetric, BioStudies, etc. |
| 10 | Full-text JATS XML | europepmc R epmc_ftxt | `epmc fulltext get --id PMC7537588` | Download OA full text |
| 11 | Post-publication evaluations | API docs | `epmc evaluations list` | Peer review access |
| 12 | Annotations by article | Annotations API | `epmc annotations by-article --articleIds MED:123` | Text-mined genes, diseases, chemicals |
| 13 | Annotations by entity | Annotations API | `epmc annotations by-entity --entity BRCA1` | Find all papers mentioning an entity |
| 14 | Annotations by relationship | Annotations API | `epmc annotations by-relationship --firstEntity BRCA1 --secondEntity "breast cancer"` | Gene-disease co-occurrence |
| 15 | MeSH synonym expansion | europepmc R | `--synonym true` on search | Automatic query expansion |
| 16 | Result type control | All wrappers | `--resultType core` | idlist/lite/core detail levels |
| 17 | Sorting | pyEuropePMC | `--sort "CITED desc"` | Citation count, date, author |
| 18 | Local data sync | None | `epmc sync --full` | Incremental SQLite sync |
| 19 | Offline FTS search | None | `epmc local-search "term"` | FTS5 with BM25 |
| 20 | SQL queries | None | `epmc sql "SELECT ..."` | Direct SQL on synced data |

## Transcendence (10 features)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Preprint-to-Publication Tracker | `track-preprint` | 9.5 | Polls PPR source for published versions, stores lifecycle timestamps in SQLite |
| 2 | Annotation-Powered Relation Miner | `mine-relations` | 9.5 | Chains 3 annotation endpoints into a local gene-disease-chemical knowledge graph |
| 3 | Citation Network Builder | `cite-graph` | 9.0 | Recursive citations+references traversal stored as a graph in SQLite |
| 4 | Systematic Review Workbench | `systematic-review` | 9.0 | PRISMA workflow with cross-source MED+PMC+PPR deduplication |
| 5 | Full-Text Section Miner | `section-search` | 9.0 | Combines section-level field search with section-filtered annotations |
| 6 | Patent-Literature Bridge | `patent-lit` | 8.5 | Walks PAT references to MED/PMC, enriches with database links and annotations |
| 7 | Grant Impact Dashboard | `grant-impact` | 8.5 | Aggregates grant-linked publications with citation impact and OA compliance |
| 8 | Preprint Server Intelligence | `ppr-intel` | 8.5 | Monitors PPR source by topic, tracks citation velocity and time-to-publication |
| 9 | Cross-Database Entity Enrichment | `enrich` | 8.0 | Merges databaseLinks + datalinks + annotations into unified entity table |
| 10 | Canonical ID Resolver / Dedup | `dedup` | 8.0 | Resolves DOI/PMID/PMCID/PPR to canonical record, bulk deduplication |
