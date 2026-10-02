# Dedicated review — confirmation round 2

Same fresh-context reviewer as round 1; no new agent, `codex exec`, or source modification. Rechecked all F1–F5/W1 together, including permanent tests, original independent overlays, fresh binary/runtime probes, source-of-truth descriptions, and documentation.

**Convergence: the original behavior findings are corrected; two remaining agent-facing consistency issues prevent a clean review PASS.** Both are small metadata/help corrections. No new provider inventory, fare, capacity, timeout, security, or mutation defect was identified in the changed implementation. The builder has been informed of both issues during this round.

## Original findings

| Finding | Confirmation | Evidence |
| --- | --- | --- |
| F1 — service-row date/identity guard | CLOSED | `Services` suppresses substituted/outside-window rows before checking remaining route/direction/service-day identities. Permanent wrong-day/route/direction and outside-window fallback tests pass. The original independent wrong-day overlay now passes through explicit rejection. |
| F2 — incorrect provider MCP context | CLOSED for provider context | Fresh MCP context contains exactly the five approved workflows, offset/limit with default 20/max 100, anonymous English Go HTTP, live-only inventory and no provider cache. All four provider resource groups have `syncable:false`. See N2 for SQL tool help outside this corrected context. |
| F3 — conditions/fare discovery | CLOSED | Fresh binary queries `which conditions`, `which baggage`, and `which fares` all exit 0 with the intended command first. The full index includes all five approved commands exactly once. Source research and generated CLI/docs include the new fare wording. See N1 for two remaining description copies. |
| F4 — ignored rate option | CLOSED | Root rate settings reach all provider constructors, including routes/legacy catalog. Auto uses header observation/adaptive fallback, positive values impose ceilings, zero disables pacing, and nonfinite values are rejected. Meaningful tests prove a restrictive second request waits until its context expires without dialing, header-budget observation, modes, typed 429, and cancellation propagation. |
| F5 — stale runtime tool catalog | CLOSED for names/schema/counts | Fresh MCP has 23 tools, five provider leaves, and its complete descriptors match `tools-manifest.json` exactly. Metadata reports 23 public/runtime tools, five provider tools, and separately retains the original generation count of 1. Historical endpoint input is explicitly labeled in `generator-endpoint-provenance.json`. |
| W1 — unsupported exclusivity prose | CLOSED | Both README and SKILL omit the assertion. The complete 1159-byte Prerequisites block still matches the canonical evidence byte-for-byte. |

## Remaining actionable findings

### N1 — P2 / error: description sync remains incomplete on verified metadata and MCP context

- **Location:** `internal/mcp/tools.go:842`; `.printing-press.json` `novel_features_built` entry for `bus quote`.
- **Trigger:** compare the approved current `research.json` verified feature description with the fresh runtime MCP context and verified metadata.
- **Observed:** research, README/SKILL, root/which use `Read selected-stop Adult/Child fares, one-way party totals and capacity evidence.` MCP `command_mirror_capabilities` and metadata's built entry still use `Compare adult and child costs for exact boarding stops.` Metadata's planned entry is updated, so the two metadata sets also diverge.
- **Expected:** the description source-of-truth must propagate consistently through the verified metadata and MCP surface, as required by the Printing Press description-sync contract. Deliberate command-mirror preservation must not silently retain the prior description when an update is intended.
- **Suggested fix:** synchronize the intended command-mirror block with the explicit overwrite option, and reconcile built metadata descriptions from the current approved research. Include both checks in the post-sync refresh procedure so the drift does not return after dogfood.
- **Verification:** compare `(command,description)` maps for research `novel_features_built`, metadata planned/built entries, README/SKILL/which, and fresh MCP context. All three novel descriptions should agree with the approved research.

### N2 — P2 / error: SQL tool help still advertises the unavailable sync workflow

- **Location:** `internal/mcp/tools.go:47`–48; newly refreshed `tools-manifest.json` `sql` description/query schema.
- **Trigger:** inspect MCP `tools/list` or the actual runtime manifest for `sql`.
- **Observed:** the tool says `Requires sync first`, describes analysis of synced resources, and suggests `resources` rows with `resource_type='en'`. There is no sync tool/command or provider inventory cache in this CLI; provider context correctly says live-only. A user following this help reaches a nonexistent prerequisite.
- **Expected:** the local SQL tool should accurately describe its existing local CLI learning/store database and explicitly separate it from live provider inventory.
- **Suggested fix:** replace the inherited SQL help/query example with local-store guidance that needs no invented sync command or provider rows. Ensure any missing/empty-store guidance also names only real workflows; its current generic store-backed-command fallback is preferable to the old sync claim.
- **Verification:** rebuild and refresh the runtime catalog, then inspect `sql` description/query properties and missing/empty-store guidance. No unavailable sync/search workflow or populated provider `en` rows should be promised. Compare the refreshed manifest to the new runtime descriptors again.

## Independent checks completed

- Fresh CLI and MCP builds succeeded under `/private/tmp/jbo-review-round2-bin/`; staged release/source files were not changed by the reviewer.
- `go test -count=1 ./internal/jbo ./internal/cli ./internal/mcp` passed (local fixture binding escalation). Provider: 1.179s; CLI: 9.505s; MCP: 1.017s.
- `go test -overlay=/private/tmp/jbo-review-probes/overlay.json ./internal/jbo -run '^TestReviewer' -count=1 -v` passed. Its original wrong-day fixture now receives a clear source identity mismatch and no offered inventory.
- A separate overlay removes the generated root's direct `newRoutesCmd` registration. `go test -overlay=/private/tmp/jbo-review-probes/callback-overlay.json ./internal/cli -run '^TestReviewerCatalogCallback' -count=1 -v` passed: the hand-authored callback restores `routes list`, three root constructions each contain exactly one catalog group, and the conditions index remains single-entry. This tests the callback mechanism without writing source or running a destructive regeneration.
- Fresh stdio initialize/tools-list/context/dry-run/unknown-parameter probes succeeded. Full catalog: `evidence/reviewer-mcp-tools-round2.json`; raw responses: `evidence/reviewer-mcp-probe-round2.json`; compact comparison: `evidence/reviewer-round2-runtime-checks.json`.
- Manifest tool-name differences: none. Descriptor/schema differences: none. All five provider leaves have `readOnlyHint:true` and `destructiveHint:false`. Context advertises exactly `routes list`, `bus route`, `bus services`, `bus quote`, and `bus conditions`.
- The new maintenance/customization record lists provider files and context/discovery/rate call sites, and AGENTS.md requires rebuilding both binaries and refreshing runtime artifacts after Press sync. This is an appropriate explicit safeguard; N1 shows the refresh still needs full description reconciliation.
- The refreshed helper/probe scripts were read without executing the write-oriented artifact refresh from this reviewer. The independently saved catalog was compared with its results instead.

All seven SKILL semantic checks remain supported: triggers, verified command set, actual command intent, local unpublished/gating disclosure, anonymous auth narrative, shortlist recipe, and concrete wording. No stub, unsupported booking operation, credentials persistence, or installation-section alteration was introduced. The N1 description consistency issue must still be closed across all generated consumers.

## Tooling limitations and external wait

The source-client heuristic's indirect `c.Get` warning is a tooling limitation here: every provider GET uses the shared limiter, request/body/redirect/context bounds and typed 429 behavior. Quote's optional cancellation branch explicitly returns typed rate failures; ordinary optional fetch failures remain visible in `partial_failures`. No reserved `internal/cliutil` or `internal/mcp/cobratree` edit is requested.

The round-one Press outage-as-passing-sample issue remains an upstream retro candidate, outside source convergence. Preserved pre-maintenance real outputs and browser evidence remain the numeric/output-plausibility basis. The provider's independently confirmed HTTP-200 maintenance window is still the external constraint; this round makes no current live inventory claim. Fresh live matrix, scorecard samples, final security/tool audits, receipt closure and local promotion remain the builder's subsequent work after actual reopening.

**Review path:** same dedicated reviewer combining all required review personas/contracts. **Outcome:** original findings corrected; N1 and N2 pending. Ask this same reviewer to confirm the final reconciled source/runtime state; do not spawn a second reviewer.
