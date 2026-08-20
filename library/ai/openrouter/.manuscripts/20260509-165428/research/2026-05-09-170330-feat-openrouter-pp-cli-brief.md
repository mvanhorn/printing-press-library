# OpenRouter CLI Brief

## API Identity
- Domain: AI inference aggregator — single auth/billing front for 400+ LLMs across providers (Anthropic, OpenAI, Google, Meta, Mistral, etc.)
- Users: developers building agents, infra teams managing multi-provider routing, cost-conscious platforms
- Data profile: model catalog (~400+ rows, ~425KB), per-call generation records, usage activity time-series, account billing state

## Reachability Risk
- **None.** OpenAPI 3.1.0 spec at `https://openrouter.ai/openapi.json` (978KB, 41 paths). Live probes against `/credits`, `/key`, `/models` all return 200. Bearer auth works with `OPENROUTER_API_KEY`.

## Top Workflows (this CLI's niche)
1. **Agent-read introspection** — `Bash(openrouter creds --llm)` returning ~150 tokens for cron + scientist + scan-pipeline workflows. Replaces manual curl + LLM-format-the-response with one deterministic call.
2. **Live availability sampling** — `openrouter providers --json` to feed `provider-suspension.ts` with real availability before dispatch (preempt 429s).
3. **Cost-attribution journaling** — every CLI call logged via tier-2.0 effect-tool-call logger; per-cron billing breakdown becomes a one-liner.
4. **Generation forensics** — `openrouter generation <id>` for replay/debugging when something blew tokens unexpectedly.
5. **Threshold-gated composition** — `openrouter creds --threshold 5usd && expensive-op` as Unix pre-flight gate.

## Table Stakes (absorbed from competitor surface)
From mrgoonie/openrouter-cli (18 stars, "agent-friendly by default"):
- creds / credits show
- models list (with browse/search)
- generations get
- keys list/create/delete (admin)
- analytics show (usage by endpoint)
- providers list
- auth login (OAuth PKCE) / set-key / status / whoami
- config doctor / get / set
- chat send (we deprioritize — out of scope for v1)
- guardrails / video / rerank / embeddings / responses (beta — defer)

From grahamking/ort (36 stars, chat-focused):
- model + provider sort preference (priority-by-price/throughput/latency)
- continue-conversation flag (`-c`)
- not introspection-focused — niche differs

From simonw/llm-openrouter (LLM plugin):
- model browsing
- inherits LLM's plugin ergonomics — different runtime

Net: mrgoonie covers the broadest introspection surface. None of them ship `--llm` mode, `--record/--replay` fixtures, `--scrub` redaction, or a Printing-Press MCP wrapper. Those are our differentiators.

## Data Layer
- **Primary entities** (worth a local SQLite store):
  - `models` — id, name, context_length, pricing.prompt, pricing.completion, top_provider, supported_parameters[], modality (FTS5 over name/description for `openrouter models search "qwen"`)
  - `generations` — id, model, tokens_prompt, tokens_completion, cost, latency, created_at (cache for forensics + per-cron rollups)
  - `usage` — daily/weekly/monthly snapshot rows (sync from `/key` endpoint, computed from `/activity`)
  - `providers` — name, status, last_seen (for suspension logic)
- **Sync cursor:** `models` updated 24h TTL (catalog rarely changes more than daily); `generations` append-only by id; `usage` daily snapshot at sync time.
- **FTS5/search:** model name + description + tokenizer for fuzzy model lookup.

## Codebase Intelligence
- Source: openapi.json fetched live. 41 paths. Auth schemes: `apiKey` + `bearer`.
- Auth: `Authorization: Bearer <OPENROUTER_API_KEY>`. Distinguish from `OPENROUTER_MANAGEMENT_KEY` (admin operations like `/keys` POST/DELETE — mrgoonie's CLI splits these correctly).
- Data model: every resource has `data` envelope (`{"data": {...}}` or `{"data": [...]}`). Worth a generic unwrap helper.
- Rate limiting: per `/key` response, accounts have `limit`/`limit_remaining`/`limit_reset` (weekly here). Distinct from per-model TPM/RPM.
- Architecture: REST + Bearer, no GraphQL, no streaming for introspection (only chat/completions streams).

## User Vision (verbatim from briefing)
> Use our plan at `/Users/rick/Documents/OpenClawDocker/docs/plans/2026-05-09-printing-press-cli-default-architecture.md` as the brief. Surface (creds/models/usage/generation/key/limits), conventions (`--llm`, `--record`/`--replay`, `--scrub`), wires (provider-suspension, host-watchdog, scan-pipeline). API key is `OPENROUTER_API_KEY`.

The plan establishes `openrouter` as the proving ground for a **CLI-default architecture** decision: every external API integration → Printing-Press CLI + MCP wrapper rather than Node plugin in the gateway. Three Printing-Press conventions to crystallize via post-build retro: `--llm`, `--record/--replay`, `--scrub`. See companion wiki insight at `workspace/jarvis-wiki/cc-ingest/2026-05-09-cli-default-decouples-capability-from-merge-tax.md`.

## Product Thesis
- **Name:** `openrouter-pp-cli` (binary: `openrouter`)
- **Display name:** OpenRouter
- **Headline:** "Agent-first OpenRouter introspection — `--llm` mode keeps cron-driven cost/availability checks under 200 tokens, with local SQLite catalog and free MCP server."
- **Why it should exist:** The 7 existing OpenRouter CLIs are all chat-REPL-shaped. None target agent-loop introspection (`creds`, `models`, `usage`, `generation`, `providers`) with terse output, fixture replay, redaction, or zero-config MCP. This CLI fills the agent-introspection niche — sub-200-token outputs designed for `Bash(...)` consumers, not human chat.

## Build Priorities
1. **P0 foundation:** SQLite store (models, generations, usage, providers) + sync command pulling from `/models`, `/key`, `/activity`, `/providers`. FTS5 over models.
2. **P1 absorb (introspection surface):** `creds`, `key info`, `models list/get/search`, `generation get`, `usage [--since]`, `providers list`, `endpoints get <model>`, `keys list/create/delete` (admin/management-key-gated), `auth set-key/status/whoami`, `config get/set/doctor`, `doctor` (health).
3. **P1 absorb (inference surface, lower priority):** chat completions, embeddings, rerank — only enough to round out the API; not the headline.
4. **P2 transcend:**
   - `--llm` mode on every command (terse k:v output, single-line where possible, ~150 tok per call)
   - `--record <file>` / `--replay <file>` fixture mode
   - `--scrub <fields>` field-level redaction (default-on for `creds`/`key`)
   - `creds --threshold <usd>` → typed exit code 7 if below (composable pre-flight)
   - `cost-by` aggregation (group cached generations by model/day/cron-name) — only possible because of local store
   - `providers watch` — TTL'd cache + diff alarm when provider falls out of available set
   - `models search` over FTS5 (offline; works without network)
5. **P3 polish:** README cookbook for cron use, agent recipes, SKILL trigger phrases for "openrouter creds", "check budget", etc.

## Reachability check (Phase 1.9 prerun)
- `/credits` → 200, `{"total_credits":700,"total_usage":700.08}` (Rick is at-cap; v1 surface is read-only so this doesn't block testing)
- `/key` → 200, full key info
- `/models` → 200, ~425KB (~400 models)
- All read-only endpoints work; no rate-limit / bot-protection / auth issues
