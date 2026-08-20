# Weaviate Collections CLI Brief

## API Identity
- Domain: Weaviate Cloud — collection (schema) configuration management. Scoped to `/v1/schema/*` from the official Weaviate REST API (full OpenAPI spec has 77 paths; this CLI targets the 13 schema/collection paths, matching https://docs.weaviate.io/weaviate/config-refs/collections).
- Users: developers/ops managing Weaviate Cloud collections — creating collections, tuning vectorizer/replication/sharding config, managing multi-tenancy, rebuilding indexes.
- Data profile: collection configs (class name, properties, vectorizer, module config, replication/sharding settings), tenants, shards, property-level inverted indexes.
- Auth: Weaviate Cloud API key sent as `Authorization: Bearer <key>`. Confirmed live: `GET /v1/meta` returns 401 with no key, 200 with key. `GET /v1/schema` returns `{"classes":[]}` with key (empty instance).

## Reachability Risk
- None. Confirmed live: `https://<cluster>.weaviate.cloud/v1/schema` returns HTTP 200 with Bearer auth.

## Top Workflows
1. Create a new collection with vectorizer/module config, replication and sharding settings.
2. List/inspect existing collections and their full config (properties, indexes, replication).
3. Add properties to an existing collection; manage per-property inverted-index settings (tokenization, rebuild/cancel index).
4. Manage multi-tenancy: enable tenants, list/activate/deactivate, check tenant existence.
5. Inspect and reassign shards for a collection.

## Table Stakes (from official weaviate-cli + community tools)
- Collection CRUD (create/get/update/delete) — official `weaviate/weaviate-cli`, community `weave-cli`
- Multi-tenancy management (list/create/update/delete tenants) — official `weaviate-cli`, `mcp-weaviate` (`list_collections`, `get_schema`)
- Schema/config inspection (get_schema equivalent) — `mcp-weaviate`
- Shard listing/status — official REST API only, no CLI currently exposes it well

## Data Layer
- Primary entities: collections (classes), properties, tenants, shards.
- Sync cursor: `sync` pulls full `/v1/schema` into local SQLite for offline search/diff.
- FTS/search: search collections/properties by name, vectorizer, module.

## Product Thesis
- Name: **Weaviate Collections CLI** (`weaviate-pp-cli`)
- Why it should exist: no existing tool gives collection-config diffing, drift detection, or point-in-time snapshots across a Weaviate Cloud instance. Official `weaviate-cli` and `weave-cli` are live-only, no local history. This CLI adds an offline SQLite layer for history/diff/lint on top of full collection-config CRUD.

## Build Priorities
1. Collection CRUD + properties + indexes (typed endpoint commands from spec)
2. Multi-tenancy + shards (typed endpoint commands from spec)
3. Transcendence: schema snapshot/history, schema diff, collections lint, tenant audit, export/import bundle
