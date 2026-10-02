# HELLO CYCLING CLI final report

Status: **COMPLETE — verified, independently reviewed and atomically promoted locally.** Printing Press phases 1–21 are closed. GitHub publication is deferred to the parent task under the user's separate publication instruction; no GitHub writes occurred.

- Source project: /Users/zjsng/Projects/Personal/Coding/hello-cycling-cli
- Promoted project: /Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling
- Canonical host CLI: /Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling/hello-cycling-pp-cli
- Stdio MCP: /Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling/build/stage/bin/hello-cycling-pp-mcp
- Host MCP bundle: /Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling/build/hello-cycling-pp-mcp-darwin-arm64.mcpb
- Archived manuscripts: /Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/manuscripts/hello-cycling/20261002-224800-fc83d07b
- Run: 20261002-224800-fc83d07b; Printing Press 4.32.5; Go 1.27.1; owner/printer/creator zjsng.

## Working scope

Station find/show, explicit-coordinate nearby pickup/return discovery, compatible pickup/dropoff comparisons, vehicle-class and first-party special-rule constraints, published regional/model pricing tables with municipal exceptions, official map/app handoff, explicit atomic SQLite snapshots and observed changes. Stable station IDs, Japanese names, coordinates, feed timestamps/TTL and UTC/JST observation/evaluation times are preserved. Unknown counts are null; empty, full, closed, uninstalled, incompatible, stale and source-missing states remain distinct.

Actual provider-published GBFS 2.3 supplies station information/status and generic electric-assist class compatibility. A class does not identify a specific bike model or establish electric-cycle eligibility or its rate. Pricing retains source model columns and unset cells, with no inferred station-bike quote. Electric-cycle age/document/traffic-test constraints are checked against the current official page. Counts are observations, not reserved availability; access distances are straight-line. The official app confirms actual vehicle, return eligibility, prices and pickup/dropoff actions. No location permission, account creation, reservation, ride or payment action is implemented or performed.

Source contracts: https://api-public.odpt.org/api/v4/gbfs/hellocycling/gbfs.json and its advertised feeds; first-party publication https://note.com/openstreet/n/n2f4b51cd52b3; https://www.hellocycling.jp/getting-started/types-and-stations/; https://www.hellocycling.jp/price/ and advertised regional pages. Public GBFS attribution is HELLO CYCLING / OpenStreet Co., Ltd. via ODPT, CC BY 4.0 selected from its published license. Runtime does not use the observed 42.7 MB website map payload or individual bike/admin links.

## Verification and review

- All seven canonical shipcheck legs PASS; verify 100% (26/26), scorecard 81/100 (A).
- Actual final full live dogfood PASS: 128 executed checks, zero failures; 81 explicitly skipped framework/auth/mutation cases. All five planned domain feature paths exercised. The binary-owned source-fingerprint acceptance marker was accepted by atomic promotion.
- Full Go tests and go vet PASS. Consequential tests cover states, freshness/future timestamps, Japanese normalization, compatibility/limit-one pairs, coordinate/radius validation, source controls and 429, redirects/body caps, source selectors/fallback, SQLite URI/size limits and municipal/unset pricing.
- Exactly one fresh-context gpt-6.1-sol MAX reviewer independently built CLI/MCP, replayed live public metadata, tested source/fallback/compatibility rules and response/SQLite bounds, and verified fixes across three rounds. Final result CLEAN PASS, no remaining review findings.
- Tools audit: zero pending (two simple generated local-framework Shorts individually accepted). PII audit: zero findings. Archive secret scan: no unresolved finding; unused provider-public contact fields omitted from the archival system-information sample.
- Gosec 2.26.1: zero hand-written-code findings; 38 generated framework/template findings retained as upstream retro candidates, not presented as a clean security scan. Generated unbounded metadata reads are bounded by the shared preserved transport hook in both CLI and MCP.
- Post-promotion smoke: canonical promoted host CLI loaded the attributed offline station 5112 and returned the correct bounded meta/results JSON envelope with local provenance.

Benchmark: live stations find for 新宿, five rows — 1.52 seconds wall time, 66,125,824 bytes maximum resident set, four GETs / 12,391,344 response bytes and 9,028 output bytes. Station calls use at most four GETs capped at 16 MiB each; pricing at most two 1 MiB GETs; raw metadata 256 KiB; default whole-call context 60 seconds, no automatic retries. No caller coordinates are sent upstream. Trip comparison outputs at most ten pairs from at most 121 candidate combinations.

## Browser use and resolved tooling limitations

Reused parent native Chrome anonymous Shinjuku/marker evidence, inspected the public map in an isolated native Chrome tab, then derived the exact website request from observed public assets and verified the provider-published GBFS contract. A native CDP capture stalled for roughly 33 minutes; further CDP sessions were avoided after the user's instruction. No app approval boundary was bypassed and no Playwright fallback was needed. One broad forced-regeneration action was rejected by automatic approval review because it risked overwriting hand-written work; the existing preserved client hook safely completed the transport fix without regeneration.

Detailed proof: .manuscripts/20261002-224800-fc83d07b/proofs/hello-cycling-independent-review.md, hello-cycling-polish.md, full-live-dogfood-final.json, phase5-acceptance.json, phase-4.95-findings.md and phase-receipts-final.jsonl. The same current research/proofs/discovery are embedded in source and promoted projects and archived separately.

## Example

```sh
/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling/hello-cycling-pp-cli stations nearby --lat 35.697315 --lon 139.704995 --purpose pickup --vehicle-type 2 --limit 5 --agent
/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/hello-cycling/library/hello-cycling/hello-cycling-pp-cli pricing show --area tokyo --agent
```
