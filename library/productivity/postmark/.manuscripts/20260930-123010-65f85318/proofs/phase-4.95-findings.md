<!-- slop-gate: off -->
# Phase 4.95 local code review findings

Review path: direct subagent dispatch (feature-dev:code-reviewer) with correctness, security, and maintainability personas, three rounds, all personas each round; round 3 merged correctness+security into one reviewer.

Autofix summary: about 40 findings autofixed in-place across 3 rounds (working tree, no commits yet; see build log). Behavior-preserving consolidation of hand files renamed slice_b_/slice_c_ files to postmark_*.go with shared helpers; --help output diffed identical across 173 command paths after each refactor.

Round-3 findings fixed after the final review (verified by tests and live checks, no 4th round per the 3-round cap):
- --show-tokens could persist unmasked tokens through generated write-through to data.db. Removed the global flag; `servers tokens [name]` is now the only reveal path (internal context lookup, no cache, no store, mcp:hidden). Test: TestServersTokensRevealsWithoutStoringAndListStaysMasked; live decoy-home check found no full token in any file.
- send-once derived key ignored the body. Body now part of the key. Test: TestSendOnceDerivedKeyIncludesBody.

Surface-to-user decisions:
- MCP agents can deliver email only through `email send-once --send` and `bounces resend-blocked --send` (cobratree shell-out tools, non-read-only so hosts prompt). Every other path, including the generated MCP execute tool, is blocked at the transport unless the sandbox token is used. Intended design.

Template-shape retro candidates (not fixed in-place):
- internal/mcp/code_orch.go postmark_execute tool has no annotations or confirmation for mutating endpoints (send path mitigated by the printed CLI's transport block; template deletes and server create/delete still reachable). Generator: add destructive/open-world hints and a confirm parameter for non-GET endpoints.
- Generated write-through (writeThroughCache / writeMutationResponseToStore in data_source.go) has no hook for response redaction before store writes.

Out-of-scope retro candidates: none found in internal/cliutil or internal/mcp/cobratree.

Convergence: round-3 findings cleared by fixes; gates green (go vet, go test ./..., dogfood WARN only for the generated dead helper, verify-skill pass).
