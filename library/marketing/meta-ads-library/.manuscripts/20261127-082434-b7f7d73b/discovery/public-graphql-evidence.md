# Meta Ads Library Live Discovery Evidence

Verified live on 2026-11-27 via pure HTTP:

- `https://www.facebook.com/` — HTTP 200 (anonymous homepage bootstrap supplying `lsd`, `__spin_r`, `__hsi`, `__hs`, `v`, and session cookies `datr`, `fr`, `sb`)
- `https://www.facebook.com/api/graphql/` — HTTP 200 for:
  - `useAdLibraryTypeaheadSuggestionDataSourceQuery` (`doc_id: 9755915494515334`)
  - `AdLibrarySearchPaginationQuery` (`doc_id: 25464068859919530`)
  - `AdLibraryV3AdDetailsQuery` (`doc_id: 25068828942793558`)
- `https://www.facebook.com/ads/library/` — public web deep-link portal URL constructed by `ads url`
- `https://graph.facebook.com/v27.0/ads_archive` — Official Meta Ad Library Graph API v27.0 (optional `META_ACCESS_TOKEN`)
- Outbound redirect wrapper hosts observed inside ad `snapshot.link_url` payloads (decoded locally by `UnwrapRedirectURL`, never dialed):
  - `https://l.facebook.com/l.php`
  - `https://lm.facebook.com/l.php`
  - `https://l.instagram.com/`
