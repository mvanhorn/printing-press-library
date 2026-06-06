# Atlas Obscura Browser-Sniff Report

Goal: confirm whether AO search exposes (a) a category-facet param and (b) a native geocoder.

## Method
Drove Chrome (claude-in-chrome) to `/search?q=cemetery`, opened the global search form,
typed "Kyoto", read the search UI + captured XHR network requests.

## Findings
1. **No category facet.** The search results UI has NO category/tag filter controls. Default
   view is "Place Results" (`kind=place`); "Or, search keyword" links to `kind=keyword`. There
   is no facet param to send. => `near --category` must filter client-side via place-page tags.
2. **No usable geocoder / autocomplete.** Typing "Kyoto" showed only a STATIC dropdown
   ("Places near me", "Random place", popular destinations Paris/London/...). The only suggest
   XHR fired was `GET /search/combined?q=Kyoto`, which returns **HTTP 500** server-side for every
   variation tried (q=, query=, .json, +kind). Unusable. => geocode place names via Open-Meteo.
3. **`kind` param is the real search-mode switch (NEW).**
   - `GET /search?q=<q>&kind=keyword` => proper relevance-ranked text search (15/page).
     e.g. q=lighthouse => "Frozen Cleveland Lighthouse" first. **Use this for `search`.**
   - `kind=place` / default => place-name-geocode mode; returns a generic fallback
     ("Wacky Woods") when no geocode match. Not what we want for keyword search.
4. **Geo path confirmed.** `GET /search?lat=&lng=` => distance-sorted results
   (`distance_from_query` in miles); `q` ignored when lat/lng present.
5. Map tiles render via Google Maps with AO's embedded Google key — render-only, not a geocoder.
6. `/search/search_nearby` is the browser-geolocation entry (HTML), not a CLI-usable endpoint.

## Replayable surfaces (final contract)
- search:  GET /search?q=<q>&kind=keyword&page=<n>   (Accept: application/json, X-Requested-With: XMLHttpRequest)
- near:    GET /search?lat=<lat>&lng=<lng>&page=<n>   (same headers); --radius + --category client-side
- show:    GET /places/<slug-or-id>                   (HTML + JSON-LD Place)
- browse:  GET /categories/<slug>  ,  GET /things-to-do/<place-slug>   (HTML place-link lists)
- geocode: Open-Meteo geocoding-api.open-meteo.com/v1/search (no auth) — secondary source

## Runtime
- Transport: standard_http (probe-reachability => standard_http; Cloudflare present, no challenge).
- No auth, no clearance cookie, no resident browser. All commands replay over plain HTTP.
