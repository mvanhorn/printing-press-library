# TickTick Novel-Features Brainstorm (subagent audit trail)

## Customer model

**The daily-note ritualist.** A solo operator who runs their day out of TickTick; it is their external executive-function scaffold.
- Today: drives TickTick through the claude.ai MCP, which on 7/2 mangled habit IDs and on 7/7 corrupted his daily note (update sent `kind` → TEXT→NOTE flip, broken childIds). Babysits every agent write.
- Weekly ritual: morning agenda pull (note+tasks+habits+focus); focus-block logging; Friday Week in Review note.
- Frustration: no tool guarantees a daily-note update can't corrupt the note; agenda requires 4 separate calls.

**Agent — the orchestration consumer.** The operator's AI-agent sessions (daily-agenda and week-in-review skills) are the CLI's heaviest caller.
- Today: fans out across a 40+ tool MCP, re-fetches static data each session, must be prompted with defensive rules it sometimes ignores.
- Ritual: session briefing pull, mid-session note appends + focus logs, Friday synthesis.
- Frustration: no deterministic --json command per ritual; etag races make writes fragile.

**Dana — the streak-and-pomodoro quant.** ticktick-py / avilabss-cli audience.
- Today: habit/focus data only in mobile stats screens; V2 wrappers expose raw JSON, no streak math; V1 omits habits.
- Ritual: Sunday streak check, weekly focus hours by day/project, habits about to break.
- Frustration: hand-rolls streak math; no offline history.

## Candidates (pre-cut)
C1 note edit (safe daily-note edit) · C2 note show · C3 agenda · C4 review · C5 habits streaks · C6 focus stats · C7 note log · C8 note verify · C9 doctor · C10 tasks matrix · C11 tasks stale · C12 habits checkin-all · C13 tasks triage · C14 habits forecast

## Survivors
1. note edit 10/10 · 2. agenda 9/10 · 3. review 9/10 · 4. habits streaks 8/10 · 5. focus stats 7/10 · 6. doctor 7/10 — all hand-code. (Full table in absorb manifest.)

## Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C2 note show | Thin wrapper — tasks get + note-locate inside note edit/agenda covers reads | agenda |
| C7 note log | Same input surface as note edit --append; two write paths doubles corruption risk | note edit |
| C8 note verify | One-time forensic, not weekly ritual, once note edit is safe-by-construction | note edit |
| C10 tasks matrix | Speculative; search + --select reproduces it | review |
| C11 tasks stale | Generic garnish; one-line SQL via search | review |
| C12 habits checkin-all | Thin loop over absorbed #9 upsert | habits streaks |
| C13 tasks triage | No persona does inbox-zero ritual; duplicates search | agenda |
| C14 habits forecast | Speculative prediction, unverifiable in dogfood | habits streaks |
