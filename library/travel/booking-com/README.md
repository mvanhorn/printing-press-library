# Booking.com CLI

**Every Booking.com workflow, plus offline price history, wishlist drop alerts, and multi-leg planning no other Booking.com tool ships.**

Search Booking.com, scrape hotel detail and reviews, watch prices over time, and read your trips, wishlist, and Genius rewards via a Chrome cookie import. Local SQLite price_history powers cheapest-date sweeps, price-drop watch, wishlist diff, and seasonal price-band rollups that the booking.com UI cannot answer.

Learn more at [Booking.com](https://www.booking.com).

Created by [@mvanhorn](https://github.com/mvanhorn) (Matt Van Horn).
Contributors: [@tmchow](https://github.com/tmchow) (Trevin Chow).

## Install

The recommended path installs both the `booking-com-pp-cli` binary and the `pp-booking-com` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install booking-com
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install booking-com --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install booking-com --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install booking-com --agent claude-code
npx -y @mvanhorn/printing-press-library install booking-com --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/booking-com/cmd/booking-com-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booking-com-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install booking-com --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-booking-com --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-booking-com --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install booking-com --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

The bundle reuses your local browser session — set it up first if you haven't:

```bash
booking-com-pp-cli auth login --chrome
```

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/booking-com-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "booking-com": {
      "command": "booking-com-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public search, hotel detail, reviews, destinations, and map markers run anonymously through Surf with a Chrome TLS fingerprint that clears Booking.com's AWS WAF challenge without a clearance cookie. Authenticated commands (trips, wishlist, rewards, profile) need a logged-in Chrome session: run `booking-com-pp-cli auth login --chrome` once and the CLI imports the booking.com session cookies so subsequent authenticated calls replay through Surf.

## Quick Start

```bash
# Headline hotel search — 25 SSR-extracted property cards with --select narrowing the response shape.
booking-com-pp-cli hotels list --query Paris --checkin 2026-06-20 --checkout 2026-06-23 --adults 2 --json --select '[].name,[].price,[].review_score,[].url'

# Drill into a property: amenities, address, geo, rating from the JSON-LD Hotel schema.
booking-com-pp-cli hotels get fr auliviaopera --checkin 2026-06-20 --checkout 2026-06-23 --json

# Local price_history sweep — answers 'when is this hotel cheapest in summer?' in one call.
booking-com-pp-cli prices cheapest --slug auliviaopera --country fr --window 2026-06-01..2026-08-31 --nights 3 --agent

# Verify auth + reachability before authenticated commands. Run 'booking-com-pp-cli auth login --chrome' first if doctor reports missing cookies.
booking-com-pp-cli doctor --json

# Weekly digest of wishlist items whose price dropped at least 5 percent in the last 7 days.
booking-com-pp-cli wishlist drops --since 168h --min-pct 5 --agent

# Monday-morning alarm: upcoming trips whose free-cancellation deadline is within 7 days.
booking-com-pp-cli trips deadlines --within 168h --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`prices cheapest`** — Sweep candidate checkin dates for a fixed-night stay at one hotel and return the lowest nightly totals from local price_history.

  _Reach for this before recommending dates to a user. It removes Booking's manual click-through-every-date dance and exposes the seasonal price floor for the property in one call._

  ```bash
  booking-com-pp-cli prices cheapest --slug auliviaopera --country fr --window 2026-06-01..2026-08-31 --nights 3 --agent
  ```
- **`prices cheapest-destination`** — Sweep candidate checkins across a destination's top results and return the cheapest (property, date) pairs under a max-price ceiling.

  _Use when an agent has a flexible-date traveler in a flexible-property mood. Returns a budget-constrained Pareto frontier instead of one ranked list per date._

  ```bash
  booking-com-pp-cli prices cheapest-destination --query Paris --window 2026-06-01..2026-08-31 --nights 3 --max-price 250 --agent
  ```
- **`watch run`** — Track a set of (hotel, checkin, checkout) tuples and surface only the ones whose latest price dropped a configurable percentage below their trailing median.

  _Schedule this nightly. Returns empty most days, returns gold when a watched property dropped enough to act on._

  ```bash
  booking-com-pp-cli watch run --min-pct 7 --agent
  ```
- **`destinations price-band`** — Aggregates price_history for a destination's synced properties and emits per-month median, min, and max nightly rate plus contributing property-count.

  _Use when planning a flexible-month trip. Returns the cheapest month + 'avoid this month' signal without manual searching._

  ```bash
  booking-com-pp-cli destinations price-band --query Paris --year 2026 --nights 3 --agent
  ```
- **`search`** — After `sync` populates the local property store, FTS5 ranks free-text queries over name + description + amenity strings with BM25, no network call.

  _After repeated `sync` calls have built a corpus, this lets an agent answer cross-destination property questions without re-hitting the network._

  ```bash
  booking-com-pp-cli search "boutique near louvre with rooftop" --agent
  ```

### Agent-native plumbing
- **`wishlist drops`** — Joins the authenticated wishlist with the local price_history and surfaces saved properties whose latest observed price is N% below the previous observation.

  _Use Sunday morning. Returns the small set of wishlist items worth booking now, instead of forcing the user to eyeball 30-40 saved properties._

  ```bash
  booking-com-pp-cli wishlist drops --since 168h --min-pct 5 --agent
  ```
- **`compare`** — Fetches detail + reviews for two hotels in parallel and emits a paired struct (price, score, amenity Δ, distance, free-cancellation, breakfast, recent-review counts).

  _When an agent has narrowed to two finalists, reach for this instead of re-rendering both detail pages and asking the user to read both._

  ```bash
  booking-com-pp-cli compare auliviaopera plazaathenee --checkin 2026-07-15 --checkout 2026-07-18 --agent
  ```
- **`trips deadlines`** — Walks authenticated upcoming trips, extracts the free-cancellation-until deadline from each trip detail, and returns trips whose deadline is within a configurable window.

  _Booking penalizes missed cancellation deadlines. Run this each Monday morning to catch deadlines before they expire._

  ```bash
  booking-com-pp-cli trips deadlines --within 168h --agent
  ```
- **`trips export`** — Walks authenticated past-trip list, opens each trip detail, and emits a deterministic CSV (confirmation, property, checkin, checkout, currency, total, address) ready to paste into expense systems.

  _Use Monday morning for last week's reimbursements. One call replaces clicking through every past trip._

  ```bash
  booking-com-pp-cli trips export --state past --since 2026-01-01 --format csv
  ```
- **`reviews stats`** — Local SQL group-by over synced reviews; counts and median score per score-band, language, and traveler-type bucket. Mechanical, no NLP.

  _Reach for this when an agent is matching property fit to a traveler type. Returns a bucket distribution instead of forcing the agent to read 1000 reviews._

  ```bash
  booking-com-pp-cli reviews stats --slug auliviaopera --country fr --by score-band,language,traveler-type --agent
  ```

### Reachability mitigation
- **`trip plan`** — Given multiple destination + date legs and a total budget, picks the cheapest property per leg whose summed nightly totals fit the budget, with a bounded combinatorial fallback when greedy busts.

  _For multi-city European itineraries the agent can answer the budget-constrained question in one round-trip instead of asking the user to iterate per leg._

  ```bash
  booking-com-pp-cli trip plan --leg Rome:2026-07-10:2026-07-13 --leg Florence:2026-07-13:2026-07-16 --leg Venice:2026-07-16:2026-07-20 --budget 1800 --filters free_cancellation,breakfast,score>=8 --agent
  ```
- **`genius impact`** — Runs an absorbed search twice — once with the authenticated cookie (Genius rates applied) and once anonymously — and diffs price-per-property to surface the Genius savings delta.

  _Use when an agent helps a user evaluate whether a Booking Genius tier is worth chasing. Reports the actual unlocked discount on a real search._

  ```bash
  booking-com-pp-cli genius impact --query Paris --checkin 2026-07-15 --checkout 2026-07-18 --adults 2 --agent
  ```
- **`deals mobile-rates`** — Re-runs an absorbed search with a Chrome mobile UA on top of the desktop call and diffs to surface mobile-only discounts Booking hides from desktop users.

  _When an agent is hunting savings, reach for this before recommending the desktop-quoted rate. Mobile rates can be 5-15 percent lower on the same hotel + dates._

  ```bash
  booking-com-pp-cli deals mobile-rates --query Paris --checkin 2026-07-15 --checkout 2026-07-18 --agent
  ```

## Usage

Run `booking-com-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Authenticated account/profile page at `secure.booking.com/mysettings.html`. Returns display name, Genius tier, account email (redacted), language, currency, and country.

- **`booking-com-pp-cli account`** - Read the authenticated user's display name, Genius tier (e.g. Level 3), preferred language, and preferred currency.

### attractions

Attractions, tours, and experiences at `www.booking.com/attractions/searchresults/<country>/<city>.html`. Returns activity cards with price, duration, rating, and product slug for detail lookups.

- **`booking-com-pp-cli attractions get`** - Fetch full attraction detail (description, inclusions, duration options, meeting point, cancellation policy, reviews summary).
- **`booking-com-pp-cli attractions search`** - Search attractions in a city. Returns SSR-extracted activity cards with price, rating, and product slug. Use `attractions get <country> <slug>` for full activity detail.

### cars

Car rental landing at `www.booking.com/cars/index.html`. Booking.com cars is powered by Rentalcars and uses a self-posting form for search; deep-link results URLs are not supported. This resource exposes the top deals visible on the landing page and supplier landing pages (Hertz, Sixt, Avis, etc.) so agents can recommend pickup locations and known suppliers without claiming results we can't deliver.

- **`booking-com-pp-cli cars`** - Read the car-rental landing page: featured deals, supported suppliers, and supported city-level pickup locations. Does NOT return live search results -- Booking.com cars does not support deep-link search URLs, so live pricing requires the web UI. Use this command to enumerate suppliers and locations, then direct the user to the Booking cars page for booking.

### destinations

Booking.com destination lookup. Resolves a free-text destination string (city, region, neighborhood, landmark) to a stable `dest_id` and `dest_type` that other commands can use.

- **`booking-com-pp-cli destinations`** - Trigger Booking.com's destination resolver by posting a search with `ss=<text>` and parsing the destination context the SSR HTML returns (dest_id, dest_type, country, urlname). Use this once per destination, then cache the result for subsequent search/cheapest-dates commands.

### flights

Flights search at `flights.booking.com/flights/<ORIG>-<DEST>/`. Booking's flight search is powered by Etraveli but exposes a clean SSR URL pattern. Returns flight cards with carrier, departure/arrival times, layovers, duration, and price.

- **`booking-com-pp-cli flights <origin> <destination>`** - Search flights between two IATA airport codes. Returns SSR-extracted flight cards with carrier, times, layovers, and price for the requested date range and cabin class.

### hotels

Hotel/property search and detail. `hotels list` parses the SSR `/searchresults.html` page (25 cards per page via offset). `hotels get` parses individual `/hotel/<country>/<slug>.html` detail pages including JSON-LD Hotel schema.

- **`booking-com-pp-cli hotels get`** - Fetch full hotel detail for a given country code + property slug. Parses Booking.com's JSON-LD Hotel schema for name, rating, review count, address, geo coordinates, and amenity highlights. Pass dates + guest count to get the live nightly price displayed for that stay.
- **`booking-com-pp-cli hotels list`** - Search Booking.com hotels by destination + dates + guests + filters. Returns SSR-extracted property cards. Combine with `prices cheapest` (transcendence) to sweep nightly prices across a date window.

### map

Map-view hotel pins for a search result set. Uses Booking.com's internal GraphQL endpoint (`/dml/graphql`, operation `MapMarkersDesktop`) with the CSRF token extracted from the search-results HTML.

- **`booking-com-pp-cli map`** - Fetch map-marker data for a destination + date range: per-property latitude, longitude, summary price, and rating. Lighter payload than the full search; useful for spatial filtering before pulling detail.

### reviews

Hotel reviews. Booking.com renders the first review batch in the hotel detail HTML; further pages are at `/reviewlist.html` keyed by hotel slug.

- **`booking-com-pp-cli reviews`** - Paginated reviews for a hotel. Returns review text, score, traveler type, language, stay date, and reviewer country. Supports score-band, language, and traveler-type filters.

### rewards

Authenticated Genius loyalty + Rewards Wallet status at `secure.booking.com/rewards_and_wallet.html`. Returns Genius level (1-3), unlocked discount tiers, available credit, and pending vouchers.

- **`booking-com-pp-cli rewards`** - Get the authenticated user's Genius level, lifetime stays, current credit balance, pending vouchers, and the property categories Genius discounts currently apply to.

### trips

Authenticated `My Trips` page at `secure.booking.com/mytrips.html`. Lists upcoming + past bookings with confirmation numbers, check-in/out dates, property name, and total price. Requires cookie import.

- **`booking-com-pp-cli trips`** - List the authenticated user's upcoming and past Booking.com reservations. Reads from the SSR HTML so it does not burn API tokens.

### wishlist

Authenticated `Saved` wishlist at `www.booking.com/mywishlist.html?wl_id=<id>`. The user's wishlist id is server-resolved from the session cookie. Returns the saved properties with last-seen price snapshots.

- **`booking-com-pp-cli wishlist`** - Fetch the authenticated user's wishlist. Returns each saved property's name, slug, country, last-seen price, and the date it was added.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
booking-com-pp-cli account

# JSON for scripting and agents
booking-com-pp-cli account --json

# Filter to specific fields
booking-com-pp-cli account --json --select id,name,status

# Dry run — show the request without sending
booking-com-pp-cli account --dry-run

# Agent mode — JSON + compact + no prompts in one flag
booking-com-pp-cli account --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `BOOKING_COM_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `booking-com-pp-cli cars`
- `booking-com-pp-cli trips`
- `booking-com-pp-cli wishlist`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
booking-com-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/booking-com-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `booking-com-pp-cli doctor` to check credentials
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **All search calls return HTTP 202 with 'Just a moment' HTML body** — Booking.com refreshed its AWS WAF signature. Run `booking-com-pp-cli doctor` to confirm reachability; if the Surf+Chrome probe is failing, upgrade printing-press so the bundled Surf transport ships the latest Chrome TLS fingerprint.
- **Authenticated commands (trips, wishlist, rewards) redirect to the sign-in page** — Your imported cookies expired. Re-run `booking-com-pp-cli auth login --chrome` after refreshing the booking.com tab in Chrome.
- **Map markers command returns 'CSRF token expired'** — The internal /dml/graphql endpoint rotates the CSRF token every ~30 minutes. The CLI auto-refreshes by re-fetching the search-results page; pass `--no-cache` to force a fresh fetch on the next call.
- **`prices cheapest` returns fewer rows than --window days** — Booking shows 'sold out' for some dates and drops them from the SSR card list. Inspect with `--include-unavailable` to see which dates were probed without prices.
- **`wishlist drops` returns empty even after observed price changes** — Drop detection requires >= 2 observations per item. Run `booking-com-pp-cli wishlist get` daily (or wire it to a cron) before `wishlist drops` will diff meaningfully.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Known Gaps

v0.1 ships with these documented limitations. None block the core hotels-search workflow.

- **Populated trips/wishlist parsing is inferred.** The browser-sniff that built this CLI saw an empty trips list and empty wishlist (the printer's Booking.com account is associated with the China site, not the Global site). Selectors for populated trip/wishlist cards are derived from documentation; first user with real trips/wishlist may surface selector misses worth filing an issue for.
- **Hotel detail latitude/longitude default to 0.** Booking.com's JSON-LD `geo` field is not always populated; the parser falls through to zero rather than guess. The `map` command provides accurate coordinates via the live GraphQL endpoint when a destination id is known.
- **Attractions `price_from` and `review_score` are best-effort.** The card-text regex picks the first positive number it finds, which works for many cards but can latch onto a duration ("2 hours"), a review count ("3,414 reviews"), or a promo discount instead of the actual price or rating. The previous `-20` sentinel from leaking negative-sign promo text is fixed in v0.1; structural per-field selectors are queued for v0.2. Use `--json --select '[].name,[].slug,[].url'` to ignore the unreliable fields.
- **Reviews pagination via `/dml/graphql` deferred.** The map view's GraphQL operation (`MapMarkersDesktop`) is now wired through the `map` command, including b_csrf_token extraction and the `X-Booking-CSRF-Token` header. The review-pagination operation was not exercised during the browser-sniff capture session; `reviews list --page N` for N>1 falls back to the static SSR review list. Capturing the review-pagination GraphQL operation is queued for v0.2.
- **`prices cheapest`, `wishlist drops`, `destinations price-band`, and `watch run` require populated price_history.** These commands query a local SQLite table that grows as you run searches over time. First-run experience may return empty until repeat use populates the table.
- **`cars list` is read-only landing.** Booking.com cars (Rentalcars-powered) uses a self-posting search form, not deep-link results URLs. The `cars list` command exposes the landing-page supplier list and city pickup paths; live car-rental search would need a token-exchange flow built in v0.2.
- **`genius impact` and `deals mobile-rates` accuracy depends on Booking.com response shape.** These commands diff two search calls (auth vs anon, desktop vs mobile UA). If Booking.com already applies Genius rates to the authenticated default response, the diff will be zero — that's accurate, not a bug.
- **The embedded MapMarkersDesktop query is a frozen snapshot.** `internal/booking/operations/mapmarkers.json` was captured 2026-05-19 against booking.com's live schema. Booking.com may ship schema changes that invalidate the operation; if `map` starts returning empty or 4xx errors with no other signal, re-capture the operation via the printing-press browser-sniff flow.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.booking.com/
- Capture coverage: 1 API entries from 14 total network entries
- Reachability: browser_http (90% confidence)
- Protocols: ssr_html (95% confidence), rest_graphql (70% confidence)
- Auth signals: none; cookie — cookies: aws-waf-token, bkng, bkng_sso_session, cgumid, bkng_prue
- Protection signals: aws_waf (95% confidence)
- Generation hints: requires_browser_http, requires_browser_cookie_for_auth_endpoints, has_ssr_html_primary_surface, has_csrf_token_for_graphql, supports_offset_pagination
- Candidate command ideas: search — SSR /searchresults.html returns 25 property cards parseable from HTML.; hotel get — SSR /hotel/<country>/<slug>.html returns full hotel JSON-LD.; reviews list — /reviewlist.html exists as a separate paginated surface.; destinations autocomplete — Autocomplete dropdown on homepage fires destination resolver.; trips list — secure.booking.com/mytrips.html loaded with cookie auth and rendered user-specific UI.; wishlist get — www.booking.com/mywishlist.html?wl_id=<id> loaded with cookie auth.; rewards get — secure.booking.com/rewards_and_wallet.html linked from authenticated header.; profile get — secure.booking.com/mysettings.html linked from authenticated header (showed 'Matt Van Horn, Genius Level 3').

Warnings from discovery:
- ssr_dominant: All observed user-visible pages are server-rendered. Initial searches, hotel detail, trips, and wishlist deliver their data in HTML rather than XHR/GraphQL. The /dml/graphql endpoint exists but was not exercised during this capture; it is documented for future map-markers and review-pagination work.
- empty_authenticated_state: User's trips and wishlist were empty during capture, so response shape for populated cards was not directly observed. Schema fields are inferred from Booking.com documentation, community wrappers, and the empty-state markup.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**BookingScraper**](https://github.com/ZoranPandovski/BookingScraper) — Python (96 stars)
- [**booking_scraper**](https://github.com/HexNio/booking_scraper) — Python (36 stars)
- [**booking-reviews-scraper**](https://github.com/sudoknight/booking-reviews-scraper) — Python (28 stars)
- [**hotels_mcp_server**](https://github.com/esakrissa/hotels_mcp_server) — Python (20 stars)
- [**actor-booking-scraper**](https://github.com/dtrungtin/actor-booking-scraper) — JavaScript (17 stars)
- [**Booking.com-python-api-spider**](https://github.com/avkaz/Booking.com-python-api-spider) — Python
- [**booking-mcp-server**](https://github.com/EmilyThaHuman/booking-mcp-server) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
