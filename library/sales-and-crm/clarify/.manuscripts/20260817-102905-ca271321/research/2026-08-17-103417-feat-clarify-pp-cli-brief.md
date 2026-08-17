# Clarify CLI Brief

## API Identity
- Domain: AI-native / autonomous CRM (clarify.ai) for founder-led startups and GTM teams. Auto-builds CRM from email, calendar, and meetings.
- Users: founders, sales reps, RevOps/GTM engineers, and AI agents (Clarify explicitly markets an agent-first API).
- Data profile: JSON:API envelope over a generic object/record model. Built-in objects: person, company, deal, meeting, task. Custom objects (`c_` prefix). Workspace-slug-scoped (`/v1/workspaces/{slug}/...`). Collection fields use `{ "items": [...] }`. Meetings carry recordings + transcripts. Dynamic lists are SQL-driven saved views. Workflows are when-X-do-Y automations.

## Reachability Risk
- None. Official public API, launched publicly Jan 2026 ("Clarify's external API is now public"). Probe of `https://api.clarify.ai/v1` returns structured JSON:API errors (no WAF/bot protection). Full 200 probe pending workspace slug + key (Phase 1.9 completes at gate).
- Spec: official OpenAPI 3.1 at `https://api.clarify.ai/swagger-json` — 48 paths, 75 operations (24 GET / 25 POST / 11 PATCH / 2 PUT / 13 DELETE). Same contract that powers the API Reference.

## Auth
- API key in `Authorization: api-key <key>` — NOT Bearer. Personal keys (full user access) vs Workspace keys (public data only). Docs explicitly warn agents default to Bearer and break.
- Canonical env var: none documented; choose `CLARIFY_API_KEY`.
- Client must prepend the `api-key ` scheme prefix to the raw key.
- Workspace slug is required on every path → CLI needs a persistent `--workspace` config + `CLARIFY_WORKSPACE` env.
- OAuth 2.0 exists for partner apps only (registration via support email) — out of CLI scope.

## Rate limits
- 3,000 req/min per workspace per endpoint, rolling 60s. `X-RateLimit-*` headers + `Retry-After` on 429. Bulk endpoints recommended for imports.

## Top Workflows
1. Query/filter records: "open deals over $50k", "people at acme.com" — `filter[field][Operator]=value` syntax with 8 operator families + shorthand (`>`, `*x*`, `!=`, `null`).
2. Create/upsert records during lead capture — `match_on` upsert against unique fields (person: email_addresses, company: domains, deal: name).
3. Bulk import (CSV of leads → person records) via `/records/bulk`.
4. Meeting intelligence: pull transcripts and recordings for prep/follow-up.
5. Pipeline management: lists (incl. SQL-driven dynamic lists), record activities timeline, comments, workflows.

## Table Stakes
- Record CRUD + resources query for every object incl. custom objects (the MCP server's whole surface)
- Filtering with the full operator table; `include` for relationship expansion; JSON:API pagination
- Schema introspection (fields, types, relationships) — docs say "read the schema first"
- Bulk create/update; list CRUD + publish/unpublish + CSV rows export
- Comments CRUD; activities feed; users; settings; workflows CRUD; campaign recipients; meeting transcript/recordings; record merges; access grants

## Data Layer
- Primary entities: person, company, deal, meeting, task records (+ custom objects discovered from schema), users, lists, workflows, schemas.
- Sync cursor: JSON:API pagination links; records carry `_created_at`/`_updated_at` style system fields (dynamic-list SQL references `_created_at`).
- FTS/search: names, emails, domains, deal names, transcript text (meeting transcripts are high-value FTS content no other tool indexes locally).

## Codebase Intelligence
- Source: clarifyhq/skills (official agent skill) — confirms auth scheme, JSON:API envelope, collection-field `{items:[...]}` gotcha, `--globoff` curl gotcha.
- @getclarify/nodejs-sdk v0.0.5 — very early, thin TS SDK; no meaningful command vocabulary to absorb beyond endpoints.
- Official hosted MCP server at `https://api.clarify.ai/mcp` (closed source; Personal keys only, `X-Clarify-Workspace` header). Capabilities per marketing page: retrieve/create/update records, NL analysis, create tasks and lists, pipeline insights.
- No competing CLI exists on GitHub or npm. Greenfield.

## Product Thesis
- Name: Clarify
- Why it should exist: Clarify is agent-first but terminal-last — the only programmatic surfaces are a hosted MCP (online-only, NL-shaped, no offline state) and raw curl. A printed CLI gives typed commands for all 75 operations, handles the `api-key` scheme and `{items:[...]}`/`--globoff` gotchas natively, and adds what no Clarify surface has: a local SQLite mirror for offline pipeline analytics, transcript FTS, and cross-record joins the API itself can't express (OR-across-fields is unsupported upstream; local SQL does it trivially).

## Build Priorities
1. Records/resources CRUD + query with full filter-operator support, `include`, upsert (`match_on`), for built-in and custom objects
2. Sync → local SQLite for person/company/deal/meeting/task (+ schema-discovered custom objects); FTS over names/emails/domains/transcripts
3. Bulk import (CSV → records/bulk) and list CSV export
4. Meeting transcripts/recordings retrieval
5. Transcendence: local pipeline analytics (stale deals, velocity), cross-object search, dynamic-list SQL authoring helper
# learn no-entities escape: Clarify records are workspace-scoped user data; there is no global aliasable entity vocabulary to seed
