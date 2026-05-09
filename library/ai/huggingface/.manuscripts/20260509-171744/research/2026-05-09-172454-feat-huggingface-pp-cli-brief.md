# HuggingFace CLI Brief

## API Identity
- **Domain:** Model-intel CLI for Rick's local-inference stack. NOT an HF API wrapper (that's `huggingface_hub`). HF is the primary data source; the product is opinionated stack-aware research.
- **Users:** Rick @ CC (interactive); JARVIS / Henry / Coulson / codemonkey (autonomous agents in OpenClaw); future agents in Hermes / OpenCode.
- **Data profile:** HF model cards, file trees, config.json, trending/search lists. Local joins: openclaw.json (current models), model-eval-harness results, on-disk HF cache.

## Reachability Risk
- **None.** HF REST API is public, well-documented, no bot detection. Anonymous read works for every endpoint we touch. Rate limits are generous with token (cached at `~/.cache/huggingface/token`, approved for read-only Phase 5 only).

## Top Workflows
1. **"Should I switch from what I run today?"** — `hf vs-current <candidate>` against `data/openclaw.json` returns arch/size/license/backend delta + replacement verdict per agent role.
2. **"Find quants of X that fit my hardware"** — `hf find-quants <base> --max-size 25g --uploaders unsloth,bartowski,mradermacher` returns sorted variants.
3. **"What's hot in my size class?"** — `hf trending --size 20b-40b --library gguf --since 7d`.
4. **"Will this load on my backend?"** — `hf backend-check <id>` consults bundled matrix + arch from config.json.
5. **"Has this been benchmarked already?"** — `hf bench-history <id>` joins HF id with local harness results to stop re-evaluating known losers.
6. **"Notify me when uploader X drops a new model in size-class Y"** — `hf watch` + cron-callable `hf watch-poll` → MC API alert pipeline.

## Codebase Intelligence
- HF REST API mechanics fully documented at `research/huggingface-api-mechanics.md` (~330 lines).
- 9 endpoints in play: `/api/models` (list/search), `/api/models/{id}` (card), `/api/models/{id}/tree/{rev}` (file list), `/api/models/{id}/raw/{rev}/{path}` (config/README), `/api/users/{user}/overview` (uploader rep).
- Auth: `Authorization: Bearer <HF_TOKEN>`. Anon: ~5K req/day soft limit; authed: significantly higher.
- Rate-limit recovery: RFC draft `RateLimit: "api";r=0;t=N` header (NOT `X-RateLimit-*`). 429 with `Retry-After`.
- Sizes require `?blobs=true` query (listings omit them by default even with `expand=siblings`). For GGUF repos, `gguf.total` field is cheapest size source.
- `base_model` is in `cardData.base_model` + tags (`base_model:<id>`, `base_model:quantized:<id>`) — NOT a server-side filter. Client-side filter required for derivatives + find-quants.
- Cache layout: `~/.cache/huggingface/hub/models--<org>--<name>/snapshots/<sha>/...` — split on last `--` to recover org/name.
- Two unresolved: gated `auto` vs `manual` UX, and `/revision/{rev}` vs `?revision=` interchangeability — flagged for Phase 5 smoke.

## Data Layer
- **Primary entities:** `models` (HF cards), `quants` (derived view: GGUF siblings of a base), `uploaders` (aggregate), `watch_targets`, `watch_events`, `local_models` (on-disk inventory), `bench_runs` (joined from harness).
- **Sync cursor:** `lastModified` per model card; `watch-cursor.json` stores last-poll timestamp per target.
- **FTS/search:** SQLite FTS5 over cached cards (id, tags, description, base_model). Stops re-fetching for `compare-quants` / `bench-history` cross-refs.
- **State files:** `~/.local/state/hf-cli/{watch.json, watch-cursor.json, card-cache/, local-index.json, rate-limit-bucket.json, .lock, backend-support.override.json}`. All flock-guarded. `--no-write` bypass.

## User Vision
Rick wrote the seed plan as the authoritative spec. 15 commands across 3 groups (discovery / intel-loop / runtime utility) plus multi-runtime portability constraints. Research filled in HF mechanics; the surface is locked. See `docs/plans/2026-05-09-hf-cli-printing-press-seed.md`.

## Source Priority
Single source: HF REST API. Local config (`data/openclaw.json`) and harness data (`workspace/scripts/model-eval-harness/`) are read-only joins, not data sources.

## Product Thesis
- **Name:** `huggingface-pp-cli` (binary: `hf`)
- **Why it exists:** Existing HF CLIs (`huggingface_hub` CLI, `hf-go-cli`, etc.) are API mirrors. None answer "would this run on my backend / replace what I run / has this been benched / will it fit." Stack-aware verdicts close the loop between model discovery and operational decisions for Rick's local-inference stack and the JARVIS agent fleet.
- **Differentiator:** Bundled backend-support matrix with citations + `vs-current` / `bench-history` / `find-feature` / `watch` close-the-loop primitives. No other tool encodes Rick's heuristics (trusted uploaders, size classes, MoE active params, MTP-readiness).

## Build Priorities
1. **P0 — scaffolding & data layer.** Generated config, auth (HF_TOKEN), HTTP client with rate-limit handling, doctor, store schema, MCP tree, agent-context. Bundled `backend-support.json` data file.
2. **P1 — Group A discovery (8 commands).** `find-quants`, `trending`, `model-card`, `derivatives`, `uploader-rep`, `compare-quants`, `eval-candidates`, `find-feature`. These are the core research commands.
3. **P2 — Group B intel loop (5 commands).** `vs-current`, `backend-check`, `bench-history`, `watch` + `watch-poll`, `local`. Stack-aware joins.
4. **P3 — Group C runtime utility (2 commands).** `doctor`, `schema`. Multi-runtime probe + introspection.
5. **P4 — polish.** README, SKILL.md, scoped Phase 5 smoke against live HF + cached token.
