---
name: pp-booking-com
description: "Every Booking.com workflow, plus offline price history, wishlist drop alerts, and multi-leg planning no other... Trigger phrases: `search booking.com for hotels in`, `what's the cheapest week to stay in`, `track price drops on my booking.com wishlist`, `compare these two hotels`, `are any of my booking trips about to lose free cancellation`, `plan a multi-city trip on booking.com`, `use booking-com`, `run booking-com`."
author: "Matt Van Horn"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - booking-com-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/travel/booking-com/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Booking.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `booking-com-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install booking-com --cli-only
   ```
2. Verify: `booking-com-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/booking-com/cmd/booking-com-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent needs to search Booking.com, drill into hotel detail, compare candidates, or work with a user's logged-in Booking account (trips, wishlist, Genius rewards). It is the right choice for cheapest-date sweeps, price-drop tracking, and multi-leg itinerary planning — work that compounds across calls and benefits from local SQLite history. Reach for it instead of generic web-scraping tools because it owns Booking's AWS WAF clearance via Surf+Chrome TLS, the CSRF/cookie composition for /dml/graphql, and the Chrome cookie import for authenticated surfaces.

## Unique Capabilities

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

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-observed traffic context.
- Capture coverage: 1 API entries from 14 total network entries
- Protocols: ssr_html (95% confidence), rest_graphql (70% confidence)
- Auth signals: none; cookie — cookies: aws-waf-token, bkng, bkng_sso_session, cgumid, bkng_prue
- Generation hints: requires_browser_http, requires_browser_cookie_for_auth_endpoints, has_ssr_html_primary_surface, has_csrf_token_for_graphql, supports_offset_pagination
- Candidate command ideas: search — SSR /searchresults.html returns 25 property cards parseable from HTML.; hotel get — SSR /hotel/<country>/<slug>.html returns full hotel JSON-LD.; reviews list — /reviewlist.html exists as a separate paginated surface.; destinations autocomplete — Autocomplete dropdown on homepage fires destination resolver.; trips list — secure.booking.com/mytrips.html loaded with cookie auth and rendered user-specific UI.; wishlist get — www.booking.com/mywishlist.html?wl_id=<id> loaded with cookie auth.; rewards get — secure.booking.com/rewards_and_wallet.html linked from authenticated header.; profile get — secure.booking.com/mysettings.html linked from authenticated header (showed 'Matt Van Horn, Genius Level 3').
- Caveats: ssr_dominant: All observed user-visible pages are server-rendered. Initial searches, hotel detail, trips, and wishlist deliver their data in HTML rather than XHR/GraphQL. The /dml/graphql endpoint exists but was not exercised during this capture; it is documented for future map-markers and review-pagination work.; empty_authenticated_state: User's trips and wishlist were empty during capture, so response shape for populated cards was not directly observed. Schema fields are inferred from Booking.com documentation, community wrappers, and the empty-state markup.

## Command Reference

**account** — Authenticated account/profile page at `secure.booking.com/mysettings.html`. Returns display name, Genius tier, account email (redacted), language, currency, and country.

- `booking-com-pp-cli account` — Read the authenticated user's display name, Genius tier (e.g. Level 3), preferred language, and preferred currency.

**attractions** — Attractions, tours, and experiences at `www.booking.com/attractions/searchresults/<country>/<city>.html`. Returns activity cards with price, duration, rating, and product slug for detail lookups.

- `booking-com-pp-cli attractions get` — Fetch full attraction detail (description, inclusions, duration options, meeting point, cancellation policy, reviews...
- `booking-com-pp-cli attractions search` — Search attractions in a city. Returns SSR-extracted activity cards with price, rating, and product slug. Use...

**cars** — Car rental landing at `www.booking.com/cars/index.html`. Booking.com cars is powered by Rentalcars and uses a self-posting form for search; deep-link results URLs are not supported. This resource exposes the top deals visible on the landing page and supplier landing pages (Hertz, Sixt, Avis, etc.) so agents can recommend pickup locations and known suppliers without claiming results we can't deliver.

- `booking-com-pp-cli cars` — Read the car-rental landing page: featured deals, supported suppliers, and supported city-level pickup locations....

**destinations** — Booking.com destination lookup. Resolves a free-text destination string (city, region, neighborhood, landmark) to a stable `dest_id` and `dest_type` that other commands can use.

- `booking-com-pp-cli destinations` — Trigger Booking.com's destination resolver by posting a search with `ss=<text>` and parsing the destination context...

**flights** — Flights search at `flights.booking.com/flights/<ORIG>-<DEST>/`. Booking's flight search is powered by Etraveli but exposes a clean SSR URL pattern. Returns flight cards with carrier, departure/arrival times, layovers, duration, and price.

- `booking-com-pp-cli flights <origin> <destination>` — Search flights between two IATA airport codes. Returns SSR-extracted flight cards with carrier, times, layovers, and...

**hotels** — Hotel/property search and detail. `hotels list` parses the SSR `/searchresults.html` page (25 cards per page via offset). `hotels get` parses individual `/hotel/<country>/<slug>.html` detail pages including JSON-LD Hotel schema.

- `booking-com-pp-cli hotels get` — Fetch full hotel detail for a given country code + property slug. Parses Booking.com's JSON-LD Hotel schema for...
- `booking-com-pp-cli hotels list` — Search Booking.com hotels by destination + dates + guests + filters. Returns SSR-extracted property cards. Combine...

**map** — Map-view hotel pins for a search result set. Uses Booking.com's internal GraphQL endpoint (`/dml/graphql`, operation `MapMarkersDesktop`) with the CSRF token extracted from the search-results HTML.

- `booking-com-pp-cli map` — Fetch map-marker data for a destination + date range: per-property latitude, longitude, summary price, and rating....

**reviews** — Hotel reviews. Booking.com renders the first review batch in the hotel detail HTML; further pages are at `/reviewlist.html` keyed by hotel slug.

- `booking-com-pp-cli reviews` — Paginated reviews for a hotel. Returns review text, score, traveler type, language, stay date, and reviewer country....

**rewards** — Authenticated Genius loyalty + Rewards Wallet status at `secure.booking.com/rewards_and_wallet.html`. Returns Genius level (1-3), unlocked discount tiers, available credit, and pending vouchers.

- `booking-com-pp-cli rewards` — Get the authenticated user's Genius level, lifetime stays, current credit balance, pending vouchers, and the...

**trips** — Authenticated `My Trips` page at `secure.booking.com/mytrips.html`. Lists upcoming + past bookings with confirmation numbers, check-in/out dates, property name, and total price. Requires cookie import.

- `booking-com-pp-cli trips` — List the authenticated user's upcoming and past Booking.com reservations. Reads from the SSR HTML so it does not...

**wishlist** — Authenticated `Saved` wishlist at `www.booking.com/mywishlist.html?wl_id=<id>`. The user's wishlist id is server-resolved from the session cookie. Returns the saved properties with last-seen price snapshots.

- `booking-com-pp-cli wishlist` — Fetch the authenticated user's wishlist. Returns each saved property's name, slug, country, last-seen price, and the...


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `BOOKING_COM_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

Covered paths:

- `booking-com-pp-cli cars`
- `booking-com-pp-cli trips`
- `booking-com-pp-cli wishlist`

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
booking-com-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Cheapest week in Paris this summer

```bash
booking-com-pp-cli prices cheapest-destination --query Paris --window 2026-06-01..2026-08-31 --nights 7 --max-price 200 --agent --select '[].property_name,[].checkin,[].price'
```

Sweep every 7-night window in summer and return the cheapest property-and-date pairs under $200/night, with --select narrowing the response to the three fields an agent needs.

### Wishlist drop digest for Sunday morning

```bash
booking-com-pp-cli wishlist drops --since 168h --min-pct 5 --agent
```

Walks the user's wishlist and returns only items whose latest price is at least 5 percent below the previous observation in the last week.

### Compare two hotels with detailed deltas

```bash
booking-com-pp-cli compare auliviaopera plazaathenee --checkin 2026-07-15 --checkout 2026-07-18 --agent
```

Side-by-side amenities, price, score, distance, free-cancellation, and recent-review counts for two slug-identified properties.

### Free-cancellation alarm for the week ahead

```bash
booking-com-pp-cli trips deadlines --within 168h --agent
```

Walks authenticated upcoming trips and surfaces those with a free-cancellation deadline in the next week, sorted by urgency.

### Plan a 3-city Italian itinerary against a budget

```bash
booking-com-pp-cli trip plan --leg Rome:2026-07-10:2026-07-13 --leg Florence:2026-07-13:2026-07-16 --leg Venice:2026-07-16:2026-07-20 --budget 1800 --filters free_cancellation,breakfast,score>=8 --agent
```

Searches each leg in parallel and picks the cheapest property combination per leg whose summed nightly totals fit the budget.

## Auth Setup

Public search, hotel detail, reviews, destinations, and map markers run anonymously through Surf with a Chrome TLS fingerprint that clears Booking.com's AWS WAF challenge without a clearance cookie. Authenticated commands (trips, wishlist, rewards, profile) need a logged-in Chrome session: run `booking-com-pp-cli auth login --chrome` once and the CLI imports the booking.com session cookies so subsequent authenticated calls replay through Surf.

Run `booking-com-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  booking-com-pp-cli account --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
booking-com-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
booking-com-pp-cli feedback --stdin < notes.txt
booking-com-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.booking-com-pp-cli/feedback.jsonl`. They are never POSTed unless `BOOKING_COM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BOOKING_COM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
booking-com-pp-cli profile save briefing --json
booking-com-pp-cli --profile briefing account
booking-com-pp-cli profile list --json
booking-com-pp-cli profile show briefing
booking-com-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `booking-com-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add booking-com-pp-mcp -- booking-com-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which booking-com-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   booking-com-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `booking-com-pp-cli <command> --help`.
