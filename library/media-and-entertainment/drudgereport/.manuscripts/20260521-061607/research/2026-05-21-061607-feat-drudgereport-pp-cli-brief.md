# Drudge Report CLI Brief

## API Identity
- **Domain:** drudgereport.com — Matt Drudge's hand-curated news aggregator since 1995. Editorially-driven link blog: every story is a single outbound headline link to a primary source (msn.com, washingtonpost.com, nytimes.com, etc.). No comments, no algorithm, no API.
- **Users:** News junkies, political analysts, journalists tracking media narratives, scrapers/researchers studying Drudge's framing influence, people who want one curated feed instead of an algorithmic timeline.
- **Data profile:** A single HTML page (~89 KB, 172 outbound links typical) plus image references. Page structure is comment-delimited (`<! MAIN HEADLINE >`, `<! LINKS FIRST COLUMN>`, `<! LINKS SECOND C OL U M N>`, `<! TOP LEFT STARTS HERE >`). Visual ranking is conveyed by `<font color=red>` (breaking/urgent), splash-image proximity (main slot), bold/all-caps for prominence, and physical column placement. No semantic h1/h2 — pure 1995-vintage HTML.

## Reachability Risk
- **None.** `probe-reachability https://drudgereport.com/` → `standard_http` (HTTP 200, 179 ms via stdlib). Same for the unofficial RSS at `https://feedpress.me/drudgereportfeed` → `standard_http` (200, 566 ms). No bot protection, no clearance cookie, no auth.

## Top Workflows
1. **Read the splash + breaking stories in one terminal command.** "What's on Drudge right now?" — surface the main headline, any other red items, and the top-of-column stories without opening a browser.
2. **Diff Drudge across time.** "What changed since I checked an hour ago?" — snapshot the page on each fetch, then surface promotions (story moved to splash), demotions (splash story now gone), new arrivals, and the headline's color changing to red.
3. **Direct outbound links, no Drudge wrapper.** The HTML's `<A HREF>` is the real outbound URL (Drudge doesn't proxy through a tracker). Power user wants `drudgereport headlines --links-only` to feed a downstream reader.
4. **Source aggregation.** "Which outlets does Drudge feature most?" — aggregate outbound domains over time to track Drudge's editorial leanings and identify rising/falling sources.
5. **Splash tenure.** "How long has THIS story been the splash?" — Drudge will leave a single story up for days when it matters; this is a strong newsroom-signal a reader can use.

## Table Stakes
- Headlines list with importance ranking (splash → red items → main columns).
- Direct outbound URL extraction (no tracking wrappers).
- Color/style detection (`color=red` flag captured per story).
- Section/slot labeling (main headline vs first column vs second column vs top-left rail).
- JSON output for piping.
- Image URL extraction for the splash and column-top images.
- Search across current and historical headlines.

## Data Layer
- **Primary entities:** `story` (one row per headline observed at a point in time: title, url, slot, is_red, has_image, image_url, captured_at), `snapshot` (one row per page fetch with timestamp + raw-HTML hash), `slot_event` (story moved between slots / went red / went black / appeared / disappeared).
- **Sync cursor:** Each fetch is a full page; dedup by (title-normalized, url) into stable story_ids so the same story across snapshots is one entity in analytics.
- **FTS/search:** FTS5 over `story.title` so `drudgereport search 'iran'` returns every observed headline mentioning Iran with the snapshots it appeared in.

## Codebase Intelligence
- **No official wrapper.** The most-cited community projects:
  - `mattrasband/drudge_parser` (Python, archived Nov 2021) — pure stdlib parse, returns ordered groups of `(images[], articles[], location)` where `location ∈ {TOP_STORY, MAIN_HEADLINE, COLUMN1..3}`. Models the page as positional sections, **not** style cues — never captures `color=red`.
  - `mattrasband/drudge.in` (companion site/scraper, also archived).
  - `ghayward/drudge_report_headline_scraper` (Python+BeautifulSoup) — dumps current headlines to CSV/Excel. No styling.
  - `lukerosiak/drudge` (Python, postgres-backed) — fetches every 5 min, calculates "types of stories" stats, optional email alerts. Closest in spirit to what we'll build; archived/inactive.
  - `JonathanBrownCFA/scrape-it-like-you-mean-it` (Node+cheerio+mongoose) — web app shape, not a CLI.
- **Unofficial RSS:** `feedpress.me/drudgereportfeed` (via `drudgereportfeed.com` redirect). 37 items in current sample, each item's CDATA description contains EXPLICIT position labels: `(Main headline, 1st story, link)`, `(First column, 1st story, link)`, `(Second column, 2nd story, link)`, plus `color: red` inline style on the splash, plus pre-grouped "Related stories". GUID is the real outbound URL (deduped/normalized). pubDate per item.
- **No MCP server exists for Drudge Report.** Confirmed via web + Anthropic MCP registry searches.
- **No Claude Code plugin / skill exists for Drudge.**
- **No published Printing Press CLI exists for Drudge.** Closest peers in the library: `digg` (AI news leaderboard), `hackernews` (HN), `marginalrevolution` (econ blog).

## User Vision
> "Pretty simple just want top headlines, more love of in the middle spot or has colors in terms of ranking importance like drudge marks something as red or goes to main slot. Direct links nice too."

Translation: importance signal = splash + center placement + red color, not chronology. Direct outbound URLs matter (no Drudge-wrapper). This is the editorial-signal-aware reader.

## Product Thesis
- **Name:** `drudgereport-pp-cli` (binary), `drudgereport` (library slug). Display: **Drudge Report**.
- **Why it should exist:** Every existing Drudge scraper either dumps a flat list (loses Drudge's editorial signal entirely) or runs as a long-lived web service (heavy for someone who just wants today's splash). The slot + red-color information IS the value — Drudge's editorial decision is what makes the site worth reading, and no current tool exposes it. Plus: every other scraper is archived. We can be the live, agent-native, offline-searchable Drudge surface that didn't exist before.

## Build Priorities
1. Parser that lights up Drudge's editorial signals (slot label, red flag, image presence, column position) from the comment-marker-delimited HTML, with the RSS feed as a cross-check fallback.
2. SQLite snapshot store: dedup story_ids across snapshots so promotion/demotion events are query-able and FTS5 search works across the historical archive of headlines the CLI has observed.
3. Novel commands keyed on editorial signal: `splash`, `breaking` (red), `headlines` (ranked), `tail` (promotions/demotions since last fetch), `tenure` (how long the splash has been splash), `sources` (outbound-domain leaderboard over time).
4. Agent-native output everywhere: `--json`, `--select`, typed exit codes; auto-detect terminal vs pipe.
5. README/SKILL polish + dogfood against live drudgereport.com.
