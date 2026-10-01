# Weathernews CLI research brief

Provider: https://weathernews.jp/ and its first-party site.weathernews.jp assets only. Anonymous public HTTP pages returned 200 on 2026-10-01 JST after sandbox DNS failures (000) were resolved by authorized network execution. No credentials or paid account required for selected scope. Public website APIs are undocumented and may change; fail closed on schema drift. Commercial WxTech uses a separate API key/trial and is outside this website-focused scope: https://wxtech.weathernews.com/products/data/api/en/mcp/ . Member products remain explicit, with no session capture or paid account assumption.

Travel workflows: resolve Japanese cities/landmarks; retrieve bounded hourly and daily forecasts; search sakura/koyo inventories; inspect lazy spot evidence with current/forecast/normal separation; compare up to five exact places/dates against caller thresholds. No invented score. Unknown, stale, ended, unavailable and outside-horizon states must be machine-readable.

Source evidence: homepage client references /wnl/search/api_search.cgi and /lba/wxdata/api_data_ss1. Seasonal Nuxt server payloads carry inventory; sakura homepage says 2026 updates ended, map date May 23; koyo homepage says September 30 current status. Published WxTech reference describes 1km forecast grids (not assumed for website without website evidence). Units from homepage: temperature C, precipitation mm, wind m/s, daily rain probability percent.

Competitors/wrappers: rnikko/weathernews-python offers Location.search and Forecast.fetch; its issue listing examined for breakage. A standalone Weathernews gist uses the same forecast surface but substitutes Nominatim geocoding, which we exclude. Broader official WxTech/MCP features are paid/authenticated and outside agreed focused provider scope. Search found no public consumer-site npm/MCP equivalent. Seasonal comparisons and provenance are higher-value gaps.

Data layer: bounded disk HTTP cache with TTL, explicit refresh, source fetch times, no stale fallback; seasonal inventory summary then lazy detail. No resident browser, global config, server, sqlite daemon or unrelated provider.

Product: weathernews-pp-cli, compact Japan weather and seasonal travel evidence. Source IDs, Japanese names, canonical URLs, location/elevation nullable, JST timestamps; issue and valid time with unknowns explicit. Runtime bounded timeouts, retries/pages/output, first-party host allowlist.

Scope preapproved in user instruction. Builder performs all planning/implementation; exactly one independent fresh-context gpt-6.1-sol xhigh reviewer after implementation. No extra brainstorm or output agents.
