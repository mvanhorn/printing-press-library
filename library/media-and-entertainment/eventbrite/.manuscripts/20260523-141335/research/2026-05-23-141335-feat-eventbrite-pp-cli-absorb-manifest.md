# Eventbrite CLI Absorb Manifest

Sources surveyed: Eventbrite official SDKs (Python/JS/PHP), GearPlug community wrapper, and 5 MCP servers (joshuachestang, ibraheem4/lucitra, punkpeye, vishalsachdev, prmichaelsen). No competing Go/agent CLI exists (`brite-cli` is Eventbrite's internal dev-experience monorepo tool, not an API CLI).

## Absorbed (match or beat everything that exists)

All absorbed features are generator-emitted endpoint mirrors over the OpenAPI 3.0 spec (33 resource groups / ~100 operations), each gaining `--json`/`--select`/`--dry-run`/typed exit codes + local-store caching for free. Added value across the board: agent-native output, offline persistence, composability.

| # | Feature | Best Source | Our Implementation |
|---|---------|------------|--------------------|
| 1 | Create an event | joshuachestang MCP `create_event` | (generated endpoint) organizations events.create |
| 2 | List events by organization | all 5 MCP servers `list_events` | (generated endpoint) organizations events.list |
| 3 | Get event by ID | MCP `get_event` | (generated endpoint) events.get |
| 4 | Update event | MCP `update_event` | (generated endpoint) events.update |
| 5 | Publish / unpublish event | MCP `publish_event` | (generated endpoint) events.publish |
| 6 | Cancel / copy / delete event | MCP `cancel_event` | (generated endpoint) events.cancel |
| 7 | List events by venue / series | SDK | (generated endpoint) venues events.list |
| 8 | List attendees by event / org | ibraheem4 MCP attendees | (generated endpoint) events attendees.list |
| 9 | Get attendee | ibraheem4 MCP | (generated endpoint) events attendees.get |
| 10 | List orders by event / org | MCP orders | (generated endpoint) events orders.list |
| 11 | Get order | MCP | (generated endpoint) orders.get |
| 12 | Ticket classes CRUD | SDK | (generated endpoint) events ticket-classes.create |
| 13 | Ticket groups | SDK | (generated endpoint) ticket-groups.list |
| 14 | Discounts CRUD + search by org | SDK | (generated endpoint) discounts.create |
| 15 | Inventory tiers CRUD | SDK | (generated endpoint) events inventory-tiers.create |
| 16 | Pricing | SDK | (generated endpoint) events pricing.get |
| 17 | Venues create/get + list by org | MCP `create_venue` | (generated endpoint) venues.create |
| 18 | Categories / subcategories list | MCP `list_categories` | (generated endpoint) categories.list |
| 19 | Formats list | SDK | (generated endpoint) formats.list |
| 20 | Reports (sales / attendees) | (novel to API) | (generated endpoint) reports.get |
| 21 | Webhooks CRUD | SDK | (generated endpoint) webhooks.create |
| 22 | Organizations + members + roles | SDK | (generated endpoint) organizations.list |
| 23 | Custom order questions | SDK | (generated endpoint) events questions.list |
| 24 | Event teams / capacity / schedule / description | SDK | (generated endpoint) events teams.list |
| 25 | Seat maps / structured content / media | SDK | (generated endpoint) seat-maps.list |
| 26 | Display settings / texts overrides / ticket-buyer settings | SDK | (generated endpoint) events display-settings.get |
| 27 | Current user (me) | SDK | (generated endpoint) user.get |
| 28 | User balance | SDK | (generated endpoint) balance.get |
| 29 | Offline full-text search over synced data | (framework) | (behavior in eventbrite-pp-cli search) |
| 30 | Ad-hoc SQL over local store | (framework) | (behavior in eventbrite-pp-cli sql) |
| 31 | Aggregations over synced data | (framework) | (behavior in eventbrite-pp-cli analytics) |
| 32 | Continuation-token sync into SQLite | (framework) | (behavior in eventbrite-pp-cli sync) |

**Excluded / marked dead:** `GET /events/search/` (public Event Search) — removed by Eventbrite Dec 2019 / fully denied Feb 20 2020. Will be dropped or marked deprecated (it is the API's `Event Search` group); not surfaced as a usable command. The whole point of the transcendence layer is to restore cross-event search over the organizer's *own* synced data instead.

## Transcendence (only possible with our approach)

All 7 are hand-code (require local SQLite joins / cross-entity synthesis beyond generator emit). Source of truth for the Phase Gate 1.5 hand-code count.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|------------------------|
| 1 | Sales velocity board | `sales-velocity` | hand-code | 8/10 | Joins synced events × orders × ticket_classes for tickets-sold-per-day since on-sale + sell-out projection; ranks all live events. No API call / MCP / SDK gives cross-event rate-over-time. |
| 2 | Repeat-attendee finder | `repeat-attendees` | hand-code | 8/10 | Cross-event SQLite join over attendees by normalized email/name. Restores exactly the cross-event search Eventbrite removed in 2020. |
| 3 | Discount-code performance | `discount-performance` | hand-code | 7/10 | Joins discounts × orders for redemptions / discounted gross / % of orders per code. Eventbrite ships no discount-ROI report. |
| 4 | Multi-org client roll-up | `org-rollup` | hand-code | 7/10 | Aggregates orders per organization the token can see into a single events/tickets/gross/top-event pane. Every MCP server is single-org. |
| 5 | Door roster + check-in gap | `roster` | hand-code | 6/10 | Reads local attendee store for one event; checked-in vs not / VIP / comp, door-sorted, offline-capable when venue wifi is flaky. |
| 6 | Capacity headroom rollup | `capacity` | hand-code | 6/10 | Joins events × ticket_classes/inventory_tiers for sold vs total capacity and % remaining across all live events at once. |
| 7 | Refund / cancellation rate | `refund-rate` | hand-code | 5/10 | Aggregates order status across events for refunded/cancelled count, refunded revenue, and rate. |
| 8 | Top buyers by spend | `top-buyers` | hand-code | 7/10 | Joins orders × attendees by email and sums order totals into a cross-event lifetime-value ranking. Ported from DICE FM `fans top` (user request); distinct from repeat-attendees (spend vs event-count). |
| 9 | Attendee contact export | `fan-export` | hand-code | 6/10 | Dedupes attendees across all synced events into one re-marketing contact list, opt-in flagged where present. Ported from DICE FM `fans optin` (user request). |

Customer model + full pre-cut candidate list + killed candidates: see `2026-05-23-141335-novel-features-brainstorm.md`.

## User-requested additions (Phase 1.5 brainstorm)

The user (a multi-platform promoter) asked to cross-pollinate with the DICE FM CLI:
- **Added to Eventbrite (rows 8-9 above):** `top-buyers` (← DICE `fans top`), `fan-export` (← DICE `fans optin`).
- **Suggested DICE amends (for the user to file via `/printing-press-amend dice-fm`):** `discount-performance` (DICE has no promo-ROI command) and `capacity` (DICE has velocity but no cross-event capacity-headroom rollup). Not filed by this run.
- **Cross-CLI cookbook (documented recipe, NOT a baked-in feature):** "Using Eventbrite + DICE together" — combined per-event performance (join by event name+date) and cross-source buyer/loyalty analysis (join by normalized email). Lands in the generated SKILL/README cookbook, since both CLIs emit `--json` + a SQLite store and a hardcoded DICE dependency inside `eventbrite-pp-cli` would violate the one-CLI-wraps-one-API principle.
