# Tabelog acceptance plan

Scope: a native Go binary for discovering restaurants and bars from public English Tabelog HTML, with persistent offline trip shortlists and comparisons. Test the actual executable, HTTP client, parsers, cache, store, and renderers together. No Tabelog account or booking writes. Shortlist explanations remain conditional on the final feature contract.

## Acceptance matrix

| Workflow / risk | Deterministic binary replay | Small live verification |
|---|---|---|
| Search → shortlist → inspect | Search a realistic Japan trip query; inspect a returned canonical ID; verify identity, URL, area, cuisine, rating, review count, separate lunch/dinner budgets, station distance, and awards against independently annotated HTML. No fabricated values. | Search representative Tokyo, Kyoto, and Osaka areas, including one bar query; inspect two returned entries; manually cross-check core facts in the retrieved source. |
| Source sort and pagination | Default order preserves source order. Highest-rating mode sends the documented source sort and preserves its returned order. A requested limit bounds pages; duplicates are removed by canonical ID, without merging distinct venues with equal names. | Confirm default and highest-rating requests produce the documented source ordering. Avoid fixed-name/rating assertions against changing results. |
| Local sorting | Sort only the fetched set; identify that scope in output. Missing scores sort after known scores; ties preserve source order. Do not imply a global highest-rated result from a locally sorted partial page. | Check one deliberately small sample against its source positions. |
| Accurate compact output | Parse JSON from stdout; assert types, exact values, requested projection, limit, and stable identifiers. Unknown is null/omitted according to the contract, never zero or an empty fabricated fact. CSV/text modes are tested only if approved. Diagnostics stay on stderr. | Reuse cached responses to inspect every approved output mode without extra origin requests. |
| Shortlist explanation, if approved | Every explanation names supported query criteria and actual evidence. Separate source rank from local selection. Missing budget/distance information remains explicit. Never describe station meters as distance from the user. | Manually check each clause against the search criteria and source facts. |
| Cache and offline | First request fetches once; immediate repeat fetches zero times and returns equal facts. Expired cache behavior follows the contract. Offline cache hit makes zero requests; miss produces an actionable nonzero error. Cache writes are atomic; corrupt entries are diagnosed or refetched within the request budget. | Repeat search/inspect with shared cache and confirm zero fresh requests. |
| Persistent trip shortlist | Save a returned ID with trip/list and user notes; list/inspect offline; update/remove; restart and verify persistence. Re-saving an ID must follow the duplicate contract and preserve notes. Show rating, separate budgets, canonical link, and fetched_at/freshness. Explicit refresh updates source facts/time while preserving trip membership and notes; failed refresh retains the previous snapshot and marks the failure. | One disposable local trip list; refresh one saved venue; no account or origin mutation. |
| Offline comparisons | Compare saved venues with zero HTTP after restart. Preserve each venue's rating, budgets, links, notes, fetched_at, and freshness; distinguish unknown values from zero and differing retrieval times. Do not infer freshness or proximity from sort order. | Compare two saved venues; verify the factual differences against their captured sources. |
| Source failures | HTTP 200 challenge/login/block HTML, 403, 429, 5xx, timeout, truncated body, oversized body, and structural drift produce nonzero structured errors. Genuine empty results succeed only with recognized empty-result evidence. A partially parsed page cannot silently look complete. | An inaccessible origin is reported as blocked live verification, not an empty success or replay-only pass. |
| Input and transport bounds | Invalid args/IDs fail before HTTP. Unicode query encoding survives. Only approved English source routes/hosts are fetched; unrelated redirects are rejected. Assert no asset requests, detail fan-out, or unrequested pages. Retries and pagination are bounded. | Count requests and bytes for each cold command; confirm effective limits. |

## Parser fixtures worth keeping

Use a few captured, public English search/detail pages with provenance URL and capture date. Annotate expected facts independently before running the parser. Add targeted mutations rather than whole-output snapshots:

- A rated restaurant, a bar, and a venue with unknown score/budgets; lunch and dinner values differ.
- Grouped review counts, decimal ratings, Unicode/entity-containing names, duplicate cards, equal names with different IDs, and a missing optional section.
- Nearest-station meters explicitly remain station distance. Awards preserve the source label; absent/unavailable awards follow the agreed unknown/empty distinction.
- Review count, review excerpt, reviewer score, and restaurant aggregate score remain distinct. Full review text is fetched/emitted only when requested.
- Reordered sections or extra wrappers retain facts; removed required listing/detail anchors trigger drift. A known empty-result page, challenge page, and ordinary unrelated page must have different outcomes.
- Price filter requests use the exact observed integer-code mapping in `proofs/fixtures/oracle.json`, including unbounded code 0 and Dinner=2/Lunch=1. Literal yen amounts must never be sent as source codes; unsupported values require the final contract's explicit policy.

The replay server serves real HTML and records method/path/query, response bytes, request count, and attempted fan-out. Each case launches the compiled CLI with an isolated config/cache directory and a documented test transport override. It validates stdout/stderr and exit status directly, without shell pipelines. Do not add implementation-shaped unit tests that merely restate selector code.

## Proposed budgets

Finalize these against the agreed schema and representative captures; never truncate names, URLs, facts, or precision to pass them.

| Measurement | Acceptance target | Reason |
|---|---|---|
| Default search, 5 entries | ≤1,300 tokens and ≤5 KiB UTF-8 stdout | Retains identity, selection facts, known facilities, and compact provenance; excludes repeated boilerplate/raw HTML. |
| `id,name,score` projection, 10 entries | ≤500 tokens and ≤2 KiB | Projection should materially reduce agent context. |
| Default inspect | ≤1,600 tokens and ≤8 KiB | Structured practical facts; reviews/descriptions require explicit expansion. |
| Actionable error | ≤150 tokens; no HTML dump | Keeps failure handling cheap and useful. |
| Replay/cached command | p95 wall ≤400 ms; CPU ≤250 ms; peak RSS ≤64 MiB | Practical Go CLI bounds on the run host, measured across 20 subprocess runs after fixture setup. Report host and measurement tool. |
| Default cold search or inspect | One source HTML fetch; zero assets/detail fan-out | Avoids turning a shortlist into an implicit crawl. Any essential bootstrap fetch needs an explicit, cached contract exception. |
| Explicit pagination/retry | Requests ≤requested page cap + documented retry cap; no retry for recognized blocks | Predictable traffic; transient retry policy must be specified before testing. |
| Response safety | ≤4 MiB decoded HTML per fetch; bounded overall timeout | Caps parser memory and hanging commands. Adjust only if valid captures require more, with recorded evidence. |
| Live acceptance session | Target ≤12 uncached source requests; reuse cache for mode checks | Covers representative workflows without repeated crawling. Mandatory command coverage may justify additional counted requests. |

Measure tokens with the run's chosen tokenizer and record its name/version; retain byte counts as a portable secondary metric. Apply hard output budgets to fixed representative fixtures and report live variation. Record cold/warm CPU, RSS, latency, requests, and bytes separately; origin latency is not a parser-performance failure.

Revision at phase17: the initial 1,200-token search target was evaluated against summaries that omitted known facility tags. After restoring those facts, 20 actual subprocess samples measured 1,247 tokens and 3,897 bytes at p95 for the default five-entry source fixture. The approved fixture target is now 1,300 tokens; accuracy takes precedence over an arbitrary size target. The earlier facility-omitting 1,188-token result is retained as history and is not a lossless accuracy baseline. Original and revised measurements are in `proofs/phase17-cluster-a-measurements/`; the projected ten-entry result remains 403 tokens/1,199 bytes.

## Gate and evidence

Enumerate every approved leaf from `agent-context`: help, realistic happy path, JSON parsing/projection where supported, and one meaningful invalid-input/error path. Every mandatory replay case must pass. Every approved feature requires live evidence where it depends on the origin; replay alone does not establish current source compatibility.

Once the binary and contracts exist, run the Printing Press full live dogfood matrix under the root-selected depth and save its report. The Printing Press runner owns the phase acceptance marker; this document neither writes nor substitutes for it. Keep raw captures, annotated expectations, subprocess results, resource measurements, and request logs as separate proof artifacts. A failing flagship workflow, inaccurate facts, silent block/drift, or unbounded network work prevents acceptance.
