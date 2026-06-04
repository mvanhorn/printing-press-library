# EverBee Scorecard Improvement Design

Date: 2026-06-01
Scope: local-only improvements in this repository
Success bar: improve real CLI usefulness first; score gains are evidence, not the goal

## Context

The current scorecard is `90/100`, grade `A`. The remaining weak areas are:

- `insight`: `4/10`
- `breadth`: `7/10`
- `data_pipeline_integrity`: `7/10`
- `mcp_token_efficiency`: `7/10`
- `mcp_tool_design`: `7/10`
- `workflows`: `8/10`
- `type_fidelity`: `4/5`

The CLI already has eight flagship EverBee research commands:

- `opportunity shortlist`
- `niche score`
- `shop gaps`
- `tags gap`
- `keywords cluster`
- `trends diff`
- `competitors watch`
- `listing audit`

The previous polish pass removed dead generated placeholder stubs. This design must not reintroduce those stubs or rely on metric-only workarounds.

## Goals

1. Make the flagship commands return useful, evidence-backed research output.
2. Automatically refresh targeted EverBee data when local research is missing or stale.
3. Store fetched research locally so repeated analysis over the same scope does not repeatedly call EverBee.
4. Avoid bulk-fetching all EverBee data.
5. Improve local-only scorecard dimensions through real behavior: insight, data pipeline, workflows, MCP shape, and type fidelity.

## Non-Goals

- No upstream Printing Press generator changes.
- No scorecard implementation changes.
- No broad full-account EverBee ingestion for insight commands.
- No unrelated refactors.
- No new public API surface beyond command flags, output fields, and local workflow metadata needed for this feature.

## Architecture

Add a shared internal research evidence layer used by the eight flagship commands. Each command should remain thin: parse flags, build a research scope, request evidence, run a command-specific insight engine, and print an agent-ready response.

The shared pipeline is:

1. Read local research snapshots first.
2. Decide whether local data is sufficient and fresh for the requested scope.
3. If data is missing or stale, fetch only the relevant EverBee data for that scope.
4. Store the fetched research snapshot locally with metadata for reuse.
5. Normalize raw records into typed evidence.
6. Run the insight engine against local evidence.
7. Return structured JSON with source, freshness, confidence, warnings, and next actions.

This keeps live calls targeted and repeatable. It also makes trend and competitor commands stronger over time because every successful targeted fetch becomes a durable research snapshot.

## Components

### Research Store

Persists query-scoped snapshots. Each snapshot should record:

- scope type and value, such as `query`, `keyword`, `shop`, or `listing_id`
- source resources used
- fetched timestamp
- freshness window
- compact raw records
- normalized evidence records
- coverage metadata
- fetch warnings or partial-failure details

The store prevents repeated EverBee calls for the same fresh research and provides history for trend and competitor comparisons.

### Freshness Planner

Given a command request, returns one of four plans:

- `use_local`: fresh matching snapshot exists
- `refresh_targeted`: snapshot is missing or stale and live access is available
- `fallback_local`: refresh failed, but matching stale data exists
- `insufficient_data`: no usable local or live data exists

The planner must reject unrelated local records. A stale but matching snapshot is safer than fresh unrelated data.

### Targeted Fetchers

Fetch only data needed for the current scope. Fetchers should wrap existing EverBee client/command paths and use small page limits by default. They should support:

- keyword-scoped product and keyword research
- shop-scoped shop/product context
- listing-scoped listing/tag context where available
- query-scoped product, tag, and keyword context

Fetchers must save successful responses through the research store before analysis.

### Evidence Normalizer

Converts raw EverBee records into typed evidence records:

- product evidence
- keyword evidence
- shop evidence
- listing evidence
- tag evidence
- trend snapshot evidence

Evidence should preserve stable identifiers, source resource names, timestamps, titles/names, tags, keywords, price fields, estimated sales/revenue fields when available, rank/position when available, and raw-field coverage.

### Insight Engines

Small command-specific modules:

- opportunity ranker
- niche scorer
- shop gap detector
- tag gap detector
- keyword clusterer
- trend differ
- competitor watcher
- listing auditor

Each engine should accept normalized evidence plus scope and return command-specific results in a common response envelope.

## Data Flow

1. Build a `ResearchScope`.
2. Ask the freshness planner for a `ResearchPlan`.
3. Execute targeted fetch only when the plan requires it.
4. Store fetched research snapshot.
5. Normalize raw records into evidence.
6. Run the relevant insight engine.
7. Print agent-ready JSON.

The output envelope should include:

- `scope`
- `data_source`: `local`, `refreshed`, `stale-local-fallback`, or `none`
- `freshness`
- `summary`
- `records` or command-specific result collection
- `evidence`
- `confidence`
- `coverage`
- `warnings`
- `next_actions`

## Freshness And Refresh Policy

Default behavior:

- Use fresh local data when available.
- Refresh targeted stale or missing data when live access is available.
- Store every successful targeted refresh.
- Fall back to stale local data only when it matches the requested scope.
- Never prompt under `--agent`.

Command-level controls:

- `--max-age`: override freshness window.
- `--refresh`: force targeted refresh before analysis.
- `--no-refresh`: use local data only.

The CLI should show freshness explicitly so agents know whether they are using local, refreshed, stale fallback, or no data.

## Error Handling

- Missing local data plus unavailable live refresh returns `insufficient_data` with exact next actions.
- Stale local data plus successful refresh analyzes the refreshed snapshot.
- Stale local data plus failed refresh analyzes stale matching data with a warning.
- Unrelated local data is ignored.
- Partial EverBee responses are stored with coverage metadata and lower confidence.
- Missing or expired auth returns an auth-specific warning and preserves existing snapshots.
- Normalization failures store raw data but return low-confidence output with field-level warnings.

## Workflow Metadata

Add a local workflow manifest for the core research loop:

1. Refresh or reuse scoped research data.
2. Run `opportunity shortlist`.
3. Run `niche score`.
4. Run `tags gap`.
5. Run `listing audit`.
6. Optionally run `trends diff` or `competitors watch` when history exists.

The manifest should match current CLI behavior and support `workflow-verify`.

## MCP And Agent Output

The same command output should serve CLI and MCP command-mirror users. Improve agent usefulness by keeping outputs compact and predictable:

- stable envelope fields across all flagship commands
- compact evidence references by default
- full raw details only behind an explicit flag if needed
- clear `next_actions`
- clear warnings for stale data, auth issues, partial data, and low confidence

This should improve MCP token efficiency without adding a second implementation path.

## Type Fidelity

Use typed internal structs for scopes, plans, snapshots, evidence records, engine results, freshness, confidence, and coverage. Avoid ad hoc maps inside the core analysis path.

Where local spec or manifest fields are wrong or vague, update them narrowly:

- ID fields should stay strings when they represent Etsy/listing identifiers.
- Nullable fields should be explicit.
- Arrays should be typed consistently.
- Numeric metrics should preserve precision and avoid scientific notation for identifiers.

## Testing Strategy

Unit tests:

- Fresh matching snapshot avoids live calls.
- Stale matching snapshot triggers one targeted refresh.
- Refresh failure falls back only to matching stale data.
- Unrelated local records are ignored.
- `--refresh`, `--no-refresh`, and `--max-age` affect planning correctly.
- Normalizer preserves identifiers and evidence source metadata.
- Each insight engine has at least one fixture-backed useful-output test.

Command tests:

- All eight flagship commands return the standard envelope fields.
- Empty or insufficient data returns actionable next steps.
- Partial data lowers confidence and emits warnings.
- Agent mode remains non-interactive.

Verification commands:

- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `cli-printing-press dogfood --dir ...`
- `cli-printing-press verify --dir ... --json`
- `cli-printing-press workflow-verify --dir ... --json`
- `cli-printing-press verify-skill --dir ... --json`
- `cli-printing-press scorecard --dir ... --live-check --json`
- `cli-printing-press tools-audit ... --json`
- `cli-printing-press pii-audit ... --json`
- `gosec`, with unresolved hand-authored findings treated as blockers

## Expected Scorecard Impact

- `insight`: better evidence-backed outputs and useful non-empty fixture/live examples.
- `data_pipeline_integrity`: explicit fetch, store, normalize, analyze flow.
- `workflows`: workflow manifest and verified research loop.
- `mcp_token_efficiency`: compact stable envelopes and evidence references.
- `mcp_tool_design`: clearer command outputs and next actions.
- `type_fidelity`: typed evidence structs and narrow spec cleanup.
- `breadth`: real command coverage remains broad without dead stubs.

## Implementation Boundaries

Keep the first implementation plan focused on one vertical slice before broadening:

1. Research store and freshness planner.
2. One or two targeted fetchers.
3. Evidence normalizer for the selected resources.
4. `opportunity shortlist` as the first engine.
5. Extend the pattern to the other flagship commands after the slice is tested.

This reduces risk and prevents an overlarge rewrite.
