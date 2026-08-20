# Spotify Web API CLI Brief

## API Identity
Spotify Web API is the official, mature REST surface (`https://api.spotify.com/v1`) for Spotify's music and podcast catalog plus a logged-in user's library and active playback session. It serves three audiences: app developers building Spotify-aware tools, data folks pulling listening history and metadata, and agents/automations that need to read or steer what a user is hearing. Data covers tracks, albums, artists, genres, playlists, podcasts/shows, episodes, saved-library state, top-items rollups, recently-played, devices, and playback state. The API is free to use under the standard developer-app quota; reading public catalog data only requires a registered app. Mutating playback (`/me/player/*` write operations) requires the user to be a Spotify Premium subscriber — non-Premium accounts receive `403 PREMIUM_REQUIRED` on those endpoints. Reading playback state and library data works for free accounts.

Official OpenAPI spec: `https://developer.spotify.com/reference/web-api/open-api-schema.yaml`. A community-fixed mirror suitable for code generation lives at `https://raw.githubusercontent.com/sonallux/spotify-web-api/main/fixed-spotify-open-api.yml` (sonallux maintains corrections for issues in the official spec that trip codegen tools).

## Reachability Risk
**Low** — official, stable, well-documented. Two gotchas to design around:
- **Rate limit**: rolling 30-second window; 429 returns `Retry-After` in seconds. Development-mode apps get a lower threshold than extended-quota apps. The client must honor `Retry-After`.
- **Quota tier asymmetry (deprecation)**: On 2024-11-27 Spotify removed access for *new apps* to: `GET /audio-features`, `GET /audio-analysis`, `GET /recommendations`, `GET /artists/{id}/related-artists`, `GET /browse/featured-playlists`, `GET /browse/categories/{id}/playlists`, 30-second preview URLs in multi-get track responses, and algorithmic/editorial playlists. Apps that already held a pending or granted extended-quota grant before that date retain access. **Implication for this CLI**: detect 403/404 on these endpoints at runtime and surface a clear "this endpoint is only available to legacy quota-extended apps" error rather than silently failing. The user's credentials in `.env` are for a personal dev app — these endpoints will return errors.

## Top Workflows
1. **Steer current playback** — see what's playing on which device, then play/pause/skip/seek/volume/shuffle/repeat from the terminal.
2. **Search the catalog** — query tracks, albums, artists, playlists, shows, episodes with one command and a `--type` filter.
3. **Curate playlists** — list owned playlists, create new ones, add/remove tracks, reorder, snapshot-diff against a saved baseline.
4. **Manage saved library** — list/add/remove saved tracks, albums, followed artists, saved shows.
5. **See top items** — top tracks and top artists by `short_term` / `medium_term` / `long_term` range; useful for "what's been on heavy rotation this month."
6. **Pull recently played** — capped at 50 by API; a sync loop run frequently extends the local history far past that cap.
7. **Transfer playback between devices** — list available devices, move the stream to phone/desktop/speaker without leaving the terminal.
8. **Podcast/show workflow** — saved-shows list, episode lookup, mark playback resume positions.

## Table Stakes
What competitors already do — the absorbed-feature floor:
- **Playback control** (play/pause/next/prev/seek/volume/queue/shuffle/repeat): `spotify-tui` (Rust TUI + CLI), `ncspot`, `spotipy` (Python library, not a CLI). `shpotify` controls the *desktop app via AppleScript*, not the Web API — distinct mental model.
- **Search across all entity types**: `spotipy.search()`, `spotify-tui`, every MCP server.
- **Library reads (saved tracks/albums/artists/podcasts)**: `spotipy`, `spotify-tui list --liked --limit 50`, `marcelmarais/spotify-mcp-server` (`getUsersSavedTracks`).
- **Playlist CRUD** (create, list, add/remove/reorder, transfer-of-ownership semantics — note: Spotify doesn't truly transfer ownership; you collaborate-clone): `spotipy`, `spotify-tui`.
- **Top items by time range**: `spotipy.current_user_top_tracks(time_range=)`.
- **Recently played (50-cap)**: `spotipy`, `marcelmarais/spotify-mcp-server` (`getRecentlyPlayed`).
- **Recommendations seeds + targets**: `spotipy.recommendations()` — but **deprecated for new apps** since 2024-11-27.
- **Audio features (tempo/energy/valence/danceability)**: `spotipy.audio_features()` — **deprecated for new apps**.
- **Audio analysis (bars/beats/segments)**: `spotipy.audio_analysis()` — **deprecated for new apps**.
- **Devices + transfer playback**: every wrapper.
- **Auth**: `spotipy` supports Auth Code, Auth Code + PKCE, Client Credentials. `spotify-tui` uses Auth Code with secret. MCP servers default to Auth Code with secret.

Reference MCP servers worth surveying for tool shape: `marcelmarais/spotify-mcp-server` (lightweight, ~10 tools), `PeterAkande/spotify_mcp` (36 tools, FastMCP), `varunneal/spotify-mcp`, `tylerpina/spotify-mcp`. Closest "CLI of record" is `spotify-tui`'s `spt` binary.

## Data Layer
Primary SQLite tables:
- `tracks` (id PK, name, album_id, duration_ms, popularity, explicit, isrc, preview_url, fetched_at)
- `track_artists` (track_id, artist_id, position) — junction
- `albums` (id PK, name, release_date, total_tracks, album_type, label, fetched_at)
- `album_artists` (album_id, artist_id, position)
- `artists` (id PK, name, popularity, followers_total, fetched_at)
- `artist_genres` (artist_id, genre) — junction
- `playlists` (id PK, name, owner_id, public, collaborative, snapshot_id, tracks_total, description, fetched_at)
- `playlist_tracks` (playlist_id, track_id, position, added_at, added_by) — junction; snapshot-keyed
- `saved_tracks` (track_id PK, saved_at)
- `saved_albums` (album_id PK, saved_at)
- `followed_artists` (artist_id PK, followed_at — synthetic; API only returns "is following")
- `saved_shows` (show_id PK, saved_at)
- `shows` / `episodes` (id PK, name, publisher, duration_ms, …)
- `top_tracks_snapshot` (snapshot_id, time_range, track_id, rank, captured_at) — history of pulls so we can compute rotation drift
- `top_artists_snapshot` (parallel)
- `play_history` (played_at PK, track_id, context_uri) — extended past the 50-cap by periodic syncs
- `devices_seen` (device_id PK, name, type, last_seen_at) — for "which speaker did I use last?"

**Sync cursors**: playlists keyed by `snapshot_id` (Spotify's playlist version stamp — re-fetch only when changed); `last_fetched_at` per table for saved-library and top-items; `played_at` watermark for `play_history`.

**FTS candidates**: `tracks.name`, `albums.name`, `artists.name`, `playlists.name`, `episodes.name` — one FTS5 table per entity (or unified with a `kind` column).

**Deprecation footnote**: an `audio_features` table is conditional on whether the user's app has legacy quota access. Generate the schema and migration; gate population behind a runtime feature-check.

## Codebase Intelligence
- **Auth flows**: Authorization Code with PKCE (recommended for CLI — no secret stored), Authorization Code with secret (works, but the secret in `.env` is unnecessary risk for a local tool), Client Credentials (app-only — reads public catalog but cannot read user library or control playback).
- **PKCE flow**: generate `code_verifier` (random 43–128 char string), derive `code_challenge = base64url(sha256(verifier))`, redirect user to `/authorize` with `code_challenge_method=S256`, capture `code` on redirect callback (loopback `http://127.0.0.1:<port>/callback`), POST to `/api/token` with `code` + `code_verifier` to get `{access_token, refresh_token, expires_in: 3600}`.
- **Token refresh**: access tokens last 1 hour. Refresh tokens are long-lived but rotate on each refresh (the response includes a new `refresh_token` — must persist the new one).
- **Rate-limit headers**: `Retry-After: <seconds>` on 429. Implement exponential-backoff jitter on top.

## User Vision
The user provided `SPOTIFY_CLIENT_ID` and `SPOTIFY_SECRET` in `.env` and asked for a generic build. They're set up for the secret-based Authorization Code flow, but PKCE is the better default for a local CLI (the secret in `.env` is not needed and is a small leak risk). **Build the CLI to support both**: default to PKCE when only `SPOTIFY_CLIENT_ID` is set; use Authorization Code with secret when `SPOTIFY_SECRET` is also present (some power users prefer it for parity with their server-side dev apps). Auto-detect; don't make the user pick.

## Product Thesis
`spotify-pp-cli` beats `spotipy` (Python library — not a CLI), `shpotify` (controls desktop app, not the Web API), and `spotify-tui` (interactive TUI, not agent-friendly) on two axes: **(1)** a single static binary with `--select`/`--json` everywhere and a built-in MCP server, so an agent can drive Spotify with the same tools a human uses; **(2)** a SQLite-backed local library that lets you ask "saved tracks added in the last 30 days," "playlists whose snapshot changed since I last looked," or "top tracks that dropped out of medium-term but stayed in long-term" as one-shot SQL — none of the existing CLIs persist enough state to answer those. The transcendence move is **drift queries on top of `top_tracks_snapshot`** (rotation analytics nobody else builds) and **snapshot-aware playlist diffing** (Spotify gives us `snapshot_id` for free; nobody exposes it in CLI form).

## Build Priorities
1. **`auth login`** — OAuth PKCE by default, secret-based fallback if `SPOTIFY_SECRET` set; loopback redirect on `127.0.0.1:<port>`; cache token + refresh in `~/.config/spotify-pp-cli/token.json`; auto-refresh on use.
2. **`now-playing`** — current track, album art URL, device, progress, shuffle/repeat state, is-playing.
3. **`play` / `pause` / `next` / `previous` / `seek <ms>` / `volume <0-100>`** — Premium-gated; surface a clean error on `403 PREMIUM_REQUIRED`.
4. **`search <query> --type tracks|albums|artists|playlists|shows|episodes --limit N`** — defaults to `tracks`, supports comma-separated multi-type.
5. **`playlist list|get|create|add|remove|reorder`** — full CRUD; snapshot-aware (warn if snapshot changed between read and write).
6. **`saved-tracks list|add|remove`** plus `saved-albums`, `saved-shows`, `followed-artists` parallel subtrees.
7. **`top tracks|artists --range short|medium|long`** — snapshots into SQLite on each call so drift queries become possible.
8. **`recently-played`** — surfaces API's 50-cap; recommends running `sync` periodically to extend local history.
9. **`devices list` / `devices transfer <id>`** — for moving playback between speakers.
10. **`sync`** — populates `saved_tracks`, `playlists` + `playlist_tracks` (snapshot-keyed re-fetch), `top_*_snapshot`, `play_history`. Idempotent.

Deferred until a legacy-quota path is confirmed: `audio-features`, `audio-analysis`, `recommendations`. Generate the commands but have them check feature availability at runtime and emit a clear deprecation notice.
