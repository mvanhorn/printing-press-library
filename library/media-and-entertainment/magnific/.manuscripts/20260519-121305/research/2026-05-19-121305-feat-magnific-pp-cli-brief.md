# Magnific CLI Brief

## API Identity
- Domain: AI creative platform — image generation, video generation, image editing (upscaling, relighting, style transfer, background removal, expand/inpaint), audio (music, SFX, isolation), stock content (icons, videos, resources), and utility AI (classifier, image-to-prompt, change-camera, LoRA training).
- Users: developers, agencies, automation pipelines building visual content workflows. The product was rebranded from Freepik API → Magnific in early 2026; both `api.freepik.com` and `api.magnific.com` accept the same key.
- Data profile: heavy on async task lifecycles (POST `/v1/ai/<model>` returns task_id → poll GET `/v1/ai/<model>/{task-id}` or use webhook_url). Generation outputs are images/videos/audio at signed URLs. Stock content has rich metadata (orientation, content type, license tier).

## Reachability Risk
- **None.** `https://api.magnific.com/v1/icons` returns a clean 401 with structured JSON and a key-acquisition URL. The OpenAPI spec is public-hosted on GCS. No bot-protection on the API. (The marketing site `www.magnific.com` does sit behind a WAF, but that's a separate origin and irrelevant.)

## Top Workflows
1. **Generate an image** — POST `/v1/ai/mystic` (or one of 22 text-to-image models), poll, download.
2. **Upscale an image** — POST `/v1/ai/image-upscaler` (creative) or `/v1/ai/image-upscaler-precision` (faithful), get higher-res result. Magnific's flagship feature, the reason the company exists.
3. **Generate a video** — POST `/v1/ai/image-to-video/<model>` (48 i2v + 11 t2v variants), poll, download MP4.
4. **Run lightweight image edits** — relight, background-remove, style-transfer, expand, inpaint (5 distinct endpoints).
5. **Search and download stock content** — icons, videos, resources from Freepik's curated catalog of 250M+ assets.

## Table Stakes
- Text-to-image with model selection (≥10 models in any usable CLI: Mystic, Flux 2 Pro/Turbo/Klein, Flux Pro 1.1, Flux Dev, Hyperflux, Seedream 4/4.5, Z-Image Turbo, RunWay).
- Text-to-video and image-to-video (Kling family is the headline; Hailuo, WAN, RunWay, Seedance, LTX, PixVerse, OmniHuman, Veo 3.1 all matter).
- Upscaler (creative + precision) — this is the namesake feature.
- Background remove + image expand + relight + style transfer + inpaint.
- Audio: music generation (ElevenLabs), sound effects, audio isolation (SAM).
- Stock content search/download for icons, videos, resources.
- Task management: list tasks, poll status, cancel/abandon if supported.
- Account: credits balance, usage history.
- Auth via `x-freepik-api-key` header; same key for both hostnames. Env var `FREEPIK_API_KEY` (or `MAGNIFIC_API_KEY` per the rebrand).

## Data Layer
- **Primary entities:** `tasks` (every async generation/edit/upscale request — unified across all model families), `assets` (downloaded files with local paths and source task IDs), `models` (model registry with capabilities, credit cost, family), `prompts` (saved prompt templates), `icons`/`videos`/`resources` (stock content discovery cache).
- **Sync cursor:** task list per-model endpoint (each model exposes its own GET list). For full sync, walk the 22+ task-list endpoints with pagination cursors.
- **FTS/search:** prompts table FTS5 (search your own generation history by prompt text); local asset metadata FTS5 (find downloaded files by tags/prompt). Stock content searched via API (not pre-synced — too large).

## Codebase Intelligence
- Source: GitHub `freepik-company/freepik-mcp` (official Python MCP server, 67 stars).
- Auth: `x-freepik-api-key` header. Reads `FREEPIK_API_KEY` env var. `httpx.AsyncClient(base_url, headers={"x-freepik-api-key": config.api_key}, timeout=config.timeout)`.
- Data model: domain-driven, separated into application/domain/infrastructure layers. Dynamically loads the OpenAPI spec from GCS at startup, applies a route-whitelist to expose only a curated MCP subset.
- Rate limiting: not addressed in the MCP code; per docs, **10 req/s avg, 50 req/s peak** (5s burst window); RPD ceilings 100–10,000 by service & tier.
- Architecture: thin facade that proxies whitelisted spec routes plus a hand-written Mystic service. Spec-loader caches the OpenAPI YAML to a tempfile.
- Spec URL discovered: `https://storage.googleapis.com/fc-freepik-pro-rev1-eu-api-specs/freepik-api-v1-openapi.yaml` (2 MB, 278 paths, 388 operations).

## Product Thesis
- Name: **magnific-pp-cli** (binary), library slug **magnific**.
- Why it should exist: Magnific is the AI creative platform of 2026 — 80+ generation models in one place — but the only existing CLI (`Balneario-de-Cofrentes/freepik-cli`, TypeScript, 3 stars) requires Node, can't search your own history, and re-builds the whole task lifecycle as ad-hoc TypeScript per command. A single-binary Go CLI with a local SQLite ledger of every task, prompt, and downloaded asset turns Magnific from "API I poll" into "creative archive I query." Plus the agent-native MCP code-orchestration pattern shrinks the tool catalog from 388 raw endpoints to a search+execute pair an agent can actually reason about.

## Build Priorities
1. **Foundation** — typed client (`x-freepik-api-key`), config (FREEPIK_API_KEY + MAGNIFIC_API_KEY env vars), local SQLite store with `tasks`, `assets`, `prompts`, `models` tables + FTS, and a unified `sync` that walks every model's task-list endpoint.
2. **Absorbed surface** — generate (text-to-image, all major models), video (i2v + t2v), upscale (creative + precision), edit (relight, style-transfer, remove-bg, expand × 3 variants, inpaint, change-camera), audio (music, sfx, isolation), stock (icons, videos, resources search/get/download), utility (classify, image-to-prompt, improve-prompt, LoRAs).
3. **Transcendence** — see Phase 1.5 manifest. Unified `task wait` / `task watch`, local `history` FTS + cost ledger, `compare` (run prompt through N models, show output + cost), `pipeline` (chain operations), `gallery` (offline asset browser), `cost forecast`, `webhook serve` (local webhook receiver to short-circuit polling), `models` registry with per-model cost/latency stats from your history.
