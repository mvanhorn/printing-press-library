# Sprocket Sports — Browser-Sniff Discovery Report

## 1. User Goal Flow
- **Goal:** Check the team's upcoming schedule and roster (parent of a travel-team player).
- Backend: authenticated SPA on `https://jfcsoccer.sprocketsports.com` (Jacksonville FC tenant).
- Steps completed:
  1. Loaded `/dashboard` (authenticated) → fired the dashboard data load (account, players, calendar widget, teams).
  2. Reloaded to capture the full initial API surface via the Performance API.
  3. Probed the authenticated REST endpoints with the live Bearer token (read-only GET/POST, shapes only).
  4. Drove the Club Calendar widget / probed `/api/public/calendar` to recover its POST body shape.
- Coverage: full dashboard + calendar surface (54 on-platform API paths observed).

## 2. Backend / Configuration
- Backend: **PrimeNG/Angular SPA + .NET (Microsoft-IIS) REST API.** Client-rendered.
- Capture backend: **chrome-MCP** (drove the user's already-running, logged-in Chrome in an
  isolated fresh tab; no cookie transfer; tab closed after capture).
- Runtime reachability: `probe-reachability` → **`standard_http`** (conf 0.95). Plain stdlib HTTP
  200. No WAF/Cloudflare/clearance gate. Printed CLI ships **standard HTTP transport**.
- API base: `https://jfcsoccer.sprocketsports.com` (`/api/...` and a small `/meta-api/...`).
- Multi-tenant: each club is `<club>.sprocketsports.com`; base URL is the only tenant difference.

## 3. Authentication (critical)
- **OAuth2 / OIDC (Duende IdentityServer).**
  - Issuer: `https://login.sprocketsports.com`
  - authorize: `/connect/authorize`, token: `/connect/token`, endsession: `/connect/endsession`
  - PKCE supported; grant types include `authorization_code`; scopes include
    `openid profile WebApplicationAPI IdentityServerApi offline_access`.
  - OIDC client id (web app): `sprocket-sports`. Web session scope: `openid profile WebApplicationAPI`.
- **API auth = `Authorization: Bearer <access_token>` (JWT).**
  - Confirmed: `/api/club-users/me` → **401** with cookies only, **200** with the Bearer token.
  - Token lives in browser `sessionStorage["oidc.user:...sprocket-sports"].access_token` (oidc-client-ts).
  - Web session has **no refresh token** (offline_access not requested by the web app).
- **CLI auth model (v1):** `bearer_token` via env var `SPROCKET_TOKEN`. We do not control client
  registration on Sprocket's IdentityServer, so an interactive PKCE/loopback flow can't be assumed to
  work (web client redirect URIs only). Token-paste is the honest, working v1. Known limitation:
  access tokens expire (~1h) → re-paste. Future enhancement: a real `auth login` if a native/device
  client can be registered or the loopback redirect is allowed.
- **No secrets stored in any artifact.** Token values, cookies, and JWTs were used in-session only and
  never written to disk. Real names/emails/IDs observed in responses were NOT recorded; only field
  names + types are captured below.

## 4. Endpoints Discovered (curated, all auth-required Bearer, all 200 unless noted)
| Method | Path | Notes |
|---|---|---|
| POST | /api/public/calendar | body `{start,end}` (YYYY-MM-DD, **max ~31-day window**) → `{responses[], events[]}` |
| GET | /api/public/calendar/calendar-event-types | `?includeFacilityRentals=` → event type lookup |
| GET | /api/public/calendar/settings | calendar config |
| GET | /api/public/teams/all | 120 teams: `{clubID,teamID,name,teamColorOptionID}` |
| GET | /api/club-users/player-teams | my players' team assignments (nested `team` objects) |
| GET | /api/players | `?includeInactives=` → my players |
| GET | /api/club-users/family | `?includeParentsWithoutLogin=` → `{familyID,parents,players}` |
| GET | /api/public/programs/all | club programs/seasons |
| GET | /api/players/open-programs | `?includeCredits=` → programs open for registration |
| GET | /api/club-users/me | account/profile |
| GET | /api/account/roles | account roles |
| GET | /api/account/multiple-clubs | clubs this account belongs to |
| GET | /api/club-users/completed-registrations | registration + payment history |
| GET | /api/club-users/completed-team-registrations | team registrations |
| GET | /api/club-users/overdue-invoice-payments | dues owed (empty for this account) |
| GET | /api/club-users/overdue-ticket-payments | ticket dues |
| GET | /api/club-users/failed-payments | `?includeDonations=` |
| GET | /api/public/clubs/settings | club settings |
| GET | /meta-api/public/club | club meta |

Other observed (lower value, page-builder/CMS — excluded from v1 CLI): `/api/public/page-builder/*`,
`/api/public/column/{id}/*`, `/api/public/sponsors/*`, `/api/private/library/featured`,
`/api/field-options`, `/api/clubs/current/options`, `/api/financial-aid/settings`,
`/api/app-settings/environment`, `/api/club-users/missing-waivers`, `/api/club-users/unaccepted-invites`,
`/api/club-users/schedule-sign-ups`, `/api/players/{id}/surveys/*`.

## 5. Key Response Shapes (field names only)
- **Calendar event** (`events[]` element): `id, clubId, clubCalendarEventId, isOwner, clubCalendarEvent{
  id, clubEventTypeID, title, shortDescription, division, series, teamID, opponentTeamID, teamIDs,
  startDate, endDate, timeZoneID, locationID, facilityFieldID, opponent, awayGame, collectParticipation,
  showTime, showTimeTBD, isPrivate, isCancelled, ... }`. (`responses[]` = per-event RSVP/attendance:
  `playerID, clubUserID, clubCalendarEventID, notes, playerName, isStaff, hasAttended`.)
- **Team**: `clubID, teamID, name, teamColorOptionID`.
- **PlayerTeam** (player-teams): `playerID, programID, teamID, assignedOn, inviteStatusID, isGuest,
  acceptedOn, team{teamID,name,shortName,levelID,leagueID,totalGames,playersAssigned,...}`.
- **Player**: `playerID, userID, user, documents, inactive, suspended, isRestricted, playerClubData,
  playerCustomData, family, hasPhoto`.
- **Program**: `programID, parentProgramID, name, programTypeName, programTypeID, startDate, endDate,
  playerCount, staffCount, divisionCount, teams, registrations`.
- **Registration** (completed-registrations): `playerRegistrationID, registrationID, playerID,
  registrationPrice, totalRegistrationPrice, completed, completedOn, player, team, registration,
  payments, amountPaid, amountDue, remainingBalance, totalCreditsUsed`.
- **EventType**: `label, value, icon, descriptionText, disabled`.
- **Family**: `familyID, parents, players`.

## 6. Coverage Analysis
- Exercised: account, players, family, calendar/schedule, teams, programs, registrations, payments.
- Not exercised (out of v1 scope): page-builder CMS, surveys/player-progress, waivers/invites,
  full rosters of arbitrary teams (parent view only exposes own players' teams), league
  scores/standings (not surfaced in the parent dashboard — likely league-admin only).

## 7. Authentication Context
- Authenticated session used: **yes** (chrome-MCP against the user's live logged-in Chrome).
- Auth-only endpoints: effectively all `/api/club-users/*`, `/api/players/*`, `/api/account/*`, and the
  `/api/public/calendar` POST (returns the user's RSVP data) require the Bearer token.
- Session state excluded from manuscript archiving: yes (token never written to disk).
