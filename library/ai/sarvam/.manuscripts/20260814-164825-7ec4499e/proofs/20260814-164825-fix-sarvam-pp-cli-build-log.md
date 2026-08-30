Manifest transcendence rows: 8 planned, 8 built. Phase 3 passes.

## What was built
- All 8 transcendence features implemented (hand-code):
  1. voices preview — TTS speaker auditioning with labeled WAV output (live)
  2. chat resume — continue a past chat thread from local history (auto: local read + live continue)
  3. stt-job retry — re-run failed batch files via initiate→upload→start orchestration (live)
  4. pron-check — TTS→STT pronunciation round-trip with normalized comparison (live)
  5. subs — .srt/.vtt subtitle export from timestamped transcriptions (local)
  6. docai schema — save/list/get/diff/delete extraction schemas in local store (local)
  7. docai batch — run saved schema over a folder with presign→upload→extract→poll→results (auto)
  8. stt-job report — per-file batch digest with typed exit codes for cron (live)
- All 21 absorbed features covered by generated endpoint commands (translate, transliterate, text-lid, text-to-speech, speech-to-text, chat, models, pron-dict CRUD, stt-job lifecycle, doc-ai job endpoints)
- Priority 0 (data layer): generated SQLite store with typed Upsert* methods; every API call persists to local history automatically
- Priority 1 Review Gate: sampled translate/chat/models — --help, --dry-run, --json all correct

## Intentionally deferred
- Voice Agents platform (apps.sarvam.ai): separate X-API-Key auth, out of scope (documented in README/brief)
- WebSocket streaming endpoints: AsyncAPI surfaces, out of scope

## Skipped body fields
- None blocking; complex nested chat response_format/tools accepted as JSON-string flags

## Generator limitations found
- None blocking
