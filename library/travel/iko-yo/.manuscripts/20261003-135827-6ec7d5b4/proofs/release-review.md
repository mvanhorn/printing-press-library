# Iko-yo Trip release review

**Overall verdict: SHIP.** No unresolved findings remain in the reviewed release snapshot. One dedicated fresh-context reviewer completed phases 14–17 and rechecked the fixes applied by the builder.

| Required review | Explicit verdict |
|---|---|
| 14 — Agentic SKILL review | **PASS** — all seven checks assessed; triggers, examples, scope and the five built feature descriptions match the four Trip commands. |
| 15 — README/SKILL/AGENTS correctness | **PASS** — commands, flags, exits, sourcing and privacy claims match runtime behavior; no executable command placeholders or unsupported source/auth/sync claims remain. |
| 16 — Agentic output plausibility | **PASS** — **5 eligible passing samples assessed**, plus independent CLI/MCP and format checks; no unresolved relevance, formatting, aggregation or ordering findings. |
| 17 — Local code/security/privacy review | **PASS** — handwritten parser, HTTP client, cache, CLI adapters, MCP overlay and generated boundaries reviewed; reported defects cleared after recheck. |

## Verification

Reviewed the approved Trip-only manifest, research, six minimized source fixtures and browser-discovery evidence. Ordinary public HTTP through the actual shipping binaries corroborated [spot 8220](https://trip.iko-yo.net/spots/8220), [event 8412](https://trip.iko-yo.net/events/8412) and [Saitama listings](https://trip.iko-yo.net/events/regions/6/prefectures/11). Core Iko-yo is excluded; this review used no login, private API, challenge bypass or remote mutation.

Independent final checks: 17 CLI cases (15 successful reads and expected exit-3 missing-detail / exit-5 deadline cases); six successful MCP tools/call cases including all four Trip commands, context and the recipe; 9 CSV/plain/quiet cases; and two final comparison amenity-projection cases. tools/list exposed 24 tools; context, recipe and Trip mirrors all advertised readOnlyHint=true and destructiveHint=false. All assessed MCP calls returned normalized JSON.

Dated discovery scanned 30 Saitama records over two pages, found event 8412, counted two unknown schedules, and disclosed 22 remaining pages. The negative keyword case disclosed its bounded zero. The 6 months–12 years age description, child/adult JPY 300 statements and the adult race-admission qualifier survived normalization. Event 8412 retained the November 15 schedule, September 1–October 16 application interval, separate payment condition, 30-person/lottery evidence and unknown availability. Cutoff comparison distinguished October 16 from October 17; spot daily operation remained unknown.

Native command paths apply boundCtx before requests; the client also bounds redirects, host, body, per-request time, pacing and scans. HTTP access/rate errors do not become empty results or saved-detail fallback. Cache tests verify atomic replacement, preserved detail observations, removal of stale positive evidence and bounded provenance reads. No contributor profiles, article archives, cookies or credentials appeared in fixtures or normalized outputs. A profile containing no-learn=false no longer journals a successful Trip invocation; journal content remained unchanged.

Independent go test passed for internal/trip, internal/store, internal/cli and internal/mcp, including external reviewer overlay cases. The builder’s reviewed full test/vet and all seven shipcheck legs also passed in shipcheck-reviewed.json. The final small comparison projection change was inspected and verified on rebuilt artifacts.

## Resolved findings

Locations identify the current implementation or relevant generated boundary. All rows are closed.

| Severity / location | Reproduction and verified fix |
|---|---|
| P2 — internal/trip/parse.go:289 | Compound nursing/stroller/changing sentences transferred another amenity’s predicate. Negated/speculative amenities and a negated age range were also confirmed. Clause-local affirmative grammar and age guards pass all reviewer overlay cases. |
| P2 — internal/cli/iko_yo_trip_privacy.go:17 | Save a profile with no-learn=false, then run trip cached with it: previously a Trip journal entry appeared. Suppression now holds after profile application; no Trip entry was added. |
| P2 — internal/cli/iko_yo_trip_helpers.go:129,164,208 | quiet returned counts/statuses/evidence IDs, and CSV/plain selected the wrong rows. Source-specific projections now return canonical refs and identified fact/assessment rows, including requested amenity statuses. |
| P2 — internal/mcp/tools.go:849; internal/mcp/iko_yo_trip_surface.go:15 | context previously claimed cursor/after paging, sync and search. The hand-owned SDK overlay now advertises page=N, bounded Trip commands and partial saved facts; shipping entrypoint registers it. |
| P2 — internal/mcp/intents.go:31; internal/mcp/iko_yo_trip_surface.go:44 | published_family_facts was labeled destructive. SDK registration repairs readonly hints; an actual recipe call returns the normalized fact card. |
| P2 — README.md:322,357; SKILL.md:184,204,222 | Executable placeholders, generic sync/auth/header prose and unconditional envelope advice mismatched this CLI. Concrete examples and accurate no-auth, sourcing, format and relocation wording now match runtime. |
| P3 — README.md / SKILL.md Unique blocks | Unsupported categorical competitor claim removed; feature descriptions remain aligned with research.json. |

## Generator boundaries and snapshot

Generic context/SQL guidance, lifted-recipe default hints, formatter unwrapping and regenerated marketing text are upstream template candidates. Shipping fixes use source-owned adapters/SDK registration and documented durability guards; reserved cliutil/cobratree packages were not patched. The builder’s separate 31 framework/reserved gosec candidates remain in gosec-triage.json; they do not imply 31 verified vulnerabilities. No actionable reserved-package defect was confirmed by this review.

Exact binary and 26 reviewed-file hashes are in reviewer-final-snapshot.json. Evidence: reviewer-live-cli.json, reviewer-live-mcp.json, reviewer-format-final.json, reviewer-compare-amenities-final.json, reviewer-privacy-final.json and reviewer-final-tests.txt. Raw normalized outputs and isolated homes should remain private; this report contains only minimal source facts and synthetic reproductions.

```text
---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
```

## Post-signoff help and fixture recheck

**PASS; phase 14–17 verdicts and overall SHIP remain unchanged.** Inspected the source-owned endpoint Example hook and the three positional happy-args annotations at internal/cli/trip_inspect.go:20, trip_cached.go:21 and trip_compare.go:40. All six compatibility help examples are concrete and match supported flags; the three first Trip examples match the authoritative positional fixtures without adding alternative ref/refs/query flags. Actual rebuilt CLI help exposed all nine examples. The three first Trip examples also succeeded against saved facts and returned normalized JSON; no business-semantic edits were identified. Both CLI/MCP binaries are newer than the changed source files.

The updated full Press live gate reports PASS, 112 executed cases passed, zero failed, with 93 skipped/unverified cases disclosed (live-gate-summary.json); phase5-acceptance.json has status pass. Focused reviewer evidence is in reviewer-post-signoff-help.json and reviewer-post-signoff-examples.json. Exact changed-file and shipping-binary hashes were refreshed in reviewer-final-snapshot.json.
