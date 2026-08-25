# Printing Press Retro: giustizia-amministrativa

Session type: `/printing-press emboss` on an already-published CLI, followed by a blocked `/printing-press-publish`.
Binary: cli-printing-press 4.30.1. CLI last generated with 4.24.0 (run `20260621-094103`).

## Session Stats
- API: giustizia-amministrativa (public library, `library/productivity/giustizia-amministrativa`, release `2026.7.2`)
- Spec source: internal YAML (`spec_format: internal`, browser-sniffed origin)
- Scorecard: 85/100 (A) with `--spec ""`; 75/100 (B) with `--spec spec.yaml` — see F2
- Verify pass rate: 100% (23/23, 0 critical), up from 96% (24/25, 1 critical)
- Fix loops: 0 (no shipcheck loop needed)
- Manual code edits: 5 files (1 deletion, 1 registration comment, 3 docs)
- Features built from scratch: 0 (emboss cycle, not a print)
- Publish outcome: **blocked at `publish validate`** — see F1

## Findings

### 1. A three-field additive manifest schema bump makes every pre-4.30 CLI unpublishable, with no migration path (Assumption mismatch)

- **What happened:** `publish validate` fails with `manifest — schema_version must be 2 (found 1)`. The CLI is already in the public library and works; the only change this session was removing a broken command and fixing an install path. The manifest was written by press 4.24.0.
- **Scorer correct?** Partially. The floor is a real contract, but the delta between schema 1 and 2 is three additive fields (`auth_env_vars`, `auth_env_var_specs`, `spec_path`) plus dropping `mcp_public_tool_count`. None of them carry provenance that needs re-deriving from a fresh run.
- **Root cause:** Binary (`publish validate` / `publish package`). The main SKILL's documented remedy is *"re-print or re-package with current Printing Press metadata"*, but `publish package` runs `validate` first and aborts, so the re-package half of that remedy is unreachable. There is no `schema migrate` command; `schema` only exposes `phase5-marker`, `phase5-skip`, and `traffic-analysis`.
- **Cross-API check:** Yes, immediately. Eight of nine CLIs in this local library are schema 1 and are all in the same state today.
- **Frequency:** every CLI generated before 4.30.
- **Cross-API evidence (Step B):** `sec-edgar` (schema 1, press 4.24.0), `verto` (schema 1, 4.24.0), `aol-puglia` (schema 1, 4.24.0), plus `anac-pl` (1, 4.17.0), `eutrack` (1, 4.17.0), `elezioni-sicilia` (1, 4.18.0), `ars-sicilia` (1, 4.18.0). Only `acquistinretepa` — generated 2026-08-04 under 4.30.1 — is schema 2. The published entry for this CLI is *also* `schema_version: 1` (verified via `gh api` against `library/productivity/giustizia-amministrativa/.printing-press.json`), so the library currently hosts schema-1 entries; the floor is enforced by the local binary, not by library CI.
- **Counter-check (Step C):** Would auto-migration hurt a CLI that does not have these fields? No — the fields are additive and derivable from the tree already present (`auth_env_vars` is empty for `auth.type: none`; `spec_path` is the bundled spec). The guard is that migration must only fill additive, derivable fields and must never synthesise provenance (`run_id`, `printer`, `printing_press_version`), which is exactly what the floor exists to protect.
- **Fallback if the Printing Press doesn't fix it:** A full reprint under 4.30.1 for every pre-4.30 CLI. For this one that means reconciling 26 hand-authored files (14 in `internal/cli`, 11 in `internal/gaclient`, 1 in `internal/store`), 24 patch-ledger entries, and `regen-merge`'s `TEMPLATED-WITH-ADDITIONS` review halt — to add three fields. The realistic outcome is that small fixes to published CLIs stop being shipped at all.
- **Worth a Printing Press fix?** Yes. The asymmetry between the size of the schema delta and the size of the sanctioned remedy is the whole finding.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Let `publish package` migrate additive, derivable manifest fields and bump `schema_version`, then validate the migrated manifest — i.e. make the documented "re-package" remedy actually reachable. Fail loudly only when a required field cannot be derived without a fresh run. Alternatively add an explicit `manifest migrate` command and have `validate` name it in the error text instead of implying a reprint.
- **Test:** Positive — `publish package` on a schema-1 `auth.type: none` CLI produces a schema-2 manifest with `auth_env_vars: []` and a populated `spec_path`, and validate passes. Negative — a manifest missing `run_id` or `printer` still fails, with the message pointing at a reprint rather than a migration.
- **Evidence:** `publish validate --dir . --json` → `{"name":"manifest","passed":false,"error":"schema_version must be 2 (found 1)"}`; `publish package --target …` reproduces the same failure before writing anything.
- **Related prior retros:** `ars-sicilia` retro (`20260628-182011-retro-ars-sicilia-pp-cli.md`) — `extends`. It raised the same *class* of problem for a different artifact: `.printing-press-patches` schema 1 (single file) vs schema 2 (per-patch directory). Two schema bumps have now stranded older CLIs; the shared gap is that neither ships a migration path.

### 2. `scorecard --spec` scores an internal-format spec as if it were OpenAPI, giving a false `path_validity 0/10` (Scorer bug)

- **What happened:** `scorecard --dir . --spec spec.yaml` reports `path_validity 0/10` and drops the grade from A (85) to B (75). Without `--spec`, `path_validity` is correctly N/A. The spec is internal YAML, not OpenAPI — the flag's own help says *"Path to OpenAPI spec JSON for semantic validation"*.
- **Scorer correct?** No. `dogfood` on the same tree and the same file reports `Path Validity: 0/0 valid (SKIP) — internal-yaml spec: paths validated at parse time`. Two tools read the same file; one knows the format, the other does not.
- **Root cause:** Scorer. `scorecard` does not detect spec format before scoring path validity, and scores an unparseable-as-OpenAPI document as zero valid paths rather than not-applicable.
- **Cross-API check:** Yes. Any CLI whose manifest says `spec_format: internal`.
- **Frequency:** every internal-spec CLI.
- **Cross-API evidence (Step B):** `sec-edgar`, `verto`, `anac-pl` all carry `spec_format: internal` in their manifests, as do `eutrack`, `elezioni-sicilia`, `aol-puglia`, and `acquistinretepa` — eight of the nine CLIs in this library. Running `scorecard --spec` on any of them produces the same false zero.
- **Counter-check (Step C):** Returning N/A for a non-OpenAPI spec cannot hurt OpenAPI CLIs; their detection path is unchanged. No guard needed beyond the format check itself.
- **Fallback if the Printing Press doesn't fix it:** The operator has to know which flag to omit. This session spent real effort chasing `path_validity 0/10` as if it were a defect, including an experiment that removed a command to test a hypothesis, before the flag's help text revealed it was a measurement artifact. A 10-point grade swing that depends on a flag rather than on the code teaches operators to distrust the score.
- **Worth a Printing Press fix?** Yes — the detection already exists in `dogfood` and just needs to be shared.
- **Inherent or fixable:** Fixable.
- **Durable fix:** Have `scorecard` detect spec format the way `dogfood` already does; when the spec is internal YAML, mark `path_validity` as N/A and exclude it from the denominator instead of scoring it 0.
- **Test:** Positive — `scorecard --spec <internal-yaml>` reports `path_validity: N/A` and the same total as omitting `--spec`. Negative — `scorecard --spec <real OpenAPI>` still scores path validity as it does today.
- **Evidence:** Same tree, same file: `scorecard --dir . --json` → total 85, `path_validity` unscored; `scorecard --dir . --spec spec.yaml --json` → total 75, `path_validity: 0`; `dogfood --dir .` → `Path Validity: 0/0 valid (SKIP) — internal-yaml spec`.
- **Related prior retros:** `acquistinretepa` retro finding 3 — `extends`. That one is also a scorer accuracy bug that produced a false failure (live-check scoring store-backed features against a cold mirror and discarding stderr). Different mechanism, same theme: the scorer lacks context another tool already has.

### 3. `dogfood --research-dir` silently rewrites `internal/mcp/tools.go` and reverts later fixes (Bug)

- **What happened:** Running `dogfood --dir . --research-dir .manuscripts/<run>` rewrote the `command_mirror_capabilities` block in `handleContext` from the June `research.json`, reverting three fixes made after generation: the `tool` (MCP name, `watch_run`) + `cli_command` (shell spelling, `watch run`) key pair collapsed back to a single `command` key, `get` lost *"Passa l'ECLI in `id`"*, and `stats` lost its `sede-sweep` clause. The key pair is commit `6578e7a` — an agent that reads `watch run` cannot call it, because the tool is named `watch_run`. It was caught only by reading `git status` before committing.
- **Scorer correct?** No. The sync is documented behaviour, but a diagnostic command mutating tracked source and discarding differing committed content without naming what it discarded is not.
- **Root cause:** Scorer (`dogfood`). Two problems compound. First, the sync prints `dogfood: synced ... from novel_features_built` but does not say it *replaced* differing content, so the revert is invisible in the command output. Second, `research.json`'s `novel_features[]` has a single `command` field where the MCP command mirror needs two distinct values (MCP tool name and CLI spelling), so the SKILL's documented remedy — *"fix the source text in research.json"* — cannot express the fix. There is no correct place to put it.
- **Cross-API check:** Yes. Seven CLIs in this library carry a `command_mirror_capabilities` block generated from `novel_features` and are exposed to the same silent revert.
- **Frequency:** every CLI with novel features whose `tools.go` has been hand-corrected.
- **Cross-API evidence (Step B):** `ars-sicilia` (8 novel features, block present), `sec-edgar` (5, present), `verto` (5, present); also `acquistinretepa` (6), `anac-pl` (3), `elezioni-sicilia` (3). Each would lose any post-generation correction to that block on the next `dogfood --research-dir`.
- **Counter-check (Step C):** Warning about reverted content, or requiring an explicit flag to sync, cannot hurt CLIs that never hand-edited the block — for them the sync is a no-op and the warning never fires.
- **Fallback if the Printing Press doesn't fix it:** The operator must diff after every dogfood run. This session only caught it by habit. On a CLI whose `tools.go` fix is older than the operator's memory, the revert ships silently and the MCP surface goes back to advertising names an agent cannot call.
- **Worth a Printing Press fix?** Yes. Silent loss of committed work by a read-shaped diagnostic command is the most severe class of finding in this session, even though the blast radius is narrower than F1's.
- **Inherent or fixable:** Fixable. The sync itself is useful; its silence is not.
- **Durable fix:** When the sync would replace existing content that differs from what it is about to write, print the file and a summary of what changed, and gate the write behind an explicit flag (or make sync opt-in rather than a side effect of `--research-dir`). Separately, give `novel_features[]` a way to carry both the MCP tool name and the CLI command spelling so the documented "fix it in research.json" remedy exists for this field.
- **Test:** Positive — running `dogfood --research-dir` on a CLI whose `tools.go` block differs from `research.json` reports the difference and leaves the file untouched without the flag. Negative — a CLI whose block already matches sees no warning and no diff, as today.
- **Evidence:** `git diff internal/mcp/tools.go` after the run showed 7 lines replaced, each losing `"tool"`/`"cli_command"` for a single `"command"`. Reverted with `git checkout --`; hazard recorded in the CLI's `AGENTS.md`.
- **Related prior retros:** None matching. `acquistinretepa` finding 3 is a scorer-accuracy bug, not a source-mutation bug.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F1 | Manifest schema bump strands every pre-4.30 CLI with no migration path | binary (publish) | every pre-4.30 CLI (8/9 here) | low — the sanctioned remedy is a full reprint, so fixes stop shipping | medium | migrate only additive derivable fields; never synthesise `run_id`/`printer`/`printing_press_version` |
| F3 | `dogfood --research-dir` silently reverts hand-fixes in `tools.go` | scorer (dogfood) | every novel-feature CLI with a corrected mirror block (7 here) | low — requires the operator to diff after every run | small | warn/opt-in only; no-op CLIs see no change |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| F2 | `scorecard --spec` scores an internal spec as OpenAPI → false `path_validity 0/10` | scorer (scorecard) | every internal-spec CLI (8/9 here) | medium — operator must know to omit the flag | small | detection already exists in `dogfood`; OpenAPI path unchanged |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|------------------------|
| C4 | `scorecard --live-check` default probe timeout of 10s fails network-bound novel features (`massime`, `appeal-chain` pass at 60s; `Insight` drops 10→7) | Step B: only one API with evidence. Adjacent to `acquistinretepa` finding 3, already filed against the same component — better folded into that work than raised separately. |
| C5 | `--live-check` relevance heuristic treats a positional argument as a query token (`watch run appalti-rm` "fails" because the watch *name* is absent from the results) | Step B: single observation, no second API. Same component as C4 and `acquistinretepa` finding 3. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| C6 | Generator emits the dead `resolvePaginatedRead` → `paginatedGet` → `extractPaginatedItems` subtree for a spec with no pagination; `dogfood` flags only the leaf, not the unreachable root | unproven-one-off — `dead_code` already scores 5/5, so the cost is a WARN worth zero points, and removing generic pagination plumbing risks CLIs that do use it |
| C7 | `verify`/`emboss --dir .` compile a binary named after the directory rather than the CLI, leaving it untracked | printed-CLI — one `.gitignore` line fixes it locally; no cross-CLI harm beyond tidiness |
| C8 | `emboss` prints `scorecard error: writing scorecard.md: mkdir : no such file or directory` on every run and continues with a partial baseline (reproduced on `sec-edgar`) | *see note below* |

**Note on C8.** This one was reclassified rather than dropped outright: it reproduces on a second CLI (`sec-edgar`, identical message) so it is clearly not API-dependent, and it looks like an empty output-directory variable in `emboss`. But its only observed effect is a noisy line plus a "partial" baseline whose numbers were in fact complete and correct in both runs. Without evidence that the partial baseline ever loses data, the case-against ("cosmetic; the delta was accurate") is at least as strong as the case-for, and Step G's default direction is don't-file. Recorded here so the next retro that sees it can file it with the missing evidence.

## Anti-patterns

- **A diagnostic command that writes to tracked source.** `dogfood` reads as read-only from its name and its role in the pipeline. Its sync side effect is documented in the SKILL but invisible at the call site.
- **A score that depends on which flag you passed.** A 10-point, one-grade swing between `--spec ""` and `--spec spec.yaml` on identical code trains operators to pick the invocation that flatters the number.
- **A schema floor whose only escape hatch is a full regeneration.** When the remedy costs more than the defect, the remedy does not get used; the fix simply does not ship.
- **Removing a broken command lowers the score.** Deleting `import` — which could only return 404/405 — cost one point of `vision`, because `vision` counts commands. Worth noting, not worth filing: the incentive is mild and the dimension is doing roughly the right thing overall.

## What the Printing Press Got Right

- **`verify` found the one real code defect.** `resource-path:import` pointed straight at a command that could only fail, and the message named the reason precisely.
- **`verify-skill`'s `canonical-sections` caught stale published-install text** that no human had noticed in two weeks of a live listing — and printed the exact expected text, so the fix was a paste rather than a reconstruction.
- **`dogfood` distinguishes internal-YAML specs from OpenAPI** and says so in plain words. That message is what solved F2's diagnosis; the fix for F2 is just to share it with `scorecard`.
- **`dogfood --research-dir` confirmed 7/7 novel features survived** 16+ hand fixes since generation — real reassurance that the CLI still does what it claims.
- **`publish validate` refused to proceed and named exactly which checks failed,** rather than opening a partial PR. The blocker in F1 is about the remedy being unreachable, not about the gate firing.

## Filing outcome (2026-08-08)

Dedup scan against open issues on `mvanhorn/cli-printing-press` found two of the three findings already tracked. Filed as two comments and one new issue.

| Finding | Outcome | Where |
|---|---|---|
| F1 — manifest schema bump strands pre-4.30 CLIs | Comment (same issue) | [#3425](https://github.com/mvanhorn/cli-printing-press/issues/3425#issuecomment-5226990011) |
| F2 — `scorecard --spec` false `path_validity 0/10` | Comment (same issue) | [#3459](https://github.com/mvanhorn/cli-printing-press/issues/3459#issuecomment-5226990075) |
| F3 — `dogfood --research-dir` silently reverts `tools.go` | New issue, P1 | [#4037](https://github.com/mvanhorn/cli-printing-press/issues/4037) |

The F1 comment carries a correction, not just corroboration: #3425 asserts that a v1 manifest already satisfies the v2 field contract and only the integer is stale. That is not true here — this manifest is missing `auth_env_vars`, `auth_env_var_specs`, and `spec_path` — so a pure integer restamp would produce a manifest claiming v2 while missing v2 fields. The proposed fix still holds, but it has to derive the additive fields.

The F2 comment surfaces a possible root-cause ambiguity: #3459 attributes the false 0 to `response_format: html`, while `dogfood` on the same file keys on `spec_format: internal`. This CLI is both, so the two triggers could not be separated. The two fixes have different blast radii, so the trigger is worth confirming before implementing.

Artifacts were not uploaded to catbox.moe — the evidence is inline in the issue bodies, and the retro doc lives in manuscript proofs.

## Correction to F1 (2026-08-08, after filing)

The claim in F1 that the v1 manifest is **missing** `auth_env_vars`, `auth_env_var_specs`, and `spec_path`, and that a pure integer restamp would therefore be insufficient, is **wrong**. It was inferred from a key-set diff against a 4.30.1-generated CLI and never tested before being written.

What was measured afterwards: `lock promote` under 4.30.1 restamps `schema_version` 1 → 2 and `printing_press_version` 4.24.0 → 4.30.1 with no field added and none lost — those three fields are *still* absent — and `publish validate`'s `manifest` check then passes. They are optional or only populated when applicable. #3425's original proposal is sufficient as written.

Corrected publicly at https://github.com/mvanhorn/cli-printing-press/issues/3425#issuecomment-5227063771.

Two things learned while unblocking, which do hold:

- `lock promote` runs its own **phase5 gate before** the manifest write, so the ordering is fresh live-dogfood marker → promote → validate, not promote → validate.
- The phase5 marker exists in **five copies across three scopes**; promote reads the one under `.runstate/<scope>/runs/<run-id>/proofs/`. Writing only the CLI-local `.manuscripts/` copy is not enough.
