# Ollama Cloud CLI — Absorb Manifest

## Source Tools Surveyed

| Tool | URL | Surface |
|---|---|---|
| `ollama` (official) | https://github.com/ollama/ollama | Native daemon + cloud client. CLI: pull/push/run/list/show/ps/cp/rm + chat/embed via API. |
| `ollama-js` | https://github.com/ollama/ollama-js | Official JS SDK. chat, generate, embed, list, show, ps, pull, push, copy, delete. |
| `ollama-python` | https://github.com/ollama/ollama-python | Official Python SDK. Same surface. |
| `litellm` | https://github.com/BerriAI/litellm | Cross-provider routing with hardcoded heuristics ("auto" mode picks GPT-3.5 vs GPT-4 by length only). Closest competitor to `advise`. |
| `openai-python` | https://github.com/openai/openai-python | Reference for OpenAI-compat surface (`/v1/chat/completions`, `/v1/embeddings`, `/v1/models`). |
| `aichat` | https://github.com/sigoden/aichat | Multi-provider CLI with config-based model selection. No prompt-driven routing. |

No existing tool combines (a) live Ollama Cloud catalog, (b) prompt-feature scoring, (c) session-context awareness, (d) cost+latency-aware routing, (e) JSON-shaped `why` output. `advise` is genuinely novel.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Native chat (`/api/chat`) | ollama-js `.chat()` | `chat --model X --messages-file F --stream` | `--dry-run`, `--json`, typed exit codes, 429 surfaces with `upgrade` hint |
| 2 | OpenAI-compat chat (`/v1/chat/completions`) | openai-python | `completions --model X ...` | Same flags as `chat`; identical schema reduces switching cost |
| 3 | Native embed (`/api/embeddings`) | ollama-js `.embeddings()` | `embed --model X --input "..." [--stdin]` | stdin batch (one input per line), `--select embedding[*]` |
| 4 | OpenAI-compat embed (`/v1/embeddings`) | openai-python | `embed --openai-compat` flag | Single command, two schemas |
| 5 | List catalog (`/api/tags`) | ollama CLI `ollama list` | `tags [--family qwen] [--supports-tools]` | Local filter, `--json`, persisted snapshot to SQLite for offline `advise` |
| 6 | List catalog (`/v1/models`) | openai-python `models.list()` | `models` (alias of `tags --openai-compat`) | Same |
| 7 | Show running models (`/api/ps`) | ollama CLI `ollama ps` | `ps [--json]` | Same, normalized output |
| 8 | Show model details | ollama CLI `ollama show` | `show <model>` | Pulls from cached `models.json` overlay too — gives strengths/ctx/price |
| 9 | Auth doctor | (no existing equivalent) | `doctor` | Probes auth, catalog, optional 1-token chat. Surfaces 429 weekly-cap clearly. |
| 10 | Search models (FTS) | (no existing equivalent) | `search "coding long context"` | SQLite FTS5 over name + family + strengths |
| 11 | Cross-provider router | litellm `auto` mode | `advise` (see transcendence) | Live catalog, prompt features, session context, stable JSON output |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This | Score |
|---|---|---|---|---|
| 1 | Model advisor | `advise --prompt-file P [--session-file S] --task-hint H ...` | Combines live catalog + heuristic prompt-feature extraction + curated portable model metadata + optional cheap meta-LLM tiebreak + stable JSON output. Litellm's `auto` mode is hardcoded length-based. No other tool emits a `why` + `alternatives[]` envelope. | 10 |
| 2 | Side-by-side comparison | `compare --prompt-file P --models A,B,C` | Runs the same prompt against N models in parallel, emits side-by-side response + tokens + latency + cost. Requires the same catalog metadata `advise` uses. Useful for calibrating `advise`. | 8 |
| 3 | Advisor explain | `advise ... --explain` (or `model-explain`) | Trace of the scoring: feature extraction → per-model scores → filter passes → tiebreak. Renders both as JSON (machine) and markdown (human). Only possible because `advise` is heuristic-first (auditable). | 8 |
| 4 | Advisor replay | `advise-replay --log advisor-log.jsonl --judge-with gpt-oss:120b` | Replays recommendations against a judge LLM scoring whether the advisor's pick handled the prompt better than alternatives. Powers the engagement-canary follow-up. | 7 |
| 5 | Budget probe | `doctor --budget` (extends doctor) | Detects weekly-cap proximity by issuing a 1-token probe and parsing 429 prose. Surfaces "free tier exhausted: X" actionable error before chat workflows blow up. | 6 |
| 6 | Cost trace | `cost-trace --log advisor-log.jsonl --since 7d` | Aggregates `est_cost_usd` from JSONL log; compares per-model and per-task-hint. Powers "should I upgrade?" decisions. | 5 |

Total: **11 absorbed + 6 transcendence = 17 features**. The `advise` verb is the headliner; everything else either composes with it (compare, advise-replay, cost-trace) or is foundation (tags, chat, embed).

## Stubs

No stubs. All 17 features are shipping-scope in v1.

## Confidence

- `advise` algorithm: high confidence on heuristic scoring; medium on cheap-meta-LLM tiebreak (Ollama Cloud free-tier 429 may interfere — handled by deterministic fallback per design).
- `compare`: high confidence; mechanical.
- `advise-replay`: medium — depends on having a judge LLM with quota. Phase 5 may skip live judging.
- Catalog persistence: high; SQLite + FTS5 already standard pattern in printing-press.
