# coffee-goat CLI Brief (Session 2 — Extension)

> Extends the Session 1 brief at `research/prior-brief.md`. Session 1's full content (API identity, data layer, 24-roaster registry, Coffee Review/Sprudge editorial sources, reachability probes, constraints) remains in force. This file documents the four new sources the user added in Session 2 and the integration deltas.

## API Identity (extended)

The Session 1 thesis ("third-wave coffee terminal — 24 elite roasters + Coffee Review + personal brew log") stays intact. Session 2 broadens the editorial corpus from "Coffee Review + Sprudge" to also include **two coffee YouTubers** and **competition history**, plus a real **cafe finder** that Session 1 only stubbed.

## New Sources (Session 2 additions)

### 1. James Hoffmann (YouTube only)

- **YouTube channel:** `@jameshoffmann` / channel ID `UCMb0O2CdPBNi-QqPk5T3gsQ`, 2.5M+ subscribers.
- **Content shape:** bean reviews, brew gear, technique essays, mini-documentaries. Bag-review density: moderate (he reviews specific roasters' bags regularly).
- **Blog (`jameshoffmann.com`)**: unreachable during probe (connection refused). **Deferred** until reachability is reverified; YouTube alone covers the headline use case.
- **Integration:** call the already-published `youtube-pp-cli` via subprocess.
  - `youtube-pp-cli youtube channel-uploads UCMb0O2CdPBNi-QqPk5T3gsQ --json` → recent uploads
  - `youtube-pp-cli youtube videos-transcript <video-id> --json` → transcripts via the public timedtext endpoint (no auth)
- **Auth:** none. The transcript path uses YouTube's unauthenticated timedtext endpoint per youtube-pp-cli's design.

### 2. Lance Hedrick (YouTube only)

- **YouTube channel:** `@LanceHedrick`, 425k+ subscribers, ~15M total views.
- **Content shape:** ~60% gear reviews, ~30% espresso technique (shot-pulling, WDT, flow-rate optimization), ~10% bean reviews. Complementary to Hoffmann (Hoffmann is bag-and-origin; Hedrick is espresso-mechanics and gear).
- **Integration:** same path as Hoffmann via `youtube-pp-cli`. Channel lookup needs to resolve `@LanceHedrick` → channel ID; youtube-pp-cli's `channel-uploads` already accepts `@handle`.
- **Auth:** none.

### 3. World Barista Championship + sibling competitions

- **Canonical source:** `wcc.coffee` (World Coffee Championships; `worldbaristachampionship.org` redirects here). Sister competitions: World Brewers Cup, World AeroPress Championship, World Latte Art, WCIGS, World Coffee Roasting Championship.
- **Data scope at official site:** sparse. Past rankings in HTML tables (year × competition × winner × country). **No structured recipe data published officially** — grind/dose/yield/time/temp + bean origin/producer/roaster come from post-competition coverage on Sprudge and Daily Coffee News.
- **Reachability:** `wcc.coffee` returns 200 to plain HTTPS; no bot-blocking observed during probe. Wikipedia's `World_Barista_Championship` article provides the cleanest winners table (Wikidata-linked).
- **Expected completeness:** ~60-70% of recipe fields fillable, decaying for older years (rich for 2018+, sparse before).
- **Integration:** primary scrape from `wcc.coffee/wbc-past-rankings`; cross-reference Sprudge editorial coverage (already in Session 1 source list with Chrome-cookie fallback) for bean/recipe per finalist. Cache to `champion_recipes` table with documented partial-fill columns.

### 4. Cafe finder (no-auth, two-tier)

- **Tier 1 (breadth): OpenStreetMap Overpass API**
  - Endpoint `https://overpass-api.de/api/interpreter`. No auth, fair-use rate limits.
  - Query: `[bbox:lat-d,lng-d,lat+d,lng+d]; (node[amenity=cafe]; way[amenity=cafe];); out center;` plus secondary filters on `cuisine=coffee_shop`, `drink:coffee=yes`, `coffee:brand=*`.
  - Coverage breadth: high; specialty-roastery tagging consistency: variable.
- **Tier 2 (curation): Sprudge city guides**
  - Hub at `sprudge.com/guides` lists 22+ city guides ("The Sprudge Guide to Coffee in <City>").
  - URL pattern `sprudge.com/the-sprudge-guide-to-coffee-in-<city>-<id>.html`.
  - Cloudflare-protected; **reuses Session 1's existing Sprudge cookie path**.
- **Cafe → bean linkage: NOT VIABLE.** No public no-auth source links specific cafes to served beans. Cafe finder works at the location level only.
- **Integration:** `cafes(id, name, address, lat, lng, source, source_url, tags, last_seen_at)` table. Overpass on demand for radius queries; Sprudge guide pre-sync for the supported city set.

## Source Priority (carried from briefing + Multi-Source Priority Gate)

```
1. roaster-sites (the 24 elite specialty roaster catalogs — Session 1 list)
2. james-hoffmann (YouTube via youtube-pp-cli)
3. coffee-review (already in Session 1)
4. world-barista-championship (wcc.coffee + Sprudge cross-ref)
5. lance-hedrick (YouTube via youtube-pp-cli)
6. other-trusted-sources (extensible — Sprudge already included, others TBD)
```

Economics: all sources free. No paid keys. Confirmed by user during briefing.

## Reachability Risk (Session 2 additions)

| Source | Status | Notes |
|---|---|---|
| `youtube.com` (Hoffmann channel) | ✓ via youtube-pp-cli timedtext path | No auth on transcripts |
| `youtube.com` (Hedrick channel) | ✓ via youtube-pp-cli timedtext path | No auth on transcripts |
| `jameshoffmann.com` (blog) | ✗ ECONNREFUSED during probe | Deferred to v0.2; revisit reachability |
| `wcc.coffee` (WBC site) | ✓ 200 OK on plain HTTPS | Sparse data, HTML scrape |
| `en.wikipedia.org` (WBC article) | ✓ via Wikipedia REST API | Clean winners table |
| `sprudge.com` (city guides) | ✓ via existing Chrome-cookie path | Same fallback as editorial RSS |
| `overpass-api.de` (OSM) | ✓ public, no auth | Fair-use rate limits |

## New Integration Risks

1. **YouTube transcript extraction quality.** Bean mentions in transcripts are noisy (auto-captions misspell roaster names). Need fuzzy match against the 24-roaster slug set + LLM-assisted extraction as fallback.
2. **youtube-pp-cli subprocess dependency.** coffee-goat will need youtube-pp-cli installed on PATH. Surface this clearly in `doctor` + README install steps.
3. **WBC recipe data sparsity.** Many years lack structured recipe data. Commands that read this table must tolerate partial rows.
4. **OSM tagging variance.** Most cafes lack specialty-coffee tags. The `cafe-near` command will return both specialty and generic cafes; filtering quality is mapper-dependent.

## Codebase Intelligence (new dependencies)

- **youtube-pp-cli** — already published at `~/printing-press/library/youtube/youtube-pp-cli`. Surface: `youtube channel-uploads`, `youtube videos-transcript`, `youtube search-bulk`, all `--json`-capable. coffee-goat invokes via subprocess.
- **Overpass-Turbo** — Wiki for query patterns. Single endpoint, GET/POST.
- **wcc.coffee** — HTML pages, no API. Single scraper.

## User Vision (verbatim from Session 2 briefing)

See `user-briefing-context.md`. Summary: diehard enthusiast with 10 bags at home, palate-dial-in toward the "God cup," catalog every bean across roaster sites + Hoffmann/Hedrick YouTube + Coffee Review + WBC, plus cafe finder. No auth, low barrier to entry. Personas span home brewer / cafe owner / roaster / comp barista / curious agent.

## Build Priorities (Session 2)

The Session 1 build priorities (data foundation → personal data → tier-2 sources → editorial → 13 novel features → polish) stay first. Session 2 adds, after the Session 1 core lands:

7. **YouTube creator sync.** Adapter that calls `youtube-pp-cli` as subprocess, ingests `youtube_reviews` (creator, video_id, video_title, video_published_at, transcript_excerpt, mentioned_roaster_slug, mentioned_bean_handle, confidence_score). Bean-mention extraction is the hardest piece — start with regex over the 24-roaster slug set, add LLM fallback only if regex misses are common.
8. **WBC + Wikipedia sync.** Adapter for `wcc.coffee` past-rankings + Wikipedia winners table. Cache `champion_recipes` rows with partial-fill tolerance.
9. **Cafe finder.** Overpass query helper + Sprudge city-guide aggregator. `cafes` table.
10. **New transcendence commands** layered on top (proposed in Phase 1.5; subject to the user-locked manifest).

## Constraints Specific to Session 2

- **youtube-pp-cli must be on PATH** at runtime for any creator-related command. Doctor must check this.
- **`jameshoffmann.com` blog**: cut from this session's scope until reachability is verified. Not promoted to v0.2 commitment.
- **No cafe→bean linkage**: this is a hard limit; do not promise it. The `cafe-near` command returns cafes only.
