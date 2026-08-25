# Sensor Tower CLI Brief

## API Identity

- **Domain:** Mobile app market intelligence — store rankings, app/publisher profiles,
  category charts across the iOS App Store and Google Play.
- **Surface being built:** `app.sensortower.com/api/*` — the internal endpoints behind the
  Sensor Tower React dashboard. **Not** `api.sensortower.com` (the paid enterprise product),
  which is unreachable on this account (`api_authorized: false`, `plan: free`, no API key).
- **Users:** ASO managers, UA/growth marketers, indie app developers, mobile PMs,
  competitive-intel analysts, and VC/market analysts doing due diligence.
- **Data profile:** Exact daily category ranks per country/device/chart-type; full app
  profiles (51 fields incl. 400+ version-history entries, IAP lists, rating histograms,
  related apps); publisher portfolios; a 99-entity catalog search. Revenue/download
  estimates exist but are **1-significant-figure bucketed**.

## Reachability Risk

- **None / Low.** No bot protection: in-browser challenge check returned
  `{"challenge": false}`; no Cloudflare/Turnstile/DataDome/CAPTCHA. Every endpoint replays
  through plain `curl` + Chrome UA. Runtime is `standard_http` — no browser sidecar.
- **Rate limiting is the real constraint, not blocking.** ~13 requests succeed, #14 → 429;
  recovery ≈240s. `server: awselb/2.0`, no `Retry-After`, no `X-RateLimit-*`, empty body.
  Enforced at the AWS ELB. Reproduced independently in this run (a sticky 429 survived a
  30s backoff; cleared after 90s+).
- **Prior-art corroboration:** `onatcipli/skills` `api_notes.md` independently documents
  this API as no-auth with a ~180-id 429 wall and coarsely bucketed estimates.
- Tier/permission hints from 401 body: `{"error":"You need to sign in or sign up before
  continuing."}` on `/api/unified/apps` — the one place the session cookie matters.

## Auth model (A/B verified, anonymous vs cookie)

Nearly the whole surface is **open anonymously**. The session cookie unlocks exactly the
`unified/*` cross-platform identity endpoints (`/api/unified/apps`, `/api/unified/publishers`),
which 401 without it. Spec shape: `auth.type: cookie` + `no_auth: true` on the 9 open
endpoints, so the CLI works with zero setup and `auth login --chrome` unlocks unified.

## Top Workflows

1. **Weekly rank check on my app and my rivals.** Open the dashboard, look at where an app
   sits in the free/grossing charts for a country, eyeball whether it moved. Repeated every
   Monday. The dashboard shows *now* — it keeps no history the user can query, and rank
   deltas across weeks must be reconstructed by memory or manual screenshots.
2. **Competitor teardown when a rival spikes.** A competitor jumps the charts; the analyst
   pulls their app profile, version history, IAP list, and category ranks to infer what
   changed (a release? a price change? a new IAP tier?).
3. **New-entrant / chart monitoring.** Scan a category's top chart to spot apps that
   appeared or climbed sharply since the last look — the "who's new in Finance this week"
   ritual.
4. **Publisher portfolio review.** Given a publisher, enumerate their whole app portfolio
   and see which titles carry the ranking weight.
5. **Cross-platform identity resolution.** Establish that an iOS app and an Android package
   are the same product before comparing them (the one cookie-gated workflow).

## Table Stakes

Features that exist in any competing tool and must be matched:

- Top charts / category rankings (free, paid, grossing × iphone/ipad/phone)
- Rank deltas vs previous period
- App metadata lookup (name, publisher, rating, price, screenshots, description)
- Version / update history timeline
- Publisher portfolio enumeration
- Catalog search by app or publisher name
- In-app purchase listings
- Rating breakdown / review sampling
- Downloads & revenue estimates (bucketed here)
- Related / similar apps

## Data Layer

- **Primary entities:** `apps` (iOS/Android/unified), `publishers`, `rankings`
  (chart rows per category/country/device/date), `versions` (release timeline),
  `categories` (reference data).
- **Sync cursor:** chart `date` for rankings; `versions[].date` (epoch ms) for release
  history; snapshot timestamp for app profiles.
- **FTS/search:** app `name`, `publisher_name`, `description`, `humanized_name`.
- **Why a store is essential:** the API is rate-limited to ~12 requests per 4-minute
  window and keeps **no history**. Local SQLite is what converts 12 expensive reads into
  rank tracking, movers detection, and trend analysis — and it is the only way to answer
  "what changed since last week" at all.

## Codebase Intelligence

- Source: prior-art analysis of `FerdiKT/sensortower-cli` (Go, 1★), `ivangomozov/Sensortower-top100`
  (Python), `onatcipli/skills` (`api_notes.md`), `evekeen/appstore-revenue-mcp` (TS).
- **Auth:** `FerdiKT/sensortower-cli` uses `SENSORTOWER_COOKIE`; `onatcipli` and `evekeen`
  document the surface as no-auth. Matches this run's A/B finding.
- **Data model:** `/api/ios/apps/{id}` is the hub object — 51 keys, embeds rankings,
  versions, IAPs, reviews, related apps. Chart rows carry `previous_rank` (free rank-delta).
- **Rate limiting:** independently reported ~180-id wall (`onatcipli`) and measured at
  ~13 req → 429 / 240s recovery in this run's research.
- **Architecture insight:** 422 responses are self-documenting — omitting `category` on
  Android returns the full ~65-value enum list. The API teaches its own schema.

## Competitive Landscape

- **`FerdiKT/sensortower-cli`** (Go, 1★) — closest prior art. Same base URL, cookie auth,
  Cobra. Commands: `search apps|publishers`, `publishers apps`, `apps get`,
  `charts category-rankings`, `aso metadata-audit|keyword-gap`, `workflow competitors|fresh-earners`.
  Self-described "early public beta", **iOS-only**, no local persistence.
- **`virusimmortal00/sensortower-mcp`** (Python, 17★) — largest MCP, but paid-API only
  (`SENSOR_TOWER_API_TOKEN`), stale since 2025-10-03. Not our path.
- **`econosopher/sensortowerR`** (R, 6★, CRAN) — paid API; clean 4-verb design worth borrowing.
- Remaining npm/PyPI entries are ~0★ paid-API MCP wrappers or fragile Selenium scrapers.
- **No dominant free-surface tool exists. The niche is open.**

## Pain Points (evidence-backed)

1. **Estimates are materially wrong.** App owner at $90K MRR: *"Sensor Tower shows my app
   at $40K MRR, which is less than half of the actual number… I see a lot of people blindly
   copying app ideas"* (@SinaSinry). Consistent with the bucketed values seen here.
2. **The informed rebuttal points at the product design.**
   *"Yes, Sensor Tower is wrong. No, it doesn't matter"* (moonbearmusings.com): consistent
   methodology means *"movement and trends are far more critical versus the actual
   underlying number."* → **rank/delta-first CLI, money as a soft signal.**
3. **Price vs. value.** G2 4.3★/142: "expensive for startups"; Capterra: "only big
   enterprises able to afford it." The free surface is the only surface most indies get.
4. **No history in the UI.** The dashboard answers "where is it now", never "what changed
   since last Monday" — the exact question every weekly ritual above is asking.

## Product Thesis

- **Name:** `sensortower-pp-cli`
- **Why it should exist:** Sensor Tower's dashboard shows you a rank *right now* and forgets
  it tomorrow. The API is rate-limited to roughly a dozen calls per four minutes and keeps
  no history at all. A cache-first CLI with a local SQLite mirror turns that scarce, amnesiac
  surface into something the web UI fundamentally cannot be: a rank history you can query,
  diff, and trend — across **both** iOS and Android, from an account that pays nothing.
  Ranks are exact and free; only the money is fuzzy. Build on the exact part.

## Build Priorities

1. **Data layer + sync** for apps, rankings, publishers, versions — with an adaptive rate
   limiter (~12 req/burst, 240s cooldown) and a typed rate-limit error. This is the
   foundation; everything transcendent depends on it.
2. **Absorbed surface:** app lookup (iOS/Android/unified), catalog search, top charts
   (iOS+Android), rank history, ranking summary, publisher portfolio, categories reference.
3. **Transcendence:** rank tracking over time, movers/new-entrant detection, cross-platform
   comparison, version-timeline diffing — the things that only exist because of the local store.
