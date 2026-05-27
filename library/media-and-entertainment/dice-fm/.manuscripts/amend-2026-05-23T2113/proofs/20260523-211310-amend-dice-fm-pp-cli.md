---
date: 2026-05-23
target_cli: dice-fm-pp-cli
amend_run_id: amend-2026-05-23T2113
scope_tier: all
findings_count: 2
pr_target: 789
pr_branch: feat/dice-fm
mode: direct
---

# Amend plan: port two eventbrite transcendence commands to dice-fm

Direct-input mode. User asked to port `discount-performance` and `capacity` from
the eventbrite CLI into dice-fm-pp-cli, reading from the local store, and to
update existing open PR #789 (the dice-fm publish PR) rather than open a new PR.

## Data-model reality check

The eventbrite source reads from a generic `resources` table keyed by
`resource_type`, with hierarchical IN-lists. dice-fm uses the same generic
`resources` table but its synced resource types are: `events`, `tickets`,
`orders`, `returns`, `transfers`, `extras`, `genres`, `fans`. Money is integer
cents. Store reads unmarshal the `data` JSON column into typed Go structs (see
`dice_revenue.go` `readOrders`, `dice_velocity.go` `eventOnSale`).

DICE has NO discount/promo/coupon entity anywhere in the synced model. Ticket
`code` is a per-ticket access barcode, not a discount code. The only real
"named pricing lever" DICE exposes is `priceTier { id name price }`, carried on
synced `tickets`.

## F2 — capacity (clean port)

- File: `internal/cli/dice_capacity.go` (+ event store reader)
- Maps eventbrite capacity directly:
  - sold = sum of order `quantity` per event (real store data)
  - capacity = event `totalTicketAllocationQty` (real store data)
  - remaining = capacity - sold
  - pct_sold = sold / capacity * 100, round2
  - exclude non-live events by DICE `state` (live-event filter analog to
    eventbrite's ebIsLiveEvent)
- Flags: `--limit` (cross-event rollup; no org filter — DICE has no org entity
  in the synced model, uses `promoters` instead; omit org filter)
- Sort: pct_sold desc, then event_id asc
- mcp:read-only

## F1 — discount-performance (re-targeted to real priceTier data)

- File: `internal/cli/dice_discount_performance.go` (+ tickets store reader)
- DICE has no discount codes. A literal port would either fabricate data
  (prohibited by anti-reimplementation) or return always-empty rows. Instead the
  command computes per-price-tier redemptions and each tier's share of total
  redemptions from the real `tickets` store — the same analytical intent as the
  eventbrite original (which pricing lever earned what share of sales) over the
  closest real DICE entity.
  - redemptions = count of non-returned tickets at each price tier
  - redemption_rate = tier redemptions / total redemptions (share of sales),
    round4 — NOT an allocation cap ratio, because DICE does not expose a
    per-tier cap
  - price = tier price (cents -> major)
  - exclude returned tickets via the `returns` store (real join, same as door)
- Flags: `--event` (filter by event ID), `--limit`
- Sort: redemptions desc, then tier name asc
- mcp:read-only
- The divergence from the eventbrite original is documented in the command's
  doc comment, the `// PATCH(...)` markers, and `.printing-press-patches.json`.

## Patch contract

- `// PATCH(amend-2026-05-23: ...)` at every new top-level decl.
- New `.printing-press-patches.json` entry with files, findings_addressed,
  patch_count, and a note on the discount-performance semantic divergence.

## Tests

Extend `dice_transcend_test.go` with table-driven tests for computeCapacity and
computeDiscountPerformance using seeded fixtures (RFC2606 example.com emails).

## Risks

- discount-performance semantic divergence: surfaced to the user at the PR
  checkpoint and in the PR body. This is an API-specific adaptation, not a
  machine gap, so no upstream cli-printing-press issue is filed.
