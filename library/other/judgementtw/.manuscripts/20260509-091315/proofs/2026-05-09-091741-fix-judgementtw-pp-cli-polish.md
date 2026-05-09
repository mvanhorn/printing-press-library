# judgementTW Polish (Phase 5.5)

Polish skill ran in forked context against `$CLI_WORK_DIR`. Result block below as emitted.

## Result block

```
Polish Results for judgementtw-pp-cli:

                    Before    After     Delta
  Scorecard:        68/100    73/100    +5
  Verify:           100%      100%      +0
  Dogfood:          WARN      PASS      +verdict
  Tools-audit:      1 pending 0 pending -1 (1 accepted with rationale)
  go vet:           0 issues  0 issues  +0

Fixes applied:
  - Removed 4 dead helper functions from internal/cli/helpers.go (extractResponseData, replacePathParam, truncateJSONArray, wantsHumanTable)
  - Deleted internal/cli/judicial_unused_anchors.go (anchor file using `_ = fn` references that did not satisfy dogfood's call-syntax detector)
  - Removed dead --plain global flag from internal/cli/root.go (only consumer was the deleted wantsHumanTable; flag had no observable effect on any command)
  - Added mcp:read-only annotation to which command (framework-skipped today, but correct classification for future-proofing)
  - Accepted missing-read-only finding on `purge orphans` in tools-audit ledger with full rationale (command name matches read heuristic but body deletes from judgments/citations/sentences/jid_refs and writes change_log; readOnlyHint=true would be a real bug per AGENTS.md)

Skipped findings:
  - live-check failures on `cites statute / cited-by` — empty local store; environmental, by design
  - live-check failure on `watch query corruption-cases` — intermittent FJUD rate-limit; environmental
  - insight 4/10 — rooted in environmental live-check failures (structural)
  - path_validity 0/10 — internal-yaml spec format SKIPs path validation (scoring proxy)
  - mcp_token_efficiency 7 / mcp_remote_transport 5 / mcp_tool_design 5 — would require spec-side mcp: block additions (feature, not polish)
  - Output Modes 9/10 — expected one-point cost of removing the dead --plain flag (gaming the scorer rejected)
  - dogfood reports auth_protocol "Uses unknown prefix" — this CLI is no-auth, so the dimension is N/A in scorecard

Remaining issues: none — all hard gates pass; remaining scorer deficits are structural

ship_recommendation: ship-with-gaps
further_polish_recommended: no
further_polish_reasoning: All hard gates pass, scorer deficits are structural (internal-yaml spec format, environmental live-check failures, spec-side mcp: block missing) and cannot be closed without spec edits or feature additions that exceed polish scope.
```

## Verdict

**ship-with-gaps** at scorecard 73/100 (below the 75 threshold for `ship` but above the structural floor). Per the polish-skill `ship-with-gaps` contract, the README has been updated with a `## Known Gaps` block documenting the five structural items: `sync` workflow gap, intentional skip of the official open-data API path, citation extraction minor noise, appeal-chain heuristic match, knowledge-link requires synced corpus, and the MCP surface scoring deficits.

## Build/test sweep after polish

- `go build` clean
- `go test ./...` all packages green (extract, judicial, store, mcp, cli, cliutil)
- `judgementtw-pp-cli --version` prints `1.0.0`
- `judgementtw-pp-cli doctor window --json` returns valid Taipei-time payload

## Retro candidates surfaced by polish (for the Printing Press maintainers)

1. **Dead-function detector requires call syntax `fn(`.** Anchor patterns like `_ = fn` or `var _ = fn` in a separate file do not register as live, so the simple "anchor it" workaround for unused-by-design generator helpers does not work; the function still gets flagged. Either the detector should recognize `_ = fn` as a liveness hint, or the generator should stop emitting helpers (extractResponseData, replacePathParam, truncateJSONArray, wantsHumanTable) when no command in the printed CLI calls them.
2. **`internal/cli/helpers.go` is generator-emitted with a DO-NOT-EDIT header but is not in the reserved namespaces** (`internal/cliutil/`, `internal/mcp/cobratree/`). Polish edits survive until next regen, at which point the dead helpers come back. The generator should either prune at emission time or polish should write to a sibling `_polish.go` file that survives regen.
