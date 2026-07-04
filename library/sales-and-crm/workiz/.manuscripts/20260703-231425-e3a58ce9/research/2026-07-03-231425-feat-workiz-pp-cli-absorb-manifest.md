# Workiz CLI Absorb Manifest

Resource naming note: `jobs` is a reserved framework Cobra command (shadows the built-in `<cli> jobs`). All resources use Workiz's own singular wire-path convention instead (`job/`, `lead/`, `team/`, `Client/`, `TimeOff/` are all singular on the wire) — `job`, `lead`, `team`, `client`, `timeoff`. Verified via `generate --dry-run` that none of these five collide with framework commands.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List jobs (paginated, filter by status/date/open) | Go/PHP/Python SDKs, Pipedream | (generated endpoint) job list | Offline search, SQL composable, no manual 100-record page-walking |
| 2 | Get job by UUID | Go/PHP/Python SDKs, Pipedream | (generated endpoint) job get | Works offline once synced |
| 3 | Create job | Go/Python SDKs, Pipedream action | (generated endpoint) job create | --dry-run, --stdin, scriptable |
| 4 | Update job schedule | Go SDK | (generated endpoint) job update | Typed flags instead of hand-built JSON |
| 5 | Assign crew to job | Go/Python SDKs | (generated endpoint) job assign | Takes a crew name directly |
| 6 | Unassign crew from job | Go/Python SDKs | (generated endpoint) job unassign | Same as above |
| 7 | List leads (paginated, filter by status/date) | Go/PHP/Python SDKs, Pipedream | (generated endpoint) lead list | Offline search |
| 8 | Get lead by UUID | Go/PHP/Python SDKs, Pipedream | (generated endpoint) lead get | Works offline once synced |
| 9 | Create lead | Go/Python SDKs, Pipedream action | (generated endpoint) lead create | --dry-run, --stdin, scriptable |
| 10 | Update lead schedule | Go SDK | (generated endpoint) lead update | Typed flags |
| 11 | Assign crew to lead | Go SDK | (generated endpoint) lead assign | Direct name-based assignment |
| 12 | Unassign crew from lead | Go SDK | (generated endpoint) lead unassign | Same |
| 13 | List team members | Go/Python SDKs, dltHub docs | (generated endpoint) team list | Offline join target for utilization features |
| 14 | Get team member by id | dltHub docs | (generated endpoint) team get | Offline once synced |
| 15 | List time-off records | PHP/Python SDKs, dltHub docs | (generated endpoint) timeoff list | Offline join target for crew availability |
| 16 | Get time-off by username | PHP/Python SDKs, dltHub docs | (generated endpoint) timeoff get | Same |
| 17 | Create client | Go SDK | (generated endpoint) customer create | Typed flags, --dry-run |
| 18 | Get client by id | Go SDK | (generated endpoint) customer get | Offline once synced |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|--------------------------|-------------------|
| 1 | Crew utilization & conflict bottleneck view | `team bottleneck --week` | 10/10 | hand-code | Joins locally synced job.Team[], team, and timeoff tables to compute per-crew scheduled load and flag double-booking/time-off violations in one view — no live endpoint aggregates this | Use this for aggregate crew load AND itemized double-booking/time-off conflicts in one pass. Do not look for a separate "conflicts" or "available" command — this subsumes both. |
| 2 | Lead-to-job conversion funnel (with source ROI) | `lead funnel` | 10/10 | hand-code | Correlates locally synced leads and jobs by client/date/status to compute conversion counts, rates, and average resulting job value per lead source, since Workiz has no convert endpoint | Use this for lead-source conversion rate and ROI ranking together. For raw dollar totals by source/status across all jobs (not just lead-originated), use `job revenue` instead. |
| 3 | Revenue pipeline by source/status | `job revenue` | 10/10 | hand-code | Aggregates TotalPrice/AmountDue across the local job mirror grouped by JobSource and Status — an aggregation Workiz has no live endpoint for | Use this for dollar-value rollups by source/status. For lead-to-job conversion counts/rates, use `lead funnel` instead. |
| 4 | Missing-data / billing-readiness audit | `job audit` | 9/10 | hand-code | Scans the local job/lead/client mirror for null/empty required fields (phone, email, AmountDue, crew assignment) that would block a downstream billing push | none |
| 5 | "Since" change digest | `digest` | 10/10 | hand-code | Diffs the local mirror against the last recorded CreatedDate/LastStatusUpdate sync cursor and lists new/changed jobs, leads, and clients grouped by entity | none |
| 6 | Full-text search across notes/comments | `job search <term>` | 8/10 | hand-code | Runs local FTS over synced JobNotes/LeadNotes and the custom-unmarshaled Comments field — content the live Workiz API cannot search at all | Use this for free-text search inside notes/comments. For structured filtering by status/date/open, use the generated `job list`/`lead list` flags instead. |

Killed candidates (from the novel-features brainstorm — see `2026-07-03-231425-novel-features-brainstorm.md` for full reasoning): crew availability finder (subsumed by `team bottleneck`), lead source ROI as standalone (folded into `lead funnel`), client 360 view, stale open-job flag, crew leaderboard, duplicate client detector, team id/name resolver (redundant with generated assign/unassign).

## Known Auth Quirk (affects Phase 2/3, not scope)

Workiz auth does not fit the generator's standard `auth.in: header|query|cookie` model:
- The API token is a **URL path segment** on every call (`.../api/v1/{token}/...`), not a header/query value. The generator's spec-driven auth injection has no `in: path` mode — confirmed via a scratch dry-run that `{token}`-in-path becomes a required positional argument on every generated command, which is the wrong UX (users would have to retype their token on every invocation).
- The API secret (`auth_secret`) is a **JSON body field** on every POST (write) call, not a header.
- **Plan:** declare `auth.env_vars: [WORKIZ_API_TOKEN, WORKIZ_API_SECRET]` (confirmed via scratch test that the generator creates two independent config fields, `WorkizApiToken`/`WorkizApiSecret`, one per env var). Author all endpoint paths WITHOUT the token segment (e.g. `path: "/job/all/"`). In Phase 3, hand-edit `internal/config/config.go`'s `Load()` to append the resolved token to `cfg.BaseURL` right after the token is read (2-line, clearly-commented addition — this is the standard fix for token-in-path auth and the only way to keep every generated endpoint command working without per-call token re-entry). Handle `auth_secret` injection into POST bodies via a small shared client-side helper rather than exposing it as a per-command flag the user must remember to pass every time.
