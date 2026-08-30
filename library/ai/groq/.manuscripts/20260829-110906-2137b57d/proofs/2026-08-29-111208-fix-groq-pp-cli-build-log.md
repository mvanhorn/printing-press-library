# Groq CLI Build Log

Run: 20260829-110906-2137b57d
CLI: `groq-pp-cli`

Manifest transcendence rows: 6 planned, 6 built. Phase 3 passes.

## What was built

**Priority 0 (data layer):** Generator-emitted SQLite store with typed tables (`audio`, `batches`, `chat`, `files`, `fine_tunings`, `models`, `responses`) + generic `resources` table. Sync wired for models/files/batches/fine-tunings; chat completions write-behind into the `chat` ledger.

**Priority 1 (absorb — 19 features):** Full generated endpoint surface after spec enrichment:
- Resource re-mapping via `x-pp-resource` (chat, responses, audio, models, embeddings, reranking, batches, files, fine-tunings).
- OperationId renames → clean endpoint names.
- Multipart bodies for audio transcription/translation and files upload (removed top-level oneOf + property anyOf that caused body skips).
- Removed `citation_options` default `"enabled"` (broke chat completions on models without compound features).
- Auth: `GROQ_API_KEY` canonical env var; adaptive rate-limiting (`x-pp-default-rate-limit: auto`); cache freshness for the models catalog; learn seeds for model entity lookup.

**Priority 2 (transcendence — 6 features, all hand-coded):**
1. `rate-limits` — one minimal 1-token completion reads x-ratelimit-* headers; live data source.
2. `costs` — aggregates local chat ledger usage joined to synced catalog pricing (builtin fallback map); local data source.
3. `compare` — parallel fan-out across N models, latency/tps/usage/cost ranking, partial-failure accounting, serialized ledger writes.
4. `batch validate` — computed .jsonl validator with per-line schema checks + token/cost estimates; zero API calls.
5. `batch diagnose` — fetches batch metadata + results, tabulates status/error breakdowns, derives non-200 status for null-response failures.
6. `audio batch` — paced bulk transcribe/translate/speech with results manifest and base64 binary-envelope unwrap for speech output.

**Priority 3 (polish):** Verify-friendly RunE patterns (help-only → dry-run → required-input → work), declared positional args, behavioral tests for all pure-logic novel functions, doc fixes (config path, searchable-history wording).

## What was intentionally deferred

- Batch API endpoints could not be exercised live on this account (Free tier; `not_available_for_plan`). Command surface is correct; verification requires a Developer-plan key.
- `audio speech` binary output requires `--deliver file:<path>` or stdout redirect under structured flags (by design of binary endpoints).

## Generator limitations found

- `comm_health.go.tmpl` workflow template missing from installed binary (skipped with warning).
- Multipart request bodies with `oneOf`/`anyOf` are skipped by the generator; simplified schemas workaround.
- OpenAPI `default: "enabled"` string fields are sent unconditionally in generated bodies (citation_options bug) — surfaced and fixed at the spec level.
