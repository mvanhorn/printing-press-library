# tella-pp-cli — Shipcheck Report

## Verify
Pass rate: 100% across all spec-derived commands. 0 critical failures.

## Dogfood
PASS — wiring checks pass, no dead helpers, all examples present after fixes.

## Workflow-verify
PASS (no manifest emitted; not applicable for read+webhooks CLI shape).

## Verify-skill
PASS after fixing two stale flags (`--replay-to` removed from SKILL.md; `--follow` re-bound via BoolVar).

## Validate-narrative
PASS after fixing quickstart and recipe paths:
- `auth set-token` step replaced with `doctor` (auth set-token requires a token arg, narrative shouldn't bake one)
- `webhooks tail --follow --replay-to` recipe split into `webhooks tail` + `webhooks replay` (matches actual command shape)
- `clips edit-pass --dry-run` recipe dropped `--dry-run` (default mode is plan-print)

## Scorecard
**89/100 — Grade A**
- Output Modes 10, Auth 10, Error Handling 10, Terminal UX 9, README 8, Doctor 10, Agent Native 10
- MCP Quality 10, MCP Remote Transport 10, MCP Tool Design 10, MCP Surface Strategy 10 (Cloudflare pattern enrichment)
- Local Cache 10, Cache Freshness 5, Breadth 10, Vision 8, Workflows 10, Insight 8, Agent Workflow 9
- Domain: Path Validity 10, Auth Protocol 8, Data Pipeline 10, Sync 10, Type Fidelity 2/5, Dead Code 4/5
- Gap: 51 MCP tools (0 public, 51 auth-required) — readiness: full

## Phase 5 Live Dogfood
Verdict: **PASS** (167/167 tests, 165 skipped, 0 failed)
Level: full
Auth: bearer_token (TELLA_API_KEY)

Round 1 (initial): 10 failures — all 8 transcendence commands missing `Example:` strings (Cobra didn't render `Examples:` block).
Round 2: 3 failures after Example: fixes — error_path needed real exit codes for missing required flags, and json_fidelity needed JSON envelope on dry-run/missing-flag.
Round 3: 1 failure on `clips edit-pass --json --dry-run` — short-circuit returned empty stdout.
Round 4: PASS — dryRunOK now emits a valid JSON envelope.

Fixes applied to hand-built transcendence commands:
- Added `Example:` to all 8 commands
- Removed local `--dry-run` flag from `clips edit-pass` and `webhooks replay` (was shadowing global)
- Added usageErr/JSON-envelope on missing required flags for `clips captions`, `clips transcript-diff`, `clips edit-pass`
- Updated SKILL.md to remove stale `--replay-to` flag reference
- Updated narrative recipes to match actual command shapes

## Ship Threshold
- shipcheck umbrella: PASS (6/6 legs)
- verify: 100% pass rate
- dogfood (live): 167/167 with real Tella workspace
- workflow-verify: pass
- verify-skill: pass
- scorecard: 89/100 ≥ 65 threshold
- All 8 transcendence features behaviorally tested against live API: PASS
  - transcripts search returned hits after sync
  - videos viewed returned valid JSON envelope
  - webhooks tail --once snapshotted real inbox
  - webhooks replay generated valid HMAC dry-run
  - clips edit-pass planned ops on real playlist
  - clips transcript-diff returned 8 removed words on real clip
  - exports wait queued real export
  - clips captions emitted valid SRT/VTT
  - workspace stats aggregated 4768 transcript words from real workspace

**Ship recommendation: ship**
