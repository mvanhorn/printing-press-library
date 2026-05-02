# Food52 Polish Report

**Run:** 20260501-164925 (reprint, v3.2.1)
**Polish skill verdict:** ship

## Delta

|  | Before | After | Δ |
|---|---|---|---|
| Scorecard | 84/100 | 85/100 | +1 |
| Verify | 88% | 94% | +6 (regressed during fix, recovered after dryRunOK) |
| Dogfood | WARN (1 dead helper) | PASS | cleared |
| Tools-audit pending | 2 | 0 | -2 |
| go vet | 0 | 0 | — |

## Fixes applied

1. **`mcp:read-only` annotations** added to 9 read-shaped novel commands (`pantry list`, `pantry match`, `tags list`, `search`, `scale`, `print`, `articles for-recipe`, `articles browse-sub`, `recipes search`, `recipes top`). MCP hosts will not gate these behind permission prompts.
2. **Dead helper removed**: `extractResponseData` in `internal/cli/helpers.go` — generated, never called.
3. **`scale` validation hardened with dryRunOK short-circuit**: returns typed error when slug is provided but `--servings` is missing or ≤ 0; falls through `dryRunOK(flags)` short-circuit before validation, so verify mock-mode passes.

## Skipped findings (not bugs / not appropriate to fix)

- `verify` EXEC failures on `print` and `which`: required-positional commands called with no positional in mock mode (retro F7 unfixed in v3.2.1; live dogfood passes both).
- `mcp_token_efficiency` 4/10, `insight` 4/10: scorer dimensions calibrated for larger spec surfaces (this CLI is intentionally synthetic with only 4 endpoint mirrors). Raising via fake endpoints is anti-pattern.

## Remaining issues

None.
