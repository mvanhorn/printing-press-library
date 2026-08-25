# Extron Literature Browser-Sniff Report

## 1. User Goal Flow
- Goal: "Browse and download Extron spec sheets and user manuals" (user-selected source: https://www.extron.com/technology/literature.aspx?tabid=5&defaultLang=true)
- Steps completed:
  1. Fetch literature index page (tabid=5, defaultLang=true) — found tab structure, language selects, alphabetical index, per-category file tables
  2. Discover download URL pattern `/download/files/<category>/<filename>.pdf`
  3. Probe alphabetical listing `?tabid=5&defaultLang=true&id=<letter>` — returns per-category tables (Description, Rev, Date, Size, Type)
  4. Verify PDF download over plain HTTP — 200, application/pdf, valid content
- Coverage: primary flow complete; secondary flows (product pages, per-product spec sheets) not needed for the chosen surface

## 2. Pages & Interactions
- https://www.extron.com/technology/literature.aspx?tabid=5&defaultLang=true — literature index (no interaction, direct fetch)
- https://www.extron.com/technology/literature.aspx?tabid=5&defaultLang=true&id=M — alphabetical listing for M (English)
- https://www.extron.com/download/files/brochure/medialink_plus_series_REV_C1_cn_ONLINE.pdf — PDF download test
- Direct HTTP only. curl is blocked by the WAF (connection reset); Go stdlib HTTP/1.1 with Chrome UA + Accept headers succeeds. Probe-reachability: `standard_http` (stdlib 200, surf 200).

## 3. Browser-Sniff Configuration
- Backend: direct HTTP (no browser automation needed — surface is server-rendered)
- Runtime: `standard_http` (no browser transport, no clearance cookies needed)
- Pacing: N/A (only a handful of requests)

## 4. Endpoints Discovered
| Method | Path | Status | Content-Type | Auth |
|--------|------|--------|--------------|------|
| GET | /technology/literature.aspx?tabid=5&defaultLang=true&id=All | 200 | text/html | public |
| GET | /technology/literature.aspx?tabid=5&defaultLang=true&id={A-Z,0-9} | 200 | text/html | public |
| GET | /download/files/{category}/{filename}.pdf | 200 | application/pdf | public |
| POST | /api/v2/search/suggest/?keyword=... | reset | n/a | WAF-gated for non-browser clients; not part of the printed surface |

## 5. Traffic Analysis
- Protocols: rest_html (server-rendered ASPX pages with embedded data tables), binary PDF downloads
- Auth signals: none (public site)
- Parameter evidence: `id` = alphabetical letter filter (values: All, 0-9, A-Z); `tabid` = page tab (5 = literature, defaultLang=true = English); `lang` = language code (1 = English)
- Protection signals: WAF resets non-browser POSTs and some GETs to /api/* paths; page GETs with browser-like headers succeed
- Generation hints: `response_format: html` for the literature index; `response_format: binary` for PDF downloads
- Candidate commands: `literature list`, `literature search`, `literature download`, `categories`

## 6. Coverage Analysis
- Resource types exercised: literature index (all categories), PDF binaries
- Missed: per-product spec-sheet pages, search API (WAF-gated) — the alphabetical index covers discovery; noted for the CLI's search to filter locally

## 7. Response Samples
- Literature row (from id=M page): `<a id="...idFileUrl" href="/download/files/brochure/Matrix50SeriesBebro.pdf" target="download">Matrix 50</a>` with Rev/Date/Size/Type columns
- Category headings: "Brochure (M - 22 files)", "Declaration of Conformity (M - 109 files)", "Design Guide (M - 0 files)", "Product Guide", "Manual", "Revit BIM files"
- PDF: binary, valid `PDF document, version 1.5`

## 8. Rate Limiting Events
- None observed (few requests). WAF connection resets on /api/* POST and intermittent page GETs — the printed CLI should retry once on connection reset (observed: retries succeed).

## 9. Authentication Context
- No authenticated session used. The literature library is public.

## 10. Bundle Extraction
- Not run (server-rendered surface; no SPA bundle needed). main.js was inspected only to locate the search-suggest endpoint and confirm the surface shape.
