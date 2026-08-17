# Sarvam AI CLI Brief

## API Identity
- Domain: Indian-language AI — speech-to-text, text-to-speech, translation, transliteration, language ID, chat completion (sarvam-105b), document intelligence (doc-ai), pronunciation dictionaries
- Users: Developers building multilingual Indic products — voice agents, IVR systems, translation pipelines, call centers, fintech/healthcare document extraction, content localization
- Data profile: ~35 REST endpoints across 8 resource families on api.sarvam.ai; plus a separate Voice Agents platform (deployments/campaigns/analytics) on apps.sarvam.ai with different auth
- Base URL: https://api.sarvam.ai (core AI), https://apps.sarvam.ai (voice agents, separate)

## Reachability Risk
- None. Live probe: `GET /v2/models` → 200 (no auth). `POST /translate` with key → 200 with Hindi translation. No bot protection. Community research: no 403/blocked issues beyond the documented "invalid key returns 403 not 401" quirk (an error-UX concern, not a reachability blocker).
- Auth: `api-subscription-key` header (sk_ format). Bearer alternative accepted. 403 + `invalid_api_key_error` is the auth-failure signal (not 401).

## Top Workflows
1. Translate/transliterate text between English and 22 Indic languages (chat support, content localization)
2. Convert text to speech with a chosen voice and save/play audio (IVR prompts, notifications)
3. Transcribe audio to text — REST for short clips, batch job API for long files with diarization (call centers, meeting notes)
4. Chat completion with sarvam-105b for multilingual assistants (streaming, tools)
5. Extract structured fields from documents via doc-ai (insurance policies, invoices, KYC)

## Table Stakes
- Text: translate, transliterate, identify language
- Speech: STT (sync + batch jobs), TTS (REST + stream)
- Chat: completions with streaming and tool calling
- Pronunciation dictionaries CRUD
- Doc-ai digitise/extract with async job polling

## Data Layer
- Primary entities: translation history, TTS generations (request_id, text, language, voice), STT transcriptions, chat completions (usage tokens), pronunciation dictionaries, doc-ai jobs
- Sync cursor: request_id-based; jobs polled to terminal state
- FTS/search: local history search across translations/transcriptions/chat

## Product Thesis
- Name: `sarvam-pp-cli` — "Sarvam AI, on your terminal"
- Why it should exist: The official SDKs (Python/JS) and MCP server are great for code, but there is NO offline-capable CLI. Existing CLIs (`sarvam-cli`, `sarvamai-cli`) are thin interactive wrappers — no local history, no batch scripting, no job orchestration, no agent-native output. A printing-press CLI gives: local SQLite history of every translation/TTS/STT/chat call, `--json`/`--agent` output, offline search across past work, dry-run safety, and typed exit codes. It also absorbs the MCP server's full tool surface (translate/tts/stt/chat/lid/doc-ai/pronunciation) into scriptable commands.

## Build Priorities
1. Text: translate / transliterate / identify-language (with full enum support, --output-script, --numerals-format)
2. TTS: convert (save base64 audio to file), stream; voices/models enums
3. STT: transcribe (multipart upload), batch job lifecycle (initiate → upload → start → status → download)
4. Chat: completions with --stream, --json
5. Doc-ai: extract/digitise + job status polling + results
6. Pronunciation dictionaries CRUD

## Voice Agents Platform (scope note)
- apps.sarvam.ai API family (deployments, campaigns, cohorts, analytics, instant outbound) uses a SEPARATE `X-API-Key` auth + org_id/workspace_id path scoping. The user's provided key is the core API key (sk_). This platform is documented in the brief but NOT in the v1 CLI spec — it's a deliberate second phase (different auth system, different base URL). The CLI will note the distinction in docs.

## Competitors (from ecosystem research)
- Official `sarvamai` SDK (PyPI/npm) — Fern-generated, typed
- Official `sarvam-mcp` (uvx sarvam-mcp) — 15 tools
- `sarvamai-cli` (npm) — agentic coding assistant
- `sarvam-cli` (PyPI) — chat/voice/text interactive wrapper
- `langchain-sarvam`, `sarvam-ai-sdk` (Vercel AI), `n8n-sarvam-node`
- Pain points to beat: 403-not-401 auth errors, model deprecations, reasoning_content in streams, bulbul:v3 param traps, async job download bugs

## Key API Facts (from docs)
- Translate: mayura:v1 (11 langs, 1000 chars, auto-detect) / sarvam-translate:v1 (22 langs, 2000 chars, formal only). ₹20/10K chars.
- TTS: bulbul:v3 (30+ voices, temp, pace 0.5-2.0, no pitch/loudness) / bulbul:v2 (pitch/loudness). ₹30/10K v3. Default sample rate 24000.
- STT: saaras:v3/v4; modes transcribe/translate/verbatim/translit/codemix. ₹30/hr (₹45 with diarization).
- Chat: sarvam-105b (₹29.28/M input) / sarvam-105b-conversations; 128K context; reasoning_content; rate limit 40 req/min on 105b.
- Doc-ai: async jobs, terminal states completed/partially_completed/failed/rejected.
- Rate limits: token-bucket; STT 60/min, TTS 60/min, translate 60/min, chat 40/min (105b) on starter.
