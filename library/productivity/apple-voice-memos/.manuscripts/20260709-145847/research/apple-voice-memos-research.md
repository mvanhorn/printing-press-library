# Apple Voice Memos local-store research

## Source boundary

Apple Voice Memos on macOS stores recording metadata in `CloudRecordings.db` and recording media in the adjacent group-container directory. Recent recordings can include Apple-generated transcript JSON in an ISO-BMFF `tsrp` atom. Apple does not publish these formats as a supported API.

## Existing-tool review

Printing Press Library had no Apple Voice Memos entry. Existing public projects covered subsets of export, MCP access, watcher-driven transcription, or paid third-party speech-to-text. None combined freshness-aware local synchronization, embedded Apple transcript extraction, versioned agent JSON, and private export semantics.

## Design decisions

- Open SQLite in read-only and query-only mode.
- Never modify or delete Apple records or recordings.
- Treat `recent` as freshness-sensitive and refresh by default.
- Kick `voicememod` first, then use a hidden Voice Memos launch only as fallback.
- Refuse ambiguous process cleanup and verify PID, executable, and process-start identity before termination.
- Extract Apple’s embedded transcript locally instead of uploading audio.
- Use synthetic SQLite and M4A fixtures in the public test suite.
- Document that unchanged local state does not prove iCloud has nothing newer.

## Compatibility risk

The SQLite schema, media atom, daemon behavior, and app path are undocumented Apple internals. The CLI validates known schema columns and fails clearly when compatibility changes instead of silently returning wrong data.
