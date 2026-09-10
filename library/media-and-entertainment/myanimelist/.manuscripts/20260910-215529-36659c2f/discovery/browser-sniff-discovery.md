# Browser-Sniff Discovery Report — myanimelist.net

Run: `20260910-215529-36659c2f`
Consent: **pre-approved** at Phase 0 (user chose "The MyAnimeList website itself").
Backend: `browser-use` CLI mode (v-present, commands `open`/`eval`/`scroll`/`close` verified).
Goal walked: *fetch and filter anime data* (read-only site; the primary flow is search → title detail → season/ranking browse).

## 1. Headline result

**The site has no hidden JSON/XHR API.** MyAnimeList is fully server-rendered. Every data surface a CLI needs is either HTML, RSS, XML, or the single JSON prefix-search endpoint. No BFF, no GraphQL, no proxy envelope.

## 2. Challenge check

| Probe | Result |
|---|---|
| `probe-reachability https://myanimelist.net/anime/1` | `mode: browser_clearance_http`, `body_evidence: ["recaptcha"]` — **false positive** (see §5) |
| `browser-use` rendered DOM on `/anime/1` | `challenge: false`, DOM length 373,843 bytes, `document.title` = `Cowboy Bebop (TV 1998) - MyAnimeList.net` |
| DOM value parse | `document.querySelector('span.score-label').textContent` → `8.75` |

## 3. First-party network resources observed (Performance API)

`/anime/1` — 11 requests to `myanimelist.net`, of which the only non-static, non-image ones were:

- `https://myanimelist.net/ux/s` (analytics/UX beacon)
- `https://myanimelist.net/img/common/pwa/mal-icon-192x192.png` (PWA icon)

Everything else on the page is third-party advertising/CMP/CDN traffic (`cdn.myanimelist.net` static assets, InMobi CMP, Prebid/Criteo/PubMatic/Taboola bidders, Google Analytics). **No application XHR/fetch to the origin.**

`/anime/season/2026/fall` — identical shape. `document.querySelectorAll('.js-categories-seasonal .anime-header').length` → **5** (TV, ONA, OVA, Movie, Special) rendered server-side; seasonal filtering (`js-visible-anime-count`) is client-side JS over already-rendered DOM.

`/anime.php?q=frieren` — search results rendered server-side (`4` result titles in `.js-block-list`); the only `.json` request on the page is `/manifest.json` (PWA manifest).

A `fetch` interceptor armed on the search page and driven by dispatching an `input` event on the search box captured **no** application fetches, confirming the typeahead endpoint is not required for page-rendered results (it is required only for the instant-suggest widget, and is reachable directly — see §4).

## 4. Replayable surfaces confirmed (the spec contract)

| Surface | Kind | Replay verified |
|---|---|---|
| `/search/prefix.json?type={anime,manga,character,person}&keyword=<q>&v=1` | JSON | `200 application/json`; payload carries `id`, `type`, `name`, `url`, `image_url`, `thumbnail_url`, `payload.{media_type,start_year,aired,score,status}` (anime) / `payload.{related_works,favorites}` (character), `es_score` |
| `/rss/news.xml`, `/rss/featured.xml` | RSS 2.0 | `200 application/xml`; `/rss.php?type=news\|featured` 301-redirects here, `/rss.php?type=rw\|rm` is 404 (removed) |
| `/sitemap/index.xml` (+ `anime-001`, `manga-00N`, `character-00N`, `people-00N`, `news-001`, `featured-001`, `company-001`, `main`) | Sitemap XML | `200 text/xml`, robots-advertised |
| All HTML routes in the brief | HTML | `200` with full anonymous content |

## 5. The `probe-reachability` false positive, in full

`probe-reachability` returned:

```json
{"mode":"browser_clearance_http","confidence":0.6,
 "body_evidence":["recaptcha"],
 "probes":[{"transport":"stdlib","status":200,"evidence":["reCAPTCHA widget"]},
           {"transport":"surf-chrome","status":200,"evidence":["reCAPTCHA widget"]}],
 "recommendation":{"runtime":"browser_clearance_http","needs_browser_capture":true,"needs_clearance_cookie":true}}
```

Both transports got `200` with real content; the classifier keyed only on the substring `recaptcha`. That substring is present because MAL's login/registration form embeds a reCAPTCHA widget and the page head defines `window.GRECAPTCHA_SITE_KEY`. The real browser capture in §2 shows no interstitial, no challenge script (`/cdn-cgi/challenge-platform` absent), and no `cf-turnstile`. **Runtime for the printed CLI is `standard_http` with a browser-like `User-Agent`.** No clearance cookie is captured, and none is needed.

## 6. Replayability verdict

**PASS.** Every surface the CLI will ship replays through plain HTTP with a browser `User-Agent`. No surface requires live page-context execution, a resident browser, or a cookie. The only non-replayable things observed were third-party ad/analytics calls, which are excluded.

## 7. Discovery-only risks recorded

- **Login-form reCAPTCHA** will make future automated *auth* discovery noisy; irrelevant to this anonymous-only scope.
- **`robo`/UA sensitivity:** robots.txt blocks a named set of scraper and AI-crawler UAs with `Disallow: /`. The CLI must send a descriptive, honest `User-Agent` and keep request volume user-driven.
- **Slug tolerance:** `/{type}/{id}/{slug}` ignores the slug, so the CLI can build canonical URLs from the id alone (no slug computation, no redirect-following required).
