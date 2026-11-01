Manifest transcendence rows: 7 planned, 7 built. Phase 3 will not pass until all 7 ship.

# Build Log — `yahoo-fantasy-pp-cli`

## Priority 0 (Foundation)
- Built `internal/yahoofantasy/yahoo.go` and `internal/yahoofantasy/yahoo_test.go`:
  - `<fantasy_content>` XML payload builders (`BuildRosterUpdateXML`, `BuildTransactionCreateXML`, `BuildTransactionUpdateXML`) and `TranslateJSONMutationToXML`.
  - XML-to-JSON response normalizer (`NormalizeYahooResponse`) for Yahoo write endpoints that return XML even when `?format=json` is requested.
  - Flexible JSON extractors (`ExtractPlayers`, `ExtractTeams`, `ExtractTransactions`, `ExtractDraftPicks`, `ExtractMatchups`) supporting both Yahoo's nested `fantasy_content` array-of-maps convention and flat JSON records in SQLite.
- Built `internal/cli/yahoo_transport.go` and `internal/cli/yahoo_transport_test.go`:
  - Registered `installYahooTransport` via `registerClientHook` so both `yahoo-fantasy-pp-cli` and `yahoo-fantasy-pp-mcp` automatically convert JSON mutation payloads into Yahoo `<fantasy_content>` XML on the wire, normalize XML responses into JSON, and support automatic OAuth 2.0 `refresh_token` exchange (`YAHOO_CLIENT_ID`, `YAHOO_CLIENT_SECRET`, `YAHOO_REFRESH_TOKEN`).
- Built `internal/cli/yahoo_store_helpers.go`:
  - Drain-first SQLite `resources` reader with `hintIfUnsynced` / `hintIfStale` integration and `--data-source` (`auto`, `local`, `live`) support.

## Priority 1 (Absorbed Features — 23 of 23)
- Verified all 23 absorbed endpoints across `games`, `users`, `leagues`, `teams`, `players`, and `transactions` (`teams update-roster`, `transactions create`, `transactions update`, `transactions cancel`, `leagues get-scoreboard`, `leagues get-players`, `teams get-roster`, etc.).

## Priority 2 (Transcendence Features — 7 of 7 Built)
1. `lineup` (`internal/cli/lineup.go`, `internal/cli/lineup_test.go`) — Audits active starting slots for `OUT`/`IR`/`D`/`BYE` players, recommends healthy eligible bench replacements, validates `--set` swaps, previews XML on `--dry-run`, and executes `PUT /team/{team_key}/roster`.
2. `add-drop` (`internal/cli/add_drop.go`, `internal/cli/add_drop_test.go`) — Validates player availability and team `faab_balance`, constructs `<fantasy_content><transaction>` XML for single add, single drop, or atomic `add/drop` with `--faab`, previews on `--dry-run`, and executes `POST /league/{league_key}/transactions`.
3. `waiver-wire` (`internal/cli/waiver_wire.go`, `internal/cli/waiver_wire_test.go`) — Ranks available `FA`/`W` players by net points-per-game upgrade over the weakest rostered bench player at each eligible position.
4. `matchup-radar` (`internal/cli/matchup_radar.go`, `internal/cli/matchup_radar_test.go`) — Compares weekly head-to-head opponents' starting lineups slot-by-slot with projected vs. actual points, win probability, and starter injury risk alerts.
5. `power-rankings` (`internal/cli/power_rankings.go`, `internal/cli/power_rankings_test.go`) — Computes weekly all-play `W-L-T` records, schedule luck delta, scoring standard deviation, and composite power score.
6. `faab-audit` (`internal/cli/faab_audit.go`, `internal/cli/faab_audit_test.go`) — Joins team `faab_balance` with historical transaction `faab_bid` logs to compute remaining budget share, positional spend breakdown, and minimum blocking bid over the next rival.
7. `draft-roi` (`internal/cli/draft_roi.go`, `internal/cli/draft_roi_test.go`) — Evaluates snake draft rounds and auction costs against season-to-date points and current roster retention (`steal`, `value`, `par`, `bust`).

## Intentionally Deferred / Skipped Body Fields / Generator Limitations
- Intentionally deferred: None (all 30 features in the Phase 1.5 absorb manifest are implemented).
- Skipped body fields: None.
- Generator limitations: Yahoo Fantasy v2 write endpoints require XML request bodies (`<fantasy_content>`) even when `?format=json` is used on reads; resolved cleanly via `registerClientHook` in `internal/cli/yahoo_transport.go` without modifying generated files.
