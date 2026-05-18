# Spotify CLI — Absorb Manifest

## Sources scanned

Step 1.5a swept the leading wrappers, terminal clients, and MCP servers for the Spotify Web API to establish the absorbed-feature floor. Sources surveyed:

- **spotipy** (Python SDK, ~7000 stars, ~300 methods) — gold-standard endpoint coverage; the de-facto reference for what a "complete" Spotify client looks like in code.
- **spotify-tui / `spt`** (Rust TUI + CLI, ~17000 stars) — closest direct competitor in CLI shape. Ships `spt playback`, `spt list`, `spt query` subtrees.
- **ncspot** (Rust ncurses client) — playback-controller patterns; not a one-shot CLI but the keybindings map cleanly onto commands.
- **shpotify** (bash + AppleScript) — controls the *desktop app*, not the Web API. Different mental model; informs the "kill local-app dependence" framing.
- **spotify-cli-linux** (Python, dbus + MPRIS) — distro-bound; useful only as a naming-convention reference.
- **spotify-web-api-node** (JavaScript SDK, ~2700 stars) — endpoint-coverage parity check.
- **rspotify** (Rust SDK) — Rust client used by spotify-tui; another endpoint completeness benchmark.
- **marcelmarais/spotify-mcp-server** (TypeScript, lean ~10-tool MCP server) — the MCP tool-shape floor.
- **PeterAkande/spotify_mcp** (Python FastMCP, 36 tools) — broader MCP coverage with named tools mapped to spotipy methods.
- **varunneal/spotify-mcp** and **tylerpina/spotify-mcp** — corroborating MCP tool-naming conventions.
- **Spotify Web API** itself — authoritative endpoint list via the community-fixed OpenAPI mirror (`sonallux/spotify-web-api`).

## Absorbed (match or beat the field)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Current playback state | spotipy.current_playback() | `now-playing [--json]` | `--select` over deeply-nested fields; `--watch <Ns>` polls and logs to `play_history`; offline replay of last-seen state when offline |
| 2 | Start / resume playback | spotipy.start_playback() | `play [--device <id>] [--uri <track-uri>] [--context-uri <playlist/album-uri>] [--offset <n>] [--position-ms <ms>]` | Premium-gated; clean `403 PREMIUM_REQUIRED` error; `cliutil.IsVerifyEnv()` short-circuit so verifier doesn't actually start audio |
| 3 | Pause playback | spotipy.pause_playback() | `pause [--device <id>]` | Premium-gated; idempotent (already-paused returns 0); verifier short-circuit |
| 4 | Skip to next track | spotipy.next_track() | `next [--device <id>]` | Premium-gated; verifier short-circuit |
| 5 | Skip to previous track | spotipy.previous_track() | `previous [--device <id>]` | Premium-gated; verifier short-circuit |
| 6 | Seek to position | spotipy.seek_track() | `seek <position-ms> [--device <id>]` | Validates 0 <= ms <= track-duration before the request; Premium-gated |
| 7 | Set volume | spotipy.volume() | `volume <0-100> [--device <id>]` | Range-checked at parse; Premium-gated |
| 8 | Toggle shuffle | spotipy.shuffle() | `shuffle on\|off [--device <id>]` | Typed const for state; Premium-gated |
| 9 | Set repeat mode | spotipy.repeat() | `repeat track\|context\|off [--device <id>]` | Typed enum; Premium-gated |
| 10 | Add to playback queue | spotipy.add_to_queue() | `queue add <uri> [--device <id>]` | Accepts track or episode URI; Premium-gated |
| 11 | List playback queue | marcelmarais/spotify-mcp-server `getQueue` | `queue list [--json]` | `--select` over `currently_playing` + `queue[].name` |
| 12 | Transfer playback | spotipy.transfer_playback() | `transfer <device-id> [--play]` | `--play` starts/keeps playback after transfer; Premium-gated |
| 13 | List devices | spotipy.devices() | `devices list [--json]` | Caches into `devices_seen` table; `--select` over `name,type,is_active` |
| 14 | Search catalog | spotipy.search() | `search <query> --type tracks\|albums\|artists\|playlists\|shows\|episodes [--market <code>] [--limit N] [--offset N]` | Multi-type via comma-separated `--type`; `--select` over `results.tracks.items.name` etc. |
| 15 | List saved tracks | spotipy.current_user_saved_tracks() | `saved-tracks list [--limit N] [--offset N] [--market <code>]` | Paginates transparently up to `--limit`; caches into `saved_tracks` |
| 16 | Save tracks | spotipy.current_user_saved_tracks_add() | `saved-tracks add <ids...>` | Accepts URIs or bare IDs; batches at 50 per request |
| 17 | Remove saved tracks | spotipy.current_user_saved_tracks_delete() | `saved-tracks remove <ids...>` | Same batching as add |
| 18 | Check saved tracks | spotipy.current_user_saved_tracks_contains() | `saved-tracks check <ids...> [--json]` | Returns `{id: bool}` map; batches at 50 |
| 19 | List saved albums | spotipy.current_user_saved_albums() | `saved-albums list [--limit N] [--market <code>]` | Caches into `saved_albums` |
| 20 | Save albums | spotipy.current_user_saved_albums_add() | `saved-albums add <ids...>` | Batched |
| 21 | Remove saved albums | spotipy.current_user_saved_albums_delete() | `saved-albums remove <ids...>` | Batched |
| 22 | Check saved albums | spotipy.current_user_saved_albums_contains() | `saved-albums check <ids...> [--json]` | Batched |
| 23 | List saved shows | spotipy.current_user_saved_shows() | `saved-shows list [--limit N]` | Caches into `saved_shows` |
| 24 | Save shows | spotipy.current_user_saved_shows_add() | `saved-shows add <ids...>` | Batched |
| 25 | Remove saved shows | spotipy.current_user_saved_shows_delete() | `saved-shows remove <ids...>` | Batched |
| 26 | List saved episodes | spotipy.current_user_saved_episodes() | `saved-episodes list [--limit N]` | |
| 27 | Save episodes | spotipy.current_user_saved_episodes_add() | `saved-episodes add <ids...>` | Batched |
| 28 | Remove saved episodes | spotipy.current_user_saved_episodes_delete() | `saved-episodes remove <ids...>` | Batched |
| 29 | List followed artists | spotipy.current_user_followed_artists() | `followed-artists list [--limit N]` | Synthesizes `followed_at` into `followed_artists` table on first observation |
| 30 | Follow artists | spotipy.user_follow_artists() | `followed-artists follow <ids...>` | Batched |
| 31 | Unfollow artists | spotipy.user_unfollow_artists() | `followed-artists unfollow <ids...>` | Batched |
| 32 | Check followed artists | spotipy.current_user_following_artists() | `followed-artists check <ids...> [--json]` | Batched |
| 33 | List current user's playlists | spotipy.current_user_playlists() | `playlists list [--limit N] [--offset N]` | Caches into `playlists`; tracks `snapshot_id` |
| 34 | Get one playlist | spotipy.playlist() | `playlists get <id> [--market <code>] [--fields <list>]` | `--select` over deep playlist structure |
| 35 | Create playlist | spotipy.user_playlist_create() | `playlists create --name <n> [--public\|--private] [--collaborative] [--description <text>]` | Defaults to private; returns new playlist URI on stdout |
| 36 | Update playlist details | spotipy.playlist_change_details() | `playlists update <id> [--name <n>] [--description <t>] [--public\|--private] [--collaborative]` | Only sends fields the user supplied |
| 37 | List playlist tracks | spotipy.playlist_items() | `playlists tracks <id> [--limit N] [--offset N] [--market <code>]` | Caches into `playlist_tracks` keyed by `snapshot_id`; `--select` over `items[].track.name,added_at` |
| 38 | Add tracks to playlist | spotipy.playlist_add_items() | `playlists add-tracks <playlist-id> <track-uris...> [--position N]` | Batched at 100/req; snapshot-aware (warns if snapshot changed since last read) |
| 39 | Remove tracks from playlist | spotipy.playlist_remove_all_occurrences_of_items() | `playlists remove-tracks <playlist-id> <track-uris...>` | Snapshot-aware |
| 40 | Reorder playlist | spotipy.playlist_reorder_items() | `playlists reorder <playlist-id> --range-start <n> --range-length <n> --insert-before <n>` | Snapshot-aware |
| 41 | Replace playlist tracks | spotipy.playlist_replace_items() | `playlists replace-tracks <playlist-id> <track-uris...>` | Destructive; requires `--confirm` |
| 42 | Get playlist cover image | spotipy.playlist_cover_image() | `playlists cover-image get <id> [--json]` | Returns URL(s) |
| 43 | Upload playlist cover | spotipy.playlist_upload_cover_image() | `playlists cover-image upload <id> --file <path>` | Base64-encodes JPEG; size validation client-side |
| 44 | Top tracks | spotipy.current_user_top_tracks() | `top tracks --range short\|medium\|long [--limit N] [--offset N]` | Snapshots into `top_tracks_snapshot` on every call so drift queries become possible |
| 45 | Top artists | spotipy.current_user_top_artists() | `top artists --range short\|medium\|long [--limit N] [--offset N]` | Snapshots into `top_artists_snapshot` |
| 46 | Recently played | spotipy.current_user_recently_played() | `recently-played [--limit N] [--after <unix-ms>] [--before <unix-ms>]` | Caches into `play_history`; surfaces API's 50-cap and suggests `sync` to extend |
| 47 | Get artist | spotipy.artist() | `artists get <id> [--json]` | Caches into `artists` + `artist_genres` |
| 48 | Artist albums | spotipy.artist_albums() | `artists albums <id> [--include-groups album,single,appears_on,compilation] [--market <code>] [--limit N]` | Filter via repeatable `--include-groups` |
| 49 | Artist top tracks | spotipy.artist_top_tracks() | `artists top-tracks <id> --market <code>` | Market required (API requirement) |
| 50 | Get album | spotipy.album() | `albums get <id> [--market <code>]` | Caches into `albums` + `album_artists` |
| 51 | Album tracks | spotipy.album_tracks() | `albums tracks <id> [--limit N] [--market <code>]` | Paginates transparently |
| 52 | Several albums | spotipy.albums() | `albums several <ids...> [--market <code>]` | Batched at 20/req |
| 53 | Get track | spotipy.track() | `tracks get <id> [--market <code>]` | Caches into `tracks` |
| 54 | Several tracks | spotipy.tracks() | `tracks several <ids...> [--market <code>]` | Batched at 50/req |
| 55 | New releases | spotipy.new_releases() | `browse new-releases [--country <code>] [--limit N]` | |
| 56 | Browse categories | spotipy.categories() | `browse categories [--country <code>] [--locale <code>] [--limit N]` | Categories list still works post-2024-11-27 (only the per-category playlists endpoint is gone) |
| 57 | Get one category | spotipy.category() | `browse categories get <id> [--country <code>] [--locale <code>]` | |
| 58 | Available markets | spotipy.available_markets() | `markets [--json]` | Returns ISO-3166-1 alpha-2 list |
| 59 | Available genre seeds | spotipy.recommendation_genre_seeds() | `genre-seeds [--json]` | Static-ish list; useful even though `recommendations` is deprecated for new apps |
| 60 | Current user profile | spotipy.current_user() | `me [--json]` | `--select` over `display_name,product,country,followers.total` |
| 61 | Public user profile | spotipy.user() | `users get <id> [--json]` | |
| 62 | Get show | spotipy.show() | `shows get <id> [--market <code>]` | Caches into `shows` |
| 63 | Show episodes | spotipy.show_episodes() | `shows episodes <id> [--limit N] [--market <code>]` | |
| 64 | Several shows | spotipy.shows() | `shows several <ids...> [--market <code>]` | Batched at 50/req |
| 65 | Get episode | spotipy.episode() | `episodes get <id> [--market <code>]` | Caches into `episodes` |
| 66 | Several episodes | spotipy.episodes() | `episodes several <ids...> [--market <code>]` | Batched at 50/req |
| 67 | OAuth login | spotipy.SpotifyOAuth / spotipy.SpotifyPKCE | `auth login [--secret] [--port <n>] [--scopes <list>]` | PKCE default; `--secret` (or presence of `SPOTIFY_SECRET`) auto-switches to Authorization Code; loopback `127.0.0.1:<port>/callback` |
| 68 | Force token refresh | spotipy refresh_access_token() | `auth refresh` | Rotates refresh_token and persists |
| 69 | Token status | (none — novel surface, MCP servers hide this) | `auth status [--json]` | Shows expiry, scopes, flow type (pkce/secret), client_id (last-4 only) |
| 70 | Logout | spotipy cache_handler.save_token_to_cache(None) | `auth logout` | Removes `~/.config/spotify-pp-cli/token.json` |

## Stubs (shipping but honest)

Each stub below is a real command in the Cobra tree so the surface is discoverable, but it exits `0` with a structured deprecation payload instead of attempting a request that we know returns `403`/`404` for apps created after 2024-11-27. Each stub respects a future `--legacy-app` opt-in flag that lets users with grandfathered extended-quota apps attempt the real call.

Common stub payload shape (with `--json`):
```json
{
  "status": "stub_deprecated",
  "reason": "Spotify removed access to this endpoint for apps created after 2024-11-27. The user's app is a new app and will receive 403/404 on this endpoint.",
  "next_action": "If you have a legacy extended-quota app, retry with --legacy-app to attempt the real request.",
  "spotify_blog_url": "https://developer.spotify.com/blog/2024-11-27-changes-to-the-web-api"
}
```

Stubs:

- `audio-features <id>` — stub. Reason: 2024-11-27 deprecation. `--legacy-app` opt-in attempts the real `GET /audio-features/{id}` call.
- `audio-features several <ids...>` — stub. Same as above for `GET /audio-features?ids=`.
- `audio-analysis <id>` — stub. Reason: 2024-11-27 deprecation. `--legacy-app` opt-in attempts the real `GET /audio-analysis/{id}` call.
- `recommendations [--seed-tracks <ids>] [--seed-artists <ids>] [--seed-genres <list>] [--target-*]` — stub. Reason: 2024-11-27 deprecation. The seed/target flag surface is preserved so users on legacy apps (with `--legacy-app`) get the full functionality.
- `artists related <id>` — stub. Reason: 2024-11-27 deprecation removed `GET /artists/{id}/related-artists`. `--legacy-app` opt-in attempts the real call.
- `playlists featured [--country <code>] [--locale <code>] [--timestamp <iso>] [--limit N]` — stub. Reason: 2024-11-27 deprecation removed `GET /browse/featured-playlists`. `--legacy-app` opt-in attempts the real call.
- `browse categories playlists <category-id> [--country <code>] [--limit N]` — stub. Reason: 2024-11-27 deprecation removed `GET /browse/categories/{id}/playlists`. The parent `browse categories` list still works and is shipped non-stub (row 56). `--legacy-app` opt-in attempts the real call.

Each stub is annotated `cmd.Annotations["mcp:read-only"] = "true"` so an MCP host knows the stub itself is safe; the `--legacy-app` real call inherits the read-only nature of the underlying endpoint (all of these are `GET`s).

## Transcendence (novel — only possible with our approach)

| # | Feature | Command | Score | Persona | Why only we can do this |
|---|---------|---------|-------|---------|--------------------------|
| T1 | Snapshot-aware playlist diff | `playlists diff <id> [--against-snapshot <sid>]` | 9/10 | Naomi (music journalist) | Reads current playlist + tracks, compares to `playlist_tracks` rows keyed by prior `snapshot_id`, emits add/remove/reorder set. Spotify's snapshot_id is a defining surface no UI exposes — competitors don't store snapshots, so they can't diff. |
| T2 | ISRC-aware playlist dedupe | `playlists dedupe <id> [--by isrc\|track-id\|title-artist] [--apply]` | 9/10 | Marcus (bedroom DJ) | Joins local `playlist_tracks` to `tracks.isrc`, groups, reports duplicates; `--apply` calls Spotify's remove-tracks with snapshot guard. Catches the album/single/EP/deluxe-reissue dupe class that ID-based dedupe misses. |
| T3 | Cross-playlist merge with dedupe | `playlists merge <src-1> <src-2> [...] --into <dest> [--dedupe-by isrc] [--order keep\|shuffle\|by-date]` | 8/10 | Marcus | Reads source rows from local `playlist_tracks`, dedupes locally, POSTs to `<dest>` in 100-track chunks. Spotify ships no merge primitive; users currently script this against `spotipy`. |
| T4 | Top-tracks rotation drift | `top drift --range short\|medium\|long --since <date>` | 9/10 | Naomi | Two-row diff over `top_tracks_snapshot` filtered by `time_range` + `captured_at`; emits risen/fallen/stable cohorts. Spotify's top-tracks endpoint returns "current" only — without local snapshotting there is no drift answer. The product thesis's headline transcendence move. |
| T5 | Release-Radar replacement | `releases since <date> [--from followed]` | 8/10 | Naomi | Iterates `followed_artists`, calls `GET /artists/{id}/albums?include_groups=album,single&min_release_date=<date>` per artist, sorts by release date desc. Replaces Spotify's algorithmic Release Radar (deprecated for new apps on 2024-11-27) with a deterministic feed the user actually controls. |
| T6 | Cross-entity track lookup | `tracks where <id-or-uri>` | 7/10 | Naomi + Marcus | Single track_id joins against `playlist_tracks`, `saved_tracks`, `play_history`, `top_tracks_snapshot` — pure local SQL. Spotify's app has no "where is this track in my library" view; users currently dupe-add tracks because they can't tell. |
| T7 | Play-history by listening context | `play history --by context [--since <window>]` | 7/10 | Marcus + Naomi | Aggregates `play_history.context_uri`, joins to playlists/albums/artists for display names, ranks by play count and duration within window. Answers "which of my set lists is actually getting played" — Spotify's API exposes context but no UI surface aggregates it. |
| T8 | Agent-friendly queue-from-saved | `queue from-saved [--limit N] [--artist <id>] [--playlist <id>]` | 7/10 | Priya (agent-loop operator) | Selects N rows from local `saved_tracks` (optional filters), POSTs each URI to `/me/player/queue`; surfaces `403 PREMIUM_REQUIRED` cleanly. Lets agents queue "from my chillout saved tracks" by intent instead of by URI. |
| T9 | Genre-walked artist discovery | `discover artists [--seed top\|saved\|followed] [--limit N] [--exclude-followed]` | 9/10 | All personas (user-requested) | Walks the `genres` field on my top/saved/followed artists, calls `/search?type=artist&q=genre:"<g>"` for each unique genre, ranks unfollowed artists by popularity × genre-match-score. Replaces Spotify's deprecated `/recommendations` for the "artists I might like" question — uses search + genre tags, which still work for new apps. |
| T10 | Co-occurrence discovery via public playlists | `discover via-playlists <seed-artist-id> [--depth 1] [--min-cooccurrence 3] [--limit N]` | 8/10 | Naomi + Marcus (user-requested) | Searches public playlists containing the seed artist (`/search?type=playlist&q=<artist-name>`), fetches each playlist's tracks, counts co-occurring artists across the corpus, ranks by frequency. Spotify's `/related-artists` is deprecated for new apps; public playlists become the graph substrate. |
| T11 | Artist deep-dive (gap finder) | `discover artist-gaps <artist-id> [--show all\|saved\|unsaved] [--include-groups album,single,compilation]` | 7/10 | Naomi (user-requested) | Lists artist's full discography chronologically from `/artists/{id}/albums`, joins to local `saved_albums` + `playlist_tracks` to mark each as saved or unsaved. Answers "I love this artist — what have I missed?" Spotify's app shows discography but not your coverage of it. |
| T12 | New releases in your genres (Release Radar replacement, wider) | `discover new-releases [--seed-from top\|followed] [--days 30] [--exclude-followed] [--limit N]` | 8/10 | Naomi + all (user-requested) | Pulls global `/browse/new-releases`, fetches each release artist's genres, keeps only releases whose artists share a genre with my top/followed artists, optionally excludes artists I already follow (T5's surface). Surfaces adjacent discovery — "new releases from artists like the ones I follow, but not them." |
| T13 | Play to device by friendly name | `play-on <device-name> [--uris <json>] [--context-uri <uri>]` | 8/10 | Priya (agent-loop) + all | Resolves a friendly name (e.g. "family room") against both the live `/me/player/devices` list and the locally cached `devices_seen` table, then starts playback. Spotify's API only takes opaque device IDs, so every other client requires list-then-copy/paste. Cached entries let agents reference devices that aren't currently online with a typed "wake the device first" hint instead of a raw 404. Side-effect: every call refreshes `devices_seen` from the live list. |

See `2026-05-12-184940-novel-features-brainstorm.md` for the full customer model, candidate list, kill verdicts, and reasoning.

## Live dogfood findings (2026-05-12 new app)

Live testing against a brand-new Spotify dashboard app (registered today) revealed the 2024-11-27 deprecation hits more endpoints than Spotify's blog post lists. Anyone reading this manifest with a "legacy" app (registered before 2024-11-27) may see fuller functionality. The product is shipping with these caveats documented:

- **`artist.genres` and `artist.popularity` are null on all artist responses for new apps.** Not on Spotify's announced deprecation list. Structurally blocks any feature that walks an artist's genre tags from a fresh API call. **T9 (`discover artists`)** returns `"no genres in seed source"` on new apps; works for legacy apps. **T12 (`discover new-releases`)** can't filter by genre overlap; the underlying `/browse/new-releases` also returns 403 (also undocumented in the blog post).
- **`/playlists/{id}/tracks` returns `{track: null}` items for Spotify-owned editorial/algorithmic playlists on new apps.** Means **T10 (`discover via-playlists`)** typically returns empty results when `/search?type=playlist` happens to return only Spotify-curated playlists. Still works when user-created playlists rank in the search results.
- **`/artists/{id}/albums` rejects `limit` > ~10 with HTTP 400 "Invalid limit" on new apps,** despite the OpenAPI spec and docs declaring max=50. **T11 (`discover artist-gaps`)** uses `limit=10` with cursor pagination via the response's `next` URL — works fine, just costs more round trips.
- **`/me/*` endpoints behave per spec** (top tracks, top artists, recently-played, saved, followed all paginate at limit=50 as documented). The post-2024-11-27 restrictions appear scoped to third-party content endpoints.

T9, T10, and T12 are shipped as runnable commands rather than stubs because they work in mock-mode + verify, dogfood gracefully (T9 returns a typed `no genres in seed source` hint, T12 surfaces a 403 with explanatory hint), and become functional automatically on legacy apps or once Spotify re-opens the endpoints.
