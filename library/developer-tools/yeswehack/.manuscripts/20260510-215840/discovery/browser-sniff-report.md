# YesWeHack Browser-Sniff Discovery Report

## Capture Summary

- Target: `https://yeswehack.com/programs?disabled=0` (and follow-on routes)
- API base: `https://api.yeswehack.com`
- Capture backend: chrome-MCP (Claude in Chrome extension)
- Auth surface: researcher-side JWT extracted from `localStorage.access_token`, sent as `Authorization: Bearer <jwt>` with `Accept: application/json`
- Reachability: standard HTTP, no Cloudflare/Vercel/DataDome challenge observed at either yeswehack.com or api.yeswehack.com
- CORS: open with preflight (OPTIONS works); fetches must NOT set `credentials: 'include'` (cookie-credentialed cross-origin appears to be blocked)
- Session: confirmed live (avatar visible, 67 active programs, no login redirect)
- Interaction rounds: 6 (programs list, reports list, hacktivity list, hunter profile click, program detail, program activity tab, ranking page)

## Auth Pattern

| Field | Value |
| - | - |
| Auth type | bearer_token |
| Header | Authorization |
| Format | `Bearer <jwt>` |
| Token source | localStorage `access_token` on yeswehack.com origin |
| Token shape | JWT (RS256, prefix `eyJ0eXAiOiJKV1Q...`) |
| Refresh path | `apps.yeswehack.com/oauth/v2/token` (per API-apps docs; same endpoint OAuth2 API apps use) |
| Required for | Every researcher-side endpoint (404/403 without it) |

The platform also documents two other auth surfaces that are out of scope for a researcher CLI:
- Personal Access Tokens (`X-AUTH-TOKEN` header) - role-gated to Business Unit Owner/Manager and Program Manager. Researcher accounts cannot generate these.
- OAuth2 API apps (`apps.yeswehack.com/oauth/v2/{authorize,token}`) - requires Customer Success Manager approval. Read-only across authorized business units.

## Endpoints Discovered

All endpoints HTTP-probed live with the user's authenticated JWT during browser-sniff. Status codes are real responses, not guesses.

| Method | Path | Status | Top-level shape | Auth | Notes |
| - | - | - | - | - | - |
| GET | `/user` | 200 | object | required | Authenticated user account |
| GET | `/user/reports` | 200 | `{pagination, items}` | required | User's submitted reports (paginated) |
| GET | `/user/invitations` | 200 | object | required | Program invitations |
| GET | `/user/email-aliases` | 200 | object | required | User-level email aliases |
| GET | `/programs` | 200 | `{pagination, items}` | required | Program list (paginated; supports `disabled`, `search`, `business_unit`, `bounty`, `vdp` filters) |
| GET | `/programs/{slug}` | 200 | object | required | Program detail. Fields: title, slug, rules, rules_html, type, status, demo, public, bounty, gift, hall_of_fame, hacktivity, report_count_per_scope, report_collaboration_active, pid, secured, triaged, disabled, vdp, reports_count, reports_imported_count, bounty_reward_min, bounty_reward_max, scopes_count, archived, thumbnail, event, business_unit, vpn_outbound_ips, id |
| GET | `/programs/{slug}/scopes` | 200 | `{items}` | required | Scope list (in-scope + out-of-scope assets) |
| GET | `/programs/{slug}/reports` | 403 | error | manager-only | Researchers cannot list a program's reports |
| GET | `/programs/{slug}/hacktivity` | 403 | error | manager-only | Researchers cannot list a program's hacktivity |
| GET | `/v2/hacktivity` | 200 | `{items}` (effectively paginated via `resultsPerPage`) | required | Public disclosed reports feed |
| GET | `/v2/hacktivity/{username}` | 200 | `{items}` (paginated via `resultsPerPage`) | required | Hunter's disclosed reports |
| GET | `/hunters/{username}` | 200 | object | required | Hunter profile. Fields: username, slug, public_firstname, public_lastname, hunter_profile, points, nb_reports, rank, impact, kyc_status, avatar, gpg_key, track_records, joined_on, nationality |
| GET | `/hunters/{username}/achievements` | 200 | object | required | Hunter achievement badges |
| GET | `/business-units` | 200 | object | required | Business unit list |
| GET | `/ranking` | 200 | object | required | Global leaderboard |
| GET | `/events` | 200 | `{pagination, items}` | required | Platform events |
| GET | `/types/vulnerable-part` | 200 | `{items}` | required | Vulnerability-part taxonomy |
| GET | `/utils/countries` | 200 | object | required | Country reference list |
| GET | `/utils/profile/urls` | 200 | object | required | Allowed profile URL types |

Endpoints returning 404 (do not exist on the live API): `/user/me`, `/user/profile`, `/user/payouts`, `/user/dashboard`, `/user/achievements`, `/user/tasks`, `/user/features`, `/user/kyc`, `/utils/categories`, `/utils/bug-types`, `/utils/severities`, `/utils/severity`, `/utils/cvss`, `/utils/cwe`, `/types/scope-type`, `/types/severity`, `/types/severities`, `/types/report-status`, `/types/environment`, `/v2/user/payouts`, `/notifications`, `/programs/{slug}/credentials`, `/programs/{slug}/email-aliases`. These were probed and ruled out.

## Pagination

All paginated endpoints use page-based query string parameters. Two flavors observed:
- `/programs`, `/user/reports`, `/ranking`, `/events`: `?page=N&itemsPerPage=N`
- `/v2/hacktivity`, `/v2/hacktivity/{username}`: `?page=N&resultsPerPage=N`

Generated CLI uses page-style pagination with the correct parameter name per resource.

## Replayability

All discovered endpoints satisfy the cardinal-rule-5 replayability check: they round-trip through plain HTTP/1.1 with the JWT in an Authorization header. No clearance cookie, no browser-resident transport, no live-page-context execution required.

The printed CLI ships plain Go `net/http` transport.

## Out-of-Scope (researcher cannot reach)

- `/programs/{slug}/reports` (403): program-manager view of all reports for a program. Requires PAT or manager-tier OAuth.
- `/programs/{slug}/hacktivity` (403): manager-side hacktivity. Same reason.
- `/asm/*`: ASM (Attack Surface Management) endpoints - documented in the PAT help center, gated to BU owners.
- `POST /reports` and friends: write-side endpoints are intentionally NOT exercised during browser-sniff. Will be implemented in Phase 3 with explicit guard-rails (dry-run, manual confirmation, no batch).

## Notes for Phase 2 Generation

- `--spec-source browser-sniffed` (no original spec exists)
- `--traffic-analysis` points to `traffic-analysis.json` (sibling of this report)
- No `--client-pattern proxy-envelope`; transport is standard REST
- No HTTP transport override needed; `standard` is correct
- Auth in spec: `bearer_token` with env var `YESWEHACK_JWT`
- Phase 3 must add a Chrome-localStorage extractor command (`auth login --chrome`) so users do not have to copy-paste JWTs from DevTools. This is the transcendence onboarding step that no competing tool has.
