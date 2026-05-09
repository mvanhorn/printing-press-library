# OpenRouter CLI — Absorb Manifest

## Absorbed (24 rows)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Account credits balance | mrgoonie credits show | `creds` (GET /credits) | `--llm`, `--threshold <usd>` typed exit 7 |
| 2 | Current key info + limits | mrgoonie auth whoami | `key info` (GET /key) | `--scrub` default-on, structured limit/usage |
| 3 | List all models | mrgoonie/ort models list | `models list` (GET /models) | local SQLite cache + FTS5, `--available`, `--provider`, `--max-cost` |
| 4 | Models by user prefs | mrgoonie | `models user` (GET /models/user) | typed JSON, --select |
| 5 | Model count | docs | `models count` (GET /models/count) | scriptable |
| 6 | Model endpoints (per provider) | docs | `endpoints get <author/slug>` (GET /models/{a}/{s}/endpoints) | --json, --select |
| 7 | ZDR endpoints preview | docs | `endpoints zdr` (GET /endpoints/zdr) | scriptable |
| 8 | Generation lookup by id | mrgoonie generations get | `generation <id>` (GET /generation) | `--include-content` (combines /generation + /generation/content) |
| 9 | Usage activity | mrgoonie analytics show | `usage [--since 7d] [--by model\|provider] [--top N]` (GET /activity) | `--llm` aggregations, persistent local store |
| 10 | Provider list | mrgoonie providers list | `providers list` (GET /providers) | local cache, status diff |
| 11 | Sub-keys list | mrgoonie keys list | `keys list` (GET /keys) | requires management key, `--scrub` default |
| 12 | Sub-key get | mrgoonie keys get | `keys get <hash>` (GET /keys/{hash}) | `--scrub` |
| 13 | Sub-key create | mrgoonie keys create | `keys create` (POST /keys) | `--dry-run`, `--label`, idempotency |
| 14 | Sub-key delete | mrgoonie keys delete | `keys delete <hash>` (DELETE /keys/{hash}) | `--dry-run`, confirm |
| 15 | Sub-key update | mrgoonie keys update | `keys update <hash>` (PATCH /keys/{hash}) | `--dry-run` |
| 16 | Auth status / set-key | mrgoonie auth | `auth set-key`, `auth status`, `auth whoami` | env-first, no key files |
| 17 | Health doctor | mrgoonie config doctor | `doctor` | composable: exit codes 0/2/4/5/7 |
| 18 | OAuth PKCE login | mrgoonie auth login | `auth login` (POST /auth/keys, POST /auth/keys/code) | optional; env path is primary (stub) |
| 19 | Embeddings create | mrgoonie embeddings | `embeddings create` (POST /embeddings) | absorb for completeness |
| 20 | Rerank | mrgoonie rerank | `rerank` (POST /rerank) | absorb for completeness |
| 21 | Chat completions | mrgoonie chat send | `chat send` (POST /chat/completions) | absorb but not headline (grahamking/ort owns chat ergonomics) |
| 22 | Messages (Anthropic-compat) | docs | `messages create` (POST /messages) | absorb for completeness |
| 23 | Responses (beta) | mrgoonie responses | `responses create` (POST /responses) | absorb beta |
| 24 | Sync command | (none — novel foundation) | `sync [--full] [--models] [--providers]` | refreshes local SQLite from /models, /providers, /key, /activity |

**Status notes:**
- Row 18 (`auth login` OAuth PKCE) — **(stub for v1)** — env-var path is primary; OAuth flow deferred to v2. README and SKILL must label this honestly.
- Rows 19-23 (inference surface: embeddings, rerank, chat, messages, responses) — **shipping but not headline**. Documented in README as "compat surface; if you want chat-first ergonomics use grahamking/ort." Generated as endpoint-mirror commands; no special transcendence.

## Transcendence (8 rows, all scoring ≥ 6/10 — every one is shipping scope)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Cost-by-cron rollup | `usage cost-by --group cron --since 7d --llm` | 9/10 | Joins local `generations` (synced from /activity) with cron-name tag from tier-2.0 effect-tool-call logger; no competitor has caller attribution |
| 2 | Models query DSL | `models query "tools=true cost.completion<1 ctx>=64k"` | 8/10 | Compiles k=v / k<v / k>=v expressions to SQL WHERE over local catalog; /models has no query params upstream |
| 3 | Providers degraded watch | `providers degraded --json` | 8/10 | Polls /providers + per-model /endpoints, computes degraded set-diff; pipe-feeds provider-suspension.ts (currently reactive in-memory) |
| 4 | Generation cost forensics | `generation explain <id>` | 7/10 | Combines /generation + /generation/content + local pricing → cost-vs-cheapest-provider delta |
| 5 | Cost regression alarm | `usage anomaly --since 24h --baseline 7d --llm` | 7/10 | z-score over per-model daily cost from local generations; leading indicator vs reactive credit-balance alarm |
| 6 | Weekly-cap ETA | `key eta --llm` | 7/10 | /key.{limit,usage,limit_reset} + 7d burn rate → linear projection of cap-trip timestamp |
| 7 | Per-cron budget contract | `budget set <cron> <usd/wk>` / `budget check <cron>` | 6/10 | Local config; `check` returns exit 0 (under) / 8 (over) from tagged generations; depends on #1 plumbing |
| 8 | Endpoint failover map | `endpoints failover <model> --json` | 6/10 | /models/{a}/{s}/endpoints ranked by (status, pricing, p50_latency local); pipe-feeds router |

## Conventions (Printing-Press meta — propose via `/printing-press-retro` after this CLI ships)

- `--llm` mode — terse k:v output, default for `Bash(...)` agent consumers
- `--record <file>` / `--replay <file>` — fixture mode for tests
- `--scrub <fields>` — field-level redaction, default-on for `creds`/`key`/`keys list`/`auth status`

These are NOT per-CLI novel features. They are conventions that should be added to every Printing-Press generated CLI. Implemented here first; retro proposal after Phase 6.

## Sources surveyed

- mrgoonie/openrouter-cli (18★) — broadest introspection surface; `OPENROUTER_API_KEY` + `OPENROUTER_MANAGEMENT_KEY` split (worth absorbing)
- grahamking/ort (36★) — Rust, chat-focused; tmux-aware
- simonw/llm-openrouter — LLM plugin
- OrChat (oop7), mexyusef, bazoocaze, jwill9999, maxxie114 — chat REPLs / niche tools
- physics91/openrouter-mcp + heltonteixeira/openrouterai — MCP servers (chat focus)

None of the seven competitors implement: `--llm` terse mode, fixture replay, field-level scrub, local SQLite catalog, per-caller cost attribution, weekly-cap projection, models query DSL, providers degraded watch.
