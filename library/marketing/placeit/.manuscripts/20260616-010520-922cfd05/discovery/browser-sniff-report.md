# Placeit Browser-Sniff Discovery Report

**Mode:** Authenticated capture (logged-in Envato session via browser-use headed, Default-profile fresh login).
**Effective rate:** ~1 req/s (conservative; no 429s observed).
**Reachability:** `browser_clearance_http` for placeit.net (Cloudflare managed challenge — cleared headed, blocked headless). Algolia host is **not** Cloudflare-fronted.

## Primary surface — Algolia (no auth, no Cloudflare) — REPLAYABLE
- `POST https://KSLVR81FGG-dsn.algolia.net/1/indexes/Stage_production/query`
  - Headers: `X-Algolia-Application-Id`, `X-Algolia-API-Key` (public search-only key, embedded in placeit.net JS — safe-to-ship default, env-overridable).
  - Body: `{query, hitsPerPage, page, facetFilters, facets, filters}`.
  - Index `Stage_production` = 164,580 published templates.
  - Replicas: `Stage_production_replica_newest`, `_replica_best_selling`, `_replica_free`.
  - Taxonomy index `Industries_production` (152 entries: objectID + name).
- **Stage record shape:** id, objectID, name, category_id, category_name, nice_category, template_type (image|blender|video|multi-stage|ios-stillshot|list), stage_link, editor_link, stage_description, seo_title, is_free, is_published, is_printify, is_elements, purchases, stage_ranking, published_date, date_for_ordering, large_thumb(+path/w/h), product_thumb(+path/w/h), preview_image_path, gif_image, device_tags, stage_tags, model_tags, gender_tags, age_tags, ethnicity_tags, color_tags, bundle_tags, invisible_tags.
- **Facets:** category_name {Mockups 67061, Design Templates 63415, Logos 26452, Videos 7637}; template_type {image 140473, blender 13186, video 10594, multi-stage 209, ios-stillshot 98, list 6}; plus all tag facets.

## Secondary surface — placeit.net /api (Cloudflare-fronted → needs Surf + cookies)
- `GET /api/v1/get_user_type_banner` — cookie auth — **200** — account + subscription status. Response fields (PII redacted): `user_type{name,id}`, `username`, `country`, `created_at`, `user_id`, `email`, `has_subscription`, `subscription_type`, `status`, `subscription_name`, `is_past_due`, `is_admin`, `envato_id`, `unsubscribe_code`. → `account` command.
- `GET /api/v2/bookmarked_stages_from_user?user_id=<id>` — cookie auth — **200** `{bookmarkedStages:[]}` (test account has none) — requires `user_id` (resolved from account). → `bookmarks` command.
- `GET /api/v2/get_active_campaigns` — no auth — **200** — active promos (badges, code_name, dates). → `campaigns` command.
- `GET /api/v2/get_pricing_plans` — no auth — **200** `{currency}` (needs params for full plans).

## Auth pattern
- **Cookie session, HttpOnly** (`document.cookie` can't read it; only `_gcl_au` visible). → spec `auth.type: cookie`, `cookie_domain: placeit.net`. CLI uses `auth login --chrome` to import placeit.net cookies from Chrome's cookie store and replay them.

## NOT captured (intentional)
- Editor render/download: async `blender` render job, artwork-upload-dependent, Cloudflare-protected. Not cleanly replayable → CLI ships `open <stage>` deep-link resolver (resolves stage_link + thumbnail URLs), not a raw render command.

## Provenance
- No existing Placeit API wrapper/SDK/scraper on GitHub/npm/PyPI. CLI is net-new.
