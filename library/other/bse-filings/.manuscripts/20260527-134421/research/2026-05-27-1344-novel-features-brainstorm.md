# BSE Filings — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona A — "Council subprocess" (IMstockbox bot fleet: Opus / Gemini / GPT-5.4)**
- Today: no first-party BSE access; bots hallucinate or ask the operator; each re-fetches/re-parses the same 40-page PDFs, burning tokens.
- Weekly ritual: during earnings season, debate buy/hold/trim; want to cite the exact concall sentence where guidance softened, as a chainable tool call not scraped prose.
- Frustration: no agent-shaped surface; no one-call "which of 19 holdings filed a Reg-30 critical item this week" (BSE forces one scrip at a time).

**Persona B — Rushyant, solo portfolio operator (terminal)**
- Today: tracks 19 scrip codes by hand; clicks bseindia.com per company; PDFs rot in Downloads; tone shifts live only in memory.
- Weekly ritual: Saturday AM (Mon–Sat IST) scan for results due next week, read latest concalls, try to recall if confidence sounds different.
- Frustration: BSE can't answer compound questions ("results due in 10d AND board meeting flagged"; "did 'demand' drop out of auto-sector calls"). 8 quarters trapped in PDFs; thesis decay invisible until the stock moved.

**Persona C — Equity-research analyst (cross-sector reader)**
- Today: reads concalls one at a time, tallies sector themes by hand.
- Weekly ritual: after a results wave, write a sector note; line up outcome numbers vs management language; spot the phrase appearing across 5 companies at once.
- Frustration: 2-wave results reconciliation is manual; no portfolio/sector-wide transcript grep; aggregators summarize single companies only.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Persona |
|---|---------|---------|-------|---------|
| 1 | Portfolio concall grep | `concall-grep <phrase> [--sector X] [--quarter QN]` | 9 | C, A |
| 2 | Thesis drift | `thesis-drift <scrip> [--terms a,b,c] [--last N] [--all]` | 9 | B, A |
| 3 | Cross-holding phrase sweep | `cross <phrase> [--min-holdings 2]` | 8 | C |
| 4 | Due-soon calendar | `due-soon [--days N] [--kind results,board,agm]` | 8 | B, A |
| 5 | Results outcomes (2-wave join) | `outcomes [--quarter QN] [--beat\|--miss]` | 8 | C, B |
| 6 | Stale-thesis scan | `stale [--days N]` | 7 | B |
| 7 | Critical-news watch | `critical [--days N]` | 8 | A, B |
| 8 | Concall extract (feeder) | `concall <scrip> [--quarter QN] [--mentions phrase]` | 6 | A, B |

## Killed candidates
| Feature | Kill reason | Sibling |
|---|---|---|
| Filing digest by category | GROUP BY over announcements feed, no leverage | concall-grep |
| Quarter coverage report | debug view, not weekly action | stale |
| Drift digest (portfolio) | same compute as thesis-drift → `--all` flag | thesis-drift |
| OCR-need flag | one-liner → warning in concall parse | concall |
| New-filing diff since cursor | undifferentiated changelog | critical |
| Sector roll-up | counts with no actionable signal | cross |
