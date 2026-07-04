## Customer model

**Dana — Lead Dispatcher, 12-tech HVAC/plumbing shop**
*Today (without this CLI):* Dana runs the daily dispatch board out of the Workiz web app, clicking through each of 12 techs' calendars one at a time and flipping to a separate Time-Off tab to cross-check who's actually available. There's no single screen that shows load-vs-availability across the crew.
*Weekly ritual:* Every Monday morning she rebuilds the week's dispatch plan by manually walking job schedules against time-off records, tech by tech, to catch double-bookings before they become no-shows.
*Frustration:* Workiz has no endpoint that answers "who's overbooked this week" — she has to hold the whole roster in her head while paging through per-tech views, and nothing flags a scheduling conflict for her.

**Marcus — Owner, 3-crew plumbing/electrical FSM business**
*Today (without this CLI):* Marcus exports job data to Excel because Workiz has no aggregate reporting endpoints — no revenue-by-source, no lead-conversion numbers, nothing rolled up.
*Weekly ritual:* Friday afternoon he manually tallies which lead sources (Yelp, referral, Google Ads) actually turned into paid jobs, trying to decide where next month's marketing dollars go.
*Frustration:* There's no lead-to-job conversion endpoint (leads become jobs app-side with no linking field exposed cleanly), so he's eyeballing status/date correlations by hand and second-guessing the numbers.

**Priya — Integration developer at a franchise/multi-account FSM integrator**
*Today (without this CLI):* Priya wires Workiz into billing and CRM systems via Pipedream/Zapier polling on `CreatedDate`, and occasionally hand-rolls curl scripts against the API for one-off data pulls across client accounts.
*Weekly ritual:* Before pushing newly created jobs/leads into the billing pipeline, she manually checks for missing phone/email/amount fields and tracks her own polling cursor to avoid double-processing records.
*Frustration:* No built-in "what's new or changed since I last checked" primitive — she has to hand-maintain sync cursors and re-derive completeness checks herself every time, per client account.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Kill/Keep check | Long Description |
|---|------|---------|-------------|---------|--------|------------------|-------------------|
| C1 | Crew utilization / bottleneck view | `team bottleneck --week` | Per-crew scheduled load vs available hours, joining jobs.Team[] + team + time_off locally | Dana | (c) cross-entity join; named in Build Priorities #3 | Reimplementation check: local-data command over synced mirror, not a fake API call — keep, tag `local-data`. No LLM/external service/auth issues. | none |
| C2 | Schedule conflict / overbooking detector | `jobs conflicts` | Flags overlapping job date-ranges for the same crew member, or crew assigned during their own time-off | Dana | (b)(c) FSM dispatch pattern + cross-entity join | Same local-data pass. Verifiable by manual join recompute. | none |
| C3 | Crew availability finder | `team available --date` | Lists crew free on a given date/window by cross-referencing time_off + assigned jobs | Dana | (c) cross-entity join | Local-data, keep on checks, but same underlying join as C1/C2 — flagged for sibling review | none |
| C4 | Lead-to-job conversion funnel | `leads funnel` | Local status/date-matched conversion rates by lead source, since no convert endpoint exists | Marcus | (b)(c) named in Product Thesis + Build Priorities | Local-data reimplementation ok (no aggregation endpoint exists to replace). No LLM. | none |
| C5 | Revenue pipeline by source/status | `jobs revenue` | Aggregates TotalPrice/AmountDue by JobSource and Status across the local mirror | Marcus | (b)(c) named in Product Thesis + Build Priorities | Local-data ok, deterministic sums, verifiable. | none |
| C6 | Lead source ROI ranking | `leads roi` | Ranks lead sources by resulting average job value | Marcus | (b)(c) derived from same join as C4/C5 | Passes checks individually but heavily overlaps C4/C5 — flagged | none |
| C7 | Missing-data / billing-readiness audit | `jobs audit` | Finds jobs/leads/clients missing phone/email/amount/crew fields that would block downstream billing | Priya | (a)(c) persona pain + Data Layer's nullable-field notes | Local-data, no LLM classification (pure field-presence checks, not "smart" validation). Keep. | none |
| C8 | "Since" change digest | `sync digest` | Shows new/changed records since last sync cursor (CreatedDate/LastStatusUpdate), grouped by entity | Priya | (a)(b) named in Product Thesis + Table Stakes polling pattern | Local-data cursor diff, mechanical, no LLM. Keep. | none |
| C9 | Full-text search across notes/comments | `jobs search "leak"` | FTS over JobNotes/LeadNotes/Comments — a capability the live API has zero of | Dana/Marcus | (c) explicit gap named in Data Layer | Reimplementation check: this is local FTS, not a fake API call, since no live search exists at all — keep. | none |
| C10 | Client 360 view | `clients profile <id>` | Joins a client with their full job/lead history, spend total, last contact | Marcus | (c) cross-entity join | Passes mechanical checks but overlaps C5/C7 territory — flagged | none |
| C11 | Stale open-job/lead flag | `jobs stale` | Jobs/leads still "open" with no LastStatusUpdate in N days | Dana/Marcus | (c) derived from Data Layer's LastStatusUpdate cursor | Passes checks, but overlaps C2/C8 — flagged | none |
| C12 | Crew revenue/job-count leaderboard | `team leaderboard` | Ranks techs by jobs completed / revenue generated | Marcus | (c) cross-entity join | Passes checks but overlaps C1/C5 — flagged, no direct brief evidence | none |
| C13 | Duplicate client detector | `clients duplicates` | Matches clients by phone/email to catch duplicate CRM entries | Priya | (c) speculative data-quality angle | Mechanical match, no LLM — passes checks, but zero evidence in brief | none |
| C14 | Team id/name resolver | `team resolve <name>` | Helper to resolve crew id↔name given API's Team.Id type inconsistency | Dana | (c) Codebase Intelligence's id/name inconsistency note | **Killed at Pass 2**: scope creep into a problem the absorbed `jobs assign`/`leads assign` commands (manifest #5/#6) already solve by taking a name directly — redundant | none |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|-------------------|
| 1 | Crew utilization & conflict bottleneck view | `team bottleneck --week` | 10/10 | hand-code | Joins locally synced `jobs.Team[]`, `team`, and `time_off` tables to compute per-crew scheduled load and flag overlapping-booking/time-off violations in one view | Brief Top Workflows #5 (crew/roster visibility), Product Thesis ("who's overbooked this week"), Build Priorities #3 (names "crew utilization/bottleneck view" explicitly) | Use this for aggregate crew load AND itemized double-booking/time-off conflicts in one pass. Do not look for a separate "conflicts" or "available" command — this subsumes both. |
| 2 | Lead-to-job conversion funnel (with source ROI) | `leads funnel` | 10/10 | hand-code | Correlates locally synced `leads` and `jobs` by client/date/status to compute conversion counts, rates, and average resulting job value per lead source, since Workiz exposes no convert endpoint | Product Thesis ("which lead sources actually convert"), Build Priorities #3 (names "lead-to-job conversion funnel" explicitly) | Use this for lead-source conversion rate and ROI ranking together. For raw dollar totals by source/status across all jobs (not just lead-originated), use `jobs revenue` instead. |
| 3 | Revenue pipeline by source/status | `jobs revenue` | 10/10 | hand-code | Aggregates `TotalPrice`/`AmountDue` across the local `jobs` mirror grouped by `JobSource` and `Status`, an aggregation Workiz has no live endpoint for | Product Thesis ("revenue pipeline by source/status"), Build Priorities #3 (names this explicitly) | Use this for dollar-value rollups by source/status. For lead-to-job conversion counts/rates, use `leads funnel` instead. |
| 4 | Missing-data / billing-readiness audit | `jobs audit` | 9/10 | hand-code | Scans the local `jobs`/`leads`/`clients` mirror for null/empty required fields (phone, email, `AmountDue`, crew assignment) that would block a downstream billing push | Build Priorities #3 (names "missing-data audit"), Codebase Intelligence (documents the nullable/inconsistent field shapes — `workizTime` "null" strings, `Unit` type-drift — that make this audit necessary) | none |
| 5 | "Since" change digest | `sync digest` | 10/10 | hand-code | Diffs the local mirror against the last recorded `CreatedDate`/`LastStatusUpdate` sync cursor and lists new/changed jobs, leads, and clients grouped by entity | Product Thesis ("what changed since I last checked"), Top Workflows / Table Stakes (polling on `CreatedDate` is the established integration pattern every existing tool uses) | none |
| 6 | Full-text search across notes/comments | `jobs search <term>` | 8/10 | hand-code | Runs local FTS over synced `JobNotes`/`LeadNotes` and the custom-unmarshaled `Comments` field — content the live Workiz API cannot search at all | Data Layer (explicitly: "none of this is searchable via the live API at all"), Codebase Intelligence (documents the custom `Comments` unmarshal needed to make the field queryable) | Use this for free-text search inside notes/comments. For structured filtering by status/date/open, use the generated `jobs list`/`leads list` flags instead. |

### Killed candidates

| Candidate | Reason |
|-----------|--------|
| Crew availability finder (`team available --date`) | Fully subsumed by `team bottleneck` — same underlying jobs+team+time_off join, no standalone value as a second command. |
| Lead source ROI ranking (`leads roi`) | Same data and join as `leads funnel`; folded in as a column rather than shipped as a sibling command to avoid tool-choice confusion. |
| Client 360 view (`clients profile <id>`) | Overlaps `jobs revenue`/`jobs audit` territory for financial/completeness rollups; no explicit brief evidence, purely speculative convenience. |
| Stale open-job/lead flag (`jobs stale`) | Overlaps `sync digest` (change tracking) and `team bottleneck` (open-job visibility); not named anywhere in Build Priorities, single-source evidence only (Data Layer's cursor note). |
| Crew revenue/job-count leaderboard (`team leaderboard`) | Overlaps `team bottleneck` (utilization) and `jobs revenue` (revenue by source); tech-ranking framing has zero support in the brief. |
| Duplicate client detector (`clients duplicates`) | No evidence anywhere in the brief of duplicate-client pain; zero-source, purely speculative CRM-hygiene feature. |
| Team id/name resolver (`team resolve <name>`) | Killed at Pass 2 as redundant — the absorbed `jobs assign`/`leads assign` commands (manifest #5/#6) already take a crew member's name directly, so this solves a problem the generated commands don't have. |
