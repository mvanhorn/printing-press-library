# Research Brief: whoop-pp-cli

## API Identity
- **Vendor:** WHOOP, Inc. (fitness/recovery wearable)
- **API:** WHOOP Developer API v2
- **Base URL:** `https://api.prod.whoop.com/developer`
- **OpenAPI spec:** `https://api.prod.whoop.com/developer/doc/openapi.json` (19 endpoints, downloaded locally)
- **Docs:** https://developer.whoop.com/api/

## Reachability Risk
- Spec is official, hosted, JSON OpenAPI. PASS.
- Auth is OAuth 2.0 authorization code (no Personal API Token surface). Live smoke testing requires browser-based auth flow — out of scope for in-session testing.

## Top Workflows
1. **Daily check-in** — recovery score + sleep + strain for today
2. **Weekly trend** — strain/recovery/sleep over last N weeks
3. **Workout review** — list & inspect workouts, re-tag mislabeled ones
4. **Coach digest** — shareable rolling summary
5. **Bulk export** — full history to CSV/JSON/SQLite for own analytics
6. **Webhook listener** — react to new sleep/workout/recovery events
7. **Recovery↔strain correlation** — personal trends across many cycles
8. **Sleep debt + strain budget** — derived recommendations

## Table Stakes (covered by spec)
- Cycles: list, get, get sleep, get recovery (5 endpoints)
- Recovery: list (1)
- Sleep: list, get (2)
- Workouts: list, get (2)
- User: profile, body measurement, access deletion (3)
- Partner: requisition, service-request, token, results, status (5 — partner-only, MCP-hidden by default)
- V1→V2 activity ID mapping (1)

## Data Layer
- Local SQLite store mirroring all read endpoints
- `sync` populates from cursor-paginated lists (`nextToken`)
- FTS5 over workout sport names + journal-style fields where present
- Tables: cycles, sleeps, recoveries, workouts, profile, body_measurements

## Codebase Intelligence
- Pagination: `nextToken` cursor, default 10 / max 25
- All datetimes ISO 8601 UTC
- Webhook events HMAC-signed (sleep/workout/recovery `updated`/`deleted`)
- 429 rate limiting documented but no published limits

## User Vision
User said "Let's go" — no specific vision overrides. Default to GOAT mode: best-in-class CLI that beats every existing Whoop tool.

## Source Priority
Single source. WHOOP OpenAPI v2 is the canonical primary.

## Product Thesis
The CLI everyone wishes Whoop shipped: agent-native, offline-capable, with derived analytics (strain budget, sleep debt, correlation, trends) that the WHOOP app itself doesn't expose. Beats `totocaster/whoopy` (closest Go competitor) by adding offline analytics, webhook daemon, multi-source merge, and a proper local store.

## Build Priorities
1. Generate from official OpenAPI v2 spec
2. Hide partner endpoints behind `--include-partner` flag
3. Add 8-10 transcendence features (analytics, digest, watch, classifier, debt, budget, diff, correlate, journal-correlate, agent-skill)
4. Local SQLite + sync for offline analytics
5. OAuth helper command (PKCE, refresh, token storage in keychain or file)
