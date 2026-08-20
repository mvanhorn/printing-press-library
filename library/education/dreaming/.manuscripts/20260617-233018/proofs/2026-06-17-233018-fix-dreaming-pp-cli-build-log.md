# Dreaming CLI — Build Log

## Foundation (generated)
- OpenAPI spec hand-authored from crowd-extracted internal API (8 resources, 10 endpoints).
- Typed SQLite domain tables: videos, external_time, day_watched_time, user, playlist (+ generic resources/FTS).
- Endpoint-mirror commands: user, videos, video, playlist, day-watched-time, external-time (list/add/delete), inspect-external-video, sign-in.
- Framework: sync, search, sql, export, import, doctor, auth, agent-context, MCP (cobratree mirror).

## Hand-built (Phase 3)
- transcript <id> + transcript sync — fetch inline WEBVTT, clean, cache cue-level (new transcript_cues FTS table in store/transcripts.go).
- concordance — KWIC corpus search: FTS words/phrases, --regex, --tense (es morphology presets), --level/--guide/--dialect scope, timestamps.
- next — roadmap-aware unwatched picker (user level -> videoLevelBands -> videos minus playlist, difficulty-sorted).
- roadmap — L1-L7 ladder + CI advisories + per-level ETAs from recent pace; --related halving.
- diet — windowed difficulty-progression trend (playlist x videos.difficulty).
- plan — required-min/day to an hours target or level by a date, vs current pace.
- stats (+ by-guide, by-tag) — totals, rolling avgs, streak, best day, breakdowns.
- external import <csv> — bulk POST /externalTime (date/minutes|hours|seconds/description/type), --dry-run, rate-limited, verify-safe.
- download <id> — resolve sources.bunny HLS + youtube; print URL + ffmpeg cmd by default; --exec downloads; verify-safe.
- migrate — honest stub (needs destination --to-token).

## Generator bugs found + patched in generated store.go (see retro-notes.md)
- extractObjectID + genericIDFieldFallbacks omitted '_id' (Mongo/Dreaming convention) -> UpsertVideos/Batch failed. Added '_id'.
- id-less association resources (playlist keyed by videoId, day_watched_time keyed by date) failed 'missing id'. Added natural-key fallbacks.

## Tests
- dreaming_domain_logic_test.go: levelForHours, videoLevelBands, parseVTT, vttToPlainText, tense presets, parseWindowDays, parseTargetHours, streaks.
- go test ./... green; go vet clean.

## Behavioral acceptance (offline, seeded store)
- stats/roadmap/diet/plan math verified; concordance word + subjunctive/conditional/future tenses verified with timestamps; next level-band + unwatched correct; by-guide join correct; write-command dry-runs exit 0.

## Deferred / stubs
- migrate: stub (needs second account credentials). Documented.
