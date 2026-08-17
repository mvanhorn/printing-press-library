# sensortower-pp-cli — Absorb Manifest

Surface: `app.sensortower.com/api/*` (internal dashboard API, free/anonymous tier).
Not `api.sensortower.com` — unreachable on this account (`api_authorized: false`).

## Absorb Manifest

### Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Catalog search by app name | FerdiKT/sensortower-cli `search apps` | (generated endpoint) find | `--json`, `--select`, typed exit codes, cached to SQLite |
| 2 | Catalog search by publisher | FerdiKT/sensortower-cli `search publishers` | (behavior in sensortower-pp-cli find) `--entity-type publisher` switches the catalog | One command, one flag |
| 3 | App profile lookup | FerdiKT/sensortower-cli `apps get` | (generated endpoint) apps get | Full 51-key hub object, agent-native output |
| 4 | Batch iOS app lookup | ivangomozov/Sensortower-top100 | (generated endpoint) apps ios | Multi-ID in one rate-limited call |
| 5 | Android app lookup | evekeen/appstore-revenue-mcp | (generated endpoint) apps android | Android parity (FerdiKT is iOS-only) |
| 6 | Category rankings / top charts | FerdiKT `charts category-rankings` | (generated endpoint) rankings ios | Cached, offline-queryable |
| 7 | Android category rankings | (none on free surface) | (generated endpoint) rankings android | Android parity |
| 8 | Publisher portfolio | FerdiKT `publishers apps` | (generated endpoint) publishers apps | Cached, joinable |
| 9 | Rank history over date range | (none) | (generated endpoint) category history | Direct endpoint access |
| 10 | Ranking summary per app | (none) | (generated endpoint) category summary | Direct endpoint access |
| 11 | Cross-platform app identity | (none free) | (generated endpoint) apps unified | Cookie-gated join |
| 12 | Cross-platform publisher identity | (none free) | (generated endpoint) publishers unified | Cookie-gated join |
| 13 | Category reference data | (none) | (generated endpoint) categories | Free enum reference |
| 14 | Version / update history | econosopher/sensortowerR (paid API) | (behavior in sensortower-pp-cli apps get) `versions[]`, 400–500 entries | Free, no contract |
| 15 | In-app purchase listing | AppTweak / Appfigures (paid) | (behavior in sensortower-pp-cli apps get) `top_in_app_purchases` | Free |
| 16 | Rating breakdown histogram | AppTweak (paid) | (behavior in sensortower-pp-cli apps get) `rating_breakdown` | Free |
| 17 | Related / similar apps | AppTweak (paid) | (behavior in sensortower-pp-cli apps get) `related_apps` | Free |
| 18 | Offline full-text search | (framework) | (framework) search | FTS5 over synced apps/publishers |
| 19 | Raw SQL over local mirror | (framework) | (framework) sql | Composable analysis |

No stubs. Every row above ships fully.

### Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Category movers and new entrants | `movers <category>` | 10/10 | hand-code | Reads chart rows' in-payload `previous_rank` from the local `rankings` table and set-differences the current snapshot against the prior one to flag new entrants — impossible from any single API call | Use this command to find which apps climbed, fell, or newly appeared in a category chart. Do NOT use this command to compare free-chart vs grossing-chart standing for the same category; use 'divergence' instead. Do NOT use this command for apps you already track by name; use 'watch digest' instead. |
| 2 | Release-vs-rank teardown | `teardown <app-id>` | 10/10 | hand-code | Joins the `versions[]` array from the `/api/ios/apps/{id}` hub object against locally stored daily rank history, aligning each release with the rank change in the following window | Use this command to investigate why one app's chart position changed, by aligning its release timeline against its rank history. Do NOT use this command to compare the same product's iOS and Android standing; use 'compare' instead. Do NOT use this command to scan a category for movers; use 'movers' instead. |
| 3 | Weekly watch digest | `watch digest` | 9/10 | hand-code | Reads a local `watchlist` table joined against the local `rankings` mirror to print each tracked app's rank per chart plus its delta since the previous sync — the API has no watchlist concept and no history at all | Use this command for the recurring "what moved on my tracked apps since last week" check across the whole watchlist. Do NOT use this command to investigate why one specific app moved; use 'teardown' instead. Do NOT use this command to scan a whole category chart for unknown apps; use 'movers' instead. |
| 4 | Free-vs-grossing divergence | `divergence <category>` | 8/10 | hand-code | Joins the local `rankings` table's `top_free` and `top_grossing` rows for one category and reports the widest rank spreads, using exact ranks only and never the bucketed money | Use this command to find monetization outliers within a category by comparing an app's free-chart rank to its grossing-chart rank. Do NOT use this command to find week-over-week rank changes; use 'movers' instead. |
| 5 | Cross-platform comparison | `compare <ios-id> <android-package>` | 7/10 | hand-code | Calls the cookie-gated `/api/unified/apps` to resolve one product identity, then joins both platforms' local rank ladders, release cadences, and rating histograms side by side | Use this command to compare the same product's iOS and Android standing after resolving cross-platform identity. Requires a session cookie. Do NOT use this command to compare two different apps on one platform; use 'teardown' on each instead. |

**Buildability tally:** 5 rows, all `hand-code`. 0 `spec-emits`.

### Global invariants (not commands — enforced across every command)

Two candidates were cut as invariants rather than features; they are recorded here so
Phase 3 implements them:

1. **Never render bucketed money as precise.** `worldwide_last_month_revenue` /
   `worldwide_last_month_downloads` are 1-significant-figure buckets
   (`{value: 100000, unit: "cent"}`, `"< $5k"`). Every command that surfaces these must
   preserve the bucket semantics (prefix/`humanized_*` strings), never a bare number.
2. **Adaptive rate limiting + typed error in the data layer.** ~13 requests → 429, ~240s
   recovery, `server: awselb/2.0`, no `Retry-After`, no `X-RateLimit-*`. The limiter belongs
   in the client, and throttling must surface a typed rate-limit error — never empty results
   (empty-on-throttle is indistinguishable from "no data exists").

### Killed candidates (audit trail)

Full reasoning in `2026-07-17-030012-novel-features-brainstorm.md`. Summary: `portfolio`
(episodic not weekly), `rank-gap` + `cadence` (fold into `teardown`), `iap-ladder`
(teardown-moment not weekly), `rating-drift` (too slow-moving for weekly signal),
`category-heat` (inferred demand, no research backing), `related-graph` (depth-2 stalls
against the rate limit), `budget` (would print CLI state as API fact), `apps get --money`
(invariant, not a feature).
