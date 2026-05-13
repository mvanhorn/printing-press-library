# FlowiseAI CLI Brief

## API Identity
- Domain: Visual builder + REST runtime for LLM chatflows, AI agents, and RAG pipelines (open-source, MIT)
- Users: Developers building chat assistants and agentic workflows; agentic systems calling the runtime via REST
- Data profile: Chatflows, Assistants, Tools, Variables, Document Stores (with chunked content + vector indexes), ChatMessages, Predictions, Leads, Feedback, Upsert History
- Base URL pattern: `{host}/api/v1/{path}` — host is per-instance (self-hosted localhost:3000, self-hosted cloud, or Flowise Cloud)
- Auth: `Authorization: Bearer <API_KEY>` (instance-level API key; some flows can override with per-flow keys)

## Reachability Risk
- **None**. The runtime is self-hosted and the API is documented + spec-published. No bot mitigation, no CAPTCHA, no Cloudflare challenge. Reachability gate is structural only — we test against a host the user supplies at runtime.

## Top Workflows (agent-operated newsletter use case)
1. **Compose a multi-section newsletter** — agent loops through N "section chatflows" (market summary, listing highlights, neighborhood spotlight, mortgage rate update, agent intro), concatenates the `text` field of each response into a single markdown document.
2. **Ingest realtor source material into a document store** — upload PDFs/CSVs of MLS listings, market reports, neighborhood data → `vectorUpsert` → RAG-backed chatflows can pull the freshest data on demand.
3. **Trigger a "distribute newsletter" chatflow** — pass the assembled draft as `question` to a flow that's wired (inside Flowise) to SendGrid / Mailchimp / a CRM. Flowise handles the side effects; the CLI just kicks it off and records `chatId` for traceability.
4. **Manage chatflows + assistants** — list / get / create / update / delete; agent needs visibility into what flows exist before it can call them.
5. **Query + replay chat history** — pull recent `ChatMessage` rows by `chatflowId` so the agent can audit what was generated, recover from a failed run, or re-issue a corrected version.

## Table Stakes (from competing tools)
- Every endpoint of the 46-endpoint REST surface exposed as a typed command
- Streaming-aware `predict` (the SDK's headline feature)
- `overrideConfig` JSON passthrough for runtime config (sessionId, temperature, vars)
- File uploads on predict (`multipart/form-data`)
- Chat history passthrough (`history` array of prior turns)
- Chatflow listing with name-regex filter (`mcp-flowise` does this)
- Bearer-token auth with env-var (`FLOWISE_API_KEY`) + per-call override
- Configurable base URL (`FLOWISE_BASE_URL` / `FLOWISE_API_ENDPOINT`)
- `--json` / `--select` / `--csv` / `--dry-run` everywhere (agent-native)

## Data Layer
- Primary entities to persist locally (SQLite):
  - `chatflows` (id, name, deployed, isPublic, category, type, flowData hash, apikeyid, timestamps)
  - `assistants` (id, type, details, credential, iconSrc, timestamps)
  - `tools` (id, name, description, color, iconSrc, schema, func, timestamps)
  - `variables` (id, name, value, type, timestamps)
  - `document_stores` (id, name, description, status, loaders, vectorStoreConfig, embeddingConfig, recordManagerConfig)
  - `chat_messages` (id, role, chatflowid, content, sourceDocuments, usedTools, fileAnnotations, fileUploads, agentReasoning, action, artifacts, chatType, chatId, memoryType, sessionId, leadEmail, createdDate)
  - `leads` (id, chatflowid, chatId, name, email, phone, createdDate)
  - `feedback` (id, chatflowid, chatId, messageId, rating, content, createdDate)
  - `upsert_history` (id, chatflowid, result, flowData, date)
  - `predictions` (locally-recorded view: id, chatflowid, chatId, question, response_text, usedTools, sourceDocuments, durationMs, capturedAt)
- Sync cursor: `updatedDate` / `createdDate` per resource
- FTS5: `chat_messages.content`, `chatflows.name`, `assistants.details`, `tools.description`, `document_stores.description`, `predictions.question` + `predictions.response_text`

## Codebase Intelligence
- **Source spec**: `FlowiseAI/Flowise:packages/api-documentation/src/yml/swagger.yml` (104 KB, 2,664 lines, 46 endpoints across 13 tags). OpenAPI 3.x fragment — no top-level `info:` or `servers:` block; the server URL is injected at runtime by the Express backend at `/api-docs`. We enrich during Phase 2.
- **Auth**: `bearerAuth` (HTTP Bearer scheme). Header: `Authorization: Bearer <FLOWISE_API_KEY>`. Confirmed against `matthewhand/mcp-flowise` source code.
- **Path prefix**: All endpoints live under `/api/v1/` when served (`/api/v1/chatflows`, `/api/v1/prediction/{id}`, etc.). Spec paths are server-relative without that prefix → must be added via spec `servers:` or generator base URL.
- **Rate limiting**: None documented; self-hosted so backpressure is the user's runtime concern.
- **Key endpoint** (`/prediction/{id}`): body is `{ question, streaming, overrideConfig, history, uploads, humanInput, form }` for JSON, or `multipart/form-data` for file uploads. Response: `{ text, json, question, chatId, chatMessageId, sessionId, sourceDocuments, usedTools }`.

## User Vision
This CLI is operated by a **Hermes agent acting as marketing manager** for a realtor newsletter pipeline. Implications for build:
- Agent-first ergonomics dominate: JSON-default output when stdout is not a TTY, `--select` dotted paths for tight context, structured exit codes, idempotent retries, every Cobra command also an MCP tool.
- Newsletter assembly is a **fan-out pattern**: one parent prompt → N section chatflows → concatenated output. The CLI must make this trivial; a `newsletter compose` transcendence command is core, not edge.
- Document-store ingestion is a **batch import pattern**: realtor source material (MLS exports, market PDFs, neighborhood docs) arrives in folders. `docstore ingest <folder>` should walk + upload + upsert in one call.
- Distribution is **not the CLI's job**; the Flowise chatflow handles SendGrid/email. The CLI just triggers and records.
- Live testing is unavailable in Phase 5 (no host/key); Phase 5 will skip with the `auth_required_no_credential` marker.

## Product Thesis
- **Name**: `flowiseai-pp-cli` (binary), branded "Flowise" in prose
- **Why it should exist**: There is **no purpose-built API-client CLI** for FlowiseAI today. The official `flowise` npm package is a *server CLI* (it starts a Flowise instance). MCP servers exist (3 of them) but live inside MCP clients — they're not callable from a shell, a cron job, or a Hermes agent that wants direct typed exit codes. The official SDKs (`flowise-sdk` / `pip install flowise`) require writing code. Our CLI gives agents a single binary with full 46-endpoint coverage + offline cache + newsletter-grade compound commands. The MCP server is built in, so the same binary serves Claude Desktop, Hermes, or any MCP client — but it also stands alone.

## Build Priorities
1. **P0 — Foundation**: Spec enrichment (add `servers:` with `/api/v1` suffix support, add `auth.env_vars: [FLOWISE_API_KEY]`), generate scaffolding, SQLite store covering all primary entities, `sync` walking the full surface, `search` over chat history + chatflows + predictions.
2. **P1 — Absorb (46 endpoints + competitor parity)**: Every REST endpoint as a typed command with `--json/--select/--csv/--dry-run`, streaming-aware `predict` (SSE consumer), `overrideConfig` JSON passthrough, file-upload `predict` via `--file`, regex-filtered `chatflows list --name <regex>`, base-URL override per call, per-chatflow API-key override.
3. **P2 — Transcendence**: `newsletter compose`, `predict batch` (CSV/NDJSON fan-out), `docstore ingest` (folder walker), `predict replay <chatId>`, `chatflow stale --days N`, `predict history --from-store`, `assistant export / import`, `tokens estimate`.
4. **P3 — Polish**: Enrich terse flag descriptions, write narrative-driven SKILL + README, MCP tool annotations (`read-only`, `hidden` where appropriate), recipes that pair `--agent` with `--select`.

## Source Priority
Single source — FlowiseAI. No combo-CLI ordering needed.

## Catalog & Lock
- No catalog entry for `flowiseai` or `flowise`
- No existing CLI in library
- No active lock
- No prior manuscripts

## Top Wrappers & MCPs Found
| # | Tool | Type | Coverage | Notes |
|---|------|------|----------|-------|
| 1 | [FlowiseAI/FlowiseSDK](https://github.com/FlowiseAI/FlowiseSDK) | Official TS/JS SDK + Python `pip install flowise` | Predictions (streaming) + base-URL/apiKey only | Authoritative auth pattern; minimal endpoint coverage (mostly `predict`) |
| 2 | [MilesP46/FlowiseAI-MCP](https://github.com/MilesP46/FlowiseAI-MCP) | Python MCP server | Claims "complete API coverage" | 30+ tools; uses `FLOWISEAI_URL` + `FLOWISEAI_API_KEY` |
| 3 | [wksbx/flowise-mcp-server](https://github.com/wksbx/flowise-mcp-server) | TypeScript MCP server | Chatflows + predictions + node discovery | Supports CHATFLOW, AGENTFLOW, MULTIAGENT, ASSISTANT types |
| 4 | [matthewhand/mcp-flowise](https://github.com/matthewhand/mcp-flowise) | Python MCP server | LowLevel (dynamic per-chatflow tools) + FastMCP (list + predict only) | Regex/ID whitelist + blacklist filtering of chatflows |
| 5 | [njfio/fluent_cli](https://github.com/njfio/fluent_cli) | Rust CLI | Originally Flowise; current README has **no Flowise commands** | Not a competitor anymore; archived value |
| 6 | Official `flowise` npm package | Node CLI | **Server lifecycle only** (`flowise start`) | Not an API client; orthogonal to our CLI |
