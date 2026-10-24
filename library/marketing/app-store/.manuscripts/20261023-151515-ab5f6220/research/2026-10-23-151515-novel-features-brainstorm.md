## Customer model

**P1. The indie iOS developer who checks their category rank in each market after every release.** Owns one to three apps. Sees their own units in App Store Connect but nothing about competitors and no rank history.

*Today (without this CLI):* They change the storefront code in apps.apple.com URLs one country at a time. They read the legacy RSS top-100 JSON, which has a hard cap of 100, so ranks 101-200 are invisible. They rerun an npm app-store-scraper script whose reviews, privacy, versionHistory and similar calls broke upstream (#203, #223, #227, #228). They cannot answer "did yesterday's release or subtitle change move my rank, my new ratings or my review stars, and in which storefronts?"

*Weekly ritual:* After each release they check the category rank in each market, read the new version's reviews country by country, and watch the ratings histogram for each storefront.

*Frustration:* Apple shows only the current state. A before/after comparison around a release needs a paid seat or a spreadsheet kept by hand.

**P2. The ASO marketer at a small studio or agency who runs a Monday keyword report for several client apps.**

*Today:* They pay for AppTweak or MobileAction (the $12-$600/month band), or try keyless MCPs. The most-starred one, appreply-co (78 stars), fakes keyword scores. drewster99 and aso-kit return true store order, and aso-kit also keeps a SQLite history. The Search API throttles at about 20 calls/min and returns 403/429 above that, so a basket of terms runs slowly. They cannot answer "who owns the top results across my whole keyword set, and is that share moving?"

*Weekly ritual:* They rerun the term basket for each storefront, note who moved, and check competitors' title, subtitle and screenshot changes. Then they ship a metadata tweak and report the next week whether it worked.

*Frustration:* Share of the top results across the basket, and whether last week's change worked, both need a paid tool or hours of stitching results one term at a time.

**P3. The competitive-intelligence analyst at a mobile game company who scans one genre across several storefronts every Monday.**

*Today:* They use a Sensor Tower seat ($29.5k-$115k a year) when the budget allows. Otherwise they read RSS charts per storefront (100 rows; top-grossing exists only there), keep screenshots in a chat channel, and notice competitor updates by chance. Sensor Tower's Update Timeline and Featured Today views are behind its paywall. They cannot answer "what did every competitor change this week?" or "is this climber rising in many markets or just one?"

*Weekly ritual:* In each storefront they check who climbed or entered the top 200 in the genre and who got featured. They also check what competitors shipped: versions and their notes, IAP prices, in-app events and privacy labels.

*Frustration:* No single view shows every competitor change across apps and storefronts. Telling a multi-market breakout apart from noise in one storefront means stitching per-country charts together by hand.

**P4. The investor or researcher who needs per-country demand over time for apps they do not own.** This is also the owner's own request in this run.

*Today:* Apple publishes no download, install or revenue numbers at any granularity. Sensor Tower sells estimates at $29.5k-$115k a year. App Store Connect sales reports exist only for owned apps, behind an API key. They read rating counts off app pages one storefront at a time, and a version release can reset those counts. They cannot answer "how is app X doing by country, daily, monthly, annually and all-time?"

*Weekly ritual:* They refresh a watchlist tracker of demand by country and rank in key markets, and build monthly and annual rollups for memos or models.

*Frustration:* There is no free per-country time series of anything related to demand. Rating counts exist only as a value for today.

## Candidates (pre-cut)

Active sources: (a), (b), (c), (e) because the brief has `## User Vision`, and (f) because it has `## Codebase Intelligence`. Source (d) does not apply because this is a first print.

**C1. Traction by storefront** · `traction <app>... --countries us,jp --period daily|monthly|annual|all-time [--since 90d]`
- Description: Public demand proxies for each storefront, labelled as proxies and never as downloads, rolled up daily, monthly, annual or all-time from local history. The proxies are:
  - new ratings, from changes in the rating count, with reset detection
  - the rating count since the last observed reset
  - best chart rank and days charted
  - captured review volume, flagged when the RSS cap of 500 reviews truncates it
- Persona: P4 (primary), P1.
- Source: (e) owner request, 2026-10-23 17:30 PKT. (f) Lookup counts are CDN-cached for 86400 s, so one row per storefront per day is the finest period the data supports.
- Long Description (draft): Use this when asked for downloads, installs or demand by country over time: it returns labelled public proxies per storefront, never download counts. Do NOT use this for one app's live per-storefront snapshot today; use 'markets' instead, and to rank a whole chart by new ratings use 'gainers'.
- Rubric:
  - LLM dependency: none. External service: none. Auth: none.
  - Scope: one rollup command (~150 LoC).
  - Verifiability: testable with fixture snapshots that include a reset.
  - Reimplementation: no. It is a local-data command, and Apple has no aggregation endpoint.
  - Verdict: **KEEP.**

**C2. Release and change impact** · `impact <app> --at latest|version:<v>|listing|<YYYY-MM-DD> --window 7d [--countries ...]`
- Description: Takes one event (a version release, a listing change or a date) and compares equal windows before and after it, per storefront, on four metrics: best chart rank, new ratings per day, average stars of reviews dated in the window, and tracked keyword positions.
- Persona: P1, P2.
- Source: (a) the P1 and P2 frustrations. (c) joins `version_entry` dates and `app_snapshot` change times with `chart_snapshot`, `ratings_snapshot`, `review` and `keyword_rank`.
- Long Description (draft): Use this to measure how rank, new ratings, review stars and tracked keyword positions moved before and after a release, a listing change or a date. Do NOT use this for the effect of an editorial placement; use 'feature-lift' instead, and to see what changed use 'changes'.
- Rubric:
  - LLM, external service, auth: none.
  - Scope: ~150 LoC.
  - Verifiability: flagged, because attribution to the event cannot be verified. Reframed to report co-movement only: before, after, delta and point counts for each metric, `insufficient_data` when a window has fewer than two captures, and no causal wording.
  - Reimplementation: no.
  - Verdict: **KEEP (reframed).**

**C3. Cross-app change feed** · `changes --since 7d [--apps ...] [--kind version,title,subtitle,price,iap,privacy,event,featured]`
- Description: One dated, typed row for each change across every tracked app inside the window. Change kinds: new versions with their notes, title and subtitle edits, price and IAP price changes, privacy-label changes, new in-app events, and Editor's Choice or featuring gained or lost.
- Persona: P3, P2.
- Source: (c) a union of diffs between consecutive snapshots across six stored tables. (f) The app page is cached for 900 s and Lookup for 86400 s, so captures from the page see a release the same day.
- Long Description (draft): Use this for a dated feed of changes across many tracked apps. Do NOT use this for a calendar of upcoming in-app events; use 'events-calendar' instead, and for storefront availability changes use 'rollout'.
- Rubric: all checks pass. Scope is ~150 LoC, reusing watch-listing's diff function. **KEEP.**

**C4. Keyword share of voice** · `keyword-share [term...] --country us --top 10`
- Description: Covers a basket of tracked terms, or every tracked term in the storefront when none are given. For each app it counts the terms the app holds in the top N, its average and best position, and the change since the previous capture. Counts are unweighted, because Apple publishes no search volume outside Search Ads.
- Persona: P2.
- Source: (a) the P2 frustration. (c) aggregates the stored top-N result lists across terms.
- Long Description (draft): Use this to see which apps own the top results across tracked terms. Do NOT use this to list only the terms a competitor holds and the target app lacks; use 'keyword-gap' instead.
- Rubric: all checks pass, with no invented popularity. **KEEP.**

**C5. Multi-storefront breakouts** · `breakouts --genre <id> --chart top-free --platform iphone --since 7d --min-markets 3 [--min-gain 10]`
- Description: Lists apps that entered or climbed one chart in at least N storefronts inside the window, ranked by storefront count and then by median rank gain. The meta block reports which storefronts were compared and which were skipped.
- Persona: P3, P4.
- Source: (c) a cross-storefront join over `chart_snapshot`.
- Long Description (draft): Use this to find apps climbing one chart in several storefronts at once. Do NOT use this to rank a chart by new ratings per day; use 'gainers' instead, and for one storefront's diff use 'movers'.
- Rubric: all checks pass. **KEEP.**

**C6. Rating-velocity leaderboard** · `gainers --genre <id> --country jp --chart top-free --since 7d`
- Description: Ranks the 200 apps in a captured chart by new ratings per day. The rating counts are already on the chart page: its 25 lockups carry `ratingCount`, and its one batched Lookup for the other 175 ids returns `userRatingCount`.
- Persona: P4, P3.
- Source: (f) the chart page hydrates 175 ids with one batched Lookup. (c).
- Long Description (draft): Use this to rank one captured chart's apps by new ratings per day. Do NOT use this for one app's per-country proxies; use 'traction' instead, and for rank changes use 'movers'.
- Rubric: verifiability flagged, because rating velocity compared across apps depends on each app's rating-prompt policy. **KEEP to Pass 3 (low confidence).**

**C7. Live market footprint** · `markets <app> --countries us,gb,jp`
- Description: One row per storefront for one app today. Each row has availability, price, rating average and count, and the chart-position badge (chart, genre, position) and Editor's Choice flag from the app page, plus deltas against the last stored snapshot.
- Persona: P1.
- Source: (a) Top Workflow 1, "across my markets". (f) the chart-position badge in the page model, available per storefront.
- Long Description (draft): Use this for one app's live per-storefront snapshot today. Do NOT use this for demand over time; use 'traction' instead, and for prices alone use 'prices'.
- Rubric: all checks pass. The live fan-out costs one app page per storefront, about 175 for all storefronts. **KEEP to Pass 3.**

**C8. Keyword gap** · `keyword-gap <app> --vs <competitor,...> --country us --top 10`
- Description: Lists tracked terms where named competitors hold top-N positions and the target app does not, plus the terms only the target holds.
- Persona: P2.
- Source: (c).
- Long Description (draft): Use this to list terms where competitors outrank the target app. Do NOT use this for overall share across terms; use 'keyword-share' instead.
- Rubric: all checks pass. **KEEP to Pass 3.**

**C9. LiveOps events calendar** · `events-calendar [--apps ...] --days 14`
- Description: Upcoming and running in-app events across tracked apps, ordered by start date, with event kind and end date.
- Persona: P3.
- Source: (b) In-App Events are an App Store content type with their own `appEvents` shelf on the app page.
- Long Description (draft): Use this for a dated calendar of in-app events across tracked apps. Do NOT use this for one app's current events; use 'events' instead.
- Rubric: all checks pass. **KEEP to Pass 3.**

**C10. Storefront rollout tracker** · `rollout [--apps ...] --since 30d`
- Description: Storefronts where tracked apps became available or disappeared between captures, such as soft launches and regional pulls.
- Persona: P3.
- Source: (b) per-storefront availability across about 175 storefronts.
- Long Description (draft): Use this to see where tracked apps launched or were pulled. Do NOT use this for today's price and availability table; use 'prices' instead.
- Rubric: needs `prices` results to be stored, a minor data-model addition. **KEEP to Pass 3.**

**C11. Category privacy scan** · `privacy-scan --genre <id> --country us --top 50`
- Description: Tabulates the App Store privacy-label categories (Data Used to Track You, Data Linked to You, Data Not Linked to You) across a chart's top apps.
- Persona: P4 (researchers).
- Source: (b) the App Store's privacy-label taxonomy. (c).
- Long Description (draft): Use this to compare privacy labels across a chart's apps. Do NOT use this for one app's label; use 'privacy' instead.
- Rubric: scope flagged, because the live form fetches one app page per charted app on every run. Reframed to a local join of the latest chart snapshot with stored privacy snapshots. **KEEP (reframed) to Pass 3.**

**C12. Keyword ideas from competitor metadata** · `keyword-ideas <app> --competitors <id,...> --country us`
- Description: Mechanical counts of one- to three-word phrases in stored competitor titles and subtitles (the visible indexed fields), marking phrases that are not yet tracked.
- Persona: P2.
- Source: (b) the App Store indexes title and subtitle; the keyword field is private.
- Long Description (draft): Use this to list candidate terms from competitors' titles and subtitles. Do NOT use this for live autocomplete hints; use 'suggest' instead.
- Rubric:
  - LLM dependency: flagged, then reframed to n-gram counts.
  - The A-Z search-hints expansion variant is cut inline: it needs 26 or more requests per seed to the robots-disallowed, undocumented `/WebObjects` hints endpoint.
  - Verdict: **KEEP (reframed) to Pass 3.**

**C13. Watchlist capture run** · `capture` (with `capture add|list|remove`)
- Description: Saves apps, competitors, storefronts, genres and terms, then runs one paced pass of every capture so daily history builds up from a single cron line.
- Persona: P1 to P4.
- Source: (a). (e) daily, monthly and annual rollups need daily captures.
- Long Description: none.
- Rubric: scope flagged, because a saved config plus orchestration is shaped like an application. Descoped to one run command. **KEEP to Pass 3.**

**C14. Rank-to-downloads estimate** · `estimate-downloads <app> --countries ...`
- Description: Converts chart rank into estimated daily downloads per storefront using a power-law curve.
- Persona: P4.
- Source: (e) the owner's downloads request.
- Long Description: none.
- Rubric: fails Reimplementation, because it presents synthesized numbers as data. It is also out of scope: Apple publishes no download numbers, the brief lists download estimates as not in scope, and the google-play sibling refuses them. **CUT.**

**C15. App Store Connect downloads** · `asc-downloads <app> --countries ... --period monthly`
- Description: Real per-country units from App Store Connect sales reports, for apps the user owns.
- Persona: P1 (own apps); P4 only when they hold the account.
- Source: (e) the brief names this as an explicit option at the absorb gate, not a default.
- Long Description: none.
- Rubric: fails "auth the user doesn't have", because it needs an App Store Connect API key and works only for owned apps. The library's asc CLI already covers App Store Connect. **CUT (the owner may override at the absorb gate).**

**C16. Editorial lift** · `feature-lift <app> --placement today|featured-in|editors-choice --window 7d`
- Description: Rank and new-rating movement in the days after an app appears in stored Today or Featured snapshots or gains Editor's Choice.
- Persona: P1, P3.
- Source: (c) joins `featured` snapshots with `chart_snapshot` and `ratings_snapshot`.
- Long Description (draft): Use this to measure movement after an editorial placement. Do NOT use this for release effects; use 'impact' instead.
- Rubric: all checks pass. **KEEP to Pass 3.**

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona Served | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|----------------|--------------|--------------|----------|------------------|
| 1 | Traction by storefront (labelled public demand proxies) | `traction <app>... --countries us,jp --period monthly` | 10/10 | P4 investor/researcher (owner's request); P1 for own markets | hand-code | This uses local `ratings_snapshot` rows from `ratings --countries` captures, `chart_snapshot` ranks from `top` and the `review` table to compute, per storefront, new ratings (resets detected, never negative), the rating count since the last observed reset, best rank, days charted and captured review volume, rolled up daily, monthly, annual or all-time, with no external dependencies. | Brief User Vision: owner request 2026-10-23 17:30 PKT for "downloads by country on a daily, monthly, annual and all time basis", to be served by labelled proxies (Lookup userRatingCount per country, resettable per version; new ratings per day; chart rank; review volume). Brief Users: Sensor Tower at $29.5k-$115k a year. Apple publishes no download numbers, and the google-play SKILL refuses download estimates. | Use this when asked for downloads, installs or demand by country over time: it returns labelled public proxies per storefront (new ratings, chart rank, captured review volume) rolled up daily, monthly, annual or all-time, never download counts. Do NOT use this for the current star histogram in each storefront; use 'ratings' instead, and for one chart's rank trajectory use 'rank-history'. Real per-country downloads exist only in App Store Connect sales reports for apps you own, which the separate asc CLI reads. |
| 2 | Release and change impact | `impact <app> --at latest --window 7d` | 9/10 | P1 indie developer; P2 ASO marketer | hand-code | This uses local `version_entry` release dates, `app_snapshot` content-hash change times and `featured` snapshots (or a given date) as the event, then reads `chart_snapshot`, `ratings_snapshot`, `review` and `keyword_rank` rows in equal windows before and after it to compute per-storefront deltas in best rank, new ratings per day, average review stars and tracked keyword positions, with no external dependencies. | Brief Top Workflows 1-3 ("how did that move this week", competitor What's New, complaints about the "latest version"). Manifest #29 cites Sensor Tower Update Timeline (paid). The google-play sibling's keyword-history exists "to prove whether a listing change actually moved a keyword over time". The brief's Data Layer already stores version dates and content-hash listing diffs. | Use this to measure how rank, new ratings, review stars and tracked keyword positions moved in equal windows before and after a release, a listing change, an editorial placement or a given date. Do NOT use this to see what changed; use 'watch-listing' for one app's field diff or 'changes' for a feed across apps, and for complaint terms by version use 'review-digest'. |
| 3 | Cross-app change feed | `changes --since 7d --apps <id,...>` | 9/10 | P3 game CI analyst; P2 ASO marketer | hand-code | This uses consecutive local snapshots in `app_snapshot` (title, subtitle, price, version, screenshots, in-app events, Editor's Choice), `version_entry`, `iap_price`, `privacy_snapshot` and `featured` across every tracked app to emit one dated, typed row per change inside the window, with no external dependencies. | Brief Top Workflow 2 (version, What's New, price, IAP prices, subtitle, privacy labels, screenshots). Manifest #29 cites Sensor Tower Update Timeline (paid). app-store-scraper's versionHistory and privacy are broken upstream (#223, #228). Codebase Intelligence: the app page is cached 900 s against Lookup's 86400 s, so page captures see releases the same day. | Use this for a dated feed of every change across your tracked apps inside a window: new versions and their notes, title and subtitle edits, price and in-app purchase price changes, privacy-label changes, new in-app events, and featuring or Editor's Choice gained or lost. Do NOT use this for a field-by-field diff of one app's two latest snapshots; use 'watch-listing' instead, and to measure whether a change moved rank or ratings use 'impact'. |
| 4 | Keyword share of voice | `keyword-share [term...] --country us --top 10 --app <id>` | 9/10 | P2 ASO marketer | hand-code | This uses local `keyword_rank` rows (term, storefront, platform, captured_at and the stored top-N result list) to compute, per app, how many tracked terms it holds in the top N, its average and best position, and the change since the previous capture, with no external dependencies. | Brief Top Workflow 4 ("who owns the top results"). The brief's Data Layer keyword rank row keeps the top-N SERP. Market sweep: AppTweak and MobileAction sit in the $12-$600/month band. MCP sweep: appreply-co (78 stars) fakes keyword scores, and aso-kit keeps a SQLite history. | Use this to see which apps hold the top search results across a set of tracked terms and how that share moved since the previous capture. Do NOT use this for one app's position on one term over time; use 'keyword-history' instead, and to capture a fresh result list use 'keyword-rank'. |
| 5 | Multi-storefront breakouts | `breakouts --genre 6014 --chart top-free --since 7d --min-markets 3` | 7/10 | P3 game CI analyst; P4 investor | hand-code | This uses local `chart_snapshot` rows for one platform, genre and chart across every captured storefront to compute, per app, the number of storefronts where it entered or gained at least --min-gain ranks inside the window and its median gain, with no external dependencies. | Brief Top Workflow 5 ("What is climbing in Games > Puzzle in Japan; who entered the top 200"). Live probe ledger: one chart page returns 200 ranks for a storefront, so each storefront captured costs one request. Manifest #25 cites AppTweak movers (paid). | Use this to find apps climbing or entering one chart in several storefronts at once over a window. Do NOT use this for one storefront's chart diff; use 'movers' instead, and for one app's rank trajectory use 'rank-history'. |

**traction** (10/10: Domain Fit 3, User Pain 3, Build Feasibility 2, Research Backing 2)
1. Weekly use: Yes. P4 refreshes a watchlist tracker weekly and rolls up monthly and annually. The owner asked for daily rows, and P1 checks their markets after each release.
2. Wrapper vs leverage: Leverage. No Apple endpoint returns rating counts over time. Traction joins three stored streams per storefront and handles version resets, so new-rating counts never go negative.
3. Transcendence proof: Local SQLite history, plus agent-shaped output. The meta block states `proxy: true`, `download_counts: false`, the first capture date and the number of storefronts with data.
4. Sibling kill: markets. Its per-storefront "today" view is traction's latest daily row. Nearby kills: gainers, estimate-downloads, asc-downloads.
5. Buildability: hand-code. About 150 LoC plus root.go wiring.
   - Annotation: `// pp:data-source local`, rejecting `--data-source live` with a no-live-equivalent error.
   - Sync hints: `hintIfUnsynced(cmd, db, "ratings_snapshot")`, then `hintIfStale`.
   - Drain-first: scan the ratings rows into a slice, check `rows.Err()`, close, then run the per-storefront chart and review queries.
   - Read-only, so no upserts.
   - Daily is the finest period, because Lookup counts are CDN-cached for 86400 s.
6. Long-description validity: The draft named 'markets' and 'gainers', both killed. The final text points at 'ratings' (manifest #10) and 'rank-history' (manifest #26), both absorbed and built. It names the separate asc CLI only as another tool.

**impact** (9/10: Domain Fit 3, User Pain 2, Build Feasibility 2, Research Backing 2)
1. Weekly use: Yes. P2 ships a metadata change and reads its effect the following Monday. P1 runs it after each release.
2. Wrapper vs leverage: Leverage. It joins an event (version date, listing-hash change time, featuring date or a given date) with four metric tables in equal windows. No endpoint returns a before/after view.
3. Transcendence proof: Local SQLite cross-entity join.
4. Sibling kill: feature-lift. An editorial placement is just one more event source here (`--at featured`).
5. Buildability: hand-code. About 150 LoC.
   - Annotation: `// pp:data-source local`.
   - Sync hints: `hintIfUnsynced(cmd, db, "")`, because it reads several resources.
   - Drain-first: resolve the event and close, then run one query per metric window.
   - Output: each metric reports before, after, delta and point counts, with `insufficient_data` when a window has fewer than two captures, and no causal wording.
   - `--at` accepts latest, version:<v>, listing, featured or a date.
6. Long-description validity: The draft named 'feature-lift' (killed and folded in) and 'changes' (survives). The final text points at 'watch-listing' (manifest #29), 'changes' (survivor) and 'review-digest' (manifest #30).

**changes** (9/10: Domain Fit 3, User Pain 2, Build Feasibility 2, Research Backing 2)
1. Weekly use: Yes. It is P3's Monday competitor review and P2's weekly client report.
2. Wrapper vs leverage: Leverage. It unions diffs between consecutive snapshots from six tables across many apps into one feed ordered by time. Any single endpoint shows one app as it is now.
3. Transcendence proof: Local SQLite history, plus App Store-specific change kinds (privacy labels, IAP prices, in-app events, Editor's Choice).
4. Sibling kill: events-calendar and rollout, whose rows are change kinds in this feed. privacy-scan as well.
5. Buildability: hand-code. About 150 LoC, reusing watch-listing's diff function for each consecutive pair.
   - Annotation: `// pp:data-source local`.
   - Sync hints: `hintIfUnsynced(cmd, db, "")`.
   - Drain-first, one source table at a time: load, close, diff in Go, merge by timestamp.
   - Availability changes are not part of this plan; they can join once `prices` results are stored.
6. Long-description validity: The draft named 'events-calendar' and 'rollout', both killed. The final text points at 'watch-listing' (manifest #29) and 'impact' (survivor).

**keyword-share** (9/10: Domain Fit 3, User Pain 2, Build Feasibility 2, Research Backing 2)
1. Weekly use: Yes. It is P2's Monday keyword report.
2. Wrapper vs leverage: Leverage. A live search answers one term. This aggregates the stored top-N lists of every tracked term and compares them with the previous capture.
3. Transcendence proof: Local SQLite SERP history, with plain counts in place of the invented scores that some keyless MCPs report.
4. Sibling kill: keyword-gap. Its job is now `--app <id>`, which marks the target app and adds the terms where competitors hold the top N and the target does not.
5. Buildability: hand-code. About 100 LoC.
   - Annotation: `// pp:data-source local`.
   - Sync hints: `hintIfUnsynced(cmd, db, "keyword_rank")`.
   - Drain-first, with the aggregation done in Go.
   - The meta block lists the terms counted and each term's last capture time.
6. Long-description validity: The draft named 'keyword-gap', which was killed. The final text points at 'keyword-history' (manifest #28) and 'keyword-rank' (manifest #27).

**breakouts** (7/10: Domain Fit 2, User Pain 2, Build Feasibility 2, Research Backing 1)
1. Weekly use: Yes. It is P3's Monday market scan and P4's weekly check.
2. Wrapper vs leverage: Leverage. One chart page covers one storefront. This aggregates rank changes for the same chart across every captured storefront.
3. Transcendence proof: Local SQLite cross-storefront join.
4. Sibling kill: gainers. Comparing rating velocity across different apps is unreliable, while rank is a fair proxy for comparing apps.
5. Buildability: hand-code. About 100 LoC.
   - Annotation: `// pp:data-source local`.
   - Sync hints: `hintIfUnsynced(cmd, db, "chart_snapshot")`.
   - Drain-first.
   - The meta block reports how many storefronts were compared and which were skipped for having fewer than two captures in the window.
   - Top-grossing has only 100 rows (RSS), so `--chart top-grossing` compares 100 ranks.
6. Long-description validity: The draft named 'gainers' (killed) and 'movers'. The final text points at 'movers' (manifest #25) and 'rank-history' (manifest #26).

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Rating-velocity leaderboard (`gainers`) | How many new ratings an app gets depends on how often it shows the in-app rating prompt, so ranking different apps by it would mislead; the within-app series belongs in traction, and rank is the fair proxy for comparing apps. | traction |
| Live market footprint (`markets`) | Its per-storefront view duplicates traction's latest daily row, and its one new field, the chart-position badge, sits on the same app page that `ratings --countries` already fetches. | traction |
| Keyword gap (`keyword-gap`) | Its output is a filtered view of keyword-share rows, now served by `keyword-share --app <id>`. | keyword-share |
| LiveOps events calendar (`events-calendar`) | No research finding asks for an events calendar (discovery skipped in-app events), and new or changed events already appear as dated rows in changes while one app's current events come from `events`. | changes |
| Storefront rollout tracker (`rollout`) | Availability changes are rare, have no stated demand in the brief, and need `prices` results stored first, after which they become one more change kind in changes. | changes |
| Category privacy scan (`privacy-scan`) | Researchers run a category privacy census once per study, monthly or quarterly rather than weekly, and label changes on tracked apps already surface in changes. | changes |
| Keyword ideas (`keyword-ideas`) | Keyword research is a monthly step while tracking is weekly, and its A-Z expansion variant would hit the robots-disallowed, undocumented hints endpoint dozens of times per seed. | keyword-share |
| Watchlist capture run (`capture`) | It is orchestration with no new computation, since a cron of the absorbed capture commands (`top`, `ratings --countries`, `reviews --countries`, `keyword-rank`) with saved profiles builds the same history. | traction |
| Editorial lift (`feature-lift`) | Editorial placements are irregular, so it fails the weekly-use test on its own, and a placement is one more event source for `impact --at featured`. | impact |
| Rank-to-downloads estimate (`estimate-downloads`) | It would invent download numbers Apple does not publish, which the brief puts out of scope and the google-play sibling refuses. | traction |
| App Store Connect downloads (`asc-downloads`) | It needs an App Store Connect API key and works only for owned apps, which the library's asc CLI already reads, so it stays an owner opt-in at the absorb gate rather than a default. | traction |
