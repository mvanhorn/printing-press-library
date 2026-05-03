# Airbnb Endpoints Captured (live HAR, 2026-05-02, authenticated session)

## Two viable transport paths

### Path A: SSR HTML scraping (openbnb pattern - RECOMMENDED for free path)
- GET `https://www.airbnb.com/s/{slug}/homes` (search results in `<script id="data-deferred-state-0">`)
- GET `https://www.airbnb.com/rooms/{listing_id}` (listing detail in same script tag)
- No auth required. Works against current live site.

### Path B: GraphQL persisted queries (authenticated, richer data)
All under `https://www.airbnb.com/api/v3/{OperationName}/{sha256Hash}` with query params:
- `operationName=<name>`
- `locale=en` `currency=USD`
- `variables=<urlencoded JSON>`
- `extensions={"persistedQuery":{"version":1,"sha256Hash":"<hash>"}}`

Method varies by op (most GET, a few POST).

| Operation | sha256Hash | Method | Purpose |
|---|---|---|---|
| AutoSuggestionsQuery | 149a45101ad29e78ce251ece5fe56311db44d4f0ba537b497e3840c2b2466a5d | GET | Location autocomplete |
| WishlistItemsAsyncQuery | c0f9d9474bb20eb7af2f94f8e022750a5ed9b7437613e1d9aa91aadea87e4467 | GET | Read wishlist items by listingIds (auth) |
| WishlistIndexPageQuery | b8b421d802c399b55fb6ac1111014807a454184ad38f198365beb7836c018c18 | GET | List user's wishlists (auth) |
| MapViewportInfoQuery | aae2b4447f90adfd800a006f1afc80e2df9f98ddc8cd932628da179ebae10c79 | GET | Bounding-box search variant |
| PlacesMapTileLayersQuery | 2801f85616c647da897107111266a31b9eb408f109250b54f442b9c6ca781b93 | GET | Neighborhood/event map tiles |
| StaysPdpSections | f6df77116e38de1d621bf5d92fa626d41da4829a1f09038f4a39f582e90a5cab | POST | Listing detail (PDP) - master query |
| StaysPdpBookItQuery | 5560c774d764520fc721f6dffca10d9cff03b25e9907478ded8530caf679d716 | GET | Booking widget (price, availability for dates) |
| SimilarListingsCarouselQuery | bd4a24e13a5419167c7b9eaddfddcb888f10efeecfb03f49c913cc8e8d38defa | GET | Similar listings recommendation |
| Header | bb590cf8c21b62e4b5122e1cd19969f1f1df72832040a335fd45af52597440e4 | GET | App header / auth state |
| IsHostQuery | ff889330f06ea6bb31cf107f0c0c50910d64669ab58a1671396857a2562af3c5 | GET | Whether current user is a host |
| GetThumbnailPicQuery | ab55c22df96bd74dfabf0f78b14f8172bf2cf52b7e2c29abc75ae65a59610d4b | GET | User profile picture |
| ArkoseSubmitTokenMutation | 45098faa13d86940aa1b43276b3f7c9c65a9374257d6098091abfce49ba93456 | POST | Bot challenge token submission (Arkose Labs CAPTCHA) |
| GetConsentFlagsQuery | ea4ccdd0346dfc6f8e581e6686615cfd79d530e9cfb7df266e841f990ec6ff3e | GET | Cookie consent state |

## ID format
- Listing IDs use Relay base64: `RGVtYW5kU3RheUxpc3Rpbmc6MzcxMjQ0OTM=` decodes to `DemandStayListing:37124493`
- User IDs: `VXNlcjoyODAwODE=` decodes to `User:280081`

## Auth
- Path A (SSR HTML): no auth needed
- Path B (GraphQL): cookies from logged-in session (httpOnly, Secure cookies). The CLI can use `auth login --chrome` to import cookies from user's Chrome profile if we need authenticated GraphQL access.

## Bot protection signal
- ArkoseSubmitTokenMutation captured = Arkose Labs CAPTCHA can fire on suspicious behavior
- DataDome subdomains observed: `f02aa.airbnb.com` and `d0a7e.airbnb.com` (challenge endpoints)
- For the SSR HTML path, no challenges were triggered during capture - openbnb pattern remains viable

## Decision for the printed CLI
- **Default transport: SSR HTML scrape** (openbnb pattern) - free, no auth, no DataDome
- **Optional authenticated mode**: cookie import + GraphQL persisted queries for wishlist/trips/calendar
