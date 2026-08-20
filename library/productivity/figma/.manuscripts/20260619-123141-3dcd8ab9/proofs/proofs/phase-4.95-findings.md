# Phase 4.95 Local Code Review — figma (reprint)

Reviewer: focused correctness+security subagent over the 8 hand-authored novel command files.
Port mechanics verdict: CLEAN — every c.Get carries cmd.Context(); no missed renames; comments_audit/dev_mode/frame fully clean.

## Autofix summary
2 findings autofixed in-place (rows.Err() checks after scan loops):
- fingerprint.go readEntries: added `if err := rows.Err(); err != nil { return nil, err }` after loop.
- tokens.go loadVariablesSnapshot: same, before the no-variables guard.
Both are behavior-preserving robustness fixes (surface driver errors instead of silently returning a truncated set). Build/vet/test green after.

## False positives (NOT fixed — correct as-is)
- fingerprint.go:159 "HIGH SQL injection via sqlStr": call sites pass literal SQL only; reviewer itself noted "safe today". No user input flows in.
- orphans.go:192 "HIGH SQL injection via table name": `table` comes only from literals "teams_components"/"teams_styles"/"variables" (orphans.go:94/101/108). SQL table names cannot be parameterized; concatenating fixed literals is the correct pattern. `--kind` selects WHICH literal function to call, never the table string.

## Deferred / low-priority (not fixed; pre-existing in prior published code, dogfood-validated 92%)
- orphans.go:133/191 LOW: db.DB().Query (not QueryContext) on local store — root --timeout not propagated to these fast local queries. Store-query, not HTTP client; boundCtx rule does not apply. Minor.
- variables.go:106 LOW: inner `name` shadows the --variable flag var inside the bindings loop; reviewer confirmed harmless at runtime. Cosmetic future-hazard.
- orphans.go:133 LOW(speculative): synced_at RFC3339 text comparison — would matter only if column format differs; prior 92% dogfood indicates format matches.

## Retro candidates (machine bugs surfaced during port — file against the generator)
1. Novel command "webhooks test" scaffolded into webhooks_test.go — Go excludes *_test.go from the package build, so newNovelWebhooksTestCmd was undefined at build. Generator should emit non-_test.go filenames for novel leaf commands whose path ends in "test".
2. Generic framework "orphans" (pm_orphans.go, newOrphansCmd, "items missing key fields") collides at root with a novel command also named "orphans" — both Use:"orphans" registered to rootCmd. Generator should detect novel-vs-framework command-name collisions.
3. scorecard mcp_token_efficiency reports 0/10 despite the MCP server exposing only the thin code-orchestration surface (figma_search + figma_execute + 3 framework tools = 5 total; endpoint-mirror suppressed). Scorer appears to read the 48-entry endpoint catalog (tools-manifest.json) rather than the exposed MCP tool count. Scorer false-negative.

## Convergence outcome
Findings cleared at round 1 (2 mechanical fixes applied; remainder are false positives or deferred low-priority).

## Review path chosen
Direct reviewer-subagent dispatch via Agent tool (general-purpose, sonnet), focused correctness+security persona over the ported novel files.
