# Scrape Creators CLI Shipcheck (Reprint)

## Verdict: ship-with-gaps (full-dogfood example gap, documented below)

## Shipcheck legs — 7/7 PASS
verify PASS · validate-narrative PASS · dogfood PASS · workflow-verify PASS · apify-audit PASS · verify-skill PASS · scorecard PASS

## Scorecard: 91/100 — Grade A (prior published: 89%)
Key gains from the accepted MCP enrichment (164-tool surface → Cloudflare pattern):
- MCP Surface Strategy: 2 → 10
- MCP Remote Transport: 5 → 10
- MCP Tool Design: 5 → 10
Auth Protocol 10/10 (canonical SCRAPECREATORS_API_KEY). Path Validity 10, Sync Correctness 10, Breadth 10, Workflows 10.
Remaining sub-10s (expected): Cache Freshness 5/10 and Data Pipeline Integrity 7/10 — deliberately NOT raised via cache auto-refresh because the API is credit-metered (pre-read refresh would burn the user's credits). Insight 7, Type Fidelity 4/5, Dead Code 4/5 (generated writeNoop — retro candidate).

## Fixes applied this phase
1. verify-skill (was 22 errors): added positional placeholders to novel command Use: fields (find <handle>, compare <handle> <handle>..., track <handle>, spikes <handle>, search <query>, triangulate <query>, monitor <brand>).
2. validate-narrative (was 3 failures): corrected research.json narrative — account credit-balance → account list; removed non-functional `sync --resources transcripts` examples (this param-required API populates the store via cached reads, not sync); fixed the transcripts-search troubleshoot.
3. Fan-out performance (live-probe 10s timeouts): parallelized creator find / trends triangulate / ads monitor / creator compare with goroutines + per-sub-request 8s bound. Live timings: creator find 5.5s, trends 8.1s (slow reddit cut at cap → fetch_failures, command still completes), ads 6.6s, compare 4.0s.

## Sample Output Probe: 7/8
Only miss: transcripts search returns [] because the probe env has no cached transcripts (missing-mirror guard, behaviorally correct). All other novel features return real correct output, independently verified live (creator find matrix 768M followers across 11 platforms; account budget runway; ads monitor nike baseline; creator compare engagement rates; trends triangulate leading platform).

## Patch watch-list — final reconciliation
- #1 auth set-token credential field: current machine writes canonical SCRAPECREATORS_API_KEY via api_key path; auth_protocol 10/10; doctor PASS. Resolved upstream.
- #2 typed-table column dedupe: N/A — generic resources table architecture; bug class structurally absent.
- #3 Go 1.26.4 floor: go.mod declares go 1.26.4. Satisfied.

## Known Gaps (Phase 5 full dogfood)
Full live dogfood: 414/541 pass. 100% of the 127 failures are generated endpoint
`--help` examples missing required parameter values (bare argless call → upstream 4xx).
Zero novel-feature, framework, or structural failures; every endpoint returns 200 with
real args. This is a generator example-synthesis gap (documented in README ## Known Gaps
and filed as the dominant retro candidate), not a CLI defect and not a regression vs the
published version.
