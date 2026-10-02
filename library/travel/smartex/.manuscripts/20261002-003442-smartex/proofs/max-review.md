# Dedicated fresh-context MAX review — phases 14–17

**Final verdict: PASS; all findings cleared at round3.** See the final convergence section below or `proofs/max-review-final.md`. Initial and intermediate findings remain preserved as the audit trail.

Initial review verdict: **FIXES REQUIRED**. Eight medium (P2) correctness/boundedness findings and four low (P3) factual/format findings. No P0/P1 issue found in the inspected paths. This is the one user-required reviewer; no further agents or Codex review processes were invoked. Source was not edited. Builder owns fixes and verification; this report does not self-approve the build.

Target: `working/smartex-pp-cli`, empty baseline. Source fingerprint: `proofs/max-review-evidence/reviewed-source-hashes.json`. The latest `proofs/shipcheck.json` reports all seven legs PASS; those mechanical passes do not cover the negative cases below.

## Confirmed findings

### R1 — P2: Seat map availability ignores advance-request and processing restrictions

Location: `internal/smartex/planning.go:136`.

Reproduce: `window --date 2027-02-28 --now 2026-10-02T12:00:00+09:00 --agent` reports `seat_map_available_now:true` before one-month seat sales. `window --date 2026-11-02 --now 2026-10-02T08:30:00+09:00 --agent` also reports true during its own `confirmation_processing_gap`.

Evidence: `max-review-evidence/advance_seat_map.json`, `processing_seat_map.json`. Current [Japanese advance guidance](https://smart-ex.jp/reservation/useful/pre_time/) disallows seat choice in one-year requests and reservation operations during the 07:30–09:59 processing interval.

Fix: Apply advance/calendar/product restrictions to seat-map usability, or explicitly name a field as reception-hours-only and expose actual usability separately. A bare daytime check cannot support an actual-availability claim.

### R2 — P2: Oversized one-year exclusion is asserted despite conflicting current sources

Location: `internal/smartex/planning.go:128`, `:145`; `README.md:57`; `AGENTS.md:7`.

Reproduce: `window --date 2027-02-28 --now 2026-10-02T12:00:00+09:00 --oversized-baggage --agent` gives categorical `one_year_request_eligible:false` and an actual opening of one-month 10:00.

Evidence: `max-review-evidence/oversized_one_year.json`. [English reception guidance](https://smart-ex.jp/en/reservation/useful/accept_time/) categorically excludes oversized seats; [current Japanese reception guidance](https://smart-ex.jp/reservation/useful/accept_time/) omits that exclusion, and [Japanese advance guidance](https://smart-ex.jp/reservation/useful/pre_time/) describes exceptions for only some train sections/facilities. This is an unresolved source conflict, analogous to the already preserved 08:00/14:00 discrepancy.

Fix: Retain both source rules and return unknown actual advance eligibility/opening for oversized seats. Keep one-month seat sales as a documented fallback, without presenting it as the universally earliest opening. Reconcile the research facts, embedded notes, README and AGENTS together.

### R3 — P2: Oversized party size uses the ordinary six-person limit

Location: `internal/smartex/planning.go:128`, `:135`.

Reproduce: `window --date 2026-10-20 --now 2026-10-02T12:00:00+09:00 --oversized-baggage --adults 6 --agent` reports maximum six and `party_fits_one_operation_now:true`.

Evidence: `max-review-evidence/oversized_party_six.json`. Both [Japanese oversized-seat guidance](https://smart-ex.jp/reservation/reserve_smart/oversized-baggage/) and [English guidance](https://smart-ex.jp/en/entraining/oversized-baggage/) limit one operation to five ordinary or four Green passengers, with no cross-car grouping and some train-specific exceptions.

Fix: Apply the oversized upper bound and disclose the Green four-person rule. Since window currently has no class input, expose class-dependent limits/unknown compatibility, or add and validate an explicit class assumption. Require multiple operations for six oversized-seat passengers. The English oversized page additionally forbids after-hours requests; the current Japanese oversized page omits that categorical prohibition. Preserve the English restriction and Japanese uncertainty rather than affirming overnight oversized eligibility from the general three-person ceiling alone.

### R4 — P2: Agent provenance is cached before mixed commands choose their actual source

Location: `internal/cli/smart_commands.go:174`, `:199`, `:227`; cached in `internal/cli/root.go:388`, consumed at `internal/cli/helpers.go:1637`.

Reproduce: `timetable --from Tokyo --to Shin-Osaka --offline --agent`, and the same query with `--data-source local`, emit `meta.source:live` with zero upstream requests. `sources --source timetable --check --agent` emits `meta.source:local` even after attempting HTTP. Products detail has the equivalent computed/live mismatch.

Evidence: `max-review-evidence/offline_freshness.json`, `timetable_data_source_local.json`, `sources_live_metadata.json`. The isolated source check failed DNS in the restricted shell; it still confirms the HTTP branch and cached metadata. Fresh live samples separately passed.

Fix: Set `flags.agentSource` from the resolved branch before printing, or build an explicit output provenance envelope. Mutating command annotations inside RunE does not update the cached value.

### R5 — P2: All failed fare quote POSTs lose the typed throttle error

Location: `internal/smartex/fare.go:286` and `:293`.

Reproduce: Successful station normalization followed by 429 for all three fare classes records three failures, then returns the generic all-classes-failed error. `lastError` is never assigned in the class failure branch, so `errors.As(..., *RateLimitError)` is false and CLI classification becomes source failure 5 instead of throttle 7.

Evidence: `max-review-evidence/isolated-regressions.log`, `TestReviewAllQuote429KeepsType`; source-identical isolated harness in `<local-artifact>`. No external requests were made. SourceCheck also stringifies its error: timetable currently recovers CLI throttle classification through the HTTP429 string fallback, but typed error identity is lost; sources' all-failed branch always returns API error 5.

Fix: Preserve a typed rate failure across class aggregation, including mixed failure order; propagate typed source errors separately from serialized error strings when all live sources fail. Keep partial successful quotes and explicit failures.

### R6 — P2: Timetable publication/freshness fields misstate drift and offline verification

Location: `internal/smartex/timetable.go:133`, `:157`.

Reproduce: Inject a current timetable page with replacement PDF URLs. Services correctly become empty and `snapshot_matches_current_pdf_links:false`, but top-level `publications` still exposes old 2603 PDFs. Offline results claim `snapshot_matches_current_pdf_links:true` without checking any current page.

Evidence: `max-review-evidence/isolated-regressions.log`, `TestReviewChangedPublicationReturnsFreshLinks`; `offline_freshness.json`.

Fix: Return current discovered PDFs in the live publication field; retain snapshot URLs under an explicitly dated snapshot field if needed. Use null/unchecked for the offline current-link comparison. Keep sample suppression and the empty-coverage warning.

### R7 — P2: Generated reference GETs bypass the promised HTML size cap

Location: `internal/client/client.go:1305`; reference path example `internal/cli/reference_timetable.go:33`; claim `README.md:69`.

Reproduce: Feed a public reference GET a 2,097,178-byte HTML response through an in-memory HTTP transport. The generated client returns the entire body with no error. Its decompression cap is 32MiB, and plain-body reading has no cap. The hand-authored smartex client correctly caps at 512KiB, but all fourteen generated reference endpoints use the other client, including typed MCP tools.

Evidence: `max-review-evidence/reference-regressions.log`, `TestReviewReferenceHTMLBound`.

Fix: Enforce the public HTML cap before materializing reference responses, consistently for CLI and MCP. An API-specific client hook/transport wrapper can preserve generator source; file the general unbounded-body behavior as a template retro candidate. Test both plain and compressed bodies. This is a real requirement gap, not merely an inaccurate README sentence.

### R8 — P2: Generated reference auto pacing has no 2RPS ceiling

Location: `internal/client/client.go:275`, `:278`; construction `internal/cli/root.go:457`; claim `README.md:69`.

Reproduce: Construct the normal auto-rate reference client; ten successful responses raise the limiter from 2.0 to 2.5RPS. Explicit higher rates or zero also bypass the advertised public ceiling. The domain client clamps the caller setting correctly, but the generated reference client does not.

Evidence: `max-review-evidence/reference-regressions.log`, `TestReviewReferenceAutoHasTwoRPSCeiling`.

Fix: Clamp actual public reference transport pacing to at most 2RPS and honor smaller caller ceilings, including MCP. An API-specific shared transport limiter/hook can preserve the generated foundation. Route the generic policy mismatch to retro as appropriate.

### R9 — P3: MCP context recommends absent sync/search and unsupported paging

Location: `internal/mcp/tools.go:963` through `:968`; SQL tool description `:172`.

Reproduce: Call MCP `context`; it advises `sync`, a `search` tool, cursor paging and default limit100. The actual tool list has no sync/search, and reference endpoints expose no cursors or limit parameters. The stateless planning CLI cannot populate the advertised SQLite workflow.

Evidence: `max-review-evidence/mcp-context.jsonl` (tool list and context), `agent-context.json` (runtime tree).

Fix: Replace query tips with station resolution, bounded planning flags, source uncertainty and canonical handoff. Remove or clearly gate the irrelevant SQL/store setup advice. Template-shape retro candidate: generic context/SQL prose is emitted for a stateless HTML target.

### R10 — P3: Invalid timetable dates are classified as upstream failures

Location: `internal/cli/smart_commands.go:202`; `internal/smartex/timetable.go:123`.

Reproduce: `timetable --from Tokyo --to Shin-Osaka --date 2026-02-30 --offline --agent` exits5 with a calendar validation error, without making a request. Documented invalid-input exit is2.

Evidence: `max-review-evidence/invalid_timetable_date.json`.

Fix: Validate the optional date as usage input in the CLI before dispatch, as fare already does, or return a typed validation error that classification preserves.

### R11 — P3: Unsupported exclusivity claim remains in skill/docs

Location: `README.md:101`; `SKILL.md:90`.

Both artifacts assert the listed capabilities are unavailable in any other tool. The research explicitly identifies the official website/app as incumbents for these same planning capabilities. This is marketing boilerplate rather than a supported factual distinction.

Fix: Remove the claim or state the concrete compact JSON/auditable planning benefits. The exact ten-command verified capability set itself is aligned.

### R12 — P3: Handoff renders singular adult counts as plurals

Location: `internal/smartex/planning.go:375`.

Reproduce: The passing handoff sample contains `1 adults, 0 children; reserved class`.

Evidence: `max-review-evidence/output-livecheck.json`, passing `handoff` sample.

Fix: Render count-aware labels, or use unambiguous `adults: 1, children: 0` checklist wording.

## Phase 14–17 checklist

| Phase/check | Result | Evidence/qualification |
|---|---|---|
| 14 — triggers match actual capabilities | PASS | Read-only Shinkansen planning, inventory/booking anti-triggers present |
| 14 — verified-set alignment | PASS | README/SKILL, research novel_features_built, runtime CLI/MCP all contain exactly the ten planning commands |
| 14 — descriptions/help/recipes/auth/stub disclosure | PASS with R11 warning | Runtime tree and concrete samples inspected; no-auth narrative accurate; child/discount/timetable limitations disclosed; exclusivity boilerplate needs correction |
| 15 — README/SKILL/AGENTS factual audit | FIXES REQUIRED | R2, R7, R8, R10, R11; broad cap/pace claims are not supported by reference transport |
| 16 — sampled output plausibility | WARN | Fresh live-check **10/10 PASS**, zero failed/skipped; all ten eligible samples directly inspected. R12 format warning; supplementary branch checks found R1/R3/R4/R6 |
| 17 — native timeout boundary | PASS | smart_commands.go calls boundCtx before sibling client use; generated references use flags.newClient; no unbounded hand-authored request context found |
| 17 — domain correctness/error aggregation | FIXES REQUIRED | R1–R6 and R10; independent in-memory failure/drift fixtures beyond current tests |
| 17 — security/resource checks | FIXES REQUIRED | R7/R8. Inspected public fixed URLs, encoding and parser guards, bounded domain HTML, MCP bearer/TLS gate, read-only SQL gate and result bounds; no credential, booking, payment or reservation mutation performed |
| 17 — template/out-of-scope routing | RECORDED | R7/R8/R9 have generated-template roots; no source edit made, and no issue found/patch made in reserved cliutil or mcp/cobratree |

## What was independently checked

Read domain catalog/planning/fare/timetable source, concrete Cobra bindings and shared command execution, runtime tree/help flag schemas, generated public reference flow, relevant root/output provenance, MCP typed/mirror registration/context, HTTP bearer/TLS protection and read-only SQL/store safeguards. Read current research/approved manifest and latest shipcheck; inspect current tests for covered versus missing paths. Read all-class live proofs for Tokaido, Sanyo, Kyushu, reverse and through fares. Visually checked captured official west/east PDF pages1–2, including the Nozomi1/Hikari631 and Sanyo/Kyushu examples; arrival/departure distinctions and incomplete/null-date semantics are appropriately preserved. No timetable numeric mismatch found in those inspected examples.

Fresh output sampling: `cli-printing-press scorecard --live-check --json`, 2026-10-01T18:09:32Z, **10 passed, 0 failed, 0 skipped**. Evidence: `max-review-evidence/output-livecheck.json`. Inspected query relevance (Osaka search, corridor coverage), Japanese encoding, canonical URLs, adult/paid-member distinction, unknown child/product/inventory fields, ordering and source aggregation. No obvious irrelevant station, phantom fare, silently dropped successful class or inappropriate inventory claim found in the passing samples. Six product source checks and three-class outputs in the earlier live proofs preserve all requested source/class entries.

Additional commands were local except the authorized fresh sample run and public official page reads. The isolated regression tests deliberately fail against this build and establish R5–R8; they are reviewer evidence, not changes to the target suite. A one-off shell source check encountered restricted-shell DNS failure and is retained honestly rather than counted as a passing live sample.

## Output-review result

---OUTPUT-REVIEW-RESULT---
status: WARN
findings:
- check: output-format
  severity: warning
  description: The passing handoff output renders one adult as "1 adults".
  suggestion: Use count-aware labels or an adults/children count mapping in the checklist.
---END-OUTPUT-REVIEW-RESULT---

Initial convergence: findings remain open. Builder should fix and verify them, then send the same dedicated reviewer the changed files and targeted evidence for a convergence pass; no second reviewer is needed.


---

# MAX review convergence — round 2

Verdict: **FIXES REQUIRED — two medium issues remain**. The same dedicated reviewer inspected the fixes; no new reviewers were dispatched and source was not edited. Eleven original findings are cleared; R7 is partially fixed but its raw compressed-body boundary remains bypassable. The new reservation-permission/seat-map logic also exposes the February29 fallback issue below.

Evidence: `max-review-convergence/command-summary.json`, current-source independent CLI/MCP binaries in that directory, `independent-regressions.log`, `mcp-context.jsonl`, `source-hashes.json`, and `automatic-gzip.log`. The initial staged binary was stale, so consequential runtime reproductions were rerun with binaries independently compiled from current source; the saved outputs reflect those current binaries.

## Cleared findings

- R1: ordinary advance and processing-gap seat maps are false; unreserved maps are suppressed.
- R2/R3: oversized annual opening/eligibility remain null with bilingual rules and a separate known one-month fallback; five ordinary/four Green maxima, six/five-person incompatibility, overnight null eligibility/party fit, and unreserved rejection reproduce correctly.
- R4: offline and local timetable commands now emit local provenance. Builder's fresh live branch proof records correct live provenance for sources/products and Japanese reference pages.
- R5: independent all-class429 and timetable429 fixtures preserve typed errors. Failure aggregation source uses errors.Join; any quote429 survives later failures; partial successful quotes remain intact.
- R6: independent PDF-drift fixture returns fresh publications and suppresses samples. Offline current-link comparison is null and snapshot URLs are separate.
- R8: independent auto-rate test remains at2.0RPS after repeated successes; source clamps disabled/high/nonfinite overrides and keeps smaller caller limits.
- R9: current MCP context no longer recommends sync/search/cursor pagination, and SQL's scaffold-only limitation is explicit.
- R10: invalid offline timetable date now exits2.
- R11/R12: unsupported exclusivity prose is removed; current handoff renders `adults: 1, children: 0`.

R7's original plain-body reproduction now rejects a2,097,178-byte response. The builder's manual gzip/deflate decoded-limit tests pass, but they do not exercise the inner transport's automatic decompression.

## Remaining R7 — P2: Automatic decompression hides raw wire bytes from the cap

Location: `internal/client/smartex_public.go:37`–`:41`.

An inner Go transport transparently decompresses an automatically negotiated gzip response before returning it to this wrapper. `resp.Body` is then decoded, and Content-Encoding is removed; the wrapper's raw limiter cannot see the original wire bytes.

Independent local TLS reproduction: ten valid gzip members with65,535-byte extra headers create **655,710 wire bytes**, decoding to only90 bytes. With the normal Go transport beneath smartEXPublicTransport, the GET succeeds with90 returned bytes and no error, exceeding the promised512KiB raw limit. See `max-review-convergence/automatic-gzip.log`; the fixture is in the source-identical isolated review module `<local-artifact>`.

Fix: Prevent transparent inner gzip negotiation/decompression so the wrapper sees wire bytes, for example by explicitly supplying a supported Accept-Encoding request header when none was supplied, preserving existing explicit headers. Then test both automatic/default request negotiation and explicitly encoded responses; enforce raw and every decoded-layer cap.

## R13 — P2: February29 unknown annual opening suppresses known one-month sales

Location: `internal/smartex/planning.go:160`, `:198`, `:210`, `:219`.

Reproduce: `window --date 2028-02-29 --now 2028-02-01T12:00:00+09:00 --agent` returns `window_status:unknown`, `reservation_operations_permitted_by_calendar:false`, and `seat_map_available_now:false`, even though its known standard sales field is2028-01-29T10:00JST. Before that fallback, it also asserts false calendar permission where annual opening is documented as unknown.

Evidence: `max-review-convergence/leap_known_sales.json`, `leap_unknown.json`. The one-month fallback remains a known calendar boundary; uncertainty about the earliest annual opening does not negate later normal sales. The same fallback treatment was correctly applied to oversized annual uncertainty.

Fix: When actual earliest opening is unknown, keep permission null before the known one-month fallback; after that boundary, compute ordinary within-window permission and seat-map rules normally while retaining null earliest annual opening. Add before/at/after fallback cases and retain actual close/date-passed conditions.

## Phase 14–17 update

Phase14 skill semantics and Phase15 docs now clear the original findings. Phase16 original fresh live sample run had10/10 eligible PASS; the builder's refreshed current-build end-to-end run is also10/10 PASS (`live-e2e-final/summary.json`). Current handoff format warning is cleared. Phase17 remains open for R7/R13 above; the original independent throttle/drift/plain-cap/rate fixtures now pass.

No account, booking, payment, message or publication action was performed. No new issue was found in the reserved cliutil/cobratree source. Generic transport/context template roots remain retro candidates as recorded by the builder.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---

Overall convergence is not yet achieved. Resolve the two issues and send the same reviewer the narrow fixes/verification for round3.


---

# Final MAX review convergence — round 3

**PASS — all review findings cleared; convergence established at round3.** The same single dedicated fresh-context reviewer performed every review round. No source edits, additional reviewers, booking/account/payment actions, messages or publication were performed by the reviewer.

Original R1–R12 are cleared, including the remaining R7 raw compressed-body boundary; the round2 February29 issue R13 is cleared. Focused checks found no cascading issue in the reviewed fixes. Generic generated-body/rate/context findings remain documented retro candidates; none is an open defect in this API-specific build.

## Final independent verification

- Source-identical isolated regression fixtures: **6/6 PASS**. All-class429 and timetable429 retain typed throttle errors; publication drift returns new PDFs and suppresses old rows; a2,097,178-byte plain response is rejected; auto-rate remains2.0RPS after successes; the previously accepted655,710-byte gzip wire body with only90 decoded bytes now returns the public-body-size error and zero output bytes.
- Current source was independently compiled into CLI/MCP binaries under `max-review-final/`. Original command reproductions were repeated: advance/gap seat maps, oversized bilingual uncertainty, ordinary/Green party caps, overnight uncertainty, unreserved rejection, offline/local provenance, invalid-date exit2 and corrected handoff wording all behave as intended.
- February29 fallback: independent before/gap/exact-opening/after cases PASS. Before the known January29 sales boundary, unknown annual opening keeps calendar permission null; processing gap is closed; at10:00 and afterward normal sales/seat-map permission resumes. Actual earliest annual opening remains null.
- Reviewed request cloning and explicit supported encoding negotiation. Existing explicit nonempty encoding headers remain intact. The real Go automatic-gzip regression reproduces the former bypass and now rejects it; raw and decoded layers remain bounded.
- Current MCP tool context was re-read and retains the corrected stateless/unsupported-data disclosure. Original unsupported exclusivity claim and handoff pluralization are removed.
- One affected live official reference GET was repeated with the final compression fix: `reference timetable-links --agent --timeout 10s`, successful current PDF links, live provenance,938-byte stdout and zero stderr. This verifies the changed request negotiation on a real public source.

Evidence: `max-review-final/independent-regressions.log`, `command-summary.json`, leap boundary JSON files, `mcp-context.jsonl`, `live-reference.json`, `live-reference.stderr`, and `source-hashes.json`. Builder's targeted final-fix checks are PASS in `max-round2-fix-tests.log`; full tests/vet and the current production live/provenance evidence were inspected in earlier rounds. The refreshed live end-to-end proof remains **10/10 PASS**, including all classes/corridors, reverse/through routes and negative/empty cases (`live-e2e-final/summary.json`), with five corrected live branches in `max-fix-live-provenance.json`.

## Phase 14–17 completion

| Phase | Final review result |
|---|---|
| 14 — semantic skill review | PASS: triggers, ten verified capabilities, limitations, auth and recipes align |
| 15 — README/SKILL/AGENTS factual audit | PASS: reviewed claims reflect current supported behavior and uncertainty |
| 16 — sampled output plausibility | PASS: ten eligible original samples assessed, refreshed10/10 live proof inspected, format finding cleared |
| 17 — local correctness/security review | PASS: original and follow-up findings cleared; consequential independent reproductions pass |

The review is complete. The builder remains responsible for the final umbrella verification, evidence reconciliation and authorized local promotion.

---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
