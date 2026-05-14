# PushPress CLI Absorb Manifest

## Scope
- **API:** PushPress Platform API v3 (Speakeasy-generated, OpenAPI 3.1, 20 endpoints)
- **Spec:** `$RESEARCH_DIR/pushpress-v3.yaml` (cached from speakeasy-sdks/pushpress-typescript-sdk)
- **Binary:** `pushpress-pp-cli` · **Skill:** `pp-pushpress`
- **Auth:** API key as `Authorization: Bearer <key>`; optional `companyId` HEADER; user pastes `PUSHPRESS_API_KEY` env var when prompted

## Absorbed (match everything in the public /v3 surface)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | List customers | /v3 `GET /customers` | typed endpoint mirror + local store sync | **one-line default** per brief (id, name, plan, status, last_visit); offline FTS5 |
| 2 | Get customer by id | /v3 `GET /customers/{id}` | typed endpoint mirror | terse default; full payload via `--full` |
| 3 | Customer search by email/phone/name | client-side post-sync | local SQLite + FTS5 | offline; agent-shaped `--json` output |
| 4 | List check-ins | /v3 `GET /checkins` | typed endpoint mirror + local store | terse default; date-range filtering via local store |
| 5 | Get check-in by id | /v3 `GET /checkins/{id}` | typed endpoint mirror | terse default |
| 6 | Get company | /v3 `GET /company` | typed endpoint mirror | one-call onboarding sanity check |
| 7 | List apps | /v3 `GET /apps` | typed endpoint mirror | offline list |
| 8 | Get app | /v3 `GET /apps/{app_id}` | typed endpoint mirror | terse default |
| 9 | Install app | /v3 `POST /apps/{app_id}/install` | typed endpoint mirror | `--dry-run` |
| 10 | List app installs | /v3 `GET /apps/{app_id}/installs` | typed endpoint mirror | terse default |
| 11 | Get app install | /v3 `GET /apps/{app_id}/installs/{install_id}` | typed endpoint mirror | terse default |
| 12 | Delete app install | /v3 `DELETE /apps/{app_id}/installs/{install_id}/delete` | typed endpoint mirror | `--dry-run`; destructive-hint MCP annotation |
| 13 | Uninstall app | /v3 `PATCH /apps/{app_id}/installs/{install_id}/uninstall` | typed endpoint mirror | `--dry-run` |
| 14 | List API keys | /v3 `GET /keys` | typed endpoint mirror | admin self-service |
| 15 | Create API key | /v3 `POST /keys` | typed endpoint mirror | `--dry-run` |
| 16 | Get API key | /v3 `GET /keys/{key_id}` | typed endpoint mirror | terse default |
| 17 | Delete API key | /v3 `DELETE /keys/{key_id}` | typed endpoint mirror | `--dry-run` |
| 18 | Revoke API key | /v3 `PATCH /keys/{key_id}/revoke` | typed endpoint mirror | `--dry-run` |
| 19 | Send email | /v3 `POST /messages/email/send` | typed endpoint mirror | `--dry-run`; MCP destructive-hint |
| 20 | Send push notification | /v3 `POST /messages/push/send` | typed endpoint mirror | `--dry-run` |
| 21 | Send ping notification | /v3 `POST /messages/notifications/ping` | typed endpoint mirror | `--dry-run` |
| 22 | List webhooks | /v3 `GET /webhooks` | typed endpoint mirror | offline list |
| 23 | Create webhook | /v3 `POST /webhooks` | typed endpoint mirror | `--dry-run` |
| 24 | Get webhook | /v3 `GET /webhooks/{webhook_id}` | typed endpoint mirror | terse default |
| 25 | Update webhook | /v3 `PATCH /webhooks/{webhook_id}` | typed endpoint mirror | `--dry-run` |
| 26 | Delete webhook | /v3 `DELETE /webhooks/{webhook_id}` | typed endpoint mirror | `--dry-run` |

**26 absorbed features.** All shipping scope, no stubs in this section.

## Stubs (the briefing's gap-flag protocol — `not supported by /v3`)

Per user briefing: *"flag the gaps — do NOT silently skip or fake the data. I'll authorize reverse-engineering as a follow-up if needed."*

These commands ship as Cobra commands with `--help` + an honest error message: `not supported by PushPress /v3 — covered by /v2 dashboard API; gated on follow-up browser-sniff (see [[ghl-cli-shipped]] + /v2 evidence in this run's discovery dir)`. They exist so an agent reading the SKILL knows the surface category exists but is not yet wired.

| # | Stub command | Why stubbed | What's needed to un-stub |
|---|---|---|---|
| S1 | `plans list` | `/v3` has no plans endpoint | `/v2/plans` via /v2 browser-sniff |
| S2 | `plans members <plan-id>` | same | `/v2/plans` + `/v2/client` cross-ref |
| S3 | `mrr today` | `/v3` has no billing/subscription endpoint | `/v2/billing`, `/v2/subscription` |
| S4 | `signups recent --days N` | `/v3` exposes `customers.dateAdded` but no source-attribution field | `/v2/activity` for source field |
| S5 | `cancellations recent --days N` | `/v3` has no cancellation/freeze surface | `/v2/billing` / `/v2/subscription` |
| S6 | `classes list` | `/v3` has no class-definition endpoint (class names embedded in `ClassCheckin` only) | `/v2/calendar` |
| S7 | `classes roster <class-id>` | same | `/v2/calendar` |
| S8 | `leads list` | `/v3` has no lead surface | `/v2/client` general |
| S9 | `tasks list --member <id>` | `/v3` has no task surface | `/v2/task` |
| S10 | `notes list --member <id>` | same | `/v2/task` / `/v2/communications` |

**10 explicit stubs.** Per Phase 1.5 stub rule, these are surfaced to the user separately in the gate showcase.

## Transcendence (only possible with our approach — 7 features, all ≥6/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| T1 | Going-dark report | `going-dark --days N` | 9/10 | Local SQLite join of synced `customers` × `checkins`; rows where `MAX(checkins.timestamp) < now - N days` AND `status = active`, or never checked in | Brief Top Workflow #1; Build Priority #2; PushPress dashboard's `/v2/calendar/report/no-show/*` proves operator demand |
| T2 | Recency ladder | `recency [--bucket 7,14,30,60,90]` | 8/10 | `GROUP BY` over days-since-last-checkin from local store; emits count + sample names per bucket | Brief Build Priority #3 (daily KPI ticker); no /v3 aggregation endpoint exists |
| T3 | Trainer roster (one-line) | `roster` | 7/10 | Local join `customers × MAX(checkin.timestamp)`; one line per row: id, name, plan, status, last_visit, days_since | Brief Top Workflow #4; trainer-dashboard named consumer; "one line per member" rule verbatim in brief |
| T4 | Daily KPI ticker | `kpi today` | 8/10 | One pass over local store: signups_today (from `dateAdded`), checkins_today (from `timestamp`), active_members, going_dark_14d count, going_dark_30d count; JSON by default | Brief Build Priority #3; business-dashboard cron named consumer |
| T5 | Member 360 | `member <id\|email>` | 7/10 | `/v3 GET /customers/{id}` (or local lookup) + local `checkins` for last 10 + current streak + cadence trend (last 4 weeks vs prior 4) | Brief Top Workflow #2; trainer-dashboard consumer; collapses 5 dashboard clicks |
| T6 | Cohort retention | `cohort --month YYYY-MM` | 6/10 | Local query: customers `dateAdded` ∈ month, then % with any checkin in days 0-30 / 0-60 / 0-90 post-join | Retention is canonical gym KPI; PushPress markets a churn report |
| T7 | Class-type mix | `class-mix [--days N]` | 6/10 | Histogram of `ClassCheckin.className` from local `checkins` over window; counts + % share | Brief coverage gap notes class names appear in `ClassCheckin` schema; only class-side signal possible without /v2 |

## Coverage of user's explicit 8 must-haves

| User must-have | Manifest coverage |
|---|---|
| 1. Members list+filters, search by email/phone/name | Absorb #1, #2, #3 (real); transcendence T3 roster, T5 member |
| 2. Signups recent + source | **Stub S4** — no source field in /v3 |
| 3. Cancellations recent + freeze-vs-cancel | **Stub S5** — not in /v3 |
| 4. Attendance + going-dark + per-member visit history | Absorb #4, #5 (real); transcendence T1 going-dark, T2 recency, T5 member-360 |
| 5. Classes + roster + instructor view | **Stubs S6, S7** + transcendence T7 class-mix (derived from check-ins) |
| 6. Plans + MRR + failed payments | **Stubs S1, S2, S3** — not in /v3 |
| 7. Tasks/notes | **Stubs S9, S10** — not in /v3 |
| 8. Leads + conversion | **Stub S8** — not in /v3 |

**Real coverage: must-haves 1 and 4 + partial 5 via class-mix.** Other 5 must-haves are stub-flagged. User explicitly authorized this shape via the gap-flag protocol.

## MCP surface plan

Total tools at runtime: ~26 endpoint mirrors + 7 transcendence + 10 stubs + ~13 framework = **~56 tools**. Past the 50 threshold, so the merged spec will use the Cloudflare pattern (`x-mcp.transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`).

## Ship vs hold checklist

- All 26 absorbed features ship shippable.
- All 7 transcendence features ship — they read from the local store the generator emits.
- 10 stubs ship as explicit "not supported by /v3" commands; user approves this shape per the brief.
- Auth: API key via `auth set-token` or `PUSHPRESS_API_KEY` env var.
- Reachability: confirmed (HTTP 401 with fake bearer).
- README's `## Known Gaps` section will name the 5 missing categories explicitly.
