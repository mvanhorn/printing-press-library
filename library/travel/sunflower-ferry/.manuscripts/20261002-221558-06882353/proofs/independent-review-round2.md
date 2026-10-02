# Independent Sunflower Ferry review — round 2 convergence

Reviewed 2026-10-02T16:47:49.657583+00:00 by the same authorized MAX reviewer, reusing the independent round-1 context. No additional agents, browser/CDP sessions, source edits, accounts, cabin selection, holds, standby registration, bookings, payments, or GitHub writes were used. The reviewer wrote only evidence/addendum files and extracted the supplied bundle into an isolated proof directory.

## Verdict

**PASS — F1–F9 resolved; no outstanding actionable finding in the hand-written ferry implementation or final domain MCP package.** Printing Press phases 14–17 and the polish output plausibility review converge at round 2. T1/T2 remain documented generator escape-hatch candidates. This is approval of the reviewed public planning scope, not a claim that the entire generated repository has a clean security scan.

The final umbrella shipcheck has all seven legs PASS. Its persisted manifest records 31/31 structural mock checks and scorecard 86/100 (grade A). Actual provider correctness is established separately by live evidence; those mock checks were never treated as live proof.

## F1–F9 resolution

| Finding | Resolution and independent verification |
|---|---|
| F1: sailing heading links | `sailingText` excludes informational anchors while preserving strict date, route and time checks. The real final Kobe→Oita car/child result now parses SUNFLOWER PEARL, 2026-10-15 19:00 → 2026-10-16 06:20 JST, and the original 85440 JPY Deluxe fare. All six source directions return correctly identified dated sailings. |
| F2: data-source enforcement | Every owned local/live handler validates its declared strategy before IO. Reviewer invocations of both final staged and bundled CLI produce exit 2 for quote with local source and routes list with live source. Focused new command tests pass. |
| F3: vehicle availability | The source's actual six-cell Vehicle/Motorbike rows are retained as `vehicle_availability`, including source label, symbol, interpreted snapshot status and `guaranteed:false`. Focused real final car and motorcycle outputs show ○ with the correct label, and source missing indicators become explicit unknowns. |
| F4: route/terminal search | Search now includes line codes and both ports' IDs, English and Japanese names. New deterministic tests pass. Bundled MCP search for osaka-terminal2 returns Osaka–Shibushi; staged/bundled CLI search for 大分港 returns Kobe–Oita. |
| F5: provenance | Agent `meta.source` now reflects local/live annotations, with provider text in a separate field. Local registry calls and all six live scorecard samples show the correct source. Default versus agent envelopes and compact preservation are described accurately. |
| F6: stale MCP | Reviewer extracted and executed the actual final MCPB. Both executable members are present. Its ten domain tools expose current schemas and read-only hints, with destructive hints false; quote has all party, vehicle, pet, cabin, date and direction inputs. Bundled command execution works. No obsolete scaffold remains in the reviewed domain tools. |
| F7: irrelevant boilerplate | No-auth/no-sync claims and empty config paths were corrected. Local state is described as settings, profiles and learning guidance. Planning JSON/compact behavior is accurate. Low-level generated HTML and body-cap differences are disclosed in README/SKILL where agents choose an interface. |
| F8: executable placeholders | Reviewed executable examples are concrete, including which, field projections, learning guidance, direct use, AGENTS examples and xattr/chmod. Remaining angle-bracket syntax is reference notation/frontmatter rather than executable example content. |
| F9: doctor dry run | Source narrative and README now say preview the doctor action without network. This matches the real dry-run output. |

## Independent verification and inspected evidence

New focused regression checks passed independently:

`go test ./internal/ferry ./internal/cli -run 'TestKobeInformationalLinksAndVehicleSnapshot|TestRouteSearchIncludesTerminalIDsAndJapanesePorts|TestFerryCommandsEnforceDeclaredSourceBeforeIO|TestValidationAndReservationBoundary' -count=1`

The builder's full Go suite, go vet, final verify-skill and full narrative proofs also pass. Registration callbacks and shared helper paths were inspected; the dead-code remover's preview was correctly not applied.

Final package proof: `proofs/reviewer-round2-bundle.json`. The actual MCPB SHA-256 is `c8d71a1a799cec6e10f82415ad426a2db6e6b5d0cd728b1b19ede7d7c2cb899a`. It includes current CLI/MCP executables, all ten domain schemas/hints, real bundled route-search result, and staged/bundled incompatible-source checks. All five incidental `source_*` tools now describe their raw HTML return, route choices, and preferred domain command instead of implying typed planning output.

Provider evidence reviewed: `vehicle-fix-live.json`, all 18 original expanded matrix outputs, final car/motorcycle reruns, the original pilot, and all six eligible passing features in `scorecard-live.json`. The earlier scorecard's 83 reflects its pre-polish snapshot; final manifest/shipcheck gives 86. No figure was substituted for the actual provider source observation.

Live outputs establish:

- All six direction codes: 21/22, 11/12, 31/32. Their endpoints, ship names and JST arrival dates agree with the source. Normal Shibushi inbound rules preserve the separate Friday/Saturday times and source effective caption.
- The outbound October 4 E/special cruise departs 12:15 and reaches Beppu October 5 at 00:05 JST. The implementation correctly crosses midnight; E does not imply same-calendar-day arrival. The inbound October 4 calendar is A, preserving direction differences.
- New Year calendar dates retain explicit source coverage through March 31 2027, with no fabricated missing dates or reused outbound bands.
- Two adults/one child/one <5 m car retain the source total and Vehicle snapshot. One >750 cc motorcycle retains the Motorbike snapshot. The focused final runs took 4.864/4.353 seconds with approximately 26.8/26.1 MB peak RSS, 6067/5494 output bytes and five source requests.
- Toddler/infant inputs are echoed and passed to the source; no independent free-fare arithmetic or guaranteed eligibility is invented. Private-room and shared-section occupancy wording is preserved. Port identities and distinct Osaka terminals remain correct.
- Prices, calendar bands, inventory snapshots, meals and missing tax/fuel/fee breakdown have honest source caveats. Cancellation and baggage distinctions remain correct.

A final precise negative lookup completed using the final staged binary: Osaka–Beppu, 2027-01-03, one adult on foot returned exit 5 after 3.359 seconds: `official fare result has no parseable sailing; no normal schedule substituted`. See `proofs/reviewer-round2-window-boundary.json`. A focused direct replay confirmed all three source steps returned HTTP 200 ending at Reserve1020, which contained only the date heading, no cabin fare/sailing rows, and the explicit source message `Reservations not yet open for this service. Cannot display availability.` See `proofs/reviewer-round2-window-boundary-source.json`. This proves a genuine no-fare/source-unavailable outcome is rejected without invented data. The retained English source states a three-month window; no unsupported assertion about a precise booking-opening time is made.

## Output plausibility verdict

All six scorecard entries have `status:pass` and actual observed source metadata. Assessed their bounded output samples and complete corresponding expanded files. Redaction markers and scorecard sample truncation are proof sanitization, not CLI format failures.

No semantic query mismatch, visible formatting bug, silent multi-source drop, or unjustified ordering/ranking finding remains. Source order is preserved; the CLI does not claim ranking. New vehicle fields agree with actual source labels/symbols. Canonical first-party sources and booking handoff URLs are retained.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

## Preserved generator escape-hatch candidates and security assessment

**T1 remains:** generated low-level typed MCP HTML endpoints skip CLI `html_extract:page` and return raw HTML. Domain command mirrors are correct. README/SKILL and final raw-source tool descriptions disclose the distinction. The upstream MCP endpoint template should carry the same extraction configuration or mirror those CLI handlers.

**T2 remains:** generic generated `internal/client/client.go:1330` reads an ordinary response with unbounded `io.ReadAll`; its 32 MiB decoded-compression limit does not bound ordinary HTML. The native ferry client retains its independent 2 MiB limit. The upstream client template should use a bounded ordinary-body reader.

The 30 gosec findings were reviewed by rule and call context, not dismissed solely by generated-file headers. None targets the hand-written ferry client/handlers. Full scanner evidence and builder ledger remain in `gosec-before.json` and `gosec-disposition.md`; do not relabel that scan as clean.

- G101 at `internal/platform/gate.go:22` is the enum string `invalid_credentials`, not a credential.
- G119 at `internal/client/client.go:454` is behind same-origin re-stamping and downgrade-refusal guards. No source auth is configured in this CLI. Keep generic redirect/header handling in the upstream review ledger; native ferry requests independently use an exact first-party HTTPS allowlist.
- G703 at `internal/cli/teach.go:210` writes the resolved local audit/teach-log path. Query text is used for record matching/content; it is not used to construct that filename. This is a shared local log-scrubbing flow, not ferry-provider traversal.
- G201/G202 store constructions use validated identifiers or fixed clause fragments with values bound separately. In particular `internal/platform/migration.go:155–156` doubles single quotes in targetPath before the quoted VACUUM INTO expression and requires verified tenant state. That flagged query is not an unescaped SQL injection, and the function is shared platform code despite its missing standard generated header.
- G304 paths are the operator's selected local config/playbook files or app-resolved state/cache/lock paths. They remain shared-template diagnostics rather than new network-controlled ferry paths.
- G302's four flagged 0700 modes apply to directories; directories need execute permission. File writes in those inspected paths use private modes. These reports need directory-aware upstream scanner disposition.
- G117 SessionKey is a local learning correlation key (hashed harness identifier/parent PID or explicit local override), not the anonymous provider cookie/anti-forgery value.
- G104 includes ignored local-store read/dry-run encoding errors and cleanup-close errors. Record the shared reliability candidates upstream; they are outside native ferry operations and do not invalidate the current provider evidence.
- **T3 / P2 shared template hardening:** `cmd/sunflower-ferry-pp-mcp/main.go:86–89` lacks ReadHeaderTimeout for optional HTTP MCP. Add a bounded header timeout in the upstream server template without breaking streaming responses. The validated MCPB defaults to stdio; HTTP transport was not started or represented as independently hardened here.
- **T4 / upstream dead-code analysis:** `deadcode-preview.json` falsely proposes removing `attachFerryCommands`, which is invoked through registered callbacks, plus shared helper paths. The remover was not applied. Teach the Press call graph about callback registration before automated pruning; current registration and final packaged schemas prove the code remains present.

The archive secret scan passed 161 text artifacts with no unredacted portal session fields. Safety checks and the reviewed boundary remain intact. The run is ready for the builder's remaining receipt/archive/promotion work under the disclosed generator limitations.
