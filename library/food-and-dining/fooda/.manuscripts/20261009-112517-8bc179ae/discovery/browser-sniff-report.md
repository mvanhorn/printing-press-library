# Fooda browser-sniff report
Backend: browser-use 0.1.4 (CDP attach to user's Chrome, fresh tab). Auth: logged-in session.
## Surfaces (all replayable over plain HTTP; no browser at runtime)
- POST https://app.fooda.com/graphql  (full-text queries, not persisted). Headers: content-type json, x-clienttoken, x-sessiontoken.
  Tokens come from <meta name="api-client-token"> / <meta name="api-session-token"> on any authenticated app.fooda.com page (fetched with _fooda_session cookie). Replay verified with curl; works with tokens alone.
  Ops: searchPublicEvents(input:{statuses,productTypes[POPUP,CAFE,DELIVERY,CATERING],accountId,buildingId,endTimeAfter,endTimeBefore}) -> Cafe/Popup/DeliveryEventPublic with restaurants[];
       getUserSubsidies (name, code, coverage, state, amount via qrInfo); getFoodaCardOutstandingBalances; getUserCards(userId); GetRecommendationsWithItem; effectiveConsentPreference (skip).
- SSR HTML (cookie `_fooda_session`): /my?date=YYYY-MM-DD (events, account/building ids in links), /settings/orders (past orders list: vendor, total, delivered date, item; links /settings/select_order/<uuid>), /settings/select_order/<uuid> (order detail), /users/subsidies, /settings/profile, /accounts/<acct>/select_events/<S-id>/items[?filterable[vendor_id]=] (menu items, SSR).
## Cloudflare
- /login is Cloudflare-challenged (cf-mitigated). Authenticated page + /graphql replay worked from curl with stdlib HTTP and a Chrome UA, with and without cf_clearance.
- CLI auth: import `_fooda_session` (+`context`, `myfooda_building_id`) cookies from Chrome (auth login --chrome); never log in via CLI.
## Not yet explored
- Order detail page, menu item pages, cart/checkout/cancel mutations (not captured; mutations must stay dry-run + --confirm).
