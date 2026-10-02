# Dedicated review — final confirmation round 3

**PASS — actionable findings cleared at round 3.** The same dedicated fresh-context gpt-6.1-sol MAX reviewer completed all three rounds. No additional agents, `codex exec`, source edits, bookings, passenger submissions, payments, account operations, or GitHub writes were used by the reviewer.

Rechecked F1–F5, W1, N1/N2 together against the final source, independent tests/overlays, fresh reviewer-built CLI/MCP binaries, actual stdio runtime descriptors, approved research and documentation. Included the final four Short refinements and the agreed quote fare-scope clarification. No further actionable source finding remains within the evidence-supported scope.

## Convergence evidence

| Contract | Final result |
| --- | --- |
| Requested service day and source identities | PASS. Source substitutions and outside-window fallback rows are suppressed first; remaining rows must match requested course/direction/service day. Wrong day/route/direction and outside-window tests pass, as does the independent original wrong-day overlay. |
| Provider rate/timeout/resource handling | PASS. All provider constructors receive the root rate option. Auto/header observation, explicit ceiling, zero, finite-value validation, timed second-request pacing, typed 429 and cancellation behavior are supported by source and passing tests. Every live command retains `boundCtx`, bounded bodies and same-provider HTTPS redirect checks. |
| Five-workflow discovery | PASS. Fresh `which conditions`, `which baggage`, and `which fares` resolve to their intended commands with exit 0. Full discovery contains all five workflows exactly once. |
| Provider MCP context | PASS. Actual context describes all five workflows, English anonymous Go HTTP, offset/limit 20–100, live-only inventory and no provider cache. Provider resource groups are unsyncable. |
| Runtime catalog/schema/hints | PASS. Fresh MCP exposes 23 tools, five provider leaves. Every complete runtime descriptor matches `tools-manifest.json`; counts and historical generation count are correctly separated. All five provider leaves have read-only/non-destructive hints. Unknown MCP parameters are rejected. |
| Description source-of-truth | PASS. Research verified descriptions, metadata planned/built descriptions, README/SKILL and actual MCP command-mirror descriptions agree. All three runtime entries include correct canonical `command`, explicit `cli_command`, and registered `mcp_tool`. |
| SQL scope | PASS. Actual tool/query help describes existing local framework/learning data and uses `sqlite_master` discovery. It promises neither provider sync nor populated `en` inventory rows. Store/error guidance and `scope_note` distinguish legacy framework resource status from learning data. SQL security gates and bounded scanning remain intact. |
| Documentation | PASS. Unsupported exclusivity prose is absent. The **entire** Install section ends at the Workflow heading and matches the canonical saved section byte-for-byte: 1159 bytes. Its local unpublished-build disclosure remains before the section. |
| Callback and final Short refinements | PASS. Independent callback overlay survives removal of direct generated catalog registration and repeated root construction without duplicates. Fresh runtime confirms the revised `bus_route`, `routes`, `learnings_list`, and `learnings_candidates` descriptions. The callback overlay still matches final root source except for the intentionally removed direct catalog call. |
| Quote fare scope | PASS. All successful Services-to-Quote paths move the inherited headline label to `headline_fare_basis` before any early return. Actual selected-fare `price_basis` is unchanged. Successful and unavailable quote tests assert that the generic conflicting `fare_basis` is absent. |

The canonical inline command-mirror list now matches the real Press sync shape; registered-tool filtering runs afterward and restores explicit CLI identities when rendering omits them. The post-sync artifact refresh reads approved research and reconciles both planned and verified metadata. Source instructions and the customization record explain the required refresh procedure rather than relying on a one-time manual description patch.

## Independent verification

- Fresh review binaries built successfully in `/private/tmp/jbo-review-round3-bin/`, including after the final description and fare-label changes.
- `go test -count=1 ./internal/jbo ./internal/cli ./internal/mcp` passed in this round. After the Short refinements, CLI/MCP packages passed again; after the fare-scope edit, the affected quote/identity/rate tests passed again. Local HTTP fixtures used the required loopback escalation.
- Original wrong-day overlay: `go test -overlay=/private/tmp/jbo-review-probes/overlay.json ./internal/jbo -run '^TestReviewer' -count=1 -v` — PASS with explicit source identity rejection.
- Callback overlay: `go test -overlay=/private/tmp/jbo-review-probes/callback-overlay.json ./internal/cli -run '^TestReviewerCatalogCallback' -count=1 -v` — PASS, including after the Short callback refinements.
- Final affected provider test selection: `TestQuoteUsesSelectedPairCapacityAndFirstAvailableService`, `TestInventoryRowsMustMatchRequestedIdentity`, `TestProviderRateLimitModes`, `TestOutsideWindowFallbackRowsStayNotOnSale`, and `TestDeclaredRatePacesRequestsAndObservesHeaders` — all PASS.
- Fresh stdio initialize/tools-list/context/quote-dry-run/unknown-parameter probe — PASS. Full final descriptors: `evidence/reviewer-mcp-tools-round3.json`; raw responses: `evidence/reviewer-mcp-probe-round3.json`; independent parity/description/installation checks and final source hashes: `evidence/reviewer-round3-consistency.json`.
- Reviewed the final live recapture helper without running it during maintenance. It uses only the five read-only provider commands, isolates local state, records resource metrics, and now requires real nonempty requested-day services and successful fare evidence for both selected quote pairs; unavailable/empty samples do not qualify as acceptance.
- Builder-owned full-suite/vet, PII and tool-audit reports were examined as supplied corroboration; the independent reviewer checks above are separate. `evidence/tools-audit-final.txt` reports zero pending findings, four resolved description findings and two accepted generator-owned framework text items.

All seven required SKILL semantic checks are PASS: capability triggers, verified-set alignment, command intent, stub/gating disclosure, anonymous auth narrative, recipe output claims and concrete wording. README/SKILL/AGENTS preserve read-only scope, anti-triggers, verified English, original names/Japanese-name nulls, source stop identity, JST 24+ semantics, one-way JPY assumptions, conservative capacity/transaction limits, unknown age/discount/gender/seat feasibility and booking handoff.

No cascading source behavior or schema issue was found. Provider public GET/session restrictions, original route/service/plan/stop identities, overnight timestamps, selected Adult/Child arithmetic, capacity lower bounds, optional cancellation failures and source policy links remain as previously verified.

## Output evidence and remaining external work

The preserved successful pre-maintenance outputs and independent native Chrome source check remain the real-source plausibility/numeric basis: full-route party total 15750 JPY; selected stops 8→9 total 15250 JPY; source boarding 25:00 becomes 2026-10-11T01:00:00+09:00 for service day 2026-10-10. They are source evidence, not fixtures and not a claim about current availability.

The earlier Press sample that classified an outage/layout error as a passing graceful-empty result remains an upstream tooling retro candidate. The indirect-source-client heuristic warning also does not establish a provider defect: all `c.Get` paths share limiter/context/body/redirect handling and optional cancellation propagates typed 429 failures. These tooling items are separate from cleared source findings; no reserved package edit is requested.

**State at initial round-three confirmation:** independent native browser evidence confirmed HTTP-200 scheduled maintenance from 2026-10-02 02:01–04:59 JST. This reviewer did not infer inventory or reopening from the clock. The genuinely successful live matrix, current output samples, binary-owned acceptance marker/receipt closure and local promotion remained builder-owned. See the post-convergence update below for the subsequently reopened source evidence. The local review PASS does not substitute for those gates.

**Review path:** one dedicated reviewer combining the required skill, document, output, correctness, security, API-contract, reliability and resource reviews. **Convergence outcome:** findings cleared at round 3; no additional reviewer is needed for this source state.

## Post-convergence help confirmation

**PASS — round-three convergence remains valid after the three Example additions.** This is a targeted continuation by the same reviewer, not another fresh-context review round.

Inspected the `bus`, `routes`, and hidden legacy `en` Example constants in `internal/cli/bus.go` and `internal/cli/promoted_en.go`. Each names an existing command and correct flags. Fresh reviewer-built CLI help shows all three Examples sections, and each example successfully resolves under `--dry-run --no-learn`. `go test -count=1 ./internal/cli` passed independently. Both fresh reviewer builds succeeded under `/private/tmp/jbo-review-post-help-bin/`.

Fresh stdio descriptors contain the same 23 tools and are identical both to the refreshed runtime manifest and the final round-three catalog. Five provider read/non-destructive hints remain correct, and hidden `en` remains absent from MCP. Provider client/quote, discovery callback, MCP implementation/context, and artifact-refresh source hashes match the final reviewed state; request behavior, defaults, SQL security and fare/date semantics have not changed.

Proofs: `evidence/reviewer-post-help-checks.json`, `evidence/reviewer-post-help-runtime-checks.json`, `evidence/reviewer-mcp-tools-post-help.json`, and `evidence/reviewer-mcp-probe-post-help.json`.

The reopened-source collector evidence was inspected: six real successful workflow files in `evidence/live-final/`, fetched at 2026-10-01 20:01 UTC (2026-10-02 05:01 JST); nonempty requested/effective service day 2026-10-10; both quotes report `fare_evidence_reported`; full-route total 15750 JPY and selected-pair total 15250 JPY. This is actual post-maintenance evidence, not an inferred reopening or fixture. The builder is rerunning canonical shipcheck and the full binary-owned matrix after these help additions; its final acceptance marker/receipts and local promotion remain pending and builder-owned.

## Post-convergence protected-section refresh confirmation

**PASS — the narrow script-only repair preserves the protected installation section.** Inspected the actual document loop in `scripts/refresh-provider-artifacts.py`: it requires the existing section to start with the unchanged canonical prefix, moves only trailing appended prose beneath the existing Workflow heading, and removes it from the Install section without duplicating prose already present under Workflow. A changed canonical prefix raises an error rather than rewriting protected instructions.

Executed that actual AST document loop against isolated temporary fixtures, without running the write-oriented refresh over source documents. Four independent cases passed: exact input remains unchanged; appended duplicate prose appears once under Workflow; new appended prose moves under Workflow; a modified protected instruction fails closed and leaves the SKILL fixture untouched. Every successful case retains the entire canonical 1159-byte section. Proof: `evidence/reviewer-install-repair-checks.json`.

Current actual SKILL bytes match the entire canonical section exactly. Fresh actual stdio descriptors still contain 23 tools and exactly match the runtime manifest; unchanged Go behavior/context source hashes remain verified. Proofs: `evidence/reviewer-install-repair-runtime-checks.json`, `evidence/reviewer-mcp-tools-install-repair.json`, and `evidence/reviewer-mcp-probe-install-repair.json`.

The builder reports the post-help binary-owned matrix now passing 92 executable checks with zero failures and 84 skips, including the primary provider checks. Its framework candidate-ID fixture skips are distinct from provider coverage. Canonical scorecard/evidence finalization, receipt closure and local promotion remain builder-owned. This narrow review did not alter Go code or infer scores from an evidence-less scorecard run.
