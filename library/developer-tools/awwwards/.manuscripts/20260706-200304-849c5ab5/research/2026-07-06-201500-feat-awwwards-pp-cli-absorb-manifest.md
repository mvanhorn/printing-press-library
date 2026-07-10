# Awwwards CLI — Absorb Manifest

Sources cataloged: jam3/awwwards-stream (dead 2016), koelll/awwwards npm (dead 2018, repo deleted), awwwards-best-of (dead 2017), awwwards-of-the-day (dead 2017), Apify easyapi actor (paid, living), bitbash/TS22082 scrapers (trivial), site-native surfaces (elements, collections, directory, filters). Competitor taxonomy references: httpster, land-book, lapa.ninja, siteinspire, recent.design. No official API, no maintained wrapper, no real MCP server exists.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Latest winners feed (the RSS that never existed) | koelll RSS-generator (dead) | awwwards-pp-cli latest | Parsed JSON cards, offline mirror, --json/--select/--compact |
| 2 | Browse by award tier (sotd/sotm/soty/nominees/honorable/developer) | jam3 awwwards-stream types (stale URLs) | (generated endpoint) websites browse | Works against current URL scheme; raw links + page meta |
| 3 | Filter by category / tag / tech | koelll filter matrix {technology, category, tag} | (behavior in awwwards-pp-cli find) --category/--tag/--tech flags | Client-side AND-intersection — server allows ONE filter at a time |
| 4 | Filter by country | koelll {country} | (behavior in awwwards-pp-cli find) --country | Composable with all other filters |
| 5 | Filter by color | koelll {color} | (behavior in awwwards-pp-cli find) --color <hex> | Composable; exact server filter + local palette data |
| 6 | Filter by font | site filter URLs (/websites/Aeonik/) | (behavior in awwwards-pp-cli find) --font | Composable |
| 7 | Free-text search | site ?text= | (behavior in awwwards-pp-cli find) --text | Composable with all other filters |
| 8 | Rank winners by sub-score | awwwards-best-of (its sole feature) | awwwards-pp-cli top | Any dimension, any filter, local mirror = no N+1 refetch |
| 9 | Per-site deep-dive (scores, palette, tags, credits) | awwwards-best-of per-site scrape | awwwards-pp-cli inspect | Overall + 4 dims + jury votes + palette + dev-award + tech |
| 10 | Individual jury votes | site detail page | (behavior in awwwards-pp-cli inspect) jury section in JSON output | Juror name + country + per-dimension scores |
| 11 | Multi-page fetch with rate limiting | jam3 startPage/pages/rate=250ms | (behavior in awwwards-pp-cli sync) --max-pages + adaptive limiter | Resumable, persisted to SQLite, typed RateLimitError |
| 12 | Thumbnail URL construction | Apify actor output | (behavior in awwwards-pp-cli latest) thumbnail_url field in JSON output | Ready-to-fetch thumb_440_330 / thumb_880_660 URLs |
| 13 | Structured export JSON/CSV | Apify JSON/CSV/XLSX ($2.99/1k) | (behavior in awwwards-pp-cli find) global --json/--csv/--select/--compact | Free, local, framework-native |
| 14 | Collections browsing | site /collections/ | (generated endpoint) collections list + collections get | Curated theme boards (dark-mode, hot-right-now, ...) |
| 15 | Agency/freelancer directory | site /directory/ | (generated endpoint) directory list + directory browse | Specialty/country filters |
| 16 | Element/section browsing | site /elements/ | (generated endpoint) elements browse | hero/footer/404_page section index |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona | Long Description |
|---|---------|---------|-------|--------------|------------------------|---------|------------------|
| 1 | Trend snapshot | trends --by tag\|color\|tech\|font --since 90d [--vs 90d] | 10/10 | hand-code | Site has zero aggregate view; frequency counts + window-over-window deltas need the local SQLite mirror | Suki (trends writer) | Use this command for aggregate frequency of tags, colors, tech, or fonts over a time window. Do NOT use this command to rank individual sites by jury score; use 'top' instead. |
| 2 | Design context pack | context-pack --category <c> [--tag t --tech t --color hex] | 10/10 | hand-code | One-shot agent briefing (top sites + dominant palettes + fonts + tech + score percentiles) synthesizes five data families no single page contains | Alex (agent-driven builder) — the User Vision verbatim | Use this command to assemble a complete design-context pack (reference sites, palettes, fonts, tech, score benchmarks) for one design brief. Do NOT use this command for a single aggregate metric; use 'trends' instead. |
| 3 | Fuzzy palette match | palette-match <hex> --distance <n> | 8/10 | hand-code | Server color filter is exact-hex only; RGB-distance nearest-color needs local palette rows | Mara (art director) | Use this command to find sites whose palette contains a color NEAR a given hex (fuzzy, local mirror). Do NOT use this command for exact server-side color filtering; use 'find --color' instead. |
| 4 | Ranked elements | elements-top <type> --dim design [--min 8] | 8/10 | hand-code | /elements/ is a flat unranked wall; ranking needs a local join of elements to parent-site jury scores | Diego (creative dev) | Use this command to rank section screenshots by their parent site's jury score from the local mirror. Do NOT use this command for a raw live listing of a section type; use 'elements browse' instead. |
| 5 | Studio profile | studio <name> | 6/10 | hand-code | Win history + tier counts + average dimension scores + tag profile is a credits-keyed join across websites and directory tables | Mara / agency users | Use this command for an aggregated award profile of one agency or studio (wins, average scores, tag profile). Do NOT use this command for browsing or filtering the agency listing; use 'directory browse' instead. |

Killed candidates (9) and full customer model: see 2026-07-06-201500-novel-features-brainstorm.md.

## Hand-code scope commitment

- 5 transcendence rows: all `hand-code` (trends, context, palette-match, elements-top, studio)
- 4 absorbed rows are hand-built flagship commands: latest, find, top, inspect (the parser-backed surface)
- Shared foundation (hand-code): internal/awwwards parser (card JSON, detail scores/jury/palette), store migrations for typed tables (websites, palette, jury_votes, elements, studios), HTML-aware sync path
- Generated by spec: websites list/browse, sites get/content, elements browse, collections list/get, directory list/browse (raw HTML fetch/links) + framework commands (sync scaffold, search, sql, doctor, context/agent tooling)

No stubs. All 21 rows (16 absorbed + 5 transcendence) are shipping scope.
