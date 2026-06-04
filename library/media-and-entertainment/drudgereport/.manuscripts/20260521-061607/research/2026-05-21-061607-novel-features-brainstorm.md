## Customer model

**Persona 1: Greg, 56, retired corporate lawyer in Naples FL**
- **Today (without this CLI):** Greg opens drudgereport.com in Safari at 5:45am with coffee, scans the splash and the red headlines, and clicks through three or four stories before the news cycle even hits cable. He has the page bookmarked as his home tab and reloads it manually every couple of hours through the day. He doesn't use Twitter, Facebook, or RSS readers — Drudge IS his feed.
- **Weekly ritual:** Saturday morning he texts his brother screenshots of the splash with one-line takes. Sunday he tries to remember what was on the splash Wednesday for a dinner argument — and can't, because Drudge has no archive.
- **Frustration:** He can't answer "what did Drudge lead with on Tuesday?" The site has no history, no search, no notion of yesterday. When the splash changes during the day he never knows when it changed, and a story he meant to read disappears.

**Persona 2: Dana, 34, political reporter at a mid-sized DC outlet**
- **Today (without this CLI):** Dana keeps Drudge open in a pinned tab next to Memeorandum and Axios. She watches the splash and the red items because Drudge's lead still drives a measurable share of right-leaning attention by mid-morning. When she's writing a media-criticism piece she manually screencaps the page through the day.
- **Weekly ritual:** Friday she pulls together a "what Drudge featured this week" graf for her newsletter — domain mix, how long the splash sat, which outlets got pulled in. Today she does this by scrolling her screenshot folder.
- **Frustration:** No tool tells her which outlets Drudge leaned on this week, how long the splash sat, or what got promoted and demoted. Every existing scraper either lost the slot/color signal or is dead.

**Persona 3: Sam, 41, indie newsletter author tracking media narratives**
- **Today (without this CLI):** Sam writes a Substack about how legacy aggregators set the day's narrative. He has a half-broken Python scraper from 2019 that dumps Drudge headlines to CSV but loses every editorial cue. He scrapes once a day at 8am.
- **Weekly ritual:** Sunday he pulls together a "splash tenure" chart by hand — which stories Drudge let sit on the splash for >12 hours. That's the chart that drives his newsletter's most-clicked posts.
- **Frustration:** He needs slot transitions over time and a stable story_id across snapshots, neither of which any existing tool gives him. Manually diffing CSVs each morning eats an hour.

**Persona 4: Riley, 28, an agent (LLM) being asked "what's on Drudge?"**
- **Today (without this CLI):** Riley curl's drudgereport.com, gets 89 KB of 1995-vintage HTML, hallucinates the splash, and quotes a sub-headline as the lead. There's no semantic structure to ground on.
- **Weekly ritual:** Riley is called dozens of times a week by Sam-style users asking "is Drudge still leading with X" and "what's red on Drudge right now." Each call burns tokens parsing HTML.
- **Frustration:** Needs a typed JSON surface — slot label, is_red, image presence, captured_at — instead of an HTML soup it has to re-parse on every call.

## Candidates (pre-cut)

| # | Name | Command | One-liner | Persona | Source | Inline verdict |
|---|------|---------|-----------|---------|--------|---------------|
| C1 | Splash now | `splash` | Show only the current splash: title, outbound URL, image, is_red, splash_tenure | Greg, Riley | (e), (b) | Keep |
| C2 | Breaking (red items) | `breaking` | List every story currently in `color=red`, ranked by slot | Greg, Dana | (e), (b) | Keep |
| C3 | Ranked headlines | `headlines` | All headlines ranked by composite editorial signal | Greg, Riley | (e), (b) | Keep |
| C4 | Promotions/demotions since last fetch | `tail` | Slot transitions between consecutive snapshots | Sam, Dana | (c), (b) | Keep |
| C5 | Splash tenure | `tenure` | How long current splash has been splash; longest splashes | Sam, Dana, Greg | (b), (e) | Keep |
| C6 | Source leaderboard over time | `sources` | Outbound-domain frequency leaderboard with deltas | Dana, Sam | (c), (b) | Keep |
| C7 | Yesterday's splash / on-this-day | `on-date YYYY-MM-DD` | Show splash + red items at a given timestamp | Greg, Dana | (c) | Keep |
| C8 | Splash diff over the day | `diff --since 12h` | Splashes ran in a window with tenure intervals | Sam, Dana | (c) | Reframe→folded into C5 |
| C9 | "Has X been on Drudge?" | `mention <term>` | FTS5 query | Dana, Greg | (c) | Folded into built-in search |
| C10 | Sources-by-slot crosstab | `sources --by-slot` | Domain leaderboard × {splash, red, column} | Dana | (c), (b) | Folded as flag on C6 |
| C11 | Image-only / splash-image archive | `images` | Splash-image URL history | Sam | (b) | Cut |
| C12 | Live watch (poll once) | `watch --once` | Single-shot diff against previous snapshot | Riley, Sam | (c), (b) | Cut (dup of C4) |
| C13 | Editorial-bent index | `bent` | Ratio of red items by domain | Dana, Sam | (c), (b) | Keep |
| C14 | Story timeline | `story <id>` | All slot_events for one story_id | Sam, Dana | (c), (b) | Keep |
| C15 | "Drudge driving narrative" detector | `driving` | Long-tenure stories from niche sources | Sam | (b), (c) | Cut (speculative, unverifiable) |
| C16 | Daily/weekly digest | `digest --week` | One-pager composite summary | Dana, Sam | (c) | Keep |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Splash now | `drudgereport splash` | 9/10 | hand-code | Parses current HTML for `<! MAIN HEADLINE >` marker, extracts title + `<A HREF>` + adjacent `<IMG>` + `<font color=red>` flag, computes splash_tenure by querying local `story` table for earliest captured_at where slot=splash AND story_id matches | User vision quote; brief workflow #1; absorb-manifest gap (no peer surfaces splash slot alone with red flag); RSS feed `(Main headline, 1st story)` |
| 2 | Breaking (red items) | `drudgereport breaking` | 8/10 | hand-code | Filters parsed stories on `is_red=true`, returns ordered by slot rank | Brief data layer (`is_red`); user vision ("colors in terms of ranking importance"); mattrasband/drudge_parser explicitly does NOT capture color |
| 3 | Ranked headlines | `drudgereport headlines [--limit N]` | 8/10 | hand-code | Orders parsed stories by composite rank: slot weight (splash=100, red=80, column-top=60, column-body=40, top-left=30) + is_red bonus + has_image bonus | Brief Build Priority #1; user vision verbatim; no peer ranks by editorial signal |
| 4 | Promotions / demotions since last fetch | `drudgereport tail [--since DURATION]` | 9/10 | hand-code | SQL over `slot_event` rows where ts > last fetch (or `--since` window): emits {appeared, promoted_to_splash, demoted_from_splash, went_red, went_black, disappeared} keyed on stable story_id | Brief workflow #2; data layer `slot_event`; no peer keeps snapshot history; Sam's frustration |
| 5 | Splash tenure | `drudgereport tenure [--history]` | 8/10 | hand-code | Queries local store for earliest captured_at of current splash story_id; `--history` returns top-N longest-tenured splashes | Brief workflow #5; Sam's weekly chart; no peer tool has this concept |
| 6 | Source leaderboard with deltas | `drudgereport sources [--window 7d] [--by-slot]` | 8/10 | hand-code | Aggregates outbound_domain over window with COUNT and prior-window delta; `--by-slot` crosstabs domain × {splash, red, column} | Brief workflow #4; absorb manifest #4 + new deltas/crosstab; Dana's frustration |
| 7 | On-this-date | `drudgereport on-date YYYY-MM-DD[THH:MM]` | 7/10 | hand-code | SELECT against snapshots table for nearest snapshot, reconstructs ranked headlines + splash + red items as of that moment | Greg's Sunday argument; brief data layer; Drudge has no archive |
| 8 | Editorial-bent index | `drudgereport bent [--window 7d]` | 7/10 | hand-code | For each outbound_domain, COUNT(red items) / COUNT(total items) over window, ranked; surfaces "which sources Drudge breaks vs columns" | Dana's Friday newsletter; requires joining stories.is_red × outbound_domain × snapshots |
| 9 | Story timeline | `drudgereport story <story_id>` | 6/10 | hand-code | SELECT all slot_event rows for one story_id ordered by ts, plus first/last captured_at and total tenure across all slots | Stable story_id is THE data model commitment; Sam's narrative tracking; no peer has stable IDs |
| 10 | Weekly digest | `drudgereport digest [--week\|--day]` | 6/10 | hand-code | Composes splash-history + tenure + sources + bent into one JSON-and-pretty-print summary | Dana's Friday newsletter; assembles primitives; no peer summarizes |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C8 — `diff --since 12h` | Overlaps `tail` (events) and `tenure --history` (ranges) | C5 `tenure --history` |
| C9 — `mention <term>` | Table-stakes `search` covers FTS5; `--slot splash` is a flag on `search`, not a new command | C3 `headlines` / built-in `search` |
| C11 — `images` | Thin weekly use; `splash --json` / `headlines --json` already expose `image_url` | C1 `splash` |
| C12 — `watch --once` | Duplicate of `tail` once polling implication is removed | C4 `tail` |
| C15 — `driving` | Requires cross-source "who broke first" data we don't have; speculative | C8 `bent` |
