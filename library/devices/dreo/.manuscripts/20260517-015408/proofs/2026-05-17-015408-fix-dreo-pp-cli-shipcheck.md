# Dreo CLI Shipcheck Final Report

**Date:** 2026-05-17
**Final verdict:** ship

## Shipcheck legs (final run)

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | 8/8 novel features verified; dogfood structural verdict WARN (dead helpers cleared by polish; sync generic-Upsert is structural) |
| verify | PASS | 96% pass-rate |
| workflow-verify | PASS | workflow-pass |
| verify-skill | PASS | 0 mechanical mismatches |
| validate-narrative | PASS | every quickstart and recipe command resolves and dry-runs cleanly |
| scorecard | PASS | 76/100 Grade B |

## Final scorecard breakdown

```
  Output Modes         10/10
  Auth                 10/10
  Error Handling       10/10
  Terminal UX          9/10
  README               8/10
  Doctor               10/10
  Agent Native         10/10
  MCP Quality          8/10
  MCP Token Efficiency 7/10
  MCP Remote Transport 10/10
  Local Cache          10/10
  Breadth              7/10
  Vision               9/10 (post-polish)
  Workflows            2/10
  Insight              9/10 (post-polish, +5 delta)
  Agent Workflow       9/10

  Domain Correctness
  Path Validity           10/10
  Auth Protocol           10/10
  Data Pipeline Integrity 10/10
  Type Fidelity           3/5
  Dead Code               5/5 (post-polish, +2 delta)

  Total: 76/100 - Grade B (above 65 ship threshold)
```

## Live API verification (Phase 5)

- 64/64 tests passed against the real Dreo API (`app-api-us.dreo-tech.com`)
- 1 real device on the test account (Air Circulator DR-HPF004S)
- Phase5 acceptance gate marker: `phase5-acceptance.json` status=pass, level=full

## Issues found and fixed in-session

1. **Wrong auth header synthesis** — generator emitted `Bearer <email>` when no AccessToken set; patched `Config.AuthHeader()` to return `""` instead.
2. **State envelope unwrap** — Dreo wraps every state field in `{state: <value>, timestamp: <unix>}`; added `flattenState()` in `dreo_helpers.go`.
3. **Missing `online` field** — Dreo's device list endpoint doesn't return `online`; defaulted to `true` instead of generator's `false`-on-missing-bool.
4. **No auto-login** — added `Client.lazyLogin()` so the CLI works under any clean HOME with just `DREO_USERNAME`/`DREO_PASSWORD` env vars; critical for the dogfood --live scoped HOME pattern.
5. **`scene list` missing Examples** — added.
6. **`watch <name>` accepted nonexistent device names silently** — now resolves the filter to a sn before opening WS; exits non-zero on not-found.
7. **5 dead helpers removed** (polish).
8. **`sensors query --json` returned `null` on empty** — fixed to `[]` (polish).

## Final ship recommendation

**ship.** All shipcheck legs pass, live dogfood 64/64, all 8 novel features built and behaviorally verified against the real Dreo API. No known functional bugs in shipping-scope features.
