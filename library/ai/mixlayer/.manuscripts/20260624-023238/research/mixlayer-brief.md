# Mixlayer Research Brief

Run ID: `20260624-023238`
Researched at: `2026-06-24T02:32:38Z`

Mixlayer exposes an OpenAI-compatible API at `https://models.mixlayer.ai/v1` with bearer-token auth through `MIXLAYER_API_KEY`. The core generated surface is intentionally small: chat completions and model catalog commands.

The CLI novelty is built around Mixlayer-specific operating patterns rather than a generic wrapper:

- Privacy firewall commands redact locally, preserve reversible pseudonyms in a local vault, and write audit receipts for outbound payloads.
- Model ladder commands compare multiple authorized model IDs, including reasoning traces where available.
- Reasoning ledger commands save prompts, answers, reasoning, token usage, latency, model IDs, and cost estimates into local SQLite.
- Cost proof commands compare local run evidence against static frontier baselines.

Model catalog notes:

- Documentation identified Qwen rungs with `131072` token context windows and tool/reasoning support.
- Console review showed additional model IDs including `qwen/qwen3.6-27b`, `qwen/qwen3.6-35b-a3b`, and `moonshotai/kimi-k2.7-code`.
- `models query` therefore seeds a conservative local cache with documented and console-visible model IDs.
- `models query --refresh` calls live `/models` and merges the authenticated account catalog into the cache.

Implementation note: static pricing remains limited to the values available during research. Unknown model prices are stored as zero until a live catalog response or future pricing source supplies explicit values.
