# Phase 17 independent code/security review — round 1

HOLD — source cache chronology and per-CLI metadata/docs require correction before final approval. Source remains frozen: all 170 source hashes and all three staged/bundle artifact hashes match the initial handoff; fingerprint f1073164df268c86004d59001eee21c86b1c562be7561cf216068f89e425f4be. No implementation was edited by the reviewer.

## Actionable findings

1. **R1 / P2 / blocking: preserve the newest saved source observation.** `internal/tabiwa/saved.go:67` uses an unconditional conflict update. A request that fetched earlier but saves after a newer request can overwrite the newer quote, restrictions and source clock. An isolated Go overlay regression fails: save 12:20 newer source facts, then delayed 12:19 older source facts, then read returns the 12:19 row. Guard the transactional conflict update by source observation time and add a behavioral regression for older and newer incoming saves. Preserve normal SQLite transactions, existing payload/record bounds and read-only reads. Overlay probe lives at `<temporary>/tabiwa-reviewer-clock_test.go`; overlay map `<temporary>/tabiwa-reviewer-overlay.json`. Command: `go test -overlay <temporary>/tabiwa-reviewer-overlay.json ./internal/tabiwa -run TestReviewerOlderObservationCannotReplaceNewer -count=1`.

2. **R2 / P2 / blocking: make MCP context describe this catalog.** `internal/mcp/tools.go:899,919-925` advertises typed endpoint tools, cursor/after paging and a default page size of 100. Actual domain tools are the Cobra mirror callbacks, there is no after/cursor flag, default output is 10/max50, and local scan cap is separate. Runtime context must give region preferences/provider IDs, catalog-only date membership, points/JPY separation, overview/full-detail limits and honest saved-store vs generic SQL/search distinction. Do not change the unrelated generic framework or source scope.

3. **R3 / P2 / blocking: refresh the emitted MCP metadata.** `.printing-press.json` still reports mcp_tool_count/mcp_public_tool_count 2; `tools-manifest.json` describes only two raw endpoint handlers with array/string-price responses and lacks region, query, scan/output limits, inspection, comparison and saved evidence. Actual tools/list has 27 total tools and five domain tools: catalog_search, catalog_inspect, catalog_compare, catalog_saved, geography_list. Define total/domain count meanings and write metadata from the current runtime schemas/normalized response shapes. The ids schema is a comma-separated string, not an array. Keep generated novel-feature descriptions aligned through research.json if any content changes.

4. **R4 / P2 / docs correction: use the actual config model.** `README.md:206` and `SKILL.md:206-207` advertise config.toml, credentials.toml and a legacy-secret migration that this no-auth generated CLI does not implement. `internal/config/config.go:92,100` and actual doctor report config.json. State the actual config file and remove unsupported credential/migration prose. Also close the earlier phase15 warnings: README.md:323 contains empty backticks as the default config path; README.md:330 refers to nonexistent root list rather than catalog search/geography list. These are surrounding template text, not a change to the approved research feature descriptions.

5. **R5 / artifact metadata warning: restrict the native bundle compatibility claim.** The actual darwin-arm64 MCPB contains only two Darwin arm64 binaries, but its manifest advertises darwin/linux/win32. Its platform declaration should match the target contents; retain separate platform support for separately built target bundles. This does not require a generator/framework redesign.

## Positive verification and limits

Phase14 SKILL capability alignment passed. Phase16 output plausibility passed with eligible successful scorecard samples and real supplemental domain outputs. Independent public/CLI/MCP proof passed 25/25 assertions across eleven probes; J0001900 point-only/no-card/QR facts and J0000900 Naoshima exclusion versus area tag were checked against ordinary public HTTP. Future date membership stayed not_listed with unknown stock. Saved reads preserved source clocks and missing storage created no home.

Three additional controlled actual-CLI probes passed: dated second-read HTTP503 exits5 with no partial comparisons; --timeout30ms reaches the sibling HTTP request and exits5 on deadline; invalid region exits2 before any source request. These local fixtures are failure checks, not counted as live product coverage. Evidence: reviewer-failure-probes.json.

All hand-written live callbacks call boundCtx before the first sibling request; saved reads pass bounded context into SQL. Provider calls are GET-only and supply only the documented region preference; redirects are refused, source response and identity failures are errors, and missing catalog facts never become stock. Query SQL uses bound parameters, source/saved payloads and record sets have explicit limits, reads use mode=ro without migration, and saves use one connection and normal transactions. No additional custom-source security finding found. No adversarial filesystem work or unrelated generic store repair is requested.

Actual extracted bundle peers match staged peers byte-for-byte. Both extracted build-info records pin modernc/sqlite1.60.1/libc1.77.1 on Go1.27.1/darwin-arm64; the stale-bundle dependency issue found in another source is absent here. Evidence: reviewer-artifact-audit.json and reviewer-frozen-round1-hashes.json.

## Review accounting

Autofix summary: 0 implementation changes by reviewer; the builder owns routine fixes under the explicit sole-reviewer/frozen-source contract.
Review path: direct review by the already assigned sole fresh-context reviewer, covering correctness, security, maintainability and domain/output contracts; no nested agent or Codex invocation.
Template-shape candidates: stale generic MCP context, raw tools/count metadata, config/TOML prose and native bundle compatibility declarations; recorded here and requested as per-CLI corrections under root's explicit instruction. No global updater or upstream publication/issue was performed.
Out-of-scope repairs: none requested; internal/cliutil and internal/mcp/cobratree were not modified.
Convergence: round1 has actionable findings; awaiting builder fixes and same-context focused recheck. No final approval or release/publish action is granted.
