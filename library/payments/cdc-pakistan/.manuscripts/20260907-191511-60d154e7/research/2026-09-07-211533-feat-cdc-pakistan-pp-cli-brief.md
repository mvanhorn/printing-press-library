# CDC Pakistan CLI Brief

## API Identity
- Domain: Central Depository Company of Pakistan (CDC) — the CSD for the Pakistani capital market.
- Users: quantitative equity researchers, corporate-actions analysts, market-structure economists
  working the Pakistani market; operators who already hold PSX prices, NCCPL clearing flows and
  MUFAP fund data in one local research DB.
- Data profile: NO JSON API. One unauthenticated WordPress AJAX endpoint returning HTML
  fragments, one HTML statistics table, and PDF report series. Whole site Cloudflare-challenged.

## Reachability Risk
- HIGH but SOLVED. Every path except /robots.txt returns 403 + `cf-mitigated: challenge`.
  NOT the TLS-fingerprint trap: curl_cffi chrome124/120/110 all 403; press `probe-reachability`
  reports `browser_clearance_http` and its surf-chrome probe also 403s.
- One headed real-Chrome visit clears it in <5s; the resulting cf_clearance then replays over
  PLAIN Go stdlib HTTP (verified 200 on HTML and PDF alike). So transport is `standard` + cookie.
- **HARD ~30-MINUTE COOKIE LIFETIME, measured twice.** The cookie's own `expires` claims 365
  days and is meaningless. Alive at t+25m, dead at t+30m, with traffic every <=5 min; a second
  run died at t+31m after 23 min idle. Idle-timeout hypothesis falsified.
- Probe-safe endpoint used: POST /wp-admin/admin-ajax.php (read-only listing action).

## Top Workflows
1. Pull the current per-security CDS-penetration cross-section and join it to PSX symbols/ISINs.
2. Reconstruct a security's identity history (symbol/name/ISIN changes) to avoid splicing two
   issuers into one return series.
3. Fold 20 years of notices/circulars into a per-security CDS-eligibility state history.
4. Audit whether a local mirror is complete, and which months/categories are genuinely absent
   at source versus never requested.
5. Track CDS aggregate statistics forward from first sync (retail participation, custody value).

## Table Stakes
- NONE. There is no incumbent. Zero tools globally read any CDC surface; `update_posts_by_year`
  has zero code hits on GitHub in any language. The absorb manifest is empty by measurement.

## Data Layer
- Primary entities: security (ISIN-keyed), document (url-keyed), eligibility_event,
  identity_event, penetration_row (vintage x ISIN), stats_snapshot (month), coverage_bucket.
- Sync cursor: (category, year, paged) for documents; vintage date for PDF series.
- FTS/search: document titles (7,956 rows) + security names.

## Source Priority
- Single source. No combo ordering needed.

## Product Thesis
- Name: cdc-pakistan-pp-cli
- Why it should exist: CDC publishes the only per-security custody-penetration data in the
  Pakistani market and does so as split PDFs with no history endpoint, no API, and behind a
  bot challenge. Nobody has ever automated it. The extraction layer is the moat, not the
  dataset — Taiwan (TDCC) and Indonesia (KSEI) ship the same panel as keyless CSV.

## Build Priorities
1. Transport: cookie+UA capture as one unit, ~27-min margin check before any work, and
   challenge-HTML classified as a TYPED TRANSPORT ERROR — never as an empty result.
2. Column-aware PDF parser (equity vs debt layout in part A; 7-column fund schema in part B).
3. Document enumeration via admin-ajax ONLY, chunked/checkpointed/resumable.
4. Event classification + eligibility state machine over 7,639 documents.
5. Identity ledger from symbol/name-change events + security-list vintages.

## Known Gaps (write into README before shipping)
- Free-float panel is ONE live vintage (Nov-2025). Not a time series.
- Statistics history is ~42 NON-CONTIGUOUS archived months (2015-09..2024-06), best-effort.
- Part A covers 29,350 securities vs the statistics page's 732 listed + 59,217 unlisted.
  Residual flagged, NOT explained.
- 1,150 of 7,956 documents have no date-bearing URL path; 1,950 were bulk-uploaded in 2015,
  so upload_year is useless as an as-of date for ~a third of the corpus.
- Holder-level positions are NOT public. The 9-class investor taxonomy is NOT published.
- cdcsrsl.com (Share Registrar) is separately Cloudflare-challenged and NOT mapped.

## Governance
/robots.txt sets `Allow: /` for `User-agent: *` with
`Content-Signal: search=yes, ai-train=no, use=reference` under an Article 4 EU DSM
reservation, and `Disallow: /` for ClaudeBot, GPTBot, CCBot, Bytespider, Google-Extended,
meta-externalagent, Amazonbot. Surfaced to the owner, who elected to proceed. The printed CLI
must not identify as any named AI crawler and this data must not feed model training.
