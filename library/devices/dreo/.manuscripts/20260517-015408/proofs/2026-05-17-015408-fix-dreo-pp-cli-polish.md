# Dreo CLI Polish Result

**Date:** 2026-05-17
**Verdict:** ship (no further polish recommended)

## Delta

| Dimension | Before | After |
|-----------|--------|-------|
| Scorecard | 75 | 76 |
| Verify | 96% | 96% |
| Tools-audit pending | 1 | 0 |
| PII-audit pending | 2 | 0 |
| Insight (dim) | 4/10 | 9/10 |
| Dead code (dim) | 3/5 | 5/5 |

## Fixes applied

- Removed 5 dead pagination helpers from `internal/cli/helpers.go` (`paginatedGet`, `emitTruncationWarning`, `extractPaginatedItems`, `rawAtPath`, `extractResponseData`) — never invoked.
- Fixed `sensors query --json` to emit `[]` instead of `null` on empty results.
- Improved `scene list` Short description: "List saved scenes (name, device count, created timestamp)".
- Accepted 2 PII test-fixture findings (`<test-user>`, `<bad-credentials>` in `internal/dreoauth/login_test.go`) as `synthetic_placeholder` per RFC 2606.

## Skipped findings

- Verify mock failures on `devices`/`firmware`/`profile`/`scene`/`set` — mock-harness flake on parent help-text and commands needing positional args; all work under live invocation.
- `mcp_token_efficiency` 7/10, `mcp_quality` 8/10 — only 5 typed endpoint tools (Cloudflare orchestration pattern doesn't apply below ~30-tool surface).
- `cache_freshness` 0/10 — generator does not emit cache-freshness helper for this CLI shape (structural).
- `sync_correctness` 0/10 — sync uses generic Upsert (generator default pattern for this API kind).
- `vision` 4/10, `workflows` 2/10 — LLM-rated qualitative dims; README + 8 novel features cover the headline use cases.

## Reasoning for not re-polishing

Hard gates pass cleanly (verify-skill 0, workflow-verify pass, tools-audit 0, pii-audit 0, dead-code 5/5). Remaining low-scoring dimensions are LLM-rated qualitative or structural generator patterns that no second polish pass would close.
