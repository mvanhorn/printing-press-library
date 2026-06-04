# eRank Scorecard Improvement Design

Date: 2026-06-01
Scope: `/Users/smacdonald/homegit/erank-cli`
Mode: eRank-only, spec-first local patches

## Goal

Raise the eRank CLI scorecard through honest local improvements without hiding
upstream Printing Press scorer or generator debt. The plan targets rows that can
be improved inside this generated CLI: Insight, Workflows, Data Pipeline
Integrity, and Dead Code. Auth Protocol and MCP Token Efficiency remain
documented upstream-only gaps unless the Printing Press generator is changed in a
separate effort.

The CLI must stay publish-ready throughout the work.

## Current Baseline

Current scorecard: 80/100, Grade A.

Known weak rows:

- MCP Token Efficiency: 0/10
- Workflows: 6/10
- Insight: 4/10
- Auth Protocol: 2/10
- Data Pipeline Integrity: 7/10
- Dead Code: 4/5

Publish validation passes. Live Phase 5 dogfood passes. The current branch is
`codex/import-erank-cli`.

## Non-Goals

- Do not fake auth prefixes or add unused auth constants to improve Auth
  Protocol. eRank uses browser cookies plus `X-XSRF-TOKEN`; the CLI works, and
  the scorer under-credits this custom composed browser-session auth.
- Do not shrink MCP behavior locally by removing useful tools. MCP Token
  Efficiency is driven by Printing Press code-orchestration catalog shape and is
  upstream generator debt.
- Do not add commands, tables, or prose solely to satisfy score heuristics.
- Do not modify the Printing Press generator in this eRank-only effort.

## Architecture

The current `internal/cli/erank_insights.go` file owns seven novel commands plus
shared fetch, decode, scoring, and token helpers. That makes the file too large
and blurs command boundaries.

Split it into focused files:

- `internal/cli/erank_signals.go`: shared signal structs, response fetches,
  response decoding, scoring primitives, token normalization, and common helpers.
- `internal/cli/opportunity.go`: keyword opportunity scoring.
- `internal/cli/listing_gaps.go`: title and tag gap analysis.
- `internal/cli/tags_consensus.go`: consensus tag ranking.
- `internal/cli/watch_drift.go`: drift history and drift summary.
- `internal/cli/lists_optimize.go`: keyword list optimization.
- `internal/cli/saturation.go`: saturation warning.
- `internal/cli/angles.go`: product angle extraction.

Command files own Cobra setup and output shaping. Shared helpers own reusable
data gathering and analysis primitives. Store code owns persistence and search.

## Data Flow

The command flow stays behavior-preserving:

```text
Cobra command -> fetchKeywordSignals -> decode response data -> command-specific analysis -> printOutputWithFlags
```

Typed data-pipeline work starts with one useful slice: keyword signal snapshots.
Persist searchable derived fields for keyword, source, country, score, rating,
tag count, top-listing count, and capture time. This supports `watch drift`,
`lists optimize`, and offline analytics without modeling every raw eRank
endpoint.

If this slice proves too broad during implementation, defer deeper typed storage
and keep the refactor plus dead-code cleanup as the first deliverable.

## Error Handling

- Partial API surface failures remain warnings unless every keyword surface
  fails.
- Invalid Printing Press sentinel inputs continue to return non-zero where live
  dogfood needs a real error path.
- Browser-session proof behavior from the polish pass stays unchanged.
- Persistence failures return explicit command errors when the command purpose
  depends on history or local data.
- Output mode behavior remains unchanged for `--json`, `--compact`, `--select`,
  CSV, plain, quiet, and human table output.

## Testing

Testing focuses on behavior, not exact score assertions.

Required tests:

- Shared helper tests for response decoding, score labels, token normalization,
  consensus ranking, and drift summary.
- Command-level tests where behavior can regress: invalid sentinel handling,
  JSON output shape, and drift history persistence.
- Existing tests remain green after the mechanical split.

Validation gates:

- `go test ./...`
- `go vet ./...`
- `cli-printing-press dogfood --dir /Users/smacdonald/printing-press/library/erank --spec /Users/smacdonald/printing-press/manuscripts/erank/20260531-142147/research/erank-reprint-spec.yaml --research-dir /Users/smacdonald/printing-press/manuscripts/erank/20260531-142147`
- `cli-printing-press verify --dir /Users/smacdonald/printing-press/library/erank --spec /Users/smacdonald/printing-press/manuscripts/erank/20260531-142147/research/erank-reprint-spec.yaml --json`
- `cli-printing-press verify-skill --dir /Users/smacdonald/printing-press/library/erank --json`
- `cli-printing-press scorecard --dir /Users/smacdonald/printing-press/library/erank --spec /Users/smacdonald/printing-press/manuscripts/erank/20260531-142147/research/erank-reprint-spec.yaml`
- `cli-printing-press publish validate --dir /Users/smacdonald/printing-press/library/erank --json`

## Rollout

1. Mechanical split
   - Move command code and shared helpers into focused files.
   - Preserve command names, examples, flags, output shape, and root wiring.
   - Verify `go test ./...` and scorecard.

2. Dead helper cleanup
   - Re-check references for `extractResponseData`.
   - Remove it only if it remains unused.
   - Verify dogfood no longer reports that dead helper.

3. Typed keyword-signal persistence
   - Add the minimal persistence/search path needed by drift and offline
     analytics.
   - Keep schema small and command-owned behavior unchanged.
   - Verify Data Pipeline Integrity and local analytics behavior.

4. Patch manifest update
   - Update `.printing-press-patches.json` with every local customization.
   - Include the reason local generated-tree changes belong in this CLI.
   - Record Auth Protocol and MCP Token Efficiency as upstream-only gaps.

5. Final readiness
   - Run the full validation gates.
   - Keep publish validation passing.
   - Report scorecard delta and remaining structural gaps.

## Success Criteria

- Publish validation passes after all changes.
- `go test ./...` and `go vet ./...` pass.
- No edited Go file exceeds 500 lines without an explicit split rationale.
- `.printing-press-patches.json` records all local generated-tree changes.
- Scorecard improves on locally actionable rows: Insight, Workflows, Data
  Pipeline Integrity, and Dead Code.
- Auth Protocol and MCP Token Efficiency are documented as upstream-only gaps,
  not locally gamed.

## Risks

- Splitting a generated file can drift from future reprints. Mitigation: prefer
  spec/research-owned changes where possible and record local changes in the
  patch manifest.
- Typed persistence can grow too broad. Mitigation: start with keyword signal
  snapshots only, then stop unless a command needs more.
- Scorecard heuristics may not reflect real quality. Mitigation: validate CLI
  behavior and publish readiness first; treat score movement as secondary
  evidence.
