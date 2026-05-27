# Dice FM Partners API CLI Brief

## API Identity
- Domain: Live music event ticketing — partners/promoters managing events and fan data
- Users: Event promoters, venues, festivals using Dice FM as their ticketing platform
- Data profile: Events (scheduling, state, genres), Tickets (holder details, pricing), Orders (financial, geographic), Returns, Ticket Transfers, Extras (merch/add-ons), Fans (contact info, opt-in status)
- API type: GraphQL (read-only; 2 root queries: `node` by ID, `viewer` for all partner data)
- Endpoint: `https://partners-endpoint.dice.fm/graphql`

## Reachability Risk
- **Low** — API requires Bearer token auth from MIO (DICE.FM AMP). No reported 403/blocked issues found on GitHub or community channels. Standard authenticated GraphQL endpoint.
- No bot protection on the Partners API endpoint.
- User has no auth credentials — live smoke testing will be skipped.

## Top Workflows

1. **Access management / door check** — Get all current ticket holders for an event with their codes and fan info; identify who has valid tickets vs. returned/transferred
2. **Audience segmentation** — Pull fan demographics (geography, genres, spend) across events for marketing campaigns and Mailchimp/email list building
3. **Financial reporting** — Summarize revenue, commissions, dice fees, and net per event; track refund/return rates
4. **Ticket velocity** — See when tickets sold over time relative to on-sale date; identify fast vs. slow-selling events
5. **Fan intelligence** — Identify repeat buyers, high-value fans, opt-in lists for direct marketing

## Table Stakes (from ecosystem)
- List events with filtering (by date range, state, ID)
- List ticket holders per event with contact info
- List orders per event with financial breakdown
- List returns and refunds per event
- List ticket transfers per event
- Export to JSON/CSV
- Cursor-based pagination support
- Filter by event ID, date range, genre, fan phone, ticket type

## Data Layer
- Primary entities: events, tickets, orders, returns, ticket_transfers, extras, fans
- Sync cursor: `after` cursor from pageInfo + `updatedAt` filters on events/orders
- FTS/search: fan email, event name, fan name for quick lookup
- SQLite store enables cross-event aggregation impossible via single API call

## Codebase Intelligence
- No GitHub SDK repos discovered; Partners API is undocumented outside official SpectaQL docs
- Auth: Bearer token via `Authorization: Bearer <token>` header; env var `DICE_FM_TOKEN`
- Data model: Viewer → connections (events, tickets, orders, returns, transfers, extras); all cursor-paginated
- GraphQL POST to `https://partners-endpoint.dice.fm/graphql`
- Rate limiting: Not documented; assume standard GraphQL rate limiting

## Competitive Landscape
- **Audience Republic integration**: Syncs every 1.5h — pulls events, ticket/order data, fan contacts. Primary real-world consumer of this API. CLI would enable the same workflows on-demand.
- **Apify Dice.fm Scraper**: Public consumer-facing scraper (no auth), not the partners API
- **No competing CLI exists** for the Dice FM Partners GraphQL API

## User Vision
- User has no auth credentials; build without live testing
- Focus on making the CLI usable for querying and exporting data once a token is available

## Source Priority
- Single source: Dice FM Partners GraphQL API
- Spec state: Complete (SpectaQL-generated docs; all types and operations documented)
- Auth: Paid (Bearer token from MIO required for all operations)

## Product Thesis
- Name: `dice-fm-pp-cli`
- Why it should exist: No CLI exists for the Dice FM Partners API. Promoters using Dice FM currently rely on Dice's web dashboard or expensive integrations (Audience Republic, etc.) to access their own ticket and fan data. A CLI gives promoters direct, scriptable access to their event data for access management, audience segmentation, and financial reporting — without needing to build a custom integration or pay for a third-party SaaS.

## Build Priorities
1. **GraphQL client** — Custom HTTP client making authenticated POST requests with GraphQL queries (not REST)
2. **Core sync** — Paginate all entities (events, tickets, orders) into SQLite store
3. **events** commands — list, get, search by date/state
4. **tickets** commands — list by event, filter by holder, access management view
5. **orders** commands — list by event, financial summary
6. **Transcendence** — audience segmentation, revenue analytics, top fans, ticket velocity
