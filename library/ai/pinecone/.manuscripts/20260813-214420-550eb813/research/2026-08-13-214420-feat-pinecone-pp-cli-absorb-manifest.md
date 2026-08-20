# Pinecone CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

The full 95-endpoint Pinecone API surface is absorbed via the official OpenAPI specs (data, control, inference, admin, metrics, oauth, assistant). Every command in the official `pc` CLI is matched: index CRUD/configure/stats, vector upsert/query/fetch/update/delete/list, namespace CRUD, collection CRUD, backup create/delete/describe/list, restore describe/list, import start/describe/list/cancel, project/org CRUD, api-key CRUD, whoami, target, auth login/configure/local-keys. Plus the official MCP surface (list-indexes, describe-index, describe-index-stats, create-index-for-model, upsert-records, search-records, rerank-documents) and inference (embed/rerank/models).

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List indexes | pc CLI / MCP list-indexes | pinecone-pp-cli indexes list | --json, --select, offline sync, typed exit codes |
| 2 | Create index | pc CLI index create | pinecone-pp-cli indexes create-index | --dry-run, agent JSON, idempotent |
| 3 | Describe index | pc CLI index describe | pinecone-pp-cli indexes describe-index | resolves host for data plane |
| 4 | Configure index | pc CLI index configure | pinecone-pp-cli indexes configure-index | --dry-run |
| 5 | Delete index | pc CLI index delete | pinecone-pp-cli indexes delete-index | --dry-run, --ignore-missing |
| 6 | Create index for model | MCP create-index-for-model | pinecone-pp-cli indexes create-index-for-model | agent JSON |
| 7 | Index stats | pc CLI index stats / MCP describe-index-stats | pinecone-pp-cli describe-index-stats | --select, --json |
| 8 | Upsert vectors | pc CLI vector upsert | pinecone-pp-cli vectors upsert | stdin JSON, --dry-run, per-record errors |
| 9 | Query vectors | pc CLI vector query | pinecone-pp-cli query | metadata filter, sparse, --select |
| 10 | Fetch vectors | pc CLI vector fetch | pinecone-pp-cli vectors fetch | --json |
| 11 | Fetch by metadata | API fetch_by_metadata | pinecone-pp-cli vectors fetch-by-metadata | --json |
| 12 | List vector IDs | pc CLI vector list | pinecone-pp-cli vectors list | prefix, pagination, --limit |
| 13 | Update vector | pc CLI vector update | pinecone-pp-cli vectors update | --dry-run |
| 14 | Delete vectors | pc CLI vector delete | pinecone-pp-cli vectors delete | ids/deleteAll/filter, --dry-run |
| 15 | Namespace CRUD | pc CLI namespace | pinecone-pp-cli namespaces | --json |
| 16 | Records search/upsert | MCP search-records/upsert-records | pinecone-pp-cli records search/upsert | agent JSON |
| 17 | Bulk imports | pc CLI import | pinecone-pp-cli bulk start-import/list-imports/describe-import/cancel-import | --json |
| 18 | Collections CRUD | pc CLI collection | pinecone-pp-cli collections | --json |
| 19 | Backups CRUD | pc CLI backup | pinecone-pp-cli backups | --json |
| 20 | Backup schedules | API backup-schedules | pinecone-pp-cli backup-schedules | --json |
| 21 | Restore jobs | pc CLI restore | pinecone-pp-cli restore-jobs | --json |
| 22 | Embed | inference API | pinecone-pp-cli embed | --json, batch inputs |
| 23 | Rerank | MCP rerank-documents | pinecone-pp-cli rerank | --json |
| 24 | Models list/describe | inference API | pinecone-pp-cli models | --json |
| 25 | Admin projects/orgs/api-keys/users/invites/service-accounts/role-bindings | Admin API (Bearer) | pinecone-pp-cli admin | --json, --dry-run |
| 26 | OAuth token | oauth API | pinecone-pp-cli oauth token | --json |
| 27 | Assistants (control+data) | Assistant API | pinecone-pp-cli assistants / files / operations / chat | --json |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Text query (embed→query pipe) | text-query | 9/10 | hand-code | Chains the in-spec `/embed` inference endpoint then the per-index-host `query` endpoint with metadata filters, resolving the host from the synced index table, and emits agent-shaped JSON | Top Workflow 3; Product Thesis RAG builders; pc lacks the pipe; MCP ships search-records/rerank-documents | Use this command for natural-language text search against a single dense index. Do NOT use it for raw vector input; use 'query'. Do NOT use it to search multiple indexes at once; use 'cascade'. |
| 2 | Index snapshot + diff | snapshot | 9/10 | hand-code | Writes per-namespace counts from describe_index_stats plus index config from describe into a local SQLite time-series, then diffs two snapshots in SQL | Product Thesis "historical snapshots" gap; Build Priorities #5; Data Layer index→host | Use this command to capture a point-in-time index state and diff it against earlier snapshots. Do NOT use it for growth/cost projection; use 'usage'. Do NOT use it for one-off grouping of synced entities; use 'analytics'. |
| 3 | Stale-chunk pruning | prune | 8/10 | hand-code | Selects stale vector IDs from synced record metadata (timestamp, drain-first local query), prints a dry-run plan, only with --apply issues batched in-spec delete (by ID) calls | Live index has timestamp/sender metadata; Top Workflow 4; Maya's weekly re-embed ritual | Use this command to delete stale vectors identified from local metadata timestamps. Do NOT use it for arbitrary filter/ID deletion; use 'delete'. |
| 4 | Cross-index cascade search | cascade | 8/10 | hand-code | Runs the text-query plan per index, merges results by vector ID keeping best score, emits one ranked agent output | Official MCP cascading-search demand signal; Product Thesis cross-index cascade; Priya eval | Use this command to run the same semantic query across multiple indexes and merge ranked results. Do NOT use it for a single index; use 'text-query'. |
| 5 | Usage / growth projection | usage | 7/10 | hand-code | Joins snapshot-history rows to compute vector-count deltas and per-namespace distribution shift, growth projection to a horizon — no external pricing data | Build Priorities #5 cost/usage estimation; pc has no historical snapshots; Dev health pass | Use this command for growth and usage deltas computed from snapshot history. Do NOT use it to capture a new point-in-time snapshot; use 'snapshot'. |
| 6 | Vector payload validation | check-vectors | 7/10 | hand-code | Parses the payload and compares dimension/ID-uniqueness/sparse-shape against synced index config plus live describe_index_stats, reporting mismatches without writing | pc has no dry-run; Top Workflow 2 batch upsert failures; Maya's blind upsert loop | Use this command to validate a vector payload against the index schema before writing. Do NOT use it to write vectors; use 'upsert'. |
| 7 | Backup/restore coverage report | coverage | 7/10 | hand-code | Joins synced backups × backup-schedules × restore-jobs × indexes tables into a per-index protection matrix with last-success timestamps | Data Layer lists backups/restore-jobs as synced entities; Top Workflow 4; Dev cannot answer "is every index protected?" | Use this command for the backup/restore protection report joined across backups, schedules, and restore jobs. Do NOT use it for per-job status inspection; use 'analytics --type backups' or 'tail --resource backup-schedules'. |

All 7 transcendence features are `hand-code` (~50-150 LoC each plus root.go wiring). No stubs. No external dependencies beyond the Pinecone API + local SQLite.
