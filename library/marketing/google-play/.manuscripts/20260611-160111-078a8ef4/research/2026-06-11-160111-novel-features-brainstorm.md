# Google Play CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Priya — Market Intelligence Analyst, mid-size mobile game studio.** Maintains a hand-curated Sheet of ~40 competitor titles, eyeballing install counts and chart positions off play.google.com weekly. Monday "movers" report: which rivals climbed/dropped in TOP_GROSSING for US/GB/DE and which shipped updates. Frustration: no record of *last week* — Play shows only live state, so every rank movement is reconstructed from memory or screenshots; the studio pays for AppMagic mostly for the chart-history view the raw data already contains free.

**Marco — Solo indie ASO practitioner.** Runs facundoolano's npm scraper for autocomplete seeds; `reviews_all` broke in the Python port, the JS one 429s after 30 keywords. Weekly: checks where his game ranks for ~15 target terms across 3 countries, harvests long-tail keyword ideas. Frustration: ranking for a term is point-in-time with no trend — can't tell if last week's metadata change moved him up because nothing stored the before.

**Dana — Competitive-intelligence PM at a publisher.** Diffs competitor listings manually via screenshots, squinting for icon/title/screenshot/IAP/ads changes. Weekly competitor-watch digest across ~12 tracked apps. Frustration: change detection is manual and lossy; Sensor Tower paywalls change-history behind a seat she can't expense.

**Auto — AI research agent** in a pipeline answering "profile this app and its competitive set." Shells out to mixed-quality scrapers, gets HTML or untyped errors it can't distinguish from rate limits, retries blindly, gets IP-banned. Frustration: existing tools emit neither agent-shaped JSON nor a *typed* rate-limit signal, so it can't back off when Play returns `PlayGatewayError` inside HTTP 200.

## Survivors (transcendence rows)

| # | Feature | Command | Score | Buildability | Persona | Long Description |
|---|---------|---------|-------|--------------|---------|------------------|
| 1 | Chart rank history for one app | `rank-history <appId> --collection --category --country` | 8/10 | hand-code | Priya | Use for ONE app's rank trajectory over time within a chart. For ranking changes across the whole chart between two snapshots, use `movers` instead. |
| 2 | Chart movers between snapshots | `movers --collection --category --country --since` | 9/10 | hand-code | Priya | Use for the whole-chart diff between two points in time. For a single app's trajectory, use `rank-history` instead. |
| 3 | Listing change detection | `watch-listing <appId>` | 8/10 | hand-code | Dana | Use to see what changed on a tracked listing over time. For a current full field dump, use `app`. |
| 4 | Keyword rank capture (live + persist) | `keyword-rank <term> --country [--app]` | 8/10 | hand-code | Marco | Use to capture today's rank for a term (and persist it). For raw search results use `search`; for the trend use `keyword-history`. |
| 5 | Keyword rank history | `keyword-history <term> --country --app` | 7/10 | hand-code | Marco | Use for the rank trend of one term/app/country over time. To capture a fresh point first, run `keyword-rank`. |
| 6 | Review aggregation by version/star | `review-digest <appId> [--since-version]` | 7/10 | hand-code | Priya/Marco | Use for mechanical review stats (star/version histograms, reply rate, term frequency). For raw reviews use `reviews`; for prose summary pipe to an LLM. |
| 7 | Multi-app side-by-side | `compare <appId>...` | 6/10 | hand-code | Priya/Dana | Use to compare current details of multiple apps. For one app's full field set use `app`; for change-over-time use `watch-listing`. |

## Killed candidates
- `keyword-difficulty` — verifiability fail (unvalidatable composite), no research demand
- `monetization` — reimplementation/wrapper of fields `app` already returns
- `data-safety-score` — domain-fit weak (personas are game intel, not privacy auditors)
- `portfolio` — wrapper over absorbed `developer`; `developer --json | claude` covers it
- `competitive-graph` — scope creep + 429/IP-ban risk from depth-2 live fan-out
- `overlap` — speculative, verifiability flag, only works after deep sync
- `review-trend` — merged into `review-digest`
- `snapshot` — framework `sync --resources charts,reviews,keyword-ranks` already writes the snapshots
- `gaps` — speculative whitespace-finder, no research backing
