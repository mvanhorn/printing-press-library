# NinjaOne Novel Features Brainstorm (audit trail)

## Customer model

**Dani — Patch & Compliance Tech (Tier 2, multi-tenant NOC)** — manages OS/software patch posture across ~40 client orgs, ~2,800 endpoints. Today: org-by-org console click-fest on Patch Tuesday. Weekly: per-org % patched + devices stuck on the same failed KB for the vCIO call. Frustration: no fleet-wide view; bulk scan/apply melts the API with 429s; recurring failed patches invisible without manual screenshot diffs.

**Marco — On-call / Alert-storm Triage Tech (Tier 1-2)** — first responder to the shared alert queue across all tenants. Today: resets alerts one-by-one; can't separate a real incident from noise. Weekly: Monday clear-out of the weekend's accumulated alerts. Frustration: can't tell "50 devices at one site offline (one incident)" from 50 unrelated alerts; no reset-by-condition/org/window; nightly flappers never surfaced.

**Priya — Fleet Health / Onboarding Lead (Tier 3 / team lead)** — owns fleet hygiene, drift, onboarding. Today: manual org-by-org audits of stale/offline/AV-not-reporting/disk-full devices. Weekly: fleet-hygiene sweep (offline >30d, missing required custom fields, policy drift). Frustration: drift invisible; maintenance-mode prep manual and boxes get left stuck in maintenance mode.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|------------------|
| 1 | Cross-org patch-gap report | `patch-gaps [--severity] [--org]` | 9/10 | hand-code | Dani | none |
| 2 | Patch-stuck detector | `patch-stuck [--cycles N]` | 7/10 | hand-code | Dani | Use for KBs failing repeatedly over time. For a point-in-time list of all current gaps use `patch-gaps`; for fixing them use `patch-sweep`. |
| 3 | Throttled cross-org patch sweep | `patch-sweep --df <filter> [--dry-run]` | 8/10 | hand-code | Dani | Use to scan+apply patches across a cohort. This mutates devices; for a read-only report use `patch-gaps`. Post-apply reboots handled here. |
| 4 | Alert-storm clustering | `alert-storms [--window]` | 8/10 | hand-code | Marco | none |
| 5 | Bulk alert reset by query | `alert-reset --where <cond\|org\|age> [--dry-run]` | 7/10 | hand-code | Marco | Use to reset many alerts by predicate. To understand the storm first use `alert-storms`; this command mutates. |
| 6 | Flapping-alert detector | `alert-flappers [--window]` | 6/10 | hand-code | Marco | none |
| 7 | Stale/offline device sweep | `stale [--offline-days N] [--reboot]` | 7/10 | hand-code | Priya | Use for devices gone quiet fleet-wide. For policy deviation use `drift`; for missing field values use `cf-hygiene`. |
| 8 | Custom-field hygiene | `cf-hygiene --require <field...>` | 6/10 | hand-code | Priya | none |

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| Policy/config drift (`drift`) | "Standard policy" not reliably derivable from API data without inference; fails verifiability | `cf-hygiene` |
| Maintenance-window orchestration (`maint-window`) | Durable value is auto-clear, which needs a background process; one-shot degrades to a wrapper | `stale` |
| Fleet inventory rollup (`inventory-rollup`) | Re-skin of shipped generic `analytics` group-by | (framework `analytics`) |
| Client onboarding scaffold (`onboard`) | Application-scope creep; descoped = thin sequence of create endpoints, no local-store leverage | — |
| AV/threat exposure sweep (`av-exposure`) | Thin filter over absorbed AV queries; delivered by framework `search`/`analytics` | (framework `search`) |
| Reboot-needed cohort (`reboot-pending`) | Overlaps `patch-sweep` post-apply reboots + generic device search | `patch-sweep` |

**Note (the survivor `stale` (`stale`) reframe was carried, not killed — the original `drift` candidate referenced in survivor #7's Long Description was killed, so that redirect is revised below.)**
