# Placeit CLI — Absorb Manifest

## Architecture note
Placeit has no OpenAPI spec. Two replayable surfaces drive the CLI:
- **Algolia** (`Stage_production`, public search key, no auth, no Cloudflare) — backs search/browse/template/facets/industries/sync. Hand-built `internal/algolia` client (POST + facet filters + pagination).
- **placeit.net /api** (cookie session, Cloudflare → Surf transport) — backs account/bookmarks/campaigns. Modeled in the internal YAML spec for generation; cookie auth via `auth login --chrome`.

The data layer (local SQLite mirror of the catalog) is populated by `sync` pulling from Algolia, enabling offline FTS + the transcendence joins below.

## Absorbed (match or beat every Placeit-UI + competitor feature)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword catalog search | Placeit UI / Algolia | `placeit-pp-cli search <query>` | Offline FTS over synced mirror, regex, `--json`/`--select`, SQL-composable |
| 2 | Filter by category | Placeit UI | `(behavior in placeit-pp-cli search) --category mockups\|logos\|videos\|designs` | Combinable with every other filter in one call |
| 3 | Filter by template type | Placeit UI | `(behavior in placeit-pp-cli search) --type image\|blender\|video\|multi-stage` | Not exposed as a UI filter |
| 4 | Filter by attribute tags | Placeit UI | `(behavior in placeit-pp-cli search) --tag <device/gender/age/ethnicity/color>` | Multi-facet AND/OR in one query |
| 5 | Sort newest / best-selling / free | Placeit UI replicas | `(behavior in placeit-pp-cli search) --sort newest\|best-selling\|free` | Backed by Algolia replicas + local `purchases` |
| 6 | Template detail + deep link + thumbnails | Placeit UI | `placeit-pp-cli template <id\|slug>` | Returns stage_link, editor_link, all thumbnail URLs as JSON |
| 7 | Browse facet distributions | Placeit UI | `placeit-pp-cli facets [--facet <name>]` | Whole-catalog counts the UI never totals |
| 8 | List categories | Placeit UI | `placeit-pp-cli categories` | With live counts |
| 9 | Industry taxonomy browse | Placeit UI | `placeit-pp-cli industries` | 152-entry taxonomy, searchable |
| 10 | Free templates only | Placeit UI | `(behavior in placeit-pp-cli search) --free` | `is_free` filter, combinable |
| 11 | Saved/bookmarked templates | Placeit UI (auth) | `placeit-pp-cli bookmarks` | Cookie auth; joinable to local mirror |
| 12 | Account + subscription status | Placeit UI (auth) | `placeit-pp-cli account` | One call: plan, status, subscription type |
| 13 | Active campaigns/promos | Placeit `get_active_campaigns` | `placeit-pp-cli campaigns` | No-auth promo feed |
| 14 | Open/resolve a template | Placeit UI download | `placeit-pp-cli open <stage> [--launch]` | Deep-link + editor-link resolver (download/render is browser-gated; honest) |
| 15 | Local catalog cache / offline | (none — net new) | `placeit-pp-cli sync [--category --max-pages]` | Full 164k-template offline mirror with FTS |
| 16 | Brand-kit / saved colors (competitor: Looka/Canva) | competitor table-stakes | `(stub) requires editor session — out of scope v1` | Honestly stubbed; editor render not replayable |

## Transcendence (only possible with our local-SQLite approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Best-seller rank within a niche | `top <query> [--category --device]` | spec-emits | Sorts the local mirror by real `purchases` count — Placeit's UI never exposes popularity as a sort | none |
| 2 | Printify POD-ready filter + export | `pod <query>` | hand-code | Filters `is_printify=1` + emits deep/editor/thumbnail links for a POD listing pipeline; UI has no POD-only filter | Use for finding Printify-compatible mockups for a POD listing pipeline. To rank any mockup by popularity regardless of POD, use 'top'. |
| 3 | Twitch matched-kit builder | `kit <style\|bundle>` | hand-code | Joins mirror on shared `bundle_tags`/style family; reports overlay/panel/emote/frame slots present vs missing | Use to assemble a streamer asset set from one style family. |
| 4 | Industry taxonomy map with counts | `industry-map [<industry>]` | hand-code | Joins the 152-entry taxonomy against the mirror for a tree with per-node template counts | Use to navigate the industry taxonomy with counts. The framework 'industries' lists entries flat with no counts. |
| 5 | Saved search / watchlist + new-since-sync diff | `watch add <query>` / `watch run` | hand-code | Persists named queries; diffs current mirror matches vs ids/published_date at last run — no Algolia call gives a time-windowed delta | Use for recurring niche monitoring across syncs. |
| 6 | Cross-facet gap finder | `gaps --facet <a> --by <b>` | hand-code | Local SQLite pivot across two tag-array facets to surface under-served mockup combinations | none |
| 7 | Template popularity percentile | `rank <id>` | hand-code | Computes a template's `purchases` percentile within its category + tag cohort from local stats | Use to judge one template's popularity in context. To rank a whole result set, use 'top'. |

Transcendence hand-code count: **6 hand-code** (pod, kit, industry-map, watch, gaps, rank) + 1 spec-emits/search-variant (top).
