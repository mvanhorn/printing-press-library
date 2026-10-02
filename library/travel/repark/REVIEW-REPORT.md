# Independent Repark CLI review

Status: **PASS — no outstanding findings.** The same reviewer verified the final bay-variation refinement and its negation fix against the current source and isolated library modules. All earlier source/domain and documentation/metadata findings remain fixed. Initial findings remain in `review-evidence/initial-review-report.md`.

Review path: direct independent reviewer dispatch for Printing Press phases 14–17. No additional reviewers, subagents, codex exec, native CDP/debug sessions, implementation edits, credentials, account actions, reservations, or payments were used.

## Gate assessment

| Review gate | Result | Evidence |
|---|---|---|
| 14 — SKILL semantics and verified-feature alignment | PASS | Five planned and five built capabilities; README/SKILL lists match `novel_features_built`, including both nearby capabilities. All six parking help paths resolve. |
| 15 — README/SKILL/AGENTS correctness | PASS | Local build/publication status, source quotes, JST, occupancy/fit, unknowns, projections, and opt-in snapshot claims match checked behavior. Snapshot examples now include the required range. |
| 16 — Agent output plausibility | PASS | Successful live detail, discovery, selected-field, unresolved-query, partial-comparison, and overnight-quote outputs were inspected. Japanese names, canonical parking IDs/URLs, ordering, and partial results are coherent. |
| 17 — Local code and safety review | PASS | Source fixes, refusal paths, source client bounds, normalization, fixtures, output wiring, and the snapshot seam were inspected. |

The canonical final `proofs/shipcheck-final.json` reports PASS on all seven legs. Final patch preservation, receipt closure, and atomic local promotion remain the builder's responsibility; this independent review covers phases 14–17.

## Fix verification

A fresh independent executable was built at `/private/tmp/repark-independent-review-fixed`. `go test ./internal/repark ./internal/cli` passed with approved loopback access.

Live REP0021148 now preserves both bay-12–14 exception lines with `kind: "unparsed"` and `amount_jpy: null`, together with day type, overnight window, repeating application, and complete source wording. Unspecified `varies_by_bay` is null; the explicit varying-bay source fixture yields true.

All five live parking command paths refuse `--data-source local` with usage exit 2 before source IO. Missing or oversized snapshot windows also fail with exit 2 before source IO.

The SKILL lookup-refresh example and all `sync --help` examples now supply a bounded `--param` or `--resource-param` and `--max-pages 1`. One real marker was snapshotted in an isolated temporary CLI home, and subsequent local search returned its raw source record. Snapshots preserve raw provider data; parking commands continue to read live planning facts using canonical REP IDs.

Verification artifacts:

- `review-evidence/fixed-bay-specific-caps.json`
- `review-evidence/fixed-offline-refusals.json`
- `review-evidence/fixed-test-verification.txt`
- `review-evidence/bounded-snapshot.json` and `bounded-snapshot.stderr`
- `review-evidence/snapshot-search.json`
- `review-evidence/final-source-sha256.json`
- Run `proofs/live-early/select-fixed.json` and `scan-bounded.json`

## Additional verified coverage

Live checks preserved 月～土 / 日祝 rate groups, day/night prices, maximum conditions, canonical parking identity, vehicle units, and source wording. An impossible named query returned `needs_refinement`, a null anchor, and an empty array without substituting personal location. Partial comparison reported the failed lot and counts, warned on stderr, and kept supplied height conflicts separate from vacancy.

The source simulator converted UTC timestamps to 2026-10-03 20:00 → 2026-10-04 08:00 JST and returned 600 JPY with four provider cautions. Invalid bay input produced no invented fee. No local total arithmetic, discounts, charge guarantees, booking, or payment was introduced.

Redirect allowlisting, body/timeout/request bounds, zero parking retries, independent matching/output caps, typed throttling, canonical identity checks, and source-interval echo checks have meaningful tests. Exact available bays, measurement time, remaining-bay fit, unspecified bay variation, tax inclusion, provider timestamp semantics, tariff changes, and coverage remain explicit unknowns or unguaranteed facts.

Generated source/framework commands remain available. Manual snapshots are explicitly supplied and bounded; they do not establish a complete offline parking inventory.

## Evidence and boundaries

Project: `/Users/zjsng/Projects/Personal/Coding/repark-cli`.

Run root: `/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/repark/.runstate/repark-cli-6f3b9709/runs/20261002-224755-65ef3fef/`.

Consulted the approved batch brief, phase review files 14–17, research/absorb manuscripts, `research.json`, discovery HTML/marker evidence, `proofs/live-early/`, source/CLI fixtures, generated CLI/MCP output wiring, and README/SKILL/AGENTS. Historical failing outputs remain separately named; fixed outputs are distinct. A sandbox httptest bind refusal was resolved with approved loopback access and was not treated as a provider failure.

Review convergence: findings cleared after two fix rounds.

## Final operator-range follow-up

PASS — no new findings. The documented `REPARK_SYNC_RANGE` setting is consulted only when the effective request has no explicit range key. It supplies the same request parameter and passes through the same coordinate/window validator; no default coordinates or verifier-specific bypass were introduced.

Targeted sync and source-strategy tests passed. An independent fresh executable performed one successful public snapshot using only the environment range in an isolated temporary CLI home. Empty and invalid values supplied through each of `--param`, `--resource-param`, and `--global-param` overrode the environment and exited 2 before source IO. Evidence is retained in `review-evidence/env-range-verification.json`, `env-range-snapshot.json`, and `env-range-snapshot.stderr`.

README, SKILL, and help disclose the environment setting, explicit-flag precedence, lack of preset location, and limited snapshot coverage. The workflow now uses supported top-level expectation keys while retaining the nested lot-ID extraction used by subsequent source detail/quote steps.

## Final persisted output samples

PASS — no findings. The reviewer inspected all five eligible `status: pass` samples in run `proofs/scorecard-live.json`; none failed or were skipped. The four live planning samples and computed capability sample match their examples. Comparison shows both requested lot IDs, supplied-height conflicts stay separate from vacancy, vacancy filtering is coherent, source tariffs/URLs remain intact, and capability bounds match the implementation. Evidence redaction markers and bounded sample truncation were treated as capture metadata. No new source requests or implementation review were needed because source had not changed.

## Final examples and MCP metadata follow-up

PASS — no findings remain. Nearby's first example matches its lot-anchor happy arguments, while explicit-coordinate discovery remains documented. Search's `[query]` usage, literal read-only annotation, parent examples, and the three raw-interface examples match the supported inputs and source contracts. Source examples are retained in the spec.

The typed MCP HTML tools forward source HTML through the MCP output bound; they do not run the CLI page extractor. The final sanctioned description overrides and generated tool descriptions now correctly promise bounded raw source HTML or a preview, preserve the no-fee-calculation distinction for calculator-form GET, and recommend `parking_detail` / `parking_quote` for normalized planning. The raw marker tool accurately describes source codes and charge groups and recommends `parking_nearby`. Evidence is recorded in `review-evidence/final-metadata-review.json` and the refreshed source hashes.

The builder is rerunning the full live matrix after the final text-only generated description update to refresh its completion marker. This review closure covers the metadata change; it does not replace that final matrix run.

## Publication preparation: bay-variation refinement

PASS — the negation finding is fixed. The current helper evaluates sentence clauses, rejects the reviewed negated difference wording, and requires both a bay-dependent reference and affirmative difference wording. Generic restricted-bay references, uniform wording, and unspecified facts remain null. An explicitly varying width remains true when a separate sentence states uniform height. Both detail and marker parsers use the same predicate.

The current on-disk regression is named `TestBaySpecificCapsRemainUnparsedAndVariationUnknown`. The reviewer independently ran that exact test with `-count=1 -v` in both the source project and isolated library; both executed and passed. Their current `parse.go` hashes match. The earlier failing helper evaluation remains historical evidence in `review-evidence/bay-variation-refinement-review.json`; current module test output is in `review-evidence/bay-variation-fixed-current-modules.json`.

No findings remain in this refinement. The builder's fresh full live gate and publication package follow this source review closure.
