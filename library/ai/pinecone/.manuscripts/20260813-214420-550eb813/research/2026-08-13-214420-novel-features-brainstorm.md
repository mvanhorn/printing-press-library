# Pinecone CLI — Novel Features Brainstorm (2026-08-13)

## Customer model

The brief's Users line names AI engineers, RAG builders, ML platform teams, and agents doing semantic search/memory; the live `travel-chat-embeddings` index (real WhatsApp travel-group chat chunks with `sender`/`timestamp` metadata) and the five Top Workflows ground three personas.

**Maya — RAG memory maintainer (owns `travel-chat-embeddings`)**
- **Today:** Each week she exports the WhatsApp travel group's chat, chunks it, and re-embeds new messages. Her tabs: the Pinecone console on the index page, a chat-export script in an editor, and a Python scratch file that calls embed then query by copy-pasting the vector. She cannot answer "which old chunks are still in the namespace and when were they written," and she has no idea whether her batch file will fail on upsert (wrong dimension, duplicate IDs) until the API rejects the whole batch.
- **Weekly ritual:** Export chat → chunk → embed → upsert into `__default__` → run 3-4 sanity queries ("what did the group decide about Kyoto?") → verify counts in describe-index-stats.
- **Frustration:** There is no offline view of what is stored (chunks, senders, timestamps) and no way to pre-validate a payload or sweep stale chat, so every week is the same blind upsert-then-fix loop.

**Dev — ML platform SRE (owns index fleet, deletion protection, backups)**
- **Today:** Before standup he clicks through console pages — index list, per-index stats, backup schedules, restore jobs — to confirm nothing is degrading. He keeps a spreadsheet of "index → last successful backup → vector count" that he updates by hand because no single screen joins backups, schedules, and restore jobs. He cannot answer "is every index protected and did anything change this week?"
- **Weekly ritual:** Index health pass: list indexes → check stats and growth → verify backup/restore coverage → confirm imports finished → scan for config drift.
- **Frustration:** Coverage is scattered across five API surfaces (indexes, backups, schedules, restore-jobs, imports) with no history — "what changed since last week?" requires him to have been watching.

**Priya — RAG evaluation engineer (dense + integrated-embedding indexes, rerank)**
- **Today:** For every retrieval experiment she re-embeds the docs corpus, runs a query, and manually embeds her natural-language query first because the query endpoint takes vectors, not text. Comparing index candidates means running the same query twice and eyeballing two result dumps. She cannot answer "which index version retrieves better for these eval queries?" in one step.
- **Weekly ritual:** Re-embed eval corpus → run eval query set with rerank → compare results across index versions → decide whether to reindex.
- **Frustration:** The two-step embed-then-query dance and the absence of any cross-index comparison tool turn every eval session into manual glue.

## Candidates (pre-cut)

1. **`text-query`** — 9/10 KEEP. Chains embed + query endpoints. Persona: Priya. Source (a)+(b).
2. **`snapshot` / `snapshot diff`** — 9/10 KEEP. Local SQLite time-series over describe_index_stats + describe. Persona: Dev. Source (b)+(c).
3. **`cascade`** — 8/10 KEEP. Multi-index merge; mirrors official MCP cascading-search. Persona: Priya. Source (a)+(b).
4. **`prune`** — 8/10 KEEP. Local metadata timestamp selection + batched delete-by-ID, dry-run default + --apply. Persona: Maya. Source (a).
5. **`usage`** — 7/10 KEEP (reframed from cost estimate to growth projection from local stats only). Persona: Dev. Source (b)+(c).
6. **`check-vectors`** — 7/10 KEEP. Offline payload validation vs synced index config. Persona: Maya. Source (a).
7. **`coverage`** — 7/10 KEEP. Backups × backup-schedules × restore-jobs × indexes protection matrix. Persona: Dev. Source (c).
8. **`hybrid`** — CUT. Local BM25 sparse unverifiable, dense-only live index, speculative pain.
9. **`diff-index`** — CUT. Monthly cadence; snapshot diff covers change detection.
10. **`namespace-report`** — CUT. Subsumed by snapshot (time axis) / analytics (group-by).
11. **`host`** — CUT. Thin wrapper of describe's host field.
12. **`pipeline-report`** — CUT. Same shape as coverage; analytics covers imports status.
13. **`reconcile`** — CUT. Re-sync dominates on 3k-vector index; sync + stale hints cover it.

## Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Text query (embed→query pipe) | `text-query <index> --text <s> [--top-k N] [--namespace ns] [--json]` | 9/10 | hand-code | Chains the in-spec `/embed` inference endpoint then the per-index-host `query` endpoint with metadata filters, resolving the host from the synced index table, and emits agent-shaped JSON | Top Workflow 3; Product Thesis RAG builders; pc lacks the pipe; MCP ships search-records/rerank-documents | Use this command for natural-language text search against a single dense index. Do NOT use it for raw vector input; use 'query'. Do NOT use it to search multiple indexes at once; use 'cascade'. |
| 2 | Index snapshot + diff | `snapshot <index> [--note s]` / `snapshot diff --index <i> --since 7d` | 9/10 | hand-code | Writes per-namespace counts from describe_index_stats plus index config from describe into a local SQLite time-series, then diffs two snapshots in SQL | Product Thesis "historical snapshots" gap; Build Priorities #5; Data Layer index→host | Use this command to capture a point-in-time index state and diff it against earlier snapshots. Do NOT use it for growth/cost projection; use 'usage'. Do NOT use it for one-off grouping of synced entities; use 'analytics'. |
| 3 | Stale-chunk pruning | `prune <index> --namespace ns --older-than 90d [--apply]` | 8/10 | hand-code | Selects stale vector IDs from synced record metadata (timestamp, drain-first local query), prints a dry-run plan, only with --apply issues batched in-spec delete (by ID) calls | Live index has timestamp/sender metadata; Top Workflow 4; Maya's weekly re-embed ritual | Use this command to delete stale vectors identified from local metadata timestamps. Do NOT use it for arbitrary filter/ID deletion; use 'delete'. |
| 4 | Cross-index cascade search | `cascade --indexes a,b [--text s] [--top-k N] [--json]` | 8/10 | hand-code | Runs the text-query plan per index, merges results by vector ID keeping best score, emits one ranked agent output | Official MCP cascading-search demand signal; Product Thesis cross-index cascade; Priya eval | Use this command to run the same semantic query across multiple indexes and merge ranked results. Do NOT use it for a single index; use 'text-query'. |
| 5 | Usage / growth projection | `usage --index <i> --since 30d` | 7/10 | hand-code | Joins snapshot-history rows to compute vector-count deltas and per-namespace distribution shift, growth projection to a horizon — no external pricing data | Build Priorities #5 cost/usage estimation; pc has no historical snapshots; Dev health pass | Use this command for growth and usage deltas computed from snapshot history. Do NOT use it to capture a new point-in-time snapshot; use 'snapshot'. |
| 6 | Vector payload validation | `check-vectors --index <i> --file <path> [--json]` | 7/10 | hand-code | Parses the payload and compares dimension/ID-uniqueness/sparse-shape against synced index config plus live describe_index_stats, reporting mismatches without writing | pc has no dry-run; Top Workflow 2 batch upsert failures; Maya's blind upsert loop | Use this command to validate a vector payload against the index schema before writing. Do NOT use it to write vectors; use 'upsert'. |
| 7 | Backup/restore coverage report | `coverage` | 7/10 | hand-code | Joins synced backups × backup-schedules × restore-jobs × indexes tables into a per-index protection matrix with last-success timestamps | Data Layer lists backups/restore-jobs as synced entities; Top Workflow 4; Dev cannot answer "is every index protected?" | Use this command for the backup/restore protection report joined across backups, schedules, and restore jobs. Do NOT use it for per-job status inspection; use 'analytics --type backups' or 'tail --resource backup-schedules'. |

## Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| `hybrid` (local BM25 sparse) | Sparse encoding can't be verified in dogfood, live index is dense-only, pain speculative | `text-query` |
| `diff-index` (A vs B live compare) | Monthly cadence; change detection already `snapshot diff` | `snapshot` |
| `namespace-report` | Without time axis duplicates analytics; with time axis it's snapshot | `snapshot` |
| `host` | Thin wrapper of describe's host field | `text-query` |
| `pipeline-report` | Same shape as coverage; analytics covers imports | `coverage` |
| `reconcile` | Re-sync dominates; sync + stale hints cover it | framework `sync` |
