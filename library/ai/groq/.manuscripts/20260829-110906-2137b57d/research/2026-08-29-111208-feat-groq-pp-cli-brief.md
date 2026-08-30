# Groq Cloud CLI Brief

## API Identity
- **Domain:** GroqCloud — ultra-low-latency LLM inference API (OpenAI-compatible REST). Providers: text chat completions, Responses API (agentic), speech-to-text (whisper), text-to-speech (Orpheus etc.), vision (image analysis via chat), embeddings, reranking, batch processing, file storage, and closed-beta fine-tuning.
- **Users:**
  - A developer iterating on prompts in the terminal, pasting curl/python snippets between docs and console, who wants `/models`, `/docs/quickstart`, Playground, and the API reference open simultaneously.
  - An agent/tool author who wires LLM calls into scripts and CI — needs non-interactive, structured output, token/cost accounting, and streaming.
  - An audio pipeline engineer transcribing/translating/synthesizing audio with whisper and Orpheus models, juggling files and rate limits.
  - A batch/fine-tuning power user submitting large jsonl workloads and polling job status from the shell.
- **Data profile:** models (small reference catalog), chat completions (transient, streaming), files (uploaded artifacts), batches (async jobs), fine-tunings (job metadata). All keyed by id; models syncable to a local catalog.

## Reachability Risk
- **None** [evidence: official public API at `https://api.groq.com`; OpenAI-compatible; docs + SDKs in production at scale; a real API key was provided by the user for live smoke testing].

## Top Workflows
1. **Chat**: send a prompt, get a streamed completion with token usage, latency, and tokens/sec (the "fast inference" core ritual).
2. **Audio pipelines**: transcribe an mp3 → text, translate audio → English, or synthesize speech from text (whisper-large-v3, Orpheus voices).
3. **Model discovery**: `GET /models` to list/compare available models and pick the right one per task (speed, context, price).
4. **RAG/embeddings**: embed text, then rerank candidate documents by relevance.
5. **Batch jobs**: upload a `.jsonl` request file → create batch → poll status → download results file.
6. **Files & fine-tuning (beta)**: manage uploaded files; create/list/get/delete fine-tunings.

## Table Stakes
- Chat completions: one-shot + SSE streaming, `stream_options`, tool/function calling (`tools`, `tool_choice`), structured outputs (`response_format`), reasoning (`reasoning_effort`), multi-modal vision inputs.
- Responses API (`POST /responses`) with MCP/remote tools, built-in tools (web search, code execution, visit website), stateful conversation via `previous_response_id`.
- Audio: transcription, translation, speech synthesis (multipart + JSON; wav response).
- Models: list + retrieve + delete.
- Embeddings and reranking endpoints.
- Files: upload (multipart), list, retrieve, delete, download content.
- Batches: create, list, retrieve, cancel.
- Fine-tuning: list, create, get, delete (closed beta).
- Auth: `Authorization: Bearer gsk_...`, canonical env var `GROQ_API_KEY`; project-scoped keys optionally send `X-Project-Id`.

## Data Layer
- Primary entities: `models` (syncable reference catalog), `files` (account artifacts), `batches` (job state), `fine_tunings` (job metadata).
- Sync cursor: none — list-based snapshots (models small & stable; files/batches/fine_tunings replaced on `sync --full`).
- FTS/search: models by id/owned_by; local completion history table for agent memory.

## User Vision
- User asked to build a CLI for Groq Cloud using the official API reference, with the provided API key for live smoke testing. Emphasis on a complete, production-grade CLI.

## Product Thesis
- **Name:** `groq-pp-cli`
- **Why it should exist:** The official SDKs and MCP server are library-shaped; the competing CLIs are single-purpose chat toys. Groq's API deserves a terminal-first CLI that covers every endpoint family — chat, responses, audio, vision, embeddings, rerank, batches, files, fine-tuning — and adds what no existing tool has: an offline synced model catalog, local completion history with full-text search, per-run token/cost accounting, rate-limit budget awareness from response headers, and multi-model comparison. Agent-native (`--json`, `--agent`, `--select`, typed exit codes) throughout.

## Build Priorities
1. `chat` — streamed chat completions with tools, reasoning, structured output, usage + latency stats, local history.
2. `audio` — `transcribe`, `translate`, `speech`.
3. `models` — `list`, `get`, `delete` + sync-to-local catalog + `search`.
4. `embeddings`, `rerank` — RAG primitives.
5. `batches` — `create` (from file/stdin), `list`, `get`, `cancel`, `results` download.
6. `files` — `upload`, `list`, `get`, `delete`, `download`.
7. `fine-tuning` — `list`, `create`, `get`, `delete` (beta-gated messaging).
8. Transcendence: cost/usage ledger, rate-limit budget, model compare, history search, multi-model prompt runner.

## Reachability Gate
- Decision: PASS (verified live in Phase 1.9 with the user-provided key against `GET /openai/v1/models`).
