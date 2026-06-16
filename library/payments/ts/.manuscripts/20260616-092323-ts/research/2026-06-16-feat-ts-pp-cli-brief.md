# TreasurySpring CLI Brief

## API Identity
- **Domain:** TreasurySpring Public API v0.6.20 (OpenAPI 3.1.0). Fixed-Term Fund (FTF) cash-investment platform for corporate/institutional treasury. Entities (legal investing entities) subscribe to FTFs (issued via ring-fenced "cells") that give regulated, diversified access to bank deposits, government and corporate debt.
- **Base URLs:** prod `https://api.treasuryspring.com/api/v1`, sandbox `https://api.sandbox.treasuryspring.com/api/v1`.
- **Users:** treasury teams, finance ops, and the systems/agents acting for them. Read-heavy: monitoring live positions, checking available products + yields, tracking lifecycle events, managing maturity rollovers.
- **Data profile:** entity-scoped. Most reads require an `entity_code`. Strong relational shape: entity → indications/subscriptions/holdings; holding → cell, obligor-exposure; events stream over all of it.

## Auth (critical)
- **OAuth2 client-credentials.** `POST /oauth/token` with HTTP Basic (`client_id:client_secret`, `application/x-www-form-urlencoded`) → bearer access token. Every other endpoint uses `Authorization: Bearer <token>`.
- Spec declares `basicAuth` (token mint) + `bearerAuth` (all calls). No `oauth2` flow block in the raw spec — must enrich before generation so the CLI emits the token-exchange flow.
- **Env vars (canonical):** `TS_CLIENT_ID` + `TS_CLIENT_SECRET`. Also accept a pre-minted `TS_BEARER_TOKEN` for users who mint tokens elsewhere. The user's brief said `TS_API_KEY`; real model is client-credentials — keep `TS_API_KEY` as a fallback alias for the bearer token only.

## Reachability Risk
- **None.** `GET /health` returns HTTP 400 `{"error":"Authorization field missing"}` — server reachable, responding, auth simply not supplied. Standard documented cloud API; no bot-protection, no community 403 reports.
- Probe-safe endpoint: `GET /health` (no params).

## API Surface (20 paths)
**Reads (core, entity-scoped):**
- `GET /entity`, `GET /entity/{code}`, `GET /entity/{code}/permissions`
- `GET /cell/{code}` — FTF cell detail (+ documents)
- `GET /indication/{code}` — available products + pricing/yield for an entity (entity-specific)
- `GET /holding`, `GET /holding/{entity_code}/{holding_uid}` — live positions (source of truth)
- `GET /holding/{entity_code}/{holding_uid}/maturity-action` — current maturity instruction
- `GET /subscription` — orders / workflow history
- `GET /obligor-exposure/{code}` — credit exposure by obligor
- `GET /event`, `GET /event/checkpoint`, `GET /event/checkpoint/{name}` — lifecycle event stream + cursor bookmarks
- `GET /holidays/{year}` — settlement calendar
- `GET /task`, `GET /task/{uid}` — pending tasks

**Writes (sensitive — surface gated behind approval):**
- `POST /subscribe` — subscribe to an FTF (commits capital) — HIGH RISK
- `PUT /holding/.../maturity-action` — set rollover/redeem at maturity — HIGH RISK
- `POST /task/{uid}` — submit a task
- `PUT/PATCH/DELETE /event/checkpoint/{name}` — manage event cursors (low risk, client-side bookmarks)
- `POST/DELETE /webhook` — register/deregister webhook notifications (low risk)

## Existing tooling
- This session has a `prod-public-api` MCP (get_entities, get_indications, get_holdings, get_holding, get_subscriptions, get_obligor_exposure, get_cells, get_entity, get_indication) — read-only mirror of the GET surface. The CLI complements it: standalone binary, local SQLite mirror, offline + compound queries, and (optionally) the write surface the MCP deliberately omits.
- No public third-party CLI / SDK / community wrapper expected (niche B2B fintech) — ecosystem search confirms scope.

## Data Layer (local SQLite mirror)
- **Primary entities to mirror:** entities, indications, holdings, subscriptions, obligor-exposures, cells, events.
- **Sync cursor:** the `/event` stream + `/event/checkpoint` is the natural incremental-sync mechanism — mirror events by checkpoint, derive position/lifecycle changes locally.
- **FTS/search:** across cells (product names/issuers), entities, indications.
- **Why local:** the API is entity-scoped and single-resource-per-call; the value is *joining across* entities/holdings/indications/obligor-exposure locally — concentration, maturity ladders, cross-entity rollups — which no single API call returns.

## Top Workflows
1. "What's maturing and what should roll?" — holdings by maturity date + current maturity-action, across all entities.
2. "What can I buy right now and at what yield?" — indications across entities, filtered by currency/term/yield.
3. "What's my credit concentration?" — obligor-exposure aggregated across holdings/entities.
4. "What changed?" — event stream since last checkpoint (issued/redeemed/extended/cash-moved).
5. "Show my live book" — holdings rollup by entity/currency/maturity with yields.

## Product Thesis
- **Name:** `ts` — the TreasurySpring CLI.
- **Why it should exist:** agent-native, scriptable access to a treasury book that today lives behind a portal and a per-call API. Local mirror turns single-resource calls into portfolio-level questions (maturity ladders, concentration, cross-entity rollups) answerable offline in milliseconds, with `--json`/`--select` for agents.

## Build Priorities
1. Auth (OAuth2 client-credentials token exchange + `auth login`/`doctor`) — nothing works without it.
2. Read surface as typed commands + local SQLite mirror + `sync` (event-driven) + FTS search.
3. Transcendence commands: maturity ladder, concentration/obligor rollup, indications screener, event "since checkpoint" diff, cross-entity book view.
4. Write surface (subscribe, maturity-action) — only if user approves; always `--dry-run` default + explicit confirm.
