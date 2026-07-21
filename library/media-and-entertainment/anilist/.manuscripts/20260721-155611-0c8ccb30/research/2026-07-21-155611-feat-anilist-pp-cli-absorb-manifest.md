# AniList Absorb Manifest

**Approval:** approved by the user’s delegated build instruction on 2026-07-21. Every `shipping` row is a binary release obligation. `novel_features_built` must equal the four approved novel command paths exactly; no row may be downgraded to a stub without returning to this gate.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---|---|---|---|---|
| 1 | Search anime and manga with filters | yuna0x0/anilist-mcp (source) | (generated endpoint) page media | Typed flags and agent-readable output. | shipping |
| 2 | Get media details and streaming metadata | yuna0x0/anilist-mcp (source) | (generated endpoint) media get | Scriptable data instead of MCP-only text. | shipping |
| 3 | Search/get characters, staff, and studios | yuna0x0/anilist-mcp (source) | (generated endpoint) page character/staff/studio | Stable CLI and MCP mirror from one tree. | shipping |
| 4 | Search/get users and viewer data | yuna0x0/anilist-mcp (source) | (generated endpoint) user/viewer | Account-aware output when a token is present. | shipping |
| 5 | Browse user anime and manga lists | yuna0x0/anilist-mcp (source) | (generated endpoint) media-list collection/list | Structured list inspection. | shipping |
| 6 | Add, update, and remove list entries | yuna0x0/anilist-mcp (source) | (generated endpoint) save/delete media-list-entry | Direct API surface with generated dry-run conventions. | shipping |
| 7 | Favorite media, characters, staff, and studios | yuna0x0/anilist-mcp (source) | (generated endpoint) toggle favourite | Explicit authenticated mutation path. | shipping |
| 8 | Browse genres and media tags | yuna0x0/anilist-mcp (source) | (generated endpoint) genre-collection/media-tag-collection | Reusable filters for agents. | shipping |
| 9 | Retrieve airing schedules | official AniList GraphQL docs | (generated endpoint) page airing-schedule | Raw schedule input for personal workflows. | shipping |
| 10 | Retrieve recommendations and reviews | yuna0x0/anilist-mcp (source) | (generated endpoint) page recommendation/review | Decision context without a separate server. | shipping |
| 11 | Retrieve activity, threads, comments, notifications, and site statistics | yuna0x0/anilist-mcp (source) | (generated endpoint) activity/thread/comment/notification/site-statistics | Complete community/account surface. | shipping |
| 12 | Public CLI output contract | tamnd/anilist-cli | (behavior in anilist-pp-cli root) --json, --agent, --select, compact output | Agent-native piped output, not a generic page scaffold. | shipping |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Evidence | Long Description | Status |
|---|---|---|---|---|---|---|---|---|
| 1 | Tonight schedule | `anilist-pp-cli schedule tonight` | 10/10 | hand-code | Paginates every authenticated CURRENT anime-list entry and matching schedule page, excludes media not followed by the viewer, and returns only episodes in the explicit IANA-zone local-day interval `[midnight, next midnight)`. | Explicit user request and weeknight-watcher workflow; no incumbent personal join. | Use this command for episodes from anime already on the authenticated viewer’s CURRENT list that air tonight. Do NOT use it for an unfiltered seasonal schedule; use `airing-schedule list` instead. | shipping |
| 2 | Safe episode check-in | `anilist-pp-cli progress check-in` | 10/10 | hand-code | Resolves one unambiguous title or ID with an existing list entry, previews by default, and calls `SaveMediaListEntry` only with `--apply` after a just-in-time progress re-fetch; it rejects regressions and progress above a known episode total, then verifies returned progress. | Explicit user request; raw incumbent mutation lacks dry-run/apply safety. | Use this command to advance one existing personal anime-list entry safely. Do NOT use it to create or broadly edit a list entry; use `save media-list-entry` instead. | shipping |
| 3 | Short backlog pick | `anilist-pp-cli backlog pick` | 10/10 | hand-code | Paginates all PLANNING entries, requires explicit episode/runtime bounds, excludes ineligible and completed media, and ranks candidates using a documented deterministic total order. | Explicit user request; no incumbent personal short-backlog ranker. | Use this command for a deterministic short anime choice from the viewer’s PLANNING list. Do NOT use it to browse the whole catalog; use `media search` instead. | shipping |
| 4 | Catch-up queue | `anilist-pp-cli progress catch-up` | 9/10 | hand-code | Paginates all CURRENT entries and relevant schedules, computes the highest already-aired episode at an explicit `--as-of` instant, and reports gaps without mutation. | Weeknight ritual and observed demand for automatic progress tracking; bounded non-mutating command. | Use this command to find followed shows with aired-but-unwatched episodes. Do NOT use it for only tonight’s releases; use `schedule tonight` instead. | shipping |

## Binary Novel Acceptance Matrix

Every item below is an implementation and focused-test obligation. All predicates in a row must pass; one failure makes that novel command non-shipping.

| Command | Required behavior predicates | Focused test obligations |
|---|---|---|
| `anilist-pp-cli schedule tonight` | 1. `--timezone` accepts only an IANA location and defaults to the process local location. 2. The query window is exactly `[local midnight, next local midnight)` expressed as instants, including DST days. 3. Every page of CURRENT entries and every required schedule page is consumed. 4. Only media IDs in the authenticated viewer’s CURRENT anime list appear in output. | Test an ordinary day, a DST boundary, and an invalid IANA zone. Test multipage CURRENT and schedule fixtures where a second-page followed item must appear. Test that an un-followed scheduled item and an item exactly at next midnight are absent. |
| `anilist-pp-cli progress check-in` | 1. Exactly one media must resolve; ambiguous title matches fail and IDs bypass title search. 2. An existing authenticated list entry is required. 3. Without `--apply`, no mutation request is made and the preview includes before/after progress. 4. `--apply` rejects target progress less than current progress and greater than a known episode count. 5. Immediately before mutation it re-fetches current progress and rejects drift. 6. A successful mutation is accepted only when returned progress equals the requested value. | Test ambiguous title, no list entry, preview-no-mutation, regression, known-total overflow, drift between first read and mutation read, and returned-progress mismatch. Test one valid `--apply` request and assert its exact media ID/progress payload. |
| `anilist-pp-cli backlog pick` | 1. `--max-episodes` and `--max-runtime-minutes` are required positive bounds. 2. Every PLANNING page is consumed. 3. Candidates with completed status, no usable episode/runtime values, or values outside either bound are excluded. 4. Ranking is a documented total order: list priority descending, personal score descending, episode count ascending, duration ascending, media ID ascending. | Test missing/zero bounds, multipage inclusion, each exclusion condition, and a tie fixture that proves every ranking key including final media-ID tie-break. |
| `anilist-pp-cli progress catch-up` | 1. `--as-of` accepts RFC3339 and defaults to invocation time. 2. Every CURRENT page and every required schedule page is consumed. 3. For each media item it computes the maximum episode whose airing instant is `<= as-of`. 4. It emits only positive `(highest aired - stored progress)` gaps and never sends a mutation. | Test explicit as-of before and after an airing instant, multipage inputs, multiple aired schedule entries selecting the highest, zero/negative-gap exclusion, invalid RFC3339, and assertion that no mutation operation is issued. |

## Gate evidence

- **Browser discovery:** skipped silently because the official direct GraphQL surface is documented, schema-backed, and reachable; `browser-browser-sniff-gate.json` records that decision.
- **Reachability:** `POST https://graphql.anilist.co` with `Media(id: 1)` returned HTTP 200 and Cowboy Bebop on 2026-07-21.
- **Incumbent conclusion:** existing public tools are a raw broad MCP wrapper and a generic one-resource CLI scaffold; neither implements the four approved personal workflows.
