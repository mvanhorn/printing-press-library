# Tiimo Browser-Sniff Discovery Report

## 1. User Goal Flow

**Primary goal:** *"Plan and run today"* — open today's timeline, inspect activities,
and exercise the write path.

Steps completed (9 of 9):
1. Loaded `webapp.tiimoapp.com` → redirected to `/home`, authenticated session confirmed
2. Profiled resource hosts via Performance API
3. Enumerated API paths (page-load calls)
4. Pulled OIDC discovery document from `auth.tiimoapp.com`
5. Identified the client's token source (`/api/auth/session`, NextAuth)
6. Probed 18 candidate endpoints; recovered required query params from 400 ProblemDetails
7. Retrieved full response schemas for every 200 endpoint
8. **Write test:** created `pp-capture-test` (201), updated it (PUT 200), read it back (GET 200)
9. **Cleanup:** deleted it (204), verified GET → 404 and day count returned to its original value

Steps skipped: none. Secondary flow (day navigation) triggered no new calls — the
client fetches a multi-day window and navigates locally, which is itself a finding.

## 2. Backend Topology

| Host | Role |
|---|---|
| `api.tiimoapp.com` | The data API. ASP.NET Core, REST/JSON, RFC 7807 ProblemDetails |
| `auth.tiimoapp.com` | OIDC provider (Duende/IdentityServer) |
| `ai.tiimoapp.com` | AI "Co-planner" service (observed, not probed) |
| `webapp.tiimoapp.com` | Next.js front end + NextAuth session broker |

Noise excluded: Sentry, Braze, RevenueCat.

## 3. Browser-Sniff Configuration

- **Backend:** Claude chrome-MCP (browser extension), browser `Brave - ZB Laptop`
- **Why not browser-use:** browser-use v0.13.1 ignored `--profile "Default"` and
  launched a throwaway `user-data-dir`, so it had no session and hit the login wall.
  Google Chrome is installed but not running; the live Tiimo session lives in Brave.
  chrome-MCP captures from the real logged-in session with no cookie transfer, and
  avoids OAuth-in-an-automated-browser (Tiimo's login is Apple/Google social).
- **Tab scope:** a fresh capture tab was created and closed afterwards; no existing
  tab was navigated.
- **Pacing:** 300–350 ms between probe requests. No 429s observed.
- **Proxy-envelope pattern:** not detected — plain REST, one path per resource.
- **GraphQL BFF:** not detected.

## 4. Endpoints Discovered

All verified live against a real account. `{pid}` = profile UUID.

| Method | Path | Status | Notes |
|---|---|---|---|
| GET | `/api/configuration` | 200 | Feature flags, locked features, streaks version |
| GET | `/api/profiles` | 200 | Array of `{profileId, name}` |
| GET | `/api/profiles/{pid}` | 200 | Single profile |
| GET | `/api/profiles/{pid}/activities?fromDate&toDate` | 200 | **Date-keyed map**, one key per day |
| GET | `/api/profiles/{pid}/activities/{activityId}` | 200 | Single activity |
| POST | `/api/profiles/{pid}/activities` | 201 | Returns the created activity |
| PUT | `/api/profiles/{pid}/activities/{activityId}` | 200 | Full update (PATCH → 405) |
| DELETE | `/api/profiles/{pid}/activities/{activityId}` | 204 | |
| GET | `/api/profiles/{pid}/routines?from&to` | 200 | Array (empty for this account) |
| GET | `/api/profiles/{pid}/tags` | 200 | Array |
| GET | `/api/profiles/{pid}/todo-tasks` | 200 | Array |
| GET | `/api/profiles/{pid}/todo-task-lists` | 200 | `{lists: [...]}` |
| GET | `/api/externalCalendar/profiles/{pid}/linkedCalendars` | 200 | Array |
| GET | `/api/externalCalendar/profiles/{pid}/externalCalendars/{calId}/activities` | 200 | Date-keyed map |
| GET | `auth.tiimoapp.com/connect/userinfo` | 200 | OIDC userinfo (different host) |

Confirmed **absent** (404): `checklists`, `moods`, `timers`, `categories`,
`activity-templates`, `settings`, `streaks`. Checklists are nested inside
activities, not a standalone resource. `/api/users/me` returns 405 — exists but
not under GET.

**Parameter names were recovered from validation errors, not guessed:**
`activities` requires `fromDate`; `routines` requires `from`. The API returns
RFC 7807 ProblemDetails naming the missing field.

## 5. Authentication

**OIDC / OAuth2 via `auth.tiimoapp.com` (Duende IdentityServer).** From the public
discovery document:

- `authorization_endpoint`: `/connect/authorize`
- `token_endpoint`: `/connect/token`
- `revocation_endpoint`: `/connect/revoke`
- `end_session_endpoint`: `/connect/logout`
- **grant types:** `authorization_code`, `client_credentials`, `password`, `refresh_token`, `apple`, `google`
- **PKCE:** `S256` and `plain` supported
- **scopes:** `openid`, `offline_access`, `tiimo_webapi`, `tiimo_webapi_admin`, `tiimo_auth`, `tiimo_auth_admin`

The data API accepts `Authorization: Bearer <access_token>` with scope `tiimo_webapi`.

The **web app** obtains its token through NextAuth and exposes it at
`webapp.tiimoapp.com/api/auth/session` → `{user:{email,user_created}, expires, accessToken}`.
The printed CLI should **not** emulate this; it should run its own OIDC flow
(`authorization_code` + PKCE + `offline_access` for refresh) directly against
`auth.tiimoapp.com`. Outstanding unknown: a usable public `client_id` and a
registered loopback redirect URI. See Open Questions.

No cookie replay is involved, so the Step 2d cookie-validation path does not apply.

## 6. Reachability & Runtime Shape

`traffic-analysis.json` reports **`reachability.mode: standard_http`** (confidence
0.65, "no browser-only reachability signals observed"), protocol `rest_json` (0.75).

No Cloudflare, WAF, DataDome, CAPTCHA, or clearance requirement was seen at any
point. **The printed CLI ships a plain Go HTTP client** — no Surf, no browser
transport, no `auth login --chrome`. This fully satisfies cardinal rule 5
(replayability): every discovered call is an ordinary authenticated HTTPS request.

## 7. Data Model Highlights

`Activity` carries **39 fields**. The ones that matter for differentiation:

- **Plan vs actual:** `startTime`/`endTime`/`duration` alongside
  `startTimeActual`/`endTimeActual`/`durationActual`/`durationPaused`.
  The app barely surfaces this; it is the raw material for drift analysis.
- **Completion:** `completedAt`, `state`
- **Nested `checklist`:** `{checklistId, checklistItems[], isCompleted}`, each item
  `{checklistItemId, title, isChecked, checkedAt, checklistDate, index, icon...}`
- **`repetition`:** `{type, weeklyFrequency, weeklyDays[], monthlyDays[], firstDay, lastDay, interval, daily/weekly/monthly/yearly}`
- **`grouping`:** `{groupingType, groupingLabel}` — drives Morning/Afternoon/Evening/Anytime buckets
- **External calendar linkage:** `origin`, `calendarId`, `externalEventId`, `isReadOnly`
- **Time zones:** `startTimeUtc`, `endTimeUtc`, `startTimeLocal`, `timeUtcOffset`
- `duration` is in **seconds** (3000 = 50 min, verified against the UI)
- `iconType: "UnicodeEmoji"` with `iconId` holding the literal emoji

## 8. Coverage Analysis

Exercised: configuration, profiles, activities (full CRUD), routines, tags,
todo-tasks, todo-task-lists, linked calendars, external-calendar activities, userinfo.

Likely missed:
- **Mood / energy tracking** — in the product, no endpoint found under the names
  probed. May be mobile-only or named differently.
- **Focus timer sessions** — no endpoint found; `durationActual`/`durationPaused`
  on the activity suggests timer state is folded into the activity record.
- **AI Co-planner** (`ai.tiimoapp.com`) — host observed, surface not probed.
- **Streaks** — 404 under that name, though `configuration.streaksVersion` exists.
- Write paths for todo-tasks, tags, and checklist items were not exercised;
  only the activity write path was tested. The activity CRUD pattern
  (POST collection / PUT item / DELETE item) is a strong prior for the others.

## 9. Rate Limiting

No 429s. ~40 requests total at ~3 req/s effective. No rate-limit headers observed.

## 10. PII Handling (deviation — read this)

`browser-sniff-capture.json` was **hand-built with synthetic values in the verified
schema** rather than dumping raw response bodies. Structure, field names, types,
nesting, and status codes are verbatim from the live capture; every *value* is
synthetic (`Focus block`, `user@example.invalid`, placeholder UUIDs).

Rationale: this is a personal ADHD planner. Raw bodies contain the account
holder's real schedule, and this artifact is archived into manuscripts that may
later be published. The generator consumes types, not values, so nothing is lost.

Verified at write time: zero `Authorization`/`Cookie`/`Set-Cookie` headers and
zero occurrences of the real profile UUID in the artifact.

## 11. Spec Quality Notes (for Phase 2)

`cli-printing-press browser-sniff` produced 15 endpoints / 4 resources / 13 types.
Four defects require hand-authoring before generation:

1. **`auth.type: none`** — wrong. Auth headers were stripped at write time so the
   analyzer had no signal. Must be enriched to OAuth2 Bearer.
2. **`/connect/userinfo` has no `base_url` override** — it lives on
   `auth.tiimoapp.com` and would otherwise resolve against `api.tiimoapp.com`.
3. **`get_activities_2`** — the analyzer's disambiguation of the single-activity
   GET. Needs a real name.
4. **Everything is grouped under a `profiles` resource** — activities, tags,
   routines, and todo-tasks all became `profiles` sub-endpoints because they share
   a path prefix. The CLI command shape should be `tiimo-pp-cli activities list`,
   not `tiimo-pp-cli profiles get-activities`.

## Open Questions

- Public `client_id` and a registered redirect URI for a CLI OIDC flow. If no
  loopback redirect is registered for a public client, the fallback is the
  `password` grant (which the CLI would prompt for at runtime) or an
  operator-supplied token via env var.
- Whether mood and focus-session data are exposed on any endpoint.
- Whether `routines` returns data for accounts that have saved routines (this
  account returned an empty array, so the item schema is unverified).
