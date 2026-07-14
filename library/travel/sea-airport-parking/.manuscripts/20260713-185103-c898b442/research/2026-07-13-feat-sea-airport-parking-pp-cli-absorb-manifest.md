# Absorb Manifest — sea-airport-parking-pp-cli

**Landscape:** No existing CLI/script targets reservesea. The site itself + travel
aggregators define the feature floor. reservesea is the *sole* channel for the
official SEA Floor-4 Reserved product (aggregators list off-airport lots only),
so "absorb" = match the site; "transcend" = range over dates + range over time
using a local quote-snapshot store that exists nowhere else.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Quote price + availability for an entry/exit datetime | reservesea site | sea-airport-parking-pp-cli quote --entry <dt> --exit <dt> | JSON/agent output, persisted to local store, per-date breakdown, sold-out reason |
| 2 | Show the Reserved Parking product (code, id, color) | reservesea booking page | (generated endpoint) parking info | Offline-cacheable, structured |
| 3 | Apply a promo code to a quote | reservesea promocodes field | (behavior in sea-airport-parking-pp-cli quote --promo <code>) | Surfaces discounted vs list price |
| 4 | Per-date (nightly) price breakdown | reservesea embedded quotes JSON | (behavior in sea-airport-parking-pp-cli quote) | Emits the nightly price array as structured rows |
| 5 | Detect sold-out for a date range | reservesea selectProduct HTML | (behavior in sea-airport-parking-pp-cli quote) | available boolean + human reason, typed exit code |
| 6 | Build a booking (guest checkout, no login) | reservesea 5-step flow | sea-airport-parking-pp-cli book --quote | Prefilled guest-checkout handoff; prints the ready URL/params and STOPS before payment (never auto-pays) |

Every absorbed row ships. Rows 1 and 6 are hand-built (custom warm-GET→POST→parse
flow); rows 3-5 are behaviors inside `quote`. Row 2 is the one generator-emitted
endpoint scaffold.

## Transcendence (only possible with our local quote-snapshot store)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Flexible-date cheapest / any-available sweep | sweep --from <d> --to <d> --nights <n> [--any] | hand-code | Fans out the live POST across every entry/exit combo in the window, parses each quote, persists all, returns the cheapest or first available — a query no aggregator can answer for the official garage | Use to search a flexible date window for the best price or any open space. Do NOT use for a single known entry/exit datetime (use 'quote'), and do NOT use it to render a per-entry-date grid (use 'calendar'). |
| 2 | Sold-out watch with notify | watch <range> --interval <dur> [--notify] [--until-price <n>] | hand-code | Re-issues the warm-GET→POST on a loop, detects the SOLD OUT↔available transition, persists each poll, fires a notification on flip — the brief's #1 unmet need, absent from the site and every aggregator | Use to be alerted when a specific sold-out or over-threshold range opens up. One foreground poll loop, not a background daemon. For a one-shot check use 'quote'; to search many ranges use 'sweep'. |
| 3 | Quote-snapshot history | history <range> [--since <d>] | hand-code | Reads persisted rows from the local SQLite quote store; the site keeps zero history and no aggregator records this product's price/availability over time | Use to see the raw recorded quote snapshots for a range. This is the read surface; for the computed first→latest delta use 'drift'. Returns nothing for a range never quoted — run 'quote'/'sweep' first. |
| 4 | Price / availability drift | drift <range> [--since <d>] | hand-code | Aggregates local snapshots into first-vs-latest price, min/max, and sold-out↔available flips — impossible without the time series only this CLI records | Use for the summarized change in a range's price/availability over time. For the full row-by-row list use 'history'. Requires ≥2 stored snapshots. |
| 5 | Entry-date availability calendar | calendar --month <YYYY-MM> --nights <n> | hand-code | Loops the live POST over each candidate entry date for a fixed stay length into an availability+price grid; the single-product domain makes a date grid the natural view | Use to render price + open/sold-out for a fixed-length stay across many entry days. Unlike 'sweep' it does not vary stay length or hunt a single winner — it returns the whole grid. |

**Hand-code count:** 5 transcendence rows (all `hand-code`) + the 2 hand-built
absorbed commands (`quote`, `book`). Generator auto-emits the framework (store,
search, sql, doctor, MCP mirror, agent output) and the one `parking info` endpoint.

## Deferred (not shipping in v1)
- **Booking auto-submit / payment** — quote+handoff only; never auto-pay.
- **Authenticated my-bookings management** — core value is auth-free; secondary.
- **Multi-airport** — same AeroParker engine powers IAH/LAS/ORD/ORF/CLT; an
  `--airport`/host override is a low-cost config surface, not a v1 command.
