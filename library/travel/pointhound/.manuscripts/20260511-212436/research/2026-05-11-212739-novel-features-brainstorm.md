# Novel-Features Brainstorm — Pointhound

(Full subagent output preserved for retro/dogfood debugging audit trail.)

## Customer model

**Persona 1: Renee, the Chase-UR-and-Bilt travel hacker (mid-30s, urban professional)**

*Today:* Renee holds ~250k Chase Ultimate Rewards, ~80k Bilt, and ~120k Amex MR across personal and business cards. She manually checks point.me, seats.aero, and PointsYeah each weekend, copying promising redemptions into a Notes app and re-checking 48 hours later because awards "change constantly; book within 24-48h."

*Weekly ritual:* Sunday morning she runs the same 4-5 high-priority routes (SFO-NRT business, JFK-CDG business, SFO-LHR business, SFO-HND for the parents) across three sites, then transfers points only after she's confirmed seats are still there on a Monday-evening recheck. She also re-pulls the same routes Tuesday and Thursday for new openings.

*Frustration:* Three things compound: (a) re-typing identical searches into three websites, (b) never knowing whether a deal she sees Tuesday is *new* vs the same Saturday inventory, and (c) Chase UR transfers to so many partners that comparing "this route via Air Canada Aeroplan vs United MileagePlus vs Air France Flying Blue" requires juggling three tabs and a mental ratio table.

**Persona 2: Marcus, the points hobbyist trip planner (40s, parent of two, 3 destinations/year)**

*Today:* Marcus is not optimizing — he's planning. He has a sticky note that says "fall 2026 family trip: Italy, Japan, or Hawaii?" and roughly 600k transferable points across Amex MR + Capital One Miles + Citi TY. He cares about the *cheapest of any of those*, in business class, sometime October-December, across 4 award seats.

*Weekly ritual:* He doesn't really have one — he opens Pointhound when he remembers, runs one-off searches that always feel ad-hoc, and then forgets what he saw last time. He'd love a year-view "where can I go in business for 4 with what I have?" but no site really answers that without 50 manual searches.

*Frustration:* Lack of memory and lack of fan-out. Every search starts from scratch; he can't tell that the JFK-FCO redemption he saw two weeks ago has gotten better or worse; and he can't ask "of all the cities Pointhound calls 'high deal rating' from my home airport (BOS), which has the cheapest business award between October and December?"

**Persona 3: Priya, the AI-augmented automation user (30s, treats Claude as an assistant)**

*Today:* Priya runs a personal-agent setup that fires off cron-style checks (mail, calendar, stocks, packages). She wants her agent to monitor 8-12 award flight watches in the background and surface only deltas — "new deal on the Tokyo route" or "the Athens redemption got 15k points cheaper" — without her having to look at a website.

*Weekly ritual:* Her agent runs scheduled `--json --quiet` queries against various CLIs each morning. Output that prints anything when there's nothing new is a bug from her perspective. She also asks her agent ad-hoc questions like "what's the cheapest way to get from BOS to anywhere in Asia in March using my Chase UR" and expects a structured answer, not a webpage.

*Frustration:* No award-search service has an agent-shaped surface. seats.aero has an API but only for Pro users and only for current state — no delta, no local store, no SQL. She wants the local SQLite + agent-quiet output combo, with terse exit codes (2 = new deal exists) so cron can branch on it.

## Candidates (pre-cut)

| # | Candidate | Source | Persona | Notes / Kill-keep |
|---|-----------|--------|---------|-------------------|
| 1 | `watch <route>` — register a saved route in local SQLite; subsequent runs poll `/api/offers` and exit 2 if new or improved offers appear since last snapshot | (a) | Renee, Priya | Keep |
| 2 | `drift <route>` — show points-cost delta between last two snapshots for a saved route | (c) | Renee, Marcus | Keep |
| 3 | `compare-transfer <earn-program> <route>` — list every viable redemption with effective points cost after transfer ratio, sorted by source-program points | (c) | Renee | Keep |
| 4 | `batch <route-csv>` — fan out search across N tuples and store all results | (a) | Renee, Priya | Keep |
| 5 | `top-deals-matrix --origins ... --dests ... --months ...` | (b) | Marcus | Keep |
| 6 | `from-home <origin> --balance "ur:250000,..."` — reachability from balance | (c) | Marcus | Keep |
| 7 | `explore-deal-rating --metro <code>` — Scout dealRating-driven explore | (b) | Marcus | Keep |
| 8 | `cards`, `cards get <slug>`, `points101` — SSR-scrape static reference | (b) | Renee, Marcus | Keep as static reference |
| 9 | `program-coverage <route>` — taxonomy of (airline, redeem, transfer-source) triples | (c) | Renee | Sibling-killed by #3 |
| 10 | `summarize-recent --since 7d` — LLM writeup | LLM-dep | n/a | Kill |
| 11 | `predict-price <route>` — ML/LLM prediction | LLM/verifiability | n/a | Kill |
| 12 | `book <offer-id>` — POST to booking endpoint | scope/auth | n/a | Kill |
| 13 | `calendar --route <r> --cabin <c>` — month heatmap | (a)(c) | Marcus, Renee | Keep |
| 14 | `compare-vs-other <route>` — cross-CLI seats.aero/point.me | external auth | n/a | Kill |
| 15 | `alerts list/create/delete` — Pointhound account alerts | thin wrapper | Renee | Reframe (endpoint mirror, not novel) |
| 16 | `transferable-sources <redeem-program>` — list earn programs feeding a redeem program | (b) | Renee, Marcus | Keep |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works |
|---|---------|---------|-------|--------------|
| 1 | Saved-route polling with delta exit code | `watch <route>` | 8/10 | Calls `/api/offers?searchId=...` on each run, diffs offer set against last snapshot in local SQLite, exits 2 if any offer is new or cheaper |
| 2 | Cross-snapshot drift report | `drift <route>` | 7/10 | Local SQLite join across `offer_snapshots`; produces new/cheaper/disappeared/unchanged columns |
| 3 | Effective-cost ranking via transfer ratios | `compare-transfer <earn-program> <route>` | 9/10 | Joins offers × transferOptions × programs locally; multiplies points cost by transferRatio; ranks by source-program-points-spent |
| 4 | Fan-out search and store | `batch <route-csv>` | 8/10 | One command issues N `/api/offers` reads with throttling; all snapshots in local store; pairs with watch |
| 5 | Reachability from points balance | `from-home <origin> --balance "ur:250000,..."` | 9/10 | Joins offers × airlines × transferOptions against user-supplied balance map; filters offers whose `points_cost / transferRatio <= balance[earn_program]`; ranks by lowest effective spend |
| 6 | Top-deals matrix with persistence | `top-deals-matrix --origins ... --dests ... --months ...` | 7/10 | Calls search-create POST per cell (cookie auth), GETs offers per result, stores everything |
| 7 | Deal-rating explore via Scout | `explore-deal-rating --metro <code>` | 6/10 | GET `scout.pointhound.com/places/search?metro=...`; filters `dealRating: high`; optional chain into `batch` |
| 8 | Best-month heatmap | `calendar --route <route> --cabin <cabin>` | 6/10 | Fan-out via batch across 12 months; groupby min(points_cost); month-grid table |
| 9 | Transfer-source lookup | `transferable-sources <redeem-program>` | 6/10 | Local read of transferOptions table; lists earn programs with ratio + transfer time |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `summarize-recent --since 7d` | LLM dependency; mechanical version is `sql` user pipes to own LLM | `drift <route>` |
| `predict-price <route>` | LLM/ML dependency + verifiability | `drift <route>` |
| `book <offer-id>` | Not in API surface (Pointhound emits instructions, not a booking endpoint) | None |
| `compare-vs-other <route>` | External service requires extra keys; out of scope | `compare-transfer` |
| `alerts list/create/delete` as novel | Thin endpoint mirror — not transcendence | `watch` (cookieless transcendent equivalent) |
| `program-coverage <route>` | Sibling-killed: same join graph as `compare-transfer` without ranking | `compare-transfer` |
| `cards` / `cards get` / `points101` static reference | Reference content, not transcendence | Kept as `// pp:novel-static-reference` outside transcendence table |
