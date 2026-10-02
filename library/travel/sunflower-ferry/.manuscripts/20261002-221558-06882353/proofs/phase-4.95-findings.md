# Phase 4.95 local code review

Nine actionable findings autofixed in-place across two review rounds; this first-print tree has no patch commits. The updated hand-authored files and independent round-2 resolution table are the authoritative source record. Findings cleared at round 2. Review path: direct single fresh-context gpt-6.1-sol MAX subagent dispatch, combining correctness/security/maintainability/API-contract/output/docs under the user explicit one-reviewer limit.

## Template-shape retro candidates

T1: generated MCP endpoint tools bypass CLI HTML extraction; raw HTML result differs from CLI page metadata. Template family: generated MCP registration/handler. T2: generated client ordinary-body io.ReadAll is unbounded. Template family: shared HTTP client reader. T3: optional HTTP MCP server lacks ReadHeaderTimeout. Template family: cmd MCP HTTP server. T4: Press dead-code remover misses registered callbacks and would remove attachFerryCommands. Detailed locations, severity and rationale are in independent-review-round2.md. None affects the verified bounded native planning interface; lower-level generated limitations are disclosed in README/SKILL.

## Out-of-scope and security diagnostics

Shared generated/reserved gosec diagnostics have 30 findings and zero native ferry findings, with contextual false positives and template candidates. Retained actual scanner result is gosec-before.json, specific dispositions gosec-disposition.md and reviewer addendum. No claim of a clean whole-repository security scan. Reserved cliutil packages were not patched.

## Surface-to-user findings

No unresolved tradeoff in the authorized ferry scope. Shared generator limitations are documented for the parent/final report; no scope shrink or external publication occurred.
