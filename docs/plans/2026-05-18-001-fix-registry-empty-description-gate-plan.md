---
type: fix
status: active
created: 2026-05-18
plan_id: 2026-05-18-001
---

# fix: harden registry parsing and gate empty descriptions

## Summary

`registry.json`'s `lawhub` entry shipped with `description: ""`. The npm installer (`@mvanhorn/printing-press@0.1.3`) throws `registry entry missing string field: description` on the first malformed entry it sees, which aborts the entire registry parse and makes every `printing-press install/search/list/update` invocation fail for every user — regardless of what CLI they were targeting.

Two structural problems combined to produce this:

1. The registry generator (`tools/generate-registry/main.go`) picks descriptions from `prior registry.json → .goreleaser.yaml brews → empty`. It never reads `.printing-press.json`'s `description` field even though every modern printed CLI populates it. `lawhub` has a populated `.printing-press.json` description (130 chars) but an empty `.goreleaser.yaml` brews block and no prior curated registry value, so it landed empty.
2. There is no PR-time validation that the registry the generator would produce is internally valid. A new CLI added without a goreleaser brews description and without a curated registry value will silently ship `description: ""` and break the installer the same way.

This plan adds the missing description fallback, backfills the two existing CLIs whose descriptions live only in the curated registry (`tiktok-shop`, `agent-capture`), adds a strict PR-time validation gate, and hardens the npm installer to skip malformed entries with a warning rather than aborting the whole parse.

The PR does **not** commit the regenerated `registry.json`. The post-merge `generate-registry.yml` workflow handles regeneration on merge per the existing generated-artifact convention.

## Target repo

`mvanhorn/printing-press-library`. The plan file itself lives at `docs/plans/2026-05-18-001-fix-registry-empty-description-gate-plan.md` and all file paths below are repo-relative.

## Requirements

- R1. Every CLI in the library today must produce a non-empty `description` in the registry the generator would emit from sources alone (without falling back to prior registry values).
- R2. Future CLIs that would produce an empty description must be blocked at PR time with a clear, slug-named error message naming the missing source.
- R3. The npm installer must survive a malformed registry entry without aborting the entire parse; bad entries are skipped with a warning, valid entries continue to load.
- R4. The fix must not regress curated registry description copy. Hand-curated values that are richer than source descriptions remain authoritative; the new source fallback only fires when no curated value exists.
- R5. The PR must not commit regenerated `registry.json` or `README.md`. The post-merge `generate-registry.yml` workflow regenerates them on merge.

## Scope Boundaries

### In scope

- Generator description-resolution change in `tools/generate-registry/main.go`.
- Strict-validation mode for the generator (new `--validate` flag).
- `.printing-press.json` description backfill for `tiktok-shop` and `agent-capture` (the only two CLIs whose curated description has no source-side counterpart today).
- New step in `.github/workflows/verify-library-conventions.yml` that runs the validator on PRs.
- Lenient parsing in `npm/src/registry.ts` (skip + warn instead of throw on per-entry errors).
- Test updates in `tools/generate-registry/main_test.go` and `npm/tests/` covering the new behaviors.
- npm patch version bump so the resilience fix reaches users via the existing auto-publish workflow.

### Deferred to Follow-Up Work

- Migrating all curated registry descriptions back into `.printing-press.json` so the registry becomes fully derivable from sources with no curated overlay. The current curated-preference logic stays; only the two CLIs without any source description get backfilled here.
- Validating optional MCP block fields (e.g., `mcp.tool_count > 0`) beyond what the npm installer requires. This plan validates exactly what the installer's `parseRegistry` requires; richer invariants can land separately.
- Hardening other generator outputs (`README.md` sentinel regions, `cli-skills/pp-*/SKILL.md`) against similar empty-string footguns.

### Outside this product's identity

- Restructuring how descriptions are sourced (e.g., a single `description.md` per CLI). The three-layer fallback (curated > goreleaser > pp manifest) reflects real history and stays.

## Key Technical Decisions

### KD1. Add `.printing-press.json` description as a third fallback, not the top of the order

The existing source comment in `registryDescription` documents the curated-first preference deliberately: 29/42 entries' curated descriptions don't match the `.goreleaser.yaml` brews description, and that's the curated-copy-as-source-of-truth design. Hoisting `.printing-press.json` to the top would clobber curated headlines like Suno's "Every Suno feature, plus a local SQLite library...". Threading it in as the third fallback (after curated and goreleaser, before empty) fixes the lawhub-shape without regressing curated copy. (See: comments above `registryDescription` in `tools/generate-registry/main.go`.)

### KD2. Validate strictly: every entry's description must resolve from sources

The validator ignores the prior `registry.json` curated value when determining whether validation passes. Otherwise a future CLI could silently rely on someone hand-editing registry.json after the fact — exactly the path that produced lawhub. Validating sources-only catches lawhub-shape at PR time even when no curated value exists yet.

Two CLIs today (`tiktok-shop`, `agent-capture`) have descriptions only in the curated registry — U3 backfills them so strict validation passes everywhere.

### KD3. PR-time validation lives in `verify-library-conventions.yml`

This workflow already runs on `library/**` paths, already overlays verifier scripts from base for fork-PR safety, and already represents the canonical library-invariant gate. Adding a `Validate registry source descriptions` step there reuses the existing trigger, checkout, and base-overlay scaffolding. Creating a new workflow file would duplicate all of that for one step.

### KD4. Don't commit regenerated `registry.json` in the PR

`verify-library-conventions.yml` explicitly rejects PRs that modify generated artifacts ("Fail on changes to generated artifacts" step). Whitelisting registry.json for this one PR would fight that convention. The accepted flow is: PR fixes the source (generator code + backfilled `.printing-press.json` files) → post-merge `generate-registry.yml` regenerates registry.json on merge → `lawhub` description becomes populated automatically.

Trade-off: users hitting the broken npm installer between PR open and merge stay broken. Mitigated by U5's lenient npm parsing, which lands in the same PR and (once the auto-publish ships a patched npm version) fixes the user-visible failure even before the library-side regen runs.

### KD5. npm parser: skip + warn, don't fail-open silently

`parseRegistryEntry` returns `null` instead of throwing when a per-entry `requiredString` would fail; `parseRegistry` filters nulls. The warn goes to `stderr` so machine-readable `--json` output stays clean, and identifies the offending entry by name (or by index when name itself is missing). This degrades gracefully without hiding the underlying problem: users still see "skipped 1 malformed entry: lawhub" in their terminal.

## System-Wide Impact

- **`registry.json` consumers** (npm installer, `printing-press-library` plugin, any downstream tool that reads `https://raw.githubusercontent.com/.../registry.json`): unblocked once npm package republishes with U5. Library-side fix unblocks them after merge + post-merge regen.
- **PR authors adding new CLIs**: A new CI step rejects PRs whose new CLI would produce an empty registry description. Error message tells them exactly which source file to populate (`.printing-press.json` description or `.goreleaser.yaml` brews description).
- **Post-merge `generate-registry.yml`**: No workflow change. Picks up the new fallback automatically when the generator code lands.

## Implementation Units

### U1. Thread `.printing-press.json` description into the registry-description fallback chain

- **Goal:** Add `.printing-press.json`'s `description` as a third fallback in `registryDescription` so lawhub-shape CLIs (populated pp manifest, empty goreleaser brews, no prior curated value) resolve correctly without operator intervention.
- **Requirements:** R1, R4.
- **Dependencies:** none.
- **Files:**
  - `tools/generate-registry/main.go` (modify `printingPressManifest`, `buildEntry`, `registryDescription`)
  - `tools/generate-registry/main_test.go` (add cases for the new fallback)
- **Approach:**
  - Add a `Description string \`json:"description"\`` field to `printingPressManifest`. The `.printing-press.json` schema already populates this; adding it to the struct lets the existing `json.Unmarshal` capture it.
  - Change `registryDescription`'s signature to accept a third argument (`ppDescription string`). New order: curated prior > goreleaser brews > pp manifest description > empty.
  - `buildEntry` passes `pp.Description` through to `registryDescription`. No other caller of `registryDescription` exists.
  - The bare-markdown-heading exception (`isBareMarkdownHeading`) continues to apply only to the prior curated value, not to the new sources. Goreleaser brews and pp manifest descriptions are author-written one-liners and don't have the legacy-bug shape.
- **Patterns to follow:** the existing fallback chain shape; comment block at line 311-317 of `tools/generate-registry/main.go`. Mirror the comment style to document the new third fallback.
- **Test scenarios:**
  - Happy path: curated value present, goreleaser empty, pp empty → returns curated (existing behavior preserved).
  - Happy path: curated empty, goreleaser populated → returns goreleaser (existing behavior preserved).
  - New behavior: curated empty, goreleaser empty, pp description populated → returns pp description.
  - New behavior: curated empty, goreleaser empty, pp empty → returns "" (validation in U2 catches this case).
  - Regression: curated is a bare markdown heading like `"# Introduction"`, pp populated → returns pp (the bare-heading exception still fires for the curated tier; new sources backfill it).
- **Verification:** `go test ./tools/generate-registry/...` passes. Manual smoke: `go run ./tools/generate-registry --print | jq '.entries[] | select(.name=="lawhub") | .description'` returns the lawhub `.printing-press.json` description verbatim.

### U2. Add `--validate` mode to the generator

- **Goal:** Hard-fail with field-named errors when any entry would have an empty required string after fallback resolution, considering only source-of-truth files (not prior `registry.json`).
- **Requirements:** R2.
- **Dependencies:** U1 (validation calls the same fallback resolution).
- **Files:**
  - `tools/generate-registry/main.go` (add `validate` flag, new `validateEntries` function, wire into `main`)
  - `tools/generate-registry/main_test.go` (validation cases)
- **Approach:**
  - Add a `validate := flag.Bool("validate", false, "...")` flag.
  - When `--validate`: call `buildEntries(libraryDir, map[string]RegistryEntry{})` — empty existing map so curated descriptions don't satisfy validation. Then call `validateEntries(entries)`.
  - `validateEntries` walks the entries and collects errors. For each entry, check:
    - `name`, `category`, `api`, `path` non-empty (these come from disk structure and the pp manifest; failing here means a malformed source file).
    - `description` non-empty.
    - If `mcp` is set: `mcp.binary` non-empty, `mcp.transports` non-empty array, `mcp.auth_type` non-empty (these are exactly the fields `npm/src/registry.ts`'s `parseRegistryEntry` requires).
  - Error messages name the slug and the field, e.g., `lawhub: description is empty (sources checked: .printing-press.json description, .goreleaser.yaml brews description)`.
  - Exit 0 when all entries valid; exit 2 with the joined error report when any entry fails. Stderr is the error channel.
  - `--validate` is mutually exclusive with `--check` and `--print` (existing modes are already mutually exclusive via the if/else cascade in `main`).
- **Patterns to follow:** existing `--check` mode's "drift detected" error reporting style (line 185-189). Reuse `log.Fatalf` only for unexpected failures; validation errors are expected output and should write to stderr explicitly + `os.Exit(2)`.
- **Test scenarios:**
  - Validation passes when every entry has a description in some source.
  - Validation fails when one entry has no source description; error names the slug and field.
  - Validation fails when multiple entries fail; all failures reported, not just the first.
  - Validation ignores curated prior `registry.json` values — pass a fixture where the curated value exists but the source files don't and confirm validation fails.
  - Validation passes when `mcp` block is absent (not all CLIs ship MCP).
  - Validation fails when `mcp` block is present but `mcp.binary` empty.
- **Verification:** `go test ./tools/generate-registry/...` passes. Manual smoke: `go run ./tools/generate-registry --validate` against the PR's tree (after U3 backfills land) exits 0; reverting one of the backfills produces a clear `tiktok-shop: description is empty` error.

### U3. Backfill `description` in `.printing-press.json` for tiktok-shop and agent-capture

- **Goal:** Restore source-reproducibility for the two CLIs whose descriptions exist only in the curated `registry.json` today. Without this, the U2 validator fails on the current tree.
- **Requirements:** R1, R4.
- **Dependencies:** U1 (the new fallback is what makes the backfilled value flow through to the regenerated registry).
- **Files:**
  - `library/commerce/tiktok-shop/.printing-press.json` (add or set `description`)
  - `library/agent-tools/agent-capture/.printing-press.json` (add or set `description`)
- **Approach:**
  - Read the current curated value from `registry.json` for each slug. Copy it verbatim into the `description` field of the respective `.printing-press.json`.
    - `tiktok-shop`: `"Safe v1 TikTok Shop Seller API CLI/MCP for auth readiness, token exchange and read endpoints; pre-flight checks gate live writes by default."`
    - `agent-capture`: `"Record, screenshot, and convert macOS windows and screens for AI agent evidence"` (verbatim from registry).
  - These are the only two CLIs where this is needed today; the audit in research found no other CLIs with `pp=empty grls=empty reg=ok`.
- **Patterns to follow:** Existing `.printing-press.json` files where `description` is populated. Field order can match whatever the file already uses; no schema-shape change.
- **Test scenarios:** none — pure data files. The U2 validator running against the modified tree is the integration test (covered there).
- **Verification:** `jq -r .description library/commerce/tiktok-shop/.printing-press.json` and `... agent-tools/agent-capture/.printing-press.json` return the expected strings. `go run ./tools/generate-registry --validate` exits 0 after U1 + U2 + U3 are in place.

### U4. Add PR-time validation step to `verify-library-conventions.yml`

- **Goal:** Reject any PR whose CLI source files would produce an empty required registry field. Catches lawhub-shape at the source.
- **Requirements:** R2.
- **Dependencies:** U2 (the workflow invokes the validator the generator now supports).
- **Files:**
  - `.github/workflows/verify-library-conventions.yml` (add `Validate registry source descriptions` step and `setup-go` action)
- **Approach:**
  - Add `actions/setup-go@v6` with `go-version: '1.26.3'` (matching the post-merge regen workflow) to the `verify` job's step list.
  - Add a new step after the existing `go.mod module path matches directory` step:

    ```yaml
    - name: Validate registry source descriptions
      run: |
        set -euo pipefail
        go run ./tools/generate-registry --validate
    ```

  - Add `'tools/generate-registry/**'` to the workflow's `paths:` trigger so changes to the validator itself also re-run this check.
- **Patterns to follow:** the existing `setup-python` + step shape in the same workflow. Path-filter pattern in the workflow `on.pull_request.paths` list.
- **Test scenarios:** none added in this unit — the validator's own tests cover its behavior. Manual verification of the workflow happens on PR open.
- **Verification:** Open the PR; the new step appears in the GitHub Actions UI and exits 0 against the PR's tree. Push a deliberate breakage (delete one of the backfilled descriptions) in a draft branch and confirm the step exits non-zero with the expected slug-named error before reverting.

### U5. Lenient registry parsing in the npm installer

- **Goal:** Make the npm installer survive a malformed registry entry. Skip the bad entry with a stderr warning, return the valid remainder so `install`/`search`/`list`/`update` continue to work.
- **Requirements:** R3.
- **Dependencies:** none — this is independent defensive hardening on the npm side.
- **Files:**
  - `npm/src/registry.ts` (modify `parseRegistry` and `parseRegistryEntry`)
  - `npm/tests/registry.test.ts` (or whichever existing test file covers registry parsing — confirm path during execution)
- **Approach:**
  - `parseRegistryEntry` continues to validate via `requiredString` etc., but catches its own thrown errors at the entry boundary. Returns `RegistryEntry | null`. On null, write a single-line warning to `process.stderr`: `[printing-press] skipping malformed registry entry: <name or "(unnamed)">: <error message>`.
  - `parseRegistry` calls `parseRegistryEntry` per element and filters nulls: `entries: value.entries.map(parseRegistryEntry).filter((e): e is RegistryEntry => e !== null)`.
  - The `schema_version !== 2` check at the registry level remains a thrown error — a wrong-schema registry is unrecoverable, not a per-entry typo.
  - Surface the count of skipped entries: after parsing, if any were skipped, write `[printing-press] skipped N malformed registry entries; install/search may be missing items.` to stderr. Helps users notice the issue without halting them.
- **Patterns to follow:** existing TypeScript style in `npm/src/registry.ts`. The `isRecord`/`requiredString` helpers stay as-is; only the entry-level exception handling changes.
- **Test scenarios:**
  - Happy path: valid registry parses with N entries; no warnings emitted.
  - Mixed registry: one entry with `description: ""` → parse returns N-1 entries, stderr captured contains the slug and "description" in the warning.
  - Multiple bad entries: all valid ones returned; one summary line at the end naming the skipped count.
  - Unnamed bad entry: an entry missing both `name` and the field that would error → warning falls back to `(unnamed at index <i>)`.
  - Registry-level failure: `schema_version: 1` still throws (not a per-entry concern); test asserts the existing error path is preserved.
  - `parseRegistry` no longer rejects when one of N entries is invalid — regression coverage for the lawhub failure.
- **Verification:** `npm test` in `npm/` passes. Manual smoke: `node npm/bin/printing-press.mjs search suno` against a copy of the current broken registry.json finds suno and prints the skip warning for lawhub.

### U6. Bump npm patch version

- **Goal:** Trigger the auto-publish workflow so the resilience fix in U5 reaches users via the existing release flow.
- **Requirements:** R3.
- **Dependencies:** U5.
- **Files:**
  - `npm/package.json` (`version` field)
  - `npm/CHANGELOG.md` (add an entry for this version)
- **Approach:**
  - Bump `version` from `0.1.3` to `0.1.4`. Patch bump is correct: behavior change is a bug fix, public surface unchanged.
  - Add a `CHANGELOG.md` entry under a `## 0.1.4` heading: brief description of the parser hardening, link to this plan or to the PR.
  - Do not touch `npm/package-lock.json` directly; `npm install` regenerates it deterministically.
- **Patterns to follow:** existing CHANGELOG entries in `npm/CHANGELOG.md`.
- **Test scenarios:** none — pure version metadata.
- **Verification:** After PR opens, the `auto-tag-npm.yml` and `npm-publish.yml` workflows pick up the version bump on merge and publish `@mvanhorn/printing-press@0.1.4` to npm. (No verification possible until merge; this is the documented release flow.)

## Risks

- **Risk:** The U5 lenient parser could mask legitimate registry corruption (e.g., a botched generator run produces all-empty entries; users get a near-empty install list without an obvious error).
  - **Mitigation:** Per-skip stderr warning + final summary line. The library side's U4 PR gate catches the root cause at PR time before it reaches users. Users see the missing items in `search`/`list` output, not silent absence.

- **Risk:** The U2 validator running with an empty existing-entries map could double-count work if `buildEntries` is expensive on the full library tree.
  - **Mitigation:** `buildEntries` is filesystem-bound and runs in < 1s on the 134-CLI tree today. Validation cost is negligible.

- **Risk:** A future printer adds a new CLI before this PR merges; their PR fails the new check despite being correct in isolation.
  - **Mitigation:** This is the desired behavior. If their CLI is missing a description, fail loud. If their CLI has one, the validator passes.

- **Risk:** U3 backfill conflicts with a parallel PR editing the same `.printing-press.json` files for unrelated reasons.
  - **Mitigation:** Low likelihood (the two files are rarely-touched). Standard rebase resolves any conflict; the backfilled value is one field.

## Verification

- All new and existing tests pass in `tools/generate-registry/` and `npm/`.
- `go run ./tools/generate-registry --validate` exits 0 against the PR's tree after U1–U4.
- `go run ./tools/generate-registry --check` against the PR's tree exits non-zero (drift expected — registry.json is intentionally stale) and that's fine: the PR does NOT commit the regenerated artifact; the post-merge workflow handles regen.
- The verify-library-conventions workflow on the PR shows the new "Validate registry source descriptions" step passing.
- After merge, the post-merge `generate-registry.yml` workflow regenerates `registry.json` and `README.md`; the resulting `registry.json` shows `lawhub` with a populated description.
- `npx -y -p @mvanhorn/printing-press printing-press install suno` succeeds against the post-publish 0.1.4 package, even before the library-side regen runs (because U5 makes parsing resilient).

## Dependencies / Prerequisites

- Local clone at `/Users/mvanhorn/printing-press-library` must be brought up to date with `origin/main` before starting `ce-work` (44 commits behind as of plan write).
- Go 1.26.3 (matching the workflow). Already installed locally.
- Node + npm for `npm/` tests. Already installed.

## Out of Scope (one-line each)

- Backfilling all curated registry descriptions back into `.printing-press.json` — only the two strictly required for source-reproducibility are touched here.
- Generator output formatting / sort changes / canonical-name lookups — orthogonal concerns.
- Any other generated artifact (`README.md` sentinel regions, `cli-skills/`) — those have their own pipelines and risk profiles.
