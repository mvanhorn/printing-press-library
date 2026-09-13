# Immoweb CLI — Absorb Manifest

Sources: lawrensylvan/immoweb-keeper, othellodesutter/immoweb-telegram-bot, vincentcox/immoweb-zimmo-scraper, adamkorompai/immoweb_bot, simon324/belgium-rental-watcher, pierodellagiustina/immoweb-be-scraper, g-coomans/ImmoWeb, ptrues antwerp-rentals, OxyHQ/Homiio, Apify actors (azzouzana, ivanvs, memo23, haketa), lobstr.io, ImmowebFilter + immoweb-chromium-extension. No Immoweb CLI or MCP server exists.

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search listings by criteria (type, sale/rent, postcodes, price, bedrooms, sort, page) | All scrapers; Apify haketa inputs; othellodesutter bot | immoweb-pp-cli find | Friendly flags (--type house --deal rent --commune ixelles), commune names resolved to postcodes, compact rows, --json/--csv, results upserted to local store |
| 2 | Raw search endpoint with every site filter (surfaces, EPC, garden, pool, construction year, public sale, life annuity) | Immoweb front-end criteria; pierodellagiustina ranges | (generated endpoint) listings search | Every upstream filter as a typed flag; exact wire names kept |
| 3 | Search from a pasted Immoweb search URL | keeper, Apify azzouzana/memo23 startUrls, lobstr | (behavior in immoweb-pp-cli find) --url parses fr/nl/en search URLs into criteria | Works with FR/NL/EN SEO URLs, no browser |
| 4 | Multi-page crawl with item cap | Apify maxItems/maxPages | (behavior in immoweb-pp-cli find) --pages / --limit | Adaptive rate limit; soft-throttle (200 + empty page) surfaced as typed error |
| 5 | Exact result count | Immoweb count widget | (generated endpoint) listings count | Uncapped count (search caps at 9,969) |
| 6 | Map results with GPS (200 per call) | Immoweb map view; ptrues GeoJSON | (generated endpoint) listings map | Bulk path, 7x lighter per listing than search pages |
| 7 | Full listing details (EPC, cadastral income, surfaces, heating, agency, photos, views/bookmarks) | Homiio get-result; Apify ivanvs/memo23 | (generated endpoint) listings get | Full JSON by ID; --select for agent-sized output |
| 8 | Human listing card (flattened key facts, contact, URL) | lobstr 24 detail fields; bootcamp scrapers | immoweb-pp-cli show | One readable card; accepts ID or any immoweb URL |
| 9 | Commune / postcode lookup | Immoweb autocomplete; haketa postcode→province | (generated endpoint) locations lookup | Returns queryValue ready for filters |
| 10 | Similar listings | Immoweb "similar" block | (generated endpoint) listings similar | JSON instead of HTML-entity blob |
| 11 | Saved searches | keeper saved searches; adamkorompai 38 searches | immoweb-pp-cli saved add | Named local searches, no Immoweb login needed |
| 12 | List / remove saved searches | keeper | immoweb-pp-cli saved list | Local, scriptable |
| 13 | New-listing alerts on a saved search | telegram bot, vincentcox Pushover, Apify monitoring new/delisted | immoweb-pp-cli watch | Diff vs last run (new / price change / gone) as JSON; pipe to any notifier; silent first run |
| 14 | Disappearance tracking (lastSeen / is-it-gone) | keeper, g-coomans | (behavior in immoweb-pp-cli watch) gone listings flagged with last_seen | Kept in local history |
| 15 | Dedupe across overlapping searches | othellodesutter | (behavior in immoweb-pp-cli watch) listings keyed by ID in store | One row per listing across all saved searches |
| 16 | Photo download | keeper | immoweb-pp-cli photos | Chosen size (small→2560px), --dir |
| 17 | Private-owner-only filter | adamkorompai, prospector | (behavior in immoweb-pp-cli find) --private-only | Uses missing agency logo signal |
| 18 | Hide under-option / sold listings | ImmowebFilter extension | (behavior in immoweb-pp-cli find) --hide-under-option | Uses flags.main=under_option |
| 19 | Dismiss / hide seen listings | immoweb-chromium-extension | immoweb-pp-cli hide | Local hidden list respected by find and watch |
| 20 | Liked / shortlisted listings | keeper liked/visited | immoweb-pp-cli shortlist add | Local shortlist with notes, no login |
| 21 | Flattened CSV / GeoJSON / JSONL export | every scraper (CSV), ptrues (GeoJSON) | immoweb-pp-cli dump | --format csv, geojson or jsonl from the local store (framework `export` is reserved) |
| 22 | Distance-to-point filter | ptrues (distance to tram stops) | (behavior in immoweb-pp-cli find) --near lat,lng --radius-km | Haversine on listing GPS |
| 23 | Bulk-harvest an area, beating the 30/page and 9,969 caps | pierodellagiustina, scrapers split by postcode | immoweb-pp-cli pull | Map endpoint (200/call), count-planned auto-split by postcode, everything lands in the local store |
| 24 | Offline full-text search over synced listings | keeper UI filters | (behavior in immoweb-pp-cli search) FTS over title, description, locality, agency | Offline, SQL composable |

### Not absorbed (out of scope, stated at the gate)
- Multi-site (Zimmo, Immovlan, Realo) and Biddit auctions — different sites; scope creep.
- Telegram / Pushover / email push — external services; `watch --json` output pipes into any notifier.
- LLM relevance scoring (rental-watcher) — LLM dependency; replaced by deterministic `triage`.
- Favourite / saved-search sync with the Immoweb account, contact-agent email — auth-only or write actions; client must have a zero-login experience.

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| 1 | Commune market snapshot (+ multi-commune compare) | market | 10/10 | hand-code | Live exact count + map-endpoint fill of the local store, then medians of price, €/m², days listed and 30-day disappearances per type × bedroom (or EPC) band; replaces the dead price.immoweb.be | Use this command for commune-level statistics (count, median price, €/m², days listed, disappearances) and side-by-side commune comparison. Do NOT use this command to judge one specific listing; use 'deal' instead. Do NOT use it for rental return; use 'yield' instead. |
| 2 | Listing deal verdict | deal | 9/10 | hand-code | Detail call + local comparables (gone rows included) + own price observations → €/m² percentile, days on market, cut history, bookmarks/day, EPC gap | Use this command to judge whether one listing's price is fair (€/m² vs comparables, days on market, price cuts, demand). Do NOT use this command for commune-wide statistics; use 'market' instead. Do NOT use it to estimate rental return; use 'yield' instead. |
| 3 | Saved-search triage | triage | 9/10 | hand-code | Saved-search criteria × hidden/seen tables × commune medians × price history → deterministic per-factor score (price percentile, freshness, cut, private seller, EPC) | Use this command to decide which current matches of a saved search to contact first. Do NOT use this command for what changed since the last run; use 'watch' instead. Do NOT use it for an in-depth look at a single listing; use 'deal' instead. |
| 4 | Price-cut leaderboard | drops | 8/10 | hand-code | Local price-observation time series: first vs latest price, cut count, cumulative % cut, days listed — the old price Immoweb's new_price badge hides | Use this command to rank price cuts across all stored listings over a time window. Do NOT use this command for what changed in one saved search since its last run; use 'watch' instead. |
| 5 | Rental yield estimator | yield | 7/10 | hand-code | Cross-transaction join (FOR_RENT × FOR_SALE) by postcode/type/bedrooms, gross yield with comparable count, refusal when n < 5 | Use this command to estimate the gross rental yield of a sale listing or a commune. Do NOT use this command to judge whether a price is fair; use 'deal' instead. Do NOT use it for general commune statistics; use 'market' instead. |
