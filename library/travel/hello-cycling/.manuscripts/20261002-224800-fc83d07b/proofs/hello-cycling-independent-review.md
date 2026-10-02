# HELLO CYCLING independent review — final fix verification

Project: /Users/zjsng/Projects/Personal/Coding/hello-cycling-cli  
Review path: the same single user-authorized fresh-context reviewer; no additional agents.  
Overall: **CLEAN PASS — no remaining review findings.** All seven initial substantive findings and the final P3 documentation warning are closed. This covers Press phases 14–17 and does not substitute for the builder’s final full-dogfood/promotion gate.

## Final documentation verification — round 3

The unsupported absolute exclusivity sentence is absent from both README.md and SKILL.md. SKILL executable code blocks contain no generic <command> placeholders; the final examples use stations show --id 5112 and its help. SHA-256 comparison against the independently tested checkpoint confirms no reviewed Go source changed. Documentation-only corrections therefore retain the prior runtime verification. Direct read-only inspection of internal/cycling/testdata/baseline.db confirms the current fixture has three station identities: 17, 1556 and 5112. The historical successful changes sample below also reports three; an interim recollection of two was corrected by the builder and required no fixture change. The builder reports the completed full live matrix at 128/128.

## Four Press review scopes

- **14 / SKILL semantics: PASS, no findings.** Trigger phrases correspond to discovery, observed returns and compatible endpoint planning. Five unique command paths match novel_features_built, runtime help and implementation. No planned-only or stub workflow is claimed. Generic class/model boundaries and app handoff are explicit; unsupported exclusivity wording was removed.
- **15 / README–SKILL–AGENTS correctness: PASS, no findings.** Nonexistent list guidance, empty config path, config.toml/credentials/auth migration prose and generic executable command placeholders were corrected. Source selectors, live-only commands, no-auth operation, shared data-directory relocation, source bounds and pending-publication instructions describe actual behavior.
- **16 / eligible output plausibility: PASS, no output findings.** Reviewed /private/tmp/hc-output-review-livecheck.json and its three genuine successful JSON samples: stations nearby, trip compare and pricing show. Ranks, canonical handoff URLs, counts, freshness and unset prices are plausible. Redacted names/addresses are privacy scrubbing. The stations changes entry contains error text despite status=pass (“graceful empty”) and was explicitly excluded. Sync was skipped because it writes local state. The earlier successful /private/tmp/hc-changes-final.json sample, observed at 2026-10-02T15:59:42.751495Z, was separately assessed: 5112 changed from one bike to zero, with baseline_station_count=3 and the original baseline observation time disclosed. That count describes this earlier sample; it does not assert the coverage of a later final-matrix fixture.
- **17 / local security and correctness: PASS, no remaining blocking finding.** Selectors, fallback boundaries, comparison candidates, relocation, URI escaping, offline pre-scan cap and raw CLI/MCP transport caps were independently verified. Provider reads remain GET-only; no account, location permission, reservation, ride or payment path was introduced.

## Independent verification

Evidence: /private/tmp/hc-final-review-aju74cbu  
Fresh binaries: /private/tmp/hc-independent-review-final-bin/hello-cycling-pp-cli and hello-cycling-pp-mcp  
Reviewed source checksums: source-hashes.json in the evidence directory.

- Fresh CLI/MCP builds succeeded. Independent go test ./internal/cycling ./internal/cli ./internal/mcp passed, including the new relative-path regression. Local mocks required ordinary sandbox escalation; no browser/CDP was used.
- Fresh deterministic A/B fixtures verified local selection without network, offline/live conflict exit 2, live-only pricing/rules/sync/changes rejection, and a valid pair at limit 1 with overlapping endpoints. See local-results.json and local-case-* outputs.
- Auto deadline fallback returned the explicit cache with local provenance, warning and request_count=0. No-cache and explicit live prevented fallback; sync/changes never fell back and preserved baseline bytes. Existing typed HTTP tests and code inspection confirm 401/403/429 remain errors. See fallback-results.json.
- HELLO_CYCLING_DATA_DIR took precedence over --home. A relative filename containing spaces, ? and # loaded correctly.
- A real 48 MiB + 1 SQLite zeroblob was rejected before returning its payload to Scan: exit 5, 0.011 seconds, 21,086,208 bytes peak child RSS. See oversized-result.json. The initial timing wrapper hit a sandbox sysctl restriction; the direct CLI rerun supplies the verified result.
- Both raw endpoints succeeded against the actual provider through fresh CLI and MCP binaries. MCP discovery/vehicles took 0.367/0.390 seconds; CLI discovery/vehicles took 0.331/0.316 seconds. See cli-live-* and mcp-live-* outputs.
- Both CLI and typed MCP succeeded against a real local identity-encoded response and rejected 256 KiB + 1 bodies. Four actual mock requests carried Accept-Encoding: identity. Existing tests also cover the exact cap and unexpected encoding. See transport-results.json.
- Independent source checks and final samples preserve Japanese identity, coordinates, app URLs, nullable unknowns and distinct operational/freshness states. Tokyo/Chiyoda/Itabashi tables retain model columns; generic class 2 does not assign a bike model or quote.

## Closure and source limits

Seven substantive findings were fixed: ignored source flags, failed raw transport, hidden limit-1 pairs, inconsistent relocation, inaccurate command/configuration prose, raw response bounds and offline pre-allocation bounds. The preserved internal/cli/hello_cycling_transport.go hook reaches both CLI and MCP client construction. It uses capped standard HTTP while retaining generated deadlines/pacing, without overwriting generator-owned code.

The generator’s unrestricted io.ReadAll remains an upstream template concern, but this CLI’s shared transport caps its raw metadata endpoints at 256 KiB. The domain client separately caps four station requests at 16 MiB each and official pages at 1 MiB. Trip output is at most ten pairs from 121 candidate combinations. Counts are observations; distances are geometric; pricing rows are model/area publications rather than quotes.

Type/dock interpretation was checked against the [primary GBFS 2.3 specification](https://raw.githubusercontent.com/MobilityData/gbfs/v2.3/gbfs.md). Generic electric-assist compatibility does not prove special electric-cycle eligibility or actual model pricing.

The builder’s /private/tmp/hc-shipcheck-final.json has all seven legs PASS. Its scorecard false positive for an absent changes baseline was excluded; the attributed public baseline and successful changes output supply separate evidence.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
