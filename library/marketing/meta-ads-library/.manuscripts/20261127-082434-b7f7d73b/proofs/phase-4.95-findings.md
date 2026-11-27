# Phase 4.95 — Local Code Review Findings (`meta-ads-library-pp-cli`)

- **Autofix summary:** 24 findings autofixed in-place across 3 rounds in working tree `$CLI_WORK_DIR` (pre-promotion; no git repo initialized yet in working tree).
- **Convergence outcome:** Findings cleared at round 3 (final `AdType` guard in `isSnapshotEligiblePageScan` applied and verified with `go test` + 7/7 `shipcheck` PASS).
- **Review path chosen:** direct subagent dispatch (`correctness`, `security`, `maintainability` lenses across 3 sequential full-scope rounds).

## Template-shape retro candidates

- **`SKILL.md` Go minimum version pin (`requires Go 1.26.10 or newer`)**: `cli-printing-press v4.35.4` emits `go 1.26.10` in `go.mod` and pins `requires Go 1.26.10 or newer` inside `SKILL.md`'s generator-owned `## Prerequisites: Install the CLI` section. Because `GO-2026-6859` requires `go 1.26.11` for clean `govulncheck`, bumping `go.mod` to `1.26.11` works, while editing the version in `SKILL.md` trips `verify-skill` `canonical-sections` drift. Filed as retro candidate rather than patching `SKILL.md` canonical section.
- **`internal/client/client.go` `tier_routing` + `auth: none` default tier**: Declaring `tier_routing` with `default_tier: free` (`auth: none`) alongside a credentialed tier emits undefined `authHeaderLooksLikePlaceholderCredential` / `authPlaceholderCredentialErrorWithSetup` references in generated `internal/client/client.go`. Resolved at spec level for this CLI; filed as generator template retro candidate.

## Out-of-scope retro candidates

None.

## Surface-to-user findings

None — all in-scope findings were mechanical, behavior-preserving fixes closed in place.
