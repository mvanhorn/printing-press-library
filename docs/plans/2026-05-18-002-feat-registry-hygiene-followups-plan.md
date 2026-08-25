---
type: feat
status: active
created: 2026-05-18
plan_id: 2026-05-18-002
---

# feat: registry hygiene follow-ups to PR #674

## Summary

PR #674 fixed the lawhub-shape registry failure (npm installer aborting on a single empty `description`) by adding `.printing-press.json` as a third description fallback, backfilling two source-only-in-registry CLIs (tiktok-shop, agent-capture), gating empty required fields at PR time, and hardening the npm installer to skip malformed entries with a warning. Its "Out of scope" section listed three deferred items. A fourth item surfaced during the post-merge verification: the npm publish workflow didn't fire on the `npm-v0.1.5` tag because `GITHUB_TOKEN` pushes can't trigger downstream workflows.

This plan addresses all four:

1. **Migrate curated descriptions back into `.printing-press.json`** and reorder the generator's description preference so the registry is reproducible from sources without depending on prior curated values.
2. **Validate additional MCP fields** in `validateEntries` beyond what the npm installer's `parseRegistry` requires.
3. **Harden `tools/generate-skills`** with a fail-fast `--validate` mode so missing or empty library SKILL.md surfaces before any cli-skills/ mirror gets written.
4. **Fix `auto-tag-npm.yml`** so it pushes the release tag with a PAT, which actually triggers `npm-publish.yml`.

## Target repo

`mvanhorn/printing-press-library`. All file paths are repo-relative.

## Requirements

- R1. After this plan ships, deleting `registry.json` and running `go run ./tools/generate-registry/main.go` from scratch (no prior registry to seed `description`) must produce descriptions from source files for all 140 CLIs. Known intentional one-time registry deltas are description refreshes for amazon-orders, american-reindustrialization, anylist, conduyt-crm, facebook-marketplace, roam, and usgs-earthquakes plus corrected MCP tool counts for conduyt-crm, roam, and squarespace.
- R2. `validateEntries` must reject every shape that the npm installer's `parseRegistryEntry` would throw on AND that the existing MCPB / scorecard tools depend on (`mcp.tool_count > 0` when an MCP block is present, `mcp.env_vars` as a JSON array, `mcp.public_tool_count` as a number when present).
- R3. `tools/generate-skills/main.go` must support `--validate` that exits non-zero with named-slug errors when any library SKILL.md is missing or empty, must NOT write any cli-skills/ output in validate mode, and must be run by the PR-time library-conventions workflow.
- R4. After merging a PR that bumps `npm/package.json`'s `version`, the published npm package must update without manual intervention; no human or agent should need to run `gh workflow run npm-publish.yml`.

## Scope Boundaries

### In scope

- Reorder `registryDescription`'s preference chain in `tools/generate-registry/main.go`.
- Backfill `description` in every `.printing-press.json` whose corresponding registry value would change under the new preference order, except where the source manifest already has materially better catalog copy and the one-time registry delta is intentional.
- Expand `validateEntries` to cover the new MCP-block invariants in R2 and grow the test matrix accordingly.
- Add `--validate` flag to `tools/generate-skills/main.go` plus tests.
- Replace `GITHUB_TOKEN` with the existing `ADMIN_PUSH_PAT` secret in `auto-tag-npm.yml`'s checkout, matching the pattern already used by `generate-registry.yml`.

### Deferred to Follow-Up Work

- Migrating any remaining "registry value happens to match goreleaser brews exactly" entries into `.printing-press.json`. After R1's stricter sourcing, those entries are already source-reproducible (registry == goreleaser); the backfill is a hygiene improvement, not a correctness fix.
- A shared validation framework for `tools/`-level generators. Each generator gets its own `--validate` for now; a unified abstraction is premature with only two consumers (`generate-registry`, `generate-skills`).
- Auditing `tools/sweep-canonical` and other tools for empty-string footguns. Different surface; different invariants.
- Replacing `auto-tag-npm.yml` with a single combined publish workflow. The two-workflow split is documented in AGENTS.md and supports the existing OIDC trusted-publishing configuration; collapsing them would re-open the npm-side trust setup.

### Outside this product's identity

- Retiring `.goreleaser.yaml` brews descriptions as a source. They still feed the Homebrew tap and serve a separate purpose; we just stop letting them outrank `.printing-press.json` for registry generation.

## Key Technical Decisions

### KD1. Reorder description preference to `pp > goreleaser > prior`

Today the chain is `prior curated > .goreleaser.yaml brews > .printing-press.json` (the third tier added in PR #674). Moving `.printing-press.json` to the top makes the manifest authored by the printing-press skill the authoritative description source for the registry. `.goreleaser.yaml` brews remains the fallback for older CLIs without a pp description, and the prior-curated tier survives as a backstop for entries whose source state is mid-migration.

The bare-markdown-heading exception (line 348-355 in `tools/generate-registry/main.go`) stays on the prior tier only. `.printing-press.json` is author-controlled by the publish skill and has never produced "# Introduction"-shaped descriptions; if it does, the right fix is upstream in the generator, not a per-tier exception here.

### KD2. Backfill targets are determined by regenerating and diffing, not from the static audit

The static audit found 26 "curated-divergent" CLIs (registry value differs from both source files). But the real question is: after reordering preferences, would any registry entry's description change? That includes the 26 divergent CLIs plus any CLI where `.printing-press.json` description differs from what the current registry shows (because today's order has `.printing-press.json` last). U1 implements the reorder first, regenerates, and lets the diff against current `main`'s registry name the exact set of pp manifests to backfill or intentionally preserve. This avoids missing edge cases and avoids backfilling entries that don't need it.

### KD3. MCP validation set mirrors what production consumers actually require

Beyond npm's `parseRegistryEntry` (which requires `mcp.binary`, `mcp.transports`, `mcp.tool_count` as number, `mcp.auth_type`), production also implicitly requires `mcp.tool_count > 0` (a 0-tool MCP block is nonsensical), `mcp.env_vars` as an array (the npm parser allows non-arrays via `Array.isArray(...) ? .map(String) : []` but downstream consumers may assume `[]`), and `mcp.public_tool_count >= 0` when present. The validator expands to cover these three.

It does NOT validate `mcp_ready` against a closed enum (current values seen on main: `"full"`, `"partial"`, plus undefined) because the enum boundary isn't well-defined yet and adding it would create false positives.

### KD4. `--validate` for `generate-skills` checks file shape, not content semantics

`tools/generate-skills/main.go` today fatals at the END of a run (after writing partial mirrors) when any library SKILL.md is missing/empty. The `--validate` mode lifts that check to the start: scan all expected SKILL.md paths, fail before any write. Frontmatter shape and slot completeness stay the responsibility of `verify-skills.yml` (which runs an independent Python verifier with its own contract).

### KD5. `auto-tag-npm.yml` switches to `ADMIN_PUSH_PAT`, matching the existing generate-registry pattern

The well-known GitHub Actions invariant: pushes authenticated with the default `GITHUB_TOKEN` don't trigger downstream workflows on the same repo, by design (prevents infinite loops). `generate-registry.yml` already uses `ADMIN_PUSH_PAT` (a maintainer PAT in repo secrets) for its post-commit push for the same reason. Reusing the existing secret keeps trust scope identical and avoids creating new secrets.

Alternative considered: `workflow_run` chaining (have `npm-publish.yml` trigger on the auto-tag workflow's completion instead of on tag push). Rejected because it would untie the tag from the publish — losing the "the tag IS the release" property the AGENTS.md release flow documents.

## System-Wide Impact

- **`registry.json` consumers**: under R1 + KD1, every entry's `description` becomes derived from a source file, so deleting and regenerating is safe. The generated registry should have only the intentional one-time deltas called out in R1.
- **`.printing-press.json` authors going forward**: the publish skill in `cli-printing-press` already writes `description` from `narrative.headline`. After this plan, that value becomes the canonical source for the registry catalog row. No publish-side change needed.
- **Homebrew tap**: untouched. `.goreleaser.yaml` brews `description` still feeds it.
- **PR authors after this plan**: same registry `--validate` gate at PR time, with expanded MCP checks, plus a new `tools/generate-skills --validate` gate for missing or empty source SKILL.md files.
- **NPM release rhythm**: under R4 + KD5, the "bump version → merge → npm package updates" flow no longer requires manual `workflow_dispatch`. The auto-tag workflow's documentation comment ("npm-publish.yml will run on the tag event.") becomes accurate.

## Implementation Units

### U1. Reorder description preference and backfill `.printing-press.json` for affected CLIs

- **Goal:** Make `registry.json`'s description field fully reproducible from `library/<cat>/<slug>/` source files alone, with `.printing-press.json` as the authoritative source.
- **Requirements:** R1.
- **Dependencies:** none.
- **Files:**
  - `tools/generate-registry/main.go` (reorder `registryDescription`, update its docstring + the call-site comment in `buildEntry`)
  - `tools/generate-registry/main_test.go` (update existing `TestRegistryDescription` cases to assert new order; add regression case for prior-curated-only entry)
  - affected `library/<cat>/<slug>/.printing-press.json` files (data backfills; final list determined by U1.4)
- **Approach:**
  1. Reorder `registryDescription(prior, goreleaser, pp)` to return `pp > goreleaser > prior > ""`. The bare-markdown-heading exception still applies only to `prior`.
  2. Run `go run ./tools/generate-registry/main.go --print | diff - registry.json` to identify every CLI whose description would change under the new order.
  3. For each diverging CLI, either copy the CURRENT `registry.json` description value verbatim into that CLI's `.printing-press.json` `description` field or intentionally keep the richer source manifest text when the current registry value is truncated or lower quality. Field placement follows the modern manifest shape.
  4. Re-run the diff and confirm only intentional deltas remain. If any unexpected CLI still diverges, it's because its `.printing-press.json` has a different description from registry — investigate per-CLI whether to update pp or accept the change.
  5. Update the docstring/comments on `registryDescription` and the call-site comment block in `buildEntry` to reflect the new order. The "Description preference" block in `buildEntry` is the canonical place to document the reorder rationale.
- **Patterns to follow:** the existing field-order shape in modern `.printing-press.json` files (suno's manifest is a clean reference). The docstring style on `registryDescription` and `validateEntries`.
- **Test scenarios:**
  - `TestRegistryDescription`: existing case "curated copy wins over both fallbacks" flips — pp now wins over curated; rename + update assertion. Existing case "bare-heading prior falls through to goreleaser" must extend to "bare-heading prior with pp populated returns pp." Add: "only-prior populated" returns prior (legacy entries with no source files still resolve).
  - Integration regression: running the generator twice produces stable output (uses existing on-disk test infrastructure if any; otherwise add a dedicated table-driven test that takes a slug-tree fixture and asserts the resolved description).
- **Verification:**
  - `go test ./...` from `tools/generate-registry/` passes.
  - `go run ./tools/generate-registry/main.go --print | diff - registry.json` shows only the intentional registry deltas from R1.
  - `go run ./tools/generate-registry/main.go --validate` exits 0.

### U2. Expand `validateEntries` to cover additional MCP-block invariants

- **Goal:** Catch malformed MCP blocks at PR time that pass the npm installer's narrow contract but break downstream consumers (MCPB bundler, scorecard, agent context surface).
- **Requirements:** R2.
- **Dependencies:** none.
- **Files:**
  - `tools/generate-registry/main.go` (extend `validateEntries`)
  - `tools/generate-registry/main_test.go` (new cases for each invariant)
- **Approach:**
  - Inside the `if e.MCP != nil` branch of `validateEntries`, add three checks:
    - `e.MCP.ToolCount > 0` → emit `<slug>: mcp.tool_count must be positive (got 0)` when zero/negative.
    - `e.MCP.EnvVars != nil` → emit `<slug>: mcp.env_vars must be a JSON array (got null)`. (The generator's `buildMCPBlock` already initializes to `[]string{}` so this should never fire from a fresh generation; the check catches malformed manual edits.)
    - `e.MCP.PublicToolCount >= 0` (the field is `int`, not pointer, so 0 is fine; negative is the only invalid shape).
  - Keep the existing `mcp.binary`, `mcp.transports` (length > 0), `mcp.auth_type` checks unchanged.
  - Use the existing `isBlank` helper for any string field consistency.
- **Patterns to follow:** existing MCP checks in `validateEntries` (line ~431-440 on main). Test pattern from `TestValidateEntries` (table-driven, substring-matched error messages).
- **Test scenarios:**
  - Happy: valid MCP block with `tool_count: 11`, populated env_vars, public_tool_count: 3 → no errors.
  - Failure: `tool_count: 0` with MCP block present → "mcp.tool_count must be positive".
  - Failure: `env_vars: nil` (synthesized in test) → "mcp.env_vars must be a JSON array".
  - Failure: `public_tool_count: -1` → "mcp.public_tool_count must be non-negative".
  - Edge: no MCP block (e.MCP == nil) → none of the new errors fire.
- **Verification:** `go test ./...` passes. `go run ./tools/generate-registry/main.go --validate` against the current library tree exits 0 (no entries violate the new invariants).

### U3. Add `--validate` flag to `tools/generate-skills/main.go`

- **Goal:** Surface missing or empty library SKILL.md files at PR time without writing any cli-skills/ output.
- **Requirements:** R3.
- **Dependencies:** none.
- **Files:**
  - `tools/generate-skills/main.go` (new flag + early-exit branch)
  - `tools/generate-skills/main_test.go` (or `main_test.go` if it doesn't exist yet — check during execution)
- **Approach:**
  - Add `validate := flag.Bool("validate", false, "exit non-zero if any library SKILL.md is missing or empty; do not write")`.
  - When `*validate`:
    - Run `discoverLibrarySkills("library")` exactly as the normal path does.
    - For each entry, check if `<entry.Path>/SKILL.md` exists and has non-empty content (after trimming whitespace, mirroring `validateEntries`'s `isBlank` semantics).
    - Collect failures. Exit 0 with a one-line OK message when none, or exit 2 with one error line per failing slug.
  - The validate branch returns before the existing write loop. No `cli-skills/` directory writes happen.
- **Patterns to follow:** `--validate` in `tools/generate-registry/main.go` (early-exit pattern, exit code 2, stderr output style). The existing missing-skill detection at line 100-103 of `generate-skills/main.go` (current source-of-truth for what counts as "empty").
- **Test scenarios:**
  - Happy: every library/<cat>/<slug>/SKILL.md exists and is non-empty → exit 0, prints `Validation passed (N entries)`.
  - Failure: synthetic fixture where one CLI's SKILL.md is empty → exit 2, error report names that slug.
  - Failure: synthetic fixture where one CLI is missing SKILL.md entirely → exit 2, error report names that slug.
  - No writes happen: after running `--validate` against a fixture, no new files appear under `cli-skills/`.
- **Verification:** `go test ./...` from `tools/generate-skills/`. `go run ./tools/generate-skills/main.go --validate` from repo root exits 0. Confirm no `cli-skills/` changes (`git status` shows no diff after).

### U4. Wire `auto-tag-npm.yml` to push with `ADMIN_PUSH_PAT` so the publish workflow fires

- **Goal:** Restore the "merge → tag → publish" auto-release flow that AGENTS.md documents. Eliminate the need for manual `gh workflow run npm-publish.yml`.
- **Requirements:** R4.
- **Dependencies:** none.
- **Files:**
  - `.github/workflows/auto-tag-npm.yml` (add `token:` on `actions/checkout@v6`; reference `ADMIN_PUSH_PAT`)
- **Approach:**
  - In the existing `actions/checkout@v6` step, add `token: ${{ secrets.ADMIN_PUSH_PAT }}` so the working tree is authenticated as the PAT owner.
  - The subsequent `git push origin "$tag"` command then uses the PAT's credentials. Pushes from a PAT-authenticated identity DO trigger downstream workflows, so `npm-publish.yml` fires on the resulting tag-push event.
  - Add a short comment block above the `token:` line explaining the GITHUB_TOKEN-doesn't-chain-workflows invariant and pointing to AGENTS.md's release-flow section.
  - No change to `npm-publish.yml` itself.
- **Patterns to follow:** `.github/workflows/generate-registry.yml` line 60-61 (`token: ${{ secrets.ADMIN_PUSH_PAT }}` on its checkout — the same trick for the same reason).
- **Test scenarios:** none added in this unit. The behavior is observable only via a live release. Manual verification path documented in Verification.
- **Verification:**
  - YAML lints clean (`python3 -c "import yaml; yaml.safe_load(...)"`).
  - Workflow run with `workflow_dispatch` still works (sanity).
  - Live verification (after merge): bump `npm/package.json` patch in a follow-up PR, merge, confirm `npm-publish.yml` fires automatically on the resulting tag without manual workflow_dispatch.

## Risks

- **U1 risk:** Reordering preference shifts the "default" source. If a `.printing-press.json` description differs from the current registry value AND U1.4's regenerate-and-diff doesn't catch it, the registry text changes silently for that CLI. Mitigated by KD2: the unit's verification is "diff contains only intentional deltas" — any unhandled divergence fails the verification and forces investigation.
- **U2 risk:** Tightening `mcp.tool_count > 0` could reject any CLI that legitimately ships a zero-tool MCP block. Reality check: such a CLI isn't useful as an MCP target; if one exists, it should remove the `mcp:` block entirely rather than declare zero tools. The source backfills make `go run ./tools/generate-registry/main.go --validate` pass before shipping.
- **U3 risk:** None known. The validate mode is read-only; the existing `verify-skills.yml` already covers SKILL.md content validation.
- **U4 risk:** `ADMIN_PUSH_PAT` has elevated privileges; using it on a path that's currently `GITHUB_TOKEN` widens the blast radius if the workflow is ever compromised. Mitigated: the workflow only triggers on `npm/package.json` changes to `main`, which already requires a merged PR — the same gate that protected the prior generate-registry workflow's use of the same PAT.

## Verification

- All Go tests pass: `cd tools/generate-registry && go test ./...`; `cd tools/generate-skills && go test ./...`.
- `go run ./tools/generate-registry/main.go --print | diff - registry.json` shows only the intentional R1 deltas.
- `go run ./tools/generate-registry/main.go --validate` exits 0 with the expanded MCP checks active.
- `go run ./tools/generate-skills/main.go --validate` exits 0 against the live library tree.
- `verify-library-conventions.yml` passes on the PR (re-runs the existing gate plus the new MCP checks).
- The PR does NOT commit a regenerated `registry.json` or `cli-skills/pp-*/SKILL.md`; the post-merge `generate-registry.yml` and `generate-skills.yml` workflows handle regeneration. (This is the existing repo convention — see `verify-library-conventions.yml`'s "Fail on changes to generated artifacts" step.)
- Post-merge: `auto-tag-npm.yml` runs on the next `npm/package.json` version bump and `npm-publish.yml` fires automatically (R4 live check).

## Dependencies / Prerequisites

- Current main checkout brought up to date with `origin/main` (PR #674 already merged so main has the new generator + npm 0.1.5).
- Go 1.26.3 toolchain (matches CI).
- `ADMIN_PUSH_PAT` secret already exists in the repo (in use by `generate-registry.yml`). No new secret required.

## Out of Scope (one-line each)

- Replacing `.goreleaser.yaml` brews descriptions with `.printing-press.json` descriptions for Homebrew tap purposes — separate concern, separate consumer.
- A shared validation framework abstracting `validateEntries` and the new `generate-skills` validator — two consumers don't justify the abstraction yet.
- Expanding the validator to gate `manifest.json` (MCPB bundle metadata) — already covered by `verify-manifests.yml`.
- Auditing other tools under `tools/` (sweep-canonical, etc.) for similar empty-string footguns — separate plan if a real bug surfaces.
