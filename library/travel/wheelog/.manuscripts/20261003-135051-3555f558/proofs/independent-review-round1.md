# WheeLog independent review — round 1

Reviewed 2026-10-03 by the single dedicated fresh-context reviewer. No agents were spawned, no source code was edited by the reviewer, and no GitHub writes occurred. This report is review evidence, not publication approval.

## Scope and ground truth

Working tree: `wheelog-pp-cli`.

Read the build brief, generated AGENTS.md, README.md and SKILL.md; the current Printing Press phase 14–17 contracts and output-review sub-skill; the absorb manifest and source brief; research.json's five approved and five built features; the source transport, normalization, command wrappers, shortlist store and relevant generated client/MCP paths.

Inspected actual sanitized outputs in proofs/p1 and proofs/p2 for detail, dated discovery, requested-question comparison, saved proximity, audit reasons, first-observation state, live refresh and offline observation comparisons. Inspected the canonical seven-leg shipcheck receipt and source-specific tests. Also built fresh CLI/MCP binaries into an isolated temporary directory, recursively inspected the source commands' help, exercised real MCP initialize/tools/list/context/categories, and ran local CLI reproductions.

## Actionable code findings

### C1 — P2 / error: MCP context gives unsupported pagination instructions

- Original location: `internal/mcp/tools.go:553` (also lines 539 and 555).
- Reproduction: call MCP `context`. The reviewed binary returned “cursor-based paging”, an `after` parameter and a default limit of 100. `spots search --help` has no `after`; it uses `--max-scan-pages` 1..5 and output `--limit` 1..50, default 5. No typed endpoint tools were registered.
- Fix: provide WheeLog-specific context with the actual mirror surface, page/output/detail bounds, date semantics and evidence limits.
- State: builder added `internal/mcp/wheelog_context.go` and the call before context serialization during review. The helper is accurate on source inspection; rebuilt shipping MCP verification remains required.

### C2 — P2 / error: unsupported API import is publicly advertised

- Locations: `internal/cli/resource_paths.go:29`, `internal/cli/root.go:416`; runtime MCP also exposed `import`.
- Reproduction: `wheelog-pp-cli import spots --input - --json --timeout 1s` with the controlled input `{"spotId":166345}` failed locally with “spot ID must be between 1 and 1000000000”, exit 5. The transport rejected the JSON body before any source request. The generated “writable” path is actually the anonymous read-only detail RPC.
- Fix: remove/disable the generic import command through the preserved WheeLog hook so CLI discovery and the MCP mirror cannot promise source create/upsert behavior.
- State: builder removed `import` in `internal/cli/wheelog_hook.go` during review. The fix preserves all five approved features; rebuilt CLI/MCP absence checks remain required.

### C3 — P2 / error: inclusive local dates exclude fractional timestamps in the day's final second

- Original location: `internal/cli/spots_search.go:269`–271.
- Reproduction: an isolated normalized SQLite fixture with `record_created_at=2026-10-02T14:59:59.500000Z` (JST October 2, 23:59:59.5) returned zero matches for local search with `--from 2026-10-02 --to 2026-10-02 --timezone Asia/Tokyo`; expected one. The echoed upper interval was `2026-10-02 14:59:59`, and the local comparison rejected anything later than that exact instant.
- Fix: retain the observed API wire interval, but apply an exclusive local upper bound one second after its whole-second endpoint. Verify that the final fractional second matches and next-day midnight does not.
- State: builder changed the comparison and added `TestWheelogLocalDateSearchIncludesFractionalFinalSecond` during review. Source inspection supports the fix; fresh regression and shipping CLI verification remain required.

## Actionable documentation findings

### D1 — P2 / error: executable examples contain unresolved placeholders

- Locations: `AGENTS.md:17`–18, 24 and 30–31; `SKILL.md:131`, 349, 366–368, 375–376 and 506–508.
- Reproduction: the fenced `wheelog-pp-cli <command> --help` / `--agent` examples are not executable shell commands; angle brackets are shell operators. Other fenced examples require unspecified resource IDs/types/files. Phase 15 explicitly requires no placeholder literals in executable examples.
- Fix: use concrete known command names and source IDs, such as `which "recorded toilet equipment"` and `spots inspect 166345 --help`. Keep syntax notation outside executable fences; use `spots` / `166345` in teach examples and explain required user-created input files.

### D2 — P2 / error: not-found guidance sends users to a nonexistent command

- Location: `README.md:359`.
- Reproduction: `wheelog-pp-cli list --help` exits 2 with “unknown command list”.
- Fix: name `spots search` for finding source IDs and `shortlist list` for inspecting saved membership; distinguish their coverage.

### D3 — P2 / error: preview and setup prose overstates dry-run output

- Locations: `SKILL.md:193`; `README.md:128`, 322 and 336; root `--dry-run` help uses the same request-preview claim.
- Reproduction: `spots search --dry-run` prints only “would run search bounded public WheeLog spot records”; `doctor --dry-run --json` returns an action stub. Neither shows form fields/a request nor actually checks setup.
- Fix: describe an action summary without executing the command, and label the doctor example as a preview. Update the root flag usage through the preserved hook, or implement the claimed request preview.

### D4 — P3 / warning: no-auth skill includes inapplicable secret/auth migration guidance

- Location: `SKILL.md:204`–205.
- Ground truth: `agent-context` reports auth mode `none`, and the command tree has no auth write operation.
- Fix: describe the actual SQLite shortlist/learning data and invocation state. Remove cookie/auth-sidecar claims and the “first auth write” migration narrative for this anonymous CLI.

### D5 — P3 / warning: argument description contradicts the working recipes

- Locations: `SKILL.md:194`; `README.md:333`.
- Ground truth: keyword and spot-ID positional arguments are supported and used throughout the source examples.
- Fix: retain the non-interactive claim and state that inputs use flags or positional arguments.

### S1 — P3 / warning: unsupported exclusivity marketing

- Locations: `SKILL.md:67`; `README.md:156`.
- Fix: replace “These capabilities aren't available in any other tool for this API” with a concrete source-specific workflow introduction. The search evidence establishes a scoped ecosystem scan, not universal exclusivity.

## SKILL semantic checks

Each assessment below is under 50 words.

1. **Trigger phrases — PASS.** The triggers map to search, inspect, requested-question comparison and saved-shortlist evidence; anti-triggers exclude routes, suitability guarantees, measured dimensions, report dates and contributor/account operations.
2. **Verified-set alignment — PASS.** The five capability entries exactly match the five `novel_features_built` entries, including the two distinct `shortlist list` flag workflows.
3. **Feature descriptions — PASS.** Help and implementations match bounded real detail expansion, requested-question alignment, observation diffs, audit reasons and straight-line proximity.
4. **Stub/gated disclosure — PASS.** None of the five is a stub or credential-gated. Saved membership/baselines, five-detail bounds, unsupported facts, local scope and anonymous-source limitations are disclosed.
5. **Auth narrative — PASS.** The auth setup correctly states anonymous HTTPS and invents no auth command. The separate generic path paragraph needs D4's cleanup.
6. **Recipe output claims — PASS.** The dated, comparison, selected inspection and offline audit recipes produce the claimed shapes. D3 covers inaccurate generic dry-run wording rather than these source recipes.
7. **Marketing-copy smell — WARN.** S1's universal exclusivity sentence is unsupported advertising. Replace it with a concrete workflow introduction.

## Actual output plausibility

PASS for the five approved workflows; no source-query, formatting, aggregation or ordering findings in the inspected successful samples.

- Dated discovery echoes the JST calendar window as UTC `2026-10-01 15:00:00` through `2026-10-02 14:59:59`, returns the original Japanese restroom name, and reports one real detail checked.
- Fixture 166345 preserves question 102 as positive 1 / negative 0, question 101 as 0 / 0 and unreported, and 107/108 as negative. Individual report observation dates remain null.
- Comparison retains both requested IDs and marks the elevator's restroom question inapplicable. No requested source is silently dropped.
- Saved proximity distances ascend approximately 6.12, 100.17 and 157.95 meters. The output explicitly limits coverage to saved facilities and makes no route promise.
- Audit output includes the elevator/ramp's explicit inapplicable-question reasons. The first observation has no baseline; immediate live refresh and offline pairs are unchanged despite new retrieval clocks.
- Canonical source URLs target `?spotId=<id>&la=ja`, with no feed/index substitution or mojibake.
- The independently exercised MCP categories tool returned the actual ten categories and stable question-ID ranges. All five approved command mirrors were discoverable; raw typed endpoint mirrors were absent.

```text
---OUTPUT-REVIEW-RESULT---
status: PASS
findings: []
---END-OUTPUT-REVIEW-RESULT---
```

## Code, privacy and reliability coverage

No further actionable defect was confirmed in the reviewed source-specific paths.

- Form requests use exact uppercase `OS_CODE: 3`, `APP_VERSION: 1`, `APP_LANGUAGE: ja`, AJAX and canonical Referer headers. The transport restricts origin/method/path, caps request and response reads, checks semantic success, strips source-private fields and cookie headers before generated caching/output.
- Empty card coordinates remain unknown. Scalar counts, missing/zero/conflicting reports, question/category applicability and source/update/retrieval clocks stay distinct.
- Source commands validate bounds and call `boundCtx` before source/store work; the generated client shares that deadline with limiter waits/retries. Default command timeout is one minute and the source commands reject values beyond two minutes.
- Shortlist observations and generic/index projections are replaced transactionally. Zero/unknown values replace old evidence; the latest two observations rotate, and retrieval-only differences do not imply physical changes.
- Straight-line origin coordinates are not stored by the source command or generic journal: journal flag values become type classes, never serialized values. No raw contributor material was printed or retained by this review.
- Existing generated migration/identifier defenses and redirect-origin checks were inspected. The available gosec report's broad scaffolding warnings do not establish a WheeLog runtime defect: the redirect re-stamp is same-origin gated, SQL field identifiers are validated, and local file operations implement documented operator-selected paths. The builder made the two source-specific best-effort closes explicit.
- No local edit is requested in generator-reserved `internal/cliutil` or `internal/mcp/cobratree`. No confirmed actionable reserved-package finding is being hidden by the convergence result.

## Verification record

- Independent fresh builds: CLI and MCP succeeded into `/private/tmp/wheelog-review-round1/bin/`.
- Independent `go test -count=1`: wheelog, store, CLI, MCP, MCP bounds and Cobra-tree packages passed. Client tests initially could not bind an httptest port under the sandbox; the same fresh client package tests passed outside that restriction. This was an environment failure, not a source test failure.
- Runtime help was inspected for search/inspect/compare, shortlist save/list/changes/remove and the root tree.
- Independent stdio MCP initialize/tools-list/context/categories succeeded. The reviewed pre-fix tree had 24 tools.
- Controlled local-date regression reproduced C3 without source traffic or source mutations.
- The canonical pre-fix shipcheck receipt reports all seven legs passing, five built features, and no placeholder source capability; these mechanical passes did not detect C1–C3.
- C1–C3 changed in source while this review ran. Their final source changes were read, but shipping binaries and the final regression must be rebuilt/rechecked before the resolution review can close them.
- The builder also made the shortlist parent a non-runnable group. Source inspection confirms its child wiring remains intact; resolution must verify group help and the absence of a hollow parent MCP tool while all child tools remain available. Hand-authored copyright display names were aligned with Jet Sng.

## Separate statuses

- **SKILL semantic: WARN** — S1 remains; the six capability/auth/recipe checks pass.
- **Docs correctness: FAIL** — D1–D3 errors and D4–D5 cleanup remain.
- **Output plausibility: PASS** — assessed actual successful outputs for all five approved workflows.
- **Code review: FINDINGS / resolution pending** — C1–C3 have source fixes, awaiting rebuilt shipping CLI/MCP and targeted regression verification. No additional open source-specific code finding was confirmed.
