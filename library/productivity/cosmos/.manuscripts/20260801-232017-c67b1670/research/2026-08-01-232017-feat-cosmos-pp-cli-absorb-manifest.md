# Cosmos Absorb Manifest

## Scope sources

- Authenticated live capture: 37 retained GraphQL operations across 19 resources.
- [`jpoindexter/cosmos-mcp`](https://github.com/jpoindexter/cosmos-mcp): 15 tools covering search, collections, library writes, profile, similarity, login, and refresh.
- [`rclaycock/cosmos-scraper-mk-3`](https://github.com/rclaycock/cosmos-scraper-mk-3): public collection pagination, media normalization/deduplication, JSON feeds, and gallery output.
- [`rawpage/suggaplay`](https://github.com/rawpage/suggaplay): collection-by-slug resolution, resumable media download, format detection, and manifests.
- [`rslosh/promptbox`](https://github.com/rslosh/promptbox): unique image counting, collection sync, and media download.
- [`likeahuman-ai/roxit-masterclass`](https://github.com/likeahuman-ai/roxit-masterclass): Cosmos search skill, compact summaries, and local HTML gallery output.
- Cosmos shipped web bundles: current operation documents for subcollections, connection edits, imports, follows, and account bootstrap.
- Crowd-sniff result: rejected from the merge because its only endpoint, `/_cosmos/remote-renderer-url`, belongs to Azure Cosmos DB rather than cosmos.so.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Authenticate with a bearer token | cosmos-mcp auth source | cosmos-pp-cli auth login | Keychain/config persistence, env override, non-interactive token input, typed auth status |
| 2 | Refresh an expired access token | cosmos-mcp `cosmos_refresh_auth` | cosmos-pp-cli auth refresh | Atomic credential update and machine-readable expiry/error output |
| 3 | Search visual elements | cosmos-mcp `cosmos_search_elements`; cosmos-inspo | cosmos-pp-cli discover elements | Cursor pagination, stable JSON/CSV, source attribution, optional local cache |
| 4 | Search collections | cosmos-mcp `cosmos_search_clusters` | cosmos-pp-cli discover collections | Cursor pagination, owner/visibility fields, composable output selectors |
| 5 | Search elements and collections together | cosmos-mcp `cosmos_search_global`; live capture | cosmos-pp-cli discover all | One normalized result envelope and agent-friendly type discrimination |
| 6 | Browse featured/trending elements and collections | cosmos-mcp featured tools; live feed | cosmos-pp-cli discover featured | Explicit element/collection modes, pagination, JSON and local sync |
| 7 | Get the authenticated profile | cosmos-mcp `cosmos_get_me` | cosmos-pp-cli profile me | Auth diagnostics and stable user ID for follow-on commands |
| 8 | List and filter personal collections | cosmos-mcp `cosmos_get_my_clusters`; live capture | cosmos-pp-cli collection list | Private/nested collection support, cursor pagination, local search fallback |
| 9 | Resolve and inspect a collection by ID or owner/slug | cosmos-mcp; suggaplay downloader | cosmos-pp-cli collection show | Accepts IDs or URLs and preserves owner, parent, visibility, counts, and cover metadata |
| 10 | Paginate every element in a collection | cosmos-scraper-mk-3; promptbox; live capture | cosmos-pp-cli collection elements | Resume cursors, full/all modes, media/source fields, deterministic ordering |
| 11 | Search inside one collection | authenticated live capture | cosmos-pp-cli collection search | Private-collection support and normalized element output |
| 12 | Create a collection | cosmos-mcp `cosmos_create_cluster` | cosmos-pp-cli collection create | `--private`, `--description`, `--dry-run`, confirmation, and structured result |
| 13 | Create a nested subcollection | Cosmos shipped bundle | cosmos-pp-cli collection create-sub | Parent resolution by ID/URL plus dry-run and confirmation semantics |
| 14 | Save a URL as a Cosmos element | cosmos-mcp `cosmos_save_url` | cosmos-pp-cli element save-url | Optional collection target, idempotency check, dry-run, and stdin batch mode |
| 15 | Connect an existing element to a collection | cosmos-mcp `cosmos_save_to_cluster` | cosmos-pp-cli collection connect | Batch IDs, no-op detection, dry-run, and safe retry output |
| 16 | Disconnect an element from a collection | cosmos-mcp connection mutation source | cosmos-pp-cli collection disconnect | Explicit destructive confirmation, dry-run, and batch support |
| 17 | List collections an element can connect to | cosmos-mcp `cosmos_get_connectable_clusters`; live capture | cosmos-pp-cli element connections | Saved/not-saved status, name filtering, and stable collection identifiers |
| 18 | Inspect an element and its social/source metadata | authenticated live capture | cosmos-pp-cli element show | Unified media variants, attribution, counts, owning collection, and share URL |
| 19 | Find visually similar elements | cosmos-mcp `cosmos_get_similar_elements`; live capture | cosmos-pp-cli element similar | Bounded pagination, source attribution, and deduplicated IDs |
| 20 | Read account activity and recent events | authenticated live capture | cosmos-pp-cli activity list | Time-window filters, unread filtering, cursor pagination, and local history |
| 21 | Fetch the personalized For You feed and recommendations | authenticated live capture | cosmos-pp-cli feed | Deterministic pagination and agent-ready media/source projection |
| 22 | Inspect active import jobs and progress | authenticated live capture | cosmos-pp-cli import status | Structured progress, failure state, and collection association |
| 23 | Export/download a collection with resume and manifest | cosmos-scraper-mk-3; suggaplay; promptbox | cosmos-pp-cli export collection | Checksums, deduplication, format detection, source attribution, resumable downloads, no shell-outs |
| 24 | Render a local searchable gallery and compact result summary | cosmos-inspo skill; cosmos-scraper-mk-3 | cosmos-pp-cli export gallery | Offline static HTML, JSON manifest, no auto-open by default, reproducible inputs |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---|---|---|---|---|---|---|
| 1 | Weekly library review | `review --since 7d` | 9/10 | hand-code | This uses locally synced elements, collection connections, source fields, and sync timestamps to compute a deterministic maintenance queue with no external dependencies. | Cosmos exposes recent activity/all-elements; no community client provides a maintenance review. | Use this command for a time-windowed maintenance review. Do NOT use this command to compare two historical snapshots; use `snapshot diff` instead. |
| 2 | Collection overlap | `collection overlap <left> <right>` | 8/10 | hand-code | This joins collection-element connections and media identifiers in SQLite to return shared, duplicate, and unique references. | Live capture exposes connection data; downloaders already need media deduplication but cannot compare boards. | Use this command to compare two current collections. Do NOT use this command for time-based change history; use `snapshot diff` instead. |
| 3 | Search coverage gap | `collection coverage --collection <id> --query <text>` | 8/10 | hand-code | This combines live `SearchGlobalElements` results with locally synced collection membership to return only not-yet-saved candidates. | Search is Cosmos's core product; the captured API and local connection store enable the missing join. | Use this command to find references missing from one collection. Do NOT use this command to inspect duplication between two existing collections; use `collection overlap` instead. |
| 4 | Provenance audit | `provenance audit` | 8/10 | hand-code | This scans local source, author, caption, and connection fields to report missing attribution and source concentration without fetching third-party sites. | Cosmos officially emphasizes artist/source/story attribution; exporters preserve sources but do not audit them. | none |
| 5 | Similarity trail | `element trail --id <id> --depth 2` | 7/10 | hand-code | This repeatedly calls `GetSimilarElements` with bounded depth, deduplicates element IDs, and emits a reproducible similarity graph. | Cosmos markets visual search; MCP and live capture expose only one-hop similarity. | none |
| 6 | Snapshot diff | `snapshot diff --from <time> --to <time>` | 8/10 | hand-code | This compares historical local connection snapshots to identify added, removed, and moved references across collections. | Cursor sync and connection records are available; existing scrapers overwrite outputs and lose history. | Use this command for historical changes. Do NOT use this command to compare two collections at the same time; use `collection overlap` instead. |

## Explicit stubs

None. Every row above is shipping scope; unsupported mutation paths must return an actionable compatibility error rather than a placeholder success.

## Deliberate exclusions

- No resident browser runtime: browser automation was discovery-only; normal commands use replayable HTTP.
- No semantic auto-tagging or image generation: those require external AI services outside the researched Cosmos contract.
- No automatic write-heavy collection merge: safe connect/disconnect primitives ship, but opaque bulk mutation does not.
- No persistent dashboard: the CLI emits static galleries and structured data instead of running an application server.
