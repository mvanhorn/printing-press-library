# Pinterest CLI Brief

## API Identity
- Domain: Pinterest REST API v5 (`https://api.pinterest.com/v5`), official OpenAPI 3.0.3 at github.com/pinterest/api-description (info.version 5.28.0; 189 paths / 266 operations; byte-equivalent to the spec used in the 2026-08-29 print apart from whitespace).
- Users: creators and small-brand social-media operators who run organic Pins and boards and sometimes ads. Here that means two brands, two brands, with ad account <ad-account-id> on Brand A. Agencies and ad buyers make up the secondary audience.
- Data profile: the user's own boards, sections, and Pins (hundreds to low thousands); per-Pin and per-account organic analytics (daily metric series); ad accounts, campaigns, ad groups, and ads with async reporting; catalogs and feeds; audiences and conversions; keyword and trend research.

## Reachability Risk
- Low. The API is official and the spec is maintained by Pinterest. The prior print passed live read-only verification 253/253 against the operator's app.
- Rate limits (raw docs, developers.pinterest.com/docs/reference/rate-limits.md): Trial access allows 1000 requests/day per app across all requests. Standard access allows 100 req/s per user per app, plus per-category limits (e.g. ads_conversions). Response headers report limit, remaining, and reset. Implication: default sync must stay bounded (see the prior patch), and the client must back off on 429.

## Auth
- Spec security schemes:
  - `pinterest_oauth2`: authorizationCode, authorizationUrl https://www.pinterest.com/oauth/, tokenUrl https://api.pinterest.com/v5/oauth/token
  - `client_credentials`: v3 URL
  - `conversion_token`: http bearer
  - `basic`: http basic, used by the oauth/token endpoint itself
- Token exchange and refresh must send client_id:client_secret as HTTP Basic.
- Continuous refresh tokens expire after 60 days and can be refreshed indefinitely. Opt in with a flag on the authorization_code grant. Use the returned expires_in or refresh_token_expires_in values and do not hard-code lifetimes.
- Brand A app: client ID <app-id>, exact registered callback `http://localhost:8875/callback`.
- Env var: the ecosystem convention is `PINTEREST_ACCESS_TOKEN` (pinterest-api-sdk on PyPI, community MCP servers). `PINTEREST_REFRESH_ACCESS_TOKEN`, `PINTEREST_APP_ID`, and `PINTEREST_APP_SECRET` also appear. The prior CLI used the slug-inferred `PINTEREST_OAUTH2`. **Enrichment accepted:** make `PINTEREST_ACCESS_TOKEN` canonical and keep `PINTEREST_OAUTH2` as a recognized alias, because the local pp-pinterest skill documents it. `PINTEREST_CLIENT_ID` and `PINTEREST_CLIENT_SECRET` are login-time inputs, not per-call credentials.

## Top Workflows
1. **Organic performance review:** account analytics plus top Pins over a date range for each brand, comparing this period with the last. Today that takes several calls plus manual date math.
2. **Board and Pin inventory:** list every board, section, and Pin, find stale or underperforming Pins, and find Pins missing links, alt text, or titles.
3. **Publish or schedule a Pin safely:** create a Pin on a board with a link, image URL, and alt text, with a dry-run preview first.
4. **Keyword and trend research:** trending keywords by region and interest to plan content (trends and keywords endpoints).
5. **Ads reporting:** campaign, ad group, and ad metrics for ad account <ad-account-id>, including async report creation and polling.

## Table Stakes (from MCP servers and SDKs)
- list/get/create/update/delete for Pins and boards, board sections, and board Pins
- user account info and analytics, Pin analytics
- search the user's own Pins and boards
- token refresh, bulk create with rate-limit respect (clugtu/pinterest-mcp)
- Streamable HTTP MCP transport (sinalkar fork, a Glama read-only server)

## Data Layer
- Primary entities: boards, board_sections, pins, ad_accounts, campaigns, plus analytics snapshots keyed by (entity, date, metric).
- Sync cursor: Pinterest list endpoints use `bookmark` pagination. Default `sync` stays bounded to the user's Pins and boards (prior patch: no fan-out across all parameterized endpoints).
- FTS/search: Pin title, description, alt_text, link, and board name, for offline "find my Pin about X".

## User Vision
- This is a reprint of the local, private `pinterest-pp-cli` under Printing Press 4.33.8, with code generation offloaded to Codex. The CLI is vendored by the local pp-pinterest skill and is not for the public library.
- Preserve the hard-won truths from the prior patches: Basic-auth token exchange, the exact localhost callback, continuous refresh, bounded default sync, and optional env vars that are not credentials.
- Live tests are read-only. Never create or modify Pins or boards on the live Brand A or Brand B accounts.

## Product Thesis
- Name: pinterest-pp-cli ("Pinterest Pro CLI")
- Why it should exist: no existing tool gives an agent-safe, complete v5 surface with offline Pin search, cross-brand analytics rollups, and correct OAuth continuous refresh. The community MCP servers cover 9–11 tools. This CLI covers all 266 operations plus workflow commands.

## Build Priorities
1. Correct auth: OAuth login with Basic-auth exchange, continuous refresh, canonical `PINTEREST_ACCESS_TOKEN` env var plus `PINTEREST_OAUTH2` alias, and a doctor that does not flag optional vars.
2. Bounded sync of boards, sections, and Pins into the local store, with FTS search.
3. Analytics workflows: an organic performance summary, top Pins, and Pin hygiene audit (missing alt text, links, or titles).
4. Safe write paths: Pin create/update with dry-run.
5. Complete MCP surface with clear tool descriptions.

## Pin Destinations
- https://example.com/ (Brand A), https://example.org/ (Brand B) — see documented-urls.txt
