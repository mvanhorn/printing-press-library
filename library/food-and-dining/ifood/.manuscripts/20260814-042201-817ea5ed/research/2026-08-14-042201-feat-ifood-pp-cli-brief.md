# iFood CLI Research Brief

## API Identity

iFood's consumer grocery surface is a private web API behind browser authentication and anti-automation controls. The source specification is a sanitized contract derived from successful authenticated web traffic and intentionally excludes checkout, payment, and order submission.

## Users

- AI agents comparing a complete grocery list across nearby markets.
- Users who want price and delivery-fee visibility before approving cart changes.
- Developers testing typed discovery and preview-first cart composition against controlled sessions.

## Top Workflows

1. Confirm the signed-in browser session and delivery location.
2. Find at least three markets with visible ratings of 4.5 or higher.
3. Search every requested item inside every candidate market and record addable products and visible prices.
4. Validate coverage and select the lowest complete estimate, including known delivery fees.
5. Generate the exact cart plan, obtain approval, then add products through the browser and verify the cart.

## Table Stakes

- Stable JSON output and machine-readable errors.
- Explicit market rating and completeness requirements.
- Dry-run or local planning before all cart writes.
- No checkout, payment, or order submission.
- No persisted browser credentials, cookies, addresses, or payment data.

## Data Layer

Live market, price, availability, and delivery information remains authoritative in iFood. The CLI's local store is limited to framework learning and receipts; browser observations are explicit input documents rather than a hidden session cache.

## User Vision

An AI agent should be able to quote the original six-item list across at least three highly rated markets and prepare a complete cart while keeping authentication inside the browser and stopping for confirmation before the first cart mutation.

## Source Priority

1. Visible signed-in iFood browser state.
2. Sanitized captured request/response contracts represented in the OpenAPI source and fixtures.
3. Deterministic local validation rules.

## Product Thesis

The useful product is not another fragile cookie-export wrapper. It is a safe coordination layer: the browser owns session truth, while the CLI owns deterministic requirements, validation, selection, and confirmation boundaries.

## Build Priorities

1. Preserve the Browser-backed `plan`, `schema`, `validate-quote`, and `cart-plan` surface.
2. Preserve preview-first direct quotation and cart commands for controlled environments.
3. Keep MCP stdio-only so credentials are not exposed through an unauthenticated network listener.
4. Retain fixture-backed tests for completeness, credential rejection, selection, and write gating.
