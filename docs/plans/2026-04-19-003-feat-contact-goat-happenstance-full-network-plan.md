---
title: "feat(contact-goat): unlock full Happenstance network graph (1st, 2nd, 3rd degree)"
type: feat
status: completed
date: 2026-04-19
---

# feat(contact-goat): unlock full Happenstance network graph (1st, 2nd, 3rd degree)

## Overview

Happenstance has no public API. contact-goat is a sniffed web-client wrapper: it replays the exact HTTP calls the Happenstance web app makes against the user's browser session cookies. That framing drives everything in this plan.

contact-goat-pp-cli today exposes only a sliver of what the web app can do. `coverage` and friends-based commands replay a narrow sniff of `/api/friends/list`, which on the web app is the top-connectors widget, not the full contact graph (Matt has 3 total there). The broader LinkedIn-connections CSV synced to Happenstance on 2025-07-09, and the 2nd/3rd-degree paths the web app surfaces in search, are not yet sniffed, so they are unreachable from the CLI. On top of that, the Clerk session refresh flow the CLI replays is a stale sniff (returns `HTTP 404` from `/v1/client/sessions/{sess}/tokens`), so even the narrow endpoints die roughly once per minute and force a Chrome cookie re-import.

This plan captures the sniff-and-replay work to make contact-goat deliver the three network tiers the web app already delivers:

1. My 1st-degree network (the synced LinkedIn CSV, queryable by company / title / industry).
2. My friends' networks (2nd-degree via the web app's people-search which walks contacts' public graphs).
3. Friends of friends (3rd-degree paths surfaced as warm-intro candidates with rationale).

The work is gated on capturing fresh HARs of each web flow, replaying them as Go HTTP calls, and hardening against schema drift since no contract exists. There is no "ask Happenstance for docs" path.

## Problem Frame

Known failure modes observed in-session on 2026-04-19:

- `coverage "Warner Bros"` returned 0 for 1st-degree and 2nd-degree, yet LinkedIn web search trivially shows dozens of 2nd-degree Warner Bros connections with named mutuals.
- `coverage "Weber"` returned 0, yet Jennifer Bonuso (President, Americas at Weber Inc, current) is a confirmed 1st-degree LinkedIn contact of Matt's and lives in his synced Happenstance CSV.
- Every Happenstance-touching command fails within about 60 seconds of auth with `clerk session refresh failed: clerk refresh: HTTP 404`. Doctor reports cookies valid, JWT about to expire, but `MaybeRefreshSession` errors out.
- `prospect` and `search` either fall through to empty results or hit `/api/dynamo` without a request_id and return `HTTP 400`.
- `dossier` / `waterfall` call `happenstance research`, which returns `empty` rather than using the actual people-search endpoint that powers the web app.

Root causes we know about:

- `internal/client/cookie_auth.go` builds refresh URL as `POST https://clerk.happenstance.ai/v1/client/sessions/{sess}/tokens?__clerk_api_version=2025-11-10`. Either the session id parse is wrong for Matt's current cookie shape or the endpoint / API version is stale. Sniffing the Chrome refresh call will resolve which.
- `internal/cli/coverage.go` relies only on `fetchHappenstanceFriends()` which hits `/api/friends/list`. There is no wrapper for the people-search endpoint that actually searches the LinkedIn-contacts graph.
- `internal/cli/promoted_research.go`, `promoted_dynamo.go`, `promoted_friends.go`, `promoted_uploads.go` exist but only one api interface ("suggested-posts") is exposed at the `api` top-level command. The promoted_* files do not wire into the broader graph commands.
- contact-goat's side of the upstream `linkedin-scraper-mcp` contract is stale: `get-person` passes `linkedin_url` and a list-typed `sections` to an MCP that now requires `linkedin_username` and a string-typed `sections`; `search_people` passes `limit` which the MCP rejects.

## Requirements Trace

- R1. A single-command answer to "who at company X" must surface Matt's 1st-degree LinkedIn contacts from the Happenstance graph, not just the 3 top connectors from `/api/friends/list`.
- R2. "Who at company X" must also surface 2nd-degree paths with identified mutual connections (the web app already renders this; the CLI must match).
- R3. `warm-intro <person>` must surface 3rd-degree friends-of-friends candidates with concrete rationale ("X knows Y via shared employer Z"), not just the top-3 Happenstance friends by raw connection count.
- R4. Happenstance-touching commands must stay authenticated across a long-running session without repeated Chrome cookie imports. A single successful `auth login --chrome` should last at least as long as the Chrome browser session does for the same cookies.
- R5. The existing failure mode where a Happenstance refresh error silently returns zero results must be replaced by a clearly labeled error so users know the result set is not empty-but-clean.
- R6. contact-goat must keep working against the current upstream `linkedin-scraper-mcp` (schema drift between versions must not mute results).
- R7. Doctor output must surface graph coverage ("Happenstance: linkedin_ext contacts: N synced, last refreshed YYYY-MM-DD"), not just cookie presence.

## Scope Boundaries

In scope:
- contact-goat-pp-cli Go code under `library/sales-and-crm/contact-goat/`.
- The existing `auth login --chrome` + Clerk refresh flow.
- Happenstance endpoints reachable with the session cookie (no new credentials).
- Minimal wrappers for the LinkedIn MCP side to stop dropping results.

Explicitly out of scope:
- Upstream `stickerdaniel/linkedin-mcp-server` work (tracked in `~/.osc/plans/2026-04-19-032-feat-linkedin-mcp-search-people-connection-filters-plan.md`). This plan only fixes the contact-goat side of the contract (arg names, dropped params).
- New paid Deepline credits beyond the existing BYOK and managed flows.
- Building a Happenstance clone. If Happenstance's web endpoints cannot answer a tier (e.g. industry), the plan records that and stops.
- Writing a persistent server. Everything remains a local CLI + cookie-jar.

## Context and Research

### Relevant Code and Patterns

- `internal/client/cookie_auth.go:201-268` - `refreshClerkSession` builds the refresh URL against `clerk.happenstance.ai/v1/client/sessions/{sess}/tokens`. The 404 means either the sessionID is wrong or the endpoint has moved. Fix unit 2 replaces the construction after sniffing.
- `internal/client/cookie_auth.go:147-196` - `clerkSessionID` has three-shape parsing (bare `sess_...`, JSON envelope, sha256-prefixed). If shape 3 is misclassifying the current Chrome value, the returned sessionID is truncated and the POST will 404.
- `internal/cli/coverage.go:42-125` - the current two-source implementation. Unit 4 expands source 1 to call the new people-search wrapper.
- `internal/cli/promoted_research.go`, `promoted_dynamo.go`, `promoted_uploads.go`, `promoted_friends.go` - existing thin wrappers. These are the scaffolding that the new commands will extend.
- `internal/cli/dynamo_list-recent-searches.go` - proves the `/api/dynamo` endpoint is reachable for reading prior searches. The create-search flow (POST) is the missing half and is what unit 3 adds.
- `internal/cli/search.go:1-218` - current `search` currently calls the dynamo retrieval endpoint with no request_id, which is why it 400s. Unit 3 reroutes `search` to the create-search flow.
- `internal/cli/linkedin.go` - where contact-goat builds MCP tool calls. Unit 6 fixes the arg-name drift (`linkedin_url` -> `linkedin_username`, list-typed `sections` -> comma-joined string, strip unsupported `limit`).

### Institutional Learnings

- From `feedback_pp_update_before_run.md`: always `go install @latest` + `git pull plugin` + restart before any PP CLI run. Applies to validation of this plan's output.
- From `feedback_pp_go_install_goprivate.md`: `GOPRIVATE='github.com/mvanhorn/*'` is required to skip sumdb 404 when installing contact-goat builds during dogfood.
- From `feedback_debug_before_reply.md`: run the new tests and verify the 1st + 2nd + 3rd degree behaviors against a live session before announcing the plan as complete.
- From this session on 2026-04-19: contact-goat silently absorbing both `clerk refresh failed` and LinkedIn MCP validation errors into empty results caused 40+ minutes of confusion. R5 exists because of this.

### External References (conditional)

There are no Happenstance API docs. The authoritative reference is the sniff capture produced in unit 1, against Matt's signed-in Chrome tab. Clerk's session-refresh docs at clerk.com will only be consulted after the sniff shows which Clerk call signature the Happenstance frontend is currently emitting, to confirm we interpret fields the same way Clerk's server does.

## Key Technical Decisions

- Decision: everything in this plan is sniff-first. Before any Go change lands, unit 1 must produce a HAR capture of the relevant web flow. No endpoint gets added to the client on speculation. Rationale: there is no Happenstance API contract; the only source of truth is the live browser session, and the codebase comments (`clerk_active_context` three-shape parsing, dynamo request_id polling) show prior sniffs have drifted.
- Decision: capture HARs with Chrome DevTools Network -> "Save all as HAR with content", store under `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-YYYY-MM-DD/`. Redact JWT and cookie values before committing. Rationale: HARs give verb, path, headers, payload, and response as one archive; redacted HARs can be replayed in unit tests.
- Decision: do not replace the Clerk cookie-based auth with an API key flow. Rationale: Happenstance does not offer a supported user API key; the cookie session flow the web app already uses is the only honest surface. The fix is to re-sniff the refresh call and keep it alive, not to avoid refresh.
- Decision: model the broader people-search as a new client method (`client.SearchPeopleByCompany`) plus an extension to `coverage.go` that merges its results before the LinkedIn fallback. Rationale: the existing coverage flow already has the merge + rank pipeline, so grafting a new source in is cheaper than a parallel command.
- Decision: treat 3rd-degree "friends of friends" as a warm-intro enhancement, not a top-level command. Rationale: `warm-intro` already scores candidates and has the scaffolding; adding a traversal layer is cleaner than a parallel command.
- Decision: fail loudly on schema drift. Every sniff-derived client method decodes into a strongly-typed struct; unexpected payload shape returns a typed error that mentions the endpoint path and the missing field. Rationale: sniffed endpoints drift without notice; silent-empty on drift is the worst failure mode, and R5 demands loud.
- Decision: ship a sniff-replay test harness. Each new client method has a unit test that replays its captured HAR against an `httptest.Server`. Rationale: production drift is detectable only by comparing live responses to the last known-good HAR; if the replay still passes but the live call fails, we know Happenstance changed, not us.
- Decision: fail loudly at the CLI layer too. Any Happenstance or LinkedIn sub-call that errors must surface in the CLI output as a labeled warning and in the JSON output as a `source_errors` key. Rationale: R5 (silent empty results are the worst UX).
- Decision: `doctor` gets a new line item for "Happenstance: graph coverage" driven by the `uploads` endpoint's `statuses.linkedin_ext` block, plus a line that distinguishes "last sniffed YYYY-MM-DD" from "live refresh succeeded within Nm ago". Rationale: R7, and a staleness indicator tells Matt when a re-sniff is overdue.

## Open Questions

### Resolved During Planning

- Q: Is the Clerk refresh endpoint path wrong, or is the sessionID parse wrong?
  - A: Unknown without sniffing. Unit 1 captures the real request and resolves this. If the path changed, update the constant. If the parse is off, fix the cookie parser. If both, do both.
- Q: Does Happenstance expose a real people-search on the LinkedIn-contacts graph?
  - A: Yes - the web app does it in the search box. `/api/research` 204s on POST in this session, which means the server accepted the request; the pattern is async. The dynamo-style retrieval endpoint is already half-wrapped. Unit 3 wires the POST half.
- Q: Does the /ce:plan call need external docs research?
  - A: No. Repo patterns are rich, the relevant files are all in one package, and the authoritative reference is Matt's own Chrome session.

### Deferred to Implementation

- What is the exact request shape for the Happenstance people-search POST? Deferred because it depends on the sniff output.
- What concrete header(s) does the Clerk refresh now require? Deferred to sniff.
- Will 3rd-degree traversal need paginated dynamo polling? Deferred until unit 3 lands and we can measure real latency.
- Which uploads.statuses fields are safe to key against for the doctor coverage line? Deferred to unit 7 after uploads payload is inspected.

## Implementation Units

- [ ] Unit 1: Sniff the Happenstance web app

Goal: capture the exact network calls the Happenstance web app makes for every flow this plan needs. Every downstream unit replays these captures. This is the foundational unit.

Requirements: R1, R2, R3, R4 (all downstream units depend on this capture).

Dependencies: none. Must complete before any other unit starts.

Files:
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/README.md` (index and endpoint summary)
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/clerk-refresh.har` (the silent refresh flow the web app runs while idle)
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/people-search-by-company.har` (company search from the web app's search box)
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/people-search-dynamo-poll.har` (the async-poll half if the people-search is async)
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/mutuals-for-person.har` (opening a specific person's mutual-connections view)
- Create: `library/sales-and-crm/contact-goat/.manuscripts/happenstance-sniff-2026-04-19/uploads-status.har` (the authenticated uploads endpoint so doctor can read linkedin_ext freshness)

Approach:
- Open Happenstance in Chrome with DevTools Network tab open. Turn on "Preserve log". Enable "Record network log".
- Clear the log, then wait about 90 seconds with the tab visible so the silent Clerk refresh fires; capture that as `clerk-refresh.har`.
- Clear the log, run a company search in the web app for "Warner Bros."; capture as `people-search-by-company.har`. If the UI shows a loading state that later populates, also capture the polling round as `people-search-dynamo-poll.har`.
- Clear the log, open one person in the results, click whatever the web app calls "mutual connections" or "how you're connected"; capture as `mutuals-for-person.har`.
- Clear the log, navigate to the settings / uploads view; capture as `uploads-status.har`.
- For each HAR, before committing: redact `__session` JWTs, `__refresh_*` tokens, and any `Authorization: Bearer` headers. Leave paths, shapes, and non-sensitive keys intact.
- Write `README.md` as an index: one section per HAR with verb, path, required headers, body shape, response JSON keys, pagination or polling notes.

Execution note: research-only; no Go code changes in this unit.

Patterns to follow: existing `.manuscripts/` convention in the contact-goat tree. Filename convention matches other CLIs' archived research dumps.

Test scenarios: none (documentation artifact). Verify by diffing the sniff index against the checklist below.

Verification:
- All 5 HAR files exist and none contain raw JWT or refresh-token values.
- README.md enumerates each endpoint's verb, path, at least 3 request headers observed, request body schema (or "none"), response top-level keys, and any async polling pattern.
- README.md explicitly names whether the people-search is synchronous or dynamo-async.
- An engineer unfamiliar with the CLI can read the README and know exactly what to POST to reproduce each flow.

- [ ] Unit 2: Fix Clerk session refresh

Goal: make `MaybeRefreshSession` succeed so that long-lived CLI sessions stop dying every 60 seconds.

Requirements: R4, R5.

Dependencies: unit 1.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/client/cookie_auth.go`
- Test: `library/sales-and-crm/contact-goat/internal/client/cookie_auth_test.go` (create if missing)

Approach:
- Update `clerkBaseURL`, `clerkAPIVersion`, and the refresh URL template to match what the sniff showed.
- Fix `clerkSessionID` if shape-3 parsing is returning a truncated id.
- Keep the 2s refresh-collapse behavior. Keep the concurrency lock.
- On 401 / 404 response, include the response's `x-clerk-auth-*` headers in the returned error. Rationale: surfacing the server's own "why" beats opaque "HTTP 404".
- Update `MaybeRefreshSession` to also proactively re-seed the cookie jar if `__refresh_*` is present but `__session` is missing (e.g. after Chrome rotated the session cookie).

Patterns to follow: existing error wrapping style in this file. Existing cookie-jar API.

Test scenarios:
- Happy path: given a jar with a valid refresh cookie and a near-expired session JWT, `MaybeRefreshSession` returns nil and the jar now holds a non-expired `__session` cookie.
- Edge case: missing `clerk_active_context` -> returns "no clerk session id in cookie jar" error; does not issue a request.
- Edge case: shape-3 sha256-prefixed cookie returns a non-truncated sessionID.
- Error path: Clerk returns 404 -> error message includes the server's `x-clerk-auth-message` when present, and does not log JWT bodies.
- Integration: two concurrent callers that both see expired JWT issue at most one refresh (the second collapses).

Verification:
- A scripted 5-minute loop (`for i in {1..20}; do contact-goat-pp-cli friends --agent >/dev/null; sleep 15; done`) completes without a single `clerk session refresh failed` on stderr.
- `doctor` reports "session JWT: valid" immediately after refresh.

- [ ] Unit 3: Add Happenstance people-search client + `hp people` CLI

Goal: expose the Happenstance people-search that powers the web app's search box. This is the single endpoint that unlocks 1st + 2nd degree.

Requirements: R1, R2, R5.

Dependencies: unit 1, unit 2.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/client/client.go` (add `SearchPeopleByCompany`, `SearchPeopleByQuery`).
- Create: `library/sales-and-crm/contact-goat/internal/cli/promoted_people_search.go` (cobra wiring).
- Modify: `library/sales-and-crm/contact-goat/internal/cli/root.go` (register new command).
- Test: `library/sales-and-crm/contact-goat/internal/client/client_people_search_test.go`
- Test: `library/sales-and-crm/contact-goat/internal/cli/people_search_test.go`

Approach:
- Implement the POST request shape from the sniff doc. Include the dynamo polling loop if the sniff shows async.
- Normalize results into the existing `flagshipPerson` type; fill `Sources: ["hp_people_search"]` and `Relationship: "happenstance_graph"` for direct-contact rows, `"2nd_degree"` for walked rows, with `Rationale` populated from the server's own reasoning blob.
- Expose as `contact-goat-pp-cli hp people <query>` (new command) for direct use. The `coverage` refactor in unit 4 calls the same client method.
- On upstream error, return a typed error that `coverage` and `prospect` can surface as a labeled warning rather than eating.

Execution note: implement test-first. Record a happy-path HAR from the sniff doc and replay it against a mock HTTP server in unit tests.

Patterns to follow: existing `internal/client/client.go` HTTP helper style, existing `promoted_*` cobra patterns, existing `flagshipPerson` shape.

Test scenarios:
- Happy path: `SearchPeopleByCompany("Weber Inc")` against a fixture server returns a list that includes Jennifer Bonuso with `Relationship == "happenstance_graph"`.
- Happy path: `SearchPeopleByCompany("Warner Bros")` returns non-zero rows with `Rationale` populated from mutual-connection reasoning.
- Edge case: zero-result query returns empty slice, not nil, not error.
- Error path: server 500 is returned as a typed error that mentions "happenstance people-search".
- Error path: dynamo poll exceeding a deadline returns a typed timeout error, not a hang.
- Integration: `contact-goat-pp-cli hp people "Weber" --agent` exits 0 and emits JSON with a `source_errors` key that is empty on happy path and populated on failure.

Verification:
- Running against Matt's live Happenstance session, `hp people "Weber Inc"` lists Jennifer Bonuso with `Relationship: happenstance_graph`.
- Running `hp people "Warner Bros."` lists a non-trivial set of 2nd-degree candidates with mutual-connection rationales.

- [ ] Unit 4: Expand `coverage` to use the graph search

Goal: make `coverage <company>` answer the "do I know anyone at X" question for real by calling the new people-search first.

Requirements: R1, R2, R5.

Dependencies: unit 3.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/cli/coverage.go`
- Test: `library/sales-and-crm/contact-goat/internal/cli/coverage_test.go` (create if missing; expand if present)

Approach:
- Replace the `sources["hp"]` branch with: call `SearchPeopleByCompany` first, then as a fallback only (not in addition), call `fetchHappenstanceFriends` and filter for matches against the company argument. Friends-list matches keep their existing `hp_friend` tag and outrank graph-search results only when they also appear in the graph result (de-dupe by Happenstance uuid).
- Add a `source_errors` map to the JSON output when any sub-source errored, so callers can distinguish "empty because nobody is there" from "empty because the call failed".
- Preserve the existing LinkedIn-search branch and the mutual-hydration step.
- Preserve backward-compatible JSON shape; `source_errors` is additive.

Patterns to follow: existing merge + rank pipeline in coverage.go.

Test scenarios:
- Happy path: graph search returns 10 people at Weber Inc; LinkedIn search returns 10 different people; final result has 20 de-duped rows with correct relationship tags.
- Happy path: graph search and friends-list both contain Jennifer Bonuso; output has one row tagged `happenstance_graph` (graph wins; friends-list tag is merged in, not duplicated).
- Edge case: empty query returns argument error, no network calls.
- Error path: graph-search fails -> output still contains LinkedIn results, and JSON output contains `source_errors["hp_people_search"]` with a non-empty message.
- Integration: `--source hp` with graph-search down yields an empty result set AND a non-empty `source_errors`. The text output says so (R5).

Verification:
- `contact-goat-pp-cli coverage "Weber Inc" --agent` returns Jennifer Bonuso in the results.
- `contact-goat-pp-cli coverage "Warner Bros." --agent` returns a non-zero count with graph-search rows.
- `--source hp` with a manually-broken cookie jar returns zero count and a populated `source_errors` block.

- [ ] Unit 5: Surface 3rd-degree warm-intro candidates

Goal: `warm-intro <target>` returns friends-of-friends candidates with concrete rationale, not just Matt's top-3 Happenstance friends by raw connection count.

Requirements: R3.

Dependencies: unit 3.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/cli/promoted_people_search.go` (add a `SearchMutualsAround` client method or similar traversal helper).
- Modify: the warm-intro command file in `internal/cli/` (it exists; find by `grep warm-intro` in the cobra registration and extend).
- Test: `library/sales-and-crm/contact-goat/internal/cli/warm_intro_test.go`

Approach:
- From the sniff doc, identify whether Happenstance exposes a people-in-common endpoint (for a target LinkedIn URL or UUID). If yes, use it. If not, approximate by calling people-search scoped to the target's current employer and returning the top N candidates tagged as "shared_employer" rationale.
- Score 3rd-degree candidates below 1st/2nd but above "raw top-connector" fallback. When a candidate has both a shared-employer match and a Happenstance-friend tag, boost the score.
- Replace the current `--sources hp` behavior that returns Matt's top-3 Happenstance friends with a clear "no evidence-based warm-intro found, falling back to top-N connectors" label so users know the rationale is weak.

Patterns to follow: existing warm-intro scoring pipeline.

Test scenarios:
- Happy path: target = Alonso Velasco (/in/alonsovelasco/). Output includes at least one warm-intro candidate whose rationale is "shared employer Warner Bros. Entertainment" or equivalent, with score above the weak fallback.
- Edge case: target has no resolvable employer -> falls back to LinkedIn sidebar-recommendation scoring (existing behavior).
- Error path: graph-search down -> warm-intro degrades to LinkedIn-only sources with a `source_errors` entry.
- Integration: `--sources hp` with a real target in your network produces at least one candidate with a non-generic rationale ("2 mutuals: X, Y" or "shared employer Z"), not the current "(N connections)" fallback.

Verification:
- `warm-intro https://www.linkedin.com/in/alonsovelasco/ --sources hp --agent` returns at least one candidate whose rationale names a concrete link (shared employer, mutual name, shared group), not just a raw connection count.

- [ ] Unit 6: Fix contact-goat side of LinkedIn MCP contract

Goal: stop dropping results due to stale arg names. This is the contact-goat-side half of the broader upstream fix tracked in the OSC plan.

Requirements: R6.

Dependencies: none.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/cli/linkedin.go`
- Modify: any MCP bridge helper that serializes tool calls (grep for `linkedin_url` and `sections`).
- Test: `library/sales-and-crm/contact-goat/internal/cli/linkedin_mcp_contract_test.go`

Approach:
- Replace `linkedin_url` arg name with `linkedin_username` on all calls that now target the upstream method.
- Join `sections` list to a comma-separated string before serialization.
- Drop the unsupported `limit` arg on `search_people` (upstream does not accept it; currently breaks prospect).
- Where contact-goat wants a post-fetch limit, apply it client-side after the MCP call returns.

Patterns to follow: existing MCP tool-call serialization in `internal/mcp/`.

Test scenarios:
- Happy path: `get-person alonsovelasco --sections contacts` constructs an MCP call with `{"linkedin_username":"alonsovelasco","sections":"contacts"}`, not `{"linkedin_url":...,"sections":["contacts"]}`.
- Happy path: `prospect "company=Warner Bros"` constructs an MCP `search_people` call without `limit` in the payload.
- Edge case: input is a full LinkedIn URL -> normalize to username before serialization.
- Integration: against a live upstream MCP, `get-person alonsovelasco` returns a non-empty payload.

Verification:
- `contact-goat-pp-cli linkedin get-person alonsovelasco --agent` exits 0 with a populated profile.
- `contact-goat-pp-cli prospect "Warner Bros" --agent` no longer emits the `Unexpected keyword argument 'limit'` warning.

- [ ] Unit 7: Doctor + dogfood updates

Goal: doctor surfaces graph coverage; dogfood-results.json exercises the three-tier behavior end-to-end.

Requirements: R4, R7.

Dependencies: units 2, 3, 4, 5, 6.

Files:
- Modify: `library/sales-and-crm/contact-goat/internal/cli/doctor.go`
- Modify: `library/sales-and-crm/contact-goat/dogfood-results.json` (regenerate via whatever the existing dogfood runner is)
- Modify: `library/sales-and-crm/contact-goat/SKILL.md` (document new `hp people` command and new `source_errors` field)

Approach:
- Add a doctor line: "Happenstance: graph coverage: linkedin_ext: N synced, last refresh YYYY-MM-DD" pulled from the `uploads` endpoint response.
- Add a doctor line that distinguishes cookie presence from session validity from refresh-fresh success. The current "will refresh on next request" line is misleading when refresh has actually been failing.
- Extend dogfood to cover: (a) `coverage` returning graph rows, (b) `hp people` returning non-zero, (c) `warm-intro` returning evidence-based rationale, (d) `source_errors` being empty on happy path.
- Update SKILL.md to document the new command and the `source_errors` field shape.

Execution note: skill changes must also trigger `go run ./tools/generate-skills/main.go` and a `plugin/.claude-plugin/plugin.json` version bump per the repo AGENTS.md convention.

Patterns to follow: existing doctor layout; existing dogfood JSON schema; existing SKILL.md sections.

Test scenarios:
- Happy path: `doctor` on a freshly-logged-in session prints the graph coverage line with a non-zero contact count.
- Edge case: `doctor` with no linkedin_ext synced prints "no LinkedIn contacts synced yet" (not a silent blank).
- Error path: `doctor` with a broken refresh prints "session JWT: expired; refresh FAILED: <reason>" rather than the current optimistic "will refresh on next request".
- Integration: dogfood runner reports all three tiers (1st/2nd/3rd) resolved for a known-good target.

Verification:
- `doctor` output on Matt's live machine includes a non-zero linkedin_ext contact count.
- `dogfood-results.json` regenerates clean with no silent-empty results.
- `plugin/skills/pp-contact-goat/SKILL.md` regenerates cleanly and the plugin version bump is committed.

## System-Wide Impact

- Interaction graph: `coverage`, `prospect`, `warm-intro`, `dossier`, `engagement`, `stale`, `since`, and `tail` all read from the same Happenstance graph. Units 2 + 3 raise the ceiling for every one of them. Unit 4 makes `coverage` use it concretely. Units 5 wires one more consumer. The rest inherit improvements automatically.
- Error propagation: today silent-empty is the failure mode. The `source_errors` convention introduced in unit 4 must be honored by every consumer of the graph search. If unit 3 lands but `prospect` and `warm-intro` do not propagate `source_errors`, the plan regresses R5 for those surfaces. Unit 5 explicitly propagates it; `prospect` should be audited when it lands.
- State lifecycle: the dynamo request_id lifecycle is new client-side state. If the client drops a request_id before polling completes, the user sees a stale "empty" result. The client should hold the request_id through the polling window, with a clear timeout.
- API surface parity: new command `hp people` is additive. Existing JSON outputs gain an additive `source_errors` field. Any consumer parsing the JSON as "no unknown keys" should be checked (none in the repo itself; external users of the JSON schema are Matt's own agent workflows).
- Integration coverage: replay-based tests against recorded sniff HARs cover request-shape correctness. Live-graph tests against Matt's own session cover the "did Jennifer Bonuso actually come back" question. Both matter; unit tests alone will not catch schema drift on Happenstance's side.
- Unchanged invariants: the existing `/api/friends/list` path and its `hp_friend` tag continue to work. No command changes output shape in a backward-incompatible way. BYOK and Deepline flows are untouched.

## Risks and Dependencies

| Risk | Mitigation |
|------|------------|
| Happenstance web endpoints have no public contract and may change | Sniff doc is dated; every client method decodes into a typed struct; unit tests replay the captured HAR; schema drift surfaces as a loud typed error rather than silent empty; doctor shows "last sniffed YYYY-MM-DD" so staleness is visible |
| Sniffs become stale between this plan and the next Happenstance deploy | Re-sniff is cheap and local (one HAR export per flow); the sniff directory is timestamped so the next revision sits next to the old one for diffing |
| Clerk API version constant becomes stale again | On refresh error, include the server's `x-clerk-auth-*` headers in the error; doctor surfaces the refresh-fresh status as a distinct line so drift is visible within 60 seconds |
| Session cookies expire or rotate during a long run | Proactive refresh on every request plus the collapse window already exists; unit 2 strengthens the re-seed path when Chrome rotated the cookie |
| Happenstance rate-limits the people-search when walked for 2nd/3rd-degree | The existing `--rate-limit` persistent flag applies; unit 3 respects it, and dynamo polling uses a bounded retry |
| Upstream `linkedin-scraper-mcp` changes args again | Unit 6 adds a contract test that fails loudly on schema drift, rather than dropping results silently |
| Matt already has a separate upstream OSC PR in flight | Unit 6 stays narrow (contact-goat side only) so it lands regardless of the upstream PR's cadence |

## Documentation and Operational Notes

- Update `library/sales-and-crm/contact-goat/SKILL.md` and README with the new `hp people` command.
- Update `plugin/skills/pp-contact-goat/SKILL.md` via `go run ./tools/generate-skills/main.go` and bump `plugin/.claude-plugin/plugin.json` per repo AGENTS.md.
- Add a one-paragraph note to the README's "Known limitations" section that Happenstance is a best-effort wrapper of unofficial endpoints and that schema drift is possible.
- No production rollout; this is a local CLI. Verification is live dogfood on Matt's machine.

## Sources and References

- Origin session: 2026-04-19 debugging session (this conversation) documenting the five failure modes in Problem Frame.
- Related plan: `~/.osc/plans/2026-04-19-032-feat-linkedin-mcp-search-people-connection-filters-plan.md` (upstream LinkedIn MCP filter PR; tracked separately).
- Related code:
  - `library/sales-and-crm/contact-goat/internal/client/cookie_auth.go`
  - `library/sales-and-crm/contact-goat/internal/cli/coverage.go`
  - `library/sales-and-crm/contact-goat/internal/cli/promoted_research.go`
  - `library/sales-and-crm/contact-goat/internal/cli/search.go`
  - `library/sales-and-crm/contact-goat/internal/cli/linkedin.go`
- Related issues: none filed yet. Unit 1's sniff artifact is the closest thing to a bug report; unit 2 may produce a follow-up issue against `clerk` if the API version constant truly is stale rather than a local parse bug.
- External references: Clerk session-tokens API docs at clerk.com (referenced only after unit 1 sniff shows which call signature matches).
