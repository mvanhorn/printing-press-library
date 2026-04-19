---
title: "feat: Add missing CLI commands to ESPN and audit Movie Goat surface"
type: feat
status: active
date: 2026-04-19
---

# feat: Add missing CLI commands to ESPN and audit Movie Goat surface

## Overview

Two Printing Press CLIs ship SKILL.md docs that promise commands the binary does not implement. The agent reads SKILL.md, calls the documented command, and gets `unknown command`. Today's session hit four such failures inside a single MLB query (`boxscore`, `leaders`, `summary <id>`, `team get`).

This plan closes the gap by implementing the genuinely useful missing commands in the two CLIs and updating SKILL.md to match the real surface. ESPN gets the bulk of the work because most documented-but-missing commands map to real public ESPN endpoints. Movie Goat needs an audit and small additions (TV `airing-today`, `on-the-air`, `top-rated` are real but undocumented; `discover tv` is real and documented but the SKILL.md example is wrong).

## Problem Frame

The pp-espn SKILL.md cache (`/Users/mvanhorn/.claude/plugins/cache/printing-press-library/printing-press-library/1.1.10/skills/pp-espn/SKILL.md`) lists the following commands in its "Command Reference":

- `boxscore <game_id>`
- `plays <game_id>`
- `preview <game_id>`
- `leaders <sport> <league>`
- `injuries <sport> <league>`
- `odds <sport> <league>`
- `transactions <sport> <league>`
- `sos <sport> <league>`
- `h2h <team1> <team2>`
- `compare <athlete1> <athlete2>`
- `trending`
- `dashboard`
- `team get|list <sport> <league> <team_id>` (real cmd is `teams get|list`)
- `athlete <sport> <league>`

None of these resolve in `espn-pp-cli --help` (verified 2026-04-19, version current as of commit d23db1f). The real CLI exposes `summary`, `recap`, `scores`, `scoreboard`, `standings`, `schedule`, `streak`, `rivals`, `today`, `watch`, `news`, `search`, `sync`, `rankings`, `teams`, plus power-user commands `sql`, `workflow`, `load`, `orphans`, `stale`. SKILL.md never mentions `today`, `scoreboard`, `streak`, `rivals`, `workflow`, `sql`.

The Movie Goat SKILL.md is mostly accurate but:

- Promotes `discover tv` as a recipe, real command exists as `discover tv` subcommand (good), but no examples elsewhere reflect that subcommand structure.
- Does not document real subcommands `tv airing-today`, `tv on-the-air`, `tv top-rated`.
- Lists `tv seasons get <seriesId> <seasonNumber>` which exists as `tv get` subcommand named `get` (Cobra collision: `tv get <id>` and `tv seasons get <id> <season>` both share `get`). Behavior needs verification.
- Lists `genres tv` - need to verify subcommand exists.

The cost of leaving this is steady agent-side friction: every retry burns context, every wrong command erodes user trust, and the SKILL.md keeps shipping promises the CLI cannot keep.

## Requirements Trace

- R1. Every command listed in pp-espn SKILL.md `## Command Reference` resolves successfully against `espn-pp-cli --help` after this PR lands, OR is removed from SKILL.md with no replacement claim.
- R2. Each new ESPN command returns valid live data against a public ESPN endpoint with `--agent` mode for an in-season league at the time of merge.
- R3. The pp-movie-goat SKILL.md `## Command Reference` matches `movie-goat-pp-cli --help` exactly (no documented-missing or real-undocumented commands).
- R4. Existing ESPN commands (`scores`, `summary`, `teams`, `standings`, `news`, `recap`, `rankings`, `today`, `scoreboard`, `streak`, `rivals`, `watch`, `search`, `sync`) continue to work without regression.
- R5. SKILL.md updates land in both the canonical per-CLI SKILL.md (`library/media-and-entertainment/<cli>/SKILL.md`) and the plugin-cached copy (`plugin/skills/pp-<cli>/SKILL.md`) so plugin users and openclaw users see consistent docs.
- R6. New ESPN commands have at least one example in SKILL.md and one in the command's `--help` examples block.
- R7. Each new command writes domain-specific store records (or explicitly opts out) so `sync` and `search` can pick it up where applicable.

## Scope Boundaries

In scope:

- Add ESPN commands: `boxscore`, `plays`, `leaders`, `injuries`, `odds`, `transactions`, `sos`, `h2h`, `compare`, `trending`, `dashboard`. Some land via spec.yaml extensions (auto-generated), others as hand-written promoted commands.
- Rename or alias `team get|list` to `teams get|list` per real CLI naming, with the documented `team` form kept as a deprecated alias for one minor version.
- Drop `athlete <sport> <league>` from docs (no real ESPN public endpoint matches; subsumed by `compare` and team rosters).
- Add `today`, `scoreboard`, `streak`, `rivals`, `sql`, `workflow` to ESPN SKILL.md command reference and recipes.
- Movie Goat: add `tv airing-today`, `tv on-the-air`, `tv top-rated` to SKILL.md. Verify `tv seasons get` and `genres tv` resolve and either fix the docs or fix the binary.

Out of scope:

- Authentication changes - ESPN stays no-auth, Movie Goat keeps TMDb + optional OMDb.
- Schema changes to existing tables in `internal/store`.
- New MCP tools (the MCP server picks up new resources automatically when spec.yaml grows; no hand-written MCP code in this PR).
- Cross-CLI shared library work (e.g., shared `agent-mode` flags) - covered elsewhere by PR #218.
- Dashboard config UI; `dashboard` command reads `~/.config/espn-pp-cli/config.toml` `[favorites]` table only.
- Live websocket streaming. ESPN endpoints stay polling-only.

## Context and Research

### Relevant Code and Patterns

ESPN package layout:

- `library/media-and-entertainment/espn/spec.yaml` - declarative API spec, 5 resources today (`scoreboard`, `teams`, `news`, `summary`, `rankings`). Generator produces resource-bound commands as `internal/cli/promoted_<resource>.go`.
- `library/media-and-entertainment/espn/internal/cli/` - hand-written command files for non-spec commands. Pattern: see `today.go`, `streak.go`, `rivals.go`, `recap.go`, `watch.go`, `scores.go`. These read from `internal/client/` for HTTP calls and `internal/store/` for sync/search.
- `library/media-and-entertainment/espn/internal/client/` - HTTP client wrapping ESPN's site, sports.core, and site.web API surfaces.
- `library/media-and-entertainment/espn/internal/store/` - SQLite domain tables.
- `library/media-and-entertainment/espn/internal/types/` - response types.
- `library/media-and-entertainment/espn/cmd/espn-pp-cli/main.go` - main entrypoint, calls `cli.Execute()`.

Movie Goat package layout:

- `library/media-and-entertainment/movie-goat/spec.yaml` - 7 resources: `movies`, `tv`, `people`, `discover`, `trending`, `genres`, `search`.
- `library/media-and-entertainment/movie-goat/internal/cli/` - same generator + hand-written split as ESPN.
- `library/media-and-entertainment/movie-goat/internal/omdb/` - OMDb enrichment client.

Add-a-command pattern (verified by reading `today.go`, `recap.go`, `rivals.go`):

- New command file `internal/cli/<verb>.go`
- `init()` registers a `*cobra.Command` with rootCmd
- Handler reads positional args + flags, calls `client.<Method>()`, prints via `helpers.PrintJSON()` or `helpers.PrintTable()`
- For commands that should also feed the local store, call `store.Upsert<Domain>()` after fetch

ESPN endpoint discovery:

- Many ESPN endpoints are undocumented but stable: `site.api.espn.com/apis/site/v2/sports/{sport}/{league}/{resource}`, `sports.core.api.espn.com/v2/sports/{sport}/leagues/{league}/...`, `site.web.api.espn.com/apis/common/v3/sports/{sport}/{league}/...`. The existing client already hits all three.
- Confirmed live endpoints used by real ESPN web app:
  - Boxscore: covered by existing `summary` command's response payload (key: `boxscore`)
  - Plays: `sports.core.api.espn.com/v2/sports/{sport}/leagues/{league}/events/{eventId}/competitions/{eventId}/plays?limit=1000`
  - Injuries: `site.web.api.espn.com/apis/site/v2/sports/{sport}/{league}/injuries`
  - Odds: contained in `summary` payload under `pickcenter`/`againstTheSpread`
  - Transactions: `sports.core.api.espn.com/v2/sports/{sport}/leagues/{league}/transactions`
  - Leaders: `site.web.api.espn.com/apis/common/v3/sports/{sport}/{league}/statistics/byathlete?limit=N&category=<cat>`
  - Athletes by team: `site.web.api.espn.com/apis/site/v2/sports/{sport}/{league}/teams/{teamId}/roster`

Verification strategy: dogfood-results.json already exists per CLI. Adding a command means appending to its example_check coverage.

### Institutional Learnings

- 2026-04-10-001 fix-cli-quality-bugs plan: previous round of CLI hygiene fixes. Pattern was small atomic commits per binary, dogfood verification before merge. Worth following here.
- Memory `feedback_pp_go_install_goprivate.md`: PP CLIs need `GOPRIVATE='github.com/mvanhorn/*'` for fresh installs. Local verification commands in this plan must include that prefix.
- Memory `feedback_evidence_every_pr.md`: every PR in a batch needs evidence. This plan splits into separate PRs per CLI for that reason.

### External References

- ESPN unofficial API reference: https://github.com/pseudo-r/Public-ESPN-API (community-maintained list of working endpoints, current as of 2025-12).
- TMDb API docs: https://developer.themoviedb.org/reference/intro/getting-started.
- Cobra subcommand collision rules: https://pkg.go.dev/github.com/spf13/cobra (relevant for the `tv get` vs `tv seasons get` overlap question in Movie Goat).

## Key Technical Decisions

- Spec-driven vs hand-written for new ESPN commands. Decision: spec.yaml for `injuries`, `transactions`, `leaders`, `plays` (single endpoint, predictable response shape, MCP server gets them for free). Hand-written for `boxscore`, `odds`, `sos`, `h2h`, `compare`, `trending`, `dashboard` (multi-endpoint composition or local-state computation, no clean spec fit). Rationale: spec.yaml gives free MCP exposure but pays cost in flexibility; commands that compose multiple endpoints or read local config want raw Go.
- `boxscore` semantics. Decision: alias `boxscore <event_id>` to `summary <sport> <league> --event <event_id>` filtered to the boxscore subtree. Sport+league inferred from the cached scores response when possible, required as flag otherwise. Rationale: user mental model is "boxscore for a game id"; making them remember sport+league + use a flag is the friction we are removing.
- `team` vs `teams` naming. Decision: keep `teams` as canonical, add `team` as a hidden alias that prints a deprecation note on stderr. Drop `team` references from SKILL.md immediately. Rationale: SKILL.md `team` is a docs bug; renaming the real command would break users on old SKILL.md.
- Dashboard config schema. Decision: `[favorites]` TOML table with `nfl = ["KC", "BAL"]`, `nba = ["OKC", "BOS"]` keyed by league abbreviation. Reuses existing config file `~/.config/espn-pp-cli/config.toml`. Rationale: smallest config surface that supports per-league favorites; matches how scores/standings already key on league.
- Movie Goat `tv get` collision. Decision: keep `tv get <seriesId>` as the series-detail call, rename the season detail to `tv seasons get <seriesId> <seasonNumber>` (verified to be the existing signature, just under-promoted). Update SKILL.md to use the explicit `tv seasons get` form so no example collides.
- SKILL.md sync. Decision: write a small repo-level `tools/skill-doctor.go` that diffs each `library/<area>/<cli>/SKILL.md` against `<binary> --help` output and fails CI if drift exceeds zero documented-missing commands. Adds it as the last unit. Rationale: the only durable way to prevent this class of bug from coming back. If the unit slips, the rest of the plan still ships value.

## Open Questions

### Resolved During Planning

- Q: Are the ESPN endpoints for plays/injuries/transactions stable enough to ship? A: Yes, used by ESPN's own scoreboard frontend; community-maintained reference confirms 2025+ stability.
- Q: Should `boxscore` print everything or be summarized? A: Default to a leaders + line score view in human mode, full structured payload in `--agent` mode. Matches the user-vs-agent mode pattern already used by `summary`.
- Q: Per-CLI PR or one combined PR? A: Per-CLI PRs. Smaller diff, easier review, matches `feedback_evidence_every_pr.md` rule.

### Deferred to Implementation

- Exact response field selection for `--compact` output of each new command (pick during implementation by inspecting one live response).
- Whether `leaders` should support `--category passing,rushing,receiving` filters at launch or in a follow-up. Probable: add minimal `--category <name>` and let the user discover via help.
- Whether `dashboard` should poll-refresh or be one-shot. Probable: one-shot, with `watch` available as the existing live-poll path.
- Sequencing inside the ESPN PR: probably ship spec-driven commands together, then hand-written ones as separate commits. Decide at implementation time based on diff size.

## Implementation Units

- [ ] Unit 1: ESPN spec.yaml extensions for injuries, transactions, leaders, plays

Goal: Add four new resources to `library/media-and-entertainment/espn/spec.yaml` so the generator produces real Cobra commands and MCP tools.

Requirements: R1, R2, R6, R7

Dependencies: None

Files:
- Modify: `library/media-and-entertainment/espn/spec.yaml`
- Regenerate: `library/media-and-entertainment/espn/internal/cli/promoted_injuries.go`, `promoted_transactions.go`, `promoted_leaders.go`, `promoted_plays.go` (generator output)
- Regenerate: `library/media-and-entertainment/espn/internal/types/` entries for new response shapes
- Modify: `library/media-and-entertainment/espn/internal/store/` if any new domain table is needed (likely yes for `leaders` and `plays`)

Approach:
- Mirror existing resource shape (`news`, `summary`) for declarative shape.
- Endpoints to register:
  - `injuries`: `GET /{sport}/{league}/injuries`, params sport+league positional.
  - `transactions`: `GET /sports/{sport}/leagues/{league}/transactions` against the sports.core base; will need a `base_url_override` field if the spec generator does not already support per-resource base swaps. If not, fall back to hand-written for transactions only.
  - `leaders`: `GET /sports/{sport}/leagues/{league}/leaders` (sports.core) or the site.web equivalent, whichever has cleaner per-stat output. Pick during implementation.
  - `plays`: `GET /sports/{sport}/leagues/{league}/events/{eventId}/competitions/{eventId}/plays`, eventId positional, optional `--limit` (default 200).
- Run the existing generator (`cli-printing-press` Go binary) against the updated spec.

Patterns to follow:
- `spec.yaml` `news:` and `summary:` blocks for resource shape.
- `internal/store/news_store.go` for the upsert pattern when the new resource needs a domain table.

Test scenarios:
- Happy path: `espn-pp-cli injuries baseball mlb --agent` returns at least one injury record for an in-season league with valid `athlete`, `team`, `status` fields.
- Happy path: `espn-pp-cli leaders baseball mlb --agent` returns a list of athletes with stats for the current season.
- Happy path: `espn-pp-cli plays basketball nba --event <live_event_id> --agent` returns plays for an in-progress game.
- Edge case: `espn-pp-cli injuries football nfl --agent` during the offseason returns an empty list, exit 0, not exit 3.
- Error path: `espn-pp-cli plays basketball nba --event 0 --agent` returns exit 3 with structured not-found error.
- Integration: `espn-pp-cli sync --sport baseball --league mlb` populates the new `leaders` and `plays` tables in the local SQLite, verified by `sqlite3 ~/.local/share/espn-pp-cli/db.sqlite ".tables"`.

Verification:
- New commands appear in `espn-pp-cli --help`.
- Each command's `--help` includes at least one example.
- `espn-pp-cli doctor` reports OK after running each command.
- `dogfood-results.json` updates show new commands covered.

- [ ] Unit 2: ESPN hand-written commands - boxscore, odds, sos, h2h, compare, trending, dashboard

Goal: Add seven hand-written commands that compose multiple endpoints or read local state.

Requirements: R1, R2, R6

Dependencies: Unit 1 (only because dogfood verification runs after both)

Files:
- Create: `library/media-and-entertainment/espn/internal/cli/boxscore.go`
- Create: `library/media-and-entertainment/espn/internal/cli/odds.go`
- Create: `library/media-and-entertainment/espn/internal/cli/sos.go`
- Create: `library/media-and-entertainment/espn/internal/cli/h2h.go`
- Create: `library/media-and-entertainment/espn/internal/cli/compare.go`
- Create: `library/media-and-entertainment/espn/internal/cli/trending.go`
- Create: `library/media-and-entertainment/espn/internal/cli/dashboard.go`
- Modify: `library/media-and-entertainment/espn/internal/cli/teams_get.go` and `teams_list.go` to register hidden `team` alias of `teams`
- Modify: `library/media-and-entertainment/espn/internal/config/` to add `[favorites]` table loader
- Test: parallel `_test.go` files for each command using table-driven tests against recorded fixtures

Approach:
- `boxscore <event_id>`: detect sport+league from a `scores` cache lookup if present, otherwise require `--sport` and `--league`. Fetch via existing `summary` client method, return only the `boxscore` subtree. In agent mode, return raw JSON; in human mode, render a leaders + line score table.
- `odds <sport> <league>`: list current games with their `pickcenter` lines from each game's summary. Avoid one summary call per game by querying the scoreboard's `competitions[].odds` field which already carries the lines.
- `sos <sport> <league>`: fetch standings, extract `strengthOfSchedule` and `remainingStrengthOfSchedule` fields per team. Sort by `strengthOfSchedule` descending in default human view.
- `h2h <team1> <team2>`: resolve abbreviations via existing teams list, fetch each team's schedule, intersect by opponent and aggregate W-L plus average scores. Reuse `internal/cli/rivals.go` pattern (already does series records).
- `compare <athlete1> <athlete2>`: search athletes via `site.web.api.espn.com/apis/common/v3/sports/{sport}/{league}/athletes?search=<name>`, fetch per-athlete season stats, render a side-by-side table. Require `--sport` and `--league`. In ambiguous matches, list candidates and exit 2.
- `trending`: hit `https://site.api.espn.com/apis/site/v2/trending` (verified live endpoint as of session date), return ranked list of athletes and teams across leagues.
- `dashboard`: read `[favorites]` from config, for each configured league fetch scores and standings for the favorited teams, render a single table grouped by league. No live polling; just one fetch.
- `team` alias: `cobra.Command{Hidden: true, Aliases: []string{"team"}}` on the `teams` parent command, with a deprecated stderr note shown when invoked.

Patterns to follow:
- `today.go` for cross-league fetches.
- `rivals.go` for head-to-head computations.
- `streak.go` for derived-stat patterns reading from synced data.
- `recap.go` for sport+league argument parsing.

Test scenarios:
- Happy path (boxscore): `espn-pp-cli boxscore 401869192 --agent` returns a boxscore for a known live or recent NBA game without requiring `--sport`/`--league` if the game is in scores cache.
- Happy path (boxscore explicit): with cache cleared, `boxscore 401869192 --sport basketball --league nba --agent` works.
- Edge case (boxscore): pre-game state returns boxscore subtree with empty player stats and exit 0.
- Happy path (odds): `espn-pp-cli odds basketball nba --agent` returns spreads, totals, and moneylines for tonight's slate.
- Happy path (sos): `espn-pp-cli sos football nfl --agent` returns 32 entries sorted by SOS descending.
- Happy path (h2h): `espn-pp-cli h2h chiefs eagles --agent` returns historical W-L and average scores.
- Edge case (h2h): unknown team abbreviation returns exit 3 with structured error naming the invalid arg.
- Happy path (compare): `espn-pp-cli compare Mahomes Allen --sport football --league nfl --agent` returns side-by-side stats.
- Error path (compare): ambiguous name returns exit 2 with a list of candidate athlete IDs.
- Happy path (trending): `espn-pp-cli trending --agent` returns at least 5 trending entries with `entity_type`, `name`, `league` fields.
- Happy path (dashboard): with a populated `[favorites]` block, `espn-pp-cli dashboard --agent` returns scores+standings for each favorited team.
- Edge case (dashboard): empty `[favorites]` returns exit 0 with a hint about how to add favorites.
- Integration (team alias): `espn-pp-cli team list football nfl` works and prints a stderr deprecation note pointing at `teams list`.

Verification:
- All seven commands appear in `espn-pp-cli --help` (except `team` which stays hidden).
- Each new command has at least one `--help` example.
- `espn-pp-cli doctor` continues to report OK.

- [ ] Unit 3: ESPN SKILL.md update

Goal: Make `library/media-and-entertainment/espn/SKILL.md` and `plugin/skills/pp-espn/SKILL.md` accurately describe the post-Unit-1+2 surface.

Requirements: R1, R3, R5, R6

Dependencies: Unit 1, Unit 2 (cannot document until commands exist)

Files:
- Modify: `library/media-and-entertainment/espn/SKILL.md`
- Modify: `plugin/skills/pp-espn/SKILL.md`

Approach:
- Replace `## Command Reference` block with a list generated from `espn-pp-cli --help` output (manual once, automated by Unit 5 going forward).
- Add `## Recipes` entries for `boxscore`, `dashboard`, `odds`, `compare`, `h2h`, `trending`, `today`, `streak`, `rivals`. Each recipe shows one realistic invocation plus one chained example.
- Drop `athlete <sport> <league>` entry entirely. Add `--athletes` flag note under `teams get` if we want to expose roster fetching.
- Replace all `team get` / `team list` references with `teams get` / `teams list`.
- Add `--event` flag note under `summary` example (currently SKILL.md shows `summary <game_id>` which fails).

Patterns to follow:
- Existing accurate sections of SKILL.md (auth setup, agent mode, exit codes) are fine and stay.
- Examples should follow the `--agent`-by-default pattern already used elsewhere in the doc.

Test scenarios:
- Happy path: every command in the updated `## Command Reference` resolves successfully when piped through `espn-pp-cli <cmd> --help`.
- Happy path: every recipe example actually runs and returns exit 0 against a live ESPN API for an in-season league.
- Edge case: SKILL.md no longer references `boxscore`, `leaders`, `plays`, etc. as missing - they all exist now.

Verification:
- Manual inspection: SKILL.md command reference matches `espn-pp-cli --help` line-for-line on top-level commands.
- Both copies (`library/.../SKILL.md` and `plugin/skills/.../SKILL.md`) have identical content for the changed sections.

- [ ] Unit 4: Movie Goat audit and SKILL.md fix

Goal: Resolve the `tv get` vs `tv seasons get` collision question, document `tv airing-today`, `tv on-the-air`, `tv top-rated`, `genres tv` if real, and align SKILL.md.

Requirements: R3, R4, R5

Dependencies: None (independent of ESPN units)

Files:
- Verify (read-only first pass): `library/media-and-entertainment/movie-goat/internal/cli/` - confirm `tv get`, `tv seasons get`, `genres tv` real signatures.
- Modify (only if collision): `library/media-and-entertainment/movie-goat/internal/cli/tv_<x>.go` to disambiguate Cobra registration if `tv get` ambiguity exists.
- Modify: `library/media-and-entertainment/movie-goat/SKILL.md`
- Modify: `plugin/skills/pp-movie-goat/SKILL.md`

Approach:
- Run `movie-goat-pp-cli tv get --help`, `movie-goat-pp-cli tv seasons get --help`, `movie-goat-pp-cli genres tv --help`. Confirm intended behavior matches docs. The `tv` subcommand `Available Commands` listed two `get` entries during the audit - this needs verification and likely a Cobra registration fix.
- Add the three undocumented TV subcommands to `## Command Reference`.
- If `tv seasons get` does not exist as documented, either add it (small implementation, TMDb endpoint `/tv/{series_id}/season/{season_number}`) or rewrite the doc to match the real call site.
- Verify `genres tv` resolves; if not, add it (TMDb has `/genre/tv/list`).

Patterns to follow:
- `internal/cli/tv_get.go` and any sibling tv_*.go for command registration shape.
- `genres.go` for genre subcommand registration shape.

Test scenarios:
- Happy path: every `tv` subcommand documented in SKILL.md resolves under `--help`.
- Happy path: `movie-goat-pp-cli tv seasons get 1399 1 --agent` returns season 1 of Game of Thrones with episodes.
- Happy path: `movie-goat-pp-cli genres tv --agent` returns TMDb TV genre list.
- Edge case: `movie-goat-pp-cli tv top-rated --agent --limit 5` returns 5 series.
- Integration: documented and real surfaces match - no `--help` command in SKILL.md returns "unknown command".

Verification:
- `movie-goat-pp-cli --help` and SKILL.md `## Command Reference` agree on top-level commands.
- All subcommand references in SKILL.md examples actually resolve.

- [ ] Unit 5: skill-doctor drift check (recommended, can ship in a follow-up PR if scope creeps)

Goal: A small Go tool that walks each `library/<area>/<cli>/SKILL.md`, extracts every `<binary> <command>` snippet from `## Command Reference` and `## Recipes`, runs `<binary> --help` for each, and fails if any documented command is not present.

Requirements: R1, R3 (durably, going forward)

Dependencies: Units 1-4 (so first run of skill-doctor passes against the freshly-aligned docs)

Files:
- Create: `tools/skill-doctor/main.go`
- Create: `tools/skill-doctor/main_test.go`
- Modify: `.github/workflows/ci.yaml` to run `go run ./tools/skill-doctor` against every CLI in `library/`
- Modify: `Makefile` (root) to add `make skill-doctor` target

Approach:
- Walk `library/*/*/SKILL.md`, parse for fenced bash blocks and inline `binary command` patterns.
- Resolve binary names from each SKILL.md's `metadata.openclaw.requires.bins` frontmatter.
- For each unique `<binary> <command>` pair, exec `<binary> <command> --help` and capture exit code.
- Exit 1 if any documented command exits with the Cobra `unknown command` error string or returns no `--help` output.
- Print a per-CLI table of `documented | resolves | drift_kind`.

Patterns to follow:
- Existing tool shape under `tools/` (small standalone Go binary).
- Existing CI step style in `.github/workflows/ci.yaml`.

Test scenarios:
- Happy path: against the post-Unit-3+4 SKILL.md, skill-doctor exits 0 for both ESPN and Movie Goat.
- Failure path: introduce a fake `espn-pp-cli madeup-command` reference in a temp SKILL.md and confirm skill-doctor exits 1 with a clear message naming the offending command and SKILL.md file.
- Edge case: SKILL.md with no `## Command Reference` section is reported as a warning, not a failure (some skills may legitimately have no command list).

Verification:
- CI run on the PR's own commit passes skill-doctor.
- A local run reproduces the CI result.

## System-Wide Impact

- Interaction graph: ESPN's MCP server auto-registers spec.yaml resources, so Unit 1 expands the MCP tool surface for free. Hand-written commands in Unit 2 stay CLI-only unless someone explicitly adds MCP wrappers.
- Error propagation: new commands must use the same exit-code conventions documented in SKILL.md (0 success, 2 usage, 3 not found, 5 API, 7 rate limited). Unit 2 specs include explicit exit-code expectations per scenario.
- State lifecycle risks: new domain tables in Unit 1 mean `sync` schema migrations. The store package likely uses `CREATE TABLE IF NOT EXISTS` already, but verify; if not, a one-time migration runner is needed before merge.
- API surface parity: SKILL.md, README.md, and `--help` should agree. Unit 5's skill-doctor enforces SKILL.md vs `--help`. README.md should be spot-checked but is lower-leverage.
- Integration coverage: dogfood-results.json gets regenerated; CI should include the new commands' example execution.
- Unchanged invariants: existing exit codes, config path (`~/.config/espn-pp-cli/config.toml`), agent-mode flag set, cache layout, SQLite path.

## Risks and Dependencies

| Risk | Mitigation |
|------|------------|
| ESPN endpoint stability for plays/leaders/transactions | Treat each new command as best-effort; on 5xx return exit 5 with the upstream URL in the error so users can debug. Don't aggressively retry. |
| `boxscore` event-id-only call requires sport+league inference and may misroute in cold-cache scenarios | Default to friendly error suggesting `--sport`/`--league` flags when inference fails. |
| Cobra collision in `tv get` may already be a real bug; fixing it changes behavior | Confirm via `--help` output during Unit 4 audit; if the collision is real, plan a one-line note in CHANGELOG and treat the rename as a fix not a feature. |
| skill-doctor false positives on legitimate prose containing the binary name | Constrain extraction to fenced bash blocks and `## Command Reference` lists; ignore narrative prose. |
| Movie Goat may have its own missing-command drift surfacing during Unit 4 audit beyond TV subcommands | Cap Unit 4 scope to documented-vs-real diff; defer any "should add this missing capability" findings to follow-up issues. |
| Per-CLI PRs vs combined PR sequencing | Land Unit 1+2+3 as the ESPN PR, Unit 4 as the Movie Goat PR, Unit 5 as a third small PR. Lets each merge on its own evidence trail. |

## Documentation and Operational Notes

- Update CHANGELOG.md per CLI with a section listing new commands.
- README.md per CLI: light update only if the README explicitly enumerates commands; otherwise leave alone.
- No infra/deployment changes. No new env vars.
- After merge, agents will start using the new ESPN commands automatically the next time the plugin cache refreshes - no agent-side migration needed.

## Sources and References

- ESPN unofficial API reference: https://github.com/pseudo-r/Public-ESPN-API
- TMDb API docs: https://developer.themoviedb.org/reference/intro/getting-started
- Local: `library/media-and-entertainment/espn/spec.yaml`
- Local: `library/media-and-entertainment/espn/internal/cli/today.go`, `recap.go`, `rivals.go`, `streak.go`
- Local: `library/media-and-entertainment/movie-goat/internal/cli/`
- Cached SKILL.md (proof of drift): `/Users/mvanhorn/.claude/plugins/cache/printing-press-library/printing-press-library/1.1.10/skills/pp-espn/SKILL.md`
- Prior plan: `docs/plans/2026-04-10-001-fix-cli-quality-bugs-plan.md`

## Addendum (2026-04-19, mid-execution finding)

While starting /ce:work I read `~/cli-printing-press/internal/generator/templates/skill.md.tmpl` and learned that SKILL.md is generated from spec.yaml. The template's `## Command Reference` block iterates `.Resources` only and has no awareness of hand-written commands (today, recap, rivals, streak, watch, scores, etc.). The currently-shipped pp-espn SKILL.md was hand-edited after generation, drifted both from the template's generator output and from the real CLI surface.

This means:

- The drift is partially the generator's fault: it cannot emit a Command Reference for hand-written commands, so authors hand-edit, and hand-edits silently invalidate on next generation or fall out of sync with new code.
- The right durable fix is machine-level in `cli-printing-press`. The generator template needs either:
  - An `extra_commands:` declaration in spec.yaml that names hand-written command files and their descriptions, OR
  - A post-build introspection step that runs `<binary> --help` after generation and merges the discovered commands into the Command Reference, OR
  - A pre-flight check (Unit 5's skill-doctor, but at the generator instead of CI) that fails generation if SKILL.md drifts from --help.
- Per repo convention (`AGENTS.md`: "Default to machine changes"), Units 1-4 should not be implemented in printing-press-library until the generator side is decided. Otherwise we hand-edit SKILL.md again and the same drift returns the next time the CLI is regenerated.

Revised execution path:

1. Land this plan as a PR in printing-press-library.
2. Open a companion plan in cli-printing-press for the generator-side fix (extra_commands declaration + skill-doctor at generation time).
3. Implement the generator fix first.
4. Then regenerate ESPN and Movie Goat with the new generator and add the missing commands (Units 1-2 here).
5. SKILL.md becomes a regenerated artifact, not a hand-edited one (Unit 3 mostly disappears - it becomes "regenerate after Units 1-2").
6. Unit 5 lives in cli-printing-press as part of generator quality gates, not as a printing-press-library CI tool.

This is recorded as an addendum rather than a rewrite because the underlying problem and required commands are unchanged - only the where and order shift.
