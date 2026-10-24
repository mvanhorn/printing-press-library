# App Store crowd-sniff report

Run 20261023-151515-ab5f6220 · 2026-10-23 17:28-17:36 PKT
Command: `cli-printing-press crowd-sniff --api app-store --base-url https://itunes.apple.com`
(owner approved "crowd-sniff and merge" at the Phase 1.8 gate, 17:27 PKT).

**Outcome: 91 endpoints across 58 resources (1 community SDK, 90 GitHub code search).
None is an App Store storefront endpoint. Not merged into the generation spec.**

## 1. npm Packages Analyzed
- The miner reported one community-SDK endpoint, `POST /account/listTeams.action`, an Apple
  Developer portal (account) route used by developer-tooling SDKs: authenticated, out of
  scope for a public storefront CLI.
- `appstore-connect-sdk` failed to process: its tarball exceeds the 10 MB decompressed limit.
  It targets App Store Connect, which is out of scope (the library's `asc` CLI covers it).
- npm download counts were unavailable (the npm downloads API returned 400).
- The packages that actually read the public store (app-store-scraper and its forks,
  @perttu/app-store-scraper, app-store-client, parse-tunes) yielded no endpoints here; their
  endpoints were catalogued from source by the Phase 1 research sweep instead.

## 2. GitHub Repos Searched
- Query derived from the API name `app-store`; GitHub token: authenticated (owner's gh token,
  passed through the environment, never written).
- Code search page 2 returned 403 (secondary rate limit), so results are page 1 only.
- The term matched unrelated projects' private APIs: app installers and "app store" UIs,
  payment and subscription backends (RevenueCat, Apple receipt validation), MCP catalogs,
  "Download on the App Store" badge services, App Store Connect sales routes.

## 3. Endpoints Discovered
| Method | Path | Source Tier | Source Count |
|---|---|---|---|
| POST | `/account/listTeams.action` | community-sdk | 1 |
| DELETE | `/api/account/delete` | code-search | 1 |
| GET | `/api/admin` | code-search | 1 |
| POST | `/api/android-tester/signup` | code-search | 1 |
| GET | `/api/app-store` | code-search | 1 |
| GET | `/api/app-store-connect/analytics-report-requests` | code-search | 1 |
| GET | `/api/app-store-connect/apps` | code-search | 1 |
| GET | `/api/app-store-connect/sales-report` | code-search | 1 |
| GET | `/api/app-store/apps` | code-search | 1 |
| GET | `/api/app-store/ingest` | code-search | 1 |
| GET | `/api/app-store/install` | code-search | 1 |
| GET | `/api/app-store/install-local` | code-search | 1 |
| GET | `/api/app-store/install-server` | code-search | 1 |
| GET | `/api/app-store/installations` | code-search | 1 |
| GET | `/api/app-store/refresh` | code-search | 1 |
| GET | `/api/app-store/{id}` | code-search | 1 |
| GET | `/api/apps` | code-search | 1 |
| GET | `/api/apps/install` | code-search | 1 |
| GET | `/api/apps/registry` | code-search | 1 |
| GET | `/api/apps/registry/install` | code-search | 1 |
| GET | `/api/apps/registry/install-stream` | code-search | 1 |
| GET | `/api/auth.js` | code-search | 1 |
| POST | `/api/auth/register` | code-search | 1 |
| GET | `/api/badges/download-on-the-app-store/black/de-de` | code-search | 1 |
| GET | `/api/badges/download-on-the-app-store/black/en-us` | code-search | 1 |
| GET | `/api/badges/download-on-the-app-store/black/it-it` | code-search | 1 |
| POST | `/api/chat/report` | code-search | 1 |
| GET | `/api/client.js` | code-search | 1 |
| GET | `/api/client.ts` | code-search | 1 |
| GET | `/api/config` | code-search | 1 |
| POST | `/api/content/seed` | code-search | 1 |
| GET | `/api/core` | code-search | 1 |
| GET | `/api/docs` | code-search | 1 |
| GET | `/api/ebay-price` | code-search | 1 |
| PUT | `/api/editor/boutique` | code-search | 1 |
| GET | `/api/graphql` | code-search | 1 |
| GET | `/api/init` | code-search | 1 |
| GET | `/api/native/v1/orders/route.ts` | code-search | 1 |
| GET | `/api/native/v1/storekit/notifications/route.ts` | code-search | 1 |
| GET | `/api/offering/simulate` | code-search | 1 |
| GET | `/api/orgs` | code-search | 1 |
| GET | `/api/overview` | code-search | 1 |
| GET | `/api/payments/bizum-checkout` | code-search | 1 |
| GET | `/api/payments/cards/set-default` | code-search | 1 |
| GET | `/api/payments/cards/setup-intent` | code-search | 1 |
| GET | `/api/process` | code-search | 1 |
| GET | `/api/reggie/messages` | code-search | 1 |
| GET | `/api/revenuecat/projects` | code-search | 1 |
| GET | `/api/safety/emergency-stop` | code-search | 1 |
| GET | `/api/session` | code-search | 1 |
| GET | `/api/settings/hf-token` | code-search | 1 |
| POST | `/api/source/test` | code-search | 1 |
| GET | `/api/sync` | code-search | 1 |
| GET | `/api/sync/history` | code-search | 1 |
| GET | `/api/system/status` | code-search | 1 |
| GET | `/api/unsubscribe` | code-search | 1 |
| POST | `/api/upload` | code-search | 1 |
| POST | `/api/upload-font` | code-search | 1 |
| GET | `/api/v0.1/countries/state/cities` | code-search | 1 |
| GET | `/api/v1/app-store/apps` | code-search | 1 |
| GET | `/api/v1/apps` | code-search | 1 |
| GET | `/api/v1/mac-app-store` | code-search | 1 |
| GET | `/api/v1/mac-app-store/apps` | code-search | 1 |
| GET | `/api/v1/mcp/catalog` | code-search | 1 |
| POST | `/api/v1/mcp/catalog/install` | code-search | 1 |
| POST | `/api/v1/mcp/catalog/uninstall` | code-search | 1 |
| POST | `/api/v1/mcp/connect` | code-search | 1 |
| POST | `/api/v1/mcp/disconnect` | code-search | 1 |
| GET | `/api/v1/mcp/tools` | code-search | 1 |
| GET | `/api/v1/mobile-device-applications` | code-search | 1 |
| GET | `/api/v1/models` | code-search | 1 |
| POST | `/api/v1/payments/validate/apple-app-store` | code-search | 1 |
| GET | `/api/v1/widget/accounts` | code-search | 1 |
| GET | `/api/v2/badges/download-on-the-app-store/black` | code-search | 1 |
| GET | `/api/v2/badges/download-on-the-app-store/black/en-us` | code-search | 1 |
| GET | `/api/v2/badges/download-on-the-app-store/black/ja-jp` | code-search | 1 |
| GET | `/api/v4/recitations` | code-search | 1 |
| GET | `/api/vibe` | code-search | 1 |
| GET | `/api/{id}/projects/{artifact.project.organization.slug}/{artifact.project.slug}/files/installablepreprodartifact/{installable.url_path}/{id}` | code-search | 1 |
| GET | `/v1/app-store-notifications.js` | code-search | 1 |
| GET | `/v1/appstore` | code-search | 1 |
| GET | `/v1/bootstrap.js` | code-search | 1 |
| GET | `/v1/messages` | code-search | 1 |
| GET | `/v1/products` | code-search | 1 |
| GET | `/v1/push-tokens.js` | code-search | 1 |
| GET | `/v1/subscribers` | code-search | 1 |
| GET | `/v1791298784/immg_wblaue.jpg` | code-search | 1 |
| GET | `/v2/library` | code-search | 1 |
| GET | `/v2/v3` | code-search | 1 |
| GET | `/v4/sabbirn26` | code-search | 1 |
| GET | `/v5/screenshots/macos` | code-search | 1 |

91 endpoints; by method: DELETE 1, GET 76, POST 13, PUT 1

## 4. Base URL Resolution
`https://itunes.apple.com` was supplied from Phase 1 research (the documented Search,
Lookup and RSS host). The discovered paths belong to other projects' hosts, so the base URL
does not apply to them; no other candidate base URL was produced.

## 5. Auth Patterns Detected
The miner inferred `bearer_token` with `APP_STORE_TOKEN`. This is an artifact of the
unrelated code it matched. Every App Store surface this CLI uses is keyless; the storefront
frontend sends an empty `Authorization: Bearer` header that Apple's edge fills itself.

## 6. Parameter Name Evidence
None (param_count 0).

## 7. Coverage Summary
- 91 endpoints: GET 76, POST 13, PUT 1, DELETE 1; tiers: code-search 90, community-sdk 1.
- Brief surfaces found: 0 of the research-verified set (Lookup, Search, RSS charts,
  customer-reviews RSS, search hints, app/chart/developer pages).
- Decision: not merged. Merging would add mutating commands (POST/PUT/DELETE) from unrelated
  codebases and a required bearer token to a read-only, keyless CLI, the opposite of the
  approved intent of adding App Store endpoints. The spec stays at
  `research/app-store-crowd-spec.yaml` for audit only. The owner can override at the
  Phase 1.5 gate.
