# Mobbin CLI — Absorb Manifest (reprint)

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|---------------------|-------------|
| 1 | List apps by platform | pdcolandrea/mobbin-mcp | (generated endpoint) apps list | Local SQLite mirror for offline autocomplete; --json/--select/--agent |
| 2 | Popular apps + preview screens | Mobbin UI | (generated endpoint) apps popular | --platform, --limit-per-category ergonomics |
| 3 | Discover page (paginated) | Mobbin UI | (generated endpoint) apps discover | --tab latest/popular/animations + --page-index |
| 4 | Search apps (auth) | pdcolandrea / official MCP | (generated endpoint) apps search | --platform/--app-categories/--sort-by, typed exit codes |
| 5 | Search screens (auth) | pdcolandrea / official MCP | (generated endpoint) screens search | Combined --screen-patterns/--screen-elements/--screen-keywords(OCR)/--has-animation |
| 6 | Search flows (auth) | pdcolandrea | (generated endpoint) flows search | --flow-actions filter |
| 7 | Full filter taxonomy | pdcolandrea | (generated endpoint) filters list | Synced to SQLite; axis-discovery offline |
| 8 | Trending apps | Mobbin UI | (generated endpoint) trending apps | --platform, agent-native |
| 9 | Trending sites | Mobbin UI | (generated endpoint) trending sites | Web surface |
| 10 | Trending filter-tags | Mobbin UI | (generated endpoint) trending filter-tags | --experience/--platform |
| 11 | Trending OCR keywords | Mobbin UI | (generated endpoint) trending keywords | Text-in-screenshot axis |
| 12 | Searchable sites | Mobbin UI | (generated endpoint) sites list | Cached locally |
| 13 | Cross-entity autocomplete | pdcolandrea | (generated endpoint) autocomplete query | Fast ID lookup |
| 14 | List workspaces | underthestars-zhy/MobbinAPI | (generated endpoint) workspaces list | Required for collection creation |
| 15 | List collections | pdcolandrea | (generated endpoint) collections list | --json/--agent |
| 16 | Collection contents | pdcolandrea | (generated endpoint) collections contents | Keyset pagination + buckets |
| 17 | Create collection | underthestars-zhy/MobbinAPI | (generated endpoint) collections create | First CLI to ship it |
| 18 | Delete collection | underthestars-zhy/MobbinAPI | (generated endpoint) collections delete | Destructive-hint MCP annotation |
| 19 | Add screen/flow/app to collection | underthestars-zhy/MobbinAPI | (generated endpoint) collections add-screen/add-flow/add-app | Write-side deck curation |
| 20 | Remove screen from collection | underthestars-zhy/MobbinAPI | (generated endpoint) collections remove-screen | Destructive-hint annotation |
| 21 | Auth login via browser session | solejay/mobbin-cli | (behavior in mobbin-pp-cli auth login --chrome) | Split Supabase JWT import + auto refresh |
| 22 | Batch full-res image download | ismailsaleekh/mobbin-agent | (behavior in mobbin-pp-cli grab) | Bytescale URL translation, dedup, cache paths |
| 23 | Cross-entity offline search | (new) | mobbin-pp-cli search | FTS5 across synced entities |
| 24 | Read-only SQL over library | (new) | mobbin-pp-cli sql | Arbitrary aggregation |
| 25 | Store population / refresh | (new) | mobbin-pp-cli sync | Timestamped snapshots for drift/audit |
| 26 | Agent context manifest | Framework | mobbin-pp-cli context | Agent-native taxonomy |
| 27 | Doctor / health / auth status | Framework | mobbin-pp-cli doctor | Auth + plan tier + reachability |

## Transcendence (only possible with our approach) — all 6 prior novels KEPT
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|-------------------------|------------------|
| 1 | Pattern Deck Export | deck | hand-code | 9 | Chains screens search + Bytescale full-res + zip/CSV; UI has no deck export | Use for a shareable crit zip+CSV; not raw folder (use grab) |
| 2 | Offline Pattern Bench | bench | hand-code | 9 | Local SQLite aggregate over screens×patterns×apps — a shape no endpoint returns | Cross-app pattern leaderboard; not audit/cross |
| 3 | Flow Audit with Delta | audit | hand-code | 9 | Local join flows×apps×captured_at with --since; API has no time filter | Time-windowed industry flow list; not drift/bench |
| 4 | Version Drift Watch | drift | hand-code | 9 | Local app_versions snapshot diff; API is current-state only | Per-app change diff between syncs; not audit |
| 5 | Batch Full-Res Grab | grab | hand-code | 8 | Bytescale URL translation + templating + manifest.json side-car | Raw templated PNGs + manifest.json; not deck |
| 6 | Cross-Platform Parity | cross | hand-code | 8 | Two platform fan-outs joined on app slug; API is platform-scoped | web/iOS parity for named apps; not bench |

Killed candidates: teardown (wrapper), gaps (verifiability), copy (sibling: FTS5 search), digest (wrapper), elements (sibling), similar (LLM/embeddings infra).

Hand-code count: **6** transcendence rows (all hand-code). Auto-emitted: rows 1-20 endpoint mirrors + framework 23-27.
