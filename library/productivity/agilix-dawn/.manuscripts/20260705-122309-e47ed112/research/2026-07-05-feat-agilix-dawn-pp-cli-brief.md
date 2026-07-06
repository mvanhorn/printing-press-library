# Agilix Dawn CLI Brief

## API Identity
- Domain: Agilix Dawn — modern LMS platform (Agilix Labs). Built from the live tenant
  `drivered.agilixdawn.com` (Idaho Home Driver Education, a parent-led driver's-ed program).
- Backend: same-origin REST BFF at `https://drivered.agilixdawn.com/api/*` (SvelteKit SPA front).
  This is the MODERN Dawn API — NOT the legacy Buzz/DLAP `cmd=` RPC API that every existing
  library (beneggett/agilix, buzzapi, StrongMind) wraps. Greenfield: no Dawn CLI/SDK exists.
- Users: LMS admins/staff (roster, catalog, commerce), instructors, and learners.
- Data profile: courses/content ("concepts") with deep section→instruction→interaction
  structure; users; organizations; Stripe-backed purchases; learner progress; conversations.

## Reachability Risk
- None. Plain HTTPS, standard_http, no Cloudflare/WAF/bot challenge. All probes returned clean 200s.
- Probe-safe endpoint used: `GET /config` (public, 200). `GET /user/me` → 403 without auth (expected).
- No GitHub issues about the Dawn `/api` being blocked/deprecated (it's undocumented, so none exist).

## Auth (verified live)
- Session token in localStorage `session` (32-hex). No auth cookie (only GA).
- Transport: `Authorization: <rawtoken>` header (NO "Bearer " prefix) — verified 200.
  Also `?_authorization=<token>` query param — verified live.
- Durable path: Dawn **API-user** service accounts (`api_` id prefix). Recommended for a CLI.
- CLI auth model: `api_key`, header `Authorization`, raw token, env `AGILIX_DAWN_TOKEN`.
- 403 error envelope: `{description, message, requestId, status}`.

## Confirmed real endpoints (verified via authenticated capture)
Collections — `GET /api/<name>?search=<urlencoded JSON>` → `{totalMatches, matches[]}`:
- `concept` (courses/content — RICH), `user`, `organization`, `purchase`, `progress`,
  `conversation`, `resource` (by-param).
Singletons/by-id: `GET /user/me`, `GET /config`, `GET /concept/{id}` (full course tree).
NOT real (SPA catch-all — prior web research over-inferred): enrollment, course, grade,
certification, certificate, activity, offer, order, report, role, publisher.

## Search DSL
`{query (Lucene), start, limit, sort:[{field:dir}], join:[paths], include:[fields]}`.
The `search={...}` wrapper is MANDATORY — top-level `?limit=2` returns totalMatches but an
EMPTY matches[]. Id prefixes: u_ user, c_ concept, r_ resource, org_ org, api_ API user.

## Top Workflows
1. Browse the catalog: list concepts, inspect a course's full structure (sections → instructions
   → interactions). PRIMARY per user (admin/staff, "course/content browsing").
2. Roster/user lookups: search users, whoami, org membership.
3. Commerce reconciliation: purchases ↔ users (who paid for what; Stripe-backed).
4. Progress/completion tracking (data exists per-learner; empty for admin accounts).
5. Config/tenant introspection.

## Table Stakes
- List/search each real resource with the Dawn search DSL, JSON + human table output,
  offline SQLite mirror, FTS search, --select field narrowing, typed exit codes.

## Data Layer
- Primary entities: concept (+ nested sections/instructions), user, organization, purchase,
  progress, conversation.
- Sync cursor: start/limit offset over the search DSL; `modified` for incremental.
- FTS/search: title + description on concepts; name/email on users; local SQLite.

## User Vision
- Role: Admin/staff. Priority: Course / content browsing.

## Product Thesis
- Name: agilix-dawn (binary `agilix-dawn-pp-cli`).
- Why it should exist: The ONLY tool for the modern Dawn `/api`. Every existing library wraps
  the wrong (legacy Buzz) generation. It turns the Dawn search DSL into ergonomic flags, renders
  a course's full section/instruction tree that the web UI only shows page-by-page, exports
  curriculum + rosters for state reporting, and keeps a local mirror for offline/agent use.

## Build Priorities
1. Foundation: client (raw-token auth), SQLite store for all real entities, sync, FTS search, SQL.
2. Absorb: list/search/get for every real endpoint + config + user/me + doctor.
3. Transcend: course tree, curriculum export, content stats (from instruction durations),
   catalog diff, purchase reconcile, roster export.
