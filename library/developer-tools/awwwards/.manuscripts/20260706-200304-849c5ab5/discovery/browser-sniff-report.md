# Awwwards Browser-Sniff Discovery Report

Run: 20260706-200304-849c5ab5 · Backend: browser-use v0.13.3 (CDP → headless scratch-profile Chrome 150, port 9223) · Anonymous session

## 1. User Goal Flow

- **Goal:** Gather design context — browse best-rated sites, filter by tag/color/award, open a winner's detail page for scores/palette/tags (from user vision: "allow an agent to gather context about the best web designs in the world to use in its own designs").
- Steps completed:
  1. Load `/websites/` listing (31 cards, embedded card JSON confirmed)
  2. Install fetch+XHR interceptors, scroll ×6 → infinite scroll fired `GET /websites/?page=2..4` (full HTML pages, not fragments)
  3. Open first card → `/sites/monolog` detail page (SOTD)
  4. Scroll full detail page (10,820px) — **zero same-site XHR**; scores, jury notes, palette all in initial HTML
  5. Load `/collections/` → collection URLs are `/<username>/collections/<slug>/`
  6. Load `/directory/` → specialty/country/budget filter links
- Steps done via direct curl (equivalent surface, site is anonymous-friendly): award/tag/color/text filters, `/elements/` + `/elements/hero/`, `/sites/monolog/content` partial
- Coverage: 6 of 6 planned interactive steps + 8 supplementary curl probes.

## 2. Pages & Interactions

| # | URL | Interaction | Result |
|---|-----|------------|--------|
| 1 | /websites/ | load + interceptor install + 6× scroll | infinite scroll → GET /websites/?page=N (HTML) |
| 2 | /sites/monolog | load + interceptor + 5× scroll | no XHR; full SSR |
| 3 | /collections/ | load | collection links discovered |
| 4 | /directory/ | load | filter taxonomy discovered |

## 3. Browser-Sniff Configuration

- Backend: browser-use v0.13.3 heredoc/CDP mode against headless scratch Chrome (user's daily Chrome untouched)
- Pacing: 0.6–2s between operations; zero 429s observed
- Proxy pattern: **not detected** (no XHR API at all — pure SSR site)
- GraphQL BFF: not detected

## 4. Endpoints Discovered (all public, all HTTP 200 anonymous)

| Method | Path | Status | Content-Type | Notes |
|--------|------|--------|--------------|-------|
| GET | /websites/ | 200 | text/html | listing; ?page=N (30/page); ?text= search |
| GET | /websites/{filter}/ | 200 | text/html | one filter: award tier / category / tag / tech / country / %23HEX color / Font |
| GET | /websites/nominees/ | 200 | text/html | award-tier example |
| GET | /sites/{slug} | 200 | text/html | detail: scores, jury votes, palette, tags |
| GET | /sites/{slug}/content | 200 | text/html | lightweight partial (173KB vs 368KB) |
| GET | /elements/ | 200 | text/html | element-type index |
| GET | /elements/{type}/ | 200 | text/html | hero, footer, 404_page, ... same card JSON |
| GET | /collections/ | 200 | text/html | curated boards index |
| GET | /{username}/collections/{slug}/ | 200 | text/html | one board's site grid |
| GET | /directory/ | 200 | text/html | agencies/freelancers; /directory/{filter}/ |

## 5. Traffic Analysis

- Protocols: `html_scrape` (confidence 0.55) — correct; the whole site is server-rendered HTML
- Auth signals: none (anonymous browsing serves everything probed)
- **Reachability REPAIR:** analyzer emitted `browser_required` (0.9) citing "CAPTCHA marker" on all 14 entries. Root cause: literal substring `captcha` in the CSS class `.legal-recaptcha` (newsletter form's reCAPTCHA legal-notice styling). All 14 captured pages contain full real content (31 cards each, 370–600KB). `cli-printing-press probe-reachability https://www.awwwards.com/websites/` returned `standard_http` at 0.95 (stdlib 200 + surf-chrome 200). Repaired `reachability.mode` → `standard_http`, cleared `requires_page_context` / `requires_protected_client` hints and the `captcha` protection entry — all rooted in the same false positive.
- Parameter evidence: `?page=N` observed live as the infinite-scroll fetch; `?text=` verified by curl; `{filter}`/`{type}`/`{slug}` path segments verified by curl across award/tag/color/font/country values.
- Generation hints (post-repair): none — plain standard HTTP transport.

## 6. Coverage Analysis

Exercised: websites listing + pagination + filters + search, site detail + content partial, elements index + type listing, collections index + board detail, directory + filters. Not exercised (out of scope for the design-context vision or auth-gated): voting/submission (requires account), jobs, market, academy, blog, annual-awards, jury pages. Robots.txt notes: `/websites/?` query-crawls, `/elements/*`, and search endpoints are robots-disallowed though they serve 200; the CLI self-rate-limits and fetches interactively (not bulk crawling).

## 7. Response Samples

**Listing card (embedded JSON in `data-collectable-model-value` attribute, one per `<li class="js-collectable">`):**
```json
{"collectableIdentifier":"monolog","collectableImage":"submissions/2026/05/6a17...jpg",
 "collectableTitle":"MONOLOG","id":64965,"images":{"thumbnail":"submissions/2026/05/6a17...jpg"},
 "slug":"monolog","title":"MONOLOG","createdAt":1783296000,
 "tags":["Design Agencies","Clean","Flat Design","Typography","GSAP","Three.js","Webflow"],"type":"submission"}
```

**Detail page structures (SSR HTML):**
- Overall scores: `<div class="layout-overall__score"><strong>7.61 / 10</strong></div>` ×4 (Design/Usability/Creativity/Content) + `data-note="7.54"` progressbar
- Score dimension titles: `grid-score--titles` → Design, Usability, Creativity, Content, Overall
- Jury votes: `list-jury-notes__item` → juror name, profile href, country (`list-jury-notes__from`), per-dimension `grid-score__item` values (~18–24 jurors)
- Palette: `list-palette__item` with `style="background: #080807"` and `list-palette__name` "HEX #080807"; each swatch links to `/websites/%23<hex>/`
- Thumbnails: `https://assets.awwwards.com/awards/media/cache/thumb_440_330/<path>` (also thumb_880_660)

## 8. Rate Limiting Events

None. ~14 curl fetches at 0.6s pacing + ~10 browser operations — all 200.

## 9. Authentication Context

No authenticated session used. All target surfaces are public. Scratch Chrome profile was used (user's session untouched); scratch profile deleted with the headless Chrome process. No cookies or credentials appear in the capture (anonymous requests only; Set-Cookie response headers stripped at write time).

## 10. Bundle Extraction

Not run — the site is server-rendered HTML without an SPA bundle; the interactive browser-sniff plus curl probes covered the surface.

## Spec disposition

The analyzer's auto-spec (12 generic endpoints, preserved at `research/awwwards-browser-sniff-spec.auto.yaml`) was replaced by a hand-authored spec at `research/awwwards-browser-sniff-spec.yaml`: 6 resources, 10 endpoints, `response_format: html` with `links`/`page` extract modes, positional filter/slug params, `happy_args` fixtures, explicit `http_transport: standard`. The embedded card JSON, score grids, and palette blocks require a hand-written parser (Phase 3, `internal/awwwards/`) since they live in data attributes and class-structured HTML, not script-tag JSON.

**Runtime shape: standard direct HTTP + structured HTML extraction. No browser, no clearance, no auth.**
