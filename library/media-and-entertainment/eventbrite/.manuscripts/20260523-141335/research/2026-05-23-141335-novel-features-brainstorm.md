# Eventbrite Novel-Features Brainstorm (subagent audit trail)

## Customer model

**Persona A — Maya, the multi-event promoter.** Runs a concert/club-night brand putting on 15-25 ticketed shows a quarter across several venues, all under one Eventbrite organization.
- **Today:** Lives in the Eventbrite web dashboard, clicking event-by-event into Reports. To compare how Friday's show sells vs the same week last month, she opens two tabs and eyeballs numbers. No way to ask "which of my 20 live events is selling slowest right now."
- **Weekly ritual:** Monday sales-velocity check across every live event; flags laggards for marketing/discount.
- **Frustration:** Eventbrite killed public event search in 2020, so even her own catalog isn't searchable across events. Per-event reporting hides the cross-event picture she manages by.

**Persona B — Devon, the day-of ops manager.** Works door and back-of-house; owns check-in and capacity.
- **Today:** Uses the Organizer app for scanning, but for a roster, capacity headroom, or "who hasn't checked in" he refreshes the web list and counts manually. Will-call = one attendee at a time.
- **Weekly ritual:** Pre-door roster pull, capacity vs sold, identify comps/VIPs; during door track checked-in vs expected.
- **Frustration:** No fast offline roster he can grep at the door when venue wifi is flaky; capacity-vs-sold is mental subtraction.

**Persona C — Priya, the ticketing agency operator.** Manages Eventbrite ticketing for several client brands, each a separate org.
- **Today:** Logs into each client org separately, exports CSVs, stitches in a spreadsheet. No single pane across orgs.
- **Weekly ritual:** Friday client roll-up: revenue, tickets sold, top events per client org; reconcile discount-code performance.
- **Frustration:** Everything per-org and per-event; no "for org X, total sold and revenue across all live events." Discount-code ROI invisible without manual tagging.

## Candidates (pre-cut)

1. Sales velocity board `sales-velocity` (a/b) — KEEP — events×orders×ticket_classes local, rate-over-time analytics can't do.
2. Stale-since drift `sales-since` (b) — MERGE into #1.
3. Repeat-attendee finder `repeat-attendees` (c) — KEEP — cross-event attendee join, restores dead search.
4. Discount-code performance `discount-performance` (b) — KEEP — discounts×orders, no EB discount-ROI report.
5. Door roster + check-in gap `roster <event>` (a/b) — KEEP — local attendee store, door-shaped output.
6. Capacity headroom rollup `capacity` (b/c) — KEEP — events×ticket_classes/inventory_tiers cross-event.
7. Multi-org client roll-up `org-rollup` (c) — KEEP — cross-org aggregation single pane.
8. Will-call lookup `find-attendee` (b) — KILL — thin rename of `search --type attendees`.
9. Sell-out forecast `forecast` (a) — MERGE into #1.
10. Refund/cancellation rate `refund-rate` (b) — KEEP (provisional) — cross-event order-status aggregation.
11. Pacing vs comparable `compare-events` (c) — KILL — narrow use, QA-heavy baseline.
12. Question-response export `question-answers` (b) — KILL — thin reshape, no join.
13. Revenue reconciliation vs Balance `reconcile-revenue` (c) — KILL — Balance semantics ambiguous/unverifiable.
14. Top-tickets-by-event `top-tickets` (b) — KILL — equals `analytics --group-by event`.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Persona |
|---|---------|---------|-------|--------------|--------------|---------|
| 1 | Sales velocity board (+ sell-out projection + since-window) | `sales-velocity [--since <dur>]` | 8/10 | hand-code | Joins synced events × orders × ticket_classes in SQLite for tickets/day since on-sale, projects sell-out, ranks live events | Maya |
| 2 | Repeat-attendee finder | `repeat-attendees [--min 2]` | 8/10 | hand-code | Cross-event SQLite join over attendees by normalized email/name, counts events-per-fan | Maya, Priya |
| 3 | Discount-code performance | `discount-performance [--event <id>]` | 7/10 | hand-code | Joins discounts × orders for redemptions, discounted gross, % of orders per code | Priya, Maya |
| 4 | Multi-org client roll-up | `org-rollup` | 7/10 | hand-code | Aggregates orders per org into events-count/tickets/gross/top-event single pane | Priya |
| 5 | Door roster + check-in gap | `roster <event>` | 6/10 | hand-code | Local attendee store for one event; checked-in vs not / VIP / comp, door-sorted, offline | Devon |
| 6 | Capacity headroom rollup | `capacity [--org <id>]` | 6/10 | hand-code | Joins events × ticket_classes/inventory_tiers for sold vs total capacity, % remaining across live events | Maya, Devon, Priya |
| 7 | Refund / cancellation rate | `refund-rate [--org <id>]` | 5/10 | hand-code | Aggregates order status across events for refunded/cancelled count, refunded revenue, rate | Maya, Priya |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Stale-since drift (`sales-since`) | Overlaps since-window of sales-velocity. | sales-velocity |
| Sell-out forecast (`forecast`) | Projection column of sales-velocity, not a distinct action. | sales-velocity |
| Will-call lookup (`find-attendee`) | Thin rename of `search "term" --type attendees`. | repeat-attendees |
| Pacing vs comparable (`compare-events`) | Narrow weekly use, QA-heavy baseline. | sales-velocity |
| Question-response export (`question-answers`) | Thin reshape of attendees/questions payloads. | roster |
| Revenue reconciliation vs Balance (`reconcile-revenue`) | Balance gross-vs-payout semantics ambiguous/unverifiable. | org-rollup |
| Top-tickets-by-event (`top-tickets`) | Equals `analytics --type ticket_classes --group-by event`. | sales-velocity |
