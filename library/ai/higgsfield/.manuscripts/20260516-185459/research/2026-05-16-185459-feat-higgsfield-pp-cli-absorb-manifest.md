# Higgsfield CLI — Absorb Manifest

## Sources surveyed
- **higgsfield-ai/cli** (official Go CLI) — installed locally v0.1.40; 12 command groups, 35-model registry, browser device-login
- **higgsfield-ai/higgsfield-client** (Python SDK) — sync + async, 6 status states, file/PIL upload
- **higgsfield-ai/higgsfield-js** (Node v2 SDK) — `Authorization: Key KEY:SECRET`, webhook support, NotEnoughCreditsError
- **Hikhakk/higgsfield-mcp-unified** (community MCP) — both backends, 27-model snapshot, official-vs-web routing
- **geopopos/higgsfield_ai_mcp** (community MCP) — official-only, 5 tools, character CRUD
- **Official MCP** at `mcp.higgsfield.ai` — Claude Desktop/Cowork/Claude Code/OpenClaw/Hermes/NemoClaw
- **Apidog Higgsfield API blog** — error-code reference, status flow

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browser device login | Official CLI `auth login` | `auth login --chrome` (delegates to local `higgsfield` CLI) | Same flow; cached token in local config |
| 2 | Manual token set | Official CLI `auth token` | `auth set-token <jwt-or-api-key>`, `auth status` | Adds source attribution (env vs file) |
| 3 | Logout | Official CLI `auth logout` | `auth logout` | Same |
| 4 | Account status | Official CLI `account status` | `account status` + cached | Offline-readable after first call |
| 5 | Credit transactions | Official CLI `account transactions` | `account transactions --size N`, `account transactions sync` | Persisted ledger, FTS, cost analytics |
| 6 | Workspace list | Official CLI `workspace list` | `workspace list` + cache | Offline-readable |
| 7 | Workspace set/status/unset | Official CLI | `workspace set <id>`, `workspace status`, `workspace unset` | Same |
| 8 | Model list (35 models) | Official CLI `model list` | `model list --type image|video|text` + cache | FTS search, backend-aware filter |
| 9 | Model inspect | Official CLI `model inspect` | `model inspect <model_id>` | Same |
| 10 | Generate create (per model) | Official CLI `generate create <model>` | `generate create <model> --prompt ...` + persist | Job in local store immediately |
| 11 | Generate cost estimate | Official CLI `generate cost <model>` | `generate cost <model> --prompt ...` | Same |
| 12 | Generate get | Official CLI `generate get <job_id>` | `generate get <job_id>` + update store | Cached |
| 13 | Generate list | Official CLI `generate list` | `generate list --type --status --data-source local|live` | Joins local store |
| 14 | Generate wait (poll) | Official CLI `generate wait <id>` | `generate wait --interval 3s --timeout 10m` | Stores intermediate states |
| 15 | Generate cancel | Hikhakk `cancel_job` | `generate cancel <id>` | Same |
| 16 | Upload media | Official CLI `upload` + Python `upload_file/upload_image` | `upload <path>` + records content hash → media_id | Re-upload dedup via hash |
| 17 | Soul ID create | Official CLI `soul-id create --image x5` | `soul-id create --name ... --image ...` (>=5 inputs) | Tracks training time, links to creator workflow |
| 18 | Soul ID list | Official CLI `soul-id list` | `soul-id list` + cache | Offline, FTS over names |
| 19 | Soul ID get/wait | Official CLI | `soul-id get <id>`, `soul-id wait <id>` | Caches results |
| 20 | Marketing Studio assets | Official CLI `marketing-studio` | Mirror subcommands; persist asset library | Local search across assets |
| 21 | Product Photoshoot | Official CLI `product-photoshoot` | Mirror with `--mode` passthrough | Same + stored |
| 22 | Marketplace Cards | Official CLI `marketplace-cards` | Mirror | Same |
| 23 | Generate speech video | Hikhakk `generate_speech_video` (official only) | `generate speech-video --prompt ...` | Same |
| 24 | Submit with webhook | Python SDK `submit(webhook_url=...)` + JS v2 | `generate create --webhook <url> --webhook-secret <s>` | Platform-only, surfaced as flag |
| 25 | Subscribe (long-poll) | Python SDK `subscribe` / `subscribe_async` | Covered by `generate wait` | Same |
| 26 | Backend routing (model→backend) | Hikhakk `BackendPool.get(spec.backend)` | Built into HTTP client; gated by `HIGGSFIELD_ENABLE_WEB_BACKEND` | Same plus visible from `model inspect` |
| 27 | Per-model param schema | Official CLI MODELS.md | `model inspect <id>` returns full schema | Schema-validated `generate create` |
| 28 | Sync (full + incremental) | Framework default | `sync --full`, `sync` (delta) | Pulls generations, soul_ids, models, workspaces, transactions |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | Batch model fanout | `fanout --prompt "..." --models veo3_1,seedance_2_0,kling3_0` | 9/10 | hand-code | Loops over `--models`, calls real per-model submit endpoint for each, persists `fanout_id` linking returned `request_id`s in SQLite — nothing in the Higgsfield ecosystem offers this primitive |
| 2 | Fanout wait + compare | `fanout wait <fanout_id>`, `fanout compare <fanout_id>` | 8/10 | hand-code | Polls each linked job via real status endpoint, renders side-by-side table from local `generations` joined on `fanout_id`; the official MCP and CLI have no concept of grouped runs |
| 3 | Soul library search | `soul-id search "riggs"` | 9/10 | hand-code | FTS5 over `soul_ids.name` + join into past prompts; ranks by last-used; the official CLI's `soul-id list` is unindexed |
| 4 | Credit guard for batches | `fanout --max-cost 200` (also `--max-cost` on `generate create`) | 8/10 | hand-code | Calls real `generate cost` per model, sums estimates locally, refuses submission if total exceeds cap, prints per-model breakdown; no upstream tool enforces a pre-flight budget |
| 5 | Prompt history search | `search "cinematic riggs"` (framework `search` command) | 8/10 | spec-emits | FTS5 over `generations.prompt`, `model`, `soul_id name`, `transaction memo` — falls out of the standard framework search once data is synced |
| 6 | Soul usage report | `soul-id usage <soul_id>` | 7/10 | hand-code | Local SQL: every generation that used this Soul ID, ordered by date, with thumbnails and total cost; impossible without a local store |
| 7 | Account spend report | `account spend --since 7d --group-by model` | 7/10 | hand-code | Local SQL aggregation over synced `transactions` grouped by linked model; the official `account transactions` returns flat events with no grouping |
| 8 | Generations export | `generate export --since 7d --format csv|jsonl` | 6/10 | hand-code | Local SQL SELECT joined with models/soul_ids, emits CSV/JSONL via standard encoder; agent-friendly pipeline output |

**Stubs:** none planned. Every row above ships fully implemented or `--enable-web-backend`-gated.

**Anti-reimplementation note:** All transcendence rows EITHER read from the local store (rows 2, 3, 5, 6, 7, 8) OR call the real API endpoint (row 1 calls real submit per model; row 4 calls real `cost` per model). No row synthesizes API responses locally.

**Backend coverage:**
- Both surfaces wired (platform + web). Web behind `HIGGSFIELD_ENABLE_WEB_BACKEND=1` gate per Hikhakk MCP convention.
- User has JWT only today; platform endpoints generate but won't return live data in Phase 5 smoke testing — they'll be flagged `BLOCKED_FIXTURE: no platform API key`.
