# Sarvam AI CLI — Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Translate text | sarvam-mcp `sarvam_translate` / SDK `text.translate` | sarvam-pp-cli translate | Offline history, --json, --dry-run, full enum validation (22 langs, modes, output_script, numerals_format), batch via --stdin |
| 2 | Transliterate | sarvam-mcp `sarvam_transliterate` / SDK `text.transliterate` | sarvam-pp-cli transliterate | Offline history, --json, spoken-form support |
| 3 | Identify language | sarvam-mcp `sarvam_identify_language` / SDK `text.identify_language` | sarvam-pp-cli detect-language | Offline history, --json, script_code output |
| 4 | Text-to-speech convert | sarvam-mcp `sarvam_tts_speak` / SDK `textToSpeech.convert` | sarvam-pp-cli tts | Saves decoded audio to file, --output, --dry-run, voice/model enums, offline history of generations |
| 5 | TTS stream | sarvam-mcp `sarvam_tts_stream` / SDK `convertStream` | (behavior in sarvam-pp-cli tts --stream) | Streams audio to stdout/file, codec/bitrate control |
| 6 | Speech-to-text transcribe | sarvam-mcp `sarvam_stt_transcribe` / SDK `speechToText.transcribe` | sarvam-pp-cli stt | Multipart upload, 5 modes, language detection, timestamps, offline history |
| 7 | STT translate (to English) | SDK `speechToText.translate` | (behavior in sarvam-pp-cli stt --mode translate) | Combined transcribe+translate |
| 8 | Chat completions | sarvam-mcp `sarvam_llm_complete` / SDK `chat.completions` | sarvam-pp-cli chat | Streaming (--stream), tools, reasoning_effort, wiki_grounding, offline history with token usage |
| 9 | Doc-ai digitise | sarvam-mcp `sarvam_vision_extract` / SDK `docAi.digitise` | sarvam-pp-cli docai digitise | Async job with status polling, output download |
| 10 | Doc-ai extract (structured) | sarvam-mcp `sarvam_vision_extract` / SDK `docAi.extract` | sarvam-pp-cli docai extract | Schema-driven extraction, annotations with confidence |
| 11 | Doc-ai job status | sarvam-mcp `sarvam_vision_job_status` / SDK `docAi.get_status` | sarvam-pp-cli docai status | Typed exit codes, --wait for terminal state |
| 12 | Doc-ai job results | SDK `docAi.get_results` | sarvam-pp-cli docai results | --json with annotations |
| 13 | Doc-ai download URL | SDK `docAi.get_download_url` | sarvam-pp-cli docai download-url | Direct URL output |
| 14 | Pronunciation list | sarvam-mcp `sarvam_pronunciation_list` / SDK `pronunciationDictionary.list` | sarvam-pp-cli pron-dict list | Offline cache, --json |
| 15 | Pronunciation get | sarvam-mcp `sarvam_pronunciation_get` / SDK `pronunciationDictionary.get` | sarvam-pp-cli pron-dict get | Offline cache |
| 16 | Pronunciation create | sarvam-mcp `sarvam_pronunciation_create` / SDK `pronunciationDictionary.create` | sarvam-pp-cli pron-dict create | --dry-run, file validation |
| 17 | Pronunciation update | SDK `pronunciationDictionary.update` | sarvam-pp-cli pron-dict update | --dry-run |
| 18 | Pronunciation delete | sarvam-mcp `sarvam_pronunciation_delete` / SDK `pronunciationDictionary.delete` | sarvam-pp-cli pron-dict delete | --dry-run, typed exit codes |
| 19 | Batch STT job lifecycle | SDK `speechToTextJob` (initiate/upload/start/status/download) | sarvam-pp-cli stt-job initiate/upload/start/status/download | Full orchestration, --wait, per-file status |
| 20 | List models | SDK `open_source_models.list_models_v2` / GET /v2/models | sarvam-pp-cli models | Offline cache, --json |
| 21 | Set API key | sarvam-mcp `sarvam_tools_set_api_key` / SDK env | sarvam-pp-cli auth set-token | Validates sk_ format, secure config |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Voice auditioning | voices preview --lang hi-IN --sample "text" | 8/10 | hand-code | Calls POST /text-to-speech once per speaker from the speaker enum for a fixed sample, decodes base64 audios to labeled files, paced under the 60/min TTS rate limit | OpenAPI enumerates 45 speakers with no speaker→language map; brief lists "bulbul:v3 param traps" as a competitor pain | Use this command to audition many TTS voices on one sample text before committing to a speaker. Do NOT use it to verify how a specific brand term sounds; use 'pron-check'. |
| 2 | Conversation resume | chat resume <conversation-id> "message" | 8/10 | hand-code | Loads the stored messages array for a conversation from local SQLite history (drain-first), then calls POST /v1/chat/completions to continue it and persists the continuation | Brief thesis: differentiator is persistent local history; chat endpoint requires the full message array, which only a local store can replay | Use this command to continue a previous chat session from local history with full context. Do NOT use it to start a fresh conversation; use 'chat'. |
| 3 | Batch job retry | stt-job retry <job_id> --failed-only | 8/10 | hand-code | Filters per-file state (API Error/Internal Server Error) from JobStatusResponse.job_details, then re-runs initiate → upload → start on exactly those file names | OpenAPI job_details exposes per-file state; brief lists "async job download bugs" as competitor pain | Use this command to re-run only the failed files of a batch STT job. Do NOT use it to inspect why files failed; use 'stt-job report'. |
| 4 | Pronunciation spot-check | pron-check "term" --lang hi-IN | 7/10 | hand-code | Calls POST /text-to-speech for the term (optionally with dict_id), transcribes the generated audio via POST /speech-to-text, prints both strings with a normalized-equality verdict | TTS dict_id + STT endpoints in spec; pron-dict CRUD ships in every competitor but none verifies a dict changes pronunciation | Use this command to verify TTS pronunciation of specific terms via a speech round-trip. Do NOT use it to audition many voices; use 'voices preview'. |
| 5 | Subtitle export | subs --from <id\|last> --format srt | 7/10 | hand-code | Reads SpeechToTextResponse.timestamps rows (words + start/end seconds) from local SQLite history and formats cue blocks into .srt/.vtt | OpenAPI timestamps returns word-aligned start/end arrays — subtitle-ready; content localization is a named user segment and competitors return raw JSON only | Use this command to emit .srt/.vtt subtitles from timestamped transcriptions in local history. Do NOT use it to transcribe new audio; use 'stt'. |
| 6 | Extraction schema library | docai schema list | 7/10 | hand-code | Stores extraction JSON schemas (name, doc-type, schema, updated_at) as SQLite rows with list/diff subcommands; schemas feed the schema parameter of docAiExtract | DocAiGetResultsResponse requires a per-call schema with confidence annotations; brief names fintech/healthcare extraction users whose schemas live in gists | Use this command to save/list/edit extraction schemas locally. Do NOT use it to run extractions; use 'docai batch'. |
| 7 | Folder extraction run | docai batch --schema invoice-v1 --dir ./docs/ | 6/10 | hand-code | Per file: upload presign → upload → extract with saved schema → poll status to terminal state with pacing and --wait, writing one results JSON per document | Brief Top Workflow 5 is batch document extraction; DocAiStartJobResponse/DocAiJobStatusResponse give terminal states | Use this command to run a saved schema over a folder of documents with pacing. Do NOT use it to run a single document; use 'docai run'. Do NOT use it to edit schemas; use 'docai schema'. |
| 8 | Batch job report | stt-job report <job_id> --json | 7/10 | hand-code | Calls sttJobStatus, digests total/successful/failed counts plus per-file job_details into a table, persists to history, exits non-zero on any failure | OpenAPI JobStatusResponse carries the exact counts and per-file states; call-center batch transcription is a top workflow needing cron-alertable typed exit codes | Use this command to get a per-file digest of a batch STT job with typed exit codes. Do NOT use it to re-run failed files; use 'stt-job retry'. |

## Buildability Summary
- hand-code: 8 (all transcendence features)
- spec-emits: 0

## Scope Notes
- Voice Agents platform (apps.sarvam.ai: deployments, campaigns, cohorts, analytics, instant outbound) uses a SEPARATE X-API-Key auth + org_id/workspace_id scoping. NOT in v1 spec (different auth system, different base URL). Documented in brief as a deliberate second phase.
- WebSocket streaming endpoints (realtime STT, TTS WS, STT-translate WS) are AsyncAPI surfaces, out of scope for the REST-focused printed CLI v1.
- Legacy endpoints (document-intelligence, speech-to-text-translate job) are deprecated, excluded.

## User Vision
- Not provided (user selected "Let's go")
