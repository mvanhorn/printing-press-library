# NCBI Entrez CLI Brief

## API Identity
- Domain: Biomedical literature and biological databases (PubMed, Gene, Protein, Nucleotide, Taxonomy, etc.)
- Users: Researchers, bioinformaticians, data scientists, pharma analysts, academic labs
- Data profile: 38+ interconnected databases accessed through 9 E-utility endpoints. High-gravity entities: PubMed articles, gene records, protein sequences, taxonomy nodes, SRA experiments

## Reachability Risk
- None. NCBI E-utilities are public, well-maintained US government infrastructure. No auth required for basic access. Optional API key raises rate limit from 3 req/s to 10 req/s. Endpoints have been stable since 2005 with only additive changes.

## Top Workflows
1. **Literature search**: Search PubMed for papers by keyword, author, date, MeSH term -> fetch abstracts or full XML
2. **Gene lookup**: Search gene database by symbol/name -> fetch gene summaries, linked proteins, pathways
3. **Sequence retrieval**: Fetch nucleotide/protein sequences in FASTA format by accession
4. **Cross-database linking**: Find related records across databases (e.g., gene -> protein -> structure -> pubmed)
5. **Citation matching**: Match partial citation strings to PMIDs for bibliography management

## Table Stakes
- All 9 E-utilities: EInfo, ESearch, EPost, ESummary, EFetch, ELink, EGQuery, ESpell, ECitMatch
- Support for all 38+ Entrez databases
- JSON and XML output
- History server support (WebEnv/query_key for large result sets)
- Pagination (retstart/retmax)
- Date filtering (datetype, reldate, mindate, maxdate)
- Rate limiting compliance (3 req/s default, 10 req/s with API key)
- FASTA/GenBank/GenPept format output for sequence databases

## Data Layer
- Primary entities: PubMed articles (PMID, title, authors, abstract, MeSH, journal, date), Gene records (GeneID, symbol, description, organism, aliases), Protein records (accession, description, sequence), Taxonomy nodes (TaxID, scientific name, lineage)
- Sync cursor: Date-based (mindate/maxdate) for incremental sync of search results
- FTS/search: Full-text search over synced PubMed abstracts, gene descriptions, taxonomy names

## Codebase Intelligence
- Source: biogo/ncbi Go package (github.com/biogo/ncbi/entrez)
- Auth: Optional API key via `api_key` query parameter or `NCBI_API_KEY` env var
- Data model: Search -> IdList -> Summary/Fetch pattern; History server for large batches
- Rate limiting: Built-in limiter at 3 req/s, configurable with API key
- Architecture: Clean function-per-endpoint design (DoSearch, DoPost, DoSummary, Fetch, DoLink, DoInfo, DoGlobal, DoSpell, DoCitMatch)

## Product Thesis
- Name: NCBI Entrez CLI
- Why it should exist: EDirect (the official NCBI CLI) is Perl-based, requires complex installation (downloading a tarball + running setup.sh), uses a pipe-only workflow with no structured output, has no local storage, and is hostile to modern agent workflows. Biopython's Bio.Entrez requires a Python runtime and heavy dependency. There is no modern Go CLI with SQLite-backed local storage, offline search, JSON/CSV output, and agent-native features. A printing-press CLI would be the first tool that lets researchers cache, search, and compose queries offline.

## Build Priorities
1. All 9 E-utilities as first-class commands with --json, --xml, --csv output
2. SQLite-backed local store for PubMed articles, genes, proteins, taxonomy
3. Offline FTS5 search across synced data
4. Cross-database link traversal as composable commands
5. Batch operations (bulk fetch, bulk citation match)
6. Novel features: citation graph, author network, MeSH term explorer, gene-to-literature bridge
