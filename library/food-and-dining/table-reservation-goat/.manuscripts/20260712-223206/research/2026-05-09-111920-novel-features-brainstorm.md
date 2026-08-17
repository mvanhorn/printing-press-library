# Novel Features Brainstorm — Table-Reservation-GOAT

## Customer model

**Persona A: Daniela, the cancellation hunter.** Hardcore foodie in Chicago. Her birthday is six weeks out and she wants Alinea (Tock) and Smyth (OpenTable). Both are fully booked.
- **Today:** Has the Alinea Tock release calendar bookmarked. Refreshes Smyth's OpenTable page on her phone during meetings. Reads r/finedining for "I just got an Alinea cancellation at 11pm on a Tuesday" posts. Has tried tockstalk once, gave up because it's a Cypress test runner. Cannot answer: "if I'm flexible on Friday OR Saturday and party 2-4, has anything moved in the last hour across either site?"
- **Weekly ritual:** 3-5x/day she manually checks Tock + OpenTable for 2-3 target restaurants. Sundays she scans her wishlist on both sites for new experience drops.
- **Frustration:** No persistent watch she trusts. Resy's Notify works but doesn't cover OT or Tock. Every refresh is manual.

**Persona B: Marcus, the agent operator.** Power user who delegates dinner planning to Claude. "Find me 4 covers Friday 7-9pm West Village, $$$ or under, prefer patio, fall back to bar." Wants the agent to just do it.
- **Today:** Pastes screenshots of OpenTable into Claude. Asks Claude to "search Tock for me" and Claude can't. Builds custom MCP wrappers but each one covers one network and breaks on persisted-query hash drift. Cannot answer programmatically: "across both networks, what's earliest at any of these 5 venues?"
- **Weekly ritual:** Friday/Saturday booking, often 2-3 dinners/week + business travel reservations.
- **Frustration:** No agent-shaped tool that hits both networks with structured JSON he can pipe into a planning loop. Authentication is a manual cookie-paste nightmare.

**Persona C: Priya, the points-and-experiences optimizer.** OpenTable Dining Rewards member with 12k points; Tock loyalty member at three venues. Travels for work, eats out 4-5 nights/week.
- **Today:** Two browser sessions, two histories, two points balances. Tracks dining frequency in a Notes app. Has no idea which cuisines/neighborhoods she actually frequents most, or whether her OT points expire faster than she earns them. Cannot answer: "where have I eaten 5+ times in the last year, and which of those have a Tock experience drop next month?"
- **Weekly ritual:** Monday review of upcoming reservations + points balance; ad-hoc booking 2-3x/week.
- **Frustration:** Two separate histories that never merge. Can't see her actual dining footprint.

**Persona D: Aaron, the trip planner.** Plans a 4-night trip to Tokyo or Napa. Wants to lock in headline dinners 30+ days out.
- **Today:** Spreadsheet with 12 candidate restaurants across both networks, manually checking nightly availability one-by-one. Cannot answer in one shot: "across these 12 venues, what nights of my trip have ≥1 slot at party 2 within my window?"
- **Weekly ritual:** Active during trip-planning weeks (4-6x/year), but the workflow is gnarly enough that he abandons half his targets.
- **Frustration:** No multi-venue × multi-night matrix view exists. He pastes screenshots into a spreadsheet.

## Candidates (pre-cut)

(Full pre-cut list, 14 candidates, source-labeled, with inline kill/keep verdicts — see SURVIVORS table below for the kept ones.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|----------|
| 1 | Cross-network unified search-and-rank | `goat <query>` | 10/10 | Marcus | Concurrent OT GraphQL `Autocomplete`+`RestaurantsAvailability` and Tock SSR + `calendar.offerings`; merged on local `restaurants` + `availability_slots`; ranked by relevance × earliest-slot × price band | Brief Top Workflow #1; 5+ wrappers cover one network only |
| 2 | Local cross-network cancellation watcher | `watch add/list/cancel/tick` | 10/10 | Daniela | Local SQLite `watches` table; `tick` reads watches, calls OT availability + Tock SSR per source's adaptive limiter; matches against window, persists hits, fires local notification + optional webhook; `--launch` opt-in to auto-book | Brief Top Workflow #4; tockstalk Cypress + Henrymarks1 OT bot + restaurant-mcp snipe (single-network only) all confirm demand |
| 3 | Multi-venue earliest-available | `earliest <slugs> --party N --within Nd` | 9/10 | Aaron, Marcus | Composes N parallel availability calls + Tock SSR; per-venue minimum slot timestamp + slot tokens | Brief Build Priority #5; absorb covers single-rest availability only |
| 4 | Trip-planner availability matrix | `matrix <slugs> --nights <range> --party N` | 8/10 | Aaron | Composes (venues × nights) calls across both networks; tabulates ≥1-slot booleans + earliest slot per cell; CSV/JSON grid | Trip-planner persona; absorb #11 is single-restaurant multi-day |
| 5 | Cross-network dining-history insights | `insights [--year YYYY] [--by cuisine\|neighborhood\|venue]` | 8/10 | Priya | Local SQLite GROUP BY across merged `reservations` from OT `UserNotifications`/past-res + Tock `pastPurchase`/`history`; emits top-N per dimension + counts/avg party | Brief Build Priority #5; data-layer section explicit |
| 6 | New-experiences drop feed | `experiences drops --since 7d --within 30d` | 9/10 | Priya, Daniela | Diffs `experiences` snapshots; emits new since `--since`, with availability join | Brief calls Tock experiences "Tock's specialty"; OT users miss them entirely today |
| 7 | Per-restaurant change feed | `drift <restaurant> [--since <ts>]` | 8/10 | Daniela | Snapshot diff of `availability_slots`+`experiences`+restaurant fields | Brief Build Priority #5 explicit |
| 8 | My-saved-list × live availability | `wishlist availability [--party N] [--within Nd]` | 9/10 | Daniela, Priya | Reads merged wishlist (OT `UserWishlist` + Tock saved/loyalty); runs availability for each; emits matched venues | No wrapper does cross-network "what on my list opened up?" |
| 9 | Unified loyalty-and-credits status | `loyalty status` | 7/10 | Priya | Authenticated reads of OT `pointsForDiscount`/`pointsRedemptions` + Tock `loyaltyProgram`/`giftCard`; merged with computed expiry buckets | Absorb #28/#31 are single-network reads; unification + expiry computation is novel |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `pp tonight --party N` | Already in absorb manifest row 32; subsumed by `goat --when tonight` | `goat` (#1) |
| `pp dupes` cross-network duplicate finder | One-time exploration query, not weekly; results already surface in `goat` output | `goat` (#1) |
| `pp gaps <neighborhood>` OT-vs-Tock coverage diff | Meta-analytical, infrequent ritual; no persona runs this weekly | `matrix` (#4) |
| `pp footprint --year` heatmap | Strict subset of `insights`; no distinct command shape worth keeping | `insights` (#5) |
| `pp michelin --available` accolades × live availability | Same join shape as `wishlist availability` with weaker weekly hook | `wishlist availability` (#8) |
