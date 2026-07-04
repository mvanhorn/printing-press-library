# Workiz Crowd-Sniff Report

## npm Packages Analyzed
- `@pipedream/workiz` — official Pipedream Workiz component wrapper. Yielded endpoints (see below). Only Workiz-named package on npm; no independent community SDK on npm.
- PyPI search for "workiz" returned no indexed packages (the `workiz-python-wrapper` project on GitHub was never published to PyPI).

## GitHub Repos Searched
Manual `gh api search/code` + `gh api repos/.../contents` archaeology (the automated `cli-printing-press crowd-sniff --api workiz` command returned `no endpoints discovered for "workiz"` — its heuristics did not surface a usable source, so this report is backed by manual equivalent research instead):

- `forward-force/workiz` (PHP SDK) — jobs, leads, timeoff (GET-only)
- `BeelineRoutes/workiz` (Go SDK) — most complete: jobs, leads, clients, team, assign/unassign, updates. Zero open issues.
- `OkoyaUsman/workiz-python-wrapper` (Python SDK) — jobs, leads, team, timeoff
- `PipedreamHQ/pipedream` `components/workiz/` (official Pipedream app + 2 actions + 2 polling sources)

GitHub token status: authenticated via `gh` CLI (broader rate limits, no throttling encountered).

## Endpoints Discovered

| Method | Path | Source Tier | Source Count |
|---|---|---|---|
| GET | `job/all/` | community-sdk | 4 (Go, PHP, Python, Pipedream) |
| GET | `job/get/{uuid}/` | community-sdk | 4 |
| POST | `job/create/` | community-sdk | 3 (Go, Python, Pipedream) |
| POST | `job/update/` | community-sdk | 1 (Go) |
| POST | `job/assign/` | community-sdk | 1 (Go) |
| POST | `job/unassign/` | community-sdk | 1 (Go) |
| GET | `lead/all/` | community-sdk | 4 |
| GET | `lead/get/{uuid}/` | community-sdk | 4 |
| POST | `lead/create/` | community-sdk | 3 |
| POST | `lead/update/` | community-sdk | 1 (Go) |
| POST | `lead/assign/` | community-sdk | 1 (Go) |
| POST | `lead/unassign/` | community-sdk | 1 (Go) |
| GET | `team/all/` | community-sdk | 3 (Go, Python, dltHub docs) |
| GET | `team/get/{user_id}/` | code-search (dltHub docs table only) | 1 |
| GET | `TimeOff/get/` | community-sdk | 3 (PHP, Python, dltHub docs) |
| GET | `TimeOff/get/{username}/` | community-sdk | 3 |
| POST | `Client/create/` | community-sdk | 1 (Go) |
| GET | `Client/get/{id}/` | community-sdk | 1 (Go) |

## Base URL Resolution
`https://api.workiz.com/api/v1/` — found identically in all 4 sources (Go, PHP, Python, Pipedream). No ambiguity.

## Auth Patterns Detected
- Token embedded as a URL path segment: `https://api.workiz.com/api/v1/{api_token}/{endpoint}`. Confirmed in all 4 sources.
- `auth_secret` sent as a JSON body field on every POST (write) call. Confirmed in Go SDK and Pipedream's `_authData()` helper.
- No header-based auth (`Authorization`, `X-API-Key`) observed anywhere.

## Parameter Name Evidence
- `records` (max 100 per docs comment in Go SDK: "docs say 100 is the most you can request at a time"), `offset`, `start_date` (YYYY-MM-DD), `only_open` (bool), `status` (repeatable) — all confirmed on `job/all/`/`lead/all/`.
- Pipedream's `create-job`/`create-lead` action props give clean, human field labels (`firstName`, `lastName`, `email`, `phone`, `address`, `city`, `state`, `postalCode`, `jobType`, `jobDateTime`) mapped to PascalCase wire fields (`FirstName`, `JobDateTime`, etc.) — strong evidence for `flag_name` authoring.
- Crew assign/unassign takes the crew member's *name* (not id) as the wire parameter — Go SDK does an id→name lookup internally because Workiz's assign/unassign API only accepts names.

## Coverage Summary
- Total endpoints found: 18 across 5 resources (jobs, leads, clients, team, time-off)
- Breakdown: 15 community-sdk (2+ sources), 1 community-sdk (single source, Go-only: `Client/create`/`Client/get`), 1 code-search (dltHub docs table only: `team/get/{user_id}`)
- Gap vs Phase 1 brief: the brief's assumption of no true webhook-registration endpoint is confirmed — every existing integration (Pipedream, dlt) uses polling on `CreatedDate`, not a webhook subscribe/unsubscribe call. No client update/delete or job delete endpoint was found in any source; treat job/lead as create+update only (no destructive endpoints exist to mirror).
