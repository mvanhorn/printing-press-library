# Best Food Trucks (bestfoodtrucks) — Absorb Manifest

## Ecosystem search (Step 1.5a)

Zero existing tools found. GitHub repository search for "bestfoodtrucks": `total_count: 0`. npm registry search for "bestfoodtrucks": `total: 0`. No MCP servers, Claude skills, competing CLIs, or SDK wrappers exist anywhere. This CLI absorbs BFT's own web UI surface — there is no third-party donor to beat.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | View a lot's schedule (today/tomorrow/upcoming dates/full schedule) | BFT web UI (`/lots/<seoName>`) | bestfoodtrucks-pp-cli lot schedule | Offline after sync, `--json`, composable with jq, no bot-checkpoint browser needed |
| 2 | Look up a lot's identity/address/social links by slug | BFT web UI | bestfoodtrucks-pp-cli lot get | Structured output, local cache |
| 3 | View a scheduled shift's menu, hours, truck, ratings | BFT web UI (`/shifts/<id>`) | bestfoodtrucks-pp-cli shift get | Full item list with prices/tags in one structured call, no ads/tracking scripts |
| 4 | Browse trucks/lots by city or market | BFT web UI (`/food-trucks/<city>`) | bestfoodtrucks-pp-cli market list | Offline browse, scriptable |
| 5 | Menu items with prices, cuisine tags, dietary tags | BFT web UI | (behavior in bestfoodtrucks-pp-cli shift get) | Structured price/tag data extracted from GraphQL, not scraped from rendered HTML |
| 6 | Rating/review count for a truck | BFT web UI | (behavior in bestfoodtrucks-pp-cli shift get) | Included in structured shift output |
| 7 | Mobile app parity (BFT has native iOS/Android apps: `com.bftcustomers`) | BFT App Store / Play Store apps | (behavior in bestfoodtrucks-pp-cli lot/shift/market commands — same backend GraphQL API) | CLI/agent-native equivalent to the mobile app's read surface, no app install |
| 8 | Subscribe to a lot's schedule (email/notification opt-in) | BFT web UI (`/customers/login?subscribe_to_lot_id=`) | (stub - requires customer login/session; user declined auth discovery this run since core workflows are fully public) | n/a |
| 9 | Order-ahead / cart checkout | BFT web UI ("exclusive order ahead technology") | (stub - requires customer login + payment, out of automation scope) | n/a |

Stub rows (8, 9) are explicitly called out for user approval at the Phase Gate below — no other row ships as a stub.

## Transcendence (only possible with our approach)

Produced via mandatory subagent brainstorm (customer model → candidate generation → adversarial cut), with two agent corrections documented in full in `2026-08-26-125727-novel-features-brainstorm.md`: one candidate reframed to avoid duplicating an absorbed row, one candidate added after the agent verified a Phase-1-flagged workflow was live-buildable (confirmed directly against the API, not assumed).

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Digest | `lot digest <seoName>` | hand-code | Synthesizes the lot's multi-day schedule into ready-to-paste announcement text — the exact artifact office coordinators currently hand-build by copy-pasting from the website. The absorbed `lot schedule` command returns structured data; this returns prose. | none |
| 2 | Cuisine Alert | `trucks find --cuisine <type> --lot <seoName>` | hand-code | Cross-references each upcoming shift's menu → FoodType tags across a schedule window. No single GraphQL query filters the schedule by cuisine; the API only returns cuisine tags nested under each shift's menu, one shift at a time. | none |
| 3 | Truck Schedule (reverse lookup) | `truck schedule <id>` | hand-code (thin — single query, no local join required for v1) | BFT's own website has zero truck-centric UI — every page is lot-centric. `truck(id).locations.records` returns every past/future scheduled location for a truck across every lot it rotates through, but no page on bestfoodtrucks.com surfaces this view to a human. Confirmed live: "The Chick Truck" has 28 recorded locations across 3+ distinct lots. | none |
| 4 | Market Hotlist | `market hotlist <city>` | hand-code | Ranks trucks operating in a market by review signal — a cross-truck aggregate the site never computes or displays. **Caveat:** discovery observed `PublicRating` shaped as one-per-order (`PublicRating:order_880945`), not a pre-aggregated truck score. Phase 3 must confirm whether a truck-level aggregate field exists on the live schema; if not, this command computes the aggregate locally from synced order-level ratings — same feature, different data path. | none |
| 5 | Route Planner | `lots digest --lots <csv>` | hand-code | Aggregates multiple lots' schedules into one cross-lot view in a single command — the site requires visiting each lot's page separately with no combined view. | none |

Minimum 5 transcendence features: met (5).

## Reachability & Runtime Summary (informs Phase 1.9 / Phase 2)

- Auth: `none` for the entire absorbed + transcendence surface (anonymous `currentCustomer: null` confirmed). Stubs (subscribe, order-ahead) are the only auth-gated capabilities, and they are NOT shipping scope this run.
- Transport: standard `net/http` POST to `https://api.bestfoodtrucks.com/graphql`. No Surf/Chrome-TLS-fingerprint transport needed — confirmed via direct `curl` with zero special headers returning 200 JSON. (The Vercel bot-checkpoint only guards `www.bestfoodtrucks.com` HTML pages, which the CLI does not need to fetch at all now that the GraphQL API is confirmed directly callable.)
- Protocol: GraphQL with Apollo Automatic Persisted Queries observed client-side; the CLI should always send full query text (skip hash-first optimization) to avoid depending on the server's persisted-query cache.
