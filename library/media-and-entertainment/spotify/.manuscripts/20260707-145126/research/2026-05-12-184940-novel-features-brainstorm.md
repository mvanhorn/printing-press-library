# Spotify CLI — Novel Features Brainstorm

(Phase 1.5c.5 subagent output. Full audit trail per
`references/novel-features-subagent.md`. The `### Survivors` table appears
in the absorb manifest; the rest is preserved for retro/dogfood.)

## Customer model

### Persona A: Naomi, music journalist
Builds themed playlists for articles + podcast. ~140 owned playlists, ~300 followed artists. Sunday top-50 worksheet, Friday release-Radar replacement scan, archives-before-retitling ritual. Pain: Spotify treats playlists as ephemeral — snapshot_id exists in the API but the app shows nothing, can't diff, can't see what algorithmic refreshes removed from collab playlists, top-50 evaporates every Sunday refresh.

### Persona B: Marcus, bedroom DJ
~25 themed set playlists, ~3,400 saved tracks, ~70 saved albums. Monday set review → Wednesday split scratchpad into themed sets → Thursday re-sequence → Friday transfer to HomePod. Pain: reordering 50 tracks in desktop UI is hell, no dedupe across saved-tracks (album/single/EP reissue dupes), can't script-merge playlists.

### Persona C: Priya, agent-loop operator
Claude Code user. SPOTIFY_CLIENT_ID + SPOTIFY_SECRET already in dotfiles. Wants Claude to drive Spotify by intent ("queue something more upbeat") not by URI. Existing MCP servers either too thin or too chatty. Doesn't want to memorize URIs or re-OAuth.

## Survivors and kills

### Survivors (8 features)

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Snapshot-aware playlist diff | `playlist diff <id> [--against-snapshot <sid>]` | 9/10 | Naomi | Reads playlist + tracks, compares to `playlist_tracks` rows keyed by prior `snapshot_id`, emits add/remove/reorder set. |
| 2 | ISRC-aware playlist dedupe | `playlist dedupe <id> [--by isrc] [--apply]` | 9/10 | Marcus | Joins `playlist_tracks` to `tracks.isrc`, groups, reports dupes; `--apply` calls Spotify DELETE with snapshot guard. |
| 3 | Cross-playlist merge with dedupe | `playlist merge <src1> <src2>... --into <dest> [--dedupe-by isrc] [--order keep\|shuffle\|by-date]` | 8/10 | Marcus | Reads sources from local store, dedupes locally, POSTs to dest in 100-track chunks. |
| 4 | Top-tracks rotation drift | `top drift --range short\|medium\|long --since <date>` | 9/10 | Naomi | Two-row diff over `top_tracks_snapshot` filtered by `time_range` + `captured_at`; emits risen/fallen/stable cohorts. |
| 5 | Release-Radar replacement | `releases since <date> [--from followed]` | 8/10 | Naomi | Iterates `followed_artists`, calls `GET /artists/{id}/albums?include_groups=album,single&min_release_date=<date>`, sorts by release date desc. |
| 6 | Cross-entity track lookup | `track where <id-or-uri>` | 7/10 | Naomi + Marcus | One track_id joins against `playlist_tracks`, `saved_tracks`, `play_history`, `top_tracks_snapshot` — pure local SQL. |
| 7 | Play-history by context | `play history --by context [--since <window>]` | 7/10 | Marcus + Naomi | Aggregates `play_history.context_uri`, joins to playlists/albums/artists for display names, ranks by play count + duration. |
| 8 | Agent-friendly queue-from-saved | `queue from-saved [--limit N] [--artist <id>] [--playlist <id>]` | 7/10 | Priya | Selects N rows from local `saved_tracks` (optional filters), POSTs each URI to `/me/player/queue`; surfaces 403 PREMIUM_REQUIRED cleanly. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| library-growth | Weekly-use weak (Marcus checks accretion monthly); thin GROUP BY wrapper | top drift |
| followed-new-this-period | Synthetic `followed_at` only true post-install; fails Verifiability | releases since |
| podcast-new-episodes | No persona centers podcasts; weekly-use signal absent in brief | releases since |
| playlist-export | Thin wrapper over local rows + Markdown formatting; `--json` + jq covers it | track where |
| library-backup | Sync's SQLite file IS the backup; the command is ceremonial | sync (already absorbed) |
| device-last-used | Users have ≤5 devices and remember them; no weekly pull | devices list + transfer (already absorbed) |
| saved-tracks-by-artist | Quarterly use at best; single-table GROUP BY borderline wrapper | track where + artist coverage (also killed) |
| playlist-snapshot | Sync already snapshots on every run; explicit snapshot command duplicates | playlist diff |
| top-rotation | Overlaps top drift — fold as `--mode sticky` flag | top drift |
| artist-coverage | Niche (quarterly column research); API-budget heavy per call | track where |
