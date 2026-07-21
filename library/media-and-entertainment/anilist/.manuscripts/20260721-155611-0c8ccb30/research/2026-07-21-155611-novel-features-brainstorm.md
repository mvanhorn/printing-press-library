## Customer model

### The weeknight anime watcher

**Today (without this CLI):** They open AniList’s list page, several show pages, and a calendar/search tab to work out whether anything they follow is actually airing tonight. They cannot reliably distinguish followed shows from the full seasonal feed.

**Weekly ritual:** Several evenings each week, they decide what to watch and record progress after an episode ends.

**Frustration:** The schedule and personal list are separate views, so “what I follow airs tonight?” takes tab-hopping and is easy to get wrong.

### The intentional backlog curator

**Today (without this CLI):** They manually scroll the Planning list, open candidates one by one, and mentally compare episode count, priority, score, and status. Generic recommendation lists repeatedly suggest shows they have already seen, dropped, or cannot finish soon.

**Weekly ritual:** At the start of a free evening, they choose one approachable show from a large backlog.

**Frustration:** AniList stores the needed personal metadata, but does not turn it into a deterministic short-list for a constrained evening.

### The shared-fandom planner

**Today (without this CLI):** They search seasonal pages and send ad-hoc links or screenshots to a friend. They can inspect recommendations but cannot quickly separate an actionable shared plan from a broad catalog search.

**Weekly ritual:** They compare a few current or upcoming shows before planning a watch session.

**Frustration:** A broad catalog result is not a concise, personal, share-safe decision card.

### The media-data explorer

**Today (without this CLI):** They use raw GraphQL queries or the AniList UI for catalog, character, staff, studio, review, recommendation, and activity exploration.

**Weekly ritual:** They investigate media details and automate selections around a watchlist.

**Frustration:** This is already materially covered by generated endpoint commands; adding another raw wrapper would add no personal-use leverage.

## Candidates (pre-cut)

| Feature | Command | Description | Persona | Source | Long Description | Rubric verdict |
|---|---|---|---|---|---|---|
| Tonight schedule | `schedule tonight` | Join the viewer’s CURRENT anime list with tonight’s airing records and emit only followed episodes due in local time. | Weeknight watcher | (a), (b), (e) | Use this command for episodes from anime already on the authenticated viewer’s CURRENT list that air tonight. Do NOT use it for an unfiltered seasonal schedule; use `airing-schedule list` instead. | Keep; authenticated, direct GraphQL join, dogfoodable. |
| Safe episode check-in | `progress check-in <media>` | Resolve a title or ID, display existing progress and intended next progress, then mutate only with `--apply`. | Weeknight watcher | (a), (e) | Use this command to advance one existing personal anime-list entry safely. Do NOT use it to create or broadly edit a list entry; use `save media-list-entry` instead. | Keep; authenticated mutation is explicitly gated by `--apply`. |
| Short backlog pick | `backlog pick` | Rank eligible PLANNING anime by personal priority/score and short-runtime constraints. | Backlog curator | (a), (b), (c), (e) | Use this command for a deterministic short anime choice from the viewer’s PLANNING list. Do NOT use it to browse the whole catalog; use `media search` instead. | Keep; mechanical ranking, no LLM or external service. |
| Catch-up queue | `progress catch-up` | Find CURRENT anime where latest aired episode exceeds stored progress and show the exact gap. | Weeknight watcher | (a), (b), (c) | Use this command to find followed shows with aired-but-unwatched episodes. Do NOT use it for only tonight’s releases; use `schedule tonight` instead. | Keep; cross-source join, bounded output. |
| Shared seasonal card | `season plan` | Produce a compact set of current-season titles, dates, and links suitable for sharing. | Shared-fandom planner | (a), (b) | none | Cut; mostly a presentation wrapper over generated media and airing commands. |
| Backlog health | `backlog health` | Count Planning entries by episode-length bands, age, and priority. | Backlog curator | (c) | none | Cut; useful but not a weekly decision command; `backlog pick` has direct leverage. |
| Next airing lookup | `airing next <media>` | Return one media item’s next airing time. | Weeknight watcher | (a) | none | Cut; thin renaming of the generated `airing-schedule list` endpoint. |
| Recommendation blend | `recommendations for <media>` | Fetch related recommendations for a chosen title. | Shared-fandom planner | (b) | none | Cut; already absorbed as generated recommendation commands. |
| Auto-progress watcher | `progress watch` | Poll AniList and automatically update watched episodes. | Weeknight watcher | (a), (e) | none | Cut; persistent background process and unsafe automatic account mutation exceed command scope. |
| Natural-language mood picker | `backlog mood "cozy"` | Pick planning media from free-form mood text. | Backlog curator | (a) | none | Cut; requires semantic classification/LLM behavior not present in the API. |
| Duplicate-service status | `streaming availability` | Find where a title can be watched. | Shared-fandom planner | (a) | none | Cut; requires an external availability service absent from the API. |
| Personal media dashboard | `dashboard` | Render a full-screen overview of lists, schedule, and stats. | All personas | (a), (c) | none | Cut; application/TUI scope rather than a focused command. |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona served | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Tonight schedule | `schedule tonight` | 10/10 | Weeknight anime watcher | hand-code | This uses authenticated `MediaListCollection` plus `AiringSchedule` GraphQL queries to compute followed CURRENT anime episodes airing in the local calendar day, with no external dependencies. | Phase 1 brief’s explicit “Tonight schedule” workflow and User Vision; official AniList GraphQL docs document both query families; incumbent assessment identifies no personal join workflow. | Use this command for episodes from anime already on the authenticated viewer’s CURRENT list that air tonight. Do NOT use it for an unfiltered seasonal schedule; use `airing-schedule list` instead. |
| 2 | Safe episode check-in | `progress check-in <media>` | 10/10 | Weeknight anime watcher | hand-code | This uses `Media` search/lookup, authenticated `MediaList` inspection, and `SaveMediaListEntry` to compute and optionally apply a verified next progress value, with no external dependencies. | Phase 1 brief’s explicit “Safe episode check-in” workflow and User Vision; official AniList mutation documentation; yuna0x0/anilist-mcp exposes raw mutation but lacks dry-run/apply safety. | Use this command to advance one existing personal anime-list entry safely. Do NOT use it to create or broadly edit a list entry; use `save media-list-entry` instead. |
| 3 | Short backlog pick | `backlog pick` | 10/10 | Intentional backlog curator | hand-code | This uses authenticated `MediaListCollection` PLANNING entries and their media episode/duration/status fields to deterministically rank eligible short candidates by priority and score, with no external dependencies. | Phase 1 brief’s explicit “Short backlog recommendation” workflow and User Vision; official AniList list/media schema; incumbent assessment identifies no personal short-backlog ranker. | Use this command for a deterministic short anime choice from the viewer’s PLANNING list. Do NOT use it to browse the whole catalog; use `media search` instead. |
| 4 | Catch-up queue | `progress catch-up` | 9/10 | Weeknight anime watcher | hand-code | This uses authenticated CURRENT `MediaListCollection` entries and `AiringSchedule` records to compare stored progress with the highest aired episode per media item, with no external dependencies. | Phase 1 weeknight-watcher research and “Maintain the personal library” workflow; official AniList schedule/list schema; automated-tracking demand noted in MALSync assessment. | Use this command to find followed shows with aired-but-unwatched episodes. Do NOT use it for only tonight’s releases; use `schedule tonight` instead. |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---|---|---|
| Shared seasonal card | Thin presentation wrapper without a cross-source personal decision rule. | `schedule tonight` |
| Backlog health | Likely monthly inspection rather than a weekly action; it does not choose what to watch. | `backlog pick` |
| Next airing lookup | Raw single-endpoint behavior is already covered by `airing-schedule list`. | `schedule tonight` |
| Recommendation blend | Existing generated recommendation operations cover it without a novel personal workflow. | `backlog pick` |
| Auto-progress watcher | Persistent polling plus automatic mutation is application scope and unsafe for autonomous use. | `progress check-in <media>` |
| Natural-language mood picker | Needs unsupported semantic/LLM classification. | `backlog pick` |
| Duplicate-service status | Depends on an external streaming-availability provider not in the API. | `backlog pick` |
| Personal media dashboard | TUI/application scope exceeds a focused, verifiable command. | `schedule tonight` |

## Reprint verdicts

Not applicable — first print; no prior `research.json` exists.
