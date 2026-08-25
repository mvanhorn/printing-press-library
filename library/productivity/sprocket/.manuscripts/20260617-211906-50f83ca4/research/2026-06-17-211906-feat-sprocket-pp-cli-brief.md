# Sprocket Sports CLI Brief

## API Identity
- Domain: Youth-sports club management SaaS (Sprocket Sports). Multi-tenant — each
  club gets `<club>.sprocketsports.com`. This run targets the **JFC soccer**
  tenant: `https://jfcsoccer.sprocketsports.com/dashboard`.
- Source type: **website / undocumented internal API.** No public/official
  developer API exists. Client is a client-rendered SPA (`<base href="/">`,
  Microsoft-IIS origin) that loads data via authenticated XHR/JSON after login.
  An iOS app (App Store id1548746450) confirms a real JSON backend exists.
- Users: club admins, coaches/team managers, and **parents/members** (the
  account this CLI is built against is a parent of a travel-team player).
- Data profile: teams, rosters/players, schedule (games + practices + events),
  league scores & standings, registrations/payments, announcements/messages.

## Reachability Risk
- **None.** `probe-reachability` → `mode: standard_http`, confidence 0.95
  (stdlib HTTP 200 + surf-chrome HTTP 200). No Cloudflare/WAF/clearance gate.
  Printed CLI ships **standard HTTP transport**. The only hard part is
  *discovery* (finding auth-gated JSON endpoints), not runtime reachability.
- Auth model: session-cookie SPA (login at the tenant subdomain). To be
  confirmed during sniff — may be raw cookie replay or a cookie-derived
  Authorization header (composed auth). Cookie-replay validity gets tested in
  capture Step 2d before promising `auth login --chrome`.

## Top Workflows (parent/member of a travel team)
1. **Check the team's upcoming schedule** — next game, this week's events,
   practice times, locations. The #1 daily check.
2. **View the team roster** — players/coaches on my kid's team.
3. **Scores & standings** — league table, recent results, my team's record.
4. **Registration / payment status** — what's registered, what's owed.
5. **Announcements / team messages** — latest from the coach/club.

## Table Stakes (vs TeamSnap / SportsEngine / GameChanger / Spond)
- Schedule/calendar view with date filtering — non-negotiable.
- Roster listing.
- Event detail (opponent, location, time, home/away).
- Scores & standings for league play.
- (RSVP/availability is common in peers; only build if the sniff finds a
  write endpoint that is safe to support — default to read-only.)

## Data Layer
- Primary entities: `team`, `event` (game/practice/other), `player`/roster
  member, `standing`, `registration`/payment, `message`/announcement.
- Sync cursor: events by date window; teams/roster are slowly-changing.
- FTS/search: events by opponent/location/title; players by name.
- Local store earns its keep: "next game", "this week", "who's on the roster",
  "what's our record" should all answer from a synced SQLite mirror, fast and
  offline — and feed an agent ("when is my kid's next game?").

## Source Priority
- Single source (JFC tenant). No combo. Spec comes entirely from browser-sniff.

## Reachability / Runtime decision
- Runtime: `standard_http`. No browser sidecar, no Surf, no clearance cookie.
- Discovery: authenticated browser-sniff of the live logged-in dashboard
  (Chrome running, session active). chrome-MCP preferred (drives existing
  Chrome, no cookie-transfer/quit dance), browser-use profile path as fallback.

## Product Thesis
- Name (slug): `sprocket` — binary `sprocket-pp-cli`. Display name "Sprocket Sports".
  Default base URL points at the JFC tenant; tenant subdomain is configurable so
  the same CLI works for any club.
- Why it should exist: parents/coaches live in this dashboard for schedules and
  rosters. A terminal/agent-native tool that answers "next game?", "this week's
  schedule", "team roster", "our record" — instantly, scriptably (cron reminders),
  and offline — is genuinely useful and nothing like it exists for this platform.

## Build Priorities
1. Data layer for events/schedule, teams, roster, standings (Priority 0).
2. Absorbed read surface: schedule list/filter, event get, roster list,
   standings, registration/payment status, messages (Priority 1).
3. Transcendence (Priority 2): local-store compounders — `next` (next event),
   `week` (this week's schedule), `record` (team record from standings),
   `since` (what changed since last sync), agent-native `--select`/`--json`.

## Sources
- https://sprocketsports.com/ (platform overview)
- https://sprocketsports.com/solutions/scheduling
- https://sprocketsports.com/solutions/league-management
- https://apps.apple.com/us/app/sprocket-sports/id1548746450 (confirms JSON backend)
