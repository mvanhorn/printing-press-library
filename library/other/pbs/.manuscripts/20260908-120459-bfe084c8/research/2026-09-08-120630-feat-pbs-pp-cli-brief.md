# PBS (Pakistan Bureau of Statistics) CLI Brief

## API Identity
- Domain: www.pbs.gov.pk — Pakistan's national statistical agency. Official price statistics.
- Users: Pakistani macro/equity analysts, economists, journalists, policy researchers, food-security
  and cost-of-living researchers, anyone modelling Pakistani inflation at sub-national granularity.
- Data profile: NOT a JSON API. Two surfaces:
  (a) an HTML index page /price-statistics/ carrying TWO JavaScript literal arrays
      (`const data` = 148 weekly SPI releases; `cpidata1` = the CPI monthly index — separate array)
  (b) static .xlsx and text-layer .pdf files at UNGUESSABLE dated URLs.
  There is no XHR API, no query interface, no bulk download, no documented spec.

## Reachability Risk
- **None.** ~60 probes, every one HTTP 200 via plain Go net/http, 30-900ms, zero Cloudflare
  challenges. Confirmed with both the Go prober and macOS LibreSSL curl. No auth, no cookies,
  no JS execution needed: the whole 148-entry index ships inside the served HTML.
- Two fetches 2s apart were byte-identical (342,712 B, identical array hash) — the page is stable.
- Tier/permission hints from 4xx body: none (no 4xx on any content URL).

## Top Workflows
1. **Assemble the weekly city price panel.** Scrape the index, fetch every annexure, parse
   17 cities x 51 items x {MIN,AVG,MAX}, persist. 128,316 city-item-week observations.
2. **Track one item's price across cities and weeks.** "What has wheat flour done in Quetta vs
   Karachi since 2023?" Impossible today: PBS publishes each week as a standalone file.
3. **Measure cross-city dispersion.** Nobody publishes a Pakistani sub-national price-spread
   series. It falls straight out of the panel.
4. **Recompute a custom basket.** The item weight vector is republished EVERY week; a local store
   can re-weight and also track how PBS itself re-weights over time.
5. **Audit coverage honestly.** 18 of 165 weeks are absent from the index; 2 of 27 sampled
   annexure URLs are hard 404s. An analyst needs the gap map, not a silently short series.

## Table Stakes
- There is no incumbent CLI, no npm/PyPI wrapper, no MCP server, and no competing tool of any kind
  for PBS. Registry screen (504 entries, 22 categories, 2026-09-08): zero Pakistani price coverage;
  FRED and us-data cover US macro only. So "table stakes" is set by what a careful analyst does by
  hand today: download the xlsx, open Excel, read one week, retype numbers.
- Minimum bar to beat that: a full local panel, queryable offline, with honest coverage accounting.

## Data Layer
- Primary entities: `release` (kind, as_of, urls, format, sha256), `price` (release, surface, city,
  item, stat, value, value_state), `weight` (release, item, weight_lowest, weight_combined),
  `spi_index` (release, quintile, index, wow, yoy), `coverage` (release, state, http_status, rows).
- Sync cursor: the release as_of date, driven off the scraped index, NOT a constructed URL.
- FTS/search: item descriptions (the only stable join key — see traps).

## User Vision
Owner's framing: personal research. The bar is explicitly NOT "wrap an API" — it is "produce a
series, panel or join nobody else offers." PBS qualifies because the site DESTROYS ITS OWN HISTORY:
Wayback's earliest capture of any SPI upload is 20251022150101 while the live index reaches
2023-07-13, and PBS already 404'd its entire pre-2023 Drupal tree in a migration. A local accruing
store becomes the only long-run record. Owner selected PBS first, over SBP/NEPRA/PPRA/OGRA, on the
combination of uniqueness + reachability + the only affirmative written licence in the candidate set.

## Product Thesis
- Name: pbs-pp-cli — the Pakistani price-panel press.
- Why it should exist: PBS publishes the richest sub-national price panel in Pakistan one
  machine-hostile weekly file at a time, indexes it in a hand-edited JavaScript array with swapped
  keys and five date encodings, and has already lost months of its own CPI history. This CLI turns
  148 scattered files into one queryable 128,316-row panel, records every gap as a gap, and refuses
  to interpolate. It is the only way to ask a cross-city or cross-week question about Pakistani
  prices at all.

## Governance
- robots.txt VERBATIM: `# START YOAST BLOCK` / `User-agent: *` / `Disallow:` (empty = allow all) /
  `Sitemap: https://www.pbs.gov.pk/sitemap_index.xml`. Re-fetched and confirmed by me directly.
- NO AI-crawler directives of any kind. No ClaudeBot/GPTBot/CCBot rule, no Content-Signal.
  (Contrast: NEPRA, OGRA, PriceOye, Graana and SECP all carry `ClaudeBot Disallow: /` + ai-train=no.)
- Licence: /data-dissemination/ Pakistan Official Statistics & Data Dissemination Policy —
  "open by default"; aggregate statistics "shall normally be released free of charge under an open
  license, in accessible human-readable and machine-readable formats"; Tier 1/2 products carry
  "a standard open government licence permitting copying, adaptation and commercial/non-commercial
  reuse with attribution." The footer's boilerplate "(c) 2026 All Rights Reserved" is contradicted
  by the policy page. This is the ONLY candidate in the pool with affirmative written permission.

## Build Priorities
1. Index scraper (BOTH JS arrays) + release store + resumable sync with 3-state coverage.
2. Hand-written xlsx parser handling the THREE STACKED ROW-BLOCKS (see trap 1) and the
   distinct zero-vs-blank-vs-N.A. states. Then the text-layer PDF path for pre-2025-10 releases.
3. Transcendence: spread, drift, basket, weight-drift, convergence, verify, coverage.
