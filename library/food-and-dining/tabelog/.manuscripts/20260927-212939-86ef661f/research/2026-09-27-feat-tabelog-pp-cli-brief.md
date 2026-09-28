# Tabelog CLI brief

## API identity

- Source: https://tabelog.com/en/ . Build from public English website responses; no official API or OpenAPI contract established.
- Product: `tabelog-pp-cli`, a small Go executable that helps an agent find, inspect, and compare restaurants and bars for a Japan trip.
- Data: location/category catalogs, ranked restaurant cards, practical restaurant details, and locally saved trip lists. Remote work is read-only. No account or paid key is needed for the pages observed so far.

## Users

These are task personas inferred from the user's stated purpose and linked firsthand reports, not claims about interviewed customers.

- **The trip planner** compares meals across neighborhoods before a Japan visit, balances cuisine and budget, and returns to the same candidates over several planning sessions. The user explicitly approved saved trip lists with notes and offline comparisons.
- **The traveler choosing the next meal** wants a few credible options near the area or station they are visiting. Repeatedly opening venue pages to understand location is a documented pain point: https://www.reddit.com/r/JapanTravelTips/comments/1d8ob21/ . A nearest-station distance is useful but is not a distance from the traveler.
- **The agent assisting a traveler** needs evidence it can cite, clear constraints, and concise results. It must distinguish a source rating from its own preference judgment, preserve unknown facts, and deepen only selected candidates. Rating/sorting confusion appears in https://www.reddit.com/r/finedining/comments/1bokthz/ and https://www.reddit.com/r/JapanTravelTips/comments/1iiy5kg/ . These reports identify needs; current behavior is checked against Tabelog itself.

## Top workflows

1. **Meal scouting:** resolve an area/station and cuisine, search within a budget, and inspect two or three candidates. Support bars as a first-class category. Show source sorting and result coverage.
2. **Trip planning sessions:** save candidates into named lists such as Kyoto dinners or Tokyo bars, with personal notes; resume and compare offline without repeating network requests.
3. **On-trip selection:** revisit saved candidates, inspect hours, closure days, access, payment and reservation information, then follow the source link. Refresh retrieved facts explicitly before relying on them.
4. **Evidence-based alternatives:** compare candidates on source facts, retain missing-data distinctions, and help the agent choose backups without inventing availability, walking times, dietary guarantees, or an authoritative quality score.
5. **Cheap progressive discovery:** discover valid locations/categories and command fields, request a small result set, project relevant fields, and fetch richer sections only when useful.

## Reachability risk

Medium. Native anonymous HTTP returned 200 for the English homepage, Tokyo highest-rated listing, and a restaurant detail during this run. Network access requires the environment's network-capable execution path; initial sandbox DNS failures are not origin blocks. Source resolution and public captures are retained in this run.

The closest wrapper, https://github.com/narumiruna/gurume , has a recent 403 report (issue 88) and transport adjustment (PR 89). Historical wrapper failure does not prove current native Go failure; validate the actual transport before generation and during live E2E. A challenge, drifted page, or incomplete parse must be an explicit error, never a successful empty restaurant list. Runtime must be direct HTTP with bounded work; browser capture is temporary discovery only.

## Source facts and contract hazards

- Official rating explanation: https://tabelog.com/en/help/score . Ratings are weighted, not a simple mean or a conclusive measure of quality. Preserve ratings and review counts; do not exclude all scores below 3.5 by default.
- English Tokyo source sorting: https://tabelog.com/en/tokyo/rstLst/?SrtT=rt is highest rated; the ordinary Tokyo page currently defaults to most reserved by travelers. Source order and local sorting must be labeled distinctly.
- Listing cards expose name/link, source rating, review count, separate lunch/dinner budgets, categories, nearest station and its distance, closure information, facilities, and sometimes awards. Compact discovery should use this one page rather than fetch every result's detail.
- Example detail: https://tabelog.com/en/tokyo/A1301/A130103/13294162/ . Details include restaurant identity, access, business hours, reservations, budgets, and payments. Parse field labels rather than flatten the page into an unbounded text dump.
- English detail JSON-LD uses aggregateRating.ratingCount in the observed page; do not copy a reviewCount-only parser from another language surface.
- Pagination links are authoritative: the observed next listing page is /en/tokyo/rstLst/2/?SrtT=rt.
- Gurume reports that a Japanese keyword path can silently escape the requested area (PR 80). English location resolution, genres, sort and budget parameters need direct contract evidence. Do not reuse Japanese genre constants blindly.
- Browser discovery observed English Ginza resolution through /en/rstLst/?pal=tokyo&LstPrf=A1301&LstAre=A130101&area_datatype=Area2&area_id=Ginza, and a native bar path /en/tokyo/A1301/A130101/rstLst/bar/. English keyword suggestions use /en/suggest/keyword_suggest?keyword=Ginza. Exact schemas and native replay belong to the phase06 evidence.
- The expanded More filters panel dynamically loads /en/search_form and does expose dinner/lunch budget radio controls and JPY minimum/maximum selects. Capture and replay the exact submitted parameters. Prefer validated source filtering. Any additional local criteria must still disclose bounded coverage and unknown-value exclusions; never silently broaden geography.

## Table stakes and competition

Gurume combines Python CLI/TUI/MCP, area/keyword/cuisine search, sorting and pagination, suggestions, structured details, review/menu/course access, and an agent skill. The source-research agent records exact supported methods and other tools in `ecosystem-evidence.md`; use that file when building the absorb manifest. Catalog competitor functionality, but propose only scope that earns its token/resource cost and can be verified. User approval at the feature gate decides the shipping subset.

Differentiate through accurate compact defaults, constraint-aware discovery, saved trip context, deterministic comparison, and measurable request/resource bounds. A TUI, nationwide crawl, image downloader, or built-in LLM is not a prerequisite for the requested product.

## Data layer

- Primary entity: restaurant identified by source ID and canonical URL; keep source attributes separate from user notes/list membership.
- Retain fetched_at, source URL and query/coverage metadata. An offline snapshot must disclose age. Refresh source facts while preserving notes and membership; failure retains the last valid snapshot with a visible error.
- Reuse the generator's local storage if economical. Initialize storage lazily and bound cache growth. No automatic background process, periodic sync, or entire-country index.
- Local search covers saved/fetched entities only and names that scope. Live restaurant discovery has a distinct command so an agent cannot mistake an empty local database for no restaurants in Japan.

## Architecture direction

Separate source retrieval/parsing, discovery policy, local trip state, and CLI rendering. The source adapter owns exact URLs, parsing invariants and source-specific failure classification. The application layer owns limits and source/local semantics. Renderers consume typed records, preserving facts across output modes.

Default to a small set (provisionally five) of concise candidates. Keep readable field names and full identity/URLs; optimize by selection and progressive disclosure rather than lossy truncation. Projection should retain enough envelope metadata to expose stale/partial results. Explicit sections can expand detail, menus or reviews if the approved scope includes them.

Use one listing request after cached location resolution, no detail fan-out, bounded pagination, total timeout, body limits, and a small explicit concurrency bound for refresh. Treat free-text resolution ambiguity as a choice, not a guessed first match. Embed deterministic catalogs only when verified; otherwise cache observed catalogs.

## User vision

- Token efficiency without sacrificing accuracy.
- Discover great restaurants and bars on a Japan trip; creative agent workflows are welcome.
- Low runtime resource use.
- Root gpt-6-astra owns intent, architecture and planning. gpt-6-sol at max reasoning writes code and runs tests.
- Prefer meaningful executable E2E over unit tests that mirror implementation.
- Explicitly approved local saved lists: ratings, budgets, links, user notes, offline comparisons, retrieval times and refresh.
- Apply writing-for-agents: short skill trigger, decision-oriented steps, branch-specific references, help/schema as command truth, no duplicated command catalog.

## Build priorities and acceptance

1. Prove the English search/location/filter/detail transport and contract with native replay.
2. Define a concise typed discovery result with provenance, coverage and unknown values; measure output size.
3. Implement approved discovery and local-list workflows fully, with no placeholders.
4. Exercise actual binary + HTTP adapter + parser + store + output through recorded real-source fixtures and a small live matrix across cities and bars.
5. Measure cold/warm latency, CPU, peak RSS, requests and bytes; document host and tokenizer. Initial targets: default discovery <=1,200 tokens; projection <=500; cached/replay p95 <=400 ms and RSS <=64 MiB. Finalize using representative fixtures, preserving complete facts. Detailed matrix: `acceptance-plan.md`.

## Product thesis

Tabelog's depth becomes useful to agents when they can retrieve a few trustworthy candidates, keep trip context, and progressively inspect the facts needed for a decision. The CLI should reduce repeated browsing and context consumption while keeping the user's choice grounded in the source.
