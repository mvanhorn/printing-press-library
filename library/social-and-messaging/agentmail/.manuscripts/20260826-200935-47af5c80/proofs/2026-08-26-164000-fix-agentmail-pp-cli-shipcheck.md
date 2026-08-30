# AgentMail Shipcheck

## Final verdict
- `shipcheck`: PASS, exit 0.
- Legs: verify PASS; validate-narrative PASS; dogfood PASS; workflow-verify PASS; apify-audit PASS; verify-skill PASS; scorecard PASS.
- Scorecard: 94/100, Grade A.
- Novel features: 6 planned, 6 found/built.

## Behavioral proof
- Full live dogfood: 310/310 tests passed, 0 failed, 439 skipped/unverified by the runner's safe-fixture rules, 100% pass rate, no hollow novel-feature coverage.
- Live reachability: authenticated `GET /v0/auth/me` returned HTTP 200.
- Local workflow fixture: triage returned one unresolved inbound thread; rollup returned counts, participants, latest direction/content, and pending draft; send check returned unsafe for overdue/idempotency-missing draft; schedule audit found overdue/unreviewed draft; delivery reconcile found outbound message with no later inbound; fleet health emitted missing-key readiness finding.
- Negative checks: mismatched inbox/thread filters returned empty arrays with reasons; missing draft returned `safe:false`; `--select` narrowed machine output; empty selected rollup retained the requested thread in its reason.

## Fixes applied
- Normalized Fern component schema keys into parser-safe derived OpenAPI JSON while preserving the official contract.
- Added canonical bearer security and `AGENTMAIL_API_KEY`; removed duplicate explicit Authorization flags from operations.
- Added explicit `query` public flag over wire `q`.
- Implemented six local workflows with bounded scans, context-aware SQLite reads, safe empty states, direction inference from mailbox addresses, composite-ID handling, draft/thread linkage, attachment linkage, schedule validation/deduplication, and local-only source guards.
- Rejected unauthenticated non-loopback MCP HTTP binds.
- Prevented API-key mutation responses from being persisted in the local mirror.
- Corrected narrative command examples and config/auth documentation; added feedback help example.

## Known generator-level gaps
- Generated MCP code-orchestration surface reports 130 auth-required tools; method-specific write annotations should be improved in the generator.
- Generated API-key/signup create responses intentionally return newly issued secrets once, but generic secret-redaction policy is a Printing Press template improvement.
- Generator reports domain response-ID fallback warnings and skips an unavailable `comm_health` workflow template; no shipcheck leg failed.
- Deterministic polish found referenced helper functions in its dead-code candidate set and restored them after the attempted removal failed compilation.

## Final recommendation
`ship`
