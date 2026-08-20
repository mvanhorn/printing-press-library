# Novel Features Brainstorm — clarify (subagent audit trail)

## Customer model

**Priya — solo founder doing founder-led sales (seed-stage devtools startup)** — Clarify auto-builds her CRM; morning tab-hopping to prep the day; Monday pipeline review by eyeball. Frustration: no one-shot morning picture or "what did I drop" list.

**Marcus — AE at a 40-person startup, 6-10 external meetings a week** — pre-call prep is a four-tab dance; Friday scan for meetings with no follow-up. Frustration: everything exists in Clarify, spread across four screens; nothing flags silent meetings.

**Dana — RevOps/GTM engineer serving two sales pods** — bulk work via curl against /records/bulk; analytics = CSV export to spreadsheet (API can't OR across fields, no analytics endpoints); endless dupes from auto-capture.

**Codey — an AI agent (Claude Code) operating the CRM** — hosted MCP is online-only, multi-call (record → relationships → activities → comments → meetings → transcript), agents break on Bearer-vs-api-key. Wants one compact agent-shaped dossier.

## Candidates (pre-cut)

15 candidates: brief, prep, followup, stale, velocity, dossier, dupes, list lint, mentions, find (cross-field OR), orphans, schema diff, doctor, tasks due, funnel. C10/C14 cut inline (duplicate absorbed framework search; thin filter).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Morning briefing | `brief` | 8/10 | hand-code | Joins today's meeting rows to company/deal/activity tables in local SQLite; drain-first, hintIfStale | Morning-briefing is a Clarify identity ritual; hosted MCP can't do it offline | Use this command for a start-of-day overview across all meetings and deals. Do NOT use it to prepare for one specific meeting; use 'prep' instead. Do NOT use it to list stalled deals only; use 'stale' instead. |
| 2 | Meeting prep pack | `prep <meeting-id\|--next>` | 9/10 | hand-code | Attendees' person/company/deal rows + transcript-FTS excerpts for same company; live-fetch meeting if unsynced | Top Workflow #4; hosted MCP needs 4-6 calls for the same | Use this command to prepare for one upcoming meeting. Do NOT use it for a general record background bundle; use 'dossier' instead. |
| 3 | Follow-up gaps | `followup --since 7d` | 8/10 | hand-code | Anti-join meetings vs later activities/comments/tasks on linked deal/company; `--no-deal` flag covers orphan companies | Post-meeting follow-up is a core ritual; API has no gap query | none |
| 4 | Stale-deal report | `stale --days 14` | 8/10 | hand-code | Open deals grouped by stage where `_updated_at` older than threshold, local mirror | Build Priority #5 names it; API has no analytics endpoints | Use this command to find deals with no recent activity. Do NOT use it for a full daily overview; use 'brief' instead. |
| 5 | Pipeline velocity | `velocity` | 7/10 | hand-code | Stage-snapshot side table written during sync; per-stage dwell time + conversion counts | Build Priority #5; Dana exports CSVs to spreadsheets today | none |
| 6 | Record dossier | `dossier <record-id>` | 8/10 | hand-code | Record + relationships + activities + comments + related meeting/transcript refs in one compact --agent payload; live fetch on miss | Clarify is agent-first; hosted MCP is online-only/multi-call | Use this command for a complete background bundle on any record. Do NOT use it to prepare for a specific meeting; use 'prep' instead. |
| 7 | Duplicate finder | `dupes --type person` | 7/10 | hand-code | GROUP BY normalized email/domain/name over local mirror; emits ready-to-run merge command lines against the real merges endpoint | Auto-built CRM is dupe-prone by design; merges endpoint exists in spec | none |

All seven: `// pp:data-source` local or auto as noted, hintIfUnsynced/hintIfStale, drain-first SQLite.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Cross-field OR search (`find`) | Duplicates framework `search` FTS | dossier |
| Transcript mentions | Delivered by `search --type meeting` + excerpts in `prep` | prep |
| Tasks due | Thin filter over generated resources query; today's slice folds into `brief` | brief |
| Orphan companies | Ships as `followup --no-deal` flag | followup |
| Funnel conversion | Folded into `velocity` output | velocity |
| Schema diff | Monthly-at-best use | dupes |
| Dynamic-list SQL lint | Episodic + unverifiable list-SQL dialect semantics | velocity |
| Auth doctor | Setup-time only; auth-scheme gotcha handled natively by the client | dossier |
