# Spotify Transcript Endpoint Capture (2026-05-17)

Captured live from logged-in Premium session on 2026-05-17 against the
Acquired episode "10 Years of Acquired (with Michael Lewis)"
(episode id `5PwtWcgg71nSkb63ZV4hGX`).

## Endpoint

```
GET https://spclient.wg.spotify.com/transcript-read-along/v2/episode/{episodeId}?format=json&maxSentenceLength=500&excludeCC=true
```

## Authentication

The Spotify web app uses an `Authorization: Bearer <token>` header injected
by its service worker (`https://open.spotify.com/service-worker.js`). The
bearer is derived from the `sp_dc` HttpOnly cookie via a TOTP-signed
bootstrap flow that calls into Spotify's web-player token endpoint
(opaque from JS). The token has roughly a 1-hour TTL.

For v0.1, podcast-goat asks the user to manually copy a fresh bearer token
from DevTools Network panel after navigating to any episode page on
open.spotify.com, and to set `SPOTIFY_BEARER` in the environment. v0.2 will
automate the bootstrap.

## Response shape

```typescript
{
  version: "1.0",
  transcriptUri: "spotify:transcript:<uri>",
  publishedAt: ISO timestamp,
  language: "en-us",
  section: Array<
    | { startMs: number, title: { title: string } }
    | { startMs: number, text: { sentence: { startMs, text, highlight[] } } }
    | { startMs: number, fallback: {...}, musicClosedCaption: {...} }
  >,
  showName: string,
  episodeName: string,
  shareable: boolean,
  timeSyncedStatus: "SYLLABLE_SYNCED" | other
}
```

Section semantics:
- `title.title` matching regex `^Speaker \d+$` is a speaker turn boundary
  (Spotify's auto-diarization emits "Speaker 1", "Speaker 2", ...; real
  names appear when the show has hand-edited transcripts)
- `title.title` otherwise is a chapter/section header
- `text.sentence` is a single sentence emitted by the current speaker
- `fallback` + `musicClosedCaption` marks non-speech content (intro music,
  ads, etc.)

## Test fixture

`library/podcast-goat/internal/source/spotify/spotify_test.go` carries a
12-section excerpt of this response with the highlight arrays stripped,
exercising both speaker-turn merging and chapter-header capture.
