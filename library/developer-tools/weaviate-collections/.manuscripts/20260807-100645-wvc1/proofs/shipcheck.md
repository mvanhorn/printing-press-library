# Shipcheck: weaviate-collections-pp-cli

## Result
Verdict: **PASS** (7/7 legs) — Scorecard 95/100, Grade A

| Leg | Result |
|---|---|
| verify | PASS |
| validate-narrative | PASS |
| dogfood | PASS (5/5 novel features survived) |
| workflow-verify | PASS |
| apify-audit | PASS (n/a) |
| verify-skill | PASS |
| scorecard | PASS |

## Bugs found and fixed during live dogfooding (against real Weaviate Cloud cluster)
1. **`schema objects-get` returned only the `properties` array, not the full collection config.** Generator picked the wrong response-extraction path (`"properties"`, confusing `Class.properties` with the unrelated `IndexStatusResponse.properties` field used by the `indexes` endpoint). Fixed in `schema_objects-get.go`.
2. **`--multi-tenancy-config` / `--object-ttl-config` flags on `schema objects-create`/`objects-update` sent the raw flag string as the body value instead of parsing it as JSON**, causing a 400 from the API. Fixed in both files to parse-and-validate like the sibling `--module-config`/`--sharding-config` flags.
3. Top-level `schema`, `tenants`, `shards`, `vectors` command groups were generated with `Hidden: true`, burying the CLI's core CRUD surface from `--help`. Un-hid all four.
4. `defaultSyncResources()` returned `[]`, making `sync` a no-op by default even though `schema` is a registered syncable resource. Fixed to default to `["schema"]`.
5. `research.json` narrative quickstart/recipes referenced a `collections list`/`collections get`/`collections update` surface that doesn't exist (novel `collections` group only has `lint`). Corrected to the real `schema dump` / `schema objects-get` / `schema objects-update` paths.

## Live smoke test (against the user's real Weaviate Cloud cluster, cleaned up after)
- Created/inspected/updated/deleted a real test collection (`PPTestArticle`) — full CRUD confirmed.
- Created a multi-tenant collection, created 2 tenants, confirmed via `tenants get`/`exists`.
- Ran all 5 novel commands live: `schema snapshot` + `schema snapshot list`, `schema diff --against <label>` (correctly detected added `category` property), `collections lint` (flagged no-vectorizer + replication-factor-1), `tenants audit` (aggregated tenant status across collections), `schema export` + `schema import` (round-tripped a real bundle).
- Cluster restored to its original empty state (0 collections) at the end.

## Known gaps
- `mcp_token_efficiency` scored 4/10 (secondary quality dimension, not a blocker).
- `Cache Freshness` 5/10 — no `cache.enabled` freshness helper wired since collection configs change infrequently and syncable-resource detection is coarse for this API; local store still serves offline reads via `sync`.
