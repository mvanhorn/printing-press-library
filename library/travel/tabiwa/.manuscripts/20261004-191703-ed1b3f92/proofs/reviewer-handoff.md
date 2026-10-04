# Stable tabiwa reviewer handoff

Review phases 14–17 using the sole fresh reviewer context /root/tabiwa_reviewer. The builder has not entered or completed those phases. Current receipt next: 14-agentic-skill-review. Read source AGENTS.md and current phase/receipt rules. No agents, nested Codex, global updaters, source booking/payment/account operations, or source access-control bypasses.

## Exact paths
Source: <source-project>
Run: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92
Run environment: <press-workspace>/run-env.sh
State: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/state.json
Phase ledger: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/pipeline/phase-receipts.jsonl
Research brief: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/research/tabiwa-brief.md
Approved manifest: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/research/tabiwa-absorb-manifest.md
Spec: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/research/tabiwa-spec.yaml (bundled copy spec.yaml)
Research metadata: <press-workspace>/.runstate/tabiwa-cli-dcb60d1f/runs/20261004-191703-ed1b3f92/research.json
CLI: <source-project>/build/stage/bin/tabiwa-pp-cli
MCP: <source-project>/build/stage/bin/tabiwa-pp-mcp
Bundle: <source-project>/build/tabiwa-pp-mcp-darwin-arm64.mcpb

## Exact approved scope and limits
Four catalog commands plus geography list: bounded dated regional discovery; payment-unit comparison; overview restriction/redemption cues; requested-date catalog membership; saved selected observations. RegionId is a documented non-auth display preference (10 せとうち,20 北陸,30 山陰,40 九州). Public catalog and region reads work repeatedly. Full /eticketDetails HTTP enters Queue-it and is excluded. No account or queue cookies are imported. Canonical product URLs are handoffs only.

JPY quotes and WESTER_POINT quotes stay separate. PointOnly is source bool or null; yen quotes do not establish cash-only terms. Unknown quote bases stay unknown except explicit per-vehicle evidence. Date membership never establishes eligibility, operation, seats or purchasability. Availability and full redemption terms/included routes are always unknown; original bounded Japanese overview cues and area tags can conflict and must remain visible. No points/cash conversion, savings calculation or geographic route inference.

HTTP response 8 MiB/1000-record bound; search scan cap default500/max1000 independent from output default10/max50. Inspect one ID, compare2–5 distinct IDs/max2 requests. Source errors and date-read failure error before fabricated facts. Cache has50 selected (region,id) observations,24 KiB per payload; normal SQLite transactions, explicit save only, newest observation replaces older. Saved reads use existing read-only database, no migration/network/create for absent storage. No adversarial filesystem redesign.

## Implementation and review focus
Custom source: internal/tabiwa/source.go and saved.go. Custom command logic: internal/cli/tabiwa_catalog.go and tabiwa_saved.go with scaffold constructors. MCP must use these same callbacks: two generated raw registrations were removed from internal/mcp/tools.go so the existing Cobra mirror owns catalog_search/geography_list. This fixes an actual smoke mismatch that ignored region and source normalization. MCP comparison uses ids comma-separated string (the generated schema uses strings for Cobra slices). Provider ops are GET only; --save is local-write annotated. Patch ledger records preserved behavior and touched files.

Review source errors vs empty lists, points-only/null/currency handling, date membership vs stock, excerpt truncation/unknown policies, region preference vs provider IDs, saved cache bounds/read behavior, CLI/MCP parity and honest docs. Source original names and labels are Japanese. Temporary proof homes and traffic are private under pipeline; curated live samples contain only normalized public facts.

## Completed proofs
Seven-leg shipcheck PASS; verify31/31, score 91/A; five approved behaviors built. Full Go tests/vet/build on stable source PASS; focused source/CLI/MCP tests PASS. SQLite actual version: 3.53.4 (shared modernc/sqlite1.60.1/libc1.77.1 pins). Bundle rebuilt from matching peers.
- proofs/shipcheck.json, dogfood-structural.json, verify-skill.json, sync-param-drop.txt
- proofs/live-feature-samples.json: 10 actual successful CLI payloads, across four regions; nonempty selected point/cash/Naoshima-exclusion/date/save evidence, plus truthful missing-product case
- proofs/mcp-live-samples.json: all five actual live domain tools PASS on final peers; search/geography region20 normalization checked, point inspection, comparison2IDs and saved2 observations
- proofs/review-source-hashes.json: exact source/peer hashes, fingerprint f1073164df268c86004d59001eee21c86b1c562be7561cf216068f89e425f4be
- pipeline/final-tests.log, proofs/tabiwa-build-log.md

No full binary-owned phase18 acceptance marker has been written yet. After independent review and any focused fixes, the builder will run the full live matrix, polish/promote/archive, install the CLI/MCP/skill, and own authorized publication plus fresh publish-time live validation and current-head Greptile/CI readiness. No PR, installation or final completion is claimed.
