# Independent Repark CLI review

Status: **CHANGES REQUIRED** — three source/domain correctness findings. Reviewed the generated project directly with one fresh-context reviewer; no additional agents, codex exec, native CDP, debugger sessions, or implementation edits.

Scope: Printing Press review phases 14–17, the approved batch brief, research/absorb scope, `internal/repark/`, the hand-written parking command seam, generated CLI/MCP output wiring, README/SKILL/AGENTS, fixture tests, and bounded anonymous live checks.

## Findings

### R1 — P2: Bay-specific exceptions become unscoped maximum prices

Initial location: `internal/repark/parse.go:194–195`, `maximums`.

Live `parking detail REP0021148 --agent` returned daytime `amount_jpy: 1400` and overnight `amount_jpy: 2100` without bay applicability. The same output preserved source wording “最大料金1600円※12～14番は1400円” and “最大料金2300円※12～14番は2100円”. Selecting the final currency amount silently substitutes the lower bay-12–14 exception for the general 1600/2300 JPY rule. Agents reading normalized caps can choose parking on an inapplicable price.

Fix: emit distinct rules with explicit bay scope, or conservatively leave conditional/multi-price lines unparsed with `amount_jpy: null` and the complete source text. Add this exact live-source regression case. Do not infer a universal numeric cap from a scoped exception.

Evidence: [bay-specific-caps.json](review-evidence/bay-specific-caps.json), [bay-specific-caps.stderr](review-evidence/bay-specific-caps.stderr). The existing discovery `markers-polymorphic.json` contains the same source note.

### R2 — P2: An explicit local-only request still makes a live source request

Initial location: `internal/cli/parking_runtime.go:36–37` and the five live parking `RunE` paths.

`parking detail REP0022209 --data-source local --json` exited 0 with `meta.source: "live"`, `requests: 1`, and the public detail URL. The live annotation supplies output provenance but does not reject incompatible source selection. The root validates only the enum, and the hand-written commands skip the existing `validateDataSourceStrategy` helper.

Fix: reject `--data-source local` before any request in search, nearby, detail, compare, and quote. Preserve auto/live behavior. Add a CLI regression test proving that the refusal happens before fetch.

Evidence: [data-source-local.json](review-evidence/data-source-local.json), [data-source-local.stderr](review-evidence/data-source-local.stderr).

### R3 — P2: Unknown per-bay variation is asserted to be false

Initial location: `internal/repark/model.go:30`, `parseLimits`, and marker limit normalization.

A plain source restriction “高さ2m、長さ5m、幅1.9m、重量2t” and an empty marker `limit_note` serialize as `varies_by_bay: false`. The source does not establish that all bays have identical restrictions; the approved contract explicitly leaves per-bay limits uncertain. The broader `remaining_bay_fit: "unknown"` safeguard is correct, but this boolean independently asserts a known uniformity fact.

Fix: use an optional boolean: null when unspecified, true for explicit varying-bay wording, and false only when the source explicitly establishes uniform limits. Test absent and explicit source notes.

Evidence: [day-type-detail.json](review-evidence/day-type-detail.json) and [data-source-local.json](review-evidence/data-source-local.json), together with the captured plain restriction / empty marker note.

## Verification and positive coverage

- `go test ./internal/repark ./internal/cli` passed with approved loopback networking. An initial sandbox-only run could not bind httptest; that was a tool restriction, not a provider or implementation defect. See [test-verification.txt](review-evidence/test-verification.txt).
- A fresh independent executable was built at `/private/tmp/repark-independent-review`. Source SHA-256 values were retained in [initial-source-sha256.json](review-evidence/initial-source-sha256.json).
- Live REP0008912 preserved Japanese 月～土 / 日祝 rate groups, 20min/220 JPY and 60min/110 JPY, distinct maximum rules, overnight rollover, canonical IDs/URLs, capacity, and vehicle units.
- Live discovery with `--select lots.id,lots.name,lots.occupancy` preserved the requested nested fields and returned pure JSON; occupancy measurements and exact available-space counts stayed null. See [agent-discovery-select.json](review-evidence/agent-discovery-select.json).
- An impossible named query returned `needs_refinement`, a null anchor, and an empty array. No personal location was substituted. See [unresolved-search.json](review-evidence/unresolved-search.json).
- Comparing one valid lot with REP9999999 reported the failed ID, successful/requested counts, partial coverage, and a stderr warning. The supplied 2.1m height independently produced a published-limit conflict. See [compare-partial.json](review-evidence/compare-partial.json).
- The public simulator converted UTC RFC3339 entry/exit to 2026-10-03 20:00 → 2026-10-04 08:00 JST and returned 600 JPY with four source cautions. No local fee arithmetic, discounts, charge guarantees, booking, or payment was introduced. See [quote-rfc3339.json](review-evidence/quote-rfc3339.json).
- An invalid bay produced a source refusal without inventing a price. A missing required detail argument under `--agent` exited 2. See the corresponding invalid-bay / agent-missing-input evidence.
- Redirect allowlisting, bounded source body reads, whole-command and per-request timeouts, scan/output limits, request budgets, zero retries, typed throttling, identity checks, and source-interval echo checks are implemented and have meaningful tests.
- Runtime needs no account, cookie, credentials, or resident browser. This review used public HTTP only and did not touch other app sessions.

## Documentation / gate notes

README and SKILL capability descriptions, command examples, projection paths, source-only quote limits, JST semantics, occupancy/fit separation, displayed JPY handling, and raw provider timestamp semantics match the checked implementation. Local-build versus separate-publication status is disclosed.

At initial review, `research.json.novel_features_built` was absent/null. The builder must complete dogfood/verified-feature synchronization before claiming final verified-set alignment for Phase 14. Full shipcheck, dogfood, patch preservation, receipts, and local promotion remain the builder's gates; this report does not claim they are complete.

Review-fix rounds: initial review complete; verification of requested fixes pending through this same reviewer.

## Source evidence consulted

Run root: `/Users/zjsng/Projects/Personal/Coding/.japan-cli-builds/batch3-press/repark/.runstate/repark-cli-6f3b9709/runs/20261002-224755-65ef3fef/`.

- `research/repark-brief.md`, `research/repark-absorb-manifest.md`, and `research.json`.
- `discovery/detail-osaka.html`, `quote-osaka.html`, `markers-osaka.json`, and `markers-polymorphic.json`.
- Existing `proofs/live-early/` detail, nearby, search, compare, selected output, and day/overnight quote evidence.
- Independent evidence under this project's `review-evidence/`.

