# Implementation contract

Approved product scope lives in `2026-09-27-feat-tabelog-pp-cli-absorb-manifest.md`; source truth lives in `website-replay-contract.md`. This document settles interfaces and ownership for parallel implementation. Root owns decisions; gpt-6-sol at max writes code and tests.

## CLI interface

- `areas QUERY` returns typed location choices with a usable source URL/selector and enough provenance to repeat the selection. A kind filter distinguishes area, prefecture and station.
- `cuisines [QUERY]` returns source category choices. A documented canonical `bar` slug means the broad source category, with its child taxonomy visible.
- `find --area AREA_OR_SOURCE_URL --cuisine CATEGORY --meal lunch|dinner --budget-min JPY --budget-max JPY --keyword TEXT --limit N --max-pages N` supports optional criteria; location is required. Prefecture slugs from the verified homepage are direct choices. Other human names use typed suggestions/cached verified area choices; ambiguity returns choices and a non-success code. Avoid source-wide searches from an omitted or failed location.
- `show URL_OR_CACHED_ID` inspects a canonical English restaurant URL or a previously fetched ID. An unknown bare ID gets a useful fetch-first error rather than guessed path components.
- `lists add LIST ID --note TEXT`, `lists show [LIST]`, `lists note LIST ID --note TEXT`, `lists remove LIST ID` complete the local notebook. `lists show` without a name lists notebooks. Save already fetched snapshots; do not manufacture restaurant records.
- `lists compare LIST [ID ...]`, `lists refresh LIST [ID ...]`, `lists alternatives LIST --for ID --match area,category [--meal dinner --budget-max JPY]`, `lists audit LIST [--require hours,payment,reservation,dinner_budget] [--max-age 24h]` follow the exact semantics in the brainstorm's Survivors section.

Use global `--agent`, `--select`, `--dry-run`, and `--data-source auto|local|live` consistently. All documented commands support a truthful no-side-effect dry run; unresolved lookups remain identified as planned lookups. Validate invalid flags/values before HTTP. Generated advanced commands remain functional if kept, with concise main help and small MCP exposure. Hide learning/raw endpoint plumbing from the main workflow; learning is disabled in the spec.

## Output and data model

Typed restaurant fields begin with `id`, `name`, `url`, `rating`, `review_count`. Preserve full names/URLs and decimal precision. Store separate lunch/dinner budget objects with original source text and parsed yen bounds; retain listed versus review-based budgets as separate sources. Keep station name/meters, categories and source area identifiers. Details add the practical source facts established by the replay contract.

Each snapshot records fetch time, source URL, source surface/sections obtained, and unknown versus not-fetched state. Per-response metadata records effective criteria, source sort, returned/scanned counts, pages and remaining coverage; cache/local use and age are visible. Default find/show output is concise structured data. Agent output is compact JSON with diagnostics confined to stderr. A find projection such as `--select items.id,items.name,items.rating,items.url` works and retains essential provenance/partial/stale metadata. Ensure field projection semantics are tested, rather than guessing the generator's formatter behavior.

No result may conflate source station vicinity with a geometric user radius. Validate the source's effective selected geography and filters; legitimate station results may cross administrative route boundaries. Source ranking is preserved, including ties. Local alternatives preserve list order and explicitly operate on the saved set.

## Module ownership

The source/build agent owns HTTP, source parsers, catalogs/resolution, normalized restaurant types, find/show/areas/cuisines, generation and root integration. Publish normalized types and the store/source interfaces early for the list agent.

The list agent owns persistence and all list commands once those interfaces are agreed. Use SQLite through the generated store infrastructure where emitted; isolate list memberships/notes from source snapshots. If the profile omits a store, coordinate the smallest SQLite integration with the build agent. The brainstorm documents the required drain-first query pattern and transaction constraints. Keep source parsing outside write transactions; apply validated snapshots atomically.

The test agent owns executable E2E/replay tests and measurement tooling. Its independent oracles are in `proofs/fixtures/oracle.json`. Test public behavior through isolated stores and a local replay server; production source URL validation remains strict. The test transport override must be explicit and documented, with no secret headers in fixtures or logs.

Root writes/reviews the agent docs and checks final scope/architecture. Agents coordinate shared interfaces directly and report blockers promptly. Each agent edits its owned files; the source agent performs the final integration build.

## Resource policy

- Default find limit5 and max-pages1. Caller can increase either explicitly, with hard upper bounds (initially limit50/pages5). Report truncated/remaining coverage rather than silently claiming completion.
- One listing fetch after cached resolution, no detail fan-out; show uses one detail request. Location resolution may require a small cached bootstrap/suggestion request and reports it in proof counts.
- Total source deadline20s and decoded body limit4MiB per response are initial bounds. No retries for blocks/429/drift. Any transient retry is bounded and counted.
- Auto mode uses a fresh cached source response or fetches; live bypasses cache; local makes zero network requests and exposes age or an actionable miss. Suggested TTLs: listings/suggestions15min; details6h. Refresh always requests current detail data.
- Bound raw response cache payload to32MiB, independently of deliberate saved user data. Open local storage only when needed. No background sync, browser process, LLM calls, nationwide ingestion or automatic image fetches.
- Refresh at most20 explicitly selected saved venues per invocation with concurrency at most2 and a bounded operation timeout; a larger list requires a narrower selection. Per-venue failures retain prior snapshots, and partial failure returns non-success status with usable per-ID results.

Record justified adjustments if real source fixtures exceed a proposed limit; user facts and accuracy take precedence over size targets. `acceptance-plan.md` owns the measurement thresholds and final evidence requirements.
