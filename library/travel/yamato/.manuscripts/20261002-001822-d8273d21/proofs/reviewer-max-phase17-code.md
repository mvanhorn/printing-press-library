# Phase17 — Local code review

1 consequential domain finding fixed by builder and independently reverified across 2 rounds; see current parcel.go/test diff and reviewer-final-focused.json. No in-scope findings remain.

## Template retro candidates

- **PP-MAX-01 / P3 — internal/cli/root.go:401.** Invalid --data-source produces exit1 rather than planning validation exit2. Reproduction: `airports --data-source bogus --json --no-learn`. README now describes it accurately. Fix upstream root usage-error wrapping on regeneration. Generated root template; exact upstream template filename was not resolved. Filed instead of locally patching shared framework behavior.
- **PP-MAX-02 / P2 — cmd/yamato-pp-mcp/main.go:86.** Optional HTTP server sets only Addr/Handler; ReadHeaderTimeout and ReadTimeout remain zero, so incomplete headers can keep unauthenticated connections alive. Static server-literal evidence and gosec G112 agree. Add a finite upstream header timeout while preserving streaming response semantics. Generated MCP entrypoint template; exact upstream template filename was not resolved. No network listener attack was performed. Local stdio remains the default.

31 gosec observations remain in gosec-before.json/security-checks.json and are generated framework shapes; none concern handwritten Yamato source or the six command bodies. G112 is the independently confirmed pertinent security candidate. Sampled origin guards, SQL parameter/identifier guards, local-state paths and directory permissions do not establish the scanner's implied vulnerabilities; this review does not blanket-dismiss all scanner observations. Provided govulncheck found 0 vulnerabilities.

## Out-of-scope candidates

No additional independently confirmed issue in generator-reserved internal/cliutil or internal/mcp/cobratree. Raw scanner observations in those paths remain machine evidence, not local patch requests.

## Scope and verification

Reviewed the full handwritten domain/command implementation and relevant generated CLI/MCP/client/store boundaries, including security, correctness, maintainability, source/API contracts, timeouts, output bounds and errors. Explicit boundCtx appears before live requests in all 5 live command bodies. Fixed-source HTTPS origins, ephemeral public cookie jars, source body bounds and snapshot fingerprints preserve the stated read-only planning contract.

Focused post-fix domain tests pass. CLI reproductions confirm exact 160/200 cm categories, exact 60 cm decimal handling and rejection of a real over 200 cm total. Provided full Go tests/vet and scanner receipts were inspected; independent public quote/policy reads succeeded after authorized network escalation. Native Chrome review verified the selected PDF rows.

Convergence: findings cleared at round 2; only non-gating machine retro candidates remain.

Review path: direct dedicated fresh-context MAX reviewer, covering correctness/security/maintainability/domain/API/docs/output personas; no further agents spawned. Reviewer did not edit source or publish.
