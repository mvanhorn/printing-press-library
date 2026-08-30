# OrcaRouter Gateway — research brief

## What OrcaRouter is

OrcaRouter is an OpenAI-compatible AI gateway built for both models and agents.
Like OpenRouter, it exposes a provider/model namespace across many models behind
a single endpoint — but it also combines adaptive routing, automatic failover,
zero-markup inference, observability, guardrails, and agent-tool governance on
that same endpoint. It also runs gateway-level, zero-trust security for AI
agents, screening every prompt/response and governing every tool call on a
default-deny basis, with no application code changes.

## API surface probed live (2026-08-30)

- `GET /v1/models` → HTTP 200, 204 models. Entries span `orcarouter/*` routing
  models (e.g. `orcarouter/free`, `orcarouter/fusion`,
  `orcarouter/fusion-flash`) and routed provider models (`anthropic/*`,
  `openai/*`, `deepseek/*`, `google/*`, `qwen/*`, `glm/*`).
- `GET /v1/models/orcarouter/free` → HTTP 200, single model with
  `supported_endpoint_types: ["openai", "openai-response", "anthropic", "gemini"]`.
- `POST /v1/chat/completions` → HTTP 200 with an OpenAI-compatible
  `chat.completion` payload (id `gen-...`, `choices[].message`, `usage`).
  Model `orcarouter/fusion-flash` routed to `openai/gpt-oss-120b` via `AkashML`.

## CLI design decisions

- **Named provider, not a passthrough.** The catalog repo already treats
  OpenRouter as a first-class named provider (`library/ai/openrouter`).
  `orcarouter` mirrors that shape: its own `library/ai/orcarouter/` directory,
  `orcarouter-pp-cli` binary, and `pp-orcarouter` skill. Users get the OrcaRouter
  gateway as a named provider instead of configuring an anonymous base URL.
- **Focused command set.** `doctor` (auth + connectivity probe), `models` /
  `models get` (namespace discovery), `chat` (OpenAI-compatible completions),
  `auth` / `auth set` (key state + persistence). No MCP server is advertised, so
  no `manifest.json` / MCP tools surface.
- **Env-first auth.** `ORCAROUTER_API_KEY` env var, matching the gateway's own
  SDK docs, with a `~/.config/orcarouter-pp-cli/config.toml` fallback.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` all pass.
- Live `doctor --json` returns `"reachable": true`, `"model_count": 204`.
- Live `chat orcarouter/fusion-flash "..." --max-tokens 40` returns HTTP 200.
- `models` lists 204 models sorted by id.
