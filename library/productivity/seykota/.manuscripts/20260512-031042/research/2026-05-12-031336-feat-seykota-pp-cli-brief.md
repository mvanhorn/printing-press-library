# Seykota (seykota.com) CLI Brief

## API Identity
- **Domain:** Ed Seykota's site — the *Trading Tribe* archive. Static HTML, no API, no auth. Three content bodies the user named: (1) the **FAQ archive** (~20 yrs of dated Q&A on trend following, "heat"/risk-per-trade, position sizing, pyramiding, exits, whipsaws, system design, trading psychology / the Trading Tribe Process); (2) the **Trading System Project (TSP)** — the Tribe's collaborative mechanical trend-following spec + charts/rules pages; (3) the **"Risk Management" essay** at `/tribe/risk/` — Seykota's standalone position-sizing math (Coin-Toss model, fixed-fraction betting, Kelly `K = W − (1−W)/R`, Timid/Bold trader rules, the Uncle Point, Lake Ratio, portfolio heat).
- **Users:** systematic/trend-following traders, students of *Market Wizards*, anyone implementing position-sizing rules; here specifically — research substrate for Buffet-System's Schwager-derived strategies (Gold Futures already runs a Seykota trend signal).
- **Data profile:** ~266 FAQ month-pages at `/tt/FAQ_Index/` (2010–2023, ~12 KB HTML / ~5 KB text each); a few hundred older day-pages at `/tribe/FAQ/` (2003–2010); ~10 TSP section pages (`resources`, `Data_Verification`, `Continuous` [~17 KB], `EA`, `SR`, `Trends`, `Diversify`, `Skid`, `Core`, `Further_Research_CC`); one ~30 KB-text risk essay page. Whole MVP corpus (266 month-pages + TSP + risk) ≈ 3–4 MB — small enough to vendor in full.

## Reachability Risk
- **None.** Plain static HTML, HTTP 200 on every probed page (`/`, `/tt/FAQ_Index/`, `/tribe/TSP/index.htm`, `/tribe/risk/index.htm`, sample month-pages). No bot protection, no client-rendering, no auth. `probe-reachability` class: `standard_http`. Browser-sniff would capture zero XHR traffic — there is no API surface; the content IS the HTML.

## Top Workflows
1. **Search the FAQ for a concept** — "what did Seykota say about whipsaws / heat / pyramiding / the Uncle Point?" → ranked hits across 20 years of Q&A, each with year/month and source URL.
2. **Pull up a specific FAQ month** — `faq 2007 Jul` → that month's Q&A text, offline.
3. **Read a TSP section's rules** — `tsp show EA` (exponential-crossover system rules), `tsp show SR` (support/resistance), `tsp show Trends` — the actual mechanical-system definitions.
4. **Get the risk math + run it** — `risk show` (the essay) and the calculators it describes: `risk kelly --win-rate W --payoff R`, `risk heat --equity --risk-pct --entry --stop`, `risk uncle-point`.
5. **Refresh the local index** — re-crawl seykota.com (politely, rate-limited) so search reflects the current site, optionally including the older `/tribe/FAQ/` era.

## Table Stakes
There is **no existing CLI** for this content — nothing to absorb feature-for-feature. The only "competitors" are: the site's own Google site-search (`/tribe/search/`), third-party mirrors/summaries (`turtletrader.com/tribe/`, `tradingtribe.com/tribe/FAQ/`), the off-site `trendfollowing.com` PDF of *Determining Optimal Risk*, and generic web-based Kelly/position-sizing calculators (not CLIs). Table stakes therefore = "do what those do, but offline, structured, agent-native": full-text search, browse-by-date, the risk math, and the position-sizing calculators — none of which exists in one tool today.

## Data Layer
- **Primary entities:** `faq_pages` (id, year, month, range, url, title, contributors, body, fetched_at) · `tsp_sections` (slug, title, url, updated, body, fetched_at) · `risk_essay` (one row, or sectioned: heading, body) · plus FTS5 virtual tables over the bodies.
- **Sync cursor:** `fetched_at` per row; `index build` re-crawls, `--full-archive` adds the `/tribe/FAQ/` day-pages.
- **FTS/search:** SQLite FTS5; `search` ranks across all three bodies, filterable by `--source faq|tsp|risk`, `--year`, `--topic`.
- **Vendored snapshot:** the MVP crawl (266 month-pages + ~10 TSP pages + risk page) is committed with the CLI so `search`/`faq`/`tsp`/`risk` work with zero network out of the box (recipe-goat pattern).

## User Vision
Build the CLI now via `/printing-press` (decided in the approved plan). FAQ archive is the primary source; TSP + Risk essay included. Original plan said "Aggregation-pages MVP (light)" — but recon found the `/tt/Aggregation/Seykota_FAQ-1.html` page is a single ~163 KB *partial* file, not a full concatenation. Corrected MVP scope: crawl the `/tt/FAQ_Index/` monthly archive in full (~266 small pages) — actually lighter to reason about and gives complete per-month coverage. The pre-2010 `/tribe/FAQ/` day-page era is the `--full-archive` extension.

## Product Thesis
- **Name:** `seykota` — "Ed Seykota's Trading Tribe archive on the command line: 20 years of FAQ, the Trading System Project rules, and the risk-of-ruin math — searchable offline, plus the position-sizing calculators built in."
- **Why it should exist:** Seykota's writing on heat / position sizing / system design is the canonical primary source for trend-following risk control, but it's locked in a sprawling 1990s static site with no search worth using and no calculators. This makes it queryable (offline, agent-native, citeable) and turns the risk essay's formulas into runnable commands.

## Build Priorities
1. **Data layer + crawler + vendored snapshot** — `faq_pages` / `tsp_sections` / `risk_essay` tables, FTS5, an `internal/crawl/` package, and a committed snapshot built by running the crawl once during this build.
2. **`search`** — FTS over all three bodies; `--source`, `--year`, `--topic`, `--limit`, `--json`, `--select`. Each hit: title, year/month or section, snippet, source URL.
3. **`faq`** — `faq` (browse: `--year`, `--month`, `--topic`); `faq show <year> <month>` (full month text); `faq topics` (the curated topic vocabulary → which pages mention them).
4. **`tsp`** — `tsp list` (sections + last-updated); `tsp show <slug>` (one section's rules/notes/links).
5. **`risk`** — `risk show` (the essay, or `--section <heading>`); `risk kelly --win-rate --payoff`; `risk heat --equity --risk-pct --entry --stop` (and `--positions` to sum portfolio heat); `risk uncle-point --equity --drawdown-pct`.
6. **`index build [--full-archive]`** — re-crawl, rate-limited; `--full-archive` adds the `/tribe/FAQ/` day-pages.
7. Standard generated chrome: `doctor`, `sql`, `--json/--csv/--select`, MCP mirror, README/SKILL.

## Notes / risks
- FAQ month-pages bundle several Q&As each (a contributor writes in; Seykota replies, sometimes with charts). MVP stores **one row per month-page** with full text — fine-grained per-Q&A parsing is fragile HTML work, deferred (could become a `--granular` index option). Search still works; you just land on the month, not the individual exchange.
- Charts/figures on TSP and FAQ pages are images — not captured as text; the body text around them is.
- The generator is API-shaped; this is a content archive. Approach: a small hand-authored internal YAML spec describing seykota.com's HTML page endpoints (so the generator emits the scaffold + chrome), then the data layer / search / browse / calculators are hand-built in Phase 3 (Priority 0–2).
