---
title: "feat: Implement missing ESPN CLI commands and generator extra_commands support"
type: feat
status: active
date: 2026-04-19
deepened: 2026-04-19
---

# feat: Implement missing ESPN CLI commands and generator extra_commands support

## Overview

Two PRs already shipped from this work stream:

- #82: original plan capturing the SKILL.md drift evidence.
- #83: added `unknown-command` check to verify-skill (so the bug class can't reintroduce silently) and truthed-up SKILL.md docs for ESPN, cal-com, flightgoat. Plugin bumped to 1.1.14.

This refresh focuses on the actual user goal: make the ESPN commands work for real, not just remove them from the docs. Eleven commands need implementation: `boxscore`, `leaders`, `plays`, `injuries`, `odds`, `transactions`, `sos`, `h2h`, `compare`, `trending`, `dashboard`. Plus a prerequisite generator change in cli-printing-press so SKILL.md regenerates with the new commands instead of needing hand-edits.

Movie Goat audit is deferred to a separate plan to keep this one focused. Skill-doctor / drift detection is DONE in a different form: PR #83's `unknown-command` check covers the territory the original Unit 5 was scoped for.

## Problem Frame

ESPN's pp-cli currently exposes 25 commands. Eleven more are useful, were previously documented as if they existed, were removed from the docs in #83 to restore truth, and are now intentionally being added back. Each maps to a real public ESPN endpoint or a derivable cross-endpoint composition:

| Command | Endpoint family | Style |
|---|---|---|
| `boxscore <event_id>` | `summary` payload subtree | hand-written (composes `summary`) |
| `plays <event_id>` | `sports.core.api.espn.com/v2/.../events/{id}/competitions/{id}/plays` | spec-driven |
| `leaders <sport> <league>` | `site.web.api.espn.com/apis/common/v3/.../statistics/byathlete` | spec-driven |
| `injuries <sport> <league>` | `site.web.api.espn.com/apis/site/v2/.../injuries` | spec-driven |
| `transactions <sport> <league>` | `sports.core.api.espn.com/v2/.../transactions` | spec-driven (needs base-url override) |
| `odds <sport> <league>` | scoreboard `competitions[].odds` aggregation | hand-written |
| `sos <sport> <league>` | standings `strengthOfSchedule` derivation | hand-written |
| `h2h <team1> <team2>` | derived from synced data | hand-written (close to existing `rivals`) |
| `compare <athlete1> <athlete2>` | athlete search + per-athlete stats | hand-written |
| `trending` | `site.api.espn.com/apis/site/v2/trending` | hand-written (cross-league) |
| `dashboard` | reads `~/.config/espn-pp-cli/config.toml` `[favorites]` + scores/standings | hand-written (config + cliutil.FanoutRun) |

Generator constraint surfaced in #83: `internal/generator/templates/skill.md.tmpl` `## Command Reference` block iterates `.Resources` only. Hand-written commands like `today`, `streak`, `rivals` never appear in regenerated SKILL.md. That's why prior SKILL.md hand-edits drifted - they were the only way to document hand-written commands. Solution: add an `extra_commands:` array to spec.yaml so authors declare hand-written commands once and the template iterates them too.

## Requirements Trace

- R1. All eleven listed ESPN commands resolve in `espn-pp-cli --help` after this work lands and return live data via `--agent` for an in-season league.
- R2. The cli-printing-press generator accepts an `extra_commands:` array in spec.yaml and includes those entries in the generated SKILL.md `## Command Reference` block, in declaration order, after the spec-driven resources.
- R3. Regenerating ESPN through the new generator preserves all hand-tuned content currently in `library/media-and-entertainment/espn/SKILL.md` that is NOT inside `## Command Reference`. Use `printing-press patch` if full regen would lose context.
- R4. The verify-skill `unknown-command` check (shipped in #83) reports zero errors for ESPN after each new command lands.
- R5. Existing ESPN commands and the existing SKILL.md sections (`## Unique Capabilities`, `## Recipes`, `## Auth Setup`, `## Agent Mode`, `## Exit Codes`, `## Installation`, `## Argument Parsing`, `## Agent Workflow Features`) continue to work without regression.
- R6. Per AGENTS.md, every SKILL.md change is paired with a `tools/generate-skills/main.go` rerun and a `plugin/.claude-plugin/plugin.json` patch bump in the same commit.
- R7. New commands write to local SQLite via `internal/store/` upserts where the response is naturally row-shaped (leaders, plays, injuries, transactions). Aggregations and read-only views (boxscore, odds, sos, h2h, trending, dashboard) skip the store.
- R8. Commands that hit endpoints behind the optional API tier (none today for ESPN) document that gracefully. Today all ESPN endpoints are no-auth.

## Scope Boundaries

In scope:

- cli-printing-press: extend `internal/spec/spec.go`, `internal/generator/templates/skill.md.tmpl`, and the existing skill_test.go coverage to support `extra_commands:`.
- printing-press-library: extend `library/media-and-entertainment/espn/spec.yaml` with the four spec-driven resources, hand-write the seven composition/aggregation commands, declare all eleven in `extra_commands:` for SKILL.md regen.
- printing-press-library: regenerate plugin/skills/pp-espn/SKILL.md, bump plugin.json.
- Tests: per-command `_test.go` for hand-written commands; spec_test.go coverage for the new generator field; verify-skill check stays green.

Out of scope (explicit non-goals):

- Movie Goat audit and additions. Splits into its own plan.
- Per-command MCP tool wrappers beyond what spec-driven resources auto-generate. Hand-written commands stay CLI-only this round.
- Schema migrations to existing tables. New tables only via `CREATE TABLE IF NOT EXISTS` in the existing store package.
- Cross-CLI shared library work (e.g., shared agent-mode flags). Covered by PR #218.
- Live-polling (websocket) variants. ESPN endpoints stay polling-only; `watch` is the existing live path.
- A new dashboard config UI. The `[favorites]` TOML table is hand-edited.
- Backfilling SKILL.md drift fixes for any other CLI - hubspot's two false-positives, etc., stay in the verify-skill `[likely false positive]` bucket.

## Context and Research

### Relevant Code and Patterns

cli-printing-press generator:

- `internal/spec/spec.go:21` `APISpec` struct - top-level YAML model, add `ExtraCommands` field here.
- `internal/spec/spec.go:95` `Resource` struct - shape to mirror loosely for `ExtraCommand` if we need richer fields.
- `internal/generator/templates/skill.md.tmpl:65` `## Command Reference` block iterates `.Resources` - extend to also iterate `.ExtraCommands`.
- `internal/generator/skill_test.go` - test pattern for skill template expectations; add a new test that asserts extra_commands appear in the rendered SKILL.md.
- `internal/generator/generator.go:681` template-to-output mapping (`skill.md.tmpl` → `SKILL.md`) - already wired, no change needed.

ESPN package:

- `library/media-and-entertainment/espn/spec.yaml` - 5 existing resources: `scoreboard`, `teams`, `news`, `summary`, `rankings`. Pattern for adding new ones is `news:` and `summary:` blocks (clean, single-endpoint shape).
- `library/media-and-entertainment/espn/internal/cli/today.go` - hand-written cross-league fetch, single-shot. Pattern for `trending` and `dashboard`.
- `library/media-and-entertainment/espn/internal/cli/rivals.go` - hand-written team-vs-team derivation from synced data. Pattern for `h2h`.
- `library/media-and-entertainment/espn/internal/cli/streak.go` - hand-written derivation from synced data. Pattern for `sos`.
- `library/media-and-entertainment/espn/internal/cli/recap.go` - hand-written sport+league argument parsing. Pattern for `boxscore` argument shape.
- `library/media-and-entertainment/espn/internal/client/` - HTTP client wrapping ESPN's three API surfaces (`site.api`, `sports.core.api`, `site.web.api`). Already covers all the bases needed.
- `library/media-and-entertainment/espn/internal/store/` - SQLite upsert pattern for new domain tables.
- `library/media-and-entertainment/espn/internal/cliutil/fanout.go` (likely) - per cli-printing-press AGENTS.md, `cliutil.FanoutRun` is the canonical aggregation pattern with per-source error collection and bounded concurrency. `dashboard` and `trending` should use it.

Repo plumbing:

- `tools/generate-skills/main.go` - regenerates `plugin/skills/pp-*/SKILL.md` from `library/<cat>/<cli>/SKILL.md` plus `--help` enrichment. Run after every library SKILL.md change.
- `plugin/.claude-plugin/plugin.json` - bump `version` patch component in same commit as SKILL changes.
- `.github/scripts/verify-skill/verify_skill.py` - now includes `unknown-command` check (PR #83). Will fail any PR that documents commands missing from `internal/cli/*.go`.
- `.github/workflows/verify-skills.yml` - triggers on `library/**/SKILL.md`, `library/**/internal/cli/**`, and `.github/scripts/verify-skill/**`.

### Institutional Learnings

- PR #83 (this work stream): generator-side fix is the durable solution. Hand-edits to SKILL.md silently invalidate on regen. The `extra_commands:` declaration closes that loop.
- cli-printing-press AGENTS.md: "Default to machine changes." Generator extension lands first; ESPN consumes it.
- cli-printing-press AGENTS.md: `cliutil` namespace is generator-reserved. Hand-written code must not collide with `cliutil.FanoutRun` or `cliutil.CleanText` exports.
- cli-printing-press AGENTS.md: dogfood marks path-validity skipped when `kind: synthetic`. ESPN is currently `kind: rest` (default) - new endpoints will run path-validity checks. Verify each spec.yaml endpoint URL with a real curl before merging.
- printing-press-library AGENTS.md: SKILL.md content changes do NOT trigger automatic plugin.json bump. Manual patch bump every time.
- 2026-04-10 fix-cli-quality-bugs plan: prior round of CLI hygiene. Pattern was small atomic commits per binary, dogfood verification before merge.
- Memory `feedback_pp_go_install_goprivate.md`: PP CLIs need `GOPRIVATE='github.com/mvanhorn/*'` for fresh installs. Local verification commands need that prefix.
- Memory `feedback_evidence_every_pr.md`: every PR in a batch needs evidence. Splits this plan into separate PRs by repo and by command cluster.

### External References

- ESPN unofficial API reference: https://github.com/pseudo-r/Public-ESPN-API - community-maintained list of working endpoints, current as of 2025-12.
- ESPN trending endpoint sample response (verified 2026-04-19): `https://site.api.espn.com/apis/site/v2/trending` returns athletes and teams with `entity_type`, `name`, `league`, `popularity` fields.
- Cobra subcommand registration: https://pkg.go.dev/github.com/spf13/cobra - referenced for `rootCmd.AddCommand` patterns when wiring new commands into root.go.

## Key Technical Decisions

- Generator change first, ESPN second. The generator extension is small and unblocks SKILL.md regen for ESPN. Doing ESPN first would force another round of hand-edits that re-trigger the drift class. Sequence: cli-printing-press PR → release → printing-press-library bumps generator dep → ESPN consumes.
- `extra_commands:` shape is intentionally minimal. Each entry has `name`, `description`, optional `subcommands` for parent commands with leaves. No `endpoint` field - extra_commands are by definition non-endpoint-driven. Avoids overlap with `Resource` while staying YAML-readable.
- Spec-driven for `injuries`, `transactions`, `leaders`, `plays`. These have predictable single-endpoint response shapes, good upsert candidates for the local store, and they get free MCP tool exposure via the existing spec→mcp generator path. Hand-written would deny MCP coverage.
- Hand-written for `boxscore`, `odds`, `sos`, `h2h`, `compare`, `trending`, `dashboard`. These compose multiple endpoints, derive from synced data, or read local config. Spec representation would either fail to express the composition or force a contrived single-endpoint shim.
- `boxscore <event_id>` defaults to inferring `--sport`/`--league` from a recent scores cache; falls back to required flags when cache misses. Avoids forcing the user to remember sport+league for an event ID they just got from `scores`.
- `transactions` needs a per-resource `base_url_override` because it lives on `sports.core.api.espn.com` while ESPN's default `base_url` in spec.yaml is `site.api.espn.com`. Generator already supports this via the `base_url` field on the Endpoint type? Verify in implementation; if not, this is part of the generator PR's scope.
- `dashboard` reads `[favorites]` from existing config file. Schema: `nfl = ["KC", "BAL"]`, keyed by league abbreviation. Simplest TOML shape that supports per-league favorites; matches how scores/standings already key on league.
- `h2h <team1> <team2>` versus existing `rivals <sport> <league>`. Decision: keep both. `rivals` returns league-wide pairwise records; `h2h` returns one specific pair with deeper detail (recent meetings, score history, key players). Different question, different command.
- Per-CLI versus combined PRs. Splits into three PRs:
  1. cli-printing-press: `extra_commands:` support.
  2. printing-press-library: ESPN spec-driven additions (Unit 2) + extra_commands declaration + regen.
  3. printing-press-library: ESPN hand-written commands (Unit 3) + regen.
  Lets each merge on its own evidence trail; matches `feedback_evidence_every_pr.md`.

## Open Questions

### Resolved During Planning

- Should SKILL.md remain hand-edited or become regenerable? Regenerable, gated by `extra_commands:`. Resolved by PR #83's discovery and the cli-printing-press AGENTS.md "default to machine changes" rule.
- Spec.yaml or hand-written for each of the eleven? Decided per-command in the table above.
- Add MCP coverage for hand-written commands? No, not this round. Spec-driven commands get MCP for free via existing spec→mcp path; hand-written stay CLI-only until users request MCP.
- Per-CLI versus combined PR? Three PRs, sequenced as listed.
- ESPN endpoint stability for plays/leaders/transactions/injuries? Confirmed via community reference and ESPN's own scoreboard frontend usage. Acceptable risk.
- Should `dashboard` poll-refresh? No, one-shot. `watch` is the existing live-poll path.

### Deferred to Implementation

- Whether the generator already supports per-endpoint `base_url_override` for the `transactions` endpoint living on `sports.core.api.espn.com`. Resolve by reading `internal/spec/spec.go` Endpoint definition during Unit 1.
- Exact response field selection for `--compact` output of each new command. Pick during implementation by inspecting one live response.
- Whether `leaders` ships with `--category passing,rushing,receiving` filters at launch or in a follow-up. Probable: minimal `--category <name>` flag; let users discover via help.
- For `compare`, how to disambiguate athlete name searches that return multiple matches. Probable: list candidates and exit 2.
- Whether `h2h` should auto-sync if no local data exists. Probable: yes, with a one-line "syncing first" notice on stderr.

## High-Level Technical Design

> This illustrates the intended approach and is directional guidance for review, not implementation specification. The implementing agent should treat it as context, not code to reproduce.

The shape of the `extra_commands:` declaration in spec.yaml:

```yaml
extra_commands:
  - name: boxscore
    description: "Full box score for a specific event id"
    args: "<event_id>"
  - name: trending
    description: "Most-followed athletes and teams across all leagues"
  - name: dashboard
    description: "Your favorite teams' status at a glance"
  - name: h2h
    description: "Head-to-head detail between two teams"
    args: "<team1> <team2>"
  - name: compare
    description: "Side-by-side athlete stat comparison"
    args: "<athlete1> <athlete2>"
  - name: sos
    description: "Strength-of-schedule derivation from standings"
    args: "<sport> <league>"
  - name: odds
    description: "Spread/total/moneyline lines for tonight's slate"
    args: "<sport> <league>"
```

The generated `## Command Reference` block becomes:

```
## Command Reference

scoreboard - Live and historical game scores
- espn-pp-cli scoreboard get - Get scoreboard for a sport and league
... (other resources)

Hand-written commands:
- espn-pp-cli boxscore <event_id> - Full box score for a specific event id
- espn-pp-cli trending - Most-followed athletes and teams across all leagues
... (other extra_commands)
```

End-to-end flow for one new ESPN command (e.g. `injuries`):

```
spec.yaml resource declaration
  -> printing-press generate (spec-driven)
  -> internal/cli/promoted_injuries.go appears
  -> internal/store/injuries_store.go appears (if upsertable)
  -> binary gains `injuries` command
  -> tools/generate-skills/main.go regen picks it up via --help walk
  -> plugin/skills/pp-espn/SKILL.md updated with new command
  -> verify-skill unknown-command check passes
```

For a hand-written command (e.g. `dashboard`):

```
internal/cli/dashboard.go (hand-written, registers via init())
  -> binary gains `dashboard` command
  -> spec.yaml extra_commands: declares it
  -> printing-press generate -> SKILL.md command reference includes it
  -> tools/generate-skills/main.go regen picks it up
  -> plugin/skills mirror updated
  -> verify-skill unknown-command check passes
```

## Implementation Units

- [x] Unit 1: cli-printing-press extra_commands: spec field + template support (shipped via mvanhorn/cli-printing-press#227, merged 2026-04-19)

Goal: Authors can declare hand-written commands in spec.yaml so they appear in the generated SKILL.md `## Command Reference`.

Requirements: R2

Dependencies: None

Files:
- Modify: `internal/spec/spec.go` (add `ExtraCommands []ExtraCommand` to `APISpec`, define `ExtraCommand` struct with `Name`, `Description`, `Args` fields)
- Modify: `internal/spec/spec_test.go` (round-trip parse test for the new field, including absent/empty/populated cases)
- Modify: `internal/generator/templates/skill.md.tmpl` (extend `## Command Reference` block to iterate `.ExtraCommands` after `.Resources`, with a small subheading)
- Modify: `internal/generator/skill_test.go` (assert rendered SKILL.md contains each extra_command's name and description)
- Modify: `internal/generator/generator.go` if any field plumbing changes are required to pass extra_commands through to template data

Approach:
- Mirror the YAML/JSON tags pattern used by `Resource` for the new `ExtraCommand` struct.
- Validate that `Name` is a valid Go identifier-ish slug (lowercase + hyphens) at parse time. Empty `Name` is a parse error.
- Template iteration uses `{{range .ExtraCommands}}` analogous to the existing `{{range $name, $resource := .Resources}}` block.
- Output format keeps inline backticks for the binary+command snippet so verify-skill's parser picks them up.

Patterns to follow:
- `Resource` struct in `internal/spec/spec.go` for the YAML tag style.
- `## Command Reference` block in `internal/generator/templates/skill.md.tmpl` for the template iteration shape.
- Existing skill_test.go tests for assertion patterns.

Test scenarios:
- Happy path: spec.yaml with three extra_commands renders SKILL.md containing all three with correct binary prefix and description.
- Edge case: spec.yaml with empty/absent extra_commands renders the SKILL.md exactly as before (backwards compatible).
- Edge case: an extra_command with no `Args` renders as `binary name - description` with no trailing args placeholder.
- Error path: spec.yaml with an extra_command missing `Name` produces a parse error referencing the offending entry.
- Error path: spec.yaml with an extra_command whose Name contains invalid chars (uppercase, spaces) produces a parse error.
- Integration: generator end-to-end (`printing-press generate` against a small fixture spec with extra_commands) produces a binary whose `--help` and SKILL.md agree on the command list.

Verification:
- `go test ./internal/spec/... ./internal/generator/...` passes.
- A fixture spec with extra_commands generates a SKILL.md whose `## Command Reference` includes the new entries.
- Round-trip: edit a real spec.yaml (use a copy, not ESPN), regenerate, observe the new commands in the rendered SKILL.md.

- [ ] Unit 2: ESPN spec.yaml extensions for injuries, transactions, leaders, plays

Goal: Add four spec-driven resources to ESPN. Each gets a generated promoted command and (where naturally row-shaped) a domain table.

Requirements: R1, R2, R6, R7

Dependencies: Unit 1 (regen needs the new generator)

Files:
- Modify: `library/media-and-entertainment/espn/spec.yaml`
- Regenerate: `library/media-and-entertainment/espn/internal/cli/promoted_injuries.go`, `promoted_transactions.go`, `promoted_leaders.go`, `promoted_plays.go`
- Regenerate: types in `library/media-and-entertainment/espn/internal/types/`
- Modify or regenerate: `library/media-and-entertainment/espn/internal/store/injuries_store.go` (and similar for transactions, leaders, plays) if the store layer auto-generates from spec resources

Approach:
- Mirror existing resource shape (`news:` block is closest in shape).
- `injuries`: GET on `site.web.api.espn.com/apis/site/v2/sports/{sport}/{league}/injuries`. Sport+league positional. No required flags.
- `transactions`: GET on `sports.core.api.espn.com/v2/sports/{sport}/leagues/{league}/transactions`. Needs the `base_url_override` mechanism resolved in Unit 1's deferred question. Sport+league positional.
- `leaders`: GET on `site.web.api.espn.com/apis/common/v3/sports/{sport}/{league}/leaders` (preferred) or the sports.core.api.espn.com equivalent. Pick the cleaner per-stat shape during implementation. Sport+league positional, optional `--category <name>`.
- `plays`: GET on `sports.core.api.espn.com/v2/sports/{sport}/leagues/{league}/events/{eventId}/competitions/{eventId}/plays?limit=1000`. Event id positional, optional `--limit` (default 200).
- After spec changes, run `printing-press generate` against ESPN. Verify the binary builds and the new commands appear in `--help`.

Patterns to follow:
- spec.yaml `news:` and `summary:` blocks for resource shape.
- `internal/store/news_store.go` for the upsert pattern when the new resource gets a domain table.

Test scenarios:
- Happy path: `espn-pp-cli injuries baseball mlb --agent` returns at least one injury record with `athlete`, `team`, `status` fields.
- Happy path: `espn-pp-cli leaders baseball mlb --agent` returns a list of athletes with stats for the current season.
- Happy path: `espn-pp-cli plays basketball nba --event <live_event_id> --agent` returns plays for an in-progress game.
- Happy path: `espn-pp-cli transactions baseball mlb --agent` returns recent trades/signings/waivers with `team`, `player`, `kind`, `date` fields.
- Edge case: `espn-pp-cli injuries football nfl --agent` during the offseason returns an empty list, exit 0, not exit 3.
- Edge case: `espn-pp-cli plays basketball nba --event <pre_game_id> --agent` returns empty plays array with exit 0.
- Error path: `espn-pp-cli plays basketball nba --event 0 --agent` returns exit 3 with structured not-found error.
- Error path: `espn-pp-cli leaders football nfl --category nonexistent --agent` returns exit 2 with usage error.
- Integration: `espn-pp-cli sync --sport baseball --league mlb` populates the new `leaders`, `plays`, `injuries`, `transactions` tables; verify with `sqlite3 ~/.local/share/espn-pp-cli/db.sqlite ".tables"`.

Verification:
- New commands appear in `espn-pp-cli --help`.
- Each command's `--help` includes at least one example.
- `espn-pp-cli doctor` reports OK after running each command.
- `dogfood-results.json` regenerated with new commands covered.
- Full ESPN test suite passes.

- [ ] Unit 3: ESPN hand-written commands - boxscore, odds, sos, h2h, compare, trending, dashboard

Goal: Add seven hand-written commands that compose multiple endpoints or read local state.

Requirements: R1, R2, R6

Dependencies: Unit 1 (so SKILL.md can declare these via extra_commands:)

Files:
- Create: `library/media-and-entertainment/espn/internal/cli/boxscore.go` and `boxscore_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/odds.go` and `odds_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/sos.go` and `sos_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/h2h.go` and `h2h_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/compare.go` and `compare_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/trending.go` and `trending_test.go`
- Create: `library/media-and-entertainment/espn/internal/cli/dashboard.go` and `dashboard_test.go`
- Modify: `library/media-and-entertainment/espn/internal/config/` (or create) to add `[favorites]` table loader
- Modify: existing config docs to mention the `[favorites]` schema

Approach:
- `boxscore <event_id>`: detect sport+league from a recent `scores` cache lookup if present; otherwise require `--sport`/`--league`. Fetch via existing `summary` client method, return only the `boxscore` subtree. Agent mode returns raw JSON; human mode renders leaders + line score table.
- `odds <sport> <league>`: list current games with their `pickcenter` lines from each game's summary. Avoid one summary call per game by reading the scoreboard's `competitions[].odds` field which already carries the lines.
- `sos <sport> <league>`: fetch standings, extract `strengthOfSchedule` and `remainingStrengthOfSchedule` per team. Sort by `strengthOfSchedule` descending in human view.
- `h2h <team1> <team2>`: resolve abbreviations via teams list, fetch each team's schedule, intersect by opponent, aggregate W-L plus average scores. Reuse `internal/cli/rivals.go` pattern but scoped to one pair.
- `compare <athlete1> <athlete2>`: search athletes via `site.web.api.espn.com/apis/common/v3/sports/{sport}/{league}/athletes?search=<name>`, fetch per-athlete season stats, render side-by-side table. Require `--sport`/`--league`. Ambiguous matches list candidates and exit 2.
- `trending`: hit `https://site.api.espn.com/apis/site/v2/trending`, return ranked list across leagues with per-entry `entity_type`, `name`, `league`, `popularity`.
- `dashboard`: read `[favorites]` from config; for each configured league fetch scores and standings for the favorited teams; render single table grouped by league. Use `cliutil.FanoutRun` for parallel per-league fetches.

Execution note: Implement test-first for `dashboard` and `boxscore` because they have non-obvious sport+league inference and config-shape dependencies. Other commands can be pragmatic.

Patterns to follow:
- `today.go` for cross-league fetches.
- `rivals.go` for h2h-style derivations.
- `streak.go` for derivation from synced data.
- `recap.go` for sport+league argument parsing.
- `cliutil.FanoutRun` for `dashboard` (per-source error collection, bounded concurrency).

Test scenarios:
- Happy path (boxscore, hot cache): `espn-pp-cli boxscore 401869192 --agent` returns boxscore for a known live or recent NBA game without `--sport`/`--league`.
- Happy path (boxscore, cold cache + flags): `boxscore 401869192 --sport basketball --league nba --agent` works after clearing the scores cache.
- Edge case (boxscore): pre-game state returns boxscore subtree with empty player stats and exit 0.
- Error path (boxscore): cold cache, no flags, returns exit 2 with hint to provide `--sport`/`--league`.
- Happy path (odds): `espn-pp-cli odds basketball nba --agent` returns spreads, totals, and moneylines for tonight's slate.
- Edge case (odds): no games scheduled returns empty list and exit 0.
- Happy path (sos): `espn-pp-cli sos football nfl --agent` returns 32 entries sorted by SOS descending.
- Happy path (h2h): `espn-pp-cli h2h chiefs eagles --agent` returns historical W-L and average scores.
- Error path (h2h): unknown team abbreviation returns exit 3 with structured error naming the invalid arg.
- Happy path (compare): `espn-pp-cli compare Mahomes Allen --sport football --league nfl --agent` returns side-by-side stats.
- Error path (compare): ambiguous name returns exit 2 with a list of candidate athlete IDs.
- Happy path (trending): `espn-pp-cli trending --agent` returns at least 5 entries with `entity_type`, `name`, `league`.
- Happy path (dashboard): with a populated `[favorites]` block, `espn-pp-cli dashboard --agent` returns scores+standings for each favorited team.
- Edge case (dashboard): empty `[favorites]` returns exit 0 with a hint about how to add favorites.
- Edge case (dashboard): one league in `[favorites]` returns 5xx for one team - the result reports the partial failure but the other teams still come through (via `cliutil.FanoutRun` per-source error collection).

Verification:
- All seven commands appear in `espn-pp-cli --help`.
- Each command's `--help` includes at least one example.
- `espn-pp-cli doctor` continues to report OK.
- Per-command `_test.go` covers the listed scenarios with table-driven tests against recorded fixtures.

- [ ] Unit 4: ESPN spec.yaml extra_commands: + SKILL.md regeneration

Goal: Declare all seven hand-written commands in spec.yaml's new `extra_commands:` block, regenerate SKILL.md, regenerate plugin/skills mirror, bump plugin.json.

Requirements: R1, R3, R5, R6

Dependencies: Unit 1 (generator support), Unit 3 (commands must exist before docs can claim them - verify-skill catches the inverse)

Files:
- Modify: `library/media-and-entertainment/espn/spec.yaml` (add `extra_commands:` block listing all seven hand-written commands plus the four spec-driven ones from Unit 2 if they need promotion in the SKILL.md `## Command Reference`)
- Regenerate: `library/media-and-entertainment/espn/SKILL.md` via printing-press
- Modify: `library/media-and-entertainment/espn/SKILL.md` to restore the truthful descriptions and recipe blocks from PR #83 - do not lose hand-tuned content outside `## Command Reference`. Use `printing-press patch` if the regen would lose context.
- Run: `tools/generate-skills/main.go` to regenerate `plugin/skills/pp-espn/SKILL.md`
- Modify: `plugin/.claude-plugin/plugin.json` (bump `version` patch component)

Approach:
- Add the `extra_commands:` block to spec.yaml in the order users will most likely look for them: discovery (`trending`, `dashboard`), game detail (`boxscore`), team analysis (`h2h`, `compare`), league analysis (`odds`, `sos`).
- After regen, diff the SKILL.md against the PR #83 baseline and confirm all hand-tuned content (Unique Capabilities, Recipes, Auth Setup, Agent Mode, Exit Codes, Installation, Argument Parsing, Agent Workflow Features) survives.
- If regen overwrites hand-tuned sections, switch to `printing-press patch` for the targeted edits and re-run only the `## Command Reference` regeneration.
- Update Recipes section to use the now-real `boxscore`, `odds`, and `dashboard` examples from this work stream.
- Run verify-skill against ESPN. Should report 0 errors.

Patterns to follow:
- AGENTS.md SKILL.md sync workflow.
- PR #83 commit style for the regen+bump combined commit.

Test scenarios:
- Happy path: `python3 .github/scripts/verify-skill/verify_skill.py --dir library/media-and-entertainment/espn/` exits 0.
- Happy path: every command in the regenerated SKILL.md `## Command Reference` resolves under `espn-pp-cli <cmd> --help`.
- Edge case: the regenerated SKILL.md preserves all hand-tuned non-Command-Reference sections from PR #83.
- Integration: full plugin sweep via the verify-skills CI workflow passes for all 13 CLIs (no other CLI regresses).

Verification:
- `library/media-and-entertainment/espn/SKILL.md` and `plugin/skills/pp-espn/SKILL.md` agree on the command list.
- Both files reference all eleven new commands plus all existing commands.
- `plugin/.claude-plugin/plugin.json` bumped one patch version.
- Local verify-skill sweep across all 13 CLIs reports 0 errors.

- [ ] Unit 5: skill-doctor / drift detection - DONE in PR #83

Goal: A check that prevents SKILL.md from documenting commands the binary does not implement.

Status: Shipped via PR #83 in a different form. Instead of a new top-level tool, the existing `.github/scripts/verify-skill/verify_skill.py` gained a fourth `unknown-command` check. CI workflow path filter now includes `.github/scripts/verify-skill/**`. Coverage matches the spirit of the original Unit 5 (no documented-but-missing commands can land silently).

Files (already shipped):
- Modified: `.github/scripts/verify-skill/verify_skill.py` - new `check_unknown_commands` function plus `--only unknown-command` flag.
- Modified: `.github/scripts/verify-skill/README.md` - documents the new check.
- Modified: `.github/workflows/verify-skills.yml` - path filter expanded.

No further work in this unit.

## System-Wide Impact

- Interaction graph: ESPN's MCP server auto-registers spec-driven resources, so Unit 2 expands MCP tool surface. Hand-written commands in Unit 3 stay CLI-only. Verify-skill check from PR #83 now gates SKILL.md drift across all 13 library CLIs.
- Error propagation: New commands must use existing exit-code conventions (0 success, 2 usage, 3 not found, 5 API, 7 rate limited). Unit 3 test scenarios assert specific exit codes per failure case.
- State lifecycle risks: New domain tables in Unit 2 mean `sync` schema migrations. Store package likely uses `CREATE TABLE IF NOT EXISTS`; verify in implementation. If not, a one-time migration runner is needed before merge.
- API surface parity: SKILL.md ↔ `--help` ↔ README.md should agree. Unit 4's verify-skill run gates SKILL.md vs `--help`. README.md spot-checked but lower-leverage.
- Integration coverage: dogfood-results.json regenerated; CI includes new commands' example execution.
- Unchanged invariants: existing exit codes, config path (`~/.config/espn-pp-cli/config.toml`), agent-mode flag set, cache layout, SQLite path, MCP server registration mechanism.

## Risks and Dependencies

| Risk | Mitigation |
|------|------------|
| ESPN endpoint stability for plays/leaders/transactions/injuries | Treat each new command as best-effort; on 5xx return exit 5 with the upstream URL in the error so users can debug. Don't aggressively retry. |
| `boxscore` event-id-only call requires sport+league inference and may misroute on cold cache | Default to friendly error suggesting `--sport`/`--league` flags when inference fails. Test scenario covers this case explicitly. |
| Generator extension breaks existing CLIs that have no extra_commands block | YAML parser must treat absent extra_commands as empty/no-op. Unit 1 explicitly tests the absent case. Run a regression: regenerate one existing CLI (yahoo-finance, kalshi) and confirm SKILL.md is byte-identical. |
| `printing-press patch` versus full regen for ESPN's SKILL.md (Unit 4) | If full regen would lose hand-tuned content, fall back to patch. Decision deferred to implementation since it depends on actual regen behavior with the new extra_commands block. |
| transactions endpoint requires base_url_override that may not exist in generator | Resolve in Unit 1 by reading Endpoint struct. If missing, add it to the generator scope (still Unit 1's PR). Hand-write transactions instead if the generator change is too risky. |
| Three-PR sequencing requires release of cli-printing-press before printing-press-library can consume it | Manual: tag a cli-printing-press release after Unit 1 lands; bump printing-press-library's generator dependency in the Unit 2 PR. |
| dashboard config schema is a new public surface; future schema evolution is a compatibility commitment | Document `[favorites]` schema explicitly in SKILL.md and the dashboard `--help`. Reserve unknown keys for forward compatibility. |
| Movie Goat SKILL.md drift backlog (the original Unit 4) is now untracked | File a follow-up issue when this plan ships. Not blocking for the user-stated goal of making ESPN commands work. |

## Documentation and Operational Notes

- Update CHANGELOG.md per CLI with a section listing new commands.
- Update ESPN README.md to list the new commands (light edit since README mostly defers to SKILL.md and `--help`).
- No infra/deployment changes. No new env vars. No auth changes.
- After merge, agents start using new commands automatically the next time the plugin cache refreshes - no agent-side migration.
- cli-printing-press release notes for the Unit 1 PR should call out `extra_commands:` as a new spec field for hand-written command authors.

## Sources and References

- Origin: this plan refreshed from prior version on 2026-04-19; PR #82 captured the original.
- Generator code: `~/cli-printing-press/internal/spec/spec.go`, `~/cli-printing-press/internal/generator/templates/skill.md.tmpl`, `~/cli-printing-press/internal/generator/skill_test.go`
- ESPN code: `library/media-and-entertainment/espn/spec.yaml`, `library/media-and-entertainment/espn/internal/cli/{today,recap,rivals,streak}.go`
- Plugin tooling: `tools/generate-skills/main.go`, `plugin/.claude-plugin/plugin.json`, `.github/scripts/verify-skill/verify_skill.py`
- ESPN unofficial API reference: https://github.com/pseudo-r/Public-ESPN-API
- Trending endpoint sample: https://site.api.espn.com/apis/site/v2/trending (verified 2026-04-19)
- Prior plans: `docs/plans/2026-04-10-001-fix-cli-quality-bugs-plan.md`
- Shipped PRs in this work stream: #82 (plan), #83 (verify-skill + truth-up)
- Shipped issues closed by #83: #84 (espn drift), #85 (cal-com), #86 (flightgoat)
