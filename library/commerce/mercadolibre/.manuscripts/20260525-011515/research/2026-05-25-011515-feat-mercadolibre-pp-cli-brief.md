# MercadoLibre CLI Brief

## API Identity

- **Domain:** MercadoLibre — the dominant e-commerce marketplace across Latin America. Operates 11 country sites (Argentina, Brasil, México, Chile, Colombia, Uruguay, Perú, Ecuador, Bolivia, Venezuela, Costa Rica) with ML being the local-currency consumer-marketplace standard in each. As of 2026 the API exposes seller workflows, public catalog (post-2024 gating notwithstanding), location taxonomy, user profiles, Q&A, ratings and order pipelines.
- **Users:** LATAM-resident developers building integrations against the marketplace they actually transact on; LATAM sellers who want to query/automate their own publications; data analysts mining the canonical catalog for pricing intelligence; AI agents that need a token-efficient, predictable interface to ML for use cases like resale opportunity radars, pricing benchmarks for service businesses (event décor, gift baskets, B2B procurement), and competitive intelligence on other sellers.
- **Data profile:** canonical product catalog (10K+ products per common keyword per site), individual marketplace listings (`/items/{id}`), seller profiles + active publications, category taxonomy + required attributes, country and site metadata, Q&A on listings, sales/orders.

## Reachability Risk

- **Low** for hand-authored OpenAPI-style spec: ML's developer portal has public reference pages per resource family (no auto-generated spec, but stable URL patterns since 2018).
- **Low-medium** for the docs portal (`developers.mercadolibre.com.ar/...`): blocks default Go HTTP UA with HTTP 403, but accepts standard Chrome UA. Docs are React-SPA-rendered, so even with a browser UA, the static HTML contains no endpoint info (would need a headless browser to scrape).
- **Medium** for the runtime API: most endpoints require OAuth 2.0 Bearer token (6 h access token, 6 mo refresh). `/sites/{site}/search` (marketplace listing search) was closed to non-certified apps in 2024 and returns HTTP 403 even with a valid token; use `/products/search` (canonical catalog, OAuth-accessible) instead. A handful of geographic/taxonomy endpoints (`/classified_locations/countries`, `/classified_locations/countries/{id}`) remain public — no auth needed.
- **Rate limits:** per-app and per-user; ML documents them at developers.mercadolibre.com/en_us/policies-of-use. CLI default rate-limit (2 req/s) is conservative.

## Top Workflows

1. **Catalog research / pricing benchmark** — given a keyword + site, return up to 200 canonical products with full attributes (brand, model, materials, dimensions, photos). Useful for cost research before pricing a service (e.g., a wedding florist costing centerpieces) or for competitive analysis.
2. **Seller inspection** — given a seller user_id, list their active publications, get profile + reputation, surface what they're offering.
3. **Listing detail pull** — given an item ID (MLA1234567890), fetch full listing data: price, stock, photos, description, seller. Used for monitoring specific products or building one-off integrations.
4. **Q&A management** — list pending questions on the user's own publications; reply via POST `/answers`. Foundation for a future `auto-reply` workflow.
5. **Category exploration** — given a category ID, walk the full ancestor path from root + see required attributes for that category. Useful when programmatically publishing new items.

## User Vision

> "I'm an ambientador floral in Argentina. Before I quote a wedding for 200 guests, I want to know what 60 centros de mesa worth of jarrones, telas y luces actually cost on MLA right now — so I can quote a margin that reflects reality, not last year's estimate. One command from my terminal, JSON out, jq it however I want."
>
> "I run a Skool community of LATAM AI enthusiasts. I want my agents (Claude Code, Codex, whatever) to inspect MercadoLibre as part of larger workflows — cost research, scraping for product imagery to use as references, etc — without writing per-call curl scripts."
>
> Domain-agnostic: works equally for event planners, B2B procurement, dropshipping research, data journalists analyzing LATAM commerce.

## Auth Strategy

**OAuth 2.0 (Bearer) primary; public-path exception for geographic endpoints.**

- Primary: create app at https://developers.mercadolibre.com/devcenter → App ID + Client Secret → Authorization Code flow with `https://httpbin.org/get` redirect_uri (for first-time setup) → exchange code for access_token (6 h) + refresh_token (6 mo). Save with `auth set-token <token>` or `MERCADOLIBRE_ACCESS_TOKEN` env var.
- Exception: `/classified_locations/*` endpoints work without any Authorization header. The CLI's HTTP client (see `internal/client/client.go`, function `isPublicPath`) explicitly skips the Authorization header for these path prefixes so the CLI works for users without a token AND for users whose token has expired — this is the v0.1.1 novel feature documented in `.printing-press-patches.json`.
- Optional future: client-credentials flow for app-only catalog queries (less private, no user context). Not implemented in v0.1.x.

## Novelty vs Alternatives

No CLI exists for MercadoLibre in any ecosystem. All published SDKs are archived (official Node 2020, official Python 2019, community PHP 2022). This is the first CLI and the first PP catalog entry covering LATAM commerce.

Honest novelty score: **3/10**. The CLI is functionally a faithful API wrapper. Three novel features are wired as hidden experimental stubs (`watch`, `compare`, `ml-analytics`) for promotion in v0.2.x. One real client-side novel feature ships in v0.1.1 (public-path auth omission, documented above).
