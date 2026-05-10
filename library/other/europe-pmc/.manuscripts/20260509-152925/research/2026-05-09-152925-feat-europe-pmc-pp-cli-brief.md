# Europe PMC CLI Brief

## API Identity
- Domain: Biomedical literature aggregator — 39M+ publications from PubMed, PMC, preprints (bioRxiv/medRxiv), European patents, Agricola, UK theses, clinical trials, NCBI Bookshelf
- Users: Systematic reviewers, bioinformaticians, grant program officers, patent analysts, pharmacovigilance teams, preprint watchers
- Data profile: 10 data sources (MED, PMC, PPR, PAT, AGR, ETH, CBA, HIR, CTX, NBK), 143 searchable fields, text-mined annotations (genes, diseases, chemicals, organisms), citation/reference graphs, full-text JATS XML

## Reachability Risk
- None. Fully public API, no auth required, CORS enabled. Official Swagger spec at /api/swagger.json.

## Top Workflows
1. Literature search across all sources with MeSH synonym expansion
2. Citation/reference graph traversal for systematic reviews
3. Preprint-to-publication tracking (PPR source)
4. Text-mined annotation extraction (gene-disease-chemical co-occurrence)
5. Grant-linked publication impact analysis

## Data Layer
- Primary entities: Articles (id, source, pmid, pmcid, doi, title, authors, abstract, citations, grants), Annotations (entity, type, section, provider), Citations/References (edges), Database cross-links
- Sync cursor: cursorMark-based pagination for search, page-based for citations/references
- FTS/search: Full-text search over synced abstracts and titles

## Product Thesis
- Name: Europe PMC CLI
- Why it should exist: Europe PMC covers sources NCBI Entrez doesn't (preprints, European patents, Agricola, UK theses). The R package (europepmc) is the only structured wrapper and requires R. No Go CLI exists. The annotation API enables text-mined knowledge graphs that no other literature tool exposes via command line. Combined with SQLite, this becomes the first CLI for preprint tracking, patent-literature bridging, and section-level full-text mining.

## Build Priorities
1. All search/article/profile/fields endpoints as first-class commands
2. Citation and reference graph commands
3. Annotation API integration (5 endpoints)
4. Database links, data links, labs links
5. Full-text XML and supplementary file access
6. Novel features: preprint tracker, citation graph, annotation miner, systematic review workbench
