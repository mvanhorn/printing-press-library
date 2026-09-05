# Artistly CLI Brief

## API Identity
- **Domain:** AI image generation + editing suite (text-to-image, inpainting, background removal, upscaling, redesign/restoration, image expansion, AI try-on, consistent-character creation, AI logo maker, storybook/coloring-page/merch/book-cover generation).
- **Target:** `app.artistly.ai` — the Artistly web application (login portal). `artistly.ai` is the marketing/checkout site.
- **Users:** Creators, marketers, KDP/merch sellers producing AI art and commercial assets.
- **Data profile:** Generation jobs, generated images/assets, prompts, projects/collections, styles/models, consistent characters, credits/quota, exports/downloads. (Field names unverified — must come from browser-sniff.)

## Reachability Risk
- **Runtime: PASS / standard_http.** `probe-reachability` on both `app.artistly.ai` and `artistly.ai` returned HTTP 200 via stdlib AND surf-chrome; `needs_browser_capture: false`, `needs_clearance_cookie: false`. No Cloudflare/WAF challenge at the transport layer.
- **BUT the 200 is the login page.** `app.artistly.ai` is a Laravel + Inertia.js server-rendered app; unauthenticated requests 302 → `/login`. Every useful surface is behind a session cookie. "Reachable" does not mean "usable without auth."
- **Auth model:** email/password → Laravel session cookie (`artistly_session`, httponly) + CSRF (`XSRF-TOKEN` cookie / `X-XSRF-TOKEN` header), Inertia front end (`Vary: X-Inertia`). No API key / Bearer / OAuth. Optional 2FA (SMS/authenticator) on login.
- **Operational risks:** undocumented ~400 generations/day fair-use cap where *failed* generations also count; unannounced slowdowns/maintenance. Some third-party fetchers (theresanaiforthat) hit 403 anti-bot, so live calls need a real browser-derived session + proper headers.

## Critical Constraint: No API, behind-login only
- **No official public API exists** (open feature request on roadmap.artistly.ai; `/api`, `/docs`, `/swagger`, `/openapi.json` all 404; `api.artistly.ai` does not resolve).
- The CLI must be built by reverse-engineering the Inertia/XHR endpoints behind `app.artistly.ai`, authenticating via the user's session cookie + XSRF token, and replaying those requests over direct/Surf HTTP. This is a **`cookie`/`composed` auth, browser-sniff-discovered** build — not a spec-based build.
- **Hard dependency:** discovery and the shipped CLI both require a logged-in Artistly session. With no session there is literally nothing to operate on (everything 302s to login).

## Top Workflows
1. Batch text-to-image generation from prompts (the headline workflow).
2. Edit pipeline on an existing image — upscale / background-removal / inpainting / expand.
3. Consistent-character creation across a set.
4. Commercial asset production — logos, product mockups, merch/t-shirt designs, book covers.
5. Bulk export/download of generated assets + quota/credit status.

## Table Stakes (vs the category)
- Prompt → image generation with style/resolution controls.
- Asset listing + download.
- Edit operations (upscale, bg-remove, inpaint).
- Quota/credit visibility (especially given the 400/day cap).

## Data Layer
- **Primary entities:** generations (jobs), images/assets, projects/collections, prompts, characters, credits/quota.
- **Sync cursor:** generation/asset created-at or job id.
- **FTS/search:** prompts + asset metadata.

## Codebase Intelligence
- Laravel + Inertia.js (session cookie + CSRF). No public SDK/MCP/wrapper exists — first-of-its-kind build.

## User Vision
- User explicitly targeted **app.artistly.ai** (the application), confirming intent to reverse-engineer the app's functionality rather than a (nonexistent) official API.

## Product Thesis
- **Name:** artistly-pp-cli
- **Why it should exist:** Artistly has no API and no CLI. A session-cookie-backed CLI would let creators batch-generate, batch-edit, and bulk-export assets and track quota from the terminal/agents — none of which the web UI makes scriptable. Offline asset/prompt search + a local store of generation history is value the web app does not offer.

## Build Priorities
1. Confirm a logged-in session is available; browser-sniff the authenticated app to learn the Inertia/XHR contract (generate, list assets, edit, quota).
2. Cookie/composed-auth runtime: import session cookie + XSRF, replay JSON routes.
3. Local store of generations/assets/prompts with search; bulk export.
4. Respect the 400/day cap; surface quota in `doctor`.

## Reachability Gate
- Decision: PASS (runtime standard_http, 200 from both domains). Discovery feasibility is gated on a logged-in session (see Phase 1.6).
