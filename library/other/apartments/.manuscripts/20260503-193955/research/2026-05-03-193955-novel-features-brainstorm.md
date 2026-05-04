## Customer model

**Persona 1 — Maya, the relocator (cross-city move in 6 weeks).**

*Today (without this CLI):* Maya is moving from Chicago to Austin in six weeks. Every evening she has 7 apartments.com tabs open across three Austin neighborhoods (Mueller, East Riverside, North Loop), pastes price/beds/sqft into a Google Sheet by hand, and re-runs the same three searches in her browser the next morning to spot new listings. She cannot tell which units appeared today vs. yesterday, cannot rank by $/sqft across the three neighborhoods at once, and cannot get a single ranked feed.

*Weekly ritual:* Three saved searches (one per target neighborhood), re-checked nightly, with promising units copy-pasted into a shortlist sheet on Sundays for partner review.

*Frustration:* The site has no concept of "what's new since yesterday" and no multi-neighborhood union. Every refresh re-shows yesterday's units and forces her to remember which placards she already considered.

**Persona 2 — Jordan, the data-driven hunter (value-per-dollar optimizer).**

*Today (without this CLI):* Jordan has a hard $2,800 budget for a 2BR in Denver and treats apartment hunting like a screen. They open ~30 listings a week, manually compute $/sqft and total-cost-with-pet-fees in their head, and abandon listings whose pet rent + deposit would push them over budget. They have no way to sort 60 candidates by $/sqft inside apartments.com — the site sorts by "best match" and price, not by ratio metrics, and never folds in pet fees.

*Weekly ritual:* Sunday morning sweep across two zip codes, narrow to ~10 listings ranked by $/sqft net of pet fees, send the top 5 to their partner, schedule tours for the top 2 on weekday evenings.

*Frustration:* Computing $/sqft and pet-fee-adjusted total cost across 60 listings by hand. The site's amenity filters cannot express "rank by $/sqft, but only among pet-friendly under $2,800 with in-unit washer."

**Persona 3 — Priya, the leasing agent (manages 6 client searches).**

*Today (without this CLI):* Priya is a relocation agent juggling six client searches simultaneously. Each Monday she manually re-runs each client's saved search in apartments.com, screenshots placards into a client-specific folder, and emails a "what changed this week" digest. She cannot diff a search against last Monday, cannot detect price drops, and cannot flag stale listings (units that haven't changed in 30+ days are often phantom or already leased).

*Weekly ritual:* Monday morning batch: re-run six saved searches, identify new/removed/price-changed units per client, draft six personalized digest emails by 10am.

*Frustration:* Two hours of mechanical diffing every Monday. apartments.com offers email alerts for new listings but no removal alerts, no price-drop alerts, and no per-listing change history.

**Persona 4 — Sam, the agent-host user (Claude Desktop / Cursor with MCP).**

*Today (without this CLI):* Sam wants to ask their agent "find me 2BR pet-friendly units under $2,500 in three Austin neighborhoods, rank by $/sqft, and watch for price drops." Today the agent fails because every existing apartments.com scraper is bot-blocked and there is no MCP server. They fall back to manually pasting URLs into the chat and asking the model to read them, which burns context and breaks on Akamai 403s.

*Weekly ritual:* Open Claude Desktop, ask a natural-language question that requires multi-listing reasoning, expect structured JSON output the model can join with their notes / commute-time research.

*Frustration:* No working agent-native data source for US apartment rentals. Every alternative is a 2018-era scraper that returns 403.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| C1 | Watch saved searches with diff | `watch <saved-search>` | Re-runs a stored search, diffs against last sync, surfaces new / removed / price-changed listings as a structured digest | Priya, Maya | (a) frustration; (b) saved-search content pattern | KEEP |
| C2 | Multi-slug union ranking | `nearby <slug1> <slug2> ... --rank price-per-sqft` | Fans out across multiple city/zip/neighborhood slugs, dedupes by listing URL, returns a single ranked list | Maya, Jordan | (a); (b) | KEEP |
| C3 | Total-cost-of-occupancy ranking | `value --budget 2800 --pet dog --months 12` | 12-month total = base + pet fees, ranks under budget | Jordan | (a); (c) | KEEP |
| C4 | $/sqft and $/bed ranking | `rank --by sqft\|bed` | Ratio metrics not exposed by site sort | Jordan | (a); (c) | KEEP |
| C5 | Side-by-side compare | `compare <url1> <url2> ...` | 2–8 listings, column per listing | Maya, Jordan | (a); (c) | KEEP |
| C6 | Stale-listing flag | `stale [--days 30]` | Listings unchanged for N days | Priya, Jordan | (a); (c) | KEEP |
| C7 | Price-drop alerts | `drops [--since 7d] [--min-pct 5]` | Time-window aggregation | Priya, Jordan, Maya | (a); (c) | KEEP |
| C8 | New-since-date feed | `new-since <date>` | — | Maya | (c) | MERGE INTO C1 |
| C9 | Saved-search login | `login` | — | Priya | — | KILL — auth out of scope |
| C10 | Application submission | `apply <listing>` | — | — | speculative | KILL — write auth |
| C11 | Commute-aware filter | `commute-rank --to <addr>` | — | Maya, Jordan | (a) | KILL — external service |
| C12 | Floor-plan-level value report | `floorplans --rank price-per-sqft` | Per-plan ratio | Jordan, Maya | (b); (c) | KEEP |
| C13 | Amenity must-have intersect | `must-have "term" ...` | FTS5 join | Jordan | (a); (c) | KEEP |
| C14 | Phantom-listing detector | `phantoms` | Three-signal join | Priya, Maya | (a); (c) | KEEP |
| C15 | Neighborhood market summary | `market <city-state>` | Aggregation | Maya, Sam | (b); (c) | KEEP |
| C16 | Tour-schedule export | `tours export --top 5 --format ics` | — | Jordan | (a) | KILL — scope creep |
| C17 | Saved shortlist | `shortlist add\|show` | Tag-based local shortlist | Maya, Jordan | (a); (c) | KEEP |
| C18 | Weekly digest | `digest --saved-search ... --since 7d` | Composes C1+C4+C6+C7 | Priya, Maya | (a); (c) | KEEP |
| C19 | Listing history | `history <url>` | Time-series for one listing | Priya, Jordan | (c) | KEEP |
| C20 | "Best match" semantic ranking | `recommend "..."` | — | Sam | speculative | KILL — LLM dep |

## Survivors and kills

### Survivors

13 features, all >= 5/10. See absorb manifest for the transcendence table.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| C8: New-since-date feed | Pure subset of `watch` — `watch --since <date>` covers it; standalone form is a wrapper around one query | C1 `watch` |
| C9: Saved-search login & persist | Requires apartments.com cookie session auth, which the brief explicitly puts out of scope for v1 | C1 `watch` (uses local saved-search slugs, not server-side accounts) |
| C10: Application submission | Requires write auth and a flow apartments.com gates behind logged-in users; outside our auth scope | none — different domain |
| C11: Commute-aware filter | Depends on Google Maps, an external service not in the API spec; brief marks `--commute-to` as out-of-scope-v1 | C4 `rank` |
| C16: Tour-schedule export | Scope creep (calendar generator), and apartments.com exposes no tour-slot availability to anchor on | C17 `shortlist` |
| C20: "Best match" semantic ranking | LLM-dependent per rubric kill check; users can pipe `--json` to their own model | C13 `must-have` |
