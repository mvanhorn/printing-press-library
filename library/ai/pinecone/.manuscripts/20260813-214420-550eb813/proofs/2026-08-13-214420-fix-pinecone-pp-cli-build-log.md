# Pinecone CLI Build Log

Manifest transcendence rows: 7 planned, 7 built. Phase 3 complete.

## What was built
- **Priority 0 (foundation):** Generator-emitted 95-endpoint CLI from 8 official Pinecone OpenAPI specs (data, control, inference, admin, metrics, oauth, assistant control/data). Local SQLite store, sync/search/analytics framework, MCP server (code orchestration, 95 tools), learn loop, doctor, auth.
- **Spec enrichment:** Root base URL → `https://api.pinecone.io`; data-plane resources → `https://{index_host}`; canonical auth env var `PINECONE_API_KEY` (Api-Key header); `X-Pinecone-Api-Version: 2026-04` required header wired; `cli_description` set to narrative headline.
- **Priority 1 (absorb):** All 95 endpoints across 27 resources (indexes, vectors, namespaces, collections, backups, backup-schedules, restore-jobs, bulk imports, records, embed, rerank, models, admin, oauth, assistants, chat, files, operations, metrics) — matches every `pc` CLI and MCP command.
- **Priority 2 (transcend, 7/7 built, hand-code):**
  1. `text-query <index> --text` — embed→query pipe (inference embed + per-index-host query), dimension-aware model selection, graceful empty result when no hosted model matches the index dimension.
  2. `snapshot <index>` / `snapshot diff` — point-in-time index state (per-namespace counts, config, host) into local SQLite `pp_snapshots` table, time-windowed diff with total delta.
  3. `prune <index> --older-than 90d [--apply]` — selects stale vectors by metadata timestamp from local store, dry-run default, batched delete-by-ID with `--apply`, records runs in `pp_prune_runs`.
  4. `cascade --indexes a,b --text` — embeds once, queries N indexes in parallel, merges results deduped by ID (best score wins), partial-failure accounting.
  5. `usage --index --since` — growth/day + 30d projection + per-namespace shifts from snapshot history.
  6. `check-vectors --index --file` — validates dimension/duplicate-ID/sparse-shape against live index config before upsert.
  7. `coverage` — backups × backup-schedules × restore-jobs × indexes protection matrix from local store.
- **Priority 3 (polish):** replaced placeholder example values (`example-value` → `2026-04`), fixed flag declarations to StringVar for verify-skill, `snapshot diff` as real subcommand, graceful dimension fallbacks, `pp:no-error-path-probe` on feedback/prune, required-input exit 2 on prune bare.

## What was intentionally deferred
- Integrated-embedding `records search/upsert` live test — the test account's only index has no integrated inference configured (400 INVALID_ARGUMENT is expected; command works against integrated indexes).
- Admin API live tests — requires service-account Bearer credentials not available (documented; commands generated and dry-run verified).
- OAuth token live test — requires service-account client-id/secret (401 access_denied expected with dummy creds; typed exit 4).

## Skipped body fields / generator limitations
- None blocking. Multi-spec merge set `description: "Combined CLI for multiple API services"` — fixed via `cli_description` + spec `description` override.
- `--force` regen re-injects `newNovel*Cmd` scaffold lines into root.go; resolved by naming novel constructors `newNovel*Cmd` to match the template contract.
