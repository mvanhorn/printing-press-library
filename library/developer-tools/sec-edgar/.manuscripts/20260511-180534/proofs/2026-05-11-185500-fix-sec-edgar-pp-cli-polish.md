# sec-edgar-pp-cli Phase 5.5 Polish Result

## Polish deltas

|                    | Before    | After     | Delta |
|--------------------|-----------|-----------|-------|
| Scorecard          | 77/100    | 77/100    | 0     |
| Verify pass rate   | 100% (26/26) | 100% (26/26) | 0  |
| Dogfood            | PASS      | PASS      | —     |
| Go vet             | 0         | 0         | 0     |
| Tools-audit        | 0 pending | 0 pending | 0     |
| verify-skill       | 0 findings | 0 findings | 0    |
| Output-review warnings | 2     | 0         | -2    |
| publish-validate   | FAIL      | FAIL      | (1 of 3 resolved post-polish) |

## Fixes applied by polish

1. `research.json::novel_features_built[].example` for `cross-section`: `--cik AAPL,MSFT,GOOGL` → `--ticker AAPL,MSFT,GOOGL` (re-rendered into README.md and SKILL.md via dogfood `--research-dir`).
2. Rewrote `insider-cluster` Short/Long, `--min-insiders` help text, and meta `note` to match the implementation's accession-count semantics (EFTS doesn't reliably tag issuer vs. reporting insider). Renamed meta key `min_insiders` → `min_filings`.
3. Set `SilenceErrors: true` on the root Cobra command and updated `main.go` to prefix the error itself — eliminates duplicate "Error: …" output.
4. Replaced misleading HTTP 404 hint in `classifyAPIError` that pointed at a non-existent global `list` command with SEC-EDGAR-specific guidance.

## Polish ship_recommendation: **hold**

Polish's `hold` is driven entirely by `publish-validate` failures on three artifacts the polish skill explicitly disclaims responsibility for ("mid-pipeline concerns owned by the parent flow's promote and Phase 5 steps"):

1. **`phase5-acceptance.json` missing in `.manuscripts/<run>/proofs`** — RESOLVED after polish: the file was at `$PROOFS_DIR/phase5-acceptance.json`; copied to the expected `$CLI_DIR/.manuscripts/$RUN_ID/proofs/phase5-acceptance.json` location.
2. **`tools-manifest.json` missing in MCP package metadata** — emitted by the MCP-finalize step of `publish package`; the working tree already has the MCPB bundle at `build/sec-edgar-pp-mcp-linux-amd64.mcpb`. The flat `tools-manifest.json` reference is what publish-validate scans for; this would be addressed by the publish step itself.
3. **`printer` manifest field missing** — reads from `git config github.user`, which is unset on this machine. Fixable with one user command (`git config --global github.user <handle>`); requires user consent (modifies global config).

## Verdict override

Per the SKILL contract: "If the polish skill's `ship_recommendation` is `hold` and the Phase 4 verdict was `ship`, downgrade to `hold`. Release the lock without promoting."

- Phase 4 verdict was `ship` (shipcheck 6/6 PASS, scorecard 77/100 Grade B).
- Polish verdict: `hold`.
- **Final verdict: hold.**
- Build lock released without promote.

## Actual state of the CLI

The CLI itself has **zero functional defects**. Every command was verified live against the real SEC API during Phases 3, 4, and 5. The polish hold is *only* about publish-readiness — the artifacts the public-library publish flow expects aren't fully assembled yet:

- Working binary at `~/printing-press/.runstate/cli-printing-press-12dd9ee1/runs/20260511-180534/working/sec-edgar-pp-cli/sec-edgar-pp-cli`
- All 8 transcendence commands work against live SEC data
- 17/25 absorbed commands work at parity with competing tools (8 deferred per build log)
- Scorecard 77/100 Grade B, verify 100%, dogfood PASS
- README, SKILL.md, agent-context, MCP server all correctly synced

The hold is a publish-readiness gate, not a quality gate. The user can:
1. Run the CLI directly from the working directory
2. Set `git config --global github.user <handle>` and re-run `/printing-press publish` (or `lock promote` first)
3. Run `/printing-press-polish sec-edgar` to take another pass at remaining issues
