# Sybill CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Dana — Sales Manager, mid-market SaaS (8-rep team)**
Today: Monday "deal review" — manually opens Sybill web UI, clicks each rep's calls, reads AI summaries, copies next-steps into a doc. Weekly ritual: the call digest. Frustration: UI is one-call-at-a-time; no "everything this week grouped by deal" view. 90 min of clicking.

**Marcus — Account Executive, enterprise cycles (15-20 open deals)**
Today: long cycles where deals stall silently; finds out a deal went cold when his manager asks. Weekly ritual: Friday pipeline hygiene — open deals with no activity in N days. Frustration: no cross-entity query; joining deals to conversations by hand is effectively impossible.

**Priya — RevOps / Sales Ops analyst**
Today: maintains CRM hygiene, feeds call intel into dashboards. Weekly ritual: review crmAutofill suggestions before push; grep transcripts for competitor/pricing/legal across the team. Frustration: crmAutofill exists in payload but no reviewable diff; transcript search is one-call-at-a-time.

**Sam — Founder/AE early-stage (every hat)**
Today: closes deals personally, lives in terminal + agents. Weekly ritual: account roll-up for renewals/expansions; pipes call data into agent workflows. Frustration: rate limits slow full pulls; no SDK/CLI; every integration hand-rolled.

## Candidates (pre-cut)
(See Survivors and kills below for verdicts. 14 candidates generated across sources a/b/c.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Weekly call digest | `digest --since 7d` | 8/10 | Groups cached conversations by linked deal over a date window, extracts AI-summary + next-step fields per deal — local join, no LLM | Top Workflow #1; Dana |
| 2 | Deals gone dark | `deals dark --days N` | 9/10 | SQL MAX(conversation.date) per open deal in SQLite, filters last activity > N days — join the API can't answer | Top Workflow #2; Marcus |
| 3 | CRM autofill pending diff | `crm-autofill [--deal ID]` | 8/10 | Extracts crmAutofill suggestion objects from cached deal detail, renders field-by-field diff | Top Workflow #3; Priya |
| 4 | Account roll-up | `account rollup ID` | 7/10 | Joins accounts + conversations + deals + contacts: call count, open deals by stage, contacts, last activity | Top Workflow #5; Sam |
| 5 | Rep/owner activity aggregation | `activity --by owner --since 7d` | 6/10 | SQL group-by over conversations/deals by owner: calls, deals touched, deals gone dark per rep | Build Priority 2; Dana |
| 6 | Keyword/objection pattern report | `patterns --term <kw>` | 6/10 | FTS5 match-count over cached transcripts grouped by deal + stage (+closed/won) | Top Workflow #4; Priya |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Transcript FTS search/grep | Already absorbed feature #15 — not novel | patterns --term |
| Next-step tracker | Subsumed by digest (already extracts next-steps by deal) | digest |
| Deal momentum / velocity | Not buildable: no stage-change history / transition timestamps in payload | deals dark |
| Account engagement score | Speculative, overlaps rollup + activity, arbitrary weighting | account rollup |
| Contact directory | Low-pain utility not a workflow; contacts surface in rollup already | account rollup |
| Recording-URL refresh | Thin wrapper over absorbed conversation-detail GET | absorbed #2 |
| Sync-status report | Folded into absorbed sync command #16 | absorbed #16 |
| Open deals with zero calls | Sibling of deals dark; fold in as --include-uncovered flag | deals dark |
