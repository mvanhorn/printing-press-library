# Higgsfield CLI Brief

## API Identity
- **Domain:** AI image + video generation (Higgsfield AI, San Francisco). 35 models across image/video/Soul/marketing-studio domains.
- **Users:** Marketers, creators, producers running AI video pipelines at scale; agencies operating brand-image factories; agent builders embedding Higgsfield via MCP.
- **Data profile:** Async job-set submissions with multi-minute completion windows. Heavy state: generations history, Soul IDs (custom characters), uploaded media, workspace credits, daily transactions. The official CLI is **stateless** beyond the auth token — every list call re-hits the API.

## Reachability Risk
- **Low for platform.higgsfield.ai** — official documented Bearer-style API (`Authorization: Key KEY_ID:KEY_SECRET`). Returns 405 on root GET because routes are POST-only; this is normal, not bot protection.
- **Medium for fnf.higgsfield.ai (web backend)** — Cloudflare + Datadome protection, requires Chrome TLS impersonation (`curl_cffi`). The Hikhakk MCP source confirms working pattern. JWT auto-refresh (~1 min `__session`, ~7 day `__client`).
- **Cardinal warning from Hikhakk README:** "The cloud.higgsfield.ai surface is NOT a public API." Schema drift, auth rotation, and TOS conflict are real risks. The Printing Press CLI MUST gate web-backend commands behind an explicit `--enable-web-backend` flag or `HIGGSFIELD_ENABLE_WEB_BACKEND=1` env var so users opt in knowingly.

## Top Workflows
1. **Submit and wait** — `generate create <model> --prompt "..." --wait` for one-shot human use.
2. **Batch fanout across models** — Same prompt → Veo 3, Veo 3.1, Seedance 2, Cinematic Studio 3.0 → compare outputs. Currently requires shell scripting.
3. **Soul ID library workflow** — Create a Soul from 5 images → list → reuse `--soul-id` in `text2image_soul_v2` generations. Power users accumulate dozens of Soul IDs and forget what they're for.
4. **Marketing Studio brand pipeline** — Upload product images → Marketing Studio Image → Marketing Studio Video → cap with brand kit.
5. **Credit forecasting before submission** — `generate cost <model> --prompt "..."` to budget before burning credits. Often skipped.

## Table Stakes (must match the incumbent CLI)
- Every `higgsfield` v0.1.40 subcommand works: `auth login/logout/token`, `account status/transactions`, `workspace list/set/status/unset`, `model list/inspect`, `generate create/cost/get/list/wait`, `upload`, `soul-id create/get/list/wait`, `marketing-studio`, `product-photoshoot`, `marketplace-cards`.
- `--wait`, `--wait-timeout`, `--wait-interval` flags on generation commands.
- `--json` everywhere.
- Browser device-login flow OR explicit token / API-key auth.

## Data Layer
- **Primary entities to persist locally (SQLite):**
  - `generations` — every submitted job, model used, prompt, params hash, status, request_id, cost, completion timestamp, result URLs.
  - `models` — full registry mirror (model_id, display_name, type, params schema, backend assignment).
  - `soul_ids` — custom characters: name, image inputs, training status, last used.
  - `uploads` — local file → media_id mapping, content hash, source path, expiry.
  - `transactions` — credit ledger mirror.
  - `workspaces` — cached workspace list with active flag.
- **Sync cursor:** `updated_at` for generations and transactions; full snapshot for models/soul_ids/workspaces.
- **FTS/search:** FTS5 over `prompt`, `model`, `soul_id name`, `transaction memo`. Enables `higgsfield-pp-cli search "Riggs cinematic"` to find old generations across all models.

## Codebase Intelligence
- **Source:** Hikhakk/higgsfield-mcp-unified backends/{official,web}.py; higgsfield-ai/higgsfield-js v2 client; official Go CLI MODELS.md + command help text; local CLI probe.
- **Auth:** Platform = `Authorization: Key KEY_ID:KEY_SECRET`, env vars `HF_CREDENTIALS` (KEY:SECRET combined) or `HF_API_KEY`+`HF_API_SECRET` or `HIGGSFIELD_API_KEY`+`HIGGSFIELD_SECRET`. Web = raw `Authorization: {JWT}` (no `Bearer` prefix), env vars `HIGGSFIELD_JWT`+`HIGGSFIELD_CLERK_CLIENT`+`HIGGSFIELD_ENABLE_WEB_BACKEND=1`. Web JWT `azp` must equal `higgsfield.ai`.
- **Data model:** Job-sets contain jobs; jobs have status enum (`queued`/`in_progress`/`completed`/`failed`/`nsfw`/`canceled` lowercase per JS v2; Python SDK uses TitleCase). Soul IDs are entities with their own training lifecycle. Workspace is a billing context that scopes account access.
- **Rate limiting:** Cloudflare/Datadome on web backend; no documented platform limits. `NotEnoughCreditsError` class exists. Result caching 7 days (per geopopos MCP).
- **Architecture:** Platform API submits jobs via per-model endpoints (e.g. `POST /v1/image2video/dop`, `POST /v1/text2image/soul`, `POST /bytedance/seedream/v4/text-to-image`); job IDs poll via `GET /requests/{id}/status`. Web backend uses different submit paths (per-model) and unified `GET /jobs?size=100` polling.

## Source Priority
- **Primary:** higgsfield-web (`fnf.higgsfield.ai` + `cloud.higgsfield.ai`) — JWT auth via user's local CLI credentials. Covers the headline 2026 models (Veo 3.1, Sora 2 if/when added, Kling 3.0, Seedance 2, Soul V2, Nano Banana 2). **User does not have a platform API key**; web backend is the only working surface today.
- **Secondary:** higgsfield-platform (`platform.higgsfield.ai`) — Bearer-style auth. Will be wired in the spec but unverified at smoke-test time. Includes Soul standard, DOP, Seedance v1 Pro, Kling 2.1 Pro, Reve text-to-image, Seedream v4.
- **Economics:** Both surfaces share the user's Higgsfield credits (consumed via JWT or API key). No separate paid tier between them.
- **Inversion risk:** Spec completeness favors the platform API (well-documented in JS SDK, fewer reverse-engineering risks). Do NOT let that invert the user's stated priority — the user explicitly chose "both, unified" with the web backend as their working auth path. Platform commands stay in the spec but the README/SKILL must lead with web-backend workflows.

## User Vision (inferred from Kai's projects/current-priorities.md)
- Higgsfield is used for Scam Patrol (Soul-trained Riggs cut-ins via DOP cinematic) and SupraOS social videos (Seedance/Veo b-roll). The Printing Press CLI should let an agent reproduce these pipelines without shelling out 20 times. Specifically: batch fanout, Soul library search ("find the Riggs Soul ID I trained last month"), credit guard before expensive Veo 3.1 fanouts.

## Product Thesis
- **Name:** `higgsfield-pp-cli` (binary), `higgsfield` in library.
- **Why it should exist:** The official `higgsfield` CLI is human-shaped — one command at a time, no memory, no cross-job analysis. The official MCP is hosted (cloud-only) and doesn't expose batch/analytics tools. **Nothing in the ecosystem lets an agent see "every Soul I've trained" or "every Veo 3 generation I made this week with associated cost" without a separate database.** That's the gap. Add: local SQLite store of all generations/Souls/uploads/transactions, FTS over prompts, batch fanout across models, credit guards, agent-native JSON-first surface with MCP exposure of every command via runtime Cobra-tree mirror.

## Build Priorities
1. **Both-backend HTTP client** — single `Client` that routes to platform.higgsfield.ai or fnf.higgsfield.ai based on model's backend assignment. Web-backend behind `HIGGSFIELD_ENABLE_WEB_BACKEND=1` gate.
2. **Data layer (SQLite)** — generations, models, soul_ids, uploads, transactions, workspaces. FTS5 indexes.
3. **Absorbed surface from official CLI** — auth, account, workspace, model, generate, upload, soul-id, marketing-studio, product-photoshoot, marketplace-cards. Each command also persists results to the store on success.
4. **Sync command** — `sync --full` pulls generations history (paged), Soul list, model registry, workspace list, transactions.
5. **Transcendence features** (chosen via Step 1.5c.5 subagent) — see Phase 1.5 manifest. Anchors: `fanout` (batch across models), `soul library search`, `credit guard`, `compare runs`, `prompt history search`, `cost forecast batch`.
6. **MCP surface** — Cobra-tree mirror exposes all 60+ commands; mark read-only ones (`account status`, `model list`, store-query commands, search, sql) with `mcp:read-only`; mutating ones (`generate create`, `soul-id create`, `upload`) get default open-world annotations.

## Out of scope (deferred)
- Webhook receiver subcommand. Platform API supports webhooks on submit, but receiving them requires the user to expose an endpoint; out of scope for a CLI.
- Browser-based device login flow (the official CLI handles this; we read `HIGGSFIELD_JWT` / API key env vars). Adding `auth login --chrome` would require shelling out to the official CLI; document this as the recommended flow in README.

## Known unknowns
- Exact platform API paths for Soul ID CRUD, account/credits, workspace, marketing-studio, product-photoshoot. The agent research couldn't reach `internal/api/` source in the higgsfield-ai/cli repo via WebFetch. Two paths: (1) shell out to local `higgsfield <cmd> --json` and parse output (no spec needed); (2) HAR-capture the local CLI mid-Phase-1.7 (user declined). For v1, prefer option 1 for these specific commands: implement them as wrappers around the local `higgsfield` binary with `// pp:client-call` annotation, falling through to direct API when an API key is available. This is allowed by the anti-reimplementation carve-out and keeps us shippable today.
