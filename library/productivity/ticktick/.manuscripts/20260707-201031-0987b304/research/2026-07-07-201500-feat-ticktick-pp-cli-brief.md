# TickTick CLI Brief

## API Identity
- Domain: Task management — tasks, projects, habits, focus/pomodoro, tags, daily notes (TickTick / Dida365)
- Users: productivity power users; the requesting operator specifically: daily-note workflow (TEXT-kind task), habit tracking, focus session logging, weekly reviews
- Data profile: two API surfaces:
  - **V1 Open API** (`api.ticktick.com/open/v1`) — official, OAuth2 app (client id/secret → bearer token). Projects + tasks CRUD ONLY. No habits, no focus, no tags mgmt, no batch, no completed-task history.
  - **V2 internal API** (`api.ticktick.com/api/v2`) — unofficial web API. Everything: batch task ops (the `kind`/`etag` surface where our corruption bug lives), habits + checkins, focus/pomodoro records, tags, project groups, completed tasks, user prefs. Auth: session token from `/user/signon` (username/password) or browser `t` cookie.

## Reachability Risk
- Low. api.ticktick.com serves plain HTTPS; many community wrappers work today. Historical quirk: 503 without a browser-like User-Agent (ticktick-py issue #21) — set UA in client. V2 is unofficial: endpoints can shift without notice.
- Google-SSO accounts may lack a password for V2 signon → cookie fallback (`t` cookie) or set an account password.

## Top Workflows (operator-specific first)
1. **Daily note read/update WITHOUT corruption** — fetch task by id, update content/focus block while never sending `kind`, carrying etag + isAllDay correctly. This replaces the buggy MCP.
2. Daily agenda pull — today's tasks + habits + focus sessions in one command.
3. Habit checkins — list habits, upsert checkins (7/2 habit-ID bug in current MCP).
4. Focus session logging + stats.
5. Task/project CRUD + batch ops, completed-task history for week-in-review.

## Table Stakes (from ecosystem)
- ticktick MCPs (jacepark12, kpihx/tick-mcp 71 tools, liadgez 112 ops): task CRUD, projects, tags, habits, focus, filters, batch
- ticktick-py / ticktick-api-v2 / ticktick-sdk: V2 session auth, habits+focus read, checkin write, state sync
- avilabss/ticktick-cli (Go): tasks, projects, pomodoro, habits

## Data Layer
- Primary entities: tasks (incl. NOTE/TEXT kind), projects, project groups, habits, habit checkins, focus records, tags
- Sync cursor: V2 `/batch/check/{point}` incremental sync
- FTS/search: task title/content search offline

## Codebase Intelligence
- tick-mcp auth pattern: `TICKTICK_API_TOKEN` (V1 bearer) + `TICKTICK_SESSION_TOKEN` (V2 `t` cookie) — clean dual-tier model to absorb
- OliverStoll/ticktick-api-v2: cookie auth, habits/focus read + habit-entry write proven
- Known trap (lived experience 7/7/26): V2 task update with `kind` field converts TEXT→NOTE and breaks childIds; must never send `kind`, must carry id/projectId/title/content/startDate/dueDate/timeZone/isAllDay/etag

## User Vision
- Route around the buggy claude.ai TickTick MCP for daily-note/habit/focus workflows; safe-by-construction daily-note update command.

## Product Thesis
- Name: ticktick-pp-cli
- Why: the only TickTick tool with a **corruption-proof daily-note contract**, offline SQLite mirror for week-in-review analytics, and dual V1/V2 tier routing — built by someone who hit the exact bugs.

## Build Priorities
1. V2 client + session auth (signon + t-cookie) with dual-tier routing (V1 optional)
2. Task safe-update path (field whitelist, never `kind`), daily-note commands
3. Habits + checkins, focus records
4. Sync/SQLite mirror + agenda/review analytics
