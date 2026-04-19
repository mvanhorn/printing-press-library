# Authenticated-session scraping for printed CLIs

_Last reviewed: 2026-04-19_

When a printed CLI needs to backfill rich user state behind a login — order history, saved items, profile settings, routines, scheduled work — the tooling choice matters more than the scraper itself. This document ranks the options available in 2026 and walks through the decision tree, anchored to the worked example of `pp-instacart`.

## When this applies

Use this playbook when all of the following are true:

- The data lives behind a logged-in account (cookies, session token, or interactive auth).
- The service does not expose a clean backend API you can use with the user's credentials (no documented REST endpoint, no clean persisted-queries list, no OAuth scope that returns what you need).
- You want the data in the CLI's local SQLite so commands like `add`, `search`, and `history list` can reason over it.

If the data is public (no login needed), use `firecrawl` or plain HTTP. If the service has a real API (Stripe, HubSpot, Linear), use their API. If the state is trivial to re-enter by hand, don't scrape — ask.

## The four tiers

### Tier 1: Chrome MCP (`claude-in-chrome`)

**What it is.** The user's real Chrome, already logged in, already past any MFA / profile picker / captcha. You drive it from Claude Code via the `claude-in-chrome` MCP tools (`tabs_create_mcp`, `navigate`, `javascript_tool`, `read_network_requests`, `read_page`).

**Pick it when:**
- It's a one-time backfill or infrequent top-up
- The auth flow has an interactive step (Instacart's multi-profile picker, Google's 2FA, captchas)
- The user is present during execution

**Operational pattern:**
1. User's Chrome is already logged in to the target service
2. Agent navigates via the MCP to the relevant page
3. Agent runs JS to extract data into `localStorage`
4. Agent triggers a Blob + `<a download>` click to get a file onto the user's machine
5. CLI consumes the file via a one-shot `import` command

**Why this is tier 1:** zero setup cost. No cookie extraction, no bot detection to work around. The user has already done the hard part; the agent is just borrowing their session.

**Why it's not tier 0:** it's not reusable. Each run requires the Claude Code session to still be connected to the MCP + the tab to still be logged in. Not a fit for scheduled jobs.

### Tier 2: Programmatic browser + imported cookies

**What it is.** A headless or headed browser (Playwright, Puppeteer, `agent-browser`, Vercel's `@vercel/agent-browser`, Browserbase, gstack) driven programmatically, with the user's cookies imported from their real Chrome profile via the `compound-engineering:setup-browser-cookies` skill.

**Pick it when:**
- The same scraping job needs to run repeatedly on a schedule
- The auth flow has no interactive step once cookies are loaded
- The user won't be present during execution

**Operational pattern:**
1. One-time: user runs `setup-browser-cookies` once, picks domains
2. Scheduled job: agent/CLI spins up a headless browser, loads the imported cookies, navigates, extracts
3. Data lands in SQLite or JSONL

**Trade-offs:**
- Cookies expire. The user re-runs `setup-browser-cookies` whenever the target service logs them out.
- Services with strong bot-detection (Cloudflare, hCaptcha challenges, device fingerprinting tied to browser profile hash) often break this tier even with real cookies.
- Multi-profile / MFA-each-login services cannot be automated here — they need tier 1.

**Vercel `@vercel/agent-browser` specifically:** cloud-hosted headless Chromium, great for running on Vercel functions. Well-suited to recurring sync jobs once cookies are imported via `setCookie`. Limited when the service redirects through an interactive profile picker that requires clicks the cloud browser can't attribute to a profile.

### Tier 3: Headless browser baked into the Go CLI

**What it is.** `chromedp` (pure-Go headless Chrome driver) or `playwright-go` linked directly into the CLI binary. Cookies loaded from the user's Chrome profile at CLI start.

**Pick it when:**
- The feature is first-class in the CLI and deserves zero dependencies on external services or MCPs
- You're running in CI or another fully-unattended context
- You're willing to accept the 50-100MB binary-size increase and the Chromium dependency

**Operational pattern:**
1. CLI ships Go code that imports a headless browser driver
2. `instacart history sync` spawns a browser subprocess, loads cookies, extracts, tears down
3. Errors at each step surface as structured CLI errors with remediation hints

**Trade-offs:**
- Heavyweight. Chromium is ~250MB installed; `chromedp` handles this by calling an already-installed Chrome if present.
- Your CLI's test matrix now includes browser version drift.
- Most printed CLIs do not need this. Defer to tier 1 or 2.

### Tier 4 (rejected for this class of problem): Firecrawl

Firecrawl is excellent at crawling and scraping **public** pages. It does not carry the user's authenticated session, so for "I want my own order history out of Instacart" it will return either the login page or the marketing page, not the user's data.

Do not use for: order history, account settings, private messages, payment history, behind-login dashboards.

Do use for: public pricing pages, blog posts, docs, archive.today, open catalogs.

## Decision table

| Factor | Chrome MCP (Tier 1) | agent-browser + cookies (Tier 2) | Go + chromedp (Tier 3) | Firecrawl (Tier 4) |
|---|---|---|---|---|
| Carries user auth | ✅ (native) | ✅ (imported) | ✅ (imported) | ❌ |
| Handles MFA / profile pickers | ✅ | ❌ | ❌ | — |
| Recurring / scheduled sync | ❌ | ✅ | ✅ | ✅ (public data only) |
| Binary-size cost | 0 | 0 | +250MB Chromium dep | 0 |
| Setup effort for user | 0 | `setup-browser-cookies` once | 0 | 0 |
| Bot-detection sensitivity | low (real browser) | medium-high | medium-high | high |

## Decision tree

```
Is the data public?
├── Yes                → Firecrawl
└── No (behind login)
     │
     ├── Is there interactive auth (MFA, profile picker, captcha)?
     │   ├── Yes       → Chrome MCP (tier 1), always
     │   └── No
     │        ├── Recurring sync needed?
     │        │   ├── Yes   → agent-browser + setup-browser-cookies (tier 2)
     │        │   └── No    → Chrome MCP (tier 1), still simpler
     │        │
     │        └── Unattended / CI execution required?
     │                     → chromedp baked into Go CLI (tier 3)
```

## When the target site is server-rendered (not Apollo / not a JSON API)

Some sites render user state in server-side HTML rather than client-side JS. Instacart's `/account/orders` is one — there is no "orders" GraphQL op you can find in DevTools, because the orders list is baked into the page by the Rails app before the JS loads.

If you see this pattern:

1. **Stop hunting for GraphQL ops.** You will not find one. Confirm by opening DevTools + reloading and filtering Network → Fetch/XHR for `graphql` — if zero requests mention your data type, it's server-rendered.
2. **Scrape the DOM.** Use `document.querySelectorAll` inside the logged-in tab to pull structured data. Pair with `fetch()` for per-record detail pages if needed.
3. **Check Apollo cache for enriched data.** Sometimes server-rendered pages hydrate an Apollo client with additional data after load — `window.__APOLLO_CLIENT__.cache.extract()` may have what you need even when no network GraphQL op fires.

**Anti-pattern to avoid:** assuming a clean GraphQL op exists for every user-state dimension. This was the architectural bug in the original `pp-instacart` history sync plan — we wrote code for `BuyItAgainPage` and `CustomerOrderHistory` ops that do not actually exist in Instacart's API. See `docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md`.

## The Instacart worked example

The `pp-instacart` CLI uses the full tier-1 pattern, wrapped in a one-command skill flow so end users do not need to understand the plumbing.

End-user invocation:

> "backfill my instacart orders"

The `/pp-instacart` skill's argument parser matches any of several backfill intents ("sync my history", "download my orders", etc.) and drives the Chrome MCP loop end to end:

1. **Skill probes.** Reads the user's existing `instacart history stats` to decide full backfill vs top-up. Confirms `claude-in-chrome` MCP tools are loaded; falls through to the DevTools copy-paste path in `library/commerce/instacart/docs/backfill-devtools-fallback.md` if not.
2. **Dumper walks the orders page.** Injects `library/commerce/instacart/docs/dumper.js` into the logged-in Chrome tab. The dumper scrolls, clicks "Load more" with bounded retries, and accumulates order IDs into `localStorage.__ic_backfill_state` so interrupted sessions resume.
3. **Extractor pulls per-order detail.** For each pending order, the skill navigates to `/store/orders/<id>` and injects `extract-one.js`. That script polls the Apollo cache up to 10s for the `OrderManagerOrderDelivery` entry with `includeOrderItems:true`, then normalizes the record and appends to `localStorage.__ic_dumped`. Failures push structured skip records instead of silently losing orders.
4. **Exporter downloads JSONL.** `export-jsonl.js` filters skip records, writes the rest as JSONL, and triggers a browser download of `instacart-orders.jsonl`.
5. **CLI imports.** Skill shells out to `instacart history import ~/Downloads/instacart-orders.jsonl --agent`. Upsert is idempotent, so re-runs are safe.
6. **`add` resolves via history.** `instacart add qfc "limoncello sorbet"` now returns `resolved_via: "history"` with the exact Alden's Organic Limoncello Sorbet Bars SKU the user previously bought, instead of the three-call live search picking Talenti.

Observed calibration from the first real run (2026-04-18):

- Dump phase: ~60 seconds to walk and extract 10 orders from the user's Instacart
- Import phase: ~1 second for 10 orders × 87 items total
- Resolver impact: history-first fires for queries the user has bought before at the retailer in question, falls through to live search otherwise (as designed)

The end-user-facing entry points are documented in `library/commerce/instacart/docs/backfill-walkthrough.md` (skill-driven) and `library/commerce/instacart/docs/backfill-devtools-fallback.md` (manual). This playbook's tier model is unchanged; the skill is simply the packaged tier-1 outcome for this CLI.

## Checklist for adding this pattern to a new printed CLI

Before starting, confirm:

- [ ] The data lives behind a login
- [ ] No documented API gives you what you need
- [ ] The service does not expose a clean GraphQL op you can persist
- [ ] You have the user's Chrome logged in and are running from Claude Code with `claude-in-chrome` MCP loaded

Then:

- [ ] Map the data. Visit the relevant page in Chrome, open DevTools, identify: server-rendered HTML? Apollo cache entries? Async JSON calls?
- [ ] Write a dumper JS snippet in `library/<category>/<cli>/docs/dumper.js`. Follow the Instacart pattern — localStorage accumulator, per-page Apollo cache extraction, Blob+download exporter.
- [ ] Add an `import` subcommand to your CLI that reads the JSONL into your local schema. Keep it idempotent.
- [ ] Write the end-to-end verification: dump → import → history-first resolver fires for a query the user has a match for.
- [ ] Update your CLI's SKILL.md to point users at this playbook for first-run history setup.
- [ ] Add your CLI to the "used in the wild" list at the bottom of this doc.

## Used in the wild

- `pp-instacart` — order history + purchased-items backfill, wrapped in a one-command skill flow end users can invoke with "backfill my instacart orders" (this document's worked example)

Add your CLI here when you adopt the pattern.

## Further reading

- `library/commerce/instacart/docs/dumper.js` — the working reference implementation of the order-list walker
- `library/commerce/instacart/docs/extract-one.js` — the per-order Apollo-cache extractor
- `library/commerce/instacart/docs/export-jsonl.js` — the Blob-download exporter
- `library/commerce/instacart/docs/backfill-walkthrough.md` — end-user walkthrough for the skill-driven flow
- `library/commerce/instacart/docs/backfill-devtools-fallback.md` — manual DevTools flow for users without Chrome MCP
- `library/commerce/instacart/internal/cli/history_import.go` — the Go-side importer
- `docs/solutions/best-practices/instacart-orders-no-clean-graphql-op.md` — the architecture-gap finding that motivated this playbook
