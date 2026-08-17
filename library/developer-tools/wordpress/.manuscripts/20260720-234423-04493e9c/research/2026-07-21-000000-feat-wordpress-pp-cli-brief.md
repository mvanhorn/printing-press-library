# WordPress REST API CLI Brief

## API Identity

- **Domain:** Content management. Every WordPress site (43%+ of the web) exposes a
  full read/write REST API at `/wp-json/` covering posts, pages, media, comments,
  users, taxonomies, plugins, themes, widgets, menus, block templates, global
  styles, fonts, and settings.
- **Users:** see `Users` below.
- **Data profile:** Per-site, not per-vendor. There is no single api.wordpress.com
  host — the "API" is whatever a given site's install exposes. Content volume is
  large (wordpress.org/news alone: `X-WP-Total: 1096` posts across 548 pages at
  the default `per_page=10`), and `per_page` is hard-capped at 100.

### The defining structural fact

**WordPress self-describes its entire API surface at runtime.** `GET /wp-json/`
returns a route table with, for every route and method, a typed arg schema
(`type`, `enum`, `default`, `required`, `format`, `description`). `OPTIONS
/wp/v2/posts` returns the full 29-property resource schema.

The surface **differs per site**, and not trivially:

| Site | total routes | `/wp/v2` routes | `/wp/v2/posts` GET args | `/wp/v2/users` |
|---|---|---|---|---|
| wordpress.org/news | 527 | 148 | 27 | present |
| developer.wordpress.org | 614 | 240 | 27 | present |
| techcrunch.com | 614 | 193 | **36** | **removed** |

TechCrunch adds `yoast/v1`, `coauthors/v1`, `elasticpress/v1`, `apple-news/v1`,
`tc/v1` namespaces and 9 extra `posts` args, while deleting `/wp/v2/users`
entirely. Every existing WordPress client hardcodes a static endpoint list from
the docs, so all of them are simultaneously **wrong about what a given site
actually offers** — they miss the plugin namespaces and they offer commands that
404. Nothing on the market reads the live route table.

## Users

1. **Agency maintainer running 5–50 client sites.** Cheap shared hosts, no SSH on
   most of them, so WP-CLI is unavailable. Logs into wp-admin one site at a time.
2. **Headless / JAMstack developer.** Next.js or Astro front end pulling WP
   content. Lives in `/wp-json/`, constantly debugging why a custom post type or
   ACF field isn't showing up in REST (`show_in_rest`).
3. **Content ops / editorial manager.** Owns the publishing queue: drafts stuck in
   pending, scheduled posts, taxonomy hygiene, orphaned media.
4. **Agent / automation builder.** Wants an AI agent or cron script to publish and
   query WordPress with an Application Password, not a browser session.

## Top Workflows

1. **Bulk publish / schedule / update** posts and pages from files or a queue.
2. **Content audit** — find drafts rotting in pending, posts with no featured
   image, orphaned media, uncategorized posts.
3. **Fleet check across many sites** — WP version, pending plugin/theme updates,
   REST health, admin user inventory.
4. **Comment moderation triage** — work the hold/spam queue.
5. **Diagnose a broken REST API** — "why is this site returning 401 / 403 /
   `rest_no_route`". Extremely common and currently a manual slog.
6. **Mirror a site locally** for search, analysis, or migration.

## Reachability Risk

**Measured, not assumed.** A probe of 39 confirmed-WordPress sites:

| Outcome | Count | % |
|---|---|---|
| `/wp-json/` served JSON directly | 25 | **64%** |
| `/wp-json/` failed, `?rest_route=` worked (path-based block) | 7 | **18%** |
| App-layer block (JSON error on both forms) | 3 | 8% |
| Edge block (HTML/403/challenge on both) | 4 | 10% |
| Advertised the REST root via `Link`/`<link rel>` | 28 | **72%** |

**~36% of live WordPress sites did not answer `/wp-json/wp/v2/posts` on the first
try.** 18% were recoverable purely by switching to `?rest_route=`. 28% do not
advertise the REST root at all, so *absence of the discovery link proves nothing*.

That table is the entire justification for the flagship `diagnose` command, and it
means the `?rest_route=` fallback is a correctness requirement, not a nicety.

- **Low for the API itself.** Verified live: `GET /wp/v2` → 200 on three
  independent sites; `GET /wp/v2/settings` unauthenticated → clean `401
  {"code":"rest_forbidden"}`; bad ID → `404 {"code":"rest_post_invalid_id"}`.
  Well-formed JSON error envelopes throughout — good typed-exit-code material.
- **Wordfence (5,000,000+ installs) disables Application Passwords by default**
  (`loginSec_disableApplicationPasswords = true` →
  `add_filter('wp_is_application_passwords_available','__return_false')`). The most
  common auth failure on the platform is therefore not a bad password — it is auth
  being switched off site-side. `diagnose` must name this.
- **`rest_url_prefix` can be renamed** (WP Ghost 100K+ installs, WP Hide). Hardcoding
  `/wp-json` guarantees false negatives; the advertised root must be read first.
- **Status codes are ambiguous by design.** `rest_authorization_required_code()`
  returns 403 when logged in and 401 when not, so the *same* failure changes status
  with credentials. Key all logic on the `code` field, never the status alone.
- **Moderate at the individual-site level, and this is a product opportunity, not
  a blocker.** Security plugins (Wordfence, Solid Security, All-In-One WP
  Security) can restrict `/wp-json` to logged-in users; corrupted permalinks
  produce `rest_no_route`; many Apache/CGI hosts **strip the `Authorization`
  header**, silently breaking Application Passwords. WordPress ships a site-health
  test for exactly that last case (`/wp-site-health/v1/tests/authorization-header`),
  which is in the spec and should be wired into `doctor`.
- **Non-pretty-permalink sites** serve the API at `?rest_route=/wp/v2/posts`
  instead of `/wp-json/...`. Verified: returns 200 on wordpress.org/news. A client
  that only knows `/wp-json/` breaks on these sites. Discovery via
  `HEAD` + `Link: <...>; rel="https://api.w.org/"` — verified present.
- Tier/permission hints from 4xx body: `{"code":"rest_forbidden","message":"Sorry,
  you are not allowed to do that.","data":{"status":401}}`
- Probe-safe endpoint used: `GET /wp/v2` and `GET /wp/v2/posts?per_page=1`

## Contract Details (verified live, not from docs summaries)

- **Pagination:** `page`, `per_page` (1–100 hard cap), `offset`. Responses carry
  `X-WP-Total` and `X-WP-TotalPages` plus `Link: rel="next"`.
- **Server-side field selection:** `_fields=id,title,link` — verified; supports
  nested paths (`meta.key.nested`). Cuts payload *and* server-side computation.
- **Embedding:** `_embed=1` inlines `author` and `wp:term` — verified. Kills the
  N+1 request pattern.
- **Other globals:** `_envelope`, `_method` / `X-HTTP-Method-Override`, `_jsonp`.
- **Capability probing:** the `Allow` response header reflects what the *current
  credentials* may do on that route (`Allow: GET` unauthenticated). A read-only
  permission audit is possible with zero mutations.
- **Auth:** Application Passwords over HTTP Basic — `Authorization: Basic
  base64(user:app_password)`. Confirmed as the correct modern default. Cookie+nonce
  is browser-only and irrelevant to a CLI.
- **Incremental sync key:** `modified_after` (ISO 8601) exists on posts/pages —
  a real incremental cursor, not a full re-pull.

## Table Stakes

Competing tools and everything they offer (full enumeration lives in the absorb
manifest). Landscape summary:

- **WP-CLI** — the real incumbent. Enormous command surface, but it runs *on the
  server* via PHP and needs SSH or a local install. Unavailable on most shared
  hosting and unusable against a remote site you only have an app password for.
- **Go ecosystem — effectively abandoned.** The two libraries everyone imports,
  `sogko/go-wordpress` (141★, last commit **2016**) and `robbiet480/go-wordpress`
  (19★ but 17 importers, last commit **2018**), both target the WP 4.x plugin era.
  Neither has a `go.mod` or a tagged release. Between them they miss `search`,
  `blocks`, `block-types`, `plugins`, `themes`, `widgets`, `sidebars`, `menus`,
  `menu-items`, `templates`, `template-parts`, `global-styles`, `font-families`,
  `pattern-directory`, `autosaves`, and `application-passwords` — i.e. everything
  added since 2018. Both hardcode `/wp-json/wp/v2` with no namespace support and
  no `?rest_route=` fallback. Both model statuses and post types as **fixed
  structs**, so custom post types are silently dropped. sogko's `Update()` and
  `Delete()` are outright broken against modern WP (they send the PHP `$_SERVER`
  variable name `HTTP_X_HTTP_METHOD_OVERRIDE` instead of the wire header).
- **`tradik/wpexporter`** (2★, active 2026-07) — the most modern Go WP REST CLI
  and the best UX reference: `--resume`, `--concurrent`, `--rate-limit`,
  `--path-filter`, checkpointed exports, 14 output formats, ships an MCP binary.
  **Read-only exporter** — no writes at all.
- **`mkaz/wpsync`** (10★, archived 2019) — directory↔WP sync, JWT auth.
- **`Automattic/wpgo`** (26★, 2015) — WordPress.com/Jetpack API, not `wp/v2`.

## Data Layer

- **Primary entities:** posts, pages, media, comments, users, categories, tags,
  plugins, themes, settings, revisions, menus/menu-items, templates.
- **Sync cursor:** `modified_after` for posts/pages/media; `X-WP-Total` /
  `X-WP-TotalPages` for progress and completeness assertions.
- **FTS/search:** title + content + excerpt + slug across post types. WordPress's
  own `/wp/v2/search` is shallow (id/title/url/type only) and can't do boolean or
  regex; local FTS5 beats it outright.
- **Per-site isolation is mandatory.** The store must be keyed by site host or
  syncing site B over site A silently corrupts the mirror. This is a P0
  correctness requirement, not a nicety.

## Product Thesis

- **Name:** `wordpress-pp-cli`
- **Why it should exist:** WP-CLI is the power tool but demands SSH. The REST API
  reaches any site from anywhere with an app password, but every existing client
  is either abandoned, read-only, or hardcoded to a 2018 view of the API. This CLI
  is **WP-CLI-grade control over plain HTTPS, with no SSH** — plus two things
  neither WP-CLI nor any REST client has: it **adapts to the live route table of
  whatever site you point it at**, and it keeps a **local SQLite mirror** so
  cross-site and historical questions become one query instead of 548 paginated
  requests.

## Build Priorities

1. **Per-site store isolation + `--site` targeting.** Nothing else is correct
   without it.
2. **Full core surface** — 50 resources / 166 typed endpoints from the live schema.
3. **Runtime route discovery** — the feature no competitor has.
4. **Incremental sync via `modified_after`** + FTS5 search over the mirror.
5. **REST health diagnosis** — turn the most common WordPress support question
   into one command.
