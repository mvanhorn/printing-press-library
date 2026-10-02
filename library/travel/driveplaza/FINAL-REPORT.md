# Drive Plaza CLI final report

Status: COMPLETE — reviewed, built, verified, atomically promoted locally and archived. All required Press phases through21 completed; lock released. No publication or GitHub writes.

Working source: `<CLI_DIR>`. Promoted local library: `<CLI_DIR>`. Run: `20261002-003344-620149db`; Press4.32.5, Go1.27.1.

## Delivered scope

All 12 approved absorb rows and five practical novel rows are implemented. Nine planning leaves: interchanges, roads, route, sapa list/detail/facilities, notices, handoff and conditions. Three low-level reference pages are exposed through parsed, bounded CLI and MCP handlers. Stable source IDs/URLs, Japanese names, conditional JST/vehicle/ETC assumptions, direction-specific availability, weekday source hours, source dates and null unknowns are retained. Missing/invalid source contracts fail explicitly. No discount formulas, active-closure inference or account mutations.

Public standard HTTP runtime; default lists10/max30, local offset paging, separate default500/max2000 SA/PA scan effort, 2MiB decoded/plain response limits, 40-request domain budget and whole-command timeout. Shared domain limiter at most2requests/second. Compact domain JSON preserves provenance and complete results; --select projects fields. Real empty tabular output stays empty.

## Verification

- Complete Go tests and go vet pass. Deterministic tests cover source toll columns/zero/nulls, requested date/vehicle/time-kind/exclusion echoes, directional/icon identity, invalid source pages, bilingual/detail parsing, projection/empty output, request/body/decompression caps, timeout and typed rate-limit paths.
- Canonical post-review shipcheck: all seven legs PASS (`shipcheck-post-review.json`). Mechanical/mock verifier29/29, 100%; separate real-source proof is below. Six-step primary workflow passes. verify-skill24recipes, zero findings.
- Full runner-owned real command-tree matrix PASS:113mandatory checks, zero failures;91 explicit skips/unverified cases (204 total recorded). Every planning leaf and reference page had real happy-path/JSON tests. Skips largely concern no positional error case, framework/local mutations or fixture heuristics; they are not counted as passes. Three reference dry-run static skips were separately verified with exit0/validJSON/noHTTP. Six skipped local-learning candidate checks have temporary-store lifecycle tests and a generator fixture-planning retro note. No flagship/provider feature is blocked.
- Independent focused live proof:17checks/20requests;8extra variants;4MCPchecks. All five vehicle classes, arrival/departure, priorities/payment columns, road exclusions, five waypoints, directional/filter/detail/scan semantics, dated advisory unknowns, empty output, selection and invalid-input/timeout cases pass.
- Final live scorecard78/100 (B), with five real passing samples/zero failures. Actual runtime MCP catalog29tools; ten planning tools have read-only hints. Zero pending tools/PII audit findings; two generated framework thin descriptions individually accepted with rationale.
- Pinned gosec v2.26.1 scanned102files/35043lines, raw29findings before/after, all generator-emitted or reserved-framework candidates. Zero unresolved hand-authored findings. Generated findings are retained by rule/path in `polish-gosec-triage.json`, not claimed repaired. Structural dogfood WARN covers generic helpers, intentional stateless no-sync routing and a false command-depth warning; actual owned live matrix/MCP/workflow passes.
- Exactly one fresh-context gpt-6.1-sol MAX reviewer: Skill/Docs/Output/Code PASS. Ten initial findings plus one metadata wording clarification fixed in three rounds with the same reviewer. All five eligible passing output samples independently examined. No additional agents, codex exec, publication, GitHub writes, purchases or account changes.

Representative17-check matrix maxima:5172output bytes,3requests/command,1703ms latency,30113792bytes peak RSS (about28.7MiB). Across four MCP calls, maximum12215output bytes. These are observed samples, not worst-case guarantees.

## Access and data limits

Native Chrome through cua_repl was used for browser-first public discovery after in-app browser availability failed; no Playwright fallback was needed. Limited native schedule CDP evidence plus sanitized public HTTP replay supports the runtime. No login, CAPTCHA or genuine auth gate was encountered. Restricted-shell DNS/local-listener failures were harness limitations, resolved through approved public read-only/network and local-test execution.

English code-entry endpoints mentioned by the provider JS return404; they are excluded as route entry. Japanese code lookup works and may omit road/English-name fields. Routes use exact English IC names resolved by interchanges. Quotes remain conditional estimates; considering-traffic time is the provider's planned estimate rather than a live-now guarantee. Non-East facility source data is dated2006-03-31. Weekday hours do not prove holidays/open-now availability. RSS notices are not a comprehensive active-restriction inventory; current status remains null and canonical official handoffs are provided. Partial Japanese enrichment warns without replacing missing data with guesses.

## Evidence

Working tree: `evidence/MAX-REVIEW.md`, `evidence/deterministic-tests.log`, `evidence/live/acceptance.json`, `evidence/live/variants.json`, `evidence/mcp-acceptance.json`, `evidence/mcp-catalog.json`, `evidence/reference-dryrun.json`, `evidence/package-refresh.json`, and `evidence/research.md`.

Gate proofs: `<RUN_DIR>/proofs`. Final runner marker: `phase5-acceptance.json`, source fingerprint `cf501c00b5ebd46f4a4561e17b768a338f1d9ccb8415e7b0e9ce40d6e75f3e53`; final owned results: `20261002-dogfood-final.json`. Embedded promotion proof: `.manuscripts/20261002-003344-620149db/proofs/`. Detailed polish/acceptance/review and before/after audits retained there.

Archived research/proofs/discovery: `.manuscripts/20261002-003344-620149db`. Disposable phase receipts remain at `<RUN_DIR>/pipeline/phase-receipts.jsonl` and are not archived. Patch records include files and call sites for regeneration durability.

Promotion proof: `promotion-verification.json` records matching staged/package CLI+MCP hashes and a successful zero-HTTP eight-destination handoff smoke from the library copy. Permanent creator handle is zjsng. Final receipt: sequence39, phase21 completed; chosen next step Done locally under the explicit no-publication instruction. Source-bound final acceptance remains PASS in working, promoted and archived copies.
