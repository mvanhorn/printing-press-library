# Phase 4.95 local code review: unifi-pp-cli

Review path: direct review by the assigned agent (round 1), then direct subagent dispatch with one adversarial reviewer (round 2) and one narrow regression reviewer (round 3).
Autofix summary: findings and hardening items fixed in place across 3 rounds; no commits (no git in this tree). The per-item list is in the hand-back report to the coordinator. Regression tests: internal/{cli,client,mcp,store}/unifi_review_test.go and cmd/unifi-pp-mcp/unifi_readonly_test.go.
Convergence: stopped at the round 3 cap. All round 3 findings were fixed with tests, but no independent review ran over those last fixes or over the later user-decided changes. Items 2, 3 and 5 below remain open.
Post-fix /simplify: skipped. It is diff-based and this run forbids git commands.
Native timeout-boundary check: passed. Hand-written commands import only internal/client, store, config and cliutil; live commands use flags.newClient, and local store commands use boundCtx.

## Surface to user (real tradeoffs, undecided)
1. Decided: salted digests. Implemented; see .printing-press-patches/unifi-mirror-secrets.json. Live responses from unifi_read and raw still carry whatever the API returns; only stored forms are covered.
2. Medium. Remote MCP authentication is one shared bearer token, with no identity, scopes, expiry or lockout (cmd/unifi-pp-mcp/main.go). Category: requires a user decision.
3. Medium. UNIFI_CONSOLE_ALLOWLIST does not cover Site Manager, including carrier writes, nor mirror rows through sql/search, nor an operator-configured gateway with no console named. Category: two valid fixes.
4. Decided: MCP callers are pinned to UNIFI_CONSOLE_ID unless UNIFI_CONSOLE_ALLOWLIST lists the console. Implemented. With no console configured there is still no pin.
5. Low. raw GET is treated as a read by both read-only switches; nothing proves every console GET is free of side effects.
6. Decided: reach --probe-writes over MCP requires confirm boolean true. Implemented.

## Out-of-scope retro candidates (generator-reserved; not patched)
- internal/mcp/cobratree/shellout.go blockedRootFlags has no way to learn CLI-specific route flags. Worked around with mcp:read-flags on root, whose name describes file paths. Machine fix: a generic mcp:blocked-flags annotation.
- internal/mcp/cobratree/shellout.go cliArgsFromMCP renders non-boolean values for bool flags as --flag=value, so confirm:"true" becomes a real confirm. Worked around in the child. Machine fix: reject non-boolean values for bool flags using the flag type.
- internal/mcp/cobratree/shellout.go cliArgsFromMCP renders boolean true on a string flag as a bare flag that swallows the next token. Found by the round 2 reviewer; no guard bypass found.
- internal/cliutil/verifyenv.go IsMCPReadOnlyEnv honors exactly "1", which fails open on "true". Worked around in cmd/unifi-pp-mcp/unifi_readonly.go.

## Template-shape retro candidates (not patched)
- internal/client/client.go dryRun prints request bodies unscrubbed; internal/mcp/code_orch.go preview echoes body unscrubbed. Caller's own input, but it lands in transcripts.
- internal/client/client.go doInternalRaw: a same-origin 301/302 turns a POST into a GET, and a 200 then counts as applied. Traced only. change apply read-back catches it; raw and audit do not.
- internal/client/client.go StreamingHTTPClient(c.HTTPClient, ...) is a hazard for any multi-client CLI. Fixed here as a dual-transport seam.
