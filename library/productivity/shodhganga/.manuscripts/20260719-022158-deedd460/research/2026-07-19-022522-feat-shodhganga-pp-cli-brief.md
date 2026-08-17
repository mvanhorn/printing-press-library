# Shodhganga CLI Brief

## API Identity
- Domain: INFLIBNET Shodhganga — India's national reservoir of PhD/MPhil theses (Electronic Theses & Dissertations). 600,000+ theses from 900+ universities.
- Platform: **DSpace 5.3** (JSPUI). Handles namespaced under `10603/`.
- Users: PhD scholars, research-guides, librarians, literature-review researchers, bibliometric analysts, RAG/agent pipelines needing Indian doctoral-research metadata.
- Data profile: per-thesis Dublin Core metadata — title, researcher (DC.creator), guide/supervisor (DC.contributor), university + department + place (DC.publisher x3), subjects/keywords (DC.subject x N), completed date (DC.date), abstract (DCTERMS.abstract), language, type, handle URI. Full-text PDFs are login-gated (out of scope).

## Reachability Risk
- **None.** All target surfaces return HTTP 200 over plain HTTPS, no bot protection, no auth, no rate-limit signals observed.
- Probe-safe endpoints used: `GET /simple-search?query=physics` (200), `GET /handle/10603/305247` (200, full DC meta), `GET /browse?type=author` (200).
- Runtime: `standard_http`. No Surf/browser/clearance needed.

## Working vs Dead Surfaces (probed 2026-07-19)
| Surface | Result |
|---|---|
| `/oai/request` (OAI-PMH) | 404 — disabled |
| `/rest`, `/server/api` (DSpace REST) | 404 — not enabled |
| `/open-search` feed | 400 — broken (description doc 200 but feed disabled) |
| `/simple-search?query=&rpp=&start=` | ✓ 200 HTML, `<a href="/handle/10603/ID">Title</a>` rows, "Results 1-5 of 137604" |
| `/browse?type=title\|author\|keyword\|subject\|dateissued` | ✓ 200 HTML |
| `/handle/10603/<id>` (item detail) | ✓ 200 HTML, full `<meta name="DC.*">` tags |
| `/community-list` (universities) | community/collection handle pages 200 |

**Consequence:** no JSON API. The CLI is an HTML-extraction CLI over direct HTTP:
search/browse → handles → per-handle DC metadata parse → local SQLite store.

## Top Workflows
1. Search theses by topic/keyword and get structured, agent-parseable metadata (web UI is HTML-only; agents can't consume it).
2. Pull the full metadata record for a known handle/URI (researcher, guide, university, keywords, abstract, year).
3. Find every thesis supervised by a given research guide, or produced by a given university/department.
4. Build an offline corpus of a research area (sync N results) then run offline FTS / faceted analysis without hammering the site.
5. Discover related theses by shared subjects (literature-review expansion).

## Table Stakes (from the Shodhganga web UI)
- Keyword search with paging (rpp/start).
- Browse by title, author, subject/keyword, date.
- Item detail with full Dublin Core metadata.
- Community/collection (university/department) navigation.
- Result total counts.

## Data Layer
- Primary entity: **Thesis** (handle, title, researcher, guide[], university, department, place, subjects[], completed_date, abstract, language, type, uri).
- Secondary: **University/Community** (handle, name).
- Sync cursor: search-query or browse-facet + `start` offset paging.
- FTS/search: offline FTS over title/researcher/guide/subjects/abstract.

## Why install this instead of the web UI
- The web UI returns HTML only — no JSON, no scripting, no agent consumption. This CLI turns every thesis into a structured record with `--json`/`--select`.
- Guide-centric and university-centric views that the UI buries or doesn't offer cleanly.
- A local SQLite mirror: sync a research area once, then search/filter/aggregate offline and instantly.
- Agent-native: an MCP-exposed, typed surface over 600k Indian doctoral theses.

## Product Thesis
- Name: **shodhganga-pp-cli** ("Shodhganga CLI") — structured, scriptable, agent-native access to India's national thesis reservoir.
- Why it should exist: Shodhganga is the authoritative source for Indian doctoral research but exposes zero machine API. This CLI is the missing programmatic layer: search, extract Dublin Core metadata, persist locally, and analyze — for researchers, librarians, and AI agents doing literature review.

## Build Priorities
1. DSpace HTML client (search, browse, item-detail DC-meta parse) + local Thesis store + sync.
2. Absorbed table-stakes: search, browse-by-facet, item detail with full metadata, university navigation, `--json`/`--select`/`--csv`.
3. Transcendence: guide-centric index, university stats, subject/year trends, similar-theses, offline corpus analytics.
