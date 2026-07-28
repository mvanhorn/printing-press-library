# Acceptance Report: wpengine

Level: Full Dogfood (binary-owned live matrix, user-approved including write-side backup fixture)
Tests: 279/279 passed (175 additional rows skipped by the runner's own classification: destructive-at-auth endpoints, credential-gated variants)

## Failures encountered and resolved (1 fix loop)
- `installs offload-settings get` ×2 (happy-path + JSON-fidelity): HTTP 404 "LargeFS settings not found". Root cause: the API answers 404 for any install that has never configured LargeFS offload — a normal state for every install in the test workspace, not a defect; the CLI correctly mapped it to exit 3 with an honest message. Fix applied: annotated the command `pp:typed-exit-codes: "0,3"` and documented the command-specific exit semantics in its help. Re-run: 279/279 PASS.

## Fixes applied: 1
- installs_offload-settings_get.go: typed-exit-codes annotation + Long help documenting exit 3 as the "offload not configured yet" state.

## Printing Press issues (for retro): 1 (from this phase)
- Live dogfood matrix lacks a BLOCKED_FIXTURE/optional-resource classification: a generated GET whose upstream 404s until an optional feature is configured is counted as a hard failure unless hand-annotated. (Additional machine issues from earlier phases are catalogued in phase-4.95-findings.md.)

## Live verification highlights (redacted)
- doctor: auth valid, API reachable, config paths resolved.
- sync: full fleet mirror built (accounts/sites/installs/domains/backups/certs; 1,500+ records) in the test workspace.
- Transcendence commands returned real, relevant findings against the live fleet: expired certificates surfaced with correct day math; PHP 7.4 outliers listed; prod/staging version drift flagged on four sites; the overage projection flagged the test workspace's account as trending over its bandwidth limit; whois resolved a real client domain to its install/site/account with cert status.
- guard: exercised through the matrix's write-side fixture (checkpoint backup on a test install; poll curtailed per dogfood env).

Gate: PASS
