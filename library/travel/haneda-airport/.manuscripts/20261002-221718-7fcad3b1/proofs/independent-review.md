# Haneda Airport independent review

Reviewer: the single authorized fresh-context reviewer, reused for fix verification. No additional agents, browser sessions, or GitHub writes were used. Review and final verification were completed on 2026-10-03 JST.

**Final outcome: PASS for the authorized local Haneda build. No open implementation or documentation findings remain. All six initial findings and the follow-up corrections below were independently verified. Runtime release remains unstamped at `0.0.0-dev`; this verdict does not claim public publication.**

| Press review gate | Verdict | Basis |
| --- | --- | --- |
| 14 — Semantic SKILL review | PASS | All trigger phrases map to supported domain behavior. Unique Capabilities exactly match the five approved and verified `novel_features_built` commands. Descriptions/examples match behavior, anonymous auth is accurate, no stub/gated promise remains, and unsupported exclusivity prose has been removed. |
| 15 — README/SKILL/AGENTS correctness and agent readiness | PASS | Final docs agree with staged help, source semantics, bounded requests/output, cache paths, positive five-minute snapshot age policy, facilities flag, read-only scope, concrete Haneda examples, and unpublished installation status. No false import or auth-write guidance remains. |
| 16 — Agentic output review | PASS | Eligible real plan and rollover samples were assessed, together with real file-backed snapshot save/search/diff proofs and independent live alias probes. Queries are relevant, dates/URLs/UTF-8 output are coherent, requested source scopes are represented, and ordering is plausible. |
| 17 — Local code review | PASS | Initial defects and the legacy snapshot compatibility regression are fixed. Final parser, CLI and MCP tests pass. Runtime MCP mirrors all eleven domain leaves and excludes import/raw source; client bounds, source fidelity, atomic writes, and explicit unknown fields remain intact. |

## Initial findings — all closed

1. **[P1] Marketing aliases and accepted padded numbers fail detail/plan lookup.**
   - Initial locations: `internal/cli/haneda_commands.go:234`, `internal/haneda/client.go:109`.
   - Live `flights detail NH849 --date 2026-10-03` returns the original group, including `UA8003`, `AC6273`, and `TG6106`. `flights detail UA8003` and `flights detail NH0849` for the same date return exit 3 and zero matches. Full-board `flights search --flight UA8003` finds NH849 correctly.
   - The public exactMatch contract only resolves the source-primary number. Passing a marketing alias directly to it is insufficient. The test server currently masks the problem by returning the first fixture for every exact request regardless of its flightNumber.
   - Fix: resolve aliases and supported numeric-padding equivalents through a bounded source-board fallback, retain the original primary and codeshare order, and test both detail and plan against a primary-only exact-match fixture and the live source.
   - Proof: `review/live/detail-primary.json`, `detail-marketing.json`, `detail-padded.json`, `search-marketing.json`.

2. **[P2] The generated write-resource mapping advertises an import that does not exist.**
   - Initial locations: `internal/cli/resource_paths.go:32`, `internal/cli/import.go:24`.
   - The public board search is read-only POST, but resourceWritePaths registers it as writable `source`. Root help advertises API create/upsert import; `import source` posts each record to the search endpoint and reports imports although the operation creates nothing.
   - This violates the read-only product contract and the Press phase 15 requirement to remove inapplicable CRUD boilerplate. No live import was run.
   - Fix: remove the false write mapping and the unsupported import surface using a preserved hand-authored integration, including MCP visibility.

3. **[P2] Capability discovery omits the ordinary Haneda command leaves.**
   - Initial location: `internal/cli/which.go:28` and the hand-authored registration in `internal/cli/haneda_commands.go`.
   - `which "flights search" --json` ranks offline `snapshot search` first. `which "schedule search"` returns only snapshot search; `which "catalog airports"` returns no match. flights search/detail/disruptions, catalog airports/airlines, and schedule search are absent from the curated index.
   - The same commands are correctly registered in help, agent-context, and MCP. This finding concerns the prescribed capability-discovery entry point.
   - Fix: add the supported domain leaves through a hand-authored index extension and verify representative natural-language and exact-path queries.

4. **[P2] An accepted schedule flight ID silently becomes an empty result.**
   - Initial locations: `internal/cli/haneda_commands.go:348`, `internal/haneda/schedule.go:169`.
   - Help permits a full flight number or stable hnd group ID. `schedule search --flight NH849 --date 2026-10-03` returns one row; substituting `hnd:international:departure:20261003:NH849` succeeds with zero. Schedule matching constructs a Flight without ID, so no stable ID can match.
   - Fix: interpret a board ID's number/kind/direction/date for this lookup, or reject it with schedule-specific help and a clear usage error. Preserve published-period/weekday/feed-window limits.
   - Proof: `review/live/schedule-primary.json`, `schedule-board-id.json`, and the full-capture probe.

5. **[P2] A trailing status separator selects unrelated blank-category states.**
   - Initial location: `internal/haneda/query.go:150`.
   - On the untouched international-departure capture, `canceled` matches two rows; `canceled,` matches six, adding Check-in/Final boarding/Gate Closed rows. The empty CSV token equals the empty source category. `ALL` also passes input validation but matches zero, while `all` matches 159.
   - Fix: normalize the all sentinel and reject or ignore empty CSV tokens. Unknown status must remain an explicit request rather than a consequence of a trailing comma.
   - Proof: `review/independent-probe-results.jsonl`, records named `board-local-status-*`.

6. **[P2] Snapshot integrity checks do not establish declared source coverage or normalized row validity.**
   - Initial locations: `internal/haneda/snapshot.go:119` and `:130`.
   - A complete all/both snapshot with only one domestic-departure source entry is accepted if the summed total matches its flight count. The validator does not check the exact requested source set or each source's rows. It also accepts `scheduled_at:"x"` and an empty listed_flights array with a valid ID; SaveSnapshot and LoadSnapshot both succeed.
   - This is an internal-consistency check, not a request for cryptographic authenticity. Missing source scopes and malformed flight facts should not be presented as a validated complete observation.
   - Fix: verify the expected unique source kind/direction set and per-source totals, valid nullable RFC3339 flight timestamps, and nonempty listed flights aligned with source_primary_flight.
   - Reproductions are bounded one-row files: `review/missing-coverage.json`, `malformed-clock.json`, and `missing-list.json`.

## Verification completed before fixes

- `go test -count=1 ./internal/haneda` passed with the normal approved local-socket access required by httptest. The sandbox-only attempt failed to bind a socket; that was an environment restriction, not a code failure.
- Independent socket-free HTTP replay parsed all four untouched captured boards: 1,325 groups, complete source scope, eight requests, 2,122,802 response bytes, no unresolved airport or airline names.
- The same replay parsed all monthly feeds: domestic departures 1,136, domestic arrivals 1,136, international departures 446, international arrivals 417. All rows preserved published periods and weekday rules.
- Local board matching mapped NH849, UA8003, and NH0849 to the same stable NH849 identity.
- Live 2026-10-03 boards returned domestic departures 503, domestic arrivals 503, international departures 166, and international arrivals 163. These differ from the Oct 2 captures; live source state is explicitly dated.
- Live NH849 detail preserved original codeshare ordering, terminal T2, explicit revised time, and null actual/operator fields. The Japanese 札幌 catalog query returned CTS with source city value SPK.
- In-memory registration verified all eleven supported domain MCP tools and their read-only/write annotations. This confirms the generated tool catalog; a runtime command-mirror check remains part of fix verification.
- Six canonical handoff pages returned HTTP 200 with the expected official page titles: city list, airline list, monthly flights, T2 floor guide, terminal transfer, and connection guide. Results are in `review/live/canonical-links.json`.

## Source fidelity and scope

Ground truth was the approved research brief and absorb manifest, discovery reports/probes, untouched public JSON captures, and first-party script snippets. The date limit matches the public search script's three-calendar-month calculation and domestic/international minimum dates. Normalization uses explicit date fields for scheduled/revised JST time, retains source-primary ordering, keeps actual time and operator identity null, separates arrival exits from boarding gates, and does not manufacture seat or connection guarantees.

The reviewed handwritten client has a 30-second command boundary, no redirects/retries, 20-request/4-MiB-body/16-MiB-command budgets, scan/output limits, and explicit errors for non-JSON and inconsistent board counts. Source failures do not become empty successful boards. Snapshot saves use an exclusive atomic destination unless overwrite is explicit; diff does not call disappearance a cancellation.

## Independent fix verification

- Marketing `UA8003` and padded `NH0849` now resolve NH849 through both detail and plan against the live public source. Each lookup used five requests and 547,148 response bytes, disclosed `board_with_local_flight_lookup`, retained the order NH849 / AC6273 / TG6106 / UA8003, and kept actual/operator fields null. Proofs: `review/fix-verification/flights-detail-{UA8003,NH0849}.json` and `plan-{UA8003,NH0849}.json`.
- Import is absent from staged CLI and MCP. Curated discovery now ranks flights search, catalog airports and schedule search correctly. A schedule board ID and malformed status CSV fail explicitly with exit 2. `ALL` and `all` match the same 159 groups in the untouched captured departure board; an empty CSV token cannot select blank-category rows.
- All three original malformed snapshot files are rejected with exit 10: missing required source scopes, malformed clock, and missing ordered listed flights. The validator also checks unique/per-source counts, JST timestamps, source-primary alignment, status consistency, and unsupported actual/operator claims.
- The later snapshot compatibility regression is closed: omitted v1 `query_mode` and explicit `board` mode compare successfully when origin/date/kind/direction match. The resulting coverage is canonicalized; unrelated coverage still fails validation/comparison.
- The advertised `--include-facilities` flag is now implemented for boards and offline snapshot search. The independent saved fixture returns its five facility entries when requested. Docs name the actual cache directory `haneda-snapshots` and enumerate the supported unknown-field semantics accurately.
- A controlled observation aged exactly 600 seconds returns `stale_snapshot:true` by default and `false` with an explicit positive `--max-age 30m`. Local snapshot help states its five-minute default; nonpositive thresholds are errors. Proofs: `age-final-default.json`, `age-final-explicit-30m.json`, `snapshot-facility-flag-final.json`, and `snapshot-legacy-mode-final.json` in `review/fix-verification/`.
- The final independent command `go test -count=1 ./internal/haneda ./internal/cli ./internal/mcp` passed all three packages after the constructor and policy corrections. The stage binary reports `haneda-airport-pp-cli 0.0.0-dev`.
- A real staged MCP stdio session reported 35 total tools, all eleven required domain tools, and no import/raw source tools. An offline snapshot tool call succeeded; an invalid schedule board ID produced a tool error. Proof: `review/fix-verification/mcp-runtime.json`.

## Output-review evidence and Press classifier limitation

The eligible `status:pass` plan and rollover outputs in the run's `proofs/output-review-livecheck.json` decode as structured JSON. Plan resolved NH849 on 2026-10-03 to T2 with the canonical detail/floor/transfer/connection URLs; revised 00:19 was kept separate from scheduled 00:05 and actual/operator stayed null. Rollover returned EK313 with explicit scheduled 2026-10-03 00:05 versus revised 2026-10-02 23:58 and a signed −7-minute change. No inferred actual time or unrelated flight appeared.

The real `proofs/live-domain/save-before.json` and `save-after.json` receipts contain all four source scopes and 1,335 groups each. `offline-codeshare.json` resolves UA8003 to NH849 with zero requests; `offline-material-diff.json` compares compatible observations with zero requests and correctly reports zero changes when the source rows did not change. Consequential gate/time/status-change behavior is additionally covered by the deterministic tests; a naturally unchanged live pair is not described as proof of a live operational change.

The generic Press classifier labeled a missing-example-file snapshot diff error as a graceful-empty pass. That sample was excluded from positive evidence. It also skipped snapshot save as a write and mislabeled file-backed snapshot search as an unsynced-store prerequisite. These are Press classifier limitations, not positive snapshot proof or open Haneda implementation findings. The explicit real file-backed proofs above establish those behaviors.

The independent socket-free replay results remain in `review/independent-probe-results.jsonl` and `review/fix-verification-probe-results.jsonl`; its review-only program is preserved as `review/probe-source.go.txt` rather than shipped as a Go package.

## Supplemental final matrix and polish review

**Verdict: PASS.** The narrow final changes are sound after separating private fixture permission from public MCP hints. No new Haneda finding remains open.

- Snapshot examples now use the default cache and do not depend on nonexistent demonstration files. `pp:happy-args` no longer turns a boolean `--agent` into a stray positional `true`. The final snapshot save happy/JSON rows execute `snapshot save --kind international --direction departure --agent` without `--dry-run`.
- The initial addition of public `mcp:local-write` was corrected during this supplemental review: it would have understated arbitrary `--file`/`--overwrite` behavior and live HTTP use. The final command has no such annotation. A real staged MCP catalog reports `readOnlyHint:false`, `destructiveHint:true`, `idempotentHint:false`, and `openWorldHint:true`, while retaining file and overwrite arguments. Proof: `review/fix-verification/final-public-mcp-hints.json`.
- The isolated Press copy uses a distinct private `pp:fixture-local-write` opt-in. Real fixture execution requires explicit `--allow-destructive`, a successfully parsed default-cache happy invocation, and absence of `--file`, `--overwrite`, or `--deliver`, including equal-sign forms. Dry-run-capable POST/PUT/PATCH/DELETE mutators retain previews even with the private tag; ordinary public `mcp:local-write` is insufficient. The private classification only establishes this declared fixture as a known mutator and does not change public MCP hints.
- The refreshed `proofs/press-local-write-fix.patch` matches the reviewed source policy, including remote-method guard and private classification. Marker generation, coverage accounting, and source-fingerprint functions are unchanged by that patch. Source hashes use the Press's module/import canonicalization; raw file hashes are not the acceptance comparison.
- The expanded nine fixture-policy cases and existing destructive-default, destructive-auth bypass, remote preview, JSON-fidelity and dry-run guard tests passed independently. Snapshot roundtrip, overwrite, malformed-input and diff tests also passed after the I/O changes.
- `errors.Join(original, f.Close())` preserves the original write/sync error and any close failure while closing the temporary file. The two G304 suppressions are narrowly attached to deliberate operator-selected snapshot/cache reads; regular-file/schema/8-MiB read guards and the 1,000-entry cache-list limit remain present. The final gosec output contains no pending finding in the handwritten Haneda domain/command paths.
- Final evidence is `proofs/final-dogfood-results.json`, run at 2026-10-02 17:29:33 UTC, and its matching `proofs/phase5-acceptance.json`. All 171 executed matrix rows passed; 107 skipped/unverified rows are outside that executed denominator. The snapshot save positive rows contain actual default-cache execution. Earlier `20261003-dogfood-results.json` is historical evidence and was not used as the final-policy matrix proof.

This supplement closes the local review gates. Publication and merge actions remain the parent task's responsibility.

## Supplemental PR #2241 snapshot fix verification

**Verdict: PASS.** The three requested snapshot findings are closed in the reviewed source. No new finding remains in these changes.

- Saved search validates explicit kind/direction against the complete saved scope and requires explicit `--date` to equal `coverage.requested_date`. Defaults inherit the saved scope and retain adjacent service-day rows. Independent CLI fixtures confirmed that an adjacent row remains visible by default, the explicit request day may correctly return zero rows, and asking for the adjacent day fails with exit 2 rather than claiming complete coverage. Covered-empty queries still succeed with zero requests.
- Material diff now compares the full provider `facilities` entries. Independent name-only, title-only, type-only and map-URL-only mutations each produce one `facilities` field change without requiring a separate gate/counter change.
- Automatic diff pairs validated origin/date/kind/direction content, canonicalizes legacy empty board mode, skips a newest unpaired origin, and prefers the newest compatible cached pair when origins compete. Different request dates do not pair. Eight distinct 8-MiB observations reach the 64-MiB selection boundary successfully; a ninth is rejected with exit 10 when no compatible pair has been found. Cache filenames in these fixtures deliberately had misleading scope suffixes, so selection depended on validated content.
- `go test -count=1 ./internal/haneda -run 'Snapshot|Diff'` and `go test -count=1 ./internal/cli -run 'TestHanedaSnapshot'` passed independently. The CLI regressions used the normal approved local-socket access required by their httptest server.
- A separately built current-source CLI passed 20 additional offline checks. The provider URL pointed at an unreachable loopback port; every successful search/diff output reported zero requests. These are artificial reviewer fixtures, not live travel data. Results: `review/fix-verification/pr2241-offline.json`.
- The revised README, SKILL and command help accurately state the saved-scope guard, adjacent-day defaults, full facility/map diff and bounded compatible-pair selection.

## Supplemental PR #2241 cache-stop verification

**Verdict: PASS.** A later unrelated cache failure preserves a fully validated compatible pair and discloses ranking uncertainty. The requested edge case and its documentation correction are closed.

- A malformed older file and a cumulative 64-MiB selection stop return the known comparison with `baseline_sufficient:true`, `cache_selection_complete:false` and explicit `cache_selection_notes` stating that a newer pair may remain unexamined. Removing the malformed stop permits selection of that newer origin's pair; the fallback therefore does not claim a fully ranked cache.
- Successful completed scans and explicit valid file pairs return `cache_selection_complete:true` with empty selection notes. Explicit paths ignore unrelated cache damage while still validating both selected observations. Stops without a known pair remain errors with exit 10; no successful empty baseline is substituted for a failed scan.
- The independently rerun `TestHanedaSnapshotDefaultDiffFindsOlderCompatibleOriginPair` passed, including budget, malformed and disappeared-path cases both with and without a known pair. Existing origin compatibility, legacy board mode and newest-pair ranking assertions also passed.
- A freshly built source CLI passed seven additional offline checks, including actual cumulative 8-MiB JSON files at and above the 64-MiB boundary. Successful outputs reported zero requests with the provider URL set to unreachable loopback. These artificial fixtures are not live flight data. Proof: `review/fix-verification/pr2241-cache-stop.json`.
- README and SKILL now document both uncertainty fields, the possible unexamined newer pair, the explicit-path option and hard failures without a known baseline. Their wording matches the reviewed command behavior and help. No code or documentation finding remains in this change.

```text
---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
```
