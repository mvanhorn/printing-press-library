# Pinecone CLI Brief

## API Identity
- Domain: Vector database (serverless, control plane + per-index data plane + inference + admin)
- Users: AI engineers, RAG builders, ML platform teams, agents doing semantic search/memory
- Data profile: Indexes (config), vectors (id+values+sparse+metadata, up to 20000 dims), namespaces, collections, backups, imports (object storage), embedding/rerank models, assistants, org/project/admin entities
- API versioning: `X-Pinecone-Api-Version: 2026-04` header required (docs default to oldest stable otherwise). Quarterly releases, 12-month support.

## Reachability Risk
- None. Official OpenAPI specs published in `pinecone-io/pinecone-api` repo (2026-04). Live API verified working with user-provided key: control plane list indexes 200, data plane list/stats/query/fetch 200, existing index `travel-chat-embeddings` (1536 dim, cosine, serverless us-east-1, 2928 vectors in `__default__` namespace with metadata `chunk_id`/`sender`/`text`/`timestamp`).
- Admin API requires Bearer (service-account access token via `login.pinecone.io/oauth/token`), NOT the plain Api-Key header. Inference endpoints live at root (`/embed`, `/rerank`, `/models`) on `api.pinecone.io` — spec says servers `https://api.pinecone.io` (no /inference prefix).
- Data plane ops are per-index-host: `https://{index_host}/...` with `Api-Key` header.

## Top Workflows
1. Create/configure/list/describe/delete serverless (and pod) indexes — pick cloud+region, dimension, metric, tags, deletion protection
2. Upsert vectors (batch, from JSON/file/stdin) into a namespace, then query by vector/id/text with metadata filters, top-k, includeValues/includeMetadata
3. Search with text on integrated-embedding indexes (records/namespaces/{ns}/search) incl. rerank; also standalone inference embed + rerank
4. Manage data lifecycle: fetch by ID, list IDs by prefix/pagination, update, delete by ID/filter/deleteAll, namespaces CRUD, index stats
5. Admin/org: projects, api-keys, users, invites, service accounts, role bindings (Bearer)

## Table Stakes
- `pc` (pinecone-io/cli, Go): auth login/configure/local-keys, target org/project, index list/create/describe/delete/configure/stats, index vector upsert/query/fetch/update/delete/list, index namespace create/describe/delete/list, index record search/upsert, index collection CRUD, index backup create/delete/describe/list, index restore describe/list, index import start/describe/list/cancel, project CRUD, organization CRUD, api-key CRUD, whoami/version. Exit codes 0/1 only, no offline/local store.
- Official SDKs: Python 9.1.0, Node, Java, Go (69 stars)
- Official MCP `@pinecone-database/mcp` (v0.3.0): search-docs, list-indexes, describe-index, describe-index-stats, create-index-for-model, upsert-records, search-records, cascading-search, rerank-documents (integrated-embedding indexes only)
- Community: tullytim/pinecone-cli (Python, 22 stars, stale 2023)

## Data Layer
- Primary entities: indexes (config + status), vectors (records), namespaces, collections, backups, imports, models
- Sync cursor: list indexes (control plane) + per-index list IDs (data plane, paginated by prefix) + fetch for full records
- FTS/search: FTS over synced index metadata (records text), namespace/import/backup tables
- Index host resolution: describe index → `host` field → data plane base URL. Local store maps index→host.

## User Vision
- (none provided beyond "analyze, study, deeply think, dogfood and build")

## Product Thesis
- Name: `pinecone-pp-cli` (binary), "Pinecone CLI" display
- Why it should exist: the official `pc` CLI is thin — exit codes 0/1, no offline local store, no agent-native JSON/select/csv, no dry-run, no cross-index analysis, no historical snapshots. Our CLI matches every pc command AND adds local SQLite sync (offline query of metadata/senders), agent-native output, typed exit codes, dry-run, hybrid query helpers, and novel cross-index/cost/diff features.

## Build Priorities
1. Control plane: index list/describe/create/delete/configure, collection, backup, restore, import (full)
2. Data plane: upsert/query/fetch/list/update/delete, namespaces, stats, fetch-by-metadata (full)
3. Inference: embed, rerank, models (full)
4. Admin (Bearer): projects, api-keys, users, invites, service accounts, role bindings
5. Novel: local sync + offline search, query-with-embedding pipe, hybrid search helper, index snapshot/diff, cost/usage estimation, cross-index cascading search
