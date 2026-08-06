# Respond.io CLI Brief

## API Identity
- Domain: Customer communication / messaging platform (omnichannel inbox: WhatsApp, Instagram, Facebook, Telegram, Email, etc.)
- Users: Support and sales teams who route, assign, and respond to customer conversations across messaging channels
- Data profile: Contacts, conversations, messages, workspace users, channels, tags, custom fields (moderate volume, relational)

## Reachability Risk
- Low. Official Respond.io TypeScript SDK (respond-io/typescript-sdk) documents a live, stable Unification API v2 at https://api.respond.io/v2 with `Authorization: Bearer <token>` auth.
- API requires a bearer token (Settings > Integrations > Developer API). No token available in env -> live smoke testing skipped (auth_required_no_credential).
- Probe: unauthenticated request to https://api.respond.io/v2 returns 403 (expected when auth required and key declined).

## Top Workflows
1. Get a contact by any identifier (id:, email:, phone:) to answer "who is this and what channels do they use"
2. Send a follow-up message to a contact (text, attachment, WhatsApp template)
3. List + filter contacts, then assign / open / close the conversation
4. Manage tags and custom fields to segment and enrich the CRM/revenue data
5. Understand workspace load: which agents, which channels, which custom-field coverage

## Table Stakes (from official SDK: contacts, messaging, comments, conversations, space)
- contacts: get / create / update / delete / upsert / list / merge / tags / channels / lifecycle
- messaging: send / get / list messages
- conversation: assign / open+close
- comment: create internal comments
- space: users, custom fields, closing notes, channels, WhatsApp templates, workspace tags

## Data Layer
- Primary entities: contact, space_user, space_channel, custom_field, message, tag
- Sync cursor: responder pagination ({items, pagination.next}); contact list via POST body cursorId
- FTS/search: contact free-text offline search; per-resource FTS

## Product Thesis
- Name: respondio-pp-cli
- Why it should exist: no first-class CLI for the Respond.io API exists; a local SQLite mirror lets agents segment, report, and follow up on contacts offline (field gaps, tag cohorts, channel mix, workload by agent) instead of one-off API calls.

## Build Priorities
1. Full resource surface (contacts, messaging, conversation, comment, space) via generated endpoint commands
2. Sync contacts + workspace users + channels + custom fields into SQLite
3. Transcendence: overview (workload), report channel-mix, report workload by agent, contact by-tag, contact field-gaps, contact stale/idle, contact offline search

## Reachability Gate
- Decision: PASS (auth-required-no-credential)
- Evidence: api.respond.io/v2 returns HTTP 403 (CloudFront auth-gate error page) without a bearer token - expected for an API that requires auth. Official respond-io/typescript-sdk uses plain HTTP with `Authorization: Bearer <token>` and confirms the API is reachable with valid credentials.
- Note: probe-reachability returned browser_clearance_http - assessed as a false positive (missing-auth CDN 403, not a bot challenge). CLI ships standard bearer-token HTTP transport; not browser-clearance.
- Live smoke testing skipped (auth_required_no_credential).
