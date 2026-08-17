# Novel Features Brainstorm — subagent audit trail (faa-novel-features)

## Customer model

- **Omar, fractional-owner tail-watcher (primary user):** looks up tails from his NetJets itinerary/ADS-B in the browser; wants `N16QS` in → typed JSON out incl. Other Owner Names + Temporary Certificates. Weekly: verify tails he flies/spots; occasionally pull whole NetJets fleet.
- **Dana, ADS-B/planespotting hobbyist:** logs Mode S hex codes; resolves one at a time via converter tab + FAA site. Weekly: batch-resolve the week's hex captures. Frustration: no offline authoritative batch hex→tail→record path.
- **Marcus, pre-purchase/title researcher:** parallel FAA forms (serial, doc index, dereg) + manual expiration math. Weekly: due-diligence sweep on tails/serials under negotiation. Frustration: ownership arc split across MASTER/DEREG with no join.
- **Priya, aviation journalist/fleet analyst:** owner-name inquiry then manual spreadsheet for composition. Weekly: profile one operator's fleet. Frustration: no aggregate answer anywhere.

## Candidates (pre-cut) — 14 candidates C1-C14

C1 live lookup (absorbed row 1), C2 batch hex resolve (keep), C3 ownership history (keep), C4 fleet report (keep), C5 expiring (keep), C6 availability (absorbed row 14), C7 fleet diff (flagged; killed in Pass 3 — "depends" weekly use, collapses into C4 + absorbed watch row 13), C8 hex block report (cut: speculative, fails Research Backing), C9 reserved vanity finder (cut: invented demand), C10 fleet stats (folded into C4), C11 by-serial (absorbed row 2), C12 state/county census (absorbed row 8), C13 models fleet (keep), C14 doc-index sweep (absorbed row 7).

## Survivors and kills

### Survivors (scores: Domain Fit /3 + User Pain /3 + Build Feasibility /2 + Research Backing /2)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Fleet composition report | `fleet report --owner "NETJETS SALES INC"` | 10/10 | MASTER×ACFTREF local join: count, model mix, engine-class split, avg seats/year | Brief workflow #2; Priya frustration |
| 2 | Batch Mode S hex resolution | `hex resolve --stdin` | 10/10 | stdin batch → local Mode-S-hex index join, algorithm fallback | Brief workflow #3; Dana |
| 3 | Aircraft ownership history | `aircraft history <N>` | 9/10 | MASTER + DEREG chronological owner timeline | Brief workflow #5; Marcus |
| 4 | Expiring registrations | `expiring --owner X --within 90d` | 9/10 | Local expiration-date query | Brief workflow #4 |
| 5 | Model-class fleet breakdown | `models fleet --make CIRRUS --model SR22` | 8/10 | MASTER aggregation by model + registrant type + state | Priya/Marcus |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C7 fleet diff | Weekly use "depends" on retained snapshot; collapses into C4 + absorbed watch | C4 |
| C8 hex block report | Speculative, no surfaced demand | C2 |
| C9 reserved vanity finder | Invented demand, tangential | C5 |
| C10 fleet stats | 80% overlap, folded into C4 | C4 |
| (C1, C6, C11, C12, C14 were absorb-manifest rows, not novel candidates) | | |
