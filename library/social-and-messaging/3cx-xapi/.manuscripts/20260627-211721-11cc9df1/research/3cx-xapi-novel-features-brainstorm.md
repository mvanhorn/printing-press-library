# 3CX XAPI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**1. MSP onboarding engineer** — Today: clicks through console hundreds of times/tenant, hand-copies golden tenants, can't diff or replay. Weekly: provision new client tenants (extensions, groups, ring groups, queues, rules, DID routing). Frustration: no idempotent bulk path, no tenant comparison, unauditable.

**2. IT admin / helpdesk lead** — Today: hunts across Users/RingGroups/InboundRules/OfficeHours to diagnose breakages. Weekly: triage routing/extension breakages, "what changed since yesterday". Frustration: no cross-entity view, no change history, no integrity check.

**3. Contact-centre supervisor** — Today: slow siloed per-queue reporting screens. Weekly: check queue SLA, abandoned calls, agent login/idle, callbacks. Frustration: no cross-queue rollup, no week-over-week comparison.

**4. Security/compliance admin** — Today: scattered Security and Logs screens. Weekly: review attack surface and admin changes. Frustration: no consolidated posture report, failed-auth buried, audit uncorrelated with config.

## Candidates (pre-cut)
14 candidates generated (C1–C14). Kept C1 audit, C2 snapshot/diff, C3 provision, C4 qrollup, C5 changed (descoped), C6 search, C7 posture, C10 trace. Killed C8 tenant-diff (folded into diff), C9 wallboard (scope creep/TUI), C11 rec-audit (verifiability), C12 did-map (subset of trace), C13 watch-drift (scheduler scope creep), C14 call-cost (no persona/demand).

## Survivors (transcendence table)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|------------------|
| 1 | Config integrity audit | `audit` | 9/10 | hand-code | IT admin/MSP | Use `audit` for graph-wide dangling-reference integrity. Do NOT use it for time-based config drift (use `diff`) or one extension's routing paths (use `trace`). |
| 2 | Config snapshot + diff | `snapshot` / `diff` | 9/10 | hand-code | MSP/IT admin | Use `diff` for config drift between two snapshots or tenants. For broken references now use `audit`; for live activity use `changed`. |
| 3 | Bulk provision from CSV | `provision` | 8/10 | hand-code | MSP | none |
| 4 | Queue/agent performance rollup | `qrollup` | 8/10 | hand-code | CC supervisor | Use `qrollup` for cross-queue rollups and week-over-week. For live calls use `now`/ActiveCalls; raw per-call rows use call-history list. |
| 5 | Offline directory search | `search` | 8/10 | spec-emits | all | Use `search` for fast fuzzy lookup across entity types. For exact OData lists use per-resource list; for references use `audit`/`trace`. |
| 6 | Security posture report | `posture` | 7/10 | hand-code | Security | Use `posture` for consolidated security/attack-surface. For raw event rows use the event-log list command. |
| 7 | Extension routing trace | `trace` | 7/10 | hand-code | IT admin | Use `trace` for routing paths into one extension. For broken references use `audit`; for free-text lookup use `search`. |
| 8 | Live state merge | `changed` | 6/10 | hand-code | IT admin | Use `changed` for the live activity/event/status merge over a recent window. For drift use `diff`; for security focus use `posture`. |

## Killed candidates
- tenant-diff → folded into `diff`
- wallboard --follow → scope creep (TUI); use `tail --resource ActiveCalls --follow`
- rec-audit → weak verifiability
- did-map → subset of `trace`
- watch-drift --cron → scheduler scope creep; use `snapshot`+`diff`
- call-cost rollup → no persona/demand; use framework `analytics` on CallCost endpoints

**Hand-code count: 7** (audit, snapshot/diff, provision, qrollup, posture, trace, changed). `search` is spec-emits (framework FTS5).
