## Customer model

### 1. Marcus, the Sunday-Morning Lineup Setter & Tuesday-Night Waiver Grinder
- **Today (without this CLI):** Marcus manages three Yahoo Fantasy NFL teams from September through January. Every Tuesday night and Sunday morning at 12:15 PM ET, he juggles multiple Yahoo Fantasy browser tabs alongside injury reports and ad-hoc Python scripts built on `yfpy` or `michaelfromorg/mcp-yahoo-fantasy`. While existing wrappers can dump 50KB nested JSON trees of his roster, actually changing a starting lineup (`PUT /team/{team_key}/roster`) or submitting an atomic add/drop or FAAB waiver claim (`POST /league/{league_key}/transactions`) requires either clicking through Yahoo's web UI or hand-crafting deeply nested `<fantasy_content>` XML payloads. He cannot quickly answer across his leagues: "Which of my starting slots currently hold an `OUT`, `IR`, or `BYE` player while a healthy eligible replacement is sitting on `BN`, and which available free agents outscore my weakest bench spot?"
- **Weekly ritual:** Every Tuesday night, he scans free agents (`status=FA`) and waiver players (`status=W`) by position, compares their recent scoring and ownership percentages against his weakest bench players, and queues FAAB waiver bids. Every Sunday morning before kickoff, he audits every active roster slot for late injury/inactive designations and swaps healthy bench players into `QB`, `RB`, `WR`, `TE`, `W/R/T`, `K`, `DEF`, or `IR` slots while tracking live head-to-head matchup projections.
- **Frustration:** Translating a straightforward roster decision ("swap my injured starter to IR, move my bench RB into W/R/T, and claim this waiver WR for $14 FAAB while dropping my backup TE") into verified positional eligibility checks and Yahoo's mandatory `application/xml` `<fantasy_content>` mutation envelopes with a safe `--dry-run` preview.

### 2. Elena, the 12-Team League Commissioner & Active Trade Negotiator
- **Today (without this CLI):** Elena runs a competitive 12-team FAAB league and negotiates trades all season. She clicks through Yahoo's Standings, Scoreboard, and Transactions pages and manually updates a spreadsheet to track each manager's remaining FAAB balance, waiver priority, transaction velocity, and head-to-head schedule luck. When publishing weekly league recaps or preparing a waiver bid against a division rival, she cannot easily see how much FAAB each manager has spent by position or how a team's actual W-L record compares to their weekly "all-play" record against the entire league.
- **Weekly ritual:** Every Wednesday morning after waivers clear, she reviews all processed waiver claims, FAAB bids, drops, and pending trade proposals across all 12 managers, then computes updated power rankings and schedule-luck standings after Monday Night Football finishes.
- **Frustration:** Yahoo's `standings` endpoint only exposes raw W-L-T and aggregate points-for/points-against; calculating true weekly all-play records, schedule luck (actual wins minus expected all-play wins), and manager-by-manager FAAB burn efficiency requires pulling and joining every week's `scoreboard` and `transactions` feed manually.

### 3. Devon, the Draft Value & Multi-Week Quantitative Analyst
- **Today (without this CLI):** Devon uses `uberfastman/yfpy` and `spilchen/yahoo_fantasy_api` to pull league draft results (`draftresults`), player ownership (`percent_owned`), and weekly scoring stats (`players/stats`). Because Yahoo enforces burst rate limits (~300–500 req/hr) and existing Python MCP servers do not persist normalized entities in a local SQLite database, running cross-week regressions or comparing snake draft round/auction dollar cost against season-to-date fantasy points forces dozens of repetitive live HTTP calls that overflow LLM context windows.
- **Weekly ritual:** Each Wednesday and Thursday, he joins synced player scoring, ownership trends, draft capital, and roster tables in SQLite to identify under-owned breakout players and evaluate which draft rounds or auction price tiers delivered the highest points-over-replacement ROI.
- **Frustration:** No existing Yahoo Fantasy CLI or MCP server joins local SQLite `draft_results`, `rosters`, `players`, and `transactions` tables—forcing him to re-fetch deeply nested alternating list/dict JSON arrays from Yahoo's live API for every analytical query.

## Candidates (pre-cut)

1. **Lineup Auditor & Ergonomic Slot Setter (`lineup`)**
   - **Command:** `lineup --team <team_key> [--week <n>] [--date <yyyy-mm-dd>] [--set <slot=player_key,...>] [--auto-swap] [--dry-run]`
   - **Description:** Audits a team's starting roster for `OUT`/`IR`/`DTD`/`BYE` players in active slots, suggests valid healthy `BN` replacements using `eligible_positions`, previews the before/after slot diff, and constructs Yahoo's `<fantasy_content><roster>` XML payload to execute `PUT /team/{team_key}/roster`.
   - **Persona served:** Marcus (Sunday-Morning Lineup Setter & Tuesday-Night Waiver Grinder)
   - **Source:** `(e) User briefing` / `(f) DeepWiki`
   - **Long Description:** `Use this command to audit a team's active starting slots for OUT/IR/BYE players and move already-rostered players between starting, bench (BN), and IR positions. Do NOT use this command to sign free agents, submit waiver claims, or drop players to the league pool; use 'add-drop' instead.`
   - **Inline rubric verdict:** **KEEP**

2. **Ergonomic Add/Drop & FAAB Waiver Claim Builder (`add-drop`)**
   - **Command:** `add-drop --league <league_key> --team <team_key> [--add <player_key>] [--drop <player_key>] [--faab <int>] [--dry-run]`
   - **Description:** Resolves player identity and ownership status (`FA` vs. `W`), verifies the team's remaining `faab_balance`, constructs Yahoo's `<fantasy_content><transaction>` XML envelope for single add, single drop, or atomic `add/drop` (with optional `<faab_bid>`), and executes or dry-runs `POST /league/{league_key}/transactions`.
   - **Persona served:** Marcus (Sunday-Morning Lineup Setter & Tuesday-Night Waiver Grinder)
   - **Source:** `(e) User briefing` / `(f) DeepWiki`
   - **Long Description:** `Use this command to execute or dry-run an add, drop, atomic add/drop, or FAAB waiver claim from the league player pool. Do NOT use this command to rank available free agents against your roster (use 'waiver-wire' instead) or to swap starting positions among already-rostered players (use 'lineup' instead).`
   - **Inline rubric verdict:** **KEEP**

3. **Positional Waiver & Free-Agent Upgrade Recommender (`waiver-wire`)**
   - **Command:** `waiver-wire --league <league_key> --team <team_key> [--pos <position>] [--limit <int>]`
   - **Description:** Joins the user's current team roster against available league free agents (`FA`) and waiver players (`W`) with stats and `percent_owned`, ranking candidate additions by point-per-game differential over the weakest droppable bench player at each position.
   - **Persona served:** Marcus (Sunday-Morning Lineup Setter & Tuesday-Night Waiver Grinder)
   - **Source:** `(a) Persona-driven`
   - **Long Description:** `Use this command to rank available free agents and waiver players against your weakest rostered players by position, points, and ownership percentage. Do NOT use this command to submit the actual add/drop or FAAB transaction; use 'add-drop' instead.`
   - **Inline rubric verdict:** **KEEP**

4. **Head-to-Head Matchup Radar & Slot Advantage Breakdown (`matchup-radar`)**
   - **Command:** `matchup-radar --league <league_key> --team <team_key> [--week <n>]`
   - **Description:** Combines the weekly league scoreboard and both opponents' active rosters to compare projected vs. actual points slot-by-slot (`QB`, `RB`, `WR`, `TE`, `W/R/T`, `K`, `DEF`), remaining active starters, injury flags, and win-probability margin.
   - **Persona served:** Marcus (Sunday-Morning Lineup Setter & Tuesday-Night Waiver Grinder)
   - **Source:** `(b) Service-specific content patterns`
   - **Long Description:** `Use this command to compare a single weekly head-to-head matchup slot-by-slot between two teams. Do NOT use this command for season-long league power rankings, all-play records, or schedule luck across all teams; use 'power-rankings' instead.`
   - **Inline rubric verdict:** **KEEP**

5. **All-Play Standings, Power Rankings & Schedule Luck (`power-rankings`)**
   - **Command:** `power-rankings --league <league_key> [--through-week <n>]`
   - **Description:** Aggregates weekly scoreboards and standings from local SQLite (or live weekly scoreboards) to calculate every team's true all-play W-L-T record, schedule luck delta (actual wins minus expected all-play wins), weekly scoring standard deviation, and composite power rank.
   - **Persona served:** Elena (12-Team League Commissioner & Active Trade Negotiator)
   - **Source:** `(c) Cross-entity local queries`
   - **Long Description:** `Use this command to compute season-long all-play records, schedule luck, and scoring consistency across all teams in a league. Do NOT use this command to inspect a single week's head-to-head slot matchup; use 'matchup-radar' instead.`
   - **Inline rubric verdict:** **KEEP**

6. **League FAAB Burn & Waiver Bid Intelligence (`faab-audit`)**
   - **Command:** `faab-audit --league <league_key>`
   - **Description:** Joins league `teams` (`faab_balance`, `waiver_priority`, `number_of_moves`) with `transactions` (`add`, `add/drop`, `faab_bid`) to report each manager's remaining FAAB budget share, total FAAB spent by position, biggest winning bids, and blocking bid thresholds.
   - **Persona served:** Elena (12-Team League Commissioner & Active Trade Negotiator)
   - **Source:** `(b) Service-specific content patterns`
   - **Long Description:** `Use this command to analyze league-wide FAAB budget burn rates, winning waiver bids, and remaining bidding leverage across managers. Do NOT use this command to submit a new waiver claim (use 'add-drop' instead) or evaluate draft pick ROI (use 'draft-roi' instead).`
   - **Inline rubric verdict:** **KEEP**

7. **Draft Capital ROI & Steals/Busts Analyzer (`draft-roi`)**
   - **Command:** `draft-roi --league <league_key> [--team <team_key>]`
   - **Description:** Joins `draft_results` (snake `round`/`pick` or auction `cost`), `players` season fantasy points and `percent_owned`, and current `rosters` in local SQLite to compute points-per-draft-dollar, round-adjusted value over average, and each manager's best steals and biggest busts.
   - **Persona served:** Devon (Draft Value & Multi-Week Quantitative Analyst)
   - **Source:** `(c) Cross-entity local queries`
   - **Long Description:** `Use this command to evaluate snake draft picks or auction draft costs against season-to-date fantasy points and ownership. Do NOT use this command to audit in-season FAAB waiver spending; use 'faab-audit' instead.`
   - **Inline rubric verdict:** **KEEP**

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 20 | Lineup auditor & ergonomic XML slot setter | `lineup --team <team_key> [--week <n>] [--set <slot=player_key,...>] [--auto-swap] [--dry-run]` | 10/10 | hand-code | This uses `GET /team/{team_key}/roster`, league `settings` (or synced local SQLite `rosters`/`leagues`), and `PUT /team/{team_key}/roster` to detect `OUT`/`IR`/`BYE` starters, validate `eligible_positions`, preview slot diffs, and submit Yahoo's `<fantasy_content><roster>` XML payload with no external dependencies (Persona: Marcus). | User Vision explicit request for ergonomic lineup changes; `michaelfromorg/mcp-yahoo-fantasy` and `spilchen/yahoo_fantasy_api` require nested XML construction for `PUT /team/{team_key}/roster`. | Use this command to audit a team's active starting slots for OUT/IR/BYE players and move already-rostered players between starting, bench (BN), and IR positions. Do NOT use this command to sign free agents, submit waiver claims, or drop players to the league pool; use 'add-drop' instead. |
| 21 | Atomic add/drop & FAAB waiver claim builder | `add-drop --league <league_key> --team <team_key> [--add <player_key>] [--drop <player_key>] [--faab <int>] [--dry-run]` | 10/10 | hand-code | This uses `GET /league/{league_key}/players`, `GET /team/{team_key}` (or local SQLite `players`/`teams`), and `POST /league/{league_key}/transactions` to verify player `FA`/`W` status and team `faab_balance`, construct Yahoo's `<fantasy_content><transaction>` XML payload for single add, single drop, or atomic `add/drop` with `<faab_bid>`, and execute or preview the mutation with no external dependencies (Persona: Marcus). | User Vision explicit request to add/drop players and submit waiver claims; `derekrbreese/fantasy-football-mcp-public` and `spilchen/yahoo_fantasy_mcp` lack full XML transaction write support. | Use this command to execute or dry-run an add, drop, atomic add/drop, or FAAB waiver claim from the league player pool. Do NOT use this command to rank available free agents against your roster (use 'waiver-wire' instead) or to swap starting positions among already-rostered players (use 'lineup' instead). |
| 22 | Positional waiver & free-agent upgrade recommender | `waiver-wire --league <league_key> --team <team_key> [--pos <position>] [--limit <int>]` | 10/10 | hand-code | This uses synced local SQLite `rosters`, `players`, and `leagues` tables (or live `GET /team/{team_key}/roster` and `GET /league/{league_key}/players;status=A/stats;type=season`) to compute point-per-game upgrades between available free agents/waiver players and the team's lowest-scoring bench players at each eligible position with no external dependencies (Persona: Marcus). | Phase 1 brief Top Workflow #2; existing MCP servers dump raw 50KB player lists without comparing against the caller's droppable bench slots. | Use this command to rank available free agents and waiver players against your weakest rostered players by position, points, and ownership percentage. Do NOT use this command to submit the actual add/drop or FAAB transaction; use 'add-drop' instead. |
| 23 | Head-to-head matchup radar & slot advantage breakdown | `matchup-radar --league <league_key> --team <team_key> [--week <n>]` | 9/10 | hand-code | This uses `GET /league/{league_key}/scoreboard;week={week}` and `GET /team/{team_key}/roster;week={week}` (or synced local SQLite `matchups` and `rosters` tables) to align both opponents' starting lineups slot-by-slot and compute projected vs. actual point differentials, remaining active starters, and injury exposure with no external dependencies (Persona: Marcus). | Phase 1 brief Top Workflow #3; `michaelfromorg/mcp-yahoo-fantasy` separates scoreboard totals from starter-by-starter positional comparisons. | Use this command to compare a single weekly head-to-head matchup slot-by-slot between two teams. Do NOT use this command for season-long league power rankings, all-play records, or schedule luck across all teams; use 'power-rankings' instead. |
| 24 | All-play standings, power rankings & schedule luck | `power-rankings --league <league_key> [--through-week <n>]` | 9/10 | hand-code | This uses synced local SQLite `standings` and weekly `matchups`/`scoreboards` (with live fallback to `GET /league/{league_key}/standings` and `GET /league/{league_key}/scoreboard;week=1..N`) to compute each team's true all-play W-L-T record, schedule luck delta, scoring standard deviation, and composite power ranking with no external dependencies (Persona: Elena). | Phase 1 brief Top Workflow #4 and User Vision local analytics requirement; Yahoo's native `standings` endpoint omits all-play records and schedule luck. | Use this command to compute season-long all-play records, schedule luck, and scoring consistency across all teams in a league. Do NOT use this command to inspect a single week's head-to-head slot matchup; use 'matchup-radar' instead. |
| 25 | League FAAB burn & waiver bid intelligence | `faab-audit --league <league_key>` | 9/10 | hand-code | This uses synced local SQLite `teams` and `transactions` tables (or live `GET /league/{league_key}/teams` and `GET /league/{league_key}/transactions`) to aggregate each manager's remaining `faab_balance`, total FAAB spent by position, winning vs. losing waiver bids, and minimum blocking bid thresholds with no external dependencies (Persona: Elena). | Phase 1 brief Top Workflow #5; `uberfastman/yfpy` and `michaelfromorg/mcp-yahoo-fantasy` expose raw transaction lists without joining team FAAB balances or positional spend. | Use this command to analyze league-wide FAAB budget burn rates, winning waiver bids, and remaining bidding leverage across managers. Do NOT use this command to submit a new waiver claim (use 'add-drop' instead) or evaluate draft pick ROI (use 'draft-roi' instead). |
| 26 | Draft capital ROI & steals/busts analyzer | `draft-roi --league <league_key> [--team <team_key>]` | 8/10 | hand-code | This uses synced local SQLite `draft_results`, `players`, and `rosters` tables (or live `GET /league/{league_key}/draftresults` and `GET /league/{league_key}/players/stats`) to calculate fantasy points per auction dollar or snake-round average surplus value and rank each manager's best draft steals and biggest busts with no external dependencies (Persona: Devon). | Phase 1 brief Persona #3 and `uberfastman/yfpy` draft analysis patterns; avoids Yahoo's ~300–500 req/hr rate limit by joining synced tables locally. | Use this command to evaluate snake draft picks or auction draft costs against season-to-date fantasy points and ownership. Do NOT use this command to audit in-season FAAB waiver spending; use 'faab-audit' instead. |

### Killed candidates

| Feature | Kill Reason | Closest Surviving Sibling |
|---------|-------------|---------------------------|
| AI Trash-Talk & Trade Pitch Copywriter (`trash-talk`) | Killed by rubric **LLM dependency** check: requires generative NLP prose and sentiment writing rather than mechanical data extraction; users can pipe `power-rankings --json` into an LLM. | `power-rankings` |
| Cross-Platform Sleeper & ESPN League Arbitrage (`cross-platform-sync`) | Killed by rubric **External service** check: requires calling undocumented third-party Sleeper and ESPN APIs outside `fantasysports.yahooapis.com`. | `draft-roi` |
| Interactive Sunday Gameday Terminal Dashboard (`gameday-tui`) | Killed by rubric **Scope creep** check: requires a stateful full-screen TUI and background polling loops (>200 LoC) rather than a composable single-shot command. | `matchup-radar` |
| Vegas Props & Stadium Weather Lineup Overlay (`vegas-weather`) | Killed by rubric **External service** check: requires external sportsbook odds and stadium weather APIs not present in the Yahoo Fantasy v2 specification. | `waiver-wire` |
| Read-Only Inactive Starter Checker (`check-roster`) | Killed in Pass 3 (**Sibling kill / Wrapper vs leverage**): strictly a read-only subset of `lineup`, which already audits `OUT`/`IR`/`BYE` starters AND computes valid bench swaps with `--dry-run` or live XML execution. | `lineup` |
| Transaction Type Filter (`tx-history`) | Killed in Pass 3 (**Wrapper vs leverage**): thin renaming of `GET /league/{league_key}/transactions` (`leagues get-transactions` in the absorb manifest) without cross-entity synthesis. | `faab-audit` |
| Multi-Player Trade Fairness Calculator (`trade-eval`) | Killed in Pass 3 (**Weekly use**): most fantasy managers execute 0–2 trades per season ("occasionally"), failing the weekly ritual bar compared to weekly waiver and lineup operations. | `waiver-wire` |
