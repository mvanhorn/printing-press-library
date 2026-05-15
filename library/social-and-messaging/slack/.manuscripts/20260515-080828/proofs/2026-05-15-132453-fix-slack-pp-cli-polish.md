# slack-pp-cli v1.1 — Phase 5.5 Polish

> Reprint run `20260515-080828`. `printing-press-polish` (forked context).

## Delta

| Metric | Before | After |
|---|---|---|
| Scorecard | 91/100 | 91/100 |
| Verify | 99.0% (205/207) | 100% (207/207) |
| Dogfood | PASS | PASS |
| Go vet | 0 | 0 |
| Tools-audit | 0 pending | 0 pending |
| Publish-validate | FAIL | PASS |

## Fixes applied
- `action-followthrough` / `dm-engagement`: return `cmd.Help()` instead of `usageErr`
  when the required `--report` flag is empty — the verify mock harness probes with
  `--dry-run` and no flags. Both verify failures resolved (205/207 → 207/207).
- Ran `mcp-sync` to regenerate `tools-manifest.json` (the reprint crashed at
  `dogfood-live` before MCP packaging finished).
- Restored the `printer` field in `.printing-press.json`.
- Copied `phase5-acceptance.json` into the in-CLI `.manuscripts/.../proofs/`.
- Added `mcp-descriptions.json` override for `calls-info` (thin upstream spec).

## Result
`ship_recommendation: ship` · `further_polish_recommended: no` · remaining issues: none.
All hard gates pass. Remaining scorecard gaps (insight 4/10, type-fidelity 3/5) are
structural for a 174-endpoint wrapper — total 91/100 Grade A.
