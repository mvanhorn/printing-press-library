# seats-aero — novel-features brainstorm (subagent audit trail, 2026-09-05)

Full subagent output (Step 1.5c.5). Customer model and killed candidates are kept here for retro/dogfood debugging; only `### Survivors` and `## Reprint verdicts` flow into the absorb manifest.

## Customer model

**Persona A — "The Redemption Hunter."** Booking a specific trip (say, a business-class seat to Tokyo for a trip 3-4 months out) and chasing one origin-destination-cabin combination across every program that might have it.

*Today (without this CLI):* Keeps several browser tabs open on seats.aero's own web app, one per mileage program, re-running the same filtered search every day or two. Re-types the same origin/destination/cabin/date-window each visit because there's no saved state. Cannot answer "did anything change since yesterday across all the programs I care about" — has to hold the prior result set in memory or a scratch note and eyeball-diff it against today's page.

*Weekly ritual:* A near-daily (at minimum weekly) re-check of the same route across the ~5-6 programs she has miles in, watching for a seat to appear in the 11-month-out booking window.

*Frustration:* No persistent record of "what I already saw." Every check starts from zero, so she can't tell a genuinely new opportunity from one she already dismissed three days ago.

**Persona B — "The Region Scanner."** Flexible on destination within one region (e.g., "any Delta SkyMiles J/F seat North America→Europe this summer") because that's where her stash of miles sits.

*Today (without this CLI):* Pulls the bulk Availability calendar (or seats.aero's "Explore" feature) for one program and manually scrolls dozens-to-hundreds of date rows looking for direct flights at a reasonable mileage cost. No way to see a whole route's cabin coverage across a date range at a glance — just a flat list.

*Weekly ritual:* A weekly calendar sweep to rebuild her shortlist of viable dates before she narrows down to a specific booking.

*Frustration:* The raw bulk feed is a firehose (up to 1,000 rows) with no shape to it — she has to manually reconstruct "which dates have J availability" in her head or a spreadsheet.

**Persona C — "The Pre-Booking Verifier."** Has already found a promising Availability row from an earlier search and needs to confirm it's still real, right before she books through the airline directly.

*Today (without this CLI):* Fetches trip detail for the ID, then separately re-opens the seats.aero web page or asks in a Discord community whether the cached data might be stale, because she has no visibility into how long ago that row was cached.

*Weekly ritual:* Every time she's within a day or two of pulling the trigger on a redemption (which, across the several trips she's chasing at once, happens roughly weekly), she wants a "is this still good" gut check before spending a scarce daily API credit re-verifying it.

*Frustration:* No local sense of data freshness, and no guard against blowing through the 1,000-call daily quota mid-verification.

**Persona D — "The Miles Opportunist."** No fixed destination — has a pile of miles in one or two programs and periodically asks "where can these actually take me right now, nonstop, in business class, for the fewest miles?"

*Today (without this CLI):* Manually works through seats.aero's per-program exploration view, one program at a time, because there's no single fan-out that also confirms real dated seats exist (the destinations endpoint only tells her the theoretical cheapest fare, not whether a bookable date is nearby).

*Weekly ritual:* A weekly "what's newly reachable" check whenever she has a free weekend to consider using miles opportunistically.

*Frustration:* Cross-referencing "theoretically cheapest" against "actually bookable soon" requires two manual passes and mental cross-checking across sources.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Long Description | Inline verdict |
|---|------|---------|-------------|---------|--------|-------------------|-----------------|
| R1 | Award deal finder *(prior)* | `seats-aero-partner-search` | Prior CLI's cached-search mirror. | A | (d) reprint | none | Fully superseded by absorb row 1 (`search` endpoint + `--sources`/`--json`/`--select`). As a bare mirror it earns no transcendence row on its own — reconciled below as **prior-reframe**. |
| R2 | Program route atlas *(prior)* | `routes` | Prior CLI's route-listing mirror. | B/D | (d) reprint | none | Fully superseded by absorb row 4 (`routes` endpoint + FTS5 over 87k rows). Reconciled below as **prior-reframe**. |
| R3 | Bulk award calendar *(prior)* | `availability` | Prior CLI's bulk-availability mirror. | B | (d) reprint | none | Fully superseded by absorb row 2 (`availability` endpoint, cursor-paginated). Reconciled below as **prior-reframe**. |
| 1 | New-since award watch | `new-since` | Diff the locally synced `availability` table to show rows that became newly visible since a cutoff, for a saved route/cabin. | A | (a) persona-driven / (b) content pattern | see Survivors | Keep. Not LLM/external-service/auth-gapped. Local diff, not a live-call mirror. |
| 2 | Cabin/date calendar matrix | `calendar` | Pivot one route's synced availability into a date × Y/W/J/F matrix. | B | (a) persona-driven / (b) content pattern | see Survivors | Keep. Local reshape of already-synced data, not a live mirror. |
| 3 | Route/availability coverage audit | `coverage` | Join `routes` × `availability` to flag tracked routes with no synced or stale availability. | B/D | (c) cross-entity | none | Reframe target for R2; passes kill/keep checks (local join, no external service) but risks a weekly-use soft-kill — deferred to Pass 3. |
| 4 | Nonstop reach finder | `reach` | Fan out from one origin via `/destinations`, then cross-check top candidates against real dated seats. | D | (a) persona-driven / (c) cross-entity | see Survivors | Keep. Chains two calls with real synthesis, not a rename. |
| 5 | Credit-aware recheck | `recheck` | Pull a shortlist of matching Availability IDs from the local store and call `/refresh` on them with a quota guard. | C | (a) persona-driven / (b) content pattern | see Survivors | Keep. Local read + live call, addresses explicit tier-gating/quota risk from the brief. |
| 6 | Best-award finder (compound) | `deals` | Rank `/search` by lowest mileage, auto-fetch `/trips` for the top N. | A | (a) persona-driven | none | Flag: overlaps the brief's own planned MCP intent `find_best_award` (Build Priority 4) and the two already-absorbed commands run in sequence — weak transcendence, deferred to Pass 3. |
| 7 | Credit spend ledger | `spend` | Report trips/refresh credit usage over a period from local cache metadata. | A/C | (b) content pattern / (c) cross-entity | none | Passes kill/keep checks (local, computed) but weekly-use is doubtful — deferred to Pass 3. |
| 8 | Availability staleness report | `staleness` | Report `synced_at` age per Availability ID against a TTL, read-only. | C | (b) content pattern | none | Passes kill/keep checks but likely duplicates a computation `recheck` needs internally — deferred to Pass 3. |
| 9 | Cross-program direct-only scan | `direct-scan` | Filter already-synced availability across ALL synced programs/routes for direct-only seats under a mileage ceiling. | B/D | (b) content pattern / (c) cross-entity | see Survivors | Keep. `/availability` is single-program-per-call (Data Layer); this is a join no single live call produces. |
| 10 | Daily-quota pre-flight | `quota-budget` | Before a planned batch of refresh calls, show remaining daily quota and warn. | C | (b) content pattern | none | **Cut now.** Reimplementation/duplication check: the quota number is already surfaced by `doctor` (absorb row 8) and by `/refresh`'s own `quota{}` object (absorb row 6) — no new leverage over calling either. |

No `## User Vision` gap remains unaddressed by (a)-(d): output-bounding, MCP intents, and the local-store redesign are all Build Priorities already, not novel-feature candidates. No `## Codebase Intelligence` section is present in the brief, so source (f) does not apply.

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Data-source strategy | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|----------------------|-------------------|
| 1 | New-since award watch | `new-since --origin JFK --destination NRT --cabin business --since 24h` | 10/10 | hand-code | This uses the locally synced `availability` table's per-row `first_seen_at` timestamp (set on initial upsert during `sync`) to compute which rows became newly visible after a given cutoff, with no external dependencies. | Table Stakes: "Alerts... web-app-only feature, not exposed via Partner API... closest local approximation is a poll-and-diff pattern via sync + search/analytics." Top Workflow #5's re-check ritual. | local | Use this command to see which cached availability rows are newly visible since a past point in time. Do NOT use this to re-verify a specific already-known Availability ID is still bookable before booking; use `recheck` instead. |
| 2 | Cabin/date calendar matrix | `calendar --origin JFK --destination NRT --source united --start 2026-10-01 --end 2026-12-31` | 8/10 | hand-code | This uses the locally synced `availability` table for one route to compute a date × cabin (Y/W/J/F) pivot matrix, with no external dependencies. | Top Workflow #2: "Scan a whole region's calendar for one program... used to build shortlists." Data Layer: `availability` is the sync target dated by `Date`. | local | Use this command to view one route's full cabin-by-date matrix from already-synced availability. Do NOT use this to filter across multiple routes/programs for direct-only options under a mileage ceiling; use `direct-scan` instead. |
| 3 | Nonstop reach finder | `reach --origin JFK --cabin business --max-mileage 90000 --top 10` | 8/10 | hand-code | This chains a live `/destinations` call (cheap fan-out) with the locally synced `availability` table (falling back to a bounded live `/search` per top candidate) to compute which fan-out destinations have real dated seats, with no external dependencies beyond the seats.aero Partner API. | Table Stakes: "Nonstop where can I go fan-out — Missing — high-value novel-feature candidate." Top Workflow #4; MCP surface notes intent `explore_from_airport`. | auto | Use this command to discover which destinations are reachable nonstop from one origin airport, ranked by mileage cost. Do NOT use this to filter already-synced availability for a route you already know; use `direct-scan` or `calendar` instead. |
| 4 | Credit-aware recheck | `recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 [--dry-run]` | 10/10 | hand-code | This drains a read-only query over the locally synced `availability` table (closed before any write) to build a shortlist of matching, aging Availability IDs — reporting each row's `synced_at` age even in `--dry-run` — then calls the live `POST /refresh` endpoint on that shortlist while checking the response's `quota.remaining` field, with no external dependencies beyond the seats.aero Partner API. | Top Workflow #5: "power users hit the credit-metered /refresh on a shortlist of Availability IDs right before booking." MCP surface notes intent `refresh_before_booking` ("surface quota.remaining so the agent doesn't blow the daily credit budget"). | auto | Use this command to re-verify specific already-known Availability rows are still live before booking, with a quota guard. Do NOT use this to discover newly appeared availability across a route; use `new-since` instead. |
| 5 | Cross-program direct-only scan | `direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,ana,virgin-atlantic` | 9/10 | hand-code | This uses the locally synced `availability` table across multiple already-synced `Source` programs to compute a cross-program, direct-only shortlist under a mileage ceiling — a join no single live API call can produce, since `/availability` is scoped to one program per call — with no external dependencies. | Data Layer: "`/availability`... is scoped to one program at a time." Absorb catalog row 9 contrast (AwardTravelFinder `search_all_airlines`) — the differentiator exists for live `/search --sources` but not for the bulk-synced local table. | local | Use this command to filter already-synced availability across ALL routes and programs for direct-only flights under a mileage ceiling. Do NOT use this to view a single route's full date-by-cabin matrix (use `calendar`) or to discover new destinations from an origin with no fixed route in mind (use `reach`). |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Daily-quota pre-flight (`quota-budget`) | Duplicates quota surfacing already planned in `doctor` (absorb row 8) and already returned by the generated `refresh` endpoint's own `quota{}` object (absorb row 6) — no new leverage. | `recheck` (carries its own quota guard) |
| Route/availability coverage audit (`coverage`) | Soft-killed by the weekly-use test: auditing which routes lack synced/fresh data is occasional/before-a-plan work, not a repeated weekly ritual for any named persona. | `direct-scan` |
| Best-award finder / compound (`deals`) | Duplicates the brief's own planned MCP intent `find_best_award` (Build Priority 4) and the two already-absorbed commands (`search --order-by lowest_mileage`, `trips <id>`) run in sequence — no distinct local-store or cross-source transcendence beyond that chain. | `recheck` |
| Credit spend ledger (`spend`) | Soft-killed by the weekly-use test: a credit-spend history report is naturally a monthly/occasional glance; the load-bearing, in-the-moment version of "don't overspend" is already covered by `recheck`'s quota guard. | `recheck` |
| Availability staleness report (`staleness`) | The read-only freshness computation it would expose duplicates what `recheck` must already compute internally to decide whether to spend a refresh credit — no distinct weekly ritual of its own. | `recheck` |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---|---|---|
| Award deal finder (`seats-aero-partner-search`) | **Reframe** → `new-since` | The raw `/search` mirror is fully absorbed (absorb row 1, `--sources` multi-program etc.); the durable version of "find me a deal" that Persona A actually repeats is "tell me what's new since I last looked," which needs a local diff the mirrored endpoint alone can't produce. |
| Program route atlas (`routes`) | **Reframe** → `direct-scan` | The raw `/routes` mirror is fully absorbed (absorb row 4, FTS5 over 87k rows); its transcendent evolution is `direct-scan`'s cross-program join over synced availability, since a plain route atlas lists coverage but never live cabin/mileage state. (Note: a standalone `coverage`-gap-audit reframe was tried first but killed in Pass 3 on the weekly-use test — see Killed candidates.) |
| Bulk award calendar (`availability`) | **Reframe** → `calendar` | The raw `/availability` mirror is fully absorbed (absorb row 2, cursor-paginated); `calendar` keeps the same underlying synced data but reshapes it into the date × cabin matrix Persona B actually scans, which the raw bulk feed doesn't provide on its own. |

---

## Orchestrator corrections applied when building the manifest

- `direct-scan` example `--sources united,ana,virgin-atlantic`: `ana` is not a Partner API source and the Virgin Atlantic id is `virginatlantic`; manifest example corrected to `--sources united,virginatlantic,aeroplan`.
- `new-since` depends on a `first_seen_at` column on the typed `availability` table, set on first upsert. This is a Priority 0 store requirement, recorded in the manifest's data-layer note.
- `recheck` spends refresh credits: its live-dogfood happy path MUST be the `--dry-run` form (`pp:happy-args` includes `--dry-run`), never a real `/refresh` call.
