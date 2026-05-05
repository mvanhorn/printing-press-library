## Customer model

**Persona A — Maya, the comparison-shopping homebuyer.**
- *Today:* 8 Redfin tabs across three Austin neighborhoods, refresh nightly, screenshots into a Google Doc, manual $/sqft on a calculator. Cannot answer "which of my 12 favorites had a price drop this week?" without clicking each one.
- *Weekly ritual:* Sunday evening — scan three saved searches, eyeball new listings, copy details into a tracker.
- *Frustration:* No way to see "what changed since last Sunday" — Redfin shows current state, never deltas.

**Persona B — Raj, the small-portfolio SFR investor.**
- *Today:* Pulls Redfin CSVs (350-row cap) for five zips, dumps into Excel, computes $/sqft net of HOA, sorts by DOM to find lowball candidates. Pays $49/month for Apify Redfin actor that bot-blocks weekly.
- *Weekly ritual:* Monday — cheapest $/sqft 3+BR under $400k across 5 zips. Wednesday — sold-comp pull for any subject property.
- *Frustration:* 350-cap → run shifted price bands four times and dedupe by hand. No cross-region union ranking.

**Persona C — Priya, the cross-city relocator.**
- *Today:* Comparing Raleigh / Charlotte / Durham. Has trends pages open per city, copies median sale and DOM by hand. Doesn't know which neighborhood within a city is appreciating fastest.
- *Weekly ritual:* Saturday research — pull market trends for 3 candidate cities, decide whether the move math still works.
- *Frustration:* Redfin's UI shows one region at a time; no overlay, no appreciation ranker.

**Persona D — Tom, the buyer's agent doing comp pulls.**
- *Today:* 20 minutes per comp pull on the website; Redfin's polygon tool is clumsy and resets between sessions.
- *Weekly ritual:* 3–5 comp pulls per week.
- *Frustration:* Cannot save a "comp recipe" (radius / sqft tolerance / recency) and re-run it.

## Candidates (pre-cut)

| # | Name | Command | Persona | Source | Verdict |
|---|------|---------|---------|--------|---------|
| 1 | Saved-search watch | `watch <slug>` | Maya, Raj | (a)(e) | Keep |
| 2 | $/sqft net-HOA ranking | `rank --by price-per-sqft --net-hoa` | Raj | (c) | Keep |
| 3 | Side-by-side compare | `compare <url> <url>...` | Maya, Tom | (a)(c) | Keep |
| 4 | Listing history standalone | `history <url>` | Maya, Tom | (b)(c) | Cut — duplicates `listing get` |
| 5 | Stale + drop scan | `drops --region <slug>` | Raj, Maya | (a)(c) | Keep |
| 6 | Multi-region union | `rank --regions a,b,c` | Raj, Priya | (c) | Keep |
| 7 | Trends overlay | `trends --regions a,b,c` | Priya | (a)(c) | Keep |
| 8 | Sold-comp recipe | `comps <subject>` | Tom, Raj | (a)(b)(c) | Keep |
| 9 | Newest-listings feed | `feed new` | Maya, Raj | (b)(c) | Cut — covered in absorb |
| 10 | Bulk export past 350 cap | `export --year 2024` | Raj | (a)(b) | Keep |
| 11 | Neighborhood summary | `summary --region <slug>` | Priya, Raj | (c) | Keep |
| 12 | Weekly digest | `digest --since 7d` | Maya | (a)(c) | Cut — wrapper over `watch` |
| 13 | Region appreciation ranker | `appreciation --parent <metro>` | Priya | (c) | Keep |
| 14 | Address resolver | `resolve "<freeform>"` | Tom | (a) | Cut — wrapper |
| 15 | Photo bulk download | `photos <url> --out <dir>` | Maya | (b) | Cut — side-effect; thin |
| 16 | Open-house finder | `open-houses --weekend` | Maya | (a)(b) | Cut for v1 — N+1 fanout |
| 17 | Price-band histogram | `histogram --region` | Raj, Priya | (c) | Cut — `sql` covers it |
| 18 | Watch-with-notify | `watch --notify slack:...` | Maya | (a) | Cut — external dep, scope creep |
| 19 | "Best for me" recommender | `match --schools-min 8 --commute-to` | Maya | (a) | Cut — LLM-dependent + commute |

## Survivors and kills

### Survivors (10 features, all >= 6/10)

See absorb manifest's transcendence table for the full row format with scores and buildability proofs.

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Listing history (`history <url>`) | Already emitted by `listing get` (absorbed); standalone duplicates without leverage | `compare` |
| Newest-listings feed (`feed new`) | Absorbed at items #23/#24 | `watch` |
| Weekly digest (`digest`) | Trivial loop over `watch` across saved searches | `watch` |
| Address resolver (`resolve`) | Thin rename of autocomplete (absorbed) | `compare` (which resolves URLs internally) |
| Photo bulk download (`photos`) | Side-effect-heavy file writes, low transcendence | `compare` |
| Open-house finder | Open-house metadata is per-listing; region search needs N+1 fanout; descope to `--has-open-house` flag on `homes` | `homes` (absorbed) |
| Price-band histogram | User can write `sql` (absorbed item #27) themselves | `rank` |
| Watch-with-notify | External service (Slack/email) not in spec, daemon scope creep | `watch` (pipes cleanly) |
| "Best for me" recommender | LLM-dependent scoring + commute requires routing API not in spec | `rank`, `compare` |
