# Novel Features Brainstorm — google-trends

## Customer model

**Dana — in-house SEO/content lead at a mid-size company.**
Today: opens trends.google.com most mornings, types 2-3 candidate keywords, eyeballs the interest-over-time chart, then separately checks the related-queries box for content ideas, copy-pasting numbers into a spreadsheet by hand.
Weekly ritual: every Monday before the content calendar meeting, she re-checks 8-10 core topic keywords plus whatever writers pitched, looking for which related/rising queries are worth turning into briefs this week.
Frustration: Google Trends' UI has no memory — she can't tell whether a rising query she flagged three weeks ago is still rising or already peaked, so she re-eyeballs charts from scratch every time with no record of what she already decided to skip.

**Marcus — growth/brand marketing manager tracking brand vs. 2-3 competitors.**
Today: runs ad-hoc multi-keyword compares (brand + competitor names) before campaign planning and screenshots the chart for the deck.
Weekly ritual: Monday-morning check of brand-vs-competitor interest plus a regional breakdown to decide which markets get the next campaign push.
Frustration: every check starts from zero — there's no way to see whether the brand's regional lead over a competitor grew or shrank since last month without manually diffing two screenshots.

**Wren — data scientist building seasonality/demand-forecasting models.**
Today: pulls raw interest-over-time CSVs via pytrends scripts, manually stitches multi-year windows, and computes seasonal peaks in a notebook.
Weekly ritual: refreshes the underlying keyword set to feed a seasonality/ad-spend-timing model for merchandising planning.
Frustration: pytrends is archived and frozen, unpredictable 429s break automated pulls mid-run, and there's no built-in seasonality primitive — every peak is computed by hand after the raw pull, with no persisted cache to fall back on when a pull fails.

**Alicia — social/community manager doing newsjacking and ad-hoc "what's trending" checks.**
Today: checks trends.google.com/trending several times a day for newsjacking angles tied to her brand's category, and separately watches a couple of keywords for sudden spikes.
Weekly ritual: a retro on which trending moments the team caught vs. missed, which requires reconstructing "what was trending" on specific past days.
Frustration: the live trending page is a snapshot of right-now — once a topic scrolls off, it's gone, so she can't prove after the fact what was trending when a missed opportunity happened, and can't tell a genuine spike in her tracked keywords from normal noise without eyeballing a chart.

## Candidates (pre-cut)

| # | Feature | Command | Description | Persona | Source | Data Source | Kill/keep check |
|---|---------|---------|--------------|---------|--------|--------------|------------------|
| C1 | Trend History Search | `trends history search <query> --table related\|trending --db <path>` | Local FTS over every previously synced related-term and trending-topic row | Dana | (c) | local | Passes all checks |
| C2 | Snapshot Diff / Rising-Term Watchlist | `trends changes <keyword> --since <duration> --db <path>` | Diffs related-terms and interest scores between two dated sync snapshots | Dana | (c)/(f) | local | Passes all checks |
| C3 | Seasonality Index | `trends seasonality <keyword> --geo <geo> --db <path>` | Monthly averages, peak month, coefficient-of-variation from cached interest-over-time | Wren | (b) | computed | Passes |
| C4 | Cross-Keyword Divergence/Correlation | `trends divergence <kwA> <kwB> --db <path>` | Correlation/crossover calc over cached multi-keyword series | Marcus | (b) | computed | Flagged — close to absorbed compare feature |
| C5 | Time-Travel Trending Lookup | `trends trending at --date <date> --geo <geo> --db <path>` | Cached trending_topic rows filtered to a historical date | Alicia | (f) | local | Passes all checks |
| C6 | Content Opportunity Ranking | `trends opportunities <keyword> --db <path>` | Ranks rising related terms by rising-value x parent trend slope | Dana | (a) | computed | Passes |
| C7 | Keyword Portfolio/Tag Groups Report | `trends portfolio --tag <tag> --db <path>` | Aggregated report over a tagged keyword group | Marcus | (a) | local | Flagged — duplicates framework analytics --group-by |
| C8 | Anomaly/Breakout Alert | `trends anomaly <keyword> --db <path>` | Z-score of latest interest vs historical baseline | Alicia | (b)/(c) | computed | Flagged — overlaps with diff signal |
| C9 | Stale Query Report | `trends stale --older-than <duration> --db <path>` | Lists tracked keywords whose last sync is older than a threshold | Wren | (c) | local | Passes all checks |
| C10 | Weekly Digest Report | `trends digest --since <duration> --db <path>` | Bundle of deltas/new rising/region shifts | Marcus | (a)/(c) | local | Flagged — thin orchestration of C2+C8 |
| C11 | Related-Term Co-occurrence Graph Export | `trends graph <keyword> --db <path>` | Local self-join co-occurrence network | Dana | (c) | local | Flagged — no weekly-ritual need, unverifiable |
| C12 | Geo-Divergence Finder | `trends geo-gap <kwA> <kwB> --resolution <level> --db <path>` | Ranks per-region interest delta between two keywords | Marcus | (b)/(c) | auto | Passes |

## Survivors and kills

### Survivors
(see absorb manifest transcendence table — identical content)

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Cross-Keyword Divergence/Correlation | Too close to absorbed multi-keyword compare — thin-wrapper risk | Geo-Divergence Finder |
| Keyword Portfolio/Tag Groups Report | Duplicates generic framework `analytics --group-by` flag | Stale Query Report |
| Anomaly/Breakout Alert | Weaker evidence, overlaps with diff signal | Snapshot Diff / Rising-Term Watchlist |
| Weekly Digest Report | Thin orchestration wrapper, no new transcendence | Snapshot Diff / Rising-Term Watchlist |
| Related-Term Co-occurrence Graph Export | No weekly-ritual need, graph correctness not verifiable | Trend History Search |
