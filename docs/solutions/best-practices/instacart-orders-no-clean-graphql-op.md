# Instacart: no clean GraphQL op for order history

_Finding date: 2026-04-18_

## Summary

The original `pp-instacart` history-sync plan assumed Instacart exposes dedicated GraphQL operations like `BuyItAgainPage` and `CustomerOrderHistory`. **It does not.** The first merged PR (#78) shipped with two placeholder ops that were never callable.

## Why this matters

Future contributors searching for "instacart order history graphql" or "instacart buy it again api" should land on this note and skip the dead-end. The fix is a different data-source strategy, documented in `docs/patterns/authenticated-session-scraping.md`.

## What's actually true

- `/store/account/orders` is **server-rendered** by Instacart's Rails backend. No GraphQL op fires for the orders list.
- Individual order detail pages at `/store/orders/<id>` are also server-rendered, but they hydrate an Apollo client after load. The client runs multiple GraphQL ops including `AssociatedRetailersOrderStatusModal`, `GetRankedOrderItems`, `WhatWentWrongV2`, and `GetNotes` — none of which are a clean "order with items" aggregate.
- The user's full order + items is assembled in Apollo's local cache under the **`OrderManagerOrderDelivery`** key, keyed by `orderId` + a handful of `include*` flags. The variant with `includeOrderItems: true` contains `retailer { id, name, slug }` and `orderItems[]` with full item names and product IDs.
- That cache entry is populated via **multiple** GraphQL ops that Apollo composes client-side. There is no single op you can call with a hash and get the same result.

## How to get the data

Navigate to each `/store/orders/<id>` page in a logged-in browser, wait for Apollo to populate, then read `window.__APOLLO_CLIENT__.cache.extract().OrderManagerOrderDelivery`. The reference implementation lives at `library/commerce/instacart/docs/extract-one.js` and is tier-1 (Chrome MCP driven) per the authenticated-session-scraping playbook.

For end users, the flow is wrapped into the `/pp-instacart` skill: typing "backfill my instacart orders" into a Claude Code session drives the full loop. See `library/commerce/instacart/docs/backfill-walkthrough.md`.

## Multi-profile redirect catch

Plain HTTP scraping of `/store/account/orders` with extracted cookies redirects to `/store/profiles?next=...` because the user has multi-profile on their Instacart account. A selected-profile cookie is required, and it's set via a browser-side flow that doesn't survive plain-cookie extraction. Only a browser tab that has interactively cleared the profile picker can access the orders page.

This is why tier-1 (Chrome MCP) is the only reliable approach for Instacart specifically.

## Remediation

- PR #78 (merged 2026-04-19): shipped the broken sync scaffolding. Everything except `history sync` works — schema, history-first resolver, write-back on add.
- PR that introduced this note: added the `history import` subcommand as the working data-load path, plus the playbook that steers future contributors to the right tier.
- The fictional `BuyItAgainPage` and `CustomerOrderHistory` ops in `library/commerce/instacart/internal/instacart/ops.go` are marked deprecated with a TODO pointing at this note. A follow-up cleanup PR will remove them.

## Similar services to watch for

Based on known architectures, these likely have the same "no clean user-state GraphQL op" pattern (server-rendered, fragmented Apollo cache, require interactive auth):

- **DoorDash**: past orders similarly server-rendered
- **Uber Eats**: past orders hydrate via multiple smaller queries, no dedicated history op
- **Amazon**: orders page is entirely server-rendered HTML

Services with clean user-state APIs (confirmed, not this pattern):

- **Linear**: first-class GraphQL, clean user-scoped queries
- **HubSpot**: documented REST API
- **Stripe**: documented REST API
- **Dub**: clean REST API

When adding a new printed CLI: check the target's DevTools Network tab first. If you see lots of tiny GraphQL ops but no clear "my <thing>" aggregate, you're probably in the Instacart shape.

## Related

- `docs/patterns/authenticated-session-scraping.md` — the playbook born from this finding
- `library/commerce/instacart/docs/dumper.js`, `extract-one.js` — reference implementation
- `library/commerce/instacart/internal/cli/history_import.go` — Go-side importer
- PR #78 merged commit: `docs(instacart): add SKILL.md, surface history in doctor, regen plugin`
