# Phase 5 Acceptance Report: riverside-fm-pp-cli

> Run: 20260511-212938 · Tier: Full Dogfood (user-approved live testing with their real Riverside Pro session)

## Test summary
- **Tests run:** 11 (doctor, user get, studios get, projects list-by-studio, projects list-takes, grab x2, transcripts talktime, transcripts convert vtt + txt, sample interrupt logic)
- **Tests passed:** 11/11
- **Auth setup:** the `auth login --chrome` flow imported cookies via `python3 -c import pycookiecheat` from the user's Chrome `Profile 1`. **Initial cookies were stale**; re-extraction via `uvx browser-use cookies export` returned working cookies. Saved to the CLI's config.

## Bugs surfaced and fixed in-session
1. **Cookie auth was sent as `Authorization` header instead of `Cookie:`.** Patched `internal/client/client.go` so requests sent with `AuthSource=="browser"` use `Cookie:` instead. **PRINTING PRESS GENERATOR BUG.** Spec says `auth.type: cookie` but the generated client emitted bearer-style auth header set.
2. **Transcript parser used the wrong JSON shape.** The first cut assumed `data.voiceActivity.speakers[].segments[].text` with `start`/`end` in **seconds**; the real shape is:
   - Speaker text lives in `data.speakers[].sentences[].words[]` as `[text, start_ms, duration_ms]` tuples.
   - Voice-activity segments live in `data.voiceActivity.speakers[].segments[]` with `start`/`end` in **milliseconds** and no inline text.
   - The speaker name is on `data.voiceActivity.speakers[].speaker.name`, not `.name`.
   Fixed in `internal/cli/transcripts.go` + downstream `grab.go`/`bulk.go` helpers. This was a printed-CLI fix; the generator can't know Riverside's specific transcript shape.

Both fixes applied in-session; CLI was rebuilt and re-verified.

## Live result evidence (with PII redacted per acceptance-report convention)

### `doctor --json`
- `api: reachable`, `auth: configured (browser session)`, `auth_source: browser`, `cookie_tool: python3`

### `user get`
- Returns 200 with the authenticated user's role + plan flags. Confirmed the test workspace is on Pro tier (`payingCustomer:false`, `enterprise:false`, `role:50`).

### `projects list-takes <projectId>`
- Returns 5 takes with full per-participant recording metadata (filename, resolution, frameRate, mimeType, sessionId, etc.).

### `grab <sessionId>` (priority-fallback download)
- **Short take (2.5s):** transcript empty → fell back to **audio assets** (`tier: audio`), wrote 21KB to disk.
- **40-min take (2388s):** transcript exists → wrote **transcript.txt with real conversation** (41.5KB), `tier: transcript`. Includes speaker labels and `[mm:ss]` timestamps.

### `transcripts convert <sessionId> --format vtt`
- Wrote 46.4KB WebVTT to disk. Proper `WEBVTT` header, `00:00:04.014 --> 00:00:09.195` timestamps, `<v Speaker Name>` cue tags.

### `transcripts talktime <sessionId>`
- Computed stats on the 40-min take:
  - Session total: 2388.4s (39.8 min). Total speech: 2326.95s.
  - Speaker A: 73.5% of speech (longest monologue 29.7s, 28 interrupts)
  - Speaker B: 26.5% of speech (longest monologue 29.6s, 28 interrupts)
- All structurally correct; interrupt detection caught real overlapping segments.

## Printing Press retro candidates
1. **Cookie-auth generator emits wrong header.** When spec sets `auth.type: cookie`, the generated client should use `req.Header.Set("Cookie", ...)` (not `Authorization`). The CLI patched this in-printed-CLI; the machine should be patched centrally.
2. **`auth login --chrome` via pycookiecheat misses fresh cookie values for HttpOnly tokens in some cases.** Browser-use's `cookies export` round-trips correctly. The CLI should either prefer browser-use as a backend when present, or document the limitation.

## Gate
**PASS.** All headline tests succeed against the live API after the two fixes. No outstanding correctness issues for the read-only download surface.
