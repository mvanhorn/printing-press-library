# Rank Math CLI Absorb Manifest

## Absorb Manifest

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Set SEO title/description/focus keyword/canonical/robots on one post/product/term | Devora rank-math-api-manager; Bulk SEO Meta Editor for AI Agents; Airano MCP SEO Bridge | (generated endpoint) meta update-meta | No server-side bridge plugin needed (uses Rank Math's own rankmath/v1/updateMeta); --dry-run, --json |
| 2 | Bulk SEO meta update (many posts per call) | Bulk SEO Meta Editor for AI Agents (100/call) | (generated endpoint) meta update-meta-bulk | Native updateMetaBulk, --stdin rows, dry-run |
| 3 | CSV import of SEO meta | Bulk SEO Meta Editor; WP Meta Importer; techearl WP-CLI sheet script | rankmath-pp-cli meta import | CSV → updateMetaBulk, change-only diff vs current rendered head, dry-run default |
| 4 | Read Rank Math head tags for a URL | Easy MCP AI wp_rm_get_head | rankmath-pp-cli head get | Works even when headless support is OFF (falls back to parsing rendered HTML); JSON fields |
| 5 | Set a redirect on a post | Rank Math admin sidebar | (generated endpoint) redirections update-redirection | Scriptable |
| 6 | Update schema on a post | Rank Math schema builder | (generated endpoint) schema update-schemas | Scriptable, --stdin JSON |
| 7 | Analytics dashboard / posts / keywords rows | Rank Math Analytics admin (Pro) | (generated endpoint) gsc dashboard, posts-rows, keywords-rows, posts-overview, keywords-overview, analytics-summary | Terminal + JSON + local store |
| 8 | Rank tracking: list/add/remove tracked keywords | Rank Math Pro rank tracker | (generated endpoint) gsc get-tracked-keywords, add-track-keyword, remove-track-keyword | Scriptable |
| 9 | URL inspection results / stats | Rank Math Analytics index status | (generated endpoint) gsc inspection-results, inspection-stats | JSON |
| 10 | Instant indexing submit URLs / log / clear | Rank Math Instant Indexing (IndexNow) | (generated endpoint) indexing submit-urls, get-log, clear-log | Batch from stdin |
| 11 | Internal links: posts/link stats, audit, suggestions | Rank Math Link Genius (Pro) | (generated endpoint) link-genius links, posts-stats, audit-stats, link-suggestions | JSON |
| 12 | Export/import settings, run backup | Rank Math Status & Tools | (generated endpoint) backup export-settings, run-backup | Scriptable backups |
| 13 | Enable/disable modules, update settings | Rank Math dashboard | (generated endpoint) settings save-module, update-settings | dry-run |
| 14 | PageSpeed / page SEO score for a post | Rank Math analytics | (generated endpoint) gsc get-desktop-pagespeed, get-mobile-pagespeed, get-page-seo-score | JSON |
| 15 | Multi-site management | mcp-wordpress (multi-site config) | (behavior in rankmath-pp-cli doctor modules) multi-site via --sites / RANKMATH_SITES; reports which Rank Math modules each site exposes | Route discovery from /wp-json/ |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Long Description | Persona |
|---|---|---|---|---|---|---|---|
| 1 | SEO head audit | audit | 10/10 | hand-code | wp/v2 inventory + rendered-head parse (getHead when enabled) stored as seo_snapshots; missing/duplicate title+description, missing canonical, noindex, missing JSON-LD, change since previous snapshot | Use this command to audit on-page SEO tags across all synced posts. Do NOT use this command for internal-link audits; use 'link-genius' audit commands instead. Do NOT use it for Google index status; use 'coverage' instead. | Maya, Agent |
| 2 | Striking-distance opportunities | opportunities | 7/10 | hand-code | GSC keyword/post rows joined to seo_snapshots: queries at position 4-20 whose ranking post's title/description lacks the query | none | Maya, Agent |
| 3 | Index coverage gaps | coverage | 8/10 | hand-code | inventory + an/inspectionResults + snapshot robots: published URLs not indexed / never inspected, excluding intentional noindex | Use this command to find published posts Google has not indexed. Do NOT use this command to push recently edited URLs; use 'reindex' instead. | Maya, Dev |
| 4 | Reindex changed posts | reindex | 8/10 | hand-code | wp/v2 modified_after minus in/getLog → in/submitUrls; dry-run default, --confirm to send | Use this command to submit recently modified posts to IndexNow. Do NOT use this command to find unindexed posts; use 'coverage' instead. | Maya, Agent |
| 5 | Portfolio rollup | portfolio | 7/10 | hand-code | an/analyticsSummary across every configured site profile, skipping sites lacking the module | none | Dev |
