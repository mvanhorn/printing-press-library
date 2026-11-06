# Circle CLI Brief

## API Identity
- Domain: Circle (circle.so) community platform; Admin API V2 (`https://app.circle.so/api/admin/v2`), official OpenAPI 3.0.1 at `https://api-headless.circle.so/api/admin/v2/swagger.yaml` (881,740 bytes, sha256 prefix 2cc8d8c1bd89, fetched 2026-11-05). 115 paths, 175 operations (68 GET, 45 POST, 28 PUT, 8 PATCH, 26 DELETE).
- Users: community operators and course creators who run paid communities and cohort courses on Circle; their agents.
- Data profile: spaces/space groups, members, member tags, access groups, posts, comments, events + attendees, courses (sections, lessons), forms + submissions, paywalls, subscriptions, charges, live rooms + transcripts, flagged content, workflows.
- Homepage: https://circle.so (brand site; API docs live at api.circle.so).

## Reachability Risk
- Low. `GET /community` returns 200 with the admin token. `Authorization: Token <token>` header.
- Cloudflare 403s the `Python-urllib` User-Agent; `Go-http-client/1.1`, `circle-pp-cli/*`, curl and python-requests UAs get 200. The printed Go client is fine.
- **Monthly quota** (api.circle.so/apis/admin-api/usage-and-limits): Business 5,000 req/month, Scale and Plus Lite 30,000, other Plus 250,000. Rate limit 2,000 req / 5 min / IP. 4xx (except 401) counts toward quota; 5xx does not. Consequence: no pre-read cache auto-refresh, bounded dogfood, sync must be incremental-friendly and page with `per_page=100`.
- The spec declares a second `host` apiKey security scheme on every operation. Live calls work without it; it is dropped in the overlay.

## Top Workflows
1. Stand up and keep the community structure in sync with a declared file (space group + spaces, privacy, type).
2. Author a course as files (module folders, lesson markdown, media) and push/pull it to a Circle course space.
3. Count paid founding members vs a seat cap and answer "did 10 people pay?" from subscriptions/charges.
4. Onboard a batch of people: add member (no email), add to spaces/space groups/access groups, tag.
5. Run a recurring live session (Show & Tell): duplicate an event template on a cadence; after the session, pull transcript + attendees into a recap draft.
6. Morning triage: new members, unreplied Friction Logs posts, flagged content, form submissions, failed charges, cancellations.

## Table Stakes
- Every Admin V2 endpoint reachable (spaces, posts, comments, members, tags, events, courses, forms, paywall reads, subscriptions, charges, live rooms, workflows, segments).
- Absorb iamnortey/circle-mcp (16 tools: list/get spaces, posts, members, comments, topics, community, space groups, search; unanswered-post detection; community health; create/update post).
- Official Circle MCP (`https://app.circle.so/api/mcp`, Business plan+, admin-level, counts against quota) covers whatever the Admin API covers; our edge is offline store, declarative apply, course-as-files, quota-free repeat reads.
- `--json`, `--dry-run`, `--select`, typed exit codes, pagination (`page`/`per_page`≤100, `has_next_page`).

## Data Layer
- Primary entities: spaces, space_groups, community_members, member_tags, posts, comments, events, event_attendees, course_sections, course_lessons, community_member_subscriptions, community_member_charges, flagged_contents, form submissions.
- Sync cursor: page walk with `has_next_page`; most lists sort by created/updated, no server-side `updated_since`, so full walk per resource (small community, cheap).
- FTS/search: posts (name, body), comments (body), members (name, email, headline), lessons (name).

## User Vision
- Source: the maintainer's plan for running a Circle course community from the terminal, so agents can run the community without clicking through the admin UI. Approved novel commands, in order: (1) `spaces apply spaces.yaml`, (2) `course push/pull <folder>`, (3) `seats` + `founding --go-no-go N`, (4) `onboard <email>... / --csv`, (5) `show-tell schedule --every 2w --until DATE` + `show-tell recap <event>`, (6) `inbox --since`, (7) `sync` + SQLite + `sql`/`search`. Member-visible writes default to draft/dry-run; explicit `--publish` / `--send` to act. Env var `CIRCLE_API_TOKEN`.
- Gaps confirmed in the spec: no paywall or coupon create (paywalls: search/publish/archive/delete only; coupons: delete only), no custom domain, lesson progress is write-only (`PUT /course_lesson_progress`, no GET).
- Live state 2026-11-05: a new community with one space group, six spaces (including one event space and one course space), one member, and no tags, paywalls, subscriptions or events.

## Product Thesis
- Name: circle-pp-cli
- Why it should exist: Circle's admin UI and MCP are click- or chat-driven and quota-metered per call. A CLI with a local mirror, declarative structure files, and course-as-files lets an agent run the community repeatably, review diffs before writes, and answer business questions (seats, go/no-go, inbox) offline.

## Build Priorities
1. Generated endpoint surface + auth (`Token` prefix, `CIRCLE_API_TOKEN`) + sync/store.
2. `spaces apply`, `course push/pull`.
3. `seats`/`founding`, `onboard`, `inbox`.
4. `show-tell schedule/recap`.
