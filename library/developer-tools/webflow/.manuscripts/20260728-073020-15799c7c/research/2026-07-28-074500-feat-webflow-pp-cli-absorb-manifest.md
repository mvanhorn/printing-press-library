# Webflow CLI — Absorb Manifest

Run: 20260728-073020-15799c7c · Spec: official Webflow OpenAPI 3.1.0, 117 operations, 16 tags.

Tools surveyed: `@webflow/webflow-cli` v2.2.0 (official, 322k dl/mo), `webflow-api` npm SDK v3.3.4 (official, 476k dl/mo), `webflow/mcp-server` (135★, official MCP), `webflow/webflow-skills` (106★, 28 official agent skills), `webflow/webflow-python`, `joinflux/webflowctl`, `briantuckerdesign/tinySync`, `DAB0mB/Appfairy`.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List sites | @webflow/webflow-cli `sites list` | (generated endpoint) sites list | --json/--select/--csv, offline from local store |
| 2 | Get site detail | @webflow/webflow-cli `sites get` | (generated endpoint) sites get | offline, typed exit codes |
| 3 | List custom domains | @webflow/webflow-cli `sites domains` | (generated endpoint) sites custom-domains | needed to target a publish |
| 4 | Publish site | @webflow/webflow-cli `sites publish` | (generated endpoint) sites publish | --dry-run, domain targeting |
| 5 | List collections | @webflow/webflow-cli `cms collections list` | (generated endpoint) collections list | offline, sortable |
| 6 | Get collection schema | @webflow/webflow-cli `cms collections get` | (generated endpoint) collections get | field types cached locally |
| 7 | Create collection | @webflow/webflow-cli `cms collections create` | (generated endpoint) collections create | --dry-run |
| 8 | Delete collection | webflow-api SDK | (generated endpoint) collections delete | typed exit codes |
| 9 | Create collection field | @webflow/webflow-cli `cms fields create` | (generated endpoint) collections fields create | --dry-run |
| 10 | Update / delete collection field | webflow-api SDK | (generated endpoint) collections fields update / delete | not in the official CLI |
| 11 | List items | @webflow/webflow-cli `cms items list` | (generated endpoint) items list | offline, SQL-queryable |
| 12 | Get / create / update / delete item | @webflow/webflow-cli `cms items *` | (generated endpoint) items get / create / update / delete | --dry-run, --stdin |
| 13 | Publish items | @webflow/webflow-cli `cms items publish` | (generated endpoint) items publish | batch |
| 14 | Bulk create items | webflow-api SDK / webflow-skills bulk-cms-update | (generated endpoint) items bulk-create | rate-limit aware |
| 15 | Live (published) item CRUD | webflow-api SDK | (generated endpoint) items list-live / create-live / update-live / delete-live | staged-vs-live made explicit |
| 16 | List forms | @webflow/webflow-cli `forms list` | (generated endpoint) forms list | offline |
| 17 | List / export form submissions | @webflow/webflow-cli `forms submissions --output` | (behavior in webflow-pp-cli forms submissions) --csv plus offline FTS over submission payloads | search across submissions, not just export |
| 18 | Get / update / delete a submission | webflow-api SDK | (generated endpoint) form-submissions get / update / delete | not in the official CLI |
| 19 | List assets | @webflow/webflow-cli `assets list` | (generated endpoint) assets list | offline inventory |
| 20 | Upload asset | @webflow/webflow-cli `assets upload` | (generated endpoint) assets create | --dry-run |
| 21 | Update / delete asset | @webflow/webflow-cli `assets update` | (generated endpoint) assets update / delete | delete not in the official CLI |
| 22 | Asset folders list / create | @webflow/webflow-cli `assets folders` | (generated endpoint) asset-folders list / create / get | |
| 23 | Custom fonts CRUD | webflow-api SDK | (generated endpoint) custom-fonts list / get / create / update / delete | not in any CLI |
| 24 | List pages | webflow/mcp-server `data_pages_tool` | (generated endpoint) pages list | **not in the official CLI at all** |
| 25 | Get page settings + SEO metadata | webflow/mcp-server `data_pages_tool` | (generated endpoint) pages get | **not in any CLI** |
| 26 | Update page settings + SEO metadata | webflow/mcp-server `data_pages_tool` | (generated endpoint) pages update | **not in any CLI**, --dry-run |
| 27 | Read page DOM (text content) | webflow/mcp-server `data_pages_tool` | (generated endpoint) pages dom get | **not in any CLI** |
| 28 | Write page DOM (text content) | webflow/mcp-server `data_pages_tool` | (generated endpoint) pages dom update | **not in any CLI**, --dry-run |
| 29 | List components | webflow/mcp-server `data_components_tool` | (generated endpoint) components list | |
| 30 | Component DOM read / write | webflow/mcp-server `data_components_tool` | (generated endpoint) components dom get / update | |
| 31 | Component properties read / write | webflow/mcp-server `data_components_tool` | (generated endpoint) components properties get / update | |
| 32 | Site redirects CRUD | Webflow Data API (no CLI exists) | (generated endpoint) sites redirects list / create / update / delete | **not in any CLI** |
| 33 | robots.txt read / replace / patch / delete | Webflow Data API (no CLI exists) | (generated endpoint) sites robots-txt get / replace / update / delete | **not in any CLI** |
| 34 | Site + page custom code | webflow-skills custom-code-management | (generated endpoint) custom-code sites get/update/delete and custom-code pages get/update/delete | **not in any CLI** |
| 35 | Registered scripts (hosted + inline) | webflow-skills custom-code-management | (generated endpoint) registered-scripts list / create-hosted / create-inline | |
| 36 | Webhooks CRUD | joinflux/webflowctl | (generated endpoint) webhooks list / create / get / delete | webflowctl does only this |
| 37 | Ecommerce products + SKUs | webflow-api SDK | (generated endpoint) products list / create / get / update, products skus create / update | not in any CLI |
| 38 | Orders lifecycle | webflow-api SDK | (generated endpoint) orders list / get / update / fulfill / unfulfill / refund | not in any CLI |
| 39 | Inventory read / update | webflow-api SDK | (generated endpoint) inventory get / update | |
| 40 | Ecommerce settings | webflow-api SDK | (generated endpoint) ecommerce settings | |
| 41 | Comment threads + replies | webflow/mcp-server comments | (generated endpoint) comments list / get / replies | |
| 42 | Site activity logs | webflow-skills site-activity | (generated endpoint) sites activity-logs | |
| 43 | Workspace audit logs | Webflow Data API | (generated endpoint) workspaces audit-logs | |
| 44 | Google Tags integration | Webflow Data API | (generated endpoint) sites google-tags get / update / delete | |
| 45 | Site plan | Webflow Data API | (generated endpoint) sites plan | reveals the rate-limit tier |
| 46 | Token introspection / authorized-by | webflow-api SDK | (generated endpoint) token introspect / authorized-by | scope debugging |
| 47 | Auth token setup + status | @webflow/webflow-cli `auth login` | (behavior in webflow-pp-cli auth) `auth set-token`, `auth status`, WEBFLOW_API_TOKEN | |
| 48 | Health check / connectivity | webflow-skills webflow-cli-troubleshooter | (behavior in webflow-pp-cli doctor) verifies token, reports missing scopes, rate-limit tier, local DB freshness | scope-aware; the incumbent has no doctor |
| 49 | Column selection on lists | @webflow/webflow-cli `--fields` | (behavior in webflow-pp-cli sites list) `--select` on every command | works everywhere, not just lists |
| 50 | JSON output | @webflow/webflow-cli `--json` | (behavior in webflow-pp-cli sites list) `--json` / `--agent` / `--compact` / `--csv` globally | |
| 51 | Offline full-text search | none in the ecosystem | (behavior in webflow-pp-cli search) FTS5 across pages, items, assets, submissions | nothing in the ecosystem has this |
| 52 | Raw SQL over synced data | none in the ecosystem | (behavior in webflow-pp-cli sql) SELECT-only SQLite access | nothing in the ecosystem has this |
| 53 | Incremental sync | none in the ecosystem | (behavior in webflow-pp-cli sync) `--resources` / `--since` / `--full` | nothing in the ecosystem has this |

**Stubs:** none. Every row above ships fully implemented.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Persona | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|---------|------------------------|------------------|
| 1 | Page SEO audit | `seo audit <site-id>` | hand-code | 10/10 | Nadia (SEO consultant) | Reads the local `pages` table and applies mechanical rules to `seo.title`, `seo.description`, `openGraph`, `slug`, `isDraft` — missing, duplicated (self-join on the same site), over the SERP truncation length. The API has no audit endpoint; the official CLI has no pages surface at all; `webflow-skills` `site-audit` has no data layer so every comparison is single-shot. | Use this command to score every page's SEO metadata on one site, including missing, duplicated, and over-length `seo.title` and `seo.description`. Do NOT use this command for a pre-publish summary of what would change; use 'publish preview' instead. Do NOT use it for CMS collection field gaps; use 'collections completeness' instead. |
| 2 | Staged-vs-live drift | `drift <collection-id>` | hand-code | 10/10 | Marcus (content ops) | Joins the local staged-items table against the local live-items table and diffs them field by field. The API exposes `/items` and `/items/live` separately with no comparison helper, and no tool in the ecosystem stores both to compare. | Use this command to compare staged and live CMS items field by field for one collection. Do NOT use this command for a whole-site summary of everything a publish would change; use 'publish preview' instead. |
| 3 | Publish preview | `publish preview <site-id>` | hand-code | 9/10 | Priya (agency lead), Ari (CI agent) | Joins local `sites`, `pages`, staged/live `items`, and `redirects` to report pages modified since `sites.lastPublished`, unpublished item counts per collection, draft pages, and pending redirects. `sites publish` reports nothing about pending changes. | Use this command to see everything that would change if you published this site right now. Do NOT use this command for a field-level comparison of one collection's staged and live items; use 'drift' instead. Do NOT use it for a rollup across every site you sync; use 'overview' instead. |
| 4 | Query-driven bulk field set | `items bulk-set <collection-id>` | hand-code | 8/10 | Marcus (content ops) | Selects matching items from the local items table via repeatable `--match field=value` pairs (exact equality, AND-combined), prints the change set (dry-run by default), then PATCHes upstream paced against `X-RateLimit-Remaining` and `Retry-After`. `POST /items/bulk` is create-only — there is no bulk-PATCH endpoint to wrap. | Use this command to apply the same field value to many CMS items selected by a condition from the local mirror. Do NOT use this command to check what a publish would change before running it; use 'publish preview' instead. Do NOT use it to push the edited items live; use the generated 'items publish' command instead. |
| 5 | Collection field completeness | `collections completeness <collection-id>` | hand-code | 8/10 | Marcus (content ops) | Joins the local collection-fields schema against every synced item's field values for per-field fill rate, required-but-empty counts, and 100%-empty dead fields. No CLI or SDK exposes any coverage view over a collection. | Use this command to measure per-field fill rates and required-but-empty fields inside one CMS collection. Do NOT use this command for page SEO metadata gaps; use 'seo audit' instead. |
| 6 | Redirect table audit | `redirects audit <site-id>` | hand-code | 7/10 | Priya, Nadia | Joins the local `redirects` table against local page slugs and CMS item slugs to flag redirects shadowing a live page, targets matching no known page or item, and duplicate or looping sources. Redirects CRUD exists in the Data API and in no CLI at all. | Use this command to validate a site's redirect table against its known page and CMS item slugs. Do NOT use this command to create, edit, or list redirects; use the generated 'sites redirects' commands instead. Do NOT use it for page SEO metadata problems; use 'seo audit' instead. |
| 7 | Multi-site rollup | `overview` | hand-code | 7/10 | Priya (agency lead) | Aggregates local `sites`, `pages`, `collections`, and staged/live `items` into one row per synced site — page count, collection count, unpublished item count, SEO finding count, days since last publish. A workspace token spans every site, but no endpoint returns a cross-site aggregate. | Use this command for a one-row-per-site rollup across every site in the local mirror. Do NOT use this command to inspect what a single site's next publish would change; use 'publish preview' instead. Do NOT use it for page-level SEO findings on one site; use 'seo audit' instead. |

**Hand-code commitment: 7 of 7 transcendence rows are `hand-code`** (~50-150 LoC each plus `root.go` wiring). Zero are `spec-emits`.

Killed candidates and the full customer model live in `2026-07-28-074500-novel-features-brainstorm.md`.
