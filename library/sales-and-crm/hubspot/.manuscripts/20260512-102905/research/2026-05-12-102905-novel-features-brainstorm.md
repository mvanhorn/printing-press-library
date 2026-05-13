# Novel Features Brainstorm — HubSpot CLI (audit trail)

## Customer model

### Persona 1: Jay, solo operator at Simple Path Media

**Today:** SMB automation agency. Sells marketing automation to contractors, HVAC techs, realtors, home services. Keeps HubSpot in browser + Apollo dashboard + Google Maps scraper output sheet + Claude Code. Morning scan of overnight n8n drops. Opens contact records one-by-one to check duplicates. Eyeballs pipeline for stalls. Pokes filtered views for two-week-stale leads. Cannot answer "of leads Apollo pushed in last night, which sources actually converted?" without CSV export + manual join.

**Weekly ritual:** Monday pipeline review (every deal, every stage, age + last-touch). Stale-lead sweep. Daily intake review (last 24-48h with UTM/source + duplicate check). End-of-week Closed Won handoff (full property bundle, ClickUp-import shape).

**Frustration:** Compound questions ("deals in Proposal older than 14 days with no engagement, sorted by weighted value") require chaining three filtered views + mental math. UI cannot answer in one place. Official MCP loads 80 tools and still requires multi-call orchestration.

### Persona 2: n8n automation flow

**Today:** Hourly. Pulls Apollo enrichment, hits Google Maps for new realtors, calls HubSpot Search API directly with bearer token, paginates 100, writes contacts. No local cache — every "is this a duplicate?" check is a live API call against email + phone + domain (three round-trips per lead, 90% of daily rate-limit budget on bad days).

**Weekly ritual:** Continuous: ingest leads, dedup against HubSpot, create contact, attach UTM, attach to list, log note. Fires hourly. Idempotency depends on duplicate detection.

**Frustration:** Rate-limit pressure from dedup probing. No good way to ask "last 24h of contacts from source=apollo with UTM=google-maps-scraper" without re-implementing filter + projection per call.

### Persona 3: Claude Code session on Jay's laptop

**Today:** Jay says "review pipeline and flag stale leads." Claude either calls official HubSpot MCP (80 tools, multi-call to filter+sort+project) or shells `curl` with hand-rolled filterGroups JSON. Each compound query is 4-6 API calls minimum. Verbose JSON eats context.

**Weekly ritual:** Ad-hoc analysis. "Apollo leads last week not yet emailed." "Oldest deals in Proposal." "Any new contacts duplicates by email/phone?" Each session is 3-10 chained questions.

**Frustration:** Token economy. Official MCP burns ~12k tokens just to load. Each filtered search returns verbose JSON. Compound queries require orchestration that eats tokens and produces inconsistent results.

## Candidates (pre-cut)

[See subagent output — all 16 candidates with verdicts inline; C8 sync (framework), C13 notes-semantic (LLM-dep + already covered by FTS) cut inline.]

## Survivors and kills

### Survivors (9 features ≥7/10)

| # | Feature | Command | Score | How It Works |
|---|---------|---------|-------|--------------|
| 1 | Stale leads | `stale-leads --stage <name> --days <N>` | 10/10 | Local join contacts + 5 engagement tables; lifecyclestage filter; max(engagement_ts); sort desc |
| 2 | Pipeline health | `pipeline-health [--owner <id>]` | 10/10 | Local deals + pipelines.stages.probability; emits age-in-stage, weighted value, days-since-touch |
| 3 | Recent intake | `recent-intake --hours 24 [--source <s>]` | 10/10 | Contacts WHERE createdate > now-N; projects hs_analytics_source* + utm_* family |
| 4 | Dedup check | `dedup [--key email\|phone\|domain]` | 10/10 | Local GROUP BY normalized email/e164-phone/registrable domain |
| 5 | Closed Won handoff | `closed-won-handoff --since <date> [--format clickup]` | 9/10 | lifecyclestage = customer transition via property-history; joins deals + companies + engagements; ClickUp-import JSON |
| 6 | Engagement decay | `engagement-decay [--days 30] [--window 7]` | 10/10 | Two-window comparison of engagement counts per contact; ranks by negative delta |
| 7 | Lead trace | `lead-trace <contact-id>` | 8/10 | Walks associations contact→deals→companies + engagement timeline + source attribution; composite JSON |
| 8 | UTM cohort | `utm-cohort --campaign <name>` | 9/10 | GROUP BY utm_campaign joined to deals.dealstage; cohort size, %-by-stage, %-Closed-Won, avg amount |
| 9 | Daily digest | `digest [--since 24h]` | 7/10 | Runs C1/C2/C3 in sequence over local store; composite JSON (new + advanced + closed + stalest + recent) |

### Killed candidates

| Feature | Kill reason | Closest-surviving sibling |
|---------|-------------|---------------------------|
| C7 property-history | Thin wrapper over single endpoint; fails wrapper-vs-leverage | C5 closed-won-handoff (uses property history) |
| C8 sync resume | Framework (`sync`), not novel | (framework) |
| C10 owner-workload | Jay is solo operator — no team to slice | C2 pipeline-health |
| C11 list-audit | Monthly cadence, not weekly | C8 UTM-cohort |
| C13 notes-semantic | LLM dependency; covered by FTS | C9 lead-trace |
| C15 rate-budget | HubSpot exposes no remaining-daily; partial signal | (none — adaptive limiter handles it) |
| C16 merge-plan | Merge endpoint requires write scope (out of read-only scope set); verifiability low | C4 dedup |
