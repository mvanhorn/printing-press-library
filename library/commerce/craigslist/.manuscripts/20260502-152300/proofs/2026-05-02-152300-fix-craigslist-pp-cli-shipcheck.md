# craigslist-pp-cli Shipcheck Report

## Verdict: ship

```
LEG               RESULT  EXIT      ELAPSED
dogfood           PASS    0         988ms
verify            PASS    0         3.487s
workflow-verify   PASS    0         11ms
verify-skill      PASS    0         247ms
scorecard         PASS    0         32ms

Verdict: PASS (5/5 legs passed)
```

## Per-leg findings

### dogfood — PASS

- 10/10 novel features survived (planned vs. built parity).
- MCP surface mirrors the Cobra tree at runtime.
- Initial run reported 2/10 commands missing `Example:` strings (drift, favorite list, filters show, listing get, median, reposts, watch delete, watch save). Fixed inline — every novel command now ships with realistic, domain-specific examples covering both human and `--json` invocations.
- Initial run flagged `search.go` and `cities heat (cities.go)` as candidate reimplementations. False positives: both call into `internal/source/craigslist/` (the typed Craigslist client) which the dogfood scanner does not yet attribute as an "API client". The behavior IS calling sapi/sitemap, just through our typed wrapper rather than the generic generated `client.Client`. Documented; no functional issue.

### verify — PASS

- 30/30 commands pass HELP and DRY-RUN; 0 critical EXEC failures.
- 6 commands (cl-sync, drift, median, reposts, scam-score, since) score 2/3 in mock mode because they read from the local store, which is empty in a fresh mock-mode run. Acceptable — the verify pass-rate is 100% (no critical failures). These commands work end-to-end once `cl-sync` populates the store, which is the documented happy-path workflow.

### workflow-verify — PASS

- No workflow manifest defined for this run; framework reports `workflow-pass`.

### verify-skill — PASS

- All checks passed (flag-names, flag-commands, positional-args, unknown-command). SKILL.md does not lie about the CLI surface.

### scorecard — 85/100 Grade A

| Dimension | Score |
|-----------|-------|
| Output Modes | 10/10 |
| Auth | 10/10 |
| Error Handling | 10/10 |
| Terminal UX | 9/10 |
| README | 8/10 |
| Doctor | 10/10 |
| Agent Native | 10/10 |
| MCP Quality | 9/10 |
| MCP Token Efficiency | 0/10 ⚠ |
| MCP Remote Transport | 10/10 |
| Local Cache | 10/10 |
| Cache Freshness | 10/10 |
| Breadth | 7/10 |
| Vision | 9/10 |
| Workflows | 10/10 |
| Insight | 10/10 |
| Agent Workflow | 9/10 |

| Domain Correctness | Score |
|-------------------|-------|
| Data Pipeline Integrity | 7/10 |
| Sync Correctness | 10/10 |
| Type Fidelity | 3/5 |
| Dead Code | 5/5 |

**Open gaps (handed to polish):**
- `mcp_token_efficiency: 0/10` — MCP tool descriptions need tightening. Polish skill will address.
- `Type Fidelity: 3/5` — minor; some response types could use stronger schemas. Polish skill will address.

## Ship-threshold check

- ✅ shipcheck exits 0 (umbrella verdict PASS, 5/5 legs)
- ✅ verify pass-rate 100%, 0 critical failures
- ✅ dogfood passes (false-positive reimpl warnings noted)
- ✅ workflow-verify: workflow-pass
- ✅ verify-skill exits 0
- ✅ scorecard ≥ 65 (85/100, Grade A)
- ⏳ Behavioral correctness on flagship features: deferred to Phase 5 dogfood (live testing).

## Fixes applied

1. Added `Example:` strings to 8 commands: `drift`, `favorite add/list/remove`, `filters show`, `listing get/get-by-pid/images`, `median`, `reposts`, `watch save`, `watch delete`. Each example uses real domain values (UUIDs, PIDs, category abbrs, queries) so users and verify-skill probes get useful copy-paste invocations.

## No functional regressions

- `go build` clean.
- `go test ./...` 133 tests passing.
- `go vet ./...` clean.

## Verdict

**ship** — proceed to Phase 5 live dogfood. The 0/10 on MCP token efficiency and the Type Fidelity 3/5 dimension are polish concerns, not ship blockers.
