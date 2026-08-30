# Novel Features Brainstorm — bestfoodtrucks

> Subagent: researcher (task_id: ses_fc046d379ffeAcF5yyPMNOG6wu). Full raw output preserved below, followed by the agent's post-processing corrections.

## Customer model

- **Maya, the Office Coordinator**
  - **Today (without this CLI):** Maya maintains a spreadsheet for her office campus ("Playa District"). Every Monday, she manually visits the Best Food Trucks (BFT) website, clicks the link for their campus, and copy-pastes the truck names and times into a Slack announcement to avoid "where do we eat today" complaints. She has no visibility into upcoming menu options, so she can't highlight if a "good" truck is visiting.
  - **Weekly ritual:** She spends 15 minutes each week checking the BFT site for her campus to sync the upcoming schedule with her internal communications.
  - **Frustration:** The manual effort of navigating the BFT site, dealing with their aggressive tracking/ads, and the fact that she has to do this repeatedly for different days instead of getting a clean, one-shot weekly summary.

- **David, the Busy Engineer**
  - **Today (without this CLI):** David eats at the food trucks frequently but forgets what is there until he walks out to the lot. He often finds out his favorite burger truck is there, but he's already packed a lunch. He has no way to track which days his preferred cuisines (e.g., "Tacos", "Korean") are scheduled at his nearby lots.
  - **Weekly ritual:** He checks the local "lot" page once a week to see if there's anything interesting scheduled, but he frequently misses updates if the truck schedule changes mid-week.
  - **Frustration:** The lack of a "notify me when my favorite cuisine is nearby" or "see all taco trucks for the next 7 days across my 3 local lots" feature.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Verdict |
|---|---|---|---|---|---|---|
| 1 | Lot Weekly Summary | `lot schedule --days 7` | Get 7-day schedule for a lot in one table. | Maya | (a) | **Post-cut: duplicates absorbed row 1 (`lot schedule`) — reframed, see below** |
| 2 | Cuisine Alert | `search trucks --cuisine <type>` | Find all shifts for a cuisine at a lot. | David | (a) | Keep |
| 3 | Diet Filter | `shift get --diet <tag>` | Filter shift menu items by dietary tags. | David | (b) | Keep |
| 4 | Market Hotlist | `market trucks --trending` | List trucks with high review counts in a market. | David | (b) | Keep |
| 5 | Price Watch | `shift get --max-price <val>` | Filter shift menu by price. | David | (b) | Keep |
| 6 | Lot Discovery | `lot list --city <name>` | Find lot slugs in a city. | Maya | (c) | Keep |
| 7 | Shift Compare | `shift compare <id1> <id2>` | Compare menus between two shifts. | David | (c) | Keep |
| 8 | Truck Profile | `truck get --id <id>` | Look up truck details and total ratings. | David | (b) | Keep |
| 9 | Order Prep | `shift get --order-ahead` | Identify shifts supporting "order ahead". | Maya | (b) | Cut (auth-gated stub) |
| 10 | Route Planner | `market planner --lots <csv>` | Combine schedules for multiple lots. | Maya | (c) | Keep |
| 11 | CSV Export | `lot schedule --csv` | Export schedule data for reporting. | Maya | (b) | Keep |

## Survivors and kills (subagent output)

### Survivors (as returned)

| Score | Feature | Persona | Buildability Proof | Tag | Long Description |
|---|---|---|---|---|---|
| 9/10 | Lot Weekly Summary | Maya | Synthesizes 7 days of `lot.locationSchedule` into a compact table. | hand-code | none |
| 8/10 | Cuisine Alert | David | Cross-references `Location.menu` items with `FoodType` tags. | hand-code | none |
| 7/10 | Market Hotlist | David | Sorts trucks by `PublicRating.count` from shifted data. | hand-code | none |
| 7/10 | Route Planner | Maya | Aggregates multiple `Lot` schedules into a single cross-lot view. | hand-code | none |

### Killed candidates (as returned)

| Feature | Kill Reason | Closest Surviving Sibling |
|---|---|---|
| Diet Filter | Sibling `Cuisine Alert` provides similar filtering capabilities. | Cuisine Alert |
| Price Watch | Users prioritize cuisine/availability over strict price filtering. | Cuisine Alert |
| Lot Discovery | Thin wrapper over GraphQL query that `lot get` covers well. | Lot Weekly Summary |
| Shift Compare | Too complex for weekly ritual; low utility vs direct shift view. | Lot Weekly Summary |
| Truck Profile | `shift get` already provides truck and rating context. | Market Hotlist |
| Order Prep | Stub functionality, not fully automatable without auth. | Lot Weekly Summary |
| CSV Export | `global flag` --csv already standard in the framework. | Lot Weekly Summary |

## Agent post-processing corrections

1. **"Lot Weekly Summary" duplicated absorbed row 1.** As literally specified (`lot schedule --days 7`), this is the same capability as the absorbed `bestfoodtrucks-pp-cli lot schedule` command, not a novel one. Reframed to **`lot digest <seoName>`** — a Slack/announcement-ready **text digest** of the week's schedule (matching Maya's literal stated frustration: "copy-pastes the truck names and times into a Slack announcement"). This is a genuinely distinct output shape (formatted prose block, not structured JSON/table) from the absorbed `lot schedule` command. Score retained at 9/10 — the underlying user value the subagent identified was correct, only the command shape collided with an absorbed row.
2. **Missing candidate: reverse truck lookup.** The Phase 1 brief explicitly flagged "reverse lookup: where is Truck X scheduled" as a promising transcendence candidate (Top Workflow #4), but the subagent's Pass 2 did not generate it. The agent verified buildability directly against the live API before adding it: `truck(id: Int!): Truck` root query exists, and `Truck.locations.records: [Location]` (note: the collection wraps under a `records` field, not `items`/`nodes`/`edges` — discovered via GraphQL error-message-driven field probing since introspection is disabled) returns EVERY scheduled location for that truck — past and future, across ALL lots the truck rotates through, each with full `lot{id name lotPath}` context, in a single query. Confirmed live: "The Chick Truck" (id 11869) has 28 recorded locations across at least 3 different lots (Playa District, AT&T Los Angeles, Abbot Kinney First Friday). Added as survivor: **`truck schedule <id>`**, score 8/10 — transcendence proof is not a local join (it's one query) but that BFT's own website UI never exposes a truck-centric "everywhere this truck goes" view at all; the lot-centric UI is the only user-facing surface. Buildability: hand-code (thin) — single query, no local synthesis required for the MVP; a name-based lookup (vs. numeric ID) would additionally benefit from local-store truck-name resolution after `sync`, which is a natural `--data-source auto` extension, not a blocker for v1.

## Final transcendence set (5 features going into the absorb manifest)

1. **Digest** — `lot digest <seoName>` — 9/10 — hand-code
2. **Cuisine Alert** — `trucks find --cuisine <type> --lot <seoName>` — 8/10 — hand-code
3. **Truck Schedule (reverse lookup)** — `truck schedule <id>` — 8/10 — hand-code (thin)
4. **Market Hotlist** — `market hotlist <city>` — 7/10 — hand-code — **caveat: needs Phase 3 verification that `PublicRating` aggregates per-truck rather than only per-order; only one `PublicRating:order_880945`-shaped entity was observed in discovery, which suggests ratings may be per-order, not a pre-aggregated truck score. If no truck-level aggregate field exists, this feature computes the aggregate locally from synced order-level ratings instead — same value, different implementation path.**
5. **Route Planner** — `lots digest --lots <csv>` — 7/10 — hand-code
