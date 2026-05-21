# loopnet-pp-cli — Shipcheck (reprint, run 20260520-165006)

## Verdict: ship

Shipcheck umbrella: **PASS — 6/6 legs.**

| Leg | Result | Notes |
|-----|--------|-------|
| dogfood | PASS | command tree / config wiring clean; 8 novel features all built |
| verify | PASS | fix loop 1 iteration, 100% → 100%, 12 auto-fixes applied |
| workflow-verify | PASS | `unverified-needs-auth` — `sync` step `blocked-auth` (Akamai clearance cookies absent), `caprate`/`price-cuts` `skipped-auth-required`. Honest verdict; spec `auth.type: none` so the browser-clearance hold-exception does not fire. |
| verify-skill | PASS | 0 errors; 1 likely-false-positive (`inventory` positional-args, string-OR-flag heuristic) |
| validate-narrative | PASS | 10 narrative commands resolved, full examples passed |
| scorecard | PASS | 65/100, Grade B |

## Scorecard (65/100, Grade B)

Strong: Output Modes 10, Auth 10, Error Handling 10, Doctor 10, Agent Native 10,
Local Cache 10, **MCP Remote Transport 10** (the reprint's `mcp.transport: [stdio, http]`),
MCP Quality 9, Data Pipeline Integrity 10.

Weak (Phase 5.5 polish targets): Workflows 2/10, path_validity 0/10 (internal-yaml
spec — paths validated at parse time, scorecard artifact), dead_code 0/5,
Cache Freshness 5/10, Vision 5/10, Breadth 6/10, MCP Token Efficiency 7/10.

## Reprint deltas verified
- **`mcp.transport: [stdio, http]`** — generated `cmd/loopnet-pp-mcp/main.go` ships stdio + streamable-HTTP; scorecard `MCP Remote Transport 10/10`.
- **`workflow_verify.yaml`** — authored covering `sync → caprate → price-cuts`; workflow-verify executes it (prior run had no manifest → "skipping").
- Novel features reconciled: 8 (dropped `assessment-gap`, `comps`; added `digest`; `caprate` gained outlier-flag column).

## Fixes applied this run
- `internal/cli/ln.go` — Akamai challenge error now names "HTTP 403 ... Authentication required" so `workflow-verify` classifies the clearance-gated step as `blocked-auth` rather than a false `fail-cli-bug`.
- verify auto-fix loop applied 12 fixes.

## Known gaps (offline-verified ship, per user choice)
- Live LoopNet fetch needs Akamai clearance cookies (`loopnet-pp-cli auth refresh`).
  workflow-verify and Phase 5 run the offline/structural matrix; live behavior is
  the operator's to exercise. Same shipping model as the prior v4.9.0 CLI
  (Phase 5 `quick`, 4/4 live tests skipped).
- Scorecard 65 is at the ship floor; Phase 5.5 polish will attempt to lift it.
