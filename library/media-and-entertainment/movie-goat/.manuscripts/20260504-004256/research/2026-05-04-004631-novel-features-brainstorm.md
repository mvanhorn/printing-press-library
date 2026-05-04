## Customer model

**Persona 1 — Maya, the Tonight Decider**

*Today (without this CLI):* Maya opens TMDb, Letterboxd, Rotten Tomatoes, and JustWatch in four tabs every Friday after dinner. She wants something well-rated that's actually on the streamers she pays for (Netflix + Max + Prime). She bounces between tabs because TMDb tells her ratings but not RT, JustWatch tells her where to watch but not whether it's any good, and Letterboxd tells her vibe but not availability. She often ends up on a 20-minute scroll that ends with a rerun.

*Weekly ritual:* Two to four times a week, pick a movie or one TV episode for the night within 5-10 minutes. Constraints: must be on her services, must be in a mood (thriller / comedy / "something good I haven't seen"), must be reasonably runtime-bounded.

*Frustration:* The cross-reference. She knows what's well-reviewed; she knows what's streaming; she can't see both filters at once.

**Persona 2 — Devon, the Franchise Marathoner**

*Today (without this CLI):* Devon hosts Saturday movie nights for a friend group of six. They're working through Marvel chronologically; next month it's Mission Impossible, then Alien. He builds Google Docs with watch order, runtimes, and "skip-this-one" notes pulled from Reddit threads. Recurring problem: figuring out the canonical watch order for sprawling franchises (release order vs. in-universe vs. director-recommended) and totalling runtime so the group knows what they're committing to.

*Weekly ritual:* Plan one franchise night per week. Mid-week he stitches together a sequence with runtimes and break points (food, intermission, close-out).

*Frustration:* Manually summing runtimes across 10-25 entries pulled from a TMDb collection page, and reconciling collection metadata (which is messy — spinoffs, prequels, anthology entries) into an actual watch list.

**Persona 3 — Priya, the Cinephile Career-Tracker**

*Today (without this CLI):* Priya follows directors and DPs the way other people follow sports teams. When a new Lynne Ramsay or Hong Sang-soo film hits, she wants the full career arc in front of her: every feature, ratings trajectory, recurring collaborators, gaps between films. She uses Letterboxd lists, IMDb filmographies, and a personal Notion doc. She frequently asks: "what was the last thing this DP shot?", "who else has this director worked with three or more times?", "which of these 30 films is consensus-best vs. niche favorite?"

*Weekly ritual:* When a new release drops or a podcast namedrops a filmmaker, run a deep dive on that person's filmography in 5 minutes — chronology, ratings, collaborators.

*Frustration:* Filmographies on TMDb and IMDb are flat lists; ratings live elsewhere; recurring collaborators require manual cross-referencing across multiple title detail pages.

**Persona 4 — Sam, the Watchlist Curator**

*Today (without this CLI):* Sam keeps a watchlist of ~80 titles in Letterboxd. The pain point: knowing *when* something on the list lands on a service they have. They don't want to pay $4 to rent it; they'd rather wait. Today they spot-check JustWatch when they remember a specific title. Things expire from streamers without warning.

*Weekly ritual:* Weekend audit — what on my watchlist became streamable since last week? Anything about to leave?

*Frustration:* No tool tells them, for an arbitrary set of titles they care about, which are streamable on services they subscribe to right now.

## Candidates (pre-cut)

(see Survivors table below for retained candidates; killed inline: C6 Where-to-watch already covered, C11 LLM synopsis, C12 LLM sentiment, C13 box-office, C16 region diff. C15 absorbed into C3.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Tonight Picker | `movie tonight [--mood <genre>] [--max-runtime N] [--providers <csv>] [--region US]` | 9/10 | Maya | TMDb `/trending` + `/discover` + per-title `/watch/providers` filtered to user's `--providers` set; mechanical rank by composite rating. No external deps. |
| 2 | Marathon Planner | `movie marathon <title-or-collection-id> [--order release\|inuniverse] [--breaks-every N]` | 8/10 | Devon | Resolves title→`belongs_to_collection`→`/collection/{id}`, normalizes order, sums runtimes from per-title TMDb details, inserts breakpoints every N minutes. Pure TMDb. |
| 3 | Career Timeline | `movie career <person-id-or-name> [--since YEAR] [--role actor\|director\|dp]` | 8/10 | Priya | TMDb `/person/{id}/combined_credits` + per-title detail fan-out for ratings + OMDb (when key set) via IMDb ID; sorted chronologically. |
| 4 | Multi-Source Ratings Card | `movie ratings <id>` | 9/10 | Maya, Priya | TMDb `/movie/{id}` or `/tv/{id}` + OMDb `?i=<imdbid>`; rendered as a single card. Graceful degradation when OMDB_API_KEY unset. |
| 5 | Head-to-Head Versus | `movie versus <id-a> <id-b> [--region US]` | 7/10 | Maya | Composes Ratings Card across two titles + cast-name intersection from `/credits` + provider lists per region; aligned columnar output. |
| 6 | Watchlist | `movie watchlist add/list/remove [--available --providers <csv> --region US]` | 8/10 | Sam | SQLite table `watchlist(id, kind, title, added_at)`; `list --available` re-checks `/watch/providers` for each row, filters to `--providers`, emits flagged rows. |
| 7 | Recurring Collaborators | `movie collaborators <person> [--min-count 2] [--role actor\|crew]` | 6/10 | Priya | TMDb `/person/{id}/combined_credits` + per-title `/credits`, group-by name + count, filters by `--min-count`. |
| 8 | Recommendation Queue | `movie queue [--limit 20] [--providers <csv> --region US]` | 7/10 | Sam, Maya | For every row in local watchlist: union `/recommendations` + `/similar`, dedupe by id, rank by TMDb vote_average × log(vote_count); optional provider filter. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|------------|---------------------------|
| C6 Where to watch | Already in absorb manifest #14 (`movie watch <id>`) | Watchlist `--available` |
| C9 Franchise Discover | Thin lookup; absorbed into Marathon Planner | Marathon Planner |
| C11 Smart synopsis | LLM dependency; pipe `--json` to a chat model | Multi-Source Ratings Card |
| C12 Review sentiment | LLM dependency + low/biased TMDb review counts | Multi-Source Ratings Card |
| C13 Box-office tracker | TMDb lacks box-office; OMDb's BoxOffice text is stale | none |
| C14 Search history | Pure local-SQLite convenience with no API leverage | Watchlist |
| C15 Recent-credits-of-person | Subsumed by Career Timeline `--since` flag | Career Timeline |
| C16 Region availability diff | No persona owns it weekly | Watchlist (region-aware on-demand) |
| C17 Genre Pulse | Subsumed by Tonight Picker `--mood`/`--genre` flags | Tonight Picker |

## Reprint verdicts

| Prior feature | Prior command | Verdict | Justification |
|---------------|---------------|---------|---------------|
| Multi-Source Ratings Dashboard | `get` | Reframe (`get` → `ratings`) | Persona fit and 9/10 score intact; absorb manifest #2 already uses `movie get` for the basic title-details card, so the multi-source variant needs a distinct command. |
| Where to Watch | `watch` | Drop | Now table-stakes (absorb #14). Surviving generalization is Watchlist `--available` + `--providers`. |
| Career Timeline | `career` | Keep | Persona fit (Priya) confirmed; 8/10; same command. |
| Tonight Picker | `tonight` | Keep | Flagship; persona fit (Maya); 9/10; same command. |
| Head-to-Head Compare | `versus` | Keep | Persona fit (Maya, Devon); 7/10; same command. |
| Marathon Planner | `marathon` | Keep | Persona fit (Devon); 8/10; same command. New flags `--order`, `--breaks-every` are additive. |

Three new (non-prior) features: Watchlist, Recurring Collaborators, Recommendation Queue.
