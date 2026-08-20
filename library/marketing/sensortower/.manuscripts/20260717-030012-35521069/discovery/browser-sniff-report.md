# Sensor Tower Browser-Sniff Discovery Report

- **Run:** 20260717-030012-35521069
- **Target:** `https://app.sensortower.com/` (internal dashboard API)
- **Backend:** browser-use 0.13.1 (CLI mode), session injected via `cookies import` from agentcookie jar
- **Gate decision:** `pre-approved` (Phase 0 website choice, via pivot)
- **Account context:** signed in, `plan=free`, `api_authorized=false`, `api_key=false`

## Why this surface (and not the official API)

The user first chose Sensor Tower's **official API** (`api.sensortower.com`). That target was
proven unbuildable on this account and the run pivoted with explicit user approval:

| Evidence | Value |
|---|---|
| `api.sensortower.com/*` (docs, swagger.json, apidocs.json) | `401` JSON, every path |
| `app.sensortower.com/api/docs` (anonymous) | 200 but `effective_url=/users/sign_in` (login wall) |
| `app.sensortower.com/api/docs` (authenticated) | `403`, `content-length: 0`, `server: openresty`, `x-runtime: 0.0137` |
| `st-user-data.api_authorized` | `False` |
| `st-user-data.api_key` | `False` (absent, not hidden) |
| `st-user-data.plan` | `{id: "free", name: "Free"}` |

The 403 carries Rails security headers and a real `x-runtime` — it is the **application**
refusing an unentitled account, not a WAF. `probe-reachability` was therefore not run:
there is no challenge to classify. No amount of transport work fixes an entitlement gate.

## Reachability / transport

- **No bot protection.** In-browser challenge check returned `{"challenge": false}`.
  No `/cdn-cgi/challenge-platform`, no Turnstile, no DataDome, no CAPTCHA.
- **Runtime = `standard_http`.** Every endpoint below replays with plain `curl` + a Chrome
  UA. **No browser sidecar is required at runtime.** Replayability criterion (capture
  Rule 5) is satisfied.

## CORRECTION — two findings in the first pass of this report were wrong

The first pass of this report claimed (a) the whole surface requires the session cookie,
and (b) `GET /api/ios/apps/{app_id}` is "hard-gated 429, use the batch form instead."
Both were **wrong**, and both were corrected by re-testing after a cooldown:

1. **The 429 was self-inflicted rate limiting, not a gate.** After a 90s cooldown, a single
   anonymous request to `/api/ios/apps/460177396` returned **200 with 39,305 bytes** — the
   richest payload on the entire surface (51 top-level keys). The first pass had burned the
   request budget with rapid probing and then misread the resulting 429 as an endpoint
   property. Rate-limit state must be cleared before concluding anything about an endpoint.
2. **Most of the surface needs no auth at all.** Verified anonymously (no cookie): singular
   app, batch apps, rankings, publisher apps, `autocomplete_search`, `docs/static`.

The cookie's real value is narrow and specific — see the auth boundary below.

## Auth boundary (verified by A/B testing anonymous vs cookie)

| Endpoint | Anonymous | With cookie |
|---|---|---|
| `/api/ios/apps/{id}` | **200** (39 KB) | 200 |
| `/api/ios/apps?app_ids=` | **200** | 200 |
| `/api/android/apps?app_ids=` | **200** | 200 |
| `/api/ios/category_rankings` | **200** | 200 |
| `/api/android/category_rankings` | **200** | 200 |
| `/api/ios/publishers/{id}/apps` | **200** | 200 |
| `/api/autocomplete_search` | **200** (78 KB, 99 entities) | 200 |
| `/api/docs/static/category_ids.json` | **200** | 200 |
| **`/api/unified/apps`** | **401** `{"error":"You need to sign in..."}` | **200** |
| **`/api/unified/publishers`** | 401 | **200** |

**The cookie buys exactly one thing: the `unified/*` cross-platform identity endpoints.**
Everything else is open. The right spec shape is therefore `auth.type: cookie` with
`no_auth: true` tagged on every open endpoint — the CLI works with zero setup, and
`auth login --chrome` unlocks unified.

## Endpoints for the spec (all verified live)

Base: `https://app.sensortower.com`

| # | Method / Path | Auth | Bytes | Notes |
|---|---|---|---|---|
| 1 | `GET /api/ios/apps/{app_id}` | none | 39305 | **Richest endpoint.** 51 keys; see below. |
| 2 | `GET /api/ios/apps?app_ids=` | none | 13041 | Batch/lean; multi-ID |
| 3 | `GET /api/android/apps?app_ids=` | none | 13078 | Package names |
| 4 | `GET /api/unified/apps` | **cookie** | 445 | `app_id_type` ∈ {itunes, android, unified} |
| 5 | `GET /api/ios/category_rankings` | none | 12366 | `category`, `country`, `date`, `device`, `limit`, `offset` |
| 6 | `GET /api/android/category_rankings` | none | 12544 | category enum below |
| 7 | `GET /api/ios/category/category_history` | none | 569 | `app_ids[]`, `categories[]`, `chart_type_ids[]`, `countries[]`, date range |
| 8 | `GET /api/ios/category/app_category_ranking_summary` | none | 751 | `app_ids[]` |
| 9 | `GET /api/ios/publishers/{id}/apps` | none | 7064 | `sort_by` **required** |
| 10 | `GET /api/unified/publishers` | **cookie** | 280 | `publisher_ids`, `publisher_id_type` |
| 11 | `GET /api/autocomplete_search` | none | 78313 | **99 entities**; `term`, `entity_type`, `os` |
| 12 | `GET /api/docs/static/category_ids.json` | none | 3655 | Free category reference data |

Response samples: `discovery/samples/*.json` (credential-scanned clean).

### What `/api/ios/apps/{id}` actually returns (51 keys)

`versions` (**469 entries**, `{date: epoch_ms, value: "30.3"}`), `category_rankings`
(exact ranks per `iphone`/`ipad` × `top_free`/`top_grossing`/`top_paid`, with
`all_categories` + `primary_categories`), `top_in_app_purchases`, `rating_breakdown`
(5-star histogram), `featured_reviews`, `related_apps`, `top_countries`,
`available_countries`, `worldwide_last_month_downloads`, `worldwide_last_month_revenue`,
`screenshots`, `trailers`, `supported_languages`, `advertised_on_any_network`.

**Ranks are exact; money is bucketed.** `worldwide_last_month_revenue` comes back as
`{value: 100000, unit: "cent"}` (1 significant figure) and chart rows render as
`{prefix: "< $", string: "< $5k"}`. The CLI must lean on ranks/deltas/trends and treat
revenue as a soft signal — never present bucketed money as precise.

## Rate limiting — the dominant design constraint

- Measured by the research pass: **~13 requests succeed, #14 → 429**; recovery **≈240s**
  (still 429 at t+120s and t+180s; 200 at t+240s).
- Independently reproduced here: sustained probing produced a sticky 429 that survived a
  30s backoff; a 90s+ cooldown cleared it.
- `server: awselb/2.0`, **no `Retry-After`, no `X-RateLimit-*`**, empty body. Enforced at
  the AWS ELB — nothing to negotiate with.
- **Implication:** budget ~12 requests per burst, then a 4-minute cooldown. The CLI must be
  cache-first, not request-first, and must surface a typed rate-limit error rather than
  returning empty results (empty-on-throttle is indistinguishable from "no data").

## Gated / non-existent

| Path | Result | Meaning |
|---|---|---|
| `GET /api/ios/search_entities` | 403 | Not the real search route — use `/api/autocomplete_search` (open). |
| `GET /api/android/publishers/{name}/apps` | 404 | Not a real route (iOS-only shape). |
| `GET /api/ios/top_publishers`, `/api/ios/usage_intelligence/active_users` | 404 | Do not exist. |
| `GET /api/ios/internal_entities` | 200 `{"apps":[]}` | Empty on free (no saved entities). |
| `/api/ios/ad_intel/*` | 401 | Ad Intelligence not entitled on free. |

## Self-documenting 422s (source of spec enums)

Three endpoints returned `422` with the valid values enumerated in the error body. These
are authoritative parameter values, taken from the API itself rather than inferred:

- `android category_rankings` → `category` must be one of:
  `application, art_and_design, auto_and_vehicles, beauty, books_and_reference, business,
  comics, communication, dating, education, entertainment, events, finance, food_and_drink,
  game, health_and_fitness, house_and_home, libraries_and_demo, lifestyle,
  maps_and_navigation, medical, music_and_audio, news_and_magazines, parenting,
  personalization, photography, productivity, shopping, social, sports, tools,
  travel_and_local, video_players, weather, game_action, game_adventure, game_arcade,
  game_board, game_card, game_casino, game_casual, game_educational, game_music,
  game_puzzle, game_racing, game_role_playing, game_simulation, game_sports, …`
- `unified/publishers` → `publisher_id_type` ∈ `{unified, android, itunes}`
- `ios/publishers/{id}/apps` → `sort_by` is **required**

## Auth model for the printed CLI

```yaml
auth:
  type: cookie
  cookie_domain: .sensortower.com
  cookies:
    - sensor_tower_session
```

`sensor_tower_session` alone carries the session (Rails). `locale` and `__cf_bm` are
incidental. No `Authorization` header is constructed by the SPA — auth is pure cookie.
`auth login --chrome` is the right onboarding shape.

## Notes / limitations

- All estimate-bearing product surfaces (downloads/revenue estimates, Ad Intelligence /
  Pathmatics, Usage Intelligence) are **not** reachable on free: `pathmatics_allowed_retailers: []`,
  `web_or_pathmatics: false`. The reachable surface is **store presence + rankings**, not estimates.
- `limits` on this account: `keywords: 5`, `research: 1300`, `category_rankings: 100`, `suggestions: 0`.
- Session cookie lives only in `${TMPDIR}/printing-press-$(id -u)/session/$RUN_ID/` (0700/0600)
  and is excluded from this run's archived artifacts by location.
