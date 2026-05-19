# Magnific CLI Absorb Manifest

## Ecosystem snapshot

| Tool | Tier | Notes |
|------|------|-------|
| `Balneario-de-Cofrentes/freepik-cli` | TypeScript, MIT, 3 stars, 23 commands | The only direct CLI competitor. Single-turn async collapsing, smart-model flag, batch concurrency, history with cost. Node required. |
| `freepik-company/freepik-mcp` | Python, MIT, 67 stars (official) | Stdio MCP server; loads spec from GCS; route-whitelist for tractable agent surface; covers icons, resources, classifier, Mystic. |
| `mcerqua/freepik-mcp` | TypeScript, 2 stars | Smaller MCP (search_resources, get/download_resource, generate_image, check_status). |
| `grafikogr/freepik-mcp-server` | Flux-Dev only MCP | Niche, model-specific. |

## Absorbed (match or beat everything that exists)

Spec-derived feature rows. The Printing Press emits these as typed commands from the official Freepik OpenAPI spec (`https://storage.googleapis.com/fc-freepik-pro-rev1-eu-api-specs/freepik-api-v1-openapi.yaml`, 278 paths / 388 ops). The "Best Source" column credits the strongest existing competitor.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Text-to-image (Mystic flagship) | Magnific spec + Balneario `generate --model mystic` | `mystic generate` (spec-emitted) | Single binary, no Node, structured JSON, --dry-run, --select |
| 2 | Text-to-image (Flux family: 2 Pro/Turbo/Klein, Pro 1.1, Dev, Kontext Pro, Hyperflux) | Magnific spec + Balneario `generate --model flux-*` | per-model spec-emitted commands | Agent-native, offline history |
| 3 | Text-to-image (Seedream 4 / 4.5 / 4.5 Edit) | Magnific spec | spec-emitted | -- |
| 4 | Text-to-image (Z-Image Turbo, RunWay, Imagen 3/4 Fast/Ultra, Gemini 2.5 Flash) | Magnific spec | spec-emitted | -- |
| 5 | Image-to-video (Kling family: 2.1/2.5/2.6/Motion Control, O1, Elements, 4K, advanced custom) | Magnific spec + Balneario `video` | spec-emitted | -- |
| 6 | Image-to-video (MiniMax Hailuo 02 1080p, 2.3, Video-01-Live) | Magnific spec | spec-emitted | -- |
| 7 | Image-to-video (WAN 2.5/2.6, Seedance Pro, PixVerse V5, RunWay Gen4 Turbo, Veo 3.1/Fast, Happy Horse, LTX) | Magnific spec | spec-emitted | -- |
| 8 | Text-to-video (WAN 2.5 T2V, LTX 2.0 Pro, RunWay T2V) | Magnific spec | spec-emitted | -- |
| 9 | Reference-to-video (multi-image input) | Magnific spec | spec-emitted | -- |
| 10 | Image upscaler creative (Magnific's namesake feature) | Magnific spec + Balneario `upscale` | spec-emitted | History/cost tracking |
| 11 | Image upscaler precision (faithful) + v2 | Magnific spec | spec-emitted | -- |
| 12 | Image relight | Magnific spec + Balneario `relight` | spec-emitted | -- |
| 13 | Image style transfer | Magnific spec + Balneario `style-transfer` | spec-emitted | -- |
| 14 | Remove background (beta) | Magnific spec + Balneario `remove-bg` | spec-emitted | -- |
| 15 | Image expand (Flux Pro / Ideogram / Seedream v4.5 variants) | Magnific spec + Balneario `expand` | spec-emitted, per-variant | -- |
| 16 | Inpainting (Ideogram image edit, Seedream 4.5 edit) | Magnific spec | spec-emitted | -- |
| 17 | Change camera (perspective transform) | Magnific spec | spec-emitted | -- |
| 18 | Image-to-prompt (reverse caption) | Magnific spec + Balneario `describe` | spec-emitted | -- |
| 19 | AI image classifier (real vs AI detection) | Magnific spec + Balneario `classify` | spec-emitted | -- |
| 20 | Improve prompt | Magnific spec | spec-emitted | -- |
| 21 | Music generation (ElevenLabs) | Magnific spec + Balneario `music` | spec-emitted | -- |
| 22 | Sound effects | Magnific spec + Balneario `audio` | spec-emitted | -- |
| 23 | Audio isolation (SAM) | Magnific spec | spec-emitted | -- |
| 24 | Lip sync | Magnific spec | spec-emitted | -- |
| 25 | Skin enhancer | Magnific spec | spec-emitted | -- |
| 26 | Video upscaler / precision | Magnific spec | spec-emitted | -- |
| 27 | VFX video effects | Magnific spec | spec-emitted | -- |
| 28 | OmniHuman 1.5 (audio-driven human animation) | Magnific spec | spec-emitted | -- |
| 29 | Video edit | Magnific spec | spec-emitted | -- |
| 30 | Voiceover (ElevenLabs Turbo v2.5) | Magnific spec | spec-emitted | -- |
| 31 | Icons API (search / get / download) | Magnific spec + Balneario `search` | spec-emitted | Local download cache + FTS |
| 32 | Videos API (stock video search/get/download) | Magnific spec | spec-emitted | -- |
| 33 | Resources API (images/templates search/get/download) | Magnific spec | spec-emitted | -- |
| 34 | Text-to-icon (incl. preview + render) | Magnific spec + Balneario `icon` | spec-emitted | -- |
| 35 | LoRA training / list / inference | Magnific spec + Balneario `lora` | spec-emitted | -- |
| 36 | Apps (workflow runs) | Magnific spec | spec-emitted | -- |
| 37 | Account / credits / me | Magnific spec + Balneario `credits` | spec-emitted | -- |
| 38 | Task list / get per-resource (~80 endpoint families × GET list + GET status) | Magnific spec | spec-emitted | -- |
| 39 | Config / auth setup | Balneario `config` (set-key/get-key) | generator-emitted `auth` subcommands | Standard PP shape |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona |
|---|---------|---------|-------|--------------|--------------|----------|---------|
| T1 | Prompt history FTS | `history search "<q>" [--model] [--since]` | 8/10 | hand-code | FTS5 over local `prompts` table populated on every generate/upscale/video dispatch; joins `tasks` for cost + output URL | Brief data-layer; Mara's "I lose winning prompts" frustration; no API returns prompt-text history | Mara, Dev |
| T2 | Model bake-off | `compare "<prompt>" --models mystic,flux-2-pro,seedream-v4-5,z-image-turbo` | 9/10 | hand-code | Fans real POST calls to each model's `/v1/ai/<model>` endpoint, polls each task_id concurrently, downloads outputs to a dated folder, writes a `manifest.json` with cost + latency + URL per model | Magnific's defining trait is 22+ image models; `freepik-cli` has no compare surface; Dev's "does Flux 2 beat Mystic?" weekly question | Dev, Mara |
| T3 | Unified task wait/watch | `task wait <task-id>` / `task watch <task-id>` | 8/10 | hand-code | Resolves task_id → model endpoint via local `tasks` row, polls real GET `/v1/ai/<model>/{task-id}` with adaptive backoff until terminal state; `watch` emits JSON-line status | Async lifecycle is uniform across 80+ models but exposed as 80+ different paths; `freepik-mcp` hand-rolls polling per route | Dev, Sasha |
| T4 | Cost ledger + forecast | `cost ledger --group-by model --since 30d` / `cost forecast --model <m> --count <n>` | 7/10 | hand-code | Ledger: SQL aggregation over local `tasks.credit_cost` grouped by model/tag/day. Forecast: curated per-model credit cost × intended run count, compared to live `/v1/me/credits` balance | Mara's "which client ate my credits"; pay-per-credit is Magnific's economic identity; no API for the join | Mara, Dev |
| T5 | Local asset gallery | `gallery list [--tag] [--since]` / `gallery open <id>` | 6/10 | hand-code | Filters local `assets` table by tag/orientation/date/model; `open` honors `cliutil.IsVerifyEnv` + `--launch` flag per side-effect rule | Mara files MP4s by hand; Tomas re-downloads same icons monthly; brief calls out asset FTS as a data-layer goal | Mara, Tomas |
| T6 | Models registry with empirical stats | `models list --capability <cap> --sort cost` / `models stats <model>` | 8/10 | hand-code | Joins curated `models` static reference (capabilities, listed cost, family) with local `tasks` aggregation (your p50 latency, success rate, $ spent on this model) | Route-whitelist pattern in `freepik-mcp` exists because agents drown in 388 raw tools; Sasha's "wrong model picked" weekly frustration | Sasha, Dev |
| T7 | Stale-task reconcile | `tasks stale --since 24h` / `tasks reconcile` | 7/10 | hand-code | Local SQL finds tasks in non-terminal state past threshold; reconcile re-polls each via real GET and updates row | Polling crashes leak orphan tasks; Dev's "is this task actually still pending?" | Dev, Sasha |
| T8 | Prompt templates / replay | `prompt save <name> --text "..." --model mystic` / `prompt run <name> [--override k=v]` | 7/10 | hand-code | `save` writes to local `prompts` table with `{{var}}` placeholder syntax; `run` substitutes vars and dispatches the real model endpoint via the typed client | Mara replays prompts from memory; brief Data Layer names `prompts` table; no API for template management | Mara, Dev |
| T9 | Stock library local index | `stock library index` / `stock library search "<q>"` | 6/10 | hand-code | `index` walks downloaded icons/videos/resources into local FTS5 table; `search` queries local FTS before any API call | Tomas re-downloads same icons monthly; 250M+ stock catalog is too large to pre-sync, but his ~1000-asset working set is FTS-able locally | Tomas |
| T10 | Agent context bundle | `context` | 6/10 | hand-code (mcp:read-only) | Single command printing JSON: top 10 models used, current credit balance, last 10 prompts, recent output dirs — one local-store fetch + one `/v1/me/credits` call | `freepik-mcp` whitelists routes precisely to avoid 388-tool overload; Sasha's agent picks wrong models | Sasha |

## Stub & gap notes

- No stubs planned. Every transcendence row above is a fully-buildable command using the existing spec endpoints + local SQLite.
- The CLI does NOT ship `webhook serve` (cut for scope) — users wanting webhook receipt use the API's `webhook_url` parameter directly and run their own receiver. Documented in README.
- The CLI does NOT ship `pipeline run` (cut for scope) — multi-step orchestration is left to shell + `task wait`.
