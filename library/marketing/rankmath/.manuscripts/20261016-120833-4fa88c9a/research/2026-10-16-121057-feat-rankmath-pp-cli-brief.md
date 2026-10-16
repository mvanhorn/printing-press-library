# Rank Math CLI Brief

## API Identity
- Domain: Rank Math SEO (WordPress plugin, free + Pro). No hosted API; every site exposes its own REST namespace `rankmath/v1` (+ sub-namespaces `an` analytics, `in` instant indexing, `status`, `link-genius`, `ai-visibility`, `setupWizard`, `links`) under `<site>/wp-json/`.
- Users: WordPress site owners, SEO operators, agencies running many WP sites, AI agents doing content ops.
- Data profile: per-post SEO meta (title, description, focus keyword, robots, canonical, schema), redirections, GSC-backed analytics rows (posts, keywords, rank tracking, URL inspection), internal link graph + audits (Link Genius, Pro), instant-indexing log.
- Spec source: none published. Contract derived from (a) the live public route index `GET /wp-json/` on example.com and example.org (109 rankmath routes each, args included; saved under discovery/) and (b) plugin source github.com/rankmath/seo-by-rank-math `includes/rest/*.php`, `includes/modules/*/class-rest.php`.

## Reachability Risk
- Low. Route index is public; every data route returns 401 `rest_forbidden` without auth (verified `an/dashboard` → 401). Auth = WordPress Application Password via HTTP Basic (core WP, no plugin needed).
- Per-site variance: Pro-only modules (link-genius, ai-visibility, rank tracking) and opt-in features (headless `getHead` requires General → Others → Headless support; OFF on both probed sites → 404 rest_no_route). CLI must detect available routes from `/wp-json/` and degrade per site.
- Capability checks per route (`Helper::has_cap('onpage_general'|'onpage_snippet'|'redirections'...)`, `edit_post`).
- Rank Math meta is NOT exposed in `wp/v2/posts.meta` (verified on example.com) → reading current SEO meta needs rendered-HTML head parsing or analytics rows.

## Top Workflows
1. Bulk SEO meta edits: set title/description/focus keyword/robots/canonical for posts (`POST updateMeta` {objectType, objectID, meta}; `POST updateMetaBulk` {rows}).
2. Audit what search engines see: fetch rendered head (title, meta description, canonical, robots, OG, JSON-LD schema) per URL and diff against intent; find missing/duplicate titles & descriptions.
3. Analytics from the terminal: dashboard, posts rows, keyword rows, tracked keywords, URL inspection results (`an/*`, GET with paging/date params).
4. Instant indexing: submit URLs to IndexNow (`in/submitUrls`), read/clear log.
5. Redirect a post (`updateRedirection`), update schema (`updateSchemas`), internal link audit/stats (`link-genius/*`, Pro).

## Table Stakes
- Competitors: Easy MCP AI Rank Math tools (wp_rm_* incl. get_head), Airano MCP SEO Bridge plugin, Devora rank-math-api-manager plugin, generic mcp-wordpress (auto-detects Rank Math meta when show_in_rest is registered). All need an extra server-side plugin or headless toggle; none cover analytics/indexing/link-genius.
- Table stakes: auth via app password, multi-site profiles, update meta single + bulk, get head/meta for URL, submit to IndexNow, analytics summaries, JSON output.

## Data Layer
- Primary entities: posts (id, link, type, modified from wp/v2), seo_snapshots (url → title/desc/canonical/robots/schema types, fetched_at), analytics posts/keyword rows, index log.
- Sync cursor: wp/v2 `modified_after`; analytics by date range.
- FTS/search: post titles + SEO titles/descriptions for offline "find posts missing focus keyword / duplicate title".

## Product Thesis
- Name: rankmath-pp-cli
- Why it should exist: Rank Math's REST surface is undocumented and only used by its React admin. A CLI with app-password auth reaches it without installing bridge plugins, adds offline SEO auditing from rendered HTML, and gives agents safe bulk meta edits with dry-run.

## Build Priorities
1. Auth/profile + route discovery (`doctor`: which rankmath modules exist on this site).
2. `meta set` / `meta bulk` (updateMeta, updateMetaBulk) with --dry-run; `head get <url>` (rendered HTML parse; uses getHead when enabled).
3. Analytics read commands (an/dashboard, postsRows, keywordsRows, getTrackedKeywords, inspectionResults).
4. Instant indexing submit/log; redirection + schema update.
5. Offline audit: sync posts + head snapshots → missing/duplicate title/description, noindex drift.
