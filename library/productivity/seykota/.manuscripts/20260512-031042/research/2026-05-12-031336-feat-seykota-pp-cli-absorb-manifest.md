# Seykota CLI — Absorb Manifest

## Landscape (what exists)
No CLI, MCP server, Claude skill, or library wraps seykota.com's content. The "tools" in this space are:
- The site's own search box (`/tribe/search/`) — a Google site-search redirect.
- Third-party mirrors / summaries: `turtletrader.com/tribe/`, `tradingtribe.com/tribe/FAQ/` — read-only HTML, no search.
- `trendfollowing.com/whitepaper/DETERMI.PDF` — off-site PDF of *Determining Optimal Risk* (Seykota & Druz, S&C 1993).
- Generic web-based Kelly-criterion / position-sizing calculators — not CLIs, not tied to Seykota's text.

So there is almost nothing to "absorb feature-for-feature." The value is transcendence: one offline, structured, agent-native tool over content that today is unsearchable and uncalculable.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Find a concept in the FAQ | Google site-search box on seykota.com | `search <q>` over local FTS5 index of all crawled pages | Offline, ranked, `--json`/`--csv`/`--select`, filter by source/year/topic, returns source URL + date |
| 2 | Read a FAQ month | Browsing `/tt/FAQ_Index/` → click month | `faq` (browse by year/month/topic) + `faq show <year> <month>` | Offline, scriptable, no navigating frames; lists contributors |
| 3 | Read the TSP system rules | Browsing `/tribe/TSP/` sub-pages | `tsp list` + `tsp show <slug>` (EA, SR, Trends, Diversify, Continuous, Skid, Core, …) | Offline, one command, shows last-updated date + links |
| 4 | Read the risk essay | `/tribe/risk/index.htm` (one long page) | `risk show` (`--section <heading>` to jump) | Offline, sectioned, pipes to a pager cleanly |
| 5 | The Seykota/Druz position-sizing math | `DETERMI.PDF` (static PDF) / the risk essay's prose | `risk kelly`, `risk heat`, `risk uncle-point` calculators | The formulas become runnable commands with your own numbers |

## Transcendence (only possible with our approach)

From the novel-features subagent (customer model: Devon the systematic trend trader / Buffet-System operator; Mara the Market Wizards researcher; Sam the agent/automation builder). 6 survivors, all ≥ 5/10. (Note: offline FTS `search`, `risk kelly`, `risk heat --positions`, `risk uncle-point`, the topic vocabulary, and `sql` over the archive are all part of the absorbed/core surface above — they're table stakes here, not novel. These six are the *additional* leverage on top.)

| # | Feature | Command | Score | Why Only We Can Do This | Persona |
|---|---------|---------|-------|-------------------------|---------|
| 1 | Concept chronology | `timeline <query> [--json]` | 8/10 | Groups FTS5 hits from `faq_pages` (year/month), `tsp_sections`, `risk_essay` into a year-ordered list — "how did Seykota's thinking on heat appear across 2003→2023." The site's search is undated/unordered; no tool buckets matches by year. | Mara, Devon |
| 2 | Coin-Toss simulator | `risk coin-toss --win-rate --payoff --bet-fraction --trials --runs [--seed]` | 9/10 | Monte-Carlo of the essay's named Coin-Toss / fixed-fraction model — median terminal equity, ruin %, max drawdown, optimal-f comparison. The essay walks the simulation in prose; nothing runs it. Seeded → deterministic, verifiable. | Devon |
| 3 | Lake Ratio calculator | `risk lake-ratio --equity-curve <file\|->` | 9/10 | Computes Seykota's Lake Ratio (drawdown-"water" area ÷ equity-"land" area) over a CSV/stdin equity series. The essay defines it qualitatively (the lake metaphor); turning a curve into the number exists nowhere as a tool. | Devon, Sam |
| 4 | FAQ contributor index | `faq contributors [<name>]` | 7/10 | Parses contributor lines from `faq_pages` bodies; lists month-counts per contributor, or the months a given name appears in. Only possible with all ~266 pages in SQLite; exploits the FAQ's reader-mailbag structure. | Mara |
| 5 | Metric explainer | `risk explain <metric>` | 8/10 | Maps a curated metric vocabulary (heat, Kelly K, Uncle Point, Lake Ratio, Timid/Bold rules) onto the exact `risk_essay` section that defines it AND the `risk` calculator subcommand that runs it. Bridges the prose essay and the calculators, which the site keeps entirely separate. | Devon, Mara, Sam |
| 6 | Citation-formatted search | `cite <query> [--style faq\|tsp\|risk] [--bibtex] [--json]` | 8/10 | One citation line per FTS hit — source, date/section, snippet, URL — or BibTeX. Turns search results into ready-to-paste, attributed quotes; the raw site never surfaces the date/section metadata next to the text. | Mara, Sam |

Killed candidates (audit trail in the brainstorm file): `risk timid-bold` (thin), `faq topics` standalone (folded into `faq`), `tsp updated`/`tsp diff` (a `--sort` flag / fragile), `risk worksheet` (redundant w/ `risk heat --positions`), `risk kelly --fractional` (a flag), `read <url-or-id>` (generic), `index build --granular` (post-MVP), `related` ("more like this" needs embeddings), `stats` (no weekly pull).

## Stubs
None planned. Every row above ships fully. (`--full-archive` on `index build` — crawling the pre-2010 `/tribe/FAQ/` day-pages — is not a stub; the command works at MVP, the flag just widens the crawl. If the day-page HTML proves too irregular to parse reliably in-session it will be marked `(stub — irregular legacy HTML)` and re-presented here, not silently downgraded.)

## Build order
- **P0:** data layer (`faq_pages`, `tsp_sections`, `risk_essay`, FTS5) + `internal/crawl/` (the crawler+parser) + run the crawl once → vendored snapshot committed with the CLI.
- **P1 (absorb / core):** `search` (FTS, `--source`/`--year`/`--topic`/`--limit`/`--json`/`--select`), `faq` + `faq show` (+ topic vocabulary), `tsp list` (+ `--sort updated`) + `tsp show`, `risk show` (+ `--section`), `risk kelly`, `risk heat` (+ `--positions`), `risk uncle-point`, `index build [--full-archive]`, `sql` (wired + documented for the archive).
- **P2 (transcend — the 6 survivors):** `timeline <query>`, `risk coin-toss`, `risk lake-ratio`, `faq contributors`, `risk explain <metric>`, `cite <query>`.
- **P3:** flag-description polish, tests for `internal/crawl` (parser), `internal/risk` (calculators — Kelly/heat/uncle-point/coin-toss/lake-ratio), README/SKILL prose, MCP read-only annotations on all read commands.
