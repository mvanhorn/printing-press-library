# Phase 17 correctness review — round 1

**Result: 3 actionable product findings; 1 active generated-template retro candidate.** No product edits were made.

Ground truth: approved absorb manifest, implementation contract, source replay contract, product AGENTS.md, actual source/domain/CLI/notebook/MCP interfaces and relevant generated client/store boundaries. Existing passing tests were treated as evidence, not proof of missing branches. Focused reproducers used public captured fixtures, loopback HTTP and disposable storage; no Internet requests, broad suite or receipts were run.

## Product findings

1. **P1 — Detail responses are assigned the requested identity without validating the page identity.** `internal/source/parser.go:340` initializes ID/URL/area from the requested URL, but the Restaurant JSON-LD `@id` read at `:351` is never compared. A redirect or wrong-venue response containing a valid practical table is accepted and persisted under another restaurant's ID. Reproducer: request SAMBOA ID `13005012` while returning the genuine sushi fixture whose `@id` is `13294162`; `show` exits 0 and reports SAMBOA's ID/URL with “Sushi Dokoro Isseki Sanchou.” Refresh's `r.ID != previous.ID` guard cannot detect this because `r.ID` was already synthesized from the request. **Fix:** validate a source-declared canonical restaurant identity against the requested ID/route before cache/snapshot replacement; reject mismatches and retain the previous snapshot.

2. **P2 — Effective ranking is never validated, yet results always claim highest-rated order.** `internal/source/discovery.go:443` checks geography/category/budget/keyword but not the active sort control; `:565` unconditionally sets `source_sort: highest_rated`. A source response that switches the selected tab to Most Reserved passes and is stored as highest-rated discovery. Reproducer changes only active-sort markup/route values in a real Tokyo listing; `find` exits 0 with `source_sort: highest_rated`. **Fix:** verify the active Ranking tab/source sort before accepting the listing and on subsequent pages. Inactive navigation links have other sort values even on valid ranking pages, so do not blindly require their `SrtT` value to be `rt`.

3. **P2 — Default summaries erase fetched facility facts.** `internal/cli/tabelog_helpers.go:110` omits `facilities` from the summary's keep list. Against an unmodified captured listing, default `find --agent` drops the known “Credit card accepted” and “Non smoking” tags; explicit full-record projection returns them for the same ID. These are source facts the replay contract says to retain, and losing smoking/payment tags undermines the requested lossless compact output. **Fix:** preserve nonempty facilities in the summary; keep the existing provenance inheritance and projection behavior.

Evidence: [phase17-correctness-repro.json](phase17-correctness-repro.json), [phase17-correctness-repro.py](phase17-correctness-repro.py), [phase17-facilities-repro.json](phase17-facilities-repro.json), [phase17-facilities-repro.py](phase17-facilities-repro.py).

## Generated-template retro candidate

- **P2 — Machine-mode errors before RunE remain human prose.** `internal/cli/root.go:168` delegates flag parsing to Cobra, which prints ordinary errors; the handwritten `tripMachineDiagnostic` wrapper only covers Args/RunE. Actual `--agent find --not-a-real-flag` exits 2 with `Error: unknown flag: --not-a-real-flag`, not a JSON diagnostic. This boundary is emitted by unmodified `internal/generator/templates/root.go.tmpl:215` (same Execute/error flow), so it needs a durable generator repair and matching printed behavior rather than an isolated product patch. It is excluded from product-round convergence per the phase's template escape hatch. Reproducer evidence is in the same correctness JSON above.

## Requested targeted check

**PASS — empty/whitespace required recipe ID does not widen refresh scope.** `internal/mcp/intents.go:90` passes `required=true`; `recipeValueString` trims strings and `appendRecipePositional:114` returns missing=true, so `:91–92` returns “id is required” before running the CLI. Nonempty flag-shaped positional injection remains the separate security review's finding; it is not duplicated here.

## Other reviewed behavior

Snapshot replacement is complete rather than field-merging; notebook transactions preserve notes and membership, drain rows before subsequent queries, and validate selected IDs. Refresh uses bounded context/concurrency and retains failed snapshots while emitting usable partial results and non-success status. Default/projection provenance inheritance and unknown/not-fetched state handling were reviewed; the confirmed facility omission above is the output-loss finding. No additional concrete issue was found in those reviewed paths.
