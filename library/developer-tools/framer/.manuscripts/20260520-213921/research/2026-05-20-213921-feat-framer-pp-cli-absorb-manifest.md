# Framer CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Get project info | framer-api, framer-design-mcp | `framer project info` | --json, --select, cached in SQLite |
| 2 | Get current user | framer-design-mcp | `framer project user` | Structured output, multi-project aware |
| 3 | List pages | framer-design-mcp | `framer pages list` | Offline search, tree view, --json |
| 4 | Get canvas root | framer-design-mcp | `framer canvas root` | Part of snapshot pipeline |
| 5 | Get node by ID | framer-design-mcp (source) | `framer nodes get <id>` | Cached in SQLite, offline lookup |
| 6 | Get node children | framer-design-mcp (source) | `framer nodes children <id>` | Tree traversal, --depth flag |
| 7 | List descendants | framer-design-mcp (source) | `framer nodes list <id> --descendants` | FTS search across nodes |
| 8 | Get node parent | framer-design-mcp (source) | `framer nodes parent <id>` | Breadcrumb path output |
| 9 | Get node rect | framer-design-mcp (source) | `framer nodes rect <id>` | Position/dimension reporting |
| 10 | Find nodes by type | framer-design-mcp (source) | `framer nodes find --type FrameNode` | SQLite-backed type queries |
| 11 | Find nodes by attribute | framer-design-mcp (source) | `framer nodes find --attr "backgroundColor=#fff"` | Attribute-based search offline |
| 12 | Find nodes by name | framer-design-mcp (source) | `framer nodes find --name "Hero"` | FTS search, regex support |
| 13 | Create frame node | framer-design-mcp (source) | `framer nodes create-frame` | --json input, --dry-run, --guard |
| 14 | Create text node | framer-design-mcp (source) | `framer nodes create-text` | Markdown input support |
| 15 | Create component node | framer-design-mcp (source) | `framer nodes create-component` | By name or URL |
| 16 | Add component instance | framer-design-mcp (source) | `framer components add <url>` | With custom props via --prop flags |
| 17 | Set node attributes | framer-design-mcp (source) | `framer nodes set <id>` | --guard flag for read-back verification |
| 18 | Set text content | framer-design-mcp (source) | `framer nodes set-text <id>` | Markdown support, --stdin |
| 19 | Set node parent | framer-design-mcp (source) | `framer nodes move <id> --parent <pid>` | --dry-run |
| 20 | Clone node | framer-design-mcp (source) | `framer nodes clone <id>` | Clone with modifications |
| 21 | Remove node | framer-design-mcp (source) | `framer nodes remove <id>` | --dry-run, batch remove |
| 22 | Add SVG | framer-design-mcp (source) | `framer nodes add-svg <file>` | From file or stdin |
| 23 | Create page | framer-design-mcp (source) | `framer pages create <name>` | --type web/design |
| 24 | Upload asset | framer-design-mcp (source) | `framer assets upload <file>` | Batch upload, progress bar |
| 25 | Add image | framer-design-mcp (source) | `framer assets add-image <url>` | From URL or local file |
| 26 | List color styles | framer-design-mcp (source) | `framer styles colors list` | --json, cached |
| 27 | Create color style | framer-design-mcp (source) | `framer styles colors create` | From hex, RGB, or design token |
| 28 | List text styles | framer-design-mcp (source) | `framer styles text list` | --json, cached |
| 29 | Create text style | framer-design-mcp (source) | `framer styles text create` | Font family, size, weight flags |
| 30 | List fonts | framer-design-mcp (source) | `framer fonts list` | Search by family name |
| 31 | List code files | framer-design-mcp (source) | `framer code list` | FTS search by name/content |
| 32 | Get code file | framer-design-mcp (source) | `framer code get <id>` | Output to stdout or --output file |
| 33 | Create code file | framer-design-mcp (source) | `framer code create <name>` | From file, stdin, or inline |
| 34 | Typecheck code | framer-design-mcp (source) | `framer code typecheck <id>` | Batch typecheck all files |
| 35 | Screenshot node | framer-design-mcp (source) | `framer nodes screenshot <id>` | Save to file, --format png/jpeg |
| 36 | Export SVG | framer-design-mcp (source) | `framer nodes export-svg <id>` | Save to file |
| 37 | List CMS collections | framer-design-mcp (source), framer-api | `framer cms collections list` | Cached, --json, field counts |
| 38 | Get collection details | framer-design-mcp (source) | `framer cms collections get <id>` | Full schema + item counts |
| 39 | List collection items | framer-design-mcp (source), framer-api | `framer cms items list --collection C` | FTS, --limit, --select, offline |
| 40 | Get collection item | framer-design-mcp (source) | `framer cms items get <id>` | Full field data, --json |
| 41 | Create collection | framer-design-mcp (source), framer-api | `framer cms collections create <name>` | With field definitions via --fields |
| 42 | Update collection fields | framer-design-mcp (source) | `framer cms fields update` | add/remove/reorder modes |
| 43 | Upsert collection items | framer-design-mcp (source), framer-api | `framer cms items upsert` | From JSON/CSV, batch, --dry-run |
| 44 | Remove collection items | framer-design-mcp (source), framer-api | `framer cms items remove <ids>` | Batch remove, --dry-run |
| 45 | Set item order | framer-api | `framer cms items reorder` | Interactive or by field sort |
| 46 | Publish preview | framer-api (source) | `framer publish` | Returns preview URL, --json |
| 47 | Deploy to production | framer-api (source) | `framer deploy <deployment-id>` | From publish output, --latest |
| 48 | Get changed paths | framer-api (source) | `framer changes list` | Added/removed/modified paths |
| 49 | Get change contributors | framer-api (source) | `framer changes contributors` | Version range filter |
| 50 | Get/set redirects | framer-api | `framer redirects list/add/remove` | Bulk import from CSV |
| 51 | Get/set custom code | framer-api | `framer custom-code get/set` | Head/body injection points |
| 52 | Localization read | framer-api | `framer i18n locales list` | All locales and groups |
| 53 | Connect to project | framer-api (source) | `framer connect <url>` | Saves to local config profile |
| 54 | Doctor/health check | CLI pattern | `framer doctor` | Checks Node.js, framer-api, API key, connectivity |

### Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|------------------------|
| T1 | Site Snapshot & Diff | `snapshot` / `diff <a> <b>` | hand-code | Local SQLite stores full project state over time. No existing tool persists Framer state across snapshots. MCP tools are stateless. Diffs require two time points. |
| T2 | Multi-Project Dashboard | `dashboard` / `projects list --stale 30` | hand-code | Go binary holds a local registry of N project configs. MCP servers bind to one project. Cross-project queries only work with a local aggregation layer. |
| T3 | CMS Sync with Dry-Run | `cms sync <source> --dry-run` | hand-code | Computes diff between local source (CSV/JSON/Sheets) and remote CMS state before writing. MCP tools are fire-and-forget with no preview. Local parse + remote snapshot + diff table + confirmed push. |
| T4 | Migration Scaffold | `migrate scrape <url>` / `migrate apply` | hand-code | Scrapes source site structure + content + assets into a local manifest, then applies to Framer. No tool bridges "arbitrary website" to "Framer project." Killer feature for the porting use case. |
| T5 | Attribute Capability Matrix | `api probe --type <NodeType>` | hand-code | Automates discovery of which attributes Framer silently rejects per node type. Encodes months of hard-won knowledge into a systematic probe. No one else documents this. |
| T6 | Silent Failure Guard | `--guard` flag on mutations | hand-code | Every mutation reads back the node after writing and compares intended vs. actual state. Catches #1 API pain point (silent attribute rejection). Requires stateful client caching intended state. |
| T7 | CMS Schema Diff | `cms schema diff <spec.yaml>` | hand-code | Treats CMS schema as infrastructure-as-code. Declare schema locally, diff against live. Essential for agencies managing schema evolution across projects. |
| T8 | Bulk Asset Pipeline | `assets upload <dir> --map-to C` | hand-code | Batch upload + CMS field mapping in one command. Walk local directory, match filenames to slugs, upload in parallel, update CMS fields atomically. Existing tools do single-asset uploads. |
| T9 | Publish Guardrails | `publish --preflight` | hand-code | Pre-publish linting: broken links, missing CMS refs, orphan pages, missing OG images, empty text nodes. Full project graph traversal against local snapshot. |
| T10 | Localization Sync | `i18n pull/push --format csv` | hand-code | Bridges standard i18n toolchains (PO, XLIFF, CSV) to Framer's proprietary localization model. API surface exists but no tool exposes it as a pipeline. |
| T11 | Redirect Map Generator | `redirects generate --old-sitemap <url>` | hand-code | Combines old-site sitemap crawl + Framer page slug introspection + fuzzy URL matching. Every migration needs redirects; today fully manual. |
| T12 | CMS Relationship Validator | `cms validate` | hand-code | Graph traversal across collections to find broken refs, orphans, circular references. No tool validates CMS referential integrity. Essential as CMS grows. |
