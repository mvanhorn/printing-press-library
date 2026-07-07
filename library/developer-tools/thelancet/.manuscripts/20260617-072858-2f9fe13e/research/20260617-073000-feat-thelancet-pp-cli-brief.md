# The Lancet CLI Brief

## API Identity
- Domain: Medical research publishing (weekly journal, 20+ specialty titles)
- Users: Medical researchers, clinicians, literature review teams, citation-driven workflows
- Data profile: Articles as versioned records (DOI-keyed), highly citeable, author/affiliation-rich, temporal publication patterns

## Reachability Risk
- [High] Direct HTTP to thelancet.com returns 403 with explicit bot-detection headers; robots.txt blocks `anthropic-ai` and `ClaudeBot` by name
- Reachability signal: 6+ GitHub issues on pubmed-cli mentioning blocked/403 errors on journal sites since 2024
- Likely constraint: Elsevier (publisher) actively enforces user-agent restrictions on web access; API requires credentials
- Probe-safe endpoint: CrossRef API is public; no auth required; safe to test

## Top Workflows
1. **Search by keyword/author** — find articles matching a research question; filter by date, impact, open-access status
2. **Browse current issue** — see what's in the latest/upcoming Lancet issue; drill into article metadata
3. **Track citations** — given a DOI, find citing and cited papers within The Lancet ecosystem
4. **Author workflow** — find all articles by a specific author across Lancet titles; track their publication history
5. **Journal discovery** — compare impact/scope of Lancet's 20+ specialty titles; find the right journal for a submission

## Table Stakes
- **pubmed-cli** (reference incumbent): `search` (keyword + author), `fetch` (by PMID/DOI), `cited-by`, `references`, `related`, `mesh` (MeSH indexing), `refcheck` (batch DOI validation). Shows that medical-journal CLIs are citation-graph and structured-metadata driven, not full-text.
- **CrossRef API baseline**: metadata (title, DOI, authors, pub date, citation count, abstract when available) is freely queryable; no official Lancet API exists for public use
- **Competing surfaces**: PubMed itself (free), journal websites (bot-blocked), Elsevier's ScienceDirect (paid), bioRxiv/medRxiv (preprints, overlapping audience)
- **Must-have for Lancet credibility**: author-affiliation search, journal-family awareness (Lancet vs Lancet Oncology vs Lancet Psychiatry), open-access indicator

## Data Layer
- Primary entities: `Article` (DOI-keyed, title, authors, date, journal, citationCount), `Author` (name, affiliations, article count), `Issue` (journal + publication date + article list)
- Sync cursor: CrossRef API provides per-journal updates; timestamp-driven incremental fetch
- FTS/search: title, author name, abstract (when available); author affiliation field for institutional searches
- Local store use case: build author network (who cites whom), trend analysis (publication velocity by topic), offline bibliography export

## Reachability Path
- **Primary: CrossRef API** (public, no auth, rich metadata, Lancet ISSN: 0140-6736)
- **Secondary: Browser-sniff** (if approved in Phase 1.7): discover URL patterns, article metadata pages, journal category pages. Lancet website may serve article landing pages that are HTML-parseable when accessed anonymously (no login wall detected on public URLs); extraction point: article title, author list, abstract preview, citation count
- **Tertiary: RSS feeds** (documented, blocked for bots; requires human browser or feed aggregator workaround)
- **Economics check**: Primary (CrossRef) is completely free. Secondary (browser-sniff) adds replayable HTML extraction; no credentials needed for public pages. No paid tier required for headline features.

## Product Thesis
- **Name**: `lancet-pp-cli` (or `medical-journal-cli` if expanding to other Elsevier journals later)
- **Why it exists**: 
  - Medical researchers spend hours manually searching journals, tracking citations, and building reading lists. CrossRef + browser-extracted metadata makes that searchable offline without journal paywalls.
  - No existing free CLI exists that treats Lancet's family of journals as a cohesive corpus with cross-journal discovery.
  - Author-affiliation search unlocks institutional use (universities discovering who from their org publishes in Lancet, for grant tracking and collaboration discovery).

## Build Priorities
1. **Data layer** — CrossRef sync for all Lancet ISSN titles + local FTS on title/author/abstract
2. **Core search** — `search --keyword`, `search --author`, `search --journal`, `search --since` (date filtering)
3. **Article detail** — `article get <DOI>` returns title, authors, abstract, citations, link to full text
4. **Citations** — `article cited-by <DOI>`, `article references <DOI>` (citation graph within Lancet)
5. **Offline author network** — `author <name>` lists all their Lancet publications; `author trending <affiliation>` shows institution's publication velocity

## Data-Source Pivot (2026-06-17, Phase 2/3 boundary)
Empirical field-coverage check during Phase 3 found CrossRef carries **zero author
affiliation data** for The Lancet (0/39 authors in a 20-article sample; matches the
journal-level "affiliations: zero coverage" stat). This breaks 4 of 6 approved
transcendence features (mesh, affiliation-growth, and the institution dimension of
rank-authors/visibility-gap).

OpenAlex (https://api.openalex.org, free, no auth) carries the missing data:
61% of authorships have institutions, 100% of works have a primary topic, citation
counts present, 474k Lancet works. User approved switching the primary data source to
OpenAlex; the full 24-feature manifest stays intact. Lesson for retro: Phase 1.9
should validate field coverage for data-layer-critical fields, not just reachability.
