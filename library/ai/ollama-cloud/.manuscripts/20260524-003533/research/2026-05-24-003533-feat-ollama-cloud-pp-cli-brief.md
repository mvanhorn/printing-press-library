# Ollama Cloud CLI Brief

## API Identity
- **Domain:** `https://ollama.com` — Ollama's hosted inference API. OpenAI-compatible at `/v1/*`, native Ollama protocol at `/api/*`.
- **Users:** developers who want hosted access to large open-weights models (Qwen3, GPT-OSS, DeepSeek-V3, Kimi-K2, GLM, Llama, Gemma, etc.) without running local GPUs. Free tier is rate-limited (weekly quota, observed `429 weekly usage limit` on chat already).
- **Data profile:** stateless chat completions, embeddings, model catalog. No persistent server-side conversations or fine-tunes (yet). Catalog drifts monthly as Ollama adds/removes hosted models.

## Reachability Risk
- **None for catalog** (`/api/tags`, `/v1/models` return 200 with or without auth).
- **Low for chat/embed** (free-tier weekly cap; bearer-auth required). Probe of `gpt-oss:120b` returned `HTTP 429: you have reached your weekly usage limit, upgrade for higher limits`. Phase 5 live smoke must use a model with remaining quota OR be capped at 1-token outputs OR accept that chat lives test may 429.
- No anti-bot / Cloudflare challenge in the way. Clean REST.

## Top Workflows
1. **Pick the right hosted model for a prompt** — *the unsolved decision in the user's multi-backend stack* (local llama-server, ollama-cloud, openrouter, anthropic). The `advise` verb is the headline.
2. **One-shot chat against a chosen model** — `chat --model qwen3-coder:480b --messages-file p.json` or OpenAI-compat `completions`.
3. **Generate embeddings** — `embed --model embeddinggemma --input "..."`.
4. **Inspect the live catalog** — `tags` (native) and `models` (OpenAI-compat) for "what can I run today, what's its context, what does it cost".
5. **See what's running** — `ps` for any session-active models.

## Table Stakes
- Bearer-auth via `OLLAMA_CLOUD_API_KEY` (distinct from local Ollama; gateway/JARVIS uses this exact name in `data/.env`).
- Both native (`/api/chat`, `/api/generate`, `/api/embeddings`) and OpenAI-compat (`/v1/chat/completions`, `/v1/embeddings`) surfaces.
- Streaming support on chat (`stream:true`).
- Standard error envelope: `{"error":"..."}`.

## Data Layer
- **Primary entities:** `Model` (id, family, ctx_window, supports_tools, supports_vision, owner), `AdvisorLog` (prompt_hash, features, recommendation, alternatives, actual_chosen_model, judge_score), `RunningModel` (from `/api/ps`).
- **Sync cursor:** none upstream — catalog is small enough to refresh-on-call. Store a snapshot per `tags` invocation so `advise` can run fully offline against last-known catalog.
- **FTS/search:** over model names, families, strengths tags. Powers `advise` filtering and the standard `search` command.

## Codebase Intelligence
- Source: Ollama OSS server (https://github.com/ollama/ollama) — REST routes in `server/routes.go`. The hosted Ollama Cloud API mirrors the local Ollama daemon's surface exactly; same JSON shapes, same path layout.
- Auth: hosted-only adds `Authorization: Bearer <key>`. Local-daemon has no auth. env-var precedent: hosted users use `OLLAMA_API_KEY` upstream; this codebase already uses `OLLAMA_CLOUD_API_KEY` to disambiguate from any local-daemon variable.
- Data model: chat = OpenAI-shaped `{messages, model, stream, options}`. Embeddings = `{model, prompt}` (native) or `{model, input}` (OpenAI-compat). Tags = `{models:[{name, size, digest, modified_at, details:{...}}]}`.
- Rate limiting: free-tier weekly cap surfaces as `429` with prose error mentioning `upgrade`. No `Retry-After` header observed.

## User Vision
**This is Rick's design (delivered up-front, 4FW-derived). The wrapper is table stakes; the `advise` verb is leverage.**

### The unsolved decision
Rick runs a multi-backend stack:
- Local llama-server (Qwen3.6 35B-A3B, Gemma-4-e4b, EmbeddingGemma-300M)
- Ollama Cloud (qwen3-coder:480b, gpt-oss:120b, kimi-k2:1t, deepseek-v3.1:671b, glm-4.6, qwen3:235b, qwen3-vl:235b, …)
- OpenRouter (paid)
- Anthropic Claude (paid)

The gateway agent roster (`data/openclaw.json`) handles coarse routing (one model per persona). What's missing: **per-prompt, per-session model selection** — "given THIS prompt, THIS session context, THIS budget, which model right now?"

### The `advise` verb (must ship in v1, NOT as a stub)
```
ollama-cloud-pp-cli advise \
  --prompt-file P [--session-file S] \
  --task-hint coding|reasoning|long-context|cheap|multilingual|vision \
  --budget-remaining-usd N --max-latency-ms N --require-tools \
  --exclude m1,m2 --format json|md
```

Output (stable JSON schema, this is the C1-portable interface other consumers will read):
```json
{
  "recommended": "qwen3-coder:480b",
  "why": "code-fence density 0.34, prompt token count 4200, task-hint coding; gpt-oss:120b scored within 2% but lacks long-ctx headroom",
  "alternatives": [{"model":"gpt-oss:120b","score":0.88,"why":"..."}],
  "est_input_tokens": 4200, "est_output_tokens": 1500,
  "est_cost_usd": 0.00, "est_latency_ms": 4200,
  "fallback": "gpt-oss:120b"
}
```

### Algorithm (heuristic-first, LLM-fallback)
1. Local feature extraction (no network): token count via tokenizer, code-fence density, language detection, reasoning-depth signals, tool-use mentions, attachment count from session.
2. Live catalog from `/api/tags` (or `/v1/models`), overlaid with curated `models.json` per-model metadata: `{strengths[], ctx_window, price_in_per_1m, price_out_per_1m, latency_p50_ms, supports_tools, supports_vision, family}`. Schema versioned + provider-portable (future consolidation across local/OpenRouter/Anthropic).
3. Weighted scoring: task-hint match, ctx-fit, budget-fit, latency-fit, tool-support, code-vs-prose suitability. Hard-filter by `--exclude` and constraints.
4. If top-2 scores within 5% → optional cheap meta-LLM tiebreak (`gpt-oss:120b` or `gpt-oss:20b`), capped at 300 tokens, deterministic fallback on fail/disable.
5. Emit JSON. Append to `~/.local/state/ollama-cloud-pp-cli/advisor-log.jsonl`.

### 4FW summary baked into this brief
- **First principles:** model-selection is the unsolved decision; wrapper is table stakes, advise is leverage.
- **Second order:** stable JSON output → consumable by gateway routing middleware, /nerd dispatch, CC subagent picks; positions for cross-provider consolidation later.
- **Inversion:** catalog rot (mitigated by live `/api/tags` + versioned `models.json`); recursive hallucination (mitigated by heuristic-first + cheap meta-LLM cap); no engagement (mitigated by ship-time dogfood); provider-lock (mitigated by portable schema).
- **Recursive:** don't split into separate `model-advisor` tool yet — keep portable schema so consolidation is cheap later, not forced now.

### Engagement plan (P3 active-dogfood)
- Ship-time dogfood: run `advise` against a REAL prompt (this conversation's prompt is the natural fixture). Include recommendation in the shipcheck output.
- Follow-up (NOT a v1 blocker): register a daily LaunchAgent comparing `advise` recommendations vs. actual-model-chosen in gateway logs at `workspace/memory/advisor-canary.jsonl`. Acceptance: 5-20% divergence is the sweet spot.

## Product Thesis
- **Name:** `ollama-cloud-pp-cli` (binary), display name **Ollama Cloud**.
- **Headline:** *Routes every prompt to the right hosted Ollama model based on prompt features, session context, budget, and latency — wraps `/api/chat`, `/api/embeddings`, `/api/tags`, `/api/ps`, plus the OpenAI-compatible mirror. The advisor is the differentiator.*
- **Why it should exist:** Ollama Cloud's catalog spans 20+ models (coding, reasoning, long-context, cheap, multilingual, vision) with no built-in picker. Same sweet-spot fragmentation as OpenRouter. The CLI has the only `advise` verb that combines live catalog + prompt features + session context + portable cost/latency metadata to give an answer with `why` and `alternatives[]`.

## Build Priorities
1. **`advise`** — Phase 3 hand-built. Heuristic scorer + curated `models.json` + optional cheap meta-LLM tiebreak + JSONL logging. THIS IS THE LEVERAGE.
2. **`tags` + `models`** — wraps `/api/tags` (native) and `/v1/models` (OpenAI-compat). Cached snapshot writable to local SQLite for offline `advise`.
3. **`chat` + `completions`** — native and OpenAI-compat chat. `--stream`, `--messages-file`, `--system`, `--temperature`, `--max-tokens`. Surfaces `429 weekly limit` with actionable error.
4. **`embed`** — `/api/embeddings` and `/v1/embeddings`. `--input` accepts string or stdin batch.
5. **`ps`** — `/api/ps` running models.
6. **`doctor`** — auth probe, catalog probe, optional 1-token chat probe (skipped if free-tier 429).

## Source Priority
Single source. Not a combo CLI.

## models.json schema (v1, ships with CLI)
```yaml
schema_version: 1
provider: ollama-cloud
fetched_at: <ISO>
models:
  - id: qwen3-coder:480b
    family: qwen
    ctx_window: 262144
    price_in_per_1m: 0.0   # free tier; documented "upgrade" tier TBD
    price_out_per_1m: 0.0
    latency_p50_ms: 4200
    supports_tools: true
    supports_vision: false
    strengths: [coding, long-context, agentic]
  - id: qwen3-vl:235b-instruct
    family: qwen
    ctx_window: 131072
    supports_vision: true
    strengths: [vision, multimodal, instruction-following]
  # ...
```
Schema is **provider-portable**: a future `local-llama` or `openrouter` overlay drops in the same shape, only `provider` differs. This is the C1-portable interface other tools (gateway, /nerd, CC subagent picks) will consume — that's why it ships baked in v1, not as a follow-up.
