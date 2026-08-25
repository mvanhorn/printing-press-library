# NEJM CLI — Absorb Manifest

## Landscape (Step 1.5a)
No dedicated NEJM CLI or MCP exists. Adjacent tools an agent/clinician would otherwise reach for:
- **PubMed / medical MCP servers** (openpharma-org/pubmed-mcp, JamesANZ/medical-mcp, Cicatriiz/healthcare-mcp-public): search medical journals, filter by journal=NEJM, fetch metadata by PMID/DOI. NEJM is one of many journals; no NEJM-native issue/specialty experience.
- **RSS readers**: consume NEJM eTOC + recently-published feeds. List-only, no query/store.
- **Reference managers** (Zotero/Mendeley): import citation by DOI.
- **Crossref** (api.crossref.org, ISSN 0028-4793): DOI → metadata.
- **NEJM website/app**: current issue, browse by specialty, abstracts, free-article badges — behind Cloudflare SPA, not scriptable.

Data reality driving dispositions:
- **RSS feeds** (plain HTTP) carry bibliographic data: title, authors (`dc:creator`), DOI, volume/issue/pages, dates. No abstract, no specialty, no type, no free-flag.
- **Article detail page** (Surf) carries abstract, `articleType`, `Specialties`, `topics`, `isFree`. Enrichment = one detail fetch per DOI.
- Sync strategy: `sync` (fast, bibliographic from RSS) + `sync --enrich` / on-demand `article get` (abstract/specialty/type/free from detail). Features needing taxonomy/abstract degrade gracefully until enriched.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Current issue / TOC | RSS readers, NEJM "current issue" | nejm-pp-cli current | Offline, --json, authors+pages, free/paywalled flag when enriched |
| 2 | Recently published | RSS readers, NEJM "latest" | nejm-pp-cli recent | Offline, filterable, agent-native |
| 3 | Article metadata + abstract by DOI | PubMed MCP, Crossref, Zotero | (generated endpoint) article get | Surf meta extraction: title, authors, abstract, type, specialties, isFree |
| 4 | Search NEJM literature | PubMed MCP search-medical-journals | nejm-pp-cli search | Offline FTS over synced corpus (live NEJM search is Cloudflare-blocked); regex/SQL-composable |
| 5 | List specialties | NEJM specialty nav | (generated endpoint) specialty list | HTML link extraction of the 14 specialties |
| 6 | Filter by specialty | NEJM specialty browse | (behavior in nejm-pp-cli articles --specialty) | Offline filter over enriched corpus |
| 7 | Filter by article type | NEJM article-type facets | (behavior in nejm-pp-cli articles --type) | Offline filter (Original Article, Review, Case Report, Editorial, Perspective…) |
| 8 | Find by author | RSS dc:creator, PubMed author search | (behavior in nejm-pp-cli articles --author) | Offline author match over synced corpus |
| 9 | Sync corpus to local store | RSS reader refresh | nejm-pp-cli sync | Idempotent DOI upsert; --enrich pulls detail metadata |
| 10 | SQL over corpus | (none) | nejm-pp-cli sql | Arbitrary SELECT over local SQLite |
| 11 | Citation export by DOI | Zotero/Mendeley/reference managers | nejm-pp-cli article cite | BibTeX + RIS from local article record (offline) |
| 12 | Health check / reachability | (none) | nejm-pp-cli doctor | Surf transport + corpus freshness report |

Every absorbed row ships with --json, --select, typed exit codes, and SQLite persistence where applicable.

## Transcendence (only possible with a local NEJM corpus + NEJM-native taxonomy)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Time-windowed new articles | since 48h | hand-code | Requires local first-seen timestamps; no NEJM API offers a "what's new since" window | Use for "what did NEJM publish since I last looked". Do NOT use for a fixed issue; use 'current'. |
| 2 | Current-issue reading digest | digest | hand-code | Requires local join of issue articles grouped by specialty/type with abstracts; the site has no digest view | Use to triage the current issue at a glance. Groups by specialty/type when enriched, else by issue. |
| 3 | Personal reading list | reading-list add/ls/read/rm | hand-code | Requires local mutable per-user state the logged-out website lacks | Use to queue DOIs and track read/unread locally. |
| 4 | Specialty/type trends | trends | hand-code | Requires aggregation across the synced corpus (and across repeated syncs over time) | Use to see the distribution of NEJM output by specialty/type/issue. |
| 5 | Open-access finder | open-access | hand-code | Requires enriched isFree flags filtered locally; the site buries free full-text | Use to surface free full-text NEJM articles for unsubscribed readers, newest first. |

Minimum 5 transcendence features met. All hand-code, all leverage the local SQLite corpus.

## Stubs
None. All rows above are shipping scope.
