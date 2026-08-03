# OpenRouter Image CLI Brief

## API Identity
- Domain: AI image generation (text-to-image, image-to-image, SVG/vector via Recraft)
- Users: AI agents (cron pipelines, coding agents) and humans who need on-demand image generation with model choice
- Data profile: Model catalog (40+ image models today, dynamic), generation requests/responses (base64 images), per-endpoint pricing/capability records, usage/cost metadata per generation

## Reachability Risk
- None. Official OpenAPI spec at `https://openrouter.ai/openapi.json` (3.1.0, 73 paths, 1.7MB). Live probe of `GET /models?output_modalities=image` returned HTTP 200 with 40 image models, no auth needed for discovery. Auth required only for generation (`POST /images`).
- Auth: `Authorization: Bearer <key>` — canonical env var `OPENROUTER_API_KEY` (confirmed by official Go SDK examples). No key provided for this run; Phase 5 live testing skipped.

## Top Workflows
1. **Generate an image from a prompt** — `POST /api/v1/images` with `model` + `prompt`, optionally `n`, `resolution`/`size`, `aspect_ratio`, `quality`, `output_format`, `seed`. The CLI's headline command: pick a model, describe the image, get a file (or base64) back. Write to disk by default for humans; `--json`/`--agent` for agents.
2. **Pick the right model** — `GET /images/models` returns every image model with its `supported_parameters` (resolution tiers, seed support, streaming, input modalities). An agent should be able to ask "which models do image-to-image and 4K, cheapest first" and get a filtered, ranked answer offline.
3. **Inspect per-endpoint capabilities & pricing** — `GET /images/models/{author}/{slug}/endpoints` returns providers, allowed passthrough params, `supports_streaming`, and billable pricing lines. Compare provider prices before generating.
4. **Image-to-image editing** — pass `input_references` (URL or base64 data URL) to transform an existing image.
5. **Cost-aware generation** — each response carries `usage` with `cost` (USD). Track spend locally per model/prompt; agents want to know cost before/after generating.
6. **Streaming generation** — `stream: true` returns SSE events (`image_generation.partial_image`, `image_generation.completed`). Support for models that expose it.

## Table Stakes
- Generate from prompt with arbitrary model slug (`--model`)
- List image models (official endpoint)
- Per-endpoint pricing/capability lookup
- Save output to file; `--json` output with base64 or saved path
- `--dry-run` (show request without spending credits — image gen is all-or-nothing billed)
- Auth via `OPENROUTER_API_KEY` env var

## Data Layer
- Primary entities: image_models (id, name, architecture.modalities, supported_parameters, supports_streaming, endpoints URL), endpoints (provider, pricing lines, passthrough params), generations (id, model, prompt, cost, tokens, timestamps)
- Sync cursor: full-catalog sync for models/endpoints; generation history appended locally on each run
- FTS/search: model name/author/provider search, prompt history search

## Competitor Landscape
- `@howells/motif-cli` (npm) — agent-first fal.ai image CLI: structured output, dry runs, history, series. Proven pattern: agent-native image CLIs work.
- `imagnx` (npm) — multi-model image generation CLI (generic providers).
- `openrouter-image-mcp` (hamzatrq, npm) — MCP server for OpenRouter image gen; parallel generation, reference images, `OPENROUTER_BASE_URL` override. Proves `/images` + discovery endpoints work; lacks local store, offline model catalog, cost analytics.
- `jtxmp/openrouter-image-mcp` — MCP server, small (★3).
- `imagen-openrouter` (yusufipk) — client-side web tool with model list, gallery, references. No CLI, no agent output.
- Existing public-library `openrouter` CLI — cost/usage attribution for chat API; explicitly NOT an image generator ("skip it for chat"). No overlap with image generation surface.

## User Vision
- Primary goal: an AI agent (or human) generates images via the CLI on demand.
- Model used for generation MUST be user/agent-specifiable. Model selection is a first-class concern — not a hidden default.
- Defer feature decisions to agent recommendations.

## Product Thesis
- Name: `openrouter-image-pp-cli`
- Why it should exist: OpenRouter is the only image API with one-key access to 40+ models from every major lab (OpenAI, Google, BFL, ByteDance, Krea, Recraft, Microsoft). Existing tools are web UIs, MCP servers, or fal-specific CLIs. Nothing gives an agent: model discovery → capability/pricing compare → generate → local cost history, all offline-searchable with typed output. The existing `openrouter` library CLI covers chat cost attribution only.

## Build Priorities
1. `generate` — the headline: `--model`, `--prompt`, `--n`, `--resolution`, `--aspect-ratio`, `--size`, `--quality`, `--output-format`, `--seed`, `--output <file>`, `--dry-run`, `--json`, `--stream`
2. `models list` / `models search` — offline catalog with capability filters (image-to-image, 4K, streaming, cheapest)
3. `models endpoints <model>` — provider pricing/capability comparison
4. `history` / `usage` — local generation + cost ledger; `history show <id>` fetches remote metadata
5. `doctor` — key + reachability check
6. `sync` — cache the model catalog and endpoints locally

## Reachability Gate
- Decision: PASS
- Evidence: `GET https://openrouter.ai/api/v1/models?output_modalities=image` → HTTP 200, 40 models, no auth
