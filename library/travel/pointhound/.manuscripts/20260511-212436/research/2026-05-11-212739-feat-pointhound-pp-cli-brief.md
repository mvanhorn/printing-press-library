# Pointhound CLI Brief

## API Identity
- Domain: Award-flight search engine; turns credit-card points into specific, bookable
  redemptions across ~150 airlines.
- Users: Travel hackers, points hobbyists, and casual users who hold transferable
  points (Chase UR, Amex MR, Capital One, Citi TY, Bilt) and want to find
  high-value redemptions without scanning airline sites by hand.
- Data profile: Live award availability scrape across many airlines, transfer-partner
  metadata, deal-curation index, and a per-user alert/digest store. Search is
  short-lived (deals "change constantly; book within 24–48h"); curated digests
  are emailed when discoveries happen, not on a fixed cadence.

## Source Type
**Website-as-source (browser-sniff path).** Pointhound has no documented public API
and no developer portal (verified across pointhound.com, ycombinator.com/companies/pointhound,
crunchbase, and pitchbook on 2026-05-11). The Next.js frontend ships /api/* routes
that are robots-disallowed but not auth-gated for read-mode searches. The user
chose the "website itself" path in Phase 0, approving temporary Chrome capture
during generation so the printed CLI can replay against /api/* directly.

## Reachability Risk
- **Low.** `printing-press probe-reachability https://www.pointhound.com` returned
  `mode=standard_http`, `confidence=0.95`, stdlib HTTP 200 in 480ms. No
  Cloudflare challenge, no WAF block on the marketing pages. The /api/* routes
  return 404 on direct curl with no auth/session and need real browser context to
  enumerate — that is normal for Next.js route handlers, not a hard block.
- Caveat: This is a YC-funded consumer startup (launch Feb 2026). They could add
  rate-limit or anti-bot measures any week. The CLI is personal-use; if /api/*
  shape changes, regen.

## Top Workflows
1. **Standard search**: origin + destination + date → list of bookable redemptions
   sorted by point cost / cabin / airline, each with step-by-step booking
   instructions. The anchor workflow; no login required.
2. **Top Deals search**: up to 6 origins × 6 destinations × full year, returning
   the best matches across the matrix. Exploration mode for "where can I go
   this year with my points?".
3. **Flight Alerts**: set a watch on a route+date+cabin and get notified when
   seats open. Up to 20 active alerts on free tier; SMS notifications on Premium.
   Requires account.
4. **Deal Digests**: curated emails based on home airport + points programs;
   4–8/wk basic, customizable cadence on Premium, includes business/first class.
   Requires account.
5. **Credit-card / points-program lookup**: which cards earn into which programs,
   transfer ratios, sweet spots, etc. Static educational content under `/cards/*`
   and `/points101`; useful as a lightweight reference command.

## Authoritative Surface Surfaces
- Public, anonymous: `/`, `/search`, `/explore`, `/cards`, `/cards/<slug>`,
  `/cards/<slug>/review`, `/points101`, `/faq`, `/blog`, `/digest` (marketing),
  `/alerts` (marketing).
- robots-disallowed read endpoints (Next.js `/api/*` route handlers — paths to be
  enumerated by browser-sniff): search submit/results, top-deals submit/results,
  card catalog, airline metadata, alert CRUD (auth), digest preferences (auth).
- Auth-gated UI: `/login`, `/account/*`, `/alerts/create/*`, `/digest/onboarding/*`,
  `/onboarding/*`.

## Data Layer
- Primary entities (planned, to be confirmed by sniff):
  - **searches**: query parameters + result snapshots, keyed by query hash
  - **deals**: individual redemption rows (origin, destination, date, cabin,
    airline, points cost, taxes, source program, booking_url, expires_at)
  - **routes**: origin↔destination metadata (codes, region, airport)
  - **airlines**: IATA code, name, alliance, transfer-source programs
  - **programs**: transferable points programs and their transfer partners
  - **cards**: credit card catalog with earn rates and program affiliation
  - **alerts** (auth): user alert definitions + state
  - **digest_items** (auth-or-anonymous): curated deals that came in via email
- Sync cursor: last_seen_at per (route, date, cabin) tuple for delta polling.
- FTS/search: full-text over deals (airline names, route descriptions, card
  recommendations) and cards (card name, issuer, perks).

## Codebase Intelligence
Skipped — no public source repo or MCP server found for Pointhound.

## Source Priority
Single-source. The "official API" arm of Phase 0 was probed and found empty;
research confirmed no public API exists. The whole CLI builds against the
website's internal /api/* surface (and a thin SQLite cache).

## User Vision
The user did not volunteer a vision beyond "build a CLI." The brief therefore
optimizes for the canonical award-search workflow plus the kind of automation
no web UI ships — cron-style polling, drift detection, batch route fan-out,
multi-route SQL — that justifies a CLI over the website.

## Competing Tools (Same Space, Not Pointhound-Specific)
Captured here for absorb context in Phase 1.5; none are Pointhound wrappers.

| Tool | Programs | Strengths | Free tier? |
|---|---|---|---|
| seats.aero | ~15 | Cached, fast, IATA filter; **has a public API**; SMS alerts on Pro | Free + $9.99/mo |
| point.me | 30+ | Most programs; "explore" feature; free 60-day window | Free + paid |
| PointsYeah | ~20 | Live data; "Explorer Alerts" between non-specific O/D; 32 alerts | Free + paid |
| Roame.Travel | 6 (15 w/ SkyView) | Polished UI; 48h deals overview | Free + paid |
| AwardTool | 20+ | Free, filter by category, price-trend history | Free |
| ExpertFlyer | 400+ | Seat maps, schedules, deep airline data | Paid |
| AwardFares | 20+ | Recent point.me alternative | Paid |
| AwardWallet | n/a | Points-balance tracker + alerts | Free + paid |

No CLI competitor exists for any of these — the GitHub search returned
`jaebradley/flights-search-cli` (general flight search, not award-specific) as
the closest neighbor. Pointhound is also not in the seats.aero/point.me CLI
gap; this is greenfield CLI territory in award travel.

## Product Thesis
- **Name (working):** `pointhound-pp-cli`
- **Why it should exist:** Pointhound's web UI is great for one-off searches but
  doesn't compound. A CLI lets you (a) batch-search 20 routes overnight via
  cron, (b) keep a local SQLite of every deal you've ever seen so you can
  notice when a new redemption is genuinely better, (c) wire it into other
  workflows (calendar holds, Notion drops, group-trip planning), and
  (d) get terse `--json` output for AI agents picking redemptions on your
  behalf. The web product can't ship those without abandoning the consumer
  surface.

## Build Priorities
1. **Anchor: `search`** — origin/dest/date(s), filters for cabin/program/airline,
   real `/api/*` call replayed via Chrome cookies (or anon if anon search is
   replayable), `--json` for agents, local cache write.
2. **`top-deals`** — multi-origin × multi-destination × year matrix; surface as
   one command, store results.
3. **`cards`, `cards get <slug>`** — credit card catalog (reads `/cards/*` or
   the underlying API). Static-ish but useful as a reference table.
4. **`points101`** — programs, transfer ratios, sweet spots. Either from the
   `/api/*` if exposed or from scraping the `/points101` SSR page once.
5. **`alerts list/create/delete`** (auth-gated, gated on session cookie).
6. **`digest`** (auth-gated reads).
7. **Local store**: `sync`, `stale`, `search` (FTS), `sql`.

Phase 1.5 will enumerate the transcendence set; the obvious candidates are
**`watch`** (cron-friendly polling), **`drift`** (deal got better/worse since
last snapshot), **`batch`** (fan out one search call across N routes),
**`calendar`** (best-month-of-the-year heatmap), and **`compare`**
(point.me vs Pointhound vs seats.aero where the user has keys for the others).

## Open Questions for Browser-Sniff
- What is the exact `/api/*` shape for a Standard Search request/response?
- Is anon search replayable (cookieless) or does it require a session warmup?
- What is the Top Deals request shape (matrix structure)?
- Is the `/api/cards` catalog enumerable in one request, or paginated?
- Is the airline/program catalog exposed at `/api/*` or only via SSR?
- For alerts: does mutation require CSRF token, or just session cookie?

Browser-sniff (Phase 1.7) will close these. Reachability is settled; runtime
will be `standard_http` per the probe.
