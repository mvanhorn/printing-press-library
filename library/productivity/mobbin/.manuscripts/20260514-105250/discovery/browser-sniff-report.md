# Mobbin Browser-Sniff Discovery Report

Method: agent-browser auto-connect to running Chrome (anonymous — Chrome for Testing instance lacked logged-in cookies from user's real Chrome). Captured anonymous public endpoints across iOS / Android / Web discover pages plus the search-bar UI surface.

## Reachability
- Anonymous browsing: works for `/discover/apps/{platform}/{tab}` landing pages, search bar, popular apps preview
- App detail (`/apps/<slug>` and `/apps/<slug>/<vid>/screens`): **redirects to /?redirect_to=...` for anonymous users** — login required.
- Image CDN: `bytescale.mobbin.com` (Bytescale, not Supabase Storage as research suggested).
- No Cloudflare/captcha walls hit during 5 min of capture.

## Captured Anonymous Endpoints (9)
| Method | Path | Body |
|--------|------|------|
| GET    | `/api/searchable-apps/{platform}` | none (platform: ios/android/web) |
| POST   | `/api/search-bar/fetch-trending-apps` | `{platform}` |
| POST   | `/api/search-bar/fetch-trending-sites` | none |
| POST   | `/api/search-bar/fetch-trending-filter-tags` | `{experience, platform?}` |
| POST   | `/api/search-bar/fetch-trending-text-in-screenshot-keywords` | `{platform}` |
| POST   | `/api/search-bar/fetch-searchable-sites` | none |
| POST   | `/api/filter-tags/fetch-dictionary-definitions` | none |
| POST   | `/api/popular-apps/fetch-popular-apps-with-preview-screens` | `{platform, limitPerCategory}` |
| POST   | `/api/discover/fetch-discover-page-apps` | `{tab, platform, pageIndex}` |

## Authenticated Endpoints (Inferred from pdcolandrea/mobbin-mcp source)
Not directly captured (anonymous session redirected) but the pdcolandrea MCP (active, May 2026) demonstrates these work with Supabase JWT cookies:

| Method | Path | Purpose |
|--------|------|---------|
| POST   | `/api/content/search-apps` | Search apps (filters + query) |
| POST   | `/api/content/search-screens` | Search screens (filters + query) |
| POST   | `/api/content/search-flows` | Search flows (filters + query) |
| GET    | `/api/content/app/<slug>/screens` | App's screens (paginated) |
| GET    | `/api/content/app/<slug>/flows` | App's flows (with screen lists) |
| GET    | `/api/content/screen/<id>` | Screen detail (patterns, elements, image URLs) |
| GET    | `/api/content/filters` | Available filter taxonomy |

Confidence: high for endpoint paths (mature wrapper, 31 stars, actively maintained); medium for exact body shapes (will validate in Phase 5 live testing using user's Chrome cookie).

## Auth Model
- **Type:** `cookie` — Supabase JWT split cookie pair
- **Cookies:** `sb-ujasntkfphywizsdaapi-auth-token.0` + `sb-ujasntkfphywizsdaapi-auth-token.1`
- **Refresh:** `POST https://ujasntkfphywizsdaapi.supabase.co/auth/v1/token?grant_type=refresh_token`
- **CLI auth flow:** `mobbin auth login --chrome` reads from `~/Library/Application Support/Google/Chrome/Default/Cookies` (encrypted SQLite); auto-refreshes when access token expires.

## Pattern Classification
- Standard Next.js API routes (POST/GET, no proxy envelope, no persisted query hashes).
- `client_pattern: standard` — no `--client-pattern proxy-envelope` needed.

## Platform Values
- `platform` enum: `ios`, `android`, `web`
- `tab` enum on discover: `latest`, `popular`, `animations` (and `featured` on community)

## Image URL Pattern
- Bytescale: `https://bytescale.mobbin.com/<account>/<image-id>` (likely with resize params)
- Next.js image proxy: `https://mobbin.com/_next/image?url=<encoded>&w=<width>&q=<quality>`
