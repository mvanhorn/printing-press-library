# Bonusly CLI Brief

## API Identity
- Domain: employee recognition / peer-to-peer micro-bonus platform (bonus.ly). Users give each other points ("recognition"/"bonuses") with a reason and hashtag; points redeem for rewards. Also covers custom awards/incentives, 1:1 meetings, celebrations, and org-chart data.
- Users: every employee at a company that has adopted Bonusly. Target user for this CLI: a **regular (non-admin) employee**. Concrete user types this CLI is built for:
  - **The individual contributor watching their own allowance.** Bonusly's giving balance is a monthly-refresh allowance that does not roll over — classic "use it or lose it." This person wants to know, a few days before month-end, "how many points do I still have to give, and who have I been meaning to recognize?"
  - **The informal team lead (not a Bonusly admin, but a people-manager or senior IC) tracking their own team's recognition health.** They cannot see the admin Participation Report (that's `recognition:administer`/`reports:administer`-gated), but they CAN read the org chart (`getDirectReports`/`getReportingTree`) and the public feed (`getRecognitionFeed`) — so they want to answer "have I recognized everyone on my team this quarter?" without asking IT for a report.
  - **The person who wants to search their own recognition history instead of scrolling the web feed.** "What did people say when they recognized me for the migration project?" — today this means scrolling `app.bonus.ly`'s feed by hand; there is no non-admin export.
  - **The culture-curious employee tracking company hashtag/values trends.** Bonusly hashtags map to company values; this person wants to know which values are trending without an admin dashboard.
- Data profile: highly relational, cursor-paginated, stateful over time (feed, balances, redemptions, org chart). Excellent fit for a local SQLite mirror — the whole thesis of this CLI depends on it (see Product Thesis).

## Reachability Risk
- **None.** `GET https://bonus.ly/api/v1/users/me` (no token) → HTTP 401, body `{"success":false,"message":"Bad or missing access token"}`. `GET https://bonus.ly/api/public/analytics/recognition_events` (no token) → HTTP 401, body `{"success":false,"message":"Bearer token required"}`. Both are clean JSON error envelopes served behind Cloudflare CDN — no bot-wall, no challenge page, no CAPTCHA. Textbook PASS: endpoint exists, requires auth, auth is absent, server said so cleanly.
- Tier/permission hints from 4xx body: both confirmed bodies share one shape — `{"success": bool, "message": string}`. Use this as the canonical error-unwrap contract for the generated HTTP client.
- Probe-safe endpoint used: `GET /api/v1/users/me` (documented example endpoint from Bonusly's own "Connecting to the Bonusly API" guide).
- No rate-limit headers observed on the unauthenticated 401 responses (no `X-RateLimit-*`, no `Retry-After`). Confirmed absent from all four official docs pages checked. **Rate limits are genuinely undocumented, not just unfound** — the generated client should self-throttle defensively (adaptive backoff on 429) rather than assume any published ceiling.

## Surface Map (this is unusually important for Bonusly — three real surfaces exist, only two are usable)

| Surface | Auth | Scope needed | Usable by a regular employee? |
|---|---|---|---|
| **REST API v1** — `https://bonus.ly/api/v1/*` | PAT, `Authorization: Bearer <token>` (or `?access_token=` fallback) | per-endpoint, `user:read`/`recognition:read`/`recognition:write`/`rewards:read`/`awards:read` etc. | **Yes** — this is the primary target |
| **Analytics API** — `https://bonus.ly/api/public/analytics/*` | Same PAT | `analytics:administer` (admin/Organization-plan scope) | **No.** Confirmed admin-gated. Has the best sync design (async NDJSON snapshot + resumable `{recomputed_at,row_key}` cursor + `tombstone:true` soft-deletes across RecognitionEvents, AnalyticsUsers, group-stats, templates datasets) but it is out of reach for this CLI's target user. Document as a known gap, do not attempt to fake it. |
| **SCIM** — `https://bonus.ly/scim/v2/*` | Same PAT | `user:administer` | No — irrelevant, identity-provider sync only |
| Official MCP server — `https://bonus.ly/mcp` | OAuth (separate from PAT) | 40+ scopes | Reference surface only — see Absorb Manifest. Its tool docs are the best available description of REST v1's shape since there is no REST v1 OpenAPI spec. |

**Auth self-service confirmed three separate times from three separate official docs pages** ("Connecting to the Bonusly API", "Bonusly API access for admins" FAQ, "Moving from Zapier to the Bonusly API"): *"Any Bonusly user can [mint a PAT] here, scoped to what your role allows."* Regular-user tokens are filtered to scopes the UI would already let them exercise; company-wide `administer` scopes are never offered on the personal mint surface (`Settings → Services`). This de-risks the whole project — a non-admin can self-serve a working credential without asking an admin for anything.

**CSRF finding — decides the auth architecture:** *"If you make an API request without a token and rely on your browser session/cookies instead, non-GET requests will fail CSRF verification with 403 Forbidden."* Cookie replay cannot drive mutations regardless of browser session capture. This means composed cookie+PAT auth buys nothing for writes — **PAT-only auth is sufficient and simpler.** The only residual value of browser-sniffing `app.bonus.ly` is discovering additional *read* (GET) endpoints the SPA calls that aren't in the documented tool surface (e.g. leaderboards, trending hashtags) — narrower value than originally scoped. Flagging for the Phase 1.7 gate rather than deciding unilaterally.

**No REST v1 OpenAPI/Postman spec exists.** The legacy Apiary docs (404) were never replaced with a REST-specific reference. `docs.bonus.ly`'s OpenAPI definitions describe the *MCP JSON-RPC* wrapper (`operationId`, request envelope is `{jsonrpc, method: "tools/call", params: {name, arguments}}`), not raw REST paths — but each MCP tool's description explicitly documents the REST path it mirrors (e.g. "Mirrors the `PATCH /api/v1/bonuses/:id` endpoint", "the same shape as the `/api/v1/redemptions/:id` endpoint"). **The MCP tool catalog is the de facto REST v1 API reference** and is what Phase 2 spec-authoring should be built from.

## Top Workflows (non-admin employee)
1. **Give recognition** — `POST /api/v1/bonuses` equivalent (MCP: `giveRecognition`). Recipients + amount + reason + one hashtag.
2. **Check points balance / plan spend before month-end forfeiture** — `getPointsBalance` (giving balance, monthly budget, redeemable balance, lifetime stats). Monthly giving allowances that don't roll over are the single most common recognition-platform complaint pattern industry-wide, and Bonusly's own balance endpoint exposes exactly the fields needed to compute a burn-down.
3. **Browse/search the company recognition feed** — `getRecognitionFeed` (filterable by department/location/team/hashtag/type) and `searchRecognitions` (hybrid semantic+BM25). Richest non-admin-gated dataset in the whole API.
4. **Check who I haven't recognized lately / who hasn't recognized me** — `getRecognitionGivenToUsers`, `getRecognitionGiven`, `getRecognitionReceived`, cross-referenced with the org chart (`getDirectReports`, `getReportingTree`).
5. **Claim an incentive / check my redemptions** — `claimIncentive`, `getMyRedemptions`, `getRedemption`.

## Table Stakes (absorbed from the official MCP + REST v1)
- Give/edit/delete recognition and awards, claim incentives
- Feed browsing with department/location/team/hashtag/type filters, semantic search
- Full org-chart traversal (manager chain, reporting tree, direct reports, top-level users, departments, locations, user groups)
- Points balance, lifetime stats, redemption history
- User lookup by id/email/name with disambiguation

## Data Layer
- Primary entities: `recognitions` (bonus_id, giver_id, receiver_ids[], amount, reason, hashtags[], category, celebration_subtype, privacy, via, recognized_at — field shape confirmed from Bonusly's own Snowflake `RECOGNITION_EVENTS` flattened view, even though that admin-only endpoint itself is out of reach; mirroring its column shape locally from `getRecognitionFeed` data keeps our local schema aligned with Bonusly's own canonical analytical model), `users` (id, email, display_name, department, location, manager_id), `redemptions`, `balances` (point-in-time snapshots — this is what makes burn-down forecasting possible), `departments`/`locations` (with member counts — the denominator for participation math).
- Sync cursor: **no tombstone-aware change-data-capture available to this user tier.** `getRecognitionFeed`'s cursor is a live reverse-chronological pagination cursor, not a resumable "changed since X" cursor — sync must page until it reaches already-seen records (by id or timestamp watermark) and cannot detect deletions. Document this honestly as a known limitation rather than oversell it as equivalent to the admin Analytics API's snapshot+tombstone design.
- FTS/search: local FTS5 over synced recognition messages/reasons — genuinely differentiated since Bonusly's own in-product search is either the paid semantic tool (server-side, metered, requires live call) or nothing for non-admins.

## Codebase Intelligence
- No GitHub repos with meaningful source were found worth DeepWiki analysis — the one real Go CLI (`maxRN/bonusly-cli`) is 0 stars, last pushed 2022, and dead. Skipping per the "trivially simple, skip DeepWiki" carve-out is not quite right (auth is not trivial), but DeepWiki adds nothing when there's no substantive source to analyze.
- **Ground truth from `ajramos/mcp-bonusly` (1-star hobby MCP, Python, live REST v1 client) — read directly, not summarized:**
  - Success envelope: `{"result": <data>}`. Error envelope (independently confirmed via live probe): `{"success": false, "message": "<msg>"}`. Two different shapes for success vs error — the client must branch on HTTP status, not on a `success` field, to detect failure.
  - `GET /api/v1/bonuses` confirmed params: `limit`, `start_time`/`end_time` (bare date auto-suffixed to `T00:00:00Z`/`T23:59:59Z`), `giver_email`, `receiver_email`, `user_email`, `hashtag` (auto-prefixed with `#` if missing), `include_children`. This is **time-range + email filtering, not opaque cursor pagination** — contradicts the assumption that all REST v1 list endpoints share the MCP layer's cursor scheme. Treat each endpoint's pagination shape as independently verified, not inferred from a sibling.
  - `POST /api/v1/bonuses` body is just `{"reason": "...", "giver_email"?, "parent_bonus_id"?}`. **Amount, recipients, and hashtag are NOT separate JSON fields — they are encoded inside the `reason` string via a `+N @mention #hashtag` mini-DSL.** Confirmed independently by the official MCP's `giveRecognition` doc: *"no need to include @mentions or +amount — they're synthesized from recipients and amount."* The generated `give`/`recognition give` command MUST synthesize this string from structured flags server-side-style; never expose the raw DSL as user input.
  - `giver_email` on create is explicitly commented `# admin feature` in the hobbyist's source — giving recognition as someone else needs admin rights; giving as yourself (the default, omitting the field) does not.
  - 429 handling exists in this real client (`BonuslyRateLimitError`) despite no published rate-limit numbers — confirms 429 is a real observed behavior, not theoretical. Canonical env var in the wild: `BONUSLY_API_TOKEN`.

## User Vision
- None volunteered — user chose "Let's go" at briefing. Context gathered instead through the Phase 0 surface/role questions: **REST v1 + authenticated browser-sniff** (surface, pending Phase 1.7 re-confirmation given the CSRF finding above), **regular employee, no admin** (role — this is the single most consequential fact in this brief; it eliminated ~28 of ~66 official tools and reshaped which transcendence features are honest to build).

## Product Thesis
- Name: working title `bonusly` (canonical display name: **Bonusly**).
- Why it should exist: **Bonusly's own documentation describes the feature we'd be building, then admin-gates it.** The vendor ships a genuinely good snapshot+cursor+tombstone Analytics API for warehousing recognition data — but it requires `analytics:administer` (an Organization-plan admin scope). On-demand CSV export is admin-only too. A regular employee has *zero* self-serve export or historical-analysis path today — only the live web feed, one page at a time. Bonusly's own Zapier-deprecation notice tells its users to go build custom API tooling. This CLI is that tooling, aimed at the 99% of Bonusly users who aren't admins: a local mirror of exactly the data a non-admin **can** already read (feed, org chart, balance, own redemptions), turned into the equity/reciprocity/participation analytics that are otherwise locked behind a paid admin report.
- Competitive landscape is close to a blank slate: one dead Go CLI (0 stars, 2022), one 1-star hobby MCP server, one 2015 Node client, one 2020 Hubot integration, zero Claude skills, zero maintained SDKs, and an *officially deprecated* Zapier integration. The bar to "beat everything that exists" is low; the bar to "be honestly useful within non-admin scope constraints" is the real design target.

## Build Priorities
1. Local SQLite mirror of feed + org chart + balance + own redemptions, via `sync`, built on the honest (non-tombstone) incremental-fetch pattern described above.
2. Absorb every non-admin-reachable MCP-documented capability as a real command (give recognition, claim incentive, feed browse/search/filter, org-chart traversal, balance, redemptions).
3. Transcendence features computed entirely from non-admin-readable data: points burn-down forecast, neglected-teammate finder (org chart ⋈ last-recognized), recognition equity/distribution (feed ⋈ department headcounts), reciprocity graph, hashtag/company-values trends — see absorb manifest for scoring and buildability.
