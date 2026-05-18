# SmartLead CLI — Novel Features Brainstorm (audit trail)

See the absorb manifest transcendence table for the 6 survivors. Full subagent
output below.

## Customer model

**Persona 1 — Matt, the AI-operator running the Ozark link-building machine.** Runs
SmartLead inside the link-building project; pulls state via ad-hoc Node scripts.
Weekly ritual: campaign health checks, reply triage, dedupe a new lead batch against
every prior campaign before launch. Frustration: every "which leads went silent / is
this domain already pitched / which sender is dragging deliverability" question costs
a multi-call API loop; double-pitch incidents happened with no cross-check.

**Persona 2 — Agency growth operator running cold outbound at scale.** Lives in the
SmartLead dashboard, clicks campaign-by-campaign. Weekly ritual: Monday triage across
10-40 campaigns. Frustration: no cross-campaign view, no week-over-week drift.

**Persona 3 — Deliverability owner managing the sender pool.** Checks warmup-stats
per account one at a time, reacts to bounces after a campaign tanks. Weekly ritual:
audits the sender fleet before launch. Frustration: warmup readiness is a manual
per-account judgment call; no single launch gate.

## Killed candidates
- seq-audit (4/10, per-step stats granularity unconfirmed)
- triage (categories + message-history already absorbed; offline ranking too thin)
- stale (absorbed into `health` as a stale flag)
- bounce-map (absorbed into `sender-health` as by-domain bounce column)
- sender-load (absorbed into `sender-health` as a ranking column)
- cat-breakdown (trivial `sql`/`search` one-liner)
- preflight (scope creep — orchestration of warmup-gate + dupes; ships as a runbook)
- thread (thin wrapper over absorbed message-history endpoint)
- domain-log (absorbed into `dupes --domain`)
