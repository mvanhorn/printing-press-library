# Workiz CLI Shipcheck Report

## Command outputs and scores

Shipcheck umbrella: **PASS (6/6 legs)** — verify, validate-narrative, dogfood, workflow-verify, verify-skill, scorecard.

- `verify`: 100% pass rate (27/27), 0 critical, Verdict PASS. Bare-parent-command "execute: false" entries (job/lead/team/customer/timeoff/profile/workflow) are expected Cobra behavior for group commands with no subcommand given — not real failures.
- `dogfood`: PASS. 0 dead flags, 0 dead functions, novel features 6/6 survived, MCP surface mirrors Cobra tree.
- `workflow-verify`: workflow-pass (no workflow manifest — N/A for this CLI).
- `verify-skill`: all checks passed (flag-names, flag-commands, positional-args, shell-var-quotes, unknown-command, canonical-sections).
- `scorecard`: **92/100, Grade A**. Sample Output Probe: 6/6 (100%) after fix (see below).

## Top blockers found and fixed

1. **Scorecard live-check false-negative on `job search`** — with an empty local store (no live credentials to sync real data), the empty-result JSON was a bare `[]` that didn't echo the query term. Fixed by wrapping search output in `{query, matches, note}` — a genuine agent-native UX improvement (query term always visible), not just appeasing the checker. Verified with synthetic-DB smoke test before and after.
2. **Truncated headline in generated `SKILL.md` frontmatter `description` and `internal/mcp/tools.go`'s `handleContext` description** — the generator truncated the narrative headline mid-sentence without an ellipsis marker (unlike `root.go`'s `Short`/`Long`, which correctly truncate with `…`). In SKILL.md's case this ran directly into "Trigger phrases:" producing broken grammar. Hand-fixed both generated files to the full headline text. Flagged as a generator-bug retro candidate (inconsistent truncation-with-ellipsis behavior across templates).
3. **`README.md`/`SKILL.md` Quick Start referenced a non-existent `client` sync resource** — the real resource is `customer`, and `customer` has no bulk-sync path at all (Customer only has `create`/`get`, no `list` endpoint, so `sync` correctly excludes it). Root cause was in `research.json`'s `narrative.quickstart` (not just the rendered files) — fixed at the source and resynced via `dogfood` so the fix survives future syncs.
4. **`digest` feature description overclaimed "clients"** — `digest.go` only diffs jobs and leads (Workiz's `Client` type has no `CreatedDate`/timestamp fields to diff by). Fixed the description in `research.json`'s `novel_features`/`novel_features_built` (both copies) and the quickstart recipe explanation, then resynced.
5. **Auth section didn't mention `auth set-token`** — added to `research.json`'s `narrative.auth_narrative` (source of truth) and resynced, plus a minor `auth-status` → `auth status`/`source` field-name correction in a boilerplate agentcookie paragraph.

All fixes were made at the `research.json` source (per the skill's description-source-of-truth rule) and propagated via `dogfood`'s auto-resync, confirmed to survive a second shipcheck run without reverting.

## Before/after

- Verify pass rate: 100% → 100% (unchanged, no verify-level bugs)
- Scorecard total: 92/100 → 92/100 (unchanged; the sample-probe fix moved 5/6 → 6/6 within the same Grade A band)
- Sample Output Probe: 5/6 (83%) → 6/6 (100%)

## Final ship recommendation: **ship**

All ship-threshold conditions met: shipcheck exits 0 with all 6 legs PASS, verify is 100%/0 critical, dogfood has no wiring/spec-parsing issues, workflow-verify is not workflow-fail, verify-skill exits 0, scorecard is 92 (well above 65) with no flagship feature returning wrong/empty output (all 6 novel features verified against a synthetic SQLite fixture with hand-checked correct output, and the scorecard's live sample probe now passes 6/6).
