# Mobbin content-search migration (captured 2026-07-08, live authed session)

Mobbin renamed content search from `/api/content/search-*` (now 404) to
`/api/search/fetch-search-page-*` with a NEW request/response shape.

## screens search  (CONFIRMED 200, totalCount 861)
POST https://mobbin.com/api/search/fetch-search-page-screens
Request:
{
  "searchRequestId": "<uuid, empty ok for fresh>",
  "pageIndex": 0,
  "searchQuery": {
    "platform": "web|ios|android",
    "type": "filters",                     // filter-mode; "text" for free-text
    "categories": null | ["Business", ...],   // was appCategories (display names)
    "screenElements": null | ["Search Bar"],  // display names
    "screenPatterns": null | ["Notifications"], // display names (NOT slugs)
    "textInScreenshotQuery": null | "cancel anytime",  // was screenKeywords (OCR)
    "hasAnimation": null | true,
    "sortBy": "trending|publishedAt|popularity"
  }
}
Response: {"value":{"searchRequestId":str,"data":[...screens...],"hasNextPage":bool,"totalCount":int}}
Screen row keys: id, screenUrl, createdAt, width, height, screenElements, screenPatterns,
  appId, appName, appLogoUrl, platform, appVersionId, appVersionPublishedAt,
  screenCdnImgSources, fullpageScreenCdnImgSources, animationCdnVideoSources, restricted

## flows search  (CONFIRMED 200, totalCount 1670)
POST https://mobbin.com/api/search/fetch-search-page-flows
Request: same envelope; searchQuery = {platform, type:"filters", categories, flowActions:["Filtering & Sorting"], sortBy}
Response: {"value":{"searchRequestId","data":[...flows...],"hasNextPage","totalCount"}}
Flow row keys: id, name, actions, appVersionId, appId, appName, platform, screens[], videoCdnVideoSources

## apps search (endpoint exists: /api/search/fetch-search-page-apps; payload NOT captured — 400 on categories/appCategories)
Secondary: public `apps list`/`popular`/`discover` already cover app discovery. Leave apps-search
best-effort or map to searchable-apps; do not block on it.

## KEY MAPPING (flat flags -> new searchQuery)
--platform -> searchQuery.platform ; --screen-patterns -> screenPatterns (display names) ;
--screen-elements -> screenElements ; --screen-keywords -> textInScreenshotQuery ;
--app-categories -> categories ; --has-animation -> hasAnimation ; --flow-actions -> flowActions ;
--sort-by -> searchQuery.sortBy ; --page-index -> top-level pageIndex ; type is "filters" ;
NO pageSize (server-fixed ~24). Filter VALUES are display names (e.g. "Paywall","Notifications"),
not slugs — the filters taxonomy `name` field, not `slug`.

## COOKIE BUG
mobbin.com/api endpoints need the FULL raw browser cookie jar (all 15 cookies incl. split
sb-...-auth-token.0/.1 in ORIGINAL split form), NOT the reassembled single token. Direct curl
with the full jar -> 200; CLI sending only the reassembled token -> "unauthenticated".
Supabase RPC/PostgREST endpoints still use the reassembled Bearer + apikey.
