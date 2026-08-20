## Customer model

**Persona 1: Rick the JARVIS operator** — runs ~30 cron jobs that consume OpenRouter inference (scan-pipeline, autoresearch, scientist loops, perceiver). Every cron tick burns credits; some agents (scan-pipeline) are bursty, others (perceiver) are steady. He hit the weekly cap mid-week recently when llama-35b fell back to ollama-cloud and 42K-token prompts started chronically timing out.

- **Today (without this CLI):** Opens openrouter.ai/activity in a browser tab when something feels off, eyeballs the bar chart, exports CSV, opens it in Numbers, pivots by model. For "what is each cron costing me?" he has nothing — the dashboard groups by model+provider, not by which-cron-fired-the-call. He has a `provider-suspension.ts` module that records 429s in-memory but no way to ask "which provider is currently degraded?" without waiting for the next 429.
- **Weekly ritual:** Sunday evening: scroll activity dashboard, mentally attribute costs, decide whether to retire any cron next week. Mid-week: respond to "credits low" alarm by squinting at the dashboard.
- **Frustration:** Cost-per-cron is invisible. The dashboard groups the wrong dimension; the answer requires joining `generation_id → cron_name` and that join lives nowhere.

**Persona 2: A pre-flight gate in a bash composition** — `expensive-job.sh` runs nightly and burns ~$3 of inference. If credits are below a threshold, it should bail loudly rather than half-execute and 402.

- **Today:** A hand-rolled `curl https://openrouter.ai/api/v1/credits | jq '.data.total_credits - .data.total_usage'` in a `[ ... -gt 5 ]` test, fragile to envelope changes, no typed exit code, no LLM-friendly explanation when it fires.
- **Weekly ritual:** Fires nightly. Maybe twice a year it actually trips, but when it trips silently the rest of the chain wastes an hour.
- **Frustration:** Every team rewrites this curl-jq snippet. None of them handle 5xx vs 402 vs threshold-hit distinctly; they all collapse to "exit 1."

**Persona 3: The scientist agent picking a model** — needs to choose between candidate models for an experiment. Wants "models that support tool calling, under $1/M output, available in the last hour, with context >= 64K." Today she asks main-JARVIS, who hallucinates pricing.

- **Today:** Pastes openrouter.ai/models into Claude and asks; gets a stale answer. Or curls /models, gets 425KB of JSON, runs out of context.
- **Weekly ritual:** New experiment proposal → model shortlist → run. The shortlist step is the bottleneck.
- **Frustration:** No way to express the filter as a query. /models has no query parameters for `supported_parameters`, `pricing.completion < X`, or `context_length >= Y`.

**Persona 4: The provider-suspension feedback loop** — `src/agents/provider-suspension.ts` marks a `provider/model` skipped for 24h on 429. It is reactive (must hit 429 first) and in-memory (resets on gateway restart).

- **Today:** Waits for the 429 to learn. No cross-run memory. No way to preempt.
- **Weekly ritual:** Every gateway run, every model dispatch, every 429.
- **Frustration:** The signal exists upstream — `/providers` reports degraded status, `/endpoints/{model}` reports per-provider availability — but nothing polls it.

## Candidates (pre-cut)

(C1–C16 candidates, see subagent output; only survivors and kills retained for downstream consumption — full text saved here as audit trail.)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|----------|
| 1 | Cost-by-cron rollup | `usage cost-by --group cron --since 7d --llm` | 9/10 | P1 Rick | SQL GROUP BY over local `generations` joined with cron-name tag from tier-2.0 effect-tool-call logger sidecar | Brief Top Workflow #3; no competitor has per-caller attribution |
| 2 | Models query DSL | `models query "tools=true cost.completion<1 ctx>=64k"` | 8/10 | P3 scientist | k=v / k<v / k>=v expression compiles to SQL WHERE over local `models` table; FTS5 fallback for free text | /models has no query params; persona 3's gap |
| 3 | Providers degraded watch | `providers degraded --json` | 8/10 | P4 suspension | Polls `/providers` + per-model `/endpoints` (status field); set-diff vs last poll | Brief Top Workflow #2; current suspension is reactive in-memory |
| 4 | Generation cost forensics | `generation explain <id>` | 7/10 | P1 Rick | `/generation` + `/generation/content` + local `models.pricing` → cost-vs-cheapest-provider | Brief Top Workflow #4; mrgoonie's `generations get` is thin wrapper |
| 5 | Cost regression alarm | `usage anomaly --since 24h --baseline 7d --llm` | 7/10 | P1 Rick | z-score over per-model daily cost from local `generations`; deterministic, no LLM | Persona 1's "credits low" is reactive; this is leading indicator |
| 6 | Weekly-cap ETA | `key eta --llm` | 7/10 | P1 Rick | `/key.{limit,usage,limit_reset}` + 7d burn rate → linear projection of cap-trip timestamp | Rick has hit weekly cap mid-week; no competitor projects |
| 7 | Per-cron budget contract | `budget set <cron> <usd/wk>` / `budget check <cron>` | 6/10 | P1 Rick | Local config table cron-name → weekly cap; `check` returns exit 0/8 from tagged generations | Brief User Vision (e) — per-agent budget contracts via env scoping |
| 8 | Endpoint failover map | `endpoints failover <model> --json` | 6/10 | P4 suspension | `/models/{author}/{slug}/endpoints` parsed + ranked by (status, pricing, p50_latency local) | Multi-provider routing is the OpenRouter-defining pattern |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| C4 Threshold pre-flight gate | Absorbed in #1 (`creds --threshold`). | survivor #6 (`key eta`) |
| C7 Pricing curve over time | Pricing rarely changes; weekly use implausible. | survivor #5 (`usage anomaly`) |
| C11 BYOK-vs-OR split | Rick not BYOK; speculative persona. | survivor #1 (`usage cost-by`) |
| C12 ZDR-only routing audit | No compliance constraint in brief; speculative. | survivor #8 (`endpoints failover`) |
| C13 Record/replay fixtures | Printing-Press convention, not per-CLI feature. | n/a |
| C14 Generation diff | Forensics, not weekly; subsumed by survivor #4. | survivor #4 (`generation explain`) |
| C15 MCP server bundled | Printing-Press convention; every CLI gets it. | n/a |
