# Printing Press Retro: Amazon Jobs

## Session Stats
- API: amazon-jobs (amazon.jobs backing search.json API)
- Spec source: direct-HTTP characterization (hand-authored internal YAML; user chose "backing API" route)
- Scorecard: 84/100 (Grade A)
- Verify pass rate: 100% (PASS)
- Fix loops: 2 (generate resource-name collision; dogfood error-path annotations)
- Manual code edits: ~1000 LoC hand-written data layer (find/get/sync/new/save/searches/stats/skills + shared helpers)
- Features built from scratch: 8 commands (5 novel + 2 absorbed + 1 hand-written sync)

## Findings

### 1. response_path silently ignored on list endpoints that also declare pagination (Bug)
- **What happened:** The generated `postings search` command (endpoint has `response_format: json`,
  `response.type: array`, `response_path: jobs`, and `pagination: {type: offset}`) never extracted the
  `jobs[]` array from the `{error, hits, facets, content, jobs[]}` wrapper. The auto-cache saw the whole
  wrapper as one item and emitted `warning: 1/1 postings items returned but not cached locally (no
  extractable ID field)`; `--json` output was the full wrapper, not the list.
- **Scorer correct?** N/A (not a score penalty; a correctness bug surfaced during Phase 2 build).
- **Root cause:** `internal/cli/data_source.go` has two read paths. The non-paginated
  `resolveReadWithResponsePath(...)` (line 137) threads a `responsePath` argument and calls
  `applyResponsePath`. The paginated `resolvePaginatedReadWithStrategy(...)` (line 221) has **no
  responsePath parameter at all**, so when an endpoint declares BOTH `pagination` and `response_path`,
  the promoted command routes through the paginated variant and the response_path is dropped. Evidence:
  `promoted_postings.go:50` calls `resolvePaginatedReadWithStrategy(...)` with no response-path argument.
- **Cross-API check:** Recurs on any API whose list endpoint returns a results-wrapper AND is paginated —
  the single most common REST list shape.
- **Frequency:** most paginated list APIs with a wrapper envelope.
- **Fallback if the Printing Press doesn't fix it:** The agent must notice the broken auto-cache, then
  hand-write the entire sync/store/query layer against `store.UpsertBatch` + `store.DB()` (what happened
  here — ~1000 LoC). Easy to miss: the CLI still "works" for live queries, so the broken local store can
  ship silently and every offline command (search/stats/store-backed novel features) returns empty.
- **Worth a Printing Press fix?** Yes. High blast radius, silent failure, expensive fallback.
- **Inherent or fixable:** Fixable. Thread `responsePath` through the paginated read path and apply it to
  each page's payload before the list is extracted/cached, exactly as the non-paginated path does.
- **Durable fix:** Add a `responsePath` parameter to `resolvePaginatedReadWithStrategy` (and its
  `...JSONGuard` variant), call `applyResponsePath` on each page before item extraction/caching, and have
  the generator pass the endpoint's `response_path` into the paginated call in the promoted-command
  template (parallel to the non-paginated call site).
- **Test:** Positive — an internal spec with `response_path: X` + `pagination` on a wrapper-list endpoint
  produces `<cli> <resource> <endpoint> --json` returning the extracted array and auto-caches each item by
  id. Negative — an endpoint with no `response_path` (bare array response) still works unchanged (empty
  responsePath → applyResponsePath no-op), and a wrapper endpoint WITHOUT pagination keeps working.
- **Evidence:** Phase 2 dry-run showed the correct bracketed URL but `--json` returned the full wrapper
  under `results`, and stderr warned "no extractable ID field"; the entire data layer had to be hand-written.
- **Related prior retros:** None (first retro on this machine).

### 2. Disabled sync-hint subsystem still emits a dead flag, a dead helper, and failing tests (Bug)
- **What happened:** The generator set `const syncHintsEnabled = false` (correct — this is a manual-sync
  CLI with no `cache.enabled`), but still emitted (a) a `--max-age` persistent flag no command reads,
  (b) an uncalled `hasChangedLocalFlags` helper, and (c) `sync_hint_test.go` whose tests assert the
  hint functions write to stderr. With the subsystem off, `hintIfUnsynced`/`hintIfStale` return false, so
  `go test ./internal/cli` fails 5 tests (e.g. `TestHintIfUnsynced_EmptySyncStateWritesHintToStderr`,
  `TestHintIfStale_BackdatedSyncStateWritesHintToStderr`).
- **Scorer correct?** Partially. Dogfood correctly WARNed on the dead `--max-age` flag and dead helper.
  But neither verify nor dogfood surfaced the 5 failing `go test` cases as a blocker — a `go test`-red
  library artifact passed shipcheck. (Primary fix is the generator; a secondary scorer enhancement could
  run `go test` and flag failures.)
- **Root cause:** `internal/generator` emits the sync-hint flag, helper, and enabled-assuming test suite
  unconditionally, independent of the `syncHintsEnabled` value it also emits into `sync_hint.go:23`.
- **Cross-API check:** Deterministic — fires for every printed CLI where `syncHintsEnabled` is emitted
  false (hand-written data layers, stateless read-through wrappers, any spec without `cache.enabled` where
  the generator can't classify a syncable resource). Direct evidence: amazon-jobs this run. The failure is
  provable from the template logic alone (test asserts enabled behavior; constant can be false), so a
  maintainer can confirm on any such CLI in ~30 seconds.
- **Frequency:** subclass: CLIs where `syncHintsEnabled=false`.
- **Fallback if the Printing Press doesn't fix it:** None safe. The files are DO-NOT-EDIT and revert on
  reprint, so the agent cannot durably fix it in the printed CLI; `go test ./...` stays red and any
  publish CI running the standard test command flags 5 failures with zero runtime cause.
- **Worth a Printing Press fix?** Yes — build-breaking (not cosmetic) and self-inconsistent generation.
- **Inherent or fixable:** Fixable. Gate the emit of the `--max-age` flag, the `hasChangedLocalFlags`
  helper, and the sync-hint test suite on `syncHintsEnabled`; or make the emitted tests assert the
  disabled behavior when the subsystem is off.
- **Durable fix:** In the generator, condition the sync-hint flag/helper/test emission on the same
  `syncHintsEnabled` decision; when disabled, either skip `sync_hint_test.go` or emit
  assert-returns-false tests.
- **Test:** Positive — generate a CLI with the subsystem disabled; `go test ./internal/cli` passes and
  `<cli> --help` shows no `--max-age`. Negative — generate a CLI with the subsystem enabled; the hint
  flag, helper, and enabled-behavior tests are all present and pass.
- **Evidence:** Polish flagged it; `go test ./internal/cli` reproduces 2+ failures directly.
- **Related prior retros:** None.

## Prioritized Improvements

### P1 — High priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 1 | response_path dropped on paginated list endpoints | generator | most paginated wrapper-list APIs | Low (silent empty store) | medium | none needed (empty responsePath is a no-op) |

### P2 — Medium priority
| Finding | Title | Component | Frequency | Fallback Reliability | Complexity | Guards |
|---------|-------|-----------|-----------|---------------------|------------|--------|
| 2 | Disabled sync-hint subsystem emits dead flag/helper + failing tests | generator | subclass: syncHintsEnabled=false | None (DO-NOT-EDIT, reverts) | small | gate emit on syncHintsEnabled |

### Skip
| Finding | Title | Why it didn't make it |
|---------|-------|-----------------------|
| S1 | No framework sync/search/sql emitted for single-GET wrapper-list spec | Step G: uncertain whether intended for single-endpoint specs vs a bug; likely the same syncable-resource-classification root cause as F1, so folded into F1's fix rather than filed separately. |
| S2 | Cache Freshness 3/10 penalizes manual-sync CLIs | Step G: scorer isn't clearly wrong; manual-sync-vs-auto-refresh is a legitimate structural tradeoff, mostly per-CLI. |
| S3 | scorecard "MCP: 1 tools" reads static tools-manifest.json, undercounting the runtime cobratree surface | Step B: couldn't evidence cross-CLI harm; runtime surface is correct, so this is a cosmetic reporting gap. |

### Dropped at triage
| Candidate | One-liner | Drop reason |
|-----------|-----------|-------------|
| resource name `jobs` collided with framework command | Generation errored with a clear message + rename suggestion | handled-correctly |
| listing `sync` in research.json novel_features scaffolded a stub | Agent error (sync is a framework command name); self-corrected | iteration-noise |
| raw HTML in --agent JSON output | Hand-written command bug; fixed in polish | printed-CLI |
| new-since truncation counted after truncating (data loss) | Hand-written command bug; fixed after code review | printed-CLI |
| result_limit=0 false-zero with filters | Upstream API behavior; handled in the printed CLI | API-quirk |

## Work Units

### WU-1: Thread response_path through the paginated read path (from F1)
- **Priority:** P1
- **Component:** generator
- **Goal:** A list endpoint that declares both `response_path` and `pagination` extracts and caches the
  wrapped array on every page, instead of silently dropping response_path.
- **Target:** `internal/cli/data_source.go` paginated read functions + the promoted/endpoint command
  template that calls them (`internal/generator/`).
- **Acceptance criteria:**
  - positive: internal spec with `response_path: items` + offset pagination on a `{total, items[]}` wrapper
    → `<cli> <res> <ep> --json` returns the array and auto-caches each item by id (no "no extractable ID" warning).
  - negative: bare-array endpoint (no response_path) and wrapper-without-pagination endpoint both unchanged.
- **Scope boundary:** Does not change cursor/offset detection or add new pagination styles; only threads the
  existing response_path value through the existing paginated path.
- **Dependencies:** None.
- **Complexity:** medium

### WU-2: Gate sync-hint flag/helper/test emission on syncHintsEnabled (from F2)
- **Priority:** P2
- **Component:** generator
- **Goal:** A CLI generated with the sync-hint subsystem disabled compiles, passes `go test`, and shows
  no dead `--max-age` flag.
- **Target:** `internal/generator/` sync-hint templates (flag registration, `hasChangedLocalFlags` helper,
  `sync_hint_test.go`).
- **Acceptance criteria:**
  - positive: disabled → `go test ./internal/cli` passes; `--help` has no `--max-age`; no uncalled helper.
  - negative: enabled → flag, helper, and enabled-behavior tests present and passing.
- **Scope boundary:** Does not change when the subsystem is enabled/disabled; only makes the emitted flag,
  helper, and tests consistent with that decision.
- **Dependencies:** None.
- **Complexity:** small

## Anti-patterns
- None observed in the pipeline itself; the two findings are generator emit bugs, not process gaps.

## What the Printing Press Got Right
- The resource-name collision error was excellent: clear message naming the conflict and suggesting a
  concrete rename (`amazon-jobs_jobs`).
- The novel-features subagent produced a sharp, well-cut transcendence set (adversarial pass killed 6
  weak candidates with reasons).
- The store schema for the typed `postings` table (columns + FTS + UpsertBatch/UpsertPostings) was
  emitted correctly and made the hand-written data layer straightforward.
- Shipcheck, live dogfood (81/81), and the output/code review caught real bugs (data-loss in `new`,
  HTML-in-JSON, live/local provenance mislabel) that would otherwise have shipped.
- cobratree runtime MCP mirroring exposed all 8 hand-written commands as agent tools with zero extra work.
