# Novel Features Brainstorm — zoho-campaigns (audit trail)

(Full subagent response, persisted per novel-features-subagent.md output handling.)

## Customer model

### Persona 1 — Kent, Global Marketing Director (human operator)
**Today:** Logs into campaigns.zoho.com, opens Reports campaign by campaign (~44), hand-copies opens/clicks/bounces into the Kontur Marketing Dashboard note. The claude.ai Zoho connector keeps hitting recertification, so automations die. Cannot answer "did opens keep climbing after day 2?" or "is the main list growing or bleeding?" — Zoho shows current-state only.
**Weekly ritual:** Pull campaign performance for recent sends, check list counts, summarize "what went out this month."
**Frustration:** Reports are point-in-time only; no trajectory, no delta, no list growth history.

### Persona 2 — the Dashboard agent (kontur-marketing-dashboard skill)
**Today:** No working Zoho Campaigns path — May 2026 CLI failed on bad /json/get* paths; hosted MCPs need interactive auth.
**Ritual:** Refresh dashboard: campaigns sent, aggregate rates, list totals/growth as compact structured data.
**Frustration:** Needs one call returning the whole rollup; a dozen paginated pulls risks the 500/5min budget and 30-min lockout.

### Persona 3 — the Daily-brief agent (6:00 AM headless)
**Today:** No email-campaign section in the brief — no durable headless auth.
**Ritual:** Daily "what changed in 24h" — sends, metric movers, list shrinkage.
**Frustration:** Change-since-yesterday requires yesterday's numbers to exist; only a local snapshot store provides that.

### Persona 4 — the Intake agent (webinar/trade-show CRM intake)
**Today:** Attendees reach CRM; Campaigns list membership is manual. Nobody audits which imports later bounced.
**Ritual:** Per event: subscribe attendees, suppress do-not-mail, confirm imports didn't poison deliverability.
**Frustration:** No way to ask "which recently added contacts bounced or never engaged" without opening every campaign report.

## Candidates (pre-cut)
C1 delta (keep) · C2 digest (keep) · C3 growth (keep) · C4 sendlog (soft) · C5 benchmark (soft) · C6 engagement (keep) · C7 bounce-audit (keep) · C8 journey (keep) · C9 overlap (soft) · C10 unsub-report (soft) · C11 brief-feed (soft) · C12 quota (reframe) · C13 send-time (kill: verifiability at n=44) · C14 auth-doctor (kill: framework reimplementation) · C15 export-engaged (kill: wrapper over engagement + --csv/--select)

## Survivors and kills
Survivors (all hand-code, pp:data-source local): delta 10/10, digest 9/10, growth 9/10, engagement 8/10, bounce-audit 8/10, journey 7/10 — full table in absorb manifest.

### Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C4 Send log | Projection of digest's output over the same local join | digest |
| C5 Campaign benchmark | Occasional-use; trailing-average baseline thin at ~44 campaigns | delta |
| C9 List overlap | Too few lists at Kontur; monthly curiosity | journey |
| C10 Unsub bleed report | Single sort over columns delta/digest already expose | delta |
| C11 Daily brief feed | Identical to digest --since 24h --agent | digest |
| C12 Rate-limit ledger | Value is automatic throttling in the sync client, not a command | digest |
| C13 Best send time | Statistically meaningless buckets at n=44; misleading | bounce-audit |
| C14 Auth doctor | Duplicates framework self-client auto-refresh plumbing | none — infrastructure |
| C15 Engaged-contact export | Thin wrapper over engagement + global --csv/--select | engagement |
