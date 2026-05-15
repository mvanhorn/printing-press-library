# Cartesia CLI Brief

## API Identity
- Domain: voice synthesis + voice agents (TTS, STT, voice cloning, conversational telephony agents)
- Users: voice-AI developers building production phone agents (support, sales, scheduling, IVR); product teams iterating on prompts/voices; eval engineers grading LLM-as-a-judge on real calls.
- Data profile: agents (template/config), deployments (versioned instances), calls (with transcripts + audio + telephony metadata), metrics (LLM-judge results), voices, datasets, fine-tunes, pronunciation dictionaries, phone numbers, usage/credits.
- Base URL: `https://api.cartesia.ai`. Auth: `Authorization: Bearer <api-key>` (or short-lived JWT access token). Versioning header: `Cartesia-Version: <date>` (currently 2025-04-16/2026-03-01).

## Reachability Risk
- None. `GET /` → 200; `GET /voices` → 401 (expected without key). Official Stainless-generated OpenAPI (50 endpoints, 35 paths) is the canonical spec; pulled from `cartesia-ai/cartesia-python/.stats.yml`.

## Top Workflows
1. **Iterate on a production voice agent** — pull agent config, tweak prompt/voice, push update, watch new deployments come up, listen to fresh calls.
2. **Debug a bad call** — list recent calls for an agent, fetch transcript + audio + runtime logs, grep for the moment things went sideways.
3. **Evaluate quality at scale** — define LLM-judge metric, attach to agent, sync nightly, export CSV, slice by deployment/time-window/intent.
4. **Build voice library for an agent** — clone a voice from a 5-second clip, localize across languages, fine-tune on a dataset, attach the new voice to an agent.
5. **TTS workbench** — script-to-audio one-shots, A/B different voices on the same script, infill audio gaps, voice-change existing recordings.

## Table Stakes
- Full agent CRUD-where-supported: list/get/update/delete (creation lives in playground+git, but every other operation is API-reachable).
- Calls: list, paginate, fetch transcript, download audio (WAV), tail runtime logs.
- Deployments: list per agent, fetch one.
- Metrics: create LLM-judge, list results, export CSV, attach/detach from agent.
- Voices: list/get/clone/localize/update/delete.
- TTS: `bytes`, `sse`, websocket (streaming, multiplexed contexts).
- STT: batch `/stt` + realtime websocket.
- Voice-changer + infill (bytes + sse).
- Fine-tunes, datasets (+files), pronunciation-dicts — full CRUD.
- Templates browse (`/agents/templates`) — required to bootstrap a new agent.
- Access tokens (`POST /access-token`) for short-lived browser/edge usage.

## Data Layer
- Primary entities: `agents`, `deployments`, `calls`, `call_transcripts`, `metrics`, `metric_results`, `voices`, `fine_tunes`, `datasets`, `dataset_files`, `pronunciation_dicts`, `phone_numbers`, `templates`.
- Sync cursor: `updated_at` on agents/voices/datasets; `start_time` on calls; deployments are append-only per agent.
- FTS/search: transcripts (full-text), agent names/descriptions, voice names, template descriptions, prompts (from agent config), metric names/criteria.

## Codebase Intelligence
- Source: Stainless-generated `cartesia-ai/cartesia-python` (121 ★), `cartesia-ai/cartesia-js` (131 ★), `cartesia-ai/line` (100 ★, voice-agent SDK), `cartesia-ai/cartesia-mcp` (13 ★, official MCP).
- Auth: `Authorization: Bearer sk_car_…` API key OR short-lived JWT from `/access-token`. Two security schemes share Bearer scheme; client picks via grant type.
- Data model: agent → deployments → calls → metric_results. Agent has `git_repository` + `git_deploy_branch` — code-deploy model. Phone number 1:1 with agent.
- Rate limiting: not explicitly documented; Cartesia typically returns 429 with `Retry-After`. Stream endpoints use SSE/WebSocket and impose per-connection concurrency.
- Architecture: REST for management, SSE/WebSocket for realtime audio. CSV export of metric results capped at 100k rows.

## User Vision
- "Manage Cartesia voice agent creation" — broad enough to cover the agent lifecycle: discover templates, list/inspect/update agents, watch deployments, fetch call evidence, attach metrics, plus the voice-asset prerequisites (clone, localize, fine-tune) that feed agents.

## Product Thesis
- Name: `cartesia-pp-cli`
- Why it should exist: the official `cartesia-mcp` covers 7 TTS/voice tools; nothing surfaces agents, calls, deployments, metrics, fine-tunes, datasets, pronunciation-dicts as first-class CLI primitives. A power user iterating on a phone agent currently bounces between the playground and Python REPL. This CLI absorbs every tool that exists AND adds offline call-grep, deployment-drift detection, metric-trend windowing, and an "audit a deployment" compound command that joins agent+deployment+calls+metrics from local SQLite.

## Build Priorities
1. **Foundation (Priority 0):** generated client + SQLite store for agents, deployments, calls (with transcript), metrics, metric_results, voices, datasets, fine-tunes, pronunciation-dicts. `sync` per resource. FTS5 on call transcripts and agent prompts.
2. **Absorb (Priority 1):** every endpoint in the spec as a typed command — `agents list/get/update/delete`, `calls list/get/audio/logs`, `deployments list/get`, `metrics create/list/get/results/export/attach/detach`, `voices list/get/clone/localize/update/delete`, `tts bytes/sse/ws`, `stt transcribe/ws`, `voice-changer bytes/sse`, `infill bytes`, `fine-tunes create/list/get/delete/voices`, `datasets …`, `pronunciation-dicts …`, `templates list`, `access-token create`, `status`. Plus every tool exposed by `cartesia-mcp` (text-to-speech, list/get/clone voices, infill, voice-change, localize-voice).
3. **Transcend (Priority 2):** the commands only this CLI can do because everything is in SQLite — see absorb manifest.
4. **Polish:** flag descriptions, README cookbook, agent-native exits, narrative.

## Source Priority
- Single source (Cartesia). No combo CLI ordering needed.
