# Design: Restore the 8 dropped commands (backward-compat with the 2026-05-15 build)

**Date:** 2026-05-29
**Working copy (git-tracked):** `/Users/smacdonald/homegit/suno-cli` (module `suno-pp-cli`) — the generated CLI is mirrored here from the publish-staging copy so all work has git history.
**Publish staging:** `/Users/smacdonald/printing-press/library/suno` (what `/printing-press-publish` reads; kept in sync via a mirror step).
**Status:** Approved design — ready for implementation plan

## Problem

The current build is a fresh PP 4.14.0 reprint that re-derived its novel-feature
command set. It dropped 8 top-level commands that existed in the **2026-05-15
published build** (`suno-current` release, PP 4.6.1). Anyone on that build loses
those commands and their workflows.

Diff of the old published binary vs. the current build (commands present in old,
absent in new):

| Old command | Behavior | New analog |
|---|---|---|
| `vibes` | Save/replay local generation recipes (list/save/get/delete/use) | none |
| `burn` | Aggregate estimated credits by tag/persona/model/hour | overlaps `analytics` (different flags/output) |
| `budget` | Show/set/clear local credit caps + enforce on generate | none |
| `sessions` | Group synced clips into 30-min-gap sessions | none |
| `ship` | Editor-ready publishing bundle (audio/video/cover/lrc/json) for a clip | none |
| `tail` | Poll the API and stream changes as NDJSON | none |
| `tree` | Render local clip lineage tree | rename of `lineage` |
| `custom-model` | List pending custom-model training jobs | none |

## Decision

**Restore all 8 verbatim** as first-class commands that coexist with the new
`analytics`/`lineage`/etc. No aliasing. `tree` and `lineage` both exist; `burn`
and `analytics` both exist. The complete old surface keeps its exact behavior and
output; the new surface is unaffected.

Rejected alternatives: aliasing `tree→lineage` / `burn→analytics` (lossy — burn's
flags and output shape differ from analytics; a cobra alias can't translate
them); dropping `custom-model` (it has no new analog, so dropping it is a real
capability loss).

## Why this is a port, not a rewrite

The old source for all 8 commands plus their shared helper file
(`transcendence_helpers.go`) lives in the fork at
`library/media-and-entertainment/suno/internal/cli/`. That helper file is
**already written against the current store API** (`store.OpenWithContext`,
`defaultDBPath("suno-pp-cli")`). Every helper the 8 commands depend on
(`openExistingStore`, `openDefaultStore`, `readClipRaw`, `unmarshalObject`,
`clipCreatedAt/Tags/PersonaID/Model/ParentID/Title`, `splitList`, `mustJSON`,
`sortedKeys`, `stringAtAny`, `numberAtAny`, `timeAtAny`, `valueAt`) is in that
file. Verified: all of `custom-model`'s heavier deps (`resolveRead`,
`extractResponseData`, `wantsHumanTable`, `printProvenance`,
`wrapWithProvenance`, `filterFields`, `compactFields`, `printAutoTable`,
`isTerminal`, `printOutput`) already exist in the current `internal/cli/`.

## Components / files

New files under `internal/cli/` (matching the `suno_*.go` novel-feature naming):

- `suno_transcendence_helpers.go` — ported from `transcendence_helpers.go`
- `suno_vibes.go`, `suno_burn.go`, `suno_budget.go`, `suno_sessions.go`,
  `suno_ship.go`, `suno_tail.go`, `suno_tree.go`, `suno_custom_model.go`
- Registration: add 8 `AddCommand` calls in `root.go`

## The only deltas from verbatim (mechanical, per file)

1. **Import path:** `github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/*` → `github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/*`
2. **Client context arg:** the current client takes `ctx` first —
   `c.Get(path, params)` → `c.Get(cmd.Context(), path, params)`;
   `c.Post(path, body)` → `c.Post(cmd.Context(), path, body)`.
   Signatures confirmed: `Get(ctx, path, params) (json.RawMessage, error)`,
   `Post(ctx, path, body) (json.RawMessage, int, error)`.
3. **Collision drops:** remove `sanitizeFilename` and `defaultDBPath` from the
   ported helper file — both already exist (`suno_download.go`, `helpers.go`).
   These are the only 2 collisions across the entire port.
4. **Header normalization:** `tail` and `custom-model` carried
   "Generated … DO NOT EDIT" headers; they become hand-authored. Use the
   current files' `Copyright 2026 horknfbr. Licensed under Apache-2.0.` header.
5. **`tail` interface:** `fetchAndEmit`'s embedded `Get(string, map[string]string)`
   interface gains the `ctx` parameter to match the new client.

## Data source (technical call)

Read-commands (`burn`, `sessions`, `tree`) keep reading the generic `resources`
table (`resource_type IN ('clip','clips')`) exactly as the old build did. The
current sync still populates that table, so this is true-verbatim behavior with
zero divergence risk. The new `lineage`/`analytics` read the typed `clips`
table; both paths coexist. `vibes` and `budget` persist their own
`resource_type`s (`vibe`, `budget_setting`) via `store.Upsert`, unchanged.

## Budget enforcement (cross-cutting — approved)

The old `budget.go` shipped `budgetCapExceeded(ctx, store)` and the old generate
path called it to block generations over a configured daily/monthly cap (a
deliberate greptile-P1 fix). The current generate path
(`suno_generate_run.go` / `suno_generate.go`) does not. To make `budget`
functional rather than informational, add the same pre-flight check to the new
generate path: before submitting a generation, open the store, call
`budgetCapExceeded`, and abort with a clear error if the cap would be breached.
This is the one edit outside the new command files. It is gated so a missing/empty
budget setting is a no-op (never blocks when no cap is set), and it must not run
in dry-run / verify mode.

## Error handling

Each command preserves its original error semantics: `notFoundErr` for missing
clips/recipes, `usageErr` for bad flags, `classifyAPIError` for API failures,
and the "Run 'suno-pp-cli sync' first" hint when the local store is empty.
`ship` and `budget`/`vibes` writes respect `--dry-run` and the verify-env guard
(`PRINTING_PRESS_VERIFY` / `cliutil.IsVerifyEnv`) exactly as before.

## Testing

Add focused unit tests for the pure-logic cores (no network):

- `suno_sessions_test.go` — 30-min-gap boundary: clips 29 min apart = one
  session, 31 min apart = two; `--limit` keeps the most recent N.
- `suno_burn_test.go` — aggregation by tag/persona/model/hour; `--since` filter;
  estimated-credit math (10/generation).
- `suno_tree_test.go` — parent/child reconstruction, cycle guard, not-found.
- `suno_vibes_test.go` — save→get→use round-trip; `{topic}` template
  substitution; delete.
- `suno_budget_test.go` — `budgetCapExceeded` math at/over daily and monthly
  caps; no-cap = no-op.

Run `go build ./... && go vet ./... && go test ./...` to green.

## Ship-readiness surfaces (scope: full)

Update so the restored commands are documented and pass the publish gates:

- `SKILL.md` — add the 8 commands to the command reference + an example each.
- `README.md` — add to the command reference + recipes.
- `manifest.json`, `tools-manifest.json` — add the 8 command/tool entries.
- `internal/cli/agent_context.go` — ensure `agent-context` emits the restored
  commands (verify whether it auto-enumerates the cobra tree or needs explicit
  entries).
- Gates to green: `verify-skill`, `scorecard`, `dogfood`, plus
  `go build/vet/test`.

## Out of scope

- Publishing (`/printing-press-publish`) — a separate user-triggered turn.
- Reconciling `burn` vs `analytics` or `tree` vs `lineage` into one command —
  explicitly kept as coexisting per the decision above.

## Git workflow

The CLI source is mirrored from the publish-staging copy into the git repo at
`/Users/smacdonald/homegit/suno-cli`, where all implementation happens. A
baseline commit captures the imported PP 4.14.0 tree; the restore work lands on
a `feat/restore-dropped-commands` branch with one conventional commit per task.
When the branch is complete and green, the source is mirrored back to the
publish-staging copy so `/printing-press-publish` can ship it.

## Success criteria

1. `suno-pp-cli --help` lists all 8 restored commands alongside the new ones.
2. Each restored command reproduces the 2026-05-15 build's behavior/output.
3. `budget set daily N` then blocks the generation that would exceed N.
4. `go build/vet/test` green; new tests cover the restored logic.
5. `verify-skill`, `scorecard`, `dogfood` pass.
