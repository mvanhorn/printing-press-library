# WordPress REST API CLI — Absorb Manifest

Sources surveyed: WP-CLI (incumbent), `sogko/go-wordpress`, `robbiet480/go-wordpress`,
`tradik/wpexporter`, `mkaz/wpsync`, `Automattic/wpgo`, `wordpress-api-client` (TS),
`@wordpress/e2e-test-utils-playwright` (155k wk downloads), `@headstartwp/core`,
`node-wp-api-client`, `@yllet/client`, `wordpress-rest-api` (wpapi line),
`wordpress-rest-api-oauth-1`, `@atomic-solutions/wordpress-api-client`,
`@integrityxd/wp-rest-api-client`, `wp-types`.

**Headline finding:** no maintained, complete WordPress REST client exists in any
language, and **no WordPress core-REST CLI exists on npm or in Go**. The two Go
libraries everyone imports were last touched in 2016 and 2018.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Posts CRUD | robbiet480 `PostsService`, WP-CLI `wp post` | `(generated endpoint) posts list/get/create/update/delete` | Typed flags from live schema, `--json`, `--dry-run`, SQLite mirror |
| 2 | Pages CRUD | robbiet480 `PagesService`, WP-CLI `wp post --post_type=page` | `(generated endpoint) pages list/get/create/update/delete` | Same |
| 3 | Media list/get/delete | robbiet480 `MediaService` | `(generated endpoint) media list/get/update/delete` | Same |
| 4 | **Media binary upload** | `wordpress-api-client.media().create(fileName, Buffer, mime)`, `e2e-utils.uploadMedia` | `wordpress-pp-cli media upload` | Generated client is JSON-only; hand-coded raw-binary sibling client with `Content-Disposition` + real MIME sniffing |
| 5 | Comments CRUD + moderation | robbiet480 `CommentsService` | `(generated endpoint) comments list/get/create/update/delete` | `status` filter (hold/spam/approved) typed from live enum |
| 6 | Users CRUD + `users/me` | robbiet480 `UsersService` | `(generated endpoint) users list/get/create/update/delete` | Handles sites that removed `/wp/v2/users` (TechCrunch does) |
| 7 | Categories CRUD | robbiet480 `CategoriesService` | `(generated endpoint) categories list/get/create/update/delete` | Same |
| 8 | Tags CRUD | robbiet480 `TagsService` | `(generated endpoint) tags list/get/create/update/delete` | Same |
| 9 | Taxonomies list/get | robbiet480 `TaxonomiesService` | `(generated endpoint) taxonomies list/get` | Returns live map, not a fixed struct — custom taxonomies survive |
| 10 | Post types list/get | robbiet480 `TypesService` | `(generated endpoint) types list/get` | Custom post types survive (both Go libs drop them) |
| 11 | Post statuses list/get | robbiet480 `StatusesService` | `(generated endpoint) statuses list/get` | Custom statuses survive |
| 12 | Settings read | robbiet480 `SettingsService.List` (read-only) | `(generated endpoint) settings get` | — |
| 13 | **Settings write** | `wordpress-api-client.siteSettings().update`, `e2e-utils.updateSiteSettings` | `(generated endpoint) settings update` | Neither Go lib can write settings at all |
| 14 | Revisions list/get/delete | robbiet480 `RevisionsService` (posts/pages only) | `(generated endpoint) post-revisions, page-revisions, block-revisions, navigation-revisions, template-revisions, template-part-revisions` | Revisions for **every** revisioned type, not just posts/pages |
| 15 | Autosaves | `wordpress-api-client.reusableBlock().autosave()` (blocks only) | `(generated endpoint) posts-autosaves, pages-autosaves, blocks-autosaves, navigation-autosaves, menu-items-autosaves, templates-autosaves, template-parts-autosaves` | Nobody covers autosaves broadly |
| 16 | Search | `wordpress-api-client.search()`, `@headstartwp` `SearchNativeFetchStrategy` | `(generated endpoint) search query` | Plus local FTS5 that beats it (see T2) |
| 17 | Reusable blocks CRUD | `wordpress-api-client.reusableBlock()` | `(generated endpoint) blocks list/get/create/update/delete` | Unknown to both Go libs |
| 18 | Block types | `wordpress-api-client.blockType()` | `(generated endpoint) block-types list/get` | Unknown to both Go libs |
| 19 | Block directory search | `wordpress-api-client.blockDirectory()` | `(generated endpoint) block-directory search` | Unknown to both Go libs |
| 20 | Server-side block render | `wordpress-api-client.renderedBlock()` | `(generated endpoint) block-renderer render` | Unknown to both Go libs |
| 21 | Block patterns + categories | `wp-types` only | `(generated endpoint) block-patterns list/categories, pattern-categories CRUD` | No client implements these |
| 22 | Pattern directory | `wp-types` only | `(generated endpoint) pattern-directory list` | No client implements this |
| 23 | Plugins list/get + install | `wordpress-api-client.plugin()`, WP-CLI `wp plugin` | `(generated endpoint) plugins list/get/create/update/delete` | REST-native; WP-CLI needs SSH |
| 24 | Plugin activate/deactivate | `e2e-utils.activatePlugin/deactivatePlugin`, WP-CLI `wp plugin activate` | `(behavior in wordpress-pp-cli plugins update) --status active\|inactive` | Over HTTPS, no SSH |
| 25 | Themes list/get | `wordpress-api-client.theme()`, WP-CLI `wp theme list` | `(generated endpoint) themes list/get` | — |
| 26 | Theme activation state (read) | `e2e-utils.activateTheme`, `claudeus_wp_theme__activate` | `(behavior in wordpress-pp-cli themes list) status field` | **Core's themes controller registers `READABLE` only — theme activation is NOT possible over REST.** Competitors that advertise `activate_theme` over stock `wp/v2` are overclaiming. We report activation state honestly and document the boundary rather than shipping a command that 404s. |
| 27 | Widgets CRUD | `e2e-utils.addWidgetBlock/deleteAllWidgets`, WP-CLI `wp widget` | `(generated endpoint) widgets list/get/create/update/delete` | No Go/npm client covers widgets |
| 28 | Widget types + encode/render | none | `(generated endpoint) widget-types list/get/encode/render` | Nobody covers this |
| 29 | Sidebars | `e2e-utils.deleteAllWidgets` (partial) | `(generated endpoint) sidebars list/get/update` | — |
| 30 | Menus CRUD | `e2e-utils.createClassicMenu`, WP-CLI `wp menu` | `(generated endpoint) menus list/get/create/update/delete` | — |
| 31 | Menu items CRUD | `e2e-utils` (partial) | `(generated endpoint) menu-items list/get/create/update/delete` | — |
| 32 | Menu locations | none | `(generated endpoint) menu-locations list/get` | Nobody covers this |
| 33 | Navigation (block menus) | `e2e-utils.createNavigationMenu` | `(generated endpoint) navigation list/get/create/update/delete` | — |
| 34 | Templates + template parts | `e2e-utils.createTemplate/deleteAllTemplates` | `(generated endpoint) templates, template-parts (CRUD + lookup)` | Includes `lookup`, which nobody exposes |
| 35 | Global styles + revisions | `e2e-utils.getThemeGlobalStylesRevisions/resetThemeGlobalStyles` | `(generated endpoint) global-styles get/update/variations/theme, global-styles-revisions list/get` | — |
| 36 | Font families / faces / collections | none | `(generated endpoint) font-families, font-faces, font-collections` | No client in any language covers fonts |
| 37 | Application password management | `wordpress-api-client.applicationPassword()` | `(generated endpoint) application-passwords list/get/create/update/delete/introspect` | Only one competitor has this; ours also *uses* app passwords as primary auth |
| 38 | Site info / root metadata | robbiet480 `BasicInfo`, `tradik.GetSiteInfo` | `(behavior in wordpress-pp-cli routes) --json` surfaces name, description, namespaces, permalink structure | Also reports plugin namespaces |
| 39 | **REST root autodiscovery** | robbiet480 `DiscoverAPI`, `wpapi.WP.discover`, yllet `discover` | `wordpress-pp-cli site add` | `HEAD` + `Link: rel="https://api.w.org/"`, HTML `<link>` fallback, **and `?rest_route=` fallback** which none of them handle |
| 40 | `X-WP-Total` / `X-WP-TotalPages` pagination | robbiet480 `populatePageValues`, `wp-api-client._countItems` | `(behavior in wordpress-pp-cli sync)` + `--limit` on reads | — |
| 41 | Auto-drain all pages | `node-wp-api-client.listAll`, `@integrityxd.fetchAllItems` | `(behavior in wordpress-pp-cli sync) --max-pages` | Nobody in Go ships this; defeats the `per_page=100` cap |
| 42 | Get by slug | `node-wp-api-client.getBySlug` | `(behavior in wordpress-pp-cli posts list) --slug` | — |
| 43 | Trash vs force delete | `wordpress-api-client` `TRASHABLE` whitelist | `(behavior in wordpress-pp-cli <resource> delete) --force` | Only posts/pages/blocks trash; everything else requires `force=true` — encoded so deletes don't silently no-op |
| 44 | `_fields` server-side projection | `node-wp-api-client`, `@headstartwp` `FilterDataOptions` | `(behavior in wordpress-pp-cli --wp-fields)` | Persistent flag on every read; cuts payload *and* server compute |
| 45 | `_embed` related resources | `@yllet/client.embed()`, `node-wp-api-client` | `(behavior in wordpress-pp-cli --embed)` | Kills the N+1 pattern |
| 46 | Basic / Application Password auth | `wordpress-api-client` `AUTH_TYPE.BASIC`, robbiet480 `BasicAuthTransport` | `(behavior in wordpress-pp-cli auth)` | Native base64 Basic; the correct 2026 default |
| 47 | Rate limiting | `tradik/wpexporter`, `jamesponddotco` | `(behavior in wordpress-pp-cli --rate-limit)` | Framework flag |
| 48 | Retry / backoff | `node-wp-api-client` `RetryConfig` | `(behavior in wordpress-pp-cli)` | Framework retry on idempotent verbs |
| 49 | Resumable / checkpointed export | `tradik/wpexporter` `*WithCheckpoint`, `--resume` | `(behavior in wordpress-pp-cli sync)` | Incremental via `modified_after` beats checkpoint files |
| 50 | Bulk content export, many formats | `tradik/wpexporter` (14 formats) | `(behavior in wordpress-pp-cli)` `--json --csv --plain --quiet --select` + `--deliver file:` | Plus SQL access to the mirror |
| 51 | Directory ↔ site sync | `mkaz/wpsync` | `(behavior in wordpress-pp-cli posts create) --stdin` + `batch run` | — |
| 52 | Concurrency control | `tradik/wpexporter` `--concurrent` | `(behavior in wordpress-pp-cli sync)` | — |
| 53 | Typed error taxonomy | `@atomic-solutions` `WordPressErrorCode` | `(behavior in wordpress-pp-cli)` typed exit codes | `rest_forbidden`, `rest_post_invalid_id`, `rest_no_route`, `rest_cannot_edit` → distinct exit codes |
| 54 | oEmbed fetch + proxy | none (core route, unimplemented everywhere) | `(generated endpoint) oembed get/proxy` | No client covers oEmbed |
| 55 | Site health checks | none | `(generated endpoint) site-health authorization-header/background-updates/https-status/loopback-requests/page-cache/dotorg-communication/directory-sizes` | No client covers site health |
| 56 | Local content mirror | `tradik` file cache (flat files) | `(behavior in wordpress-pp-cli sync)` SQLite + FTS5 | Queryable, joinable, incremental |
| 57 | MCP server | `tradik/wpexporter` `wpmcp` (read-only), `claudeus-wp-mcp` (145 tools), `RaheesAhmed` (190 tools, plugin-gated) | `(behavior in wordpress-pp-cli-mcp)` | Full read+write surface, stdio + HTTP transport, **no companion WordPress plugin required** |
| 58 | Schema-driven command generation | `wp-cli/restful` (`wp rest <resource> <verb>`, generated from `?context=help`) | `(behavior in wordpress-pp-cli routes)` + generated endpoint surface | `wp-cli/restful` needs PHP + WP-CLI and has been self-described as "an experiment" for ~10 years; ours is a static binary |
| 59 | Resource diff across environments | `wp-cli/restful` `wp rest <res> diff` | `wordpress-pp-cli drift` | Diffs against the local mirror over time, not just two live aliases |
| 60 | Edit a resource in `$EDITOR` | `wp-cli/restful` `wp rest <res> edit` | `(behavior in wordpress-pp-cli posts update) --stdin` | Agent-native (stdin/JSON) rather than TTY-bound |
| 61 | Bulk dummy content generation | `wp-cli/restful` `generate`, `wp post generate` | `(behavior in wordpress-pp-cli batch run)` | Batch-backed, honest about the 25-request cap |
| 62 | Comment approve / spam / trash shortcuts | `claudeus_wp_comments__{approve,spam,trash}`, `wp_approve_comment` | `(behavior in wordpress-pp-cli comments update) --status` | Typed enum from the live schema |
| 63 | `?rest_route=` 404 fallback | `docdyhr/mcp-wordpress` (only implementer found) | `(behavior in wordpress-pp-cli)` transport retry | Required for sites with plain permalinks; ours also probes it during `site add` |
| 64 | Multi-site configuration | `claudeus-wp-mcp` (`wp-sites.json`), `docdyhr` (`mcp-wordpress.config.json`), `InstaWP` (numbered env vars) | `wordpress-pp-cli site add/list/use` | Per-site SQLite mirror, not just per-site credentials |
| 65 | **Abilities API** (`wp-abilities/v1`) | `WordPress/mcp-adapter` (official, but a PHP plugin); `novamira` (plugin) | `(generated endpoint) abilities list/get/run/categories/category` | Core since WP 6.9 and verified live on every site sampled. **No CLI in any language exposes it.** Ours reaches it over plain HTTPS with no plugin install. |
| 66 | Site health test suite | `claudeus_wp_health__*` (8 tools) | `(generated endpoint) site-health *` | Wired into `doctor`, including the `authorization-header` test that explains stripped-header auth failures |
| 67 | URL details / link preview | `claudeus_wp_search__get_url_details` | `(generated endpoint) oembed proxy` | — |

### WP-CLI parity boundary (the incumbent)

WP-CLI is the real competitor, but it executes PHP **on the server** and needs SSH,
WP-CLI installed, and filesystem access. This CLI reaches any site over plain HTTPS
with an Application Password.

| WP-CLI group | REST-reachable? | Our coverage |
|---|---|---|
| `wp post`, `wp post-type`, `wp post-meta` | Yes | rows 1, 2, 10 |
| `wp term`, `wp taxonomy` | Yes | rows 7, 8, 9 |
| `wp user`, `wp user-meta` | Yes | row 6 |
| `wp comment` | Yes | row 5 |
| `wp media` (import/regenerate) | Partly | rows 3, 4 + `media post-process` |
| `wp option` | Partly — only registered `show_in_rest` options | row 12, 13 |
| `wp plugin`, `wp theme` | Yes (5.5+) | rows 23–26 |
| `wp menu`, `wp widget`, `wp sidebar` | Yes | rows 27–33 |
| `wp site`, `wp core` (version/update) | Read-only via root + site-health | rows 38, 55 |
| `wp db`, `wp search-replace`, `wp export/import`, `wp cron`, `wp rewrite`, `wp transient`, `wp cache`, `wp config`, `wp scaffold`, `wp eval`, `wp shell`, `wp server`, `wp package` | **No** — needs DB, filesystem, or PHP runtime | Out of scope; documented as an explicit non-goal |

The last row is the honest boundary and belongs in the README: this CLI is not a
WP-CLI replacement for server-side operations, and does not pretend to be.

## Unclaimed ground (verified gaps in every competitor)

1. **Nobody reads the live route table.** Every client hardcodes a static endpoint
   list. `wp-cli/restful` is the sole exception and it needs PHP + WP-CLI.
2. **Neither leading MCP server (`claudeus-wp-mcp` 145 tools, `docdyhr/mcp-wordpress`
   71 tools) uses `_fields`, `_embed`, or `_envelope`** — verified by source grep.
   They fetch full objects on every call. Server-side projection is free performance
   and free context savings that nobody has claimed.
3. **The Abilities API has no CLI in any language.** Core since WP 6.9; the only
   tools that reach it are WordPress plugins.
4. **No tool ships a local mirror.** `tradik/wpexporter` writes flat files;
   everything else is stateless. Cross-site and historical questions are unanswerable.
5. **Every 100+-tool competitor gets there by requiring a server-side plugin.** Pure
   `wp/v2` coverage tops out around 145 tools, and we exceed that from the spec alone.

## Security context: `batch/v1` and CVE-2026-63030

Confirmed against NVD: **CVE-2026-63030**, CVSS **9.8 critical** — a REST API batch
endpoint route-confusion issue in WordPress **6.9.0–6.9.4** and **7.0.0–7.0.1**,
which combined with CVE-2026-60137 (`author__not_in` SQL injection) enables RCE.
Patched in **6.9.5** and **7.0.2**.

Consequences for this CLI, decided deliberately:

- **`batch run` still ships.** Using the endpoint as an authenticated client against
  your own patched site is a legitimate, valuable operation, and it is the only
  transactional primitive WordPress offers.
- **It must degrade gracefully.** Hosts and WAFs are actively blocking
  `/wp-json/batch/v1` and `?rest_route=/batch/v1` on vendor advice. `batch run` must
  detect 403/404 and fall back to sequential requests rather than failing outright.
- **This turns into a feature, not just a caveat** — with an honest caveat of its own.
  Verified: the REST root does **not** carry the WordPress version. The only reliable
  vectors are the front-end `<meta name="generator" content="WordPress X.Y">` tag and
  the RSS `<generator>` element (confirmed live: `WordPress 7.1-beta2-62808` from
  both). Many hardened sites strip the generator tag, so any version-audit feature
  must report `unknown` honestly rather than guessing. It is still worth building —
  "which of my 50 sites are on a version with a known critical CVE" has no
  one-command answer today — but it cannot promise universal coverage.

### Bonus discovery: the REST root advertises the app-password authorization URL

The `/wp-json/` root includes:

```json
"authentication": {"application-passwords": {"endpoints": {"authorization":
  "https://<site>/wp-admin/authorize-application.php"}}}
```

That is a per-site deep link for minting an Application Password. `site add` can hand
the user the exact URL to click instead of describing where to find it in wp-admin.
No competitor uses this field.

## Transcendence (only possible with our approach)

7 survivors from 16 candidates after the adversarial cut. Full audit trail —
customer model, all candidates, all 9 kill reasons — in
`2026-07-21-000000-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|-------|--------------|------------------------|------------------|
| T1 | Fleet rollup | `fleet` | 10/10 | hand-code | WordPress has no concept of a fleet — there is no endpoint for "all my sites". Requires a cross-site join over per-site-keyed SQLite stores. | Use this command for a cross-site rollup of every site already in the local store. Do NOT use this command to investigate why one specific site is failing or unreachable; use 'diagnose' instead. |
| T2 | REST reachability diagnosis | `diagnose` | 10/10 | hand-code | A decision table over four probes (`HEAD` Link header, `/wp-json/` vs `?rest_route=`, anon vs authenticated, the `authorization-header` site-health test) mapping observed `code` + header presence to one named cause. Nothing else turns WordPress's most common support question into one command. | Use this command when a site's REST API is unreachable, returns 401/403, or returns rest_no_route. Do NOT use this command to find out which operations working credentials are permitted to perform; use 'caps' instead. |
| T3 | Credential capability audit | `caps` | 8/10 | hand-code | WordPress's `Allow` response header reflects what the *current* credentials may do per route. Walking the live route table with and without auth yields a full permission map with **zero mutations**. Verified live; no competitor reads this header. | Use this command to learn what the configured credentials are allowed to do on each route. Do NOT use this command when the API itself is unreachable or auth is failing outright; use 'diagnose' instead. Do NOT use it to inspect field or post-type visibility; use 'schema' instead. |
| T4 | Editorial queue with age | `queue` | 9/10 | hand-code | The API returns rows; it does not return *how long* they have been pending, bucketed, across three statuses at once. Requires local duration math over synced timestamps. | Use this command for pipeline state and how long content has been sitting in it. Do NOT use this command to find missing featured images, categories, or excerpts on content; use 'audit' instead. |
| T5 | Content hygiene audit | `audit` | 9/10 | hand-code | Four completeness checks, three of which need a join (`posts` × `media` × term links) that the REST API cannot express in any single call. | Use this command for completeness and hygiene defects on posts and pages. Do NOT use this command to find unused media files; use 'orphans' instead. Do NOT use it for workflow-state questions; use 'queue' instead. |
| T6 | Orphaned media | `orphans` | 8/10 | hand-code | WordPress has no "unused media" endpoint. Requires scanning synced content for each media item's `source_url` and cross-checking `featured_media`. | Use this command to find media files no content references. Do NOT use this command for defects on posts themselves; use 'audit' instead. |
| T7 | REST visibility gap | `schema <type>` | 9/10 | hand-code | A three-way set difference — types registry × live route table × `OPTIONS` property schema × actual synced rows — that surfaces the `show_in_rest` blind spot. Exploits the self-describing route table, the defining structural fact of this API. | Use this command to find out why a post type, field, or meta key is missing from the REST surface. Do NOT use this command to check whether your credentials may write to that type; use 'caps' instead. Do NOT use it when the whole API is failing; use 'diagnose' instead. |

### Additional hand-code required (not transcendence rows, but shipping scope)

| Feature | Command | Why it must be hand-coded |
|---|---|---|
| Multi-site targeting | `site add/list/use/remove` | `fleet` is meaningless without per-site credentials and a per-site-keyed store. The generated CLI has `WORDPRESS_BASE_URL` but no site registry. `site add` also performs `HEAD` Link-header discovery, the `?rest_route=` probe, and surfaces the app-password authorization URL from the REST root. |
| Binary media upload | `media upload <file>` | The generated client JSON-marshals every body and hardcodes `Content-Type: application/json`. WordPress media upload needs a raw binary body with `Content-Disposition: attachment; filename=` and a real MIME type. Requires a sibling client. |
| Server-side projection | `--wp-fields`, `--embed` | Persistent flags injecting `_fields` / `_embed`. Verified: neither leading MCP server uses them at all. |
