# Maintainability and performance review — round 1

Result: FINDINGS. Four in-scope findings and one template-shape retro candidate. No product files changed. Reviewed approved core and notebook workflows, context/storage boundaries, projection, source freshness, retention and MCP exposure. Reserved cliutil/cobratree internals were excluded. Existing67-case replay and measurement proofs were read; only focused isolated reproducers ran, with zero origin requests.

All reproduction details, actual process outputs and SQL plans are in `phase17-maintainability-reproducer.json`; reproducible tool is `phase17-maintainability-reproduce.py`. Public HTML inputs are existing independently captured/sanitized E2E fixtures.

## In-scope findings

### M1 — medium: saved protection scans the membership table for every snapshot

- Location: `internal/notebook/notebook.go:43`, `:163`, `:169`.
- Trigger: accumulated saved notebooks plus unsaved snapshots; every successful source replacement runs `pruneUnreferenced`, even below the16MiB eviction threshold.
- Current schema indexes `(list_name,restaurant_id)` and `(list_name,position)`, but the correlated protection predicate searches `restaurant_id` alone. EXPLAIN shows a full covering membership-index scan for every snapshot.
- Impact: retention work scales approximately snapshots×memberships and blocks the write transaction. A private10k-snapshot/9k-membership SQL fixture with only128bytes per snapshot took1215.6ms for the current sum query. Adding an index on restaurant_id in that private fixture gave12.2ms, identical protected/unsaved total, and an indexed seek plan. Earlier focused runs were about1.2s versus1.4ms; the latest exact figures are retained in JSON.
- Small fix: add an idempotent child-key index `tabelog_memberships(restaurant_id)` in notebook Init. It also supports protected deletion checks; preserve current transaction and membership semantics.

### M2 — medium: first detail refresh reports a provenance-only budget change as a fact change

- Location: `internal/cli/tabelog_lists_refresh.go:58–67`.
- Trigger: fetch/save SAMBOA's listing snapshot, then refresh its details using the existing real-source fixtures.
- Proven outcome: `changes` contains `lunch_budget`, although previous/current raw values are both `-`, both numeric bounds are null, and both evidence states are source_unknown. Only nested `source` changes from listing to listed.
- Impact: the fact-change stream reports a lunch-budget change without changed or newly known budget facts. Retrieval provenance is already available on the current snapshot.
- Small fix: compare budget fact content/state separately from its source-surface metadata, retaining complete provenance in snapshots. Ignore a source-only delta or classify it as provenance/new-section evidence rather than a price/fact change. This finding does not claim the simultaneously observed dinner known→unknown transition had identical values.

### M3 — low: inherited select help demonstrates an absent restaurant field

- Location: `internal/cli/root.go:283`; domain override seam is `internal/cli/tabelog_helpers.go:263`.
- Trigger: follow visible `--select title,url` help on find.
- Proven outcome: source-cached invocation returns only URL fields and warns that title matched no fields; zero HTTP requests. It exits0 because URL matches, but loses the requested human identity field.
- Impact: the main help's projection example is misleading for the actual domain envelope and can cause unnecessary follow-up detail requests.
- Small fix: customize the select flag's usage through the existing domain hook to use real fields, e.g. `items.id,items.name,items.rating`. Do not edit the generic generator example solely for this CLI.

### M4 — medium: the final generic checkpoint write escapes the find deadline

- Location: `internal/cli/tabelog_discovery.go:75`; called method `internal/store/store.go:3025–3037`.
- Trigger: a slow/contended final sync_state metadata write after context-bound normalized snapshot persistence.
- `SaveSyncStateAt` executes through `sql.DB.Exec` without the command context. The whole-command context is no longer effective for this final operation.
- Proven outcome: with an artificial slow INSERT trigger on sync_state in an isolated actual CLI DB, `find --data-source local --timeout100ms` exited0 after229.4ms, with zero replay HTTP requests. The trigger is instrumentation, not a claim about normal source latency; it demonstrates that this boundary ignores cancellation.
- Impact: an otherwise cached command may outlive the caller's timeout, and an unrelated raw-store checkpoint failure can fail a successfully persisted normalized source operation.
- Small fix: remove this obsolete generic checkpoint write if no domain consumer needs it, or replace it with a context-aware call. The source-aware notebook hints no longer depend on raw sync-state population. Keep snapshot persistence errors visible.

## Template-shape retro candidate

### T1 — medium: generated projection warnings precede the machine error object

- Location: `internal/cli/helpers.go:2038`; template `internal/generator/templates/helpers.go.tmpl`.
- Trigger: actual domain `find --agent --select nonexistent` against an existing source cache.
- Proven outcome: exit2 with stderr `warning: --select ...` followed by `{code:2,error:...}`. The combined diagnostic is not one JSON object and cannot be parsed by the newly verified machine diagnostic consumer.
- Cause: generated `filterFieldsChecked` writes unconditionally to process stderr, before the domain error boundary can encode the returned error. This recurs in the shared template and is not a Cobra duplicate.
- Routing: report as a generator retro candidate under the phase17 template-shape escape hatch. A mode/writer-aware projection diagnostic API would let machine callers emit one structured diagnostic and human callers retain warnings. No template or reserved code was patched in this review.

## Checks with no additional finding

Snapshot replacement remains complete/atomic and separate from notes/membership; cursors are drained before subsequent queries; saved members are protected from unsaved retention. Refresh is limited to20 unique targets, concurrency2 and a bounded context; per-record source/persistence failures retain valid prior data and usable partial stdout. Listing fetches preserve older full-detail snapshots and their original timestamps. Default summaries share provenance only when equal, and explicit projection uses full typed records. Normal operation starts no browser/background service; optional MCP raw-store tools are absent. Selected-ID Entries still scans the whole list before filtering; that is an optimization opportunity, but no separate demonstrated budget violation was filed.
