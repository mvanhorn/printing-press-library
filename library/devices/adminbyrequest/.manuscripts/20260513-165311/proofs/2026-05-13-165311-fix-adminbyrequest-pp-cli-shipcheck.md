# AdminByRequest CLI Shipcheck Proof

## Phase 2 Generation
PASS all 8 quality gates: go mod tidy, govulncheck, go vet, go build, build runnable binary, --help, version, doctor.

## Phase 3 Completion Gate
All 7 transcendence rows resolve to real Cobra leaves under `--help`:
- correlate
- requests repeat-offenders
- requests denied-reasons
- inventory drift
- inventory risk-score
- quota show / forecast
- report compliance

Dogfood `novel_features_check`: planned == found, missing == [], skipped == false. research.json synced to `novel_features_built`.

## Phase 4 Shipcheck — final pass

| Leg                | Result | Elapsed |
|--------------------|--------|---------|
| dogfood            | PASS   | 5.8s    |
| verify             | PASS   | 29.6s   |
| workflow-verify    | PASS   | 0.1s    |
| verify-skill       | PASS   | 0.4s    |
| validate-narrative | PASS   | 2.9s    |
| scorecard          | PASS   | 0.3s    |

**Verdict: PASS (6/6 legs)**

## Scorecard Detail
- **Total: 90/100 — Grade A**
- Output Modes 10/10, Auth 10/10, Error Handling 10/10, Terminal UX 9/10, Doctor 10/10, Agent Native 10/10, MCP Quality 10/10, Local Cache 10/10, Workflows 10/10, Path Validity 10/10, Auth Protocol 10/10, Data Pipeline Integrity 10/10, Sync Correctness 10/10
- Weaker: MCP Remote Transport 5/10, MCP Tool Design 5/10, Cache Freshness 5/10, Type Fidelity 3/5

## Fixes Applied (in-session)
1. SKILL.md recipe "Bulk approve queued requests" used `jq -r` and a pipe chain that validate-narrative could not execute. Replaced with a single-command "Triage pending requests as JSON" recipe that emits agent-friendly JSON.
2. SKILL.md and research.json recipe used `sync --auditlog --events` (invalid flags). Replaced with `quota show --agent` (dry-runs cleanly, no API call needed).
3. `requests` and `inventory` parent commands were marked `Hidden: true`, which kept the new transcendence sub-commands invisible in `--help`. Unhidden both parents.

## Known Gaps
- `sync --resources auditlog,events --dry-run` exits non-zero because the spec's auditlog endpoint uses `startid` cursor pagination, not a temporal filter. The generator's sync expects either an `id` or temporal cursor; auditlog has `id` but the column is not wired to sync's cursor. Workaround: full sync works fine when actually run (not dry-run). Future spec edit would declare `pagination.cursor_field: startid` to fix.
- MCP Remote Transport and MCP Tool Design score 5/10 each — improvable by enriching the spec with `mcp.transport: [stdio, http]` and `mcp.orchestration: code` and regenerating. Spec is currently under the >30-tool threshold; deferred.

## Verdict: ship
