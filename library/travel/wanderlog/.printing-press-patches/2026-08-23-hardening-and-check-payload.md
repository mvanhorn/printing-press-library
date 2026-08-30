# 2026-08-23 Hardening, dry-run guards, and `--check` payload

## Intent

Preserve the post-publish hardening pass across reprints:

- **ShareDB ids come from `crypto/rand`.** `randomWanderlogID` seeded a
  `math/rand` generator from `time.Now().UnixNano()` on every call, so two calls
  inside the same nanosecond tick minted the *same* 9-digit id. Those ids address
  rows inside a shared ShareDB document, so a collision corrupts a live plan.
- **`--dry-run` returns before any file read.** `plan block apply` and
  `plan raw op` read `--ops-file` from disk before the dry-run short-circuit;
  `plan block apply` also used `MarkFlagRequired("ops-file")`, which cobra
  enforces before `RunE` runs, so a `--dry-run` probe was unreachable. Both now
  short-circuit first and report `ops: 0` with an explicit warning. `plan fill`
  under global `--dry-run` now says plainly that no sections were replaced.
- **`plan inspect --check` prints an answer, not the plan.** `--check` returned
  the whole `sections` outline alongside the checks — larger than `plan outline`
  itself. It now prints a `planChecksReport` projection (plan scalars + checks),
  roughly 70x smaller; `--with-sections` restores the outline.
- **`cobra.NoArgs` on `plan inspect`.** `--check` has a `NoOptDefVal`, so
  `--check counts` parsed as a valueless `--check` plus an ignored positional and
  silently ran *every* check. `NoArgs` makes that a usage error instead of a
  different answer than the caller asked for. Docs now always write `--check=NAMES`.
- **`schema_version` 1 -> 2 in `.printing-press.json`** (see below).
- Truthful `Short` strings on the mutating `plan block`/`plan section`/`plan raw`
  commands, `--dry-run is a no-op` examples on the read-only commands, and
  `mcp:read-only` on `plan collaborators`.
- `which` index and `agent-context` command-mirror list rebuilt to cover all 34
  novel commands (they listed 3).
- `#nosec G304` justifications on the two operator-supplied / config-derived
  `os.ReadFile` paths, plus `filepath.Clean` on `--ops-file`.
- `pp:client-call` markers on the files the resource-path scanner reads.
- Placeholder cleanup: `exampletarget` -> `YOUR_TRIP_KEY` in `plan fill`.
- `workflow_verify.yaml` added: an anonymous, non-mutating read + rehearse-copy
  flow so `workflow-verify` exercises the primary journey without credentials.
- `README.md` rebuilt from `--help` output and given a `## Known Gaps` section;
  `SKILL.md` given a `## Gotchas` section; the duplicated warnings that section
  absorbs were removed from `references/` and `AGENTS.md`.

## `schema_version` hand-edit (on the record)

`.printing-press.json` `schema_version` was changed from `1` to `2` **by hand**.
This is deliberate and worth stating plainly:

- There is no migration command. Only `generate` and the full `publish` pipeline
  stamp the field, and `publish package` aborts on `schema_version 1` before it
  would rewrite it.
- The integer has exactly one reader: an equality test in the press's
  `publish.go`.
- Schema 2 requires no field schema 1 lacks. Across the 449 manifests in this
  library, the only key present in every schema-2 manifest and absent from some
  schema-1 ones is `novel_features`, which this manifest already carries.

## Touched Surface

- `internal/cli/plan_copy.go`: `randomWanderlogID` -> `crypto/rand`; `plan fill`
  dry-run warning.
- `internal/cli/plan_batch.go`: `plan block apply` dry-run guard, `--ops-file` no
  longer `MarkFlagRequired`, `filepath.Clean` + `#nosec G304` in
  `readJSON0OpsFile`, examples.
- `internal/cli/plan_edit_more.go`: `plan raw op` dry-run guard; `Short` strings.
- `internal/cli/plan_outline.go`: `planChecksReport` + `checksOnlyReport`,
  `--with-sections`, `cobra.NoArgs`, examples, `parsePlanInspectChecks` loop.
- `internal/cli/plan_outline_test.go`: `--check` omits/keeps `sections`.
- `internal/cli/plan_history.go`: `#nosec G304` on the journal read.
- `internal/cli/plan_edit.go`, `plan_collab_ext.go`, `plan_preview.go`,
  `plan_reservation.go`, `plan_fill.go`: `Short`/`Example`/annotation fixes.
- `internal/cli/which.go`: index rebuilt, 3 -> 34 entries.
- `internal/cli/root.go`: root Long highlights list.
- `internal/mcp/tools.go`: `command_mirror_capabilities` rebuilt, 3 -> 34.
- `internal/cli/lodging.go`: `pp:data-source` / `pp:client-call` markers.
- `README.md`, `SKILL.md`, `AGENTS.md`, `references/*.md`.
- `workflow_verify.yaml` (new).
- `.printing-press.json`: `schema_version`, `novel_features`,
  `novel_features_built`, `verify`, `scorecard`.

## Verification

- `go build ./... && go vet ./... && go test ./...` — all pass.
- `gofmt -l .` — clean.
- `cli-printing-press dogfood --live --level full`.
- `cli-printing-press publish validate`.
- `cli-printing-press shipcheck --no-fix` with live credentials.
