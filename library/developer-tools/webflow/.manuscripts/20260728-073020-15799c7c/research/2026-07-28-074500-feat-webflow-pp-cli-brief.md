# Webflow CLI Brief

## API Identity
- **Domain:** Visual web development / no-code site builder with a headless CMS, ecommerce, and hosting. The Data API v2 (`https://api.webflow.com/v2`) is the programmatic surface over sites, pages, CMS collections, assets, forms, custom code, ecommerce, and publishing.
- **Users:** Webflow site owners, agencies managing many client sites, marketing/content ops teams running CMS-backed blogs and landing pages, and increasingly AI agents doing bulk content and SEO work.
- **Data profile:** Hierarchical and site-scoped. A workspace holds sites; a site holds pages, collections, assets, forms, webhooks, custom code, and (on ecommerce plans) products/orders/inventory. Collections hold typed fields and items; items have a **staged** and a **live** variant, which is the single most important modeling fact in the whole API. Page objects carry SEO metadata and openGraph. Page *content* is reachable as a DOM node tree.

## Spec Source
- **Official OpenAPI 3.1.0:** `https://raw.githubusercontent.com/webflow/openapi-spec/main/openapi/v2.yml` (3.4 MB, pushed 2026-05-26)
- 117 operations across 16 tags: Assets, Custom Fonts, Collections, Custom Code, Custom Code - Pages, Custom Code - Sites, Forms, Inventory, Items, Meta, Orders, Pages, Products & SKUs, Settings, Sites, Webhooks
- Method mix: 51 GET / 38 POST / 19 DELETE / 17 PATCH / 6 PUT
- `apis.guru` also indexes `webflow.com`, but its entry is mis-labelled (`info.title: "Lucidtech API"`, added 2023) and must NOT be used.

## Auth
- **Scheme:** `ApiKey: type: http, scheme: bearer` → `Authorization: Bearer <token>`. Also OAuth2 authorization-code with 26 granular scopes (`sites:read`, `cms:write`, `pages:write`, `custom_code:write`, `ecommerce:read`, …).
- **Canonical env var:** `WEBFLOW_API_TOKEN` — the convention used by the official JS SDK (`webflow-api`, 476k downloads/mo) and the official CLI's `--api-token` flag. The official MCP server uses `WEBFLOW_TOKEN`; `WEBFLOW_API_TOKEN` is the dominant convention and the one to ship.
- **Two token shapes:** a **site token** (Site settings → Apps & Integrations → API access → Generate API token) scoped to one site, and a **workspace token** (`ws-` prefix) covering all sites in a workspace. Both go in the same header. **A workspace token generated for DevLink carries no Data API scopes** — verified live during this run: a valid `ws-` token returned `403 OAuthForbidden: missing scopes 'sites:read'` on `GET /v2/sites`. The CLI's `doctor` must distinguish "no token" from "token present but unscoped," because the second is the failure users will actually hit.

## Rate Limits
- 60 req/min on Starter and Basic plans; 120 req/min on CMS, eCommerce, and Business; custom on Enterprise.
- Response headers on every call: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `Retry-After`. Over-limit returns `429` with `Retry-After` (typically 60s).
- **Implication:** do NOT enable generator cache auto-refresh. 60/min is tight enough that a pre-read upstream refresh is a real cost. Ship manual `sync` plus a `doctor` cache-freshness report instead.

## Reachability Risk
- **None.** Official, documented, vendor-maintained REST API. No bot protection, no Cloudflare challenge, no clearance requirement.
- Live probe this run: `GET https://api.webflow.com/v2/sites` → `403` with clean JSON body and real rate-limit headers (`x-ratelimit-limit: 60`, `x-ratelimit-remaining: 57`). The 403 is a token-scope condition, not a block.
- Tier/permission hints from 403 body: `"OAuthForbidden: You are missing the following scopes - 'sites:read'"`, `"code": "missing_scopes"`.
- Probe-safe endpoint used: `GET /v2/sites` (read-only list, no `x-pp-safe-probe` mutation probe run).
- One flake observed: `GET /v2/token/introspect` returned `500 internal_error` for an unscoped token. Worth a troubleshooting entry.

## Top Workflows
1. **Publish a site or a single page**, optionally to a chosen subset of custom domains, and know what changed before doing it.
2. **Bulk CMS content manipulation** — list/filter items in a collection, patch many at once, and control the staged-vs-live distinction deliberately.
3. **Audit and fix page SEO** — read every page's title, description, and openGraph across a site; find what's missing, duplicated, or over-length; write corrections back.
4. **Read and rewrite page text content** via the page DOM node tree (`GET/POST /pages/{page_id}/dom`) — the surface that lets an agent actually edit copy, not just metadata.
5. **Site hygiene before a launch** — redirects, `robots.txt`, custom code, webhooks, asset inventory, form submissions.

## Table Stakes (what the incumbents already do)
- `sites list` / `sites get` / `sites domains` / `sites publish` (with `--domain`, `--page`, `--dry-run`)
- `cms collections list|get|create`, `cms fields create`, `cms items list|get|create|update|delete|publish`
- `forms list`, `forms submissions` with CSV export
- `assets list|upload|update`, `assets folders list|create`
- `auth login` (OAuth), token stored locally
- `--json` output and `--fields` column selection on list commands

## Competitive Landscape
| Tool | Reach | What it covers | What it misses |
|---|---|---|---|
| `@webflow/webflow-cli` v2.2.0 (official) | 322k dl/mo | sites, cms, forms, assets, devlink, cloud/apps, designer extensions | **No pages at all.** No page DOM, no SEO metadata, no redirects, no robots.txt, no custom code, no webhooks, no components, no ecommerce, no comments, no activity logs, no workspaces |
| `webflow-api` npm SDK v3.3.4 (official) | 476k dl/mo | full Data API (Fern-generated) | It's a library, not a CLI. No local store, no offline query, no auditing |
| `webflow/mcp-server` (135★) | official MCP | consolidated `data_cms_tool`, `data_sites_tool`, `data_pages_tool`, `data_components_tool`, `ask_webflow_ai` | Designer-bridge tools (`deElement`, `dePages`, `deStyle`, `deVariable`) require the Webflow Designer open with a companion app running — not usable headless or in CI |
| `webflow/webflow-skills` (106★) | 28 official agent skills | accessibility-audit, asset-audit, bulk-cms-update, link-checker, pre-deploy-check, safe-publish, site-audit, site-activity, custom-code-management, component-audit | Skills are prompt guidance that call the MCP/CLI — they have no data layer, so every audit re-fetches and every comparison is single-shot |
| `joinflux/webflowctl` | community | webhooks only | everything else |
| `briantuckerdesign/tinySync` | community | Airtable → Webflow one-way sync | not a general CLI |
| `DAB0mB/Appfairy` | community | Webflow → React migration | not a Data API tool |
| `webflow/webflow-python` | official | full Data API SDK | library, not CLI |

## Data Layer
- **Primary entities:** sites, pages, collections, collection fields, collection items (staged + live), assets, asset folders, forms, form submissions, webhooks, redirects, custom code blocks, registered scripts, components, products, SKUs, orders, inventory, activity logs, comments.
- **Sync cursor:** `lastUpdated` on sites/collections/items; `lastPublished` on sites; `createdOn`/`lastUpdated` on pages. Items and pages both paginate with `offset`/`limit`.
- **FTS/search:** page titles + SEO descriptions + slugs; collection item field values; asset display names and filenames; form submission payloads. Cross-entity search is a genuine win — nothing in the ecosystem searches across pages, items, and assets at once.

## User Vision
> "I want to know what we can do inside the Webflow system: page publishing, content manipulation, page optimization."

All three map to real endpoints, and two of the three are entirely absent from the official CLI:
- **page publishing** → `POST /sites/{site_id}/publish` (with `customDomains` + `publishToWebflowSubdomain`), `POST /collections/{cid}/items/publish`, and the `/items/live` family. *Partially covered by the incumbent.*
- **content manipulation** → full Collections/Items CRUD incl. `POST /collections/{cid}/items/bulk`, plus `GET/POST /pages/{page_id}/dom` for page copy and `GET/POST /sites/{sid}/components/{cid}/dom` for component content. *Page DOM is not in any CLI today.*
- **page optimization** → `GET /sites/{sid}/pages` + `GET/PUT /pages/{page_id}` carry `seo.title`, `seo.description`, `openGraph`, `slug`, `isDraft`, `isMembersOnly`; plus `robots_txt`, `redirects`, `custom_code`, `google_tags`. *No CLI exposes any of this.* The *scoring/auditing* layer on top is not an API surface and must be built locally — that is transcendence work, not an absorbed row.

## Product Thesis
- **Name:** `webflow-pp-cli`
- **Why it should exist:** The official Webflow CLI stops at sites, CMS, forms, and assets. Every page-level operation a marketer or agency actually needs — SEO metadata, page copy, redirects, robots.txt, custom code — has an endpoint and no command. This CLI covers the whole 117-operation Data API, then adds the thing no Webflow tool has: a local SQLite mirror, so you can audit SEO across every page of every site offline, diff staged against live before you publish, and query content with SQL instead of paging through an API at 60 requests a minute.

## Build Priorities
1. Data layer + sync for sites, pages, collections, items (staged and live), assets, forms, redirects — the foundation every audit depends on.
2. Full absorbed surface: everything the official CLI does, plus every endpoint it skipped (pages, page DOM, SEO settings, redirects, robots.txt, custom code, webhooks, components, ecommerce, comments, activity logs).
3. Transcendence: SEO audit across pages, staged-vs-live drift, publish preview, cross-entity search, rate-limit-aware bulk operations.
4. Polish: rate-limit backoff wired to `Retry-After`, scope-aware `doctor`, honest empty-state messaging when a token lacks scopes.
