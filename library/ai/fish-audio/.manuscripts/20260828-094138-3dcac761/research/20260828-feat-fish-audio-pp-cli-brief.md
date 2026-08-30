# Fish Audio Printing Press CLI — Research Brief

Scope: TTS, speech-to-text (ASR), voice design, voice models (clones), wallet/credit. Agents/phone product is explicitly out of scope. OpenAPI spec: `/Users/jg-cc/printing-press/.runstate/jg-cc-9fdd8e85/runs/20260828-094138-3dcac761/research/fish-audio-spec.json` (11 endpoints, in-scope subset only).

## API Identity

- **Domain**: Fish Audio (fish.audio) — hosted TTS/ASR/voice-clone platform, base URL `https://api.fish.audio`. Also runs the open-source `fish-speech` model (32.4k GitHub stars) that the hosted API is built on.
- **Users**: developers building voice products (IVR, dubbing, content narration, character voices, agent phone systems — that last one out of scope for us). Public voice library has 2M+ shared voices, of inconsistent quality.
- **Data profile**: mostly write/generate operations (TTS render, ASR transcribe, voice-design generate) rather than a large synced dataset. The one syncable resource is voice **models** (`/model` CRUD) — a small, low-cardinality list per account (owned clones + favorited public voices), not a bulk-data resource. No conversation/session history, no analytics endpoints, no webhooks in the in-scope surface.
- **Auth**: `Authorization: Bearer <FISH_AUDIO_API_KEY>` (`BearerAuth` scheme, per spec `x-auth-env-vars`). Single bearer token, no OAuth, no per-request signing.

## Reachability Risk

**Medium — model-string churn, not endpoint instability.**

- No evidence of the hosted `api.fish.audio` REST API itself going down for extended periods; one isolated ASR 503 report in a GitHub issue ([fishaudio/fish-audio-python#119](https://github.com/fishaudio/fish-audio-python/issues/119)), not a pattern.
- **Model deprecation is real and repeats.** `speech-1.5` and `speech-1.6` were deprecated 2026-02-28 ([deprecations.md](https://docs.fish.audio/developer-guide/models-pricing/deprecations.md)). The Go SDK's `Model` enum marks both `// Deprecated: Use ModelS1 or ModelS2Pro instead`. The *default* model string has moved at least twice across SDK history: `speech-1.5` → `s2-pro` → `s2.1-pro` (current). The `model` header on `/v1/tts` silently falls back to `s2.1-pro` on any unrecognized value — a request never hard-errors on a stale model string, it just quietly serves a different model than intended. **Build implication: never hardcode a model default in generated code; make it a flag/env-configurable value and validate against the live enum, and treat "unrecognized model" as a warning-worthy event, not silence.**
- **The `s2.1-pro-free` "free until 2026-08-31" claim (given in the task) could not be verified anywhere in official docs, the OpenAPI spec, or the pricing page** — pricing page states flatly `$0.00/M UTF-8 bytes` with no expiry. Treat the date as unconfirmed; don't build an expiry-cutover behavior around it without another source.
- **PyPI package versioning reset**: `fish-audio-sdk` restarted at `1.0.0` after being at `2025.6.3`, introducing a new import path (`fishaudio`) alongside the frozen legacy one (`fish_audio_sdk`). Anyone pinning loosely stays on the abandoned code path. Not a risk to us directly (we're OpenAPI-generated, not SDK-wrapped), but confirms this vendor treats breaking changes casually.
- **fish-speech (self-hosted) issue tracker shows real dependency churn** (torchaudio 2.9, Gradio 6.x breakage) — irrelevant to the hosted API we're calling, only relevant if self-hosting is ever considered as a fallback.

## Top Workflows (3-5)

1. **Render TTS to a file with cost tracking.** Text (or a script/dialogue) in, an audio file out, plus a manifest recording model used, bytes, and the marginal spend so it can be compared to ElevenLabs invoices. This is Jon's stated #1 need (Pearl's pinned greeting).
2. **Clone a voice from an audio sample.** Upload one or more reference clips, get back a `model_id` usable as `reference_id` in future TTS calls. Maps to `POST /model` (multipart, `voices[]` files, `train_mode=fast` for instant availability).
3. **Batch-render a script to files.** Multiple lines/paragraphs (or a multi-speaker dialogue using `<|speaker:N|>` tags, S2-family only) rendered to a directory of numbered audio files in one invocation — avoids hand-looping curl calls.
4. **Design a voice from a text instruction (no reference audio).** `POST /v1/voice-design` — generate N candidate voices from a prompt like "warm, confident studio narrator," preview them, then promote the winning candidate into a permanent model via `voice_design_signatures` on `POST /model`.
5. **Check wallet balance before a batch job.** `GET /wallet/self/api-credit` (dev API credit ledger) and `GET /wallet/self/package` (subscription credit ledger) — these are two *separate* ledgers (dev API bills per-UTF8-byte; the web-app subscription runs on a monthly credit pool), a real gotcha worth surfacing in the CLI's balance output rather than showing just one number.

## Table Stakes

Baseline behavior every credible Fish Audio client already provides (from official SDKs + community MCP servers):

- TTS convert-to-bytes and stream-to-file, both JSON and msgpack content types (msgpack required for inline `references` zero-shot cloning).
- `model` header support with the 4-value enum (`s1`, `s2-pro`, `s2.1-pro`, `s2.1-pro-free`) and a documented default.
- ASR transcribe with optional language hint and `ignore_timestamps` toggle.
- Multi-speaker dialogue via `<|speaker:N|>` tags + array `reference_id` (S2 family only, not `s1`).
- Full `/model` CRUD: list (with `title`/`tag`/`self`/`author_id`/`language`/`title_language`/`licensed`/`sort_by` filters), get, create (multipart voices upload), update, delete.
- Voice design: instruction-based candidate generation with `n`, `speed`, `num_step`, `guidance_scale`, `seed`.
- Wallet read: credit balance + package/subscription detail.
- WebSocket real-time streaming TTS (`wss://api.fish.audio/v1/tts/live`) — all three official SDKs (Python, TS, Go) implement this; a CLI that only does request/response chunked TTS is missing table stakes for latency-sensitive use.
- Bracket-tag emotion/prosody control (`[happy]`, `[whispering]`, etc.) passed through in `text` — not a fixed enum, free-text descriptions are accepted by the model itself.
- Error handling for the documented `{status, message, reason}` shape on 401/402/429/503, plus defensive handling of 422 (not itemized per-route in spec but standard FastAPI behavior).

## Data Layer

- **Primary entities**: `Voice`/`ModelEntity` (the only real syncable list — `_id, type, title, description, state, tags[], languages[], visibility, licensed, samples[], created_at, updated_at`). Everything else (TTS output, ASR output, voice-design candidates) is a one-shot generation result, not a persisted, listable resource on Fish Audio's side — there is no `GET /tts/history` in the in-scope spec (contrast with ElevenLabs' `history` endpoint group).
- **Sync cursor**: `/model` list is paginated (`page_size`/`page_number`, `has_more`, `total`, `max_offset`, `window_limited`) — a full local sync should page through with `sort_by=created_at` for a stable, appendable cursor rather than the default `sort_by=score` (score changes over time and would reorder pages mid-sync).
- **FTS**: no server-side full-text search endpoint for models beyond the `title`/`tag` filters. If the CLI wants to support natural-language voice lookup, that means caching the `/model` list locally (SQLite, ElevenLabs-pp-cli precedent) and doing the free-text matching client-side. There is no generation-history resource to index at all, so a local "history" feature is a CLI-owned SQLite table populated at render time, not a synced Fish Audio resource — analogous to how the CLI itself becomes the durable record ElevenLabs provides natively via `history`.

## Codebase Intelligence

- **Auth header + env var**: `Authorization: Bearer <token>`, env var `FISH_AUDIO_API_KEY` (per spec `x-auth-env-vars`) — matches the sibling `elevenlabs-pp-cli` pattern (`ELEVENLABS_API_KEY`), so config file / env precedence should mirror it exactly for consistency across the Printing Press library.
- **`model` header, not a body field or query param**: unusual for this API — TTS model selection is an HTTP header (`model: s2.1-pro-free`), not part of the JSON body. Voice-design uses the same header mechanism but with a different, single-value enum (`voice-design-1`). Generated code must treat `model` as a per-request header option distinct from body params, and must not silently omit it (the API's own fallback-to-default-on-unrecognized-value behavior means a typo is invisible without client-side validation).
- **msgpack vs JSON is functionally load-bearing, not cosmetic**: `/v1/tts` accepts both `application/json` and `application/msgpack`, but **inline `references` (zero-shot voice cloning without a saved model) requires msgpack** — JSON literally cannot carry the binary audio bytes the `ReferenceAudio.audio` field needs inline. `/v1/asr` similarly accepts `multipart/form-data` or `application/msgpack` (no JSON option at all, since audio is binary). Any Go client needs a real msgpack encoder (e.g. `vmihailenco/msgpack`) wired into the request path for `references`/ASR, not just a JSON-only client with a content-type flag flipped.
- **Streaming has three distinct shapes, all different**: (1) `POST /v1/tts` — plain chunked binary stream, no envelope; (2) `POST /v1/tts/stream/with-timestamp` — SSE (`text/event-stream`), each `message` event is JSON with base64 audio + a *replace-not-append* cumulative alignment snapshot keyed by `chunk_seq`; (3) `wss://api.fish.audio/v1/tts/live` — WebSocket, msgpack-framed `start`/`text`/`flush`/`stop` client events and `audio`/`finish` server events. A CLI that unifies these under one `tts stream` command needs three separate wire-protocol handlers, not one HTTP-streaming abstraction.
- **Multi-speaker gating is model-family-based, not a flag**: `<|speaker:N|>` tags + array `reference_id` only work on `s2-pro`/`s2.1-pro`/`s2.1-pro-free`; `s1` explicitly rejects it per the schema description. Client-side validation should reject (or warn) multi-speaker requests against `model: s1` before spending a call.
- **Two separate credit ledgers, both read-only in the spec**: `GET /wallet/{user_id}/api-credit` (dev API, per-UTF8-byte billing, `?check_free_credit=` flag) vs `GET /wallet/{user_id}/package` (subscription/web-app credit pool). `{user_id}` path param defaults to the literal string `"self"` — don't require the caller to look up their own user ID.
- **Voice-design → permanent model handoff via signatures**: `voice-design` candidates aren't directly usable as `reference_id`s; promoting one requires re-uploading the candidate audio to `POST /model` with `voice_design_signatures` (one per uploaded voice file, matched by array order) — an invalid signature rejects the whole create call. This is a two-step workflow (design, then promote) that a CLI should chain into one command (`voice design-and-save` or similar) rather than leaving as two disconnected calls.

## User Vision

User is Jon Gouveia, PixelCove. Primary driver: replacing ElevenLabs for **Pearl's pinned greeting voice** (askpearl.co text-concierge SaaS — position #3 in Jon's net-worth priority stack). Current ElevenLabs voice in use: `yM93hbw8Qtvdma2wCnJG`. Wants:

- **Clone-from-audio**: upload a reference clip (likely of the current ElevenLabs voice output, or a real recorded sample) and get back a usable Fish Audio `model_id` — this is the `POST /model` multipart-voices-upload workflow above, `train_mode=fast` for immediate usability rather than waiting on `full` training.
- **Batch TTS to files**: render a set of greeting/prompt lines to a directory of audio files in one command, not one-at-a-time curl calls — supports iterating on Pearl's greeting copy without re-running the CLI per line.
- **Cost tracking vs ElevenLabs**: the CLI's render output should log per-call byte count and computed cost ($15/M UTF-8 bytes for paid models, $0.00 for `s2.1-pro-free`) so a side-by-side dollar comparison against the ElevenLabs invoice is a `grep`/sum away, not a manual reconstruction. Given the free tier's expiry is unverified, cost tracking should default to computing against the *paid* rate even when the free header is used, so the comparison stays valid once/if the free tier lapses.

## Product Thesis

**Name: `fish-audio-pp-cli`** (matches the `elevenlabs-pp-cli` naming convention already established in the Printing Press library — same install path pattern `github.com/mvanhorn/printing-press-library/library/ai/fish-audio/cmd/fish-audio-pp-cli`).

**Why**: Fish Audio is materially cheaper than ElevenLabs for straightforward narration TTS ($15/M UTF-8 bytes ≈ 180K English words for ~$15, plus a genuinely free tier for prototyping) and offers the same core primitives Pearl actually uses — voice cloning and single-line TTS render — without the agent-platform/dubbing/studio surface area Jon doesn't need. The CLI's job is narrow and deep: nail render-to-file, clone-from-audio, and cost tracking with the same agent-native ergonomics (`--agent` compact JSON, `--deliver file:<path>`, manifest output) that made `elevenlabs-pp-cli` usable inside an agent loop, so switching Pearl's greeting pipeline is a drop-in command swap rather than a rewrite.

## Build Priorities

1. **`tts render`** — text/file in, audio file + JSON manifest (model, bytes, computed cost, format) out. Table stakes, and the direct unblock for Pearl.
2. **`voice clone`** (aka `model create` wrapped) — audio file(s) in, `model_id` out, `train_mode=fast` default. Directly serves the "clone-from-audio" ask.
3. **`tts batch`** — multi-line/file input, numbered output directory, aggregate cost summary. Serves "batch TTS to files."
4. **`wallet balance`** — both ledgers (`api-credit` + `package`) in one call, clearly labeled, so cost-vs-ElevenLabs tracking has a real number to reconcile against.
5. **`voice design`** and **`voice design-save`** (design-to-model promotion via signatures) — lower priority than the above four, but needed for the "no reference audio available" path and closes a real two-step API gap the raw endpoints leave open.
6. **`model list`/`get`/`update`/`delete`** — generated CRUD, needed for completeness and to support `voice discover`-style lookup once a local cache exists, but not on the critical path for the Pearl greeting migration.
7. **Streaming (`tts stream`, WebSocket, SSE-with-timestamp)** — table stakes per the SDK survey, but Pearl's pinned-greeting use case is static pre-rendered audio, not live playback; defer behind the five items above unless a second use case (live voice) emerges.
