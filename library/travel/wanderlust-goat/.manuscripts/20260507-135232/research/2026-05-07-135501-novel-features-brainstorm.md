# wanderlust-goat — Novel Features Brainstorm (audit trail)

> Output of the novel-features subagent (Phase 1.5c.5). Customer model and killed candidates are persisted here per skill contract; only the `## Survivors` table flows into the absorb manifest.

## Customer model

### Persona 1: Mira, the kissaten-hunter on a 10-day Tokyo trip

**Today (without this CLI):** Mira opens 9 tabs every morning from her hotel: Google Maps, Tabelog (translated by browser, painfully), the kissaten-Twitter list a friend sent her, a Reddit thread from r/Tokyo about "real 70s jazz cafes," a Note.com article she had Google Translate, jp.wikipedia for the history of Yurakucho, Time Out Tokyo's "best vintage cafes" list, an English-language blog from 2019, and Apple Maps to actually walk. She copies place names back and forth, romaji-then-kanji, loses three of them by lunch, and accepts whatever's closest because she's hungry. She cannot answer "of the things within 15 minutes of where I'm standing right now, which 3 actually match what I care about?"

**Weekly ritual:** Every morning of the trip she picks a neighborhood, spends 45 minutes assembling a shortlist of 5-8 places from those 9 tabs, then walks. On a normal week at home she does the same exercise twice — Saturday coffee crawl, one Sunday "weird neighborhood" walk.

**Frustration:** The 45 minutes of tab-juggling produces a list dominated by whatever ranks in English search. Tabelog's actual 3.6+ kissaten with the burnt-orange interiors and the silent owners are buried under English-friendly tourist cafes. She wants the local-language signal but can't read the local language fast.

### Persona 2: Felix, the photographer chasing blue hour

**Today (without this CLI):** Felix runs PhotoPills for sun/blue-hour math, scrolls Instagram geotags for an hour, cross-references with Atlas Obscura for "weird viewpoints," opens Google Earth to check if the angle is even possible, then walks 25 minutes to a viewpoint that turns out to be fenced off. He has no tool that takes "the next blue hour at this location" and crosses it with "viewpoints within walking distance that photographers actually shoot." He cannot answer "for the 18-minute window starting at 18:42 tonight, what's within walking distance and elevated and not a tourist trap?"

**Weekly ritual:** 2-3 evenings a week he picks a neighborhood, computes blue hour, picks a viewpoint, walks. Once or twice a month he does a multi-stop route-view: walk from A to B and shoot whatever's interesting along the way.

**Frustration:** The viewpoint shortlist requires manually fusing sun-position math, OSM `tourism=viewpoint` tags, Atlas Obscura "secret rooftop" entries, and Reddit threads where locals warn which spots are actually accessible. There is no single query.

### Persona 3: Anya, the agent-orchestrator running deep travel research

**Today (without this CLI):** Anya is building an agent that plans her partner's birthday weekend in Seoul. She has Claude with web search, but every "find me a great kalguksu place near Bukchon that locals actually go to" turns into 6 tool calls, half of which return English-listicle slop. She has no JSON-shaped travel-research API to call against — the closest things are paid (Google Places) or English-only (Foursquare-ish remnants). She cannot hand her agent a `research-plan` that says "fan out across MangoPlate, Naver Map, /r/seoul, ko.wikivoyage, then fuse on trust + walking time."

**Weekly ritual:** 1-2 times per week, Anya constructs a multi-step research workflow for a friend's trip, a date, or a writing assignment. She does the same fanout pattern by hand each time.

**Frustration:** Every agent run re-derives the same fanout plan. There is no reusable, shaped, language-aware travel-research primitive. The agent re-invents wheels and burns tokens on irrelevant English content.

### Persona 4: Priya, the pre-trip city-syncer

**Today (without this CLI):** Two weeks before a Tokyo trip, Priya bookmarks ~40 articles, exports a Google My Maps with 60 pins, downloads the Wikivoyage offline page, screenshots six Reddit threads, and arrives in Tokyo with a mess of tabs that don't talk to each other. Once she's there with hotel wifi, she manually re-queries each one. She cannot answer "show me everything I curated, ranked by what's near where I'm standing right now."

**Weekly ritual:** 3-4 times a year she does a multi-day pre-trip sync, then queries it daily during the trip. At home, she does a smaller version weekly when planning weekend day-trips within driving range.

**Frustration:** Pre-trip research goes stale and disconnected. Nothing fuses her saved sources into a queryable local store she can ask "what's within 15 walking minutes of HERE that matches THIS taste."

## Candidates (pre-cut)

(See subagent return for full table — kept inline above; 16 candidates, sources labeled (a)/(b)/(c)/(e)/(f), inline kill verdicts on each row.)

## Survivors and kills

11 survivors, all ≥6/10, all clear the rubric's kill checks (no LLM in runtime, no missing service, no auth gap, no scope creep, all verifiable on synced fixtures, all leverage the local store or fanout — none reimplement upstream APIs).

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C8 Local-language alias preservation | Belongs as auto-behavior + `--lang` flag on `near` and `lookup`, not a top-level command — wrapper of one Nominatim call | C1 `near` |
| C10 Persona-saved criteria | Thin local config; doesn't transcend (a `~/.config/wanderlust-goat/personas.yaml` the user edits beats a CLI for save/load) | C1 `near` |
| C11 Editorial freshness check | Doesn't earn its own command — folds into `sync-city`'s default behavior to re-sync sources older than threshold | C7 `sync-city` |
| C13 Walking-radius isochrone preview | Wrapper of an OSRM loop; provides no cross-source leverage, no local-store join, no service-specific pattern | C6 `route-view` |
| C16 Local-language fanout dispatcher | Strict subset of `research-plan`'s output; emitting a "country → source list" without parameters is the worst version of the plan | C3 `research-plan` |
