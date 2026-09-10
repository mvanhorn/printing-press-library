# Novel-Features Brainstorm + Adversarial Cut — myanimelist-pp-cli

Run `20260910-215529-36659c2f`. Produced by the Phase 1.5c.5 brainstorm subagent (first print; no reprint reconciliation). Persisted verbatim as the audit trail for retro/dogfood debugging.

## Customer model

**Marisol — the seasonal chaser.** 27, hospital scheduler in Phoenix (UTC-7), watches 6–8 simulcasts a season, keeps a Notion "season grid," and always has Crunchyroll, HIDIVE, the MAL season chart, and a timezone converter open.
*Today:* each new season she opens MAL's seasonal chart, the currently-airing top list, and a JST→MST converter, retyping broadcast slots by hand because MAL publishes them only in Japan time. She has no view of *only the shows she is already tracking*, and no way to see that two of them land in the same hour. She cannot answer "did this show fall apart after episode 4" until she is four episodes into a dud.
*Weekly ritual:* Sunday night she rebuilds the week's viewing plan; Thursday/Friday she checks which new episodes landed; mid-week she decides whether to drop anything.
*Frustration:* translating a JST broadcast slot into her own clock, for just her shows, is manual and error-prone every single week.

**Dev — the backlog completionist.** 34, backend engineer in Berlin, 400+ plan-to-watch, tried trackma and abandoned it at the OAuth wall, still has a stale `myanimelist.xml` export.
*Today:* he opens MAL detail pages one at a time, reads the single average score, scrolls to `/stats`, and eyeballs a rendered bar chart to guess whether a title is worth 24 episodes. A spreadsheet of "maybe next" titles has replaced a real list manager. He cannot answer "is this 8.5 loved or just widely watched" or "will I actually finish this."
*Weekly ritual:* Friday backlog triage — pick one or two titles to start, log progress somewhere local, quietly drop what stalls.
*Frustration:* MAL collapses quality into one number and hides the distribution behind a chart; nothing tells him drop likelihood before he commits.

**Priya — the manga-first adaptation tracker.** 22, uni student in London, follows 9 ongoing manga and their adaptations, keeps a per-series "what to read next" note.
*Today:* she keeps the anime page and the manga page open side by side, counts episodes against chapters in her head, checks the manga's publication status, and hunts Reddit threads for "where does the anime leave off". She also cannot see which entries of a franchise she has skipped.
*Weekly ritual:* after each episode of an adaptation she decides whether to switch to the source and where; every few weeks she sweeps her started franchises for unread entries.
*Frustration:* MAL's anime and manga entries never state coverage, so the one question she asks constantly is answered by hand or by asking strangers.

**Kenji — the staff/VA completionist and score-watcher.** 31, motion designer in Toronto, follows two studios and five seiyuu, scores everything on a personal spreadsheet, and argues about score inflation online.
*Today:* he opens a person page, scans the animeography, and opens each title in a new tab for its score. When he wants to argue that a show's score is climbing, he screenshots `/stats` bars and compares against his memory — because MAL has no score history anywhere.
*Weekly ritual:* check what his favorite VAs and studios shipped, re-score what he finished, argue about whether a score is moving.
*Frustration:* "has this risen or fallen since I last looked" is structurally unanswerable on MAL, and filmography cross-reference is tab soup.

## Candidates (pre-cut)

| # | Feature / command | One-line description | Persona | Source | Long Description | Verdict |
|---|---|---|---|---|---|---|
| 1 | Divisiveness index — `anime divisive <id>` | Turns the `/stats` 1–10 bar chart into a polarization number: divisive vs universally loved. | Dev, Kenji | (b) service-specific content pattern | Use for score-distribution shape. Do NOT use for score movement; use 'drift'. Do NOT use for raw vote table; use 'anime stats'. | **Keep** — mechanical computation over an already-planned parser; no LLM, no external service, no auth. |
| 2 | Drop-risk — `anime drop-risk <id>` | Predicts abandonment from the status distribution, not the average score. | Dev | (b) | Use for abandonment likelihood. Do NOT use for divisiveness; use 'anime divisive'. Do NOT use for raw counts; use 'anime stats'. | **Keep** — different distribution, distinct decision (commit 24 episodes or not). |
| 3 | Episode reception curve — `anime consistency <id>` | Episode-by-episode poll averages and reply counts as a drop-off curve. | Marisol, Dev | (b)(a) | Use for episode-by-episode reception. Do NOT use for raw episode list; use 'anime episodes'. Do NOT use for overall distribution; use 'anime divisive'. | **Keep** — mechanical stddev/slope over episode rows. |
| 4 | Reception drift and movers — `drift [<id>] --since 30d` | Score/member/favorites/rank change over time from local snapshots; leaderboard mode when no id. | Kenji, Dev | (b)(c) | Use for change over time from local snapshots. Do NOT use for static distribution shape; use 'anime divisive'. | **Keep** — the brief names this the moat; purely local. |
| 5 | Weekly viewing grid — `week` | Timezone-correct 7-day grid of only the shows in the local library, with broadcast-slot collisions flagged. | Marisol | (a)(c) | Use for a library-scoped timezone-correct weekly grid incl. collisions. Do NOT use for a flat list of everything airing; use 'airing'. | **Keep** — library scoping + collision detection is the delta. |
| 6 | Source coverage — `adaptation <anime-id>` | Reports how much of a source manga an anime covered, and whether the source is still publishing. | Priya | (b)(c) | Use for anime-to-manga source coverage. Do NOT use for other franchise entries; use 'franchise gap'. | **Keep, reframed** — descoped from "read from chapter N" (fabricated precision) to an episode-vs-chapter coverage band. |
| 7 | Franchise gap — `franchise gap` | Lists entries missing from the franchises already present in the local library. | Priya | (c)(a) | Use for entries missing from started franchises. Do NOT use for ordering; use 'watch-order'. Do NOT use for source coverage; use 'adaptation'. | **Keep** — library-scoped relation filter. |
| 8 | Season verdict — `season verdict <year> <season>` | Over- and under-rated titles by score-rank vs popularity-rank delta. | Dev | (b) | none | **Kill (soft cadence)** — a few runs per season fails weekly use. |
| 9 | Season-over-season diff — `season diff <a> <b>` | Aggregate score/member/genre diff between seasons. | Kenji | (c) | none | **Kill** — seasonal cadence; needs months of snapshots. |
| 10 | Cast/staff overlap — `cast overlap <a> <b>` | Shared VAs, staff, and studios between two titles. | Kenji | (c) | none | **Kill** — trivia-grade, no weekly decision. |
| 11 | Studio quality bar — `studio stats <id>` | Studio filmography aggregate: average score, hit rate, output by year. | Kenji | (b)(c) | none | **Kill** — monthly browsing; `studio filmography` already lists the rows. |
| 12 | Library-scoped news — `news mine` | Cached news filtered to library titles. | Dev, Marisol | (c) | none | **Kill (verifiability)** — fuzzy title matching silently mis-scopes. |
| 13 | Episode discussion heat — `anime chatter <id>` | Reply-count curve as an engagement proxy. | Marisol | (b) | none | **Kill** — novelty metric with no decision; `anime episodes` returns reply counts. |
| 14 | Taste profile — `taste` | Genre/theme/studio/VA affinities and grading harshness from the library. | Dev | (c) | none | **Kill (soft cadence)** — moves too slowly; overlaps `suggest`. |
| 15 | Backlog triage — `backlog triage` | Ranks plan-to-watch by score, drop-risk, and taste fit. | Dev | (a) | none | **Kill** — `suggest` already scores local-library eligibility from public stats. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|---------|--------------|--------------|----------|------------------|
| 1 | Divisiveness index | `anime divisive <id>` | 10/10 | Dev, Kenji | hand-code | Parses the cached `/anime/{id}/_/stats` "Score Stats" 1–10 vote bars and computes polarization (9–10 share vs 1–2 share plus weighted spread) with no external dependencies; `auto` data-source with `hintIfUnsynced`/`hintIfStale`. | Brief User Pain #2; no MAL tool exposes score distributions. | Use this command for the shape of a title's 1–10 score distribution. Do NOT use it for score movement; use 'drift'. Do NOT use it for the raw vote table; use 'anime stats'. |
| 2 | Drop-risk | `anime drop-risk <id>` | 8/10 | Dev | hand-code | Reads the cached `/stats` status block (watching/completed/on-hold/dropped/plan-to-watch) plus episode count and emits a dropped-share and unfinished-risk band, drain-first over the cached record. | User Pain #5; Workflow 2. | Use for abandonment likelihood. Do NOT use for divisiveness; use 'anime divisive'. Do NOT use for raw counts; use 'anime stats'. |
| 3 | Episode reception curve | `anime consistency <id>` | 8/10 | Marisol, Dev | hand-code | Parses `.../episode` rows (number, poll average, reply count) and computes per-episode deltas, first-3-vs-last-3 slope, and the worst episode; `auto` + hints. | Workflow 2; no competitor exposes episode-level data. | Use for episode-by-episode reception. Do NOT use for the raw episode list; use 'anime episodes'. Do NOT use for the overall distribution; use 'anime divisive'. |
| 4 | Reception drift and movers | `drift [<id>] --since 30d` | 9/10 | Kenji, Dev | hand-code | Reads only the local `snapshot` table (drain-first scan into structs, `rows.Err()`, close, then id→title resolution) and diffs score/members/favorites/rank between bracketing snapshots; `// pp:data-source local`, rejects `--data-source live`. | Brief Data Layer: "History is the moat"; User Pain #7. | Use for change over time from local snapshots. Do NOT use for the static distribution shape; use 'anime divisive'. |
| 5 | Weekly viewing grid | `week` | 9/10 | Marisol | hand-code | Intersects local `library_entry` (watching) with cached broadcast slots and episode air dates, converts JST→local, flags same-hour collisions; `auto` + hints over library and schedule. | User Pain #3; Workflow 1. | Use for a timezone-correct weekly grid from your local library, including collisions. Do NOT use for a flat list of everything airing; use 'airing'. |
| 6 | Source coverage | `adaptation <anime-id>` | 7/10 | Priya | hand-code | Joins the cached anime entry (episode count, airing status) with its related manga entry (chapters, volumes, publication status) and reports a coverage band plus "source still publishing"; no chapter-precise claim. | Workflow 6; User Pain #6. | Use for whether an anime covered its source manga. Do NOT use for other franchise entries; use 'franchise gap'. |
| 7 | Franchise gap | `franchise gap` | 7/10 | Priya | hand-code | Traverses cached relation edges for every franchise touched by local `library_entry`, joins against library status, lists missing entries; `// pp:data-source local`, rejects `--data-source live`. | Product Thesis; User Pain #4; Workflow 3. | Use for entries missing from started franchises. Do NOT use for ordering; use 'watch-order'. Do NOT use for source coverage; use 'adaptation'. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Season verdict (`season verdict`) | Runs a few times per season, not weekly — fails the weekly-use test. | `anime divisive` |
| Season-over-season diff (`season diff`) | Seasonal cadence, and it needs months of accumulated snapshots before the first useful answer. | `drift` |
| Cast/staff overlap (`cast overlap`) | Trivia-grade and occasional; no weekly decision hangs on it. | `franchise gap` |
| Studio quality bar (`studio stats`) | Monthly browsing at best, and absorbed `studio filmography` already carries the rows. | `studio filmography` |
| Library-scoped news (`news mine`) | Requires fuzzy headline-to-title matching that silently mis-scopes results and is not dogfood-verifiable. | `news list` |
| Episode discussion heat (`anime chatter`) | Novelty metric with no decision attached; `anime episodes` already returns reply counts. | `anime episodes` |
| Taste profile (`taste`) | A taste profile moves too slowly to justify weekly runs, and it overlaps absorbed `suggest`. | `suggest` |
| Backlog triage (`backlog triage`) | Absorbed `suggest` already scores local-library eligibility from public stats. | `suggest` |
