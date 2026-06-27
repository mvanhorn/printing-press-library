# Vibe Signal Build Log

Manifest transcendence rows: 4 planned, 4 built. (report, evidence, sources list, sources sync)

## Built
- Generated baseline from internal-YAML HN spec (hn stories/relevance/item).
- Hand-authored aggregator layer:
  - internal/source/source.go (Signal + Source contract + rate-limit-aware Fetch with typed RateLimitError)
  - internal/source/registry.go
  - internal/source/hackernews/ (Algolia client + fixture test)
  - internal/source/techmeme/ (RSS client + fixture tests; local query filter, recency filter)
  - internal/store/vibesignal.go (signals + runs tables, lazy schema, UpsertSignals/QuerySignals/RecordRun)
- Implemented novel commands: report, evidence (fetch-on-miss), sources list, sources sync.
- Fixture-backed adapter tests under internal/source/*/testdata (real captured responses).

## Phase 3 conformance
- boundCtx timeout boundary on sibling-client commands; per-source cliutil.AdaptiveLimiter + typed RateLimitError.
- dryRunOK short-circuits; verify-friendly RunE (help on bare invocation); usageErr on missing required input.
- NULL-safe SQL scans (sql.NullString) in QuerySignals.
- cliutil.CleanText on HTML/RSS text; cliutil.ParseDurationLoose for --window.
- pp:no-error-path-probe on report/evidence/hn item (free-form topic / HN 200+null id contract have no invalid-arg path).

## Deferred (v2)
- Product Hunt (OAuth) and YouTube (API key) sources; compare / watch / briefs commands.
