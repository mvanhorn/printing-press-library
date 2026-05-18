---
name: pp-spotify
description: "An agent-native Spotify CLI with a SQLite-backed local library that lets you ask listening-drift questions no other... Trigger phrases: `play the next track`, `what am I listening to`, `show my saved tracks`, `create a spotify playlist`, `what are my top artists this month`, `use spotify-pp-cli`, `spotify`."
author: "Rob Zehner"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - spotify-pp-cli
---

# Spotify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `spotify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install spotify --cli-only
   ```
2. Verify: `spotify-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Every Spotify Web API endpoint is reachable from one static binary with --json and --select everywhere and a built-in MCP server, so an agent can drive playback, search, and library curation with the same surface a human uses. A local SQLite store captures top-tracks/top-artists snapshots, snapshot-keyed playlist tracks, and extended play history beyond Spotify's 50-event cap, enabling one-shot queries like 'which tracks dropped out of medium-term but stayed in long-term' that no existing CLI can answer. Endpoints Spotify removed for new apps on 2024-11-27 ship as honest deprecation-aware stubs with a --legacy-app opt-in.

## When to Use This CLI

Reach for spotify-pp-cli when an agent needs to read or steer a user's Spotify session, curate playlists, or answer questions about their listening history. It is the right choice over generic HTTP calls when the task benefits from the local SQLite store (drift queries, snapshot-aware playlist diffs, extended play history past the 50-event cap) or from the deprecation-aware stubs that route around endpoints Spotify removed for new apps. Prefer it over spotify-tui when the workflow is one-shot rather than interactive, and over spotipy when the host is an agent rather than a Python script.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Playlist maintenance
- **`playlists diff`** — Compare a playlist's current state against any prior snapshot to see exactly which tracks were added, removed, or reordered.

  _Reach for this when a user asks 'what changed in this playlist?' or wants to undo an algorithmic refresh on a collab playlist._

  ```bash
  spotify-pp-cli playlists diff 37i9dQZF1DXcBWIGoYBM5M --against-snapshot abc123 --agent
  ```
- **`playlists dedupe`** — Find duplicate tracks in a playlist by ISRC (catches the album/single/EP/deluxe-reissue dupe class), report by default, --apply to remove.

  _Use this when a curator's playlist has the same recording on three different albums; Spotify's app treats them as distinct._

  ```bash
  spotify-pp-cli playlists dedupe 37i9dQZF1DXcBWIGoYBM5M --by isrc --agent
  ```
- **`playlists merge`** — Combine multiple playlists into one with built-in dedupe and ordering controls.

  _Reach for this when a DJ wants to consolidate retired set playlists into a deep-archive without manually de-duplicating._

  ```bash
  spotify-pp-cli playlists merge 37i9dQZF1DXcBWIGoYBM5M 37i9dQZF1DX0XUsuxWHRQd 6rqhFgbbKwnb9MLmUQDhG6 --into <new-playlist-id> --dedupe-by isrc --order by-date
  ```

### Listening drift
- **`top drift`** — Compare two top-tracks snapshots and show who rose, fell, or stayed stable across a time window.

  _Use this when a user asks 'which artists fell off my top-50 between Q1 and Q4?' or wants to track their own listening identity over time._

  ```bash
  spotify-pp-cli top drift --range medium --since 2026-01-01 --agent --select risen,fallen,stable
  ```
- **`releases since`** — List new albums and singles released since a date by artists you follow, sorted newest first.

  _Reach for this when a user asks 'what's new from artists I follow' and the deprecated Release Radar is no longer reachable._

  ```bash
  spotify-pp-cli releases since 2026-05-01 --from followed --agent --select name,artists.0.name,release_date
  ```
- **`play history`** — Bucket your recent play history by the playlist or album that drove each play, ranked by play count and total duration.

  _Reach for this when a DJ asks 'which of my set lists is actually getting played this week' or when a journalist wants column data on listening patterns._

  ```bash
  spotify-pp-cli play history --by context --since 7d --agent --select context_name,play_count,total_duration_min
  ```

### Cross-collection lookup
- **`tracks where`** — For a given track, find every place it appears in your data: which playlists, whether saved, last played, and on which devices.

  _Use this before adding a track to a playlist to avoid duping it, or to answer 'have I played this before' questions._

  ```bash
  spotify-pp-cli tracks where 4uLU6hMCjMI75M1A2tKUQC --agent
  ```

### Agent-native playback
- **`queue from-saved`** — Pick the N most recently saved tracks from your library and queue them in one command.

  _Use this when an agent should "queue 10 more from my saved tracks" without resorting to URI manipulation. Per-artist and per-playlist filters are not yet implemented; the saved_tracks schema does not carry artist or playlist linkage and adding it requires a join against a tracks-cache table._

  ```bash
  spotify-pp-cli queue from-saved --limit 10
  ```
- **`play-on`** — Start playback on a device referenced by friendly name (e.g. "family room") instead of opaque device IDs. Resolves against both the live `/me/player/devices` list and the locally cached `devices_seen` table populated by `sync-extras`.

  _Reach for this when an agent says "play X on the kitchen speaker" or "send this to my office" — Spotify's API only takes opaque device IDs, so users normally have to list devices and copy/paste; play-on closes that loop with a name index. Cached entries let you reference devices that aren't currently online (with a typed "wake the device first" hint instead of a raw 404). Name matching is case-insensitive, exact > prefix > substring; ambiguous matches list the candidates._

  ```bash
  spotify-pp-cli play-on "family room" --uris '["spotify:track:0nys6GusuHnjSYLW0PYYb7"]'
  spotify-pp-cli play-on iphone --context-uri spotify:album:1ER3B6zev5JEAaqhnyyfbf
  ```

### Music discovery
- **`discover artists`** — Find artists you don't follow yet who match the genres of your top, saved, or followed artists, ranked by popularity within each genre.

  _Reach for this when the user asks 'find me new artists like the ones I already listen to' and the deprecated recommendations endpoint isn't an option._

  ```bash
  spotify-pp-cli discover artists --seed top --exclude-followed --limit 25 --agent --select name,genres,popularity,top_track.name
  ```
- **`discover via-playlists`** — Find artists frequently co-curated with a seed artist by searching public playlists that contain them and ranking other artists by co-occurrence count.

  _Use this for the 'who sounds like X' question Spotify used to answer with related-artists; curator-driven co-occurrence is often a better signal anyway._

  ```bash
  spotify-pp-cli discover via-playlists 0OdUWJ0sBjDrqHygGUXeCF --min-cooccurrence 5 --limit 20 --agent
  ```
- **`discover artist-gaps`** — For an artist, show their full discography chronologically with each album marked as saved or unsaved against your library.

  _Reach for this when a user says 'I love this artist, what have I missed?' — surfaces the gap in their own collection._

  ```bash
  spotify-pp-cli discover artist-gaps 0OdUWJ0sBjDrqHygGUXeCF --show unsaved --include-groups album,single
  ```
- **`discover new-releases`** — Filter Spotify's global new-releases feed down to releases whose artists share a genre with your top or followed artists; optionally exclude artists you already follow.

  _Use this for 'what new music came out this week in genres I actually listen to' — broader than just-from-followed-artists (T5) since it surfaces adjacent artists too._

  ```bash
  spotify-pp-cli discover new-releases --seed-from top --days 14 --exclude-followed --agent --select name,artists.0.name,release_date,genres
  ```

## Command Reference

**albums** — Manage albums

- `spotify-pp-cli albums get-an` — Get Spotify catalog information for a single album.
- `spotify-pp-cli albums get-multiple` — Get Spotify catalog information for multiple albums identified by their Spotify IDs.

**artists** — Manage artists

- `spotify-pp-cli artists get-an` — Get Spotify catalog information for a single artist identified by their unique Spotify ID.
- `spotify-pp-cli artists get-multiple` — Get Spotify catalog information for several artists based on their Spotify IDs.

**audio-analysis** — Manage audio analysis

- `spotify-pp-cli audio-analysis <id>` — Get a low-level audio analysis for a track in the Spotify catalog. The audio analysis describes the track’s...

**audio-features** — Manage audio features

- `spotify-pp-cli audio-features get` — Get audio feature information for a single track identified by its unique Spotify ID.
- `spotify-pp-cli audio-features get-several` — Get audio features for multiple tracks based on their Spotify IDs.

**audiobooks** — Manage audiobooks

- `spotify-pp-cli audiobooks get-an` — Get Spotify catalog information for a single audiobook. Audiobooks are only available within the US, UK, Canada,...
- `spotify-pp-cli audiobooks get-multiple` — Get Spotify catalog information for several audiobooks identified by their Spotify IDs. Audiobooks are only...

**browse** — Manage browse

- `spotify-pp-cli browse get-a-categories-playlists` — Get a list of Spotify playlists tagged with a particular category.
- `spotify-pp-cli browse get-a-category` — Get a single category used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- `spotify-pp-cli browse get-categories` — Get a list of categories used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- `spotify-pp-cli browse get-featured-playlists` — Get a list of Spotify featured playlists (shown, for example, on a Spotify player's 'Browse' tab).
- `spotify-pp-cli browse get-new-releases` — Get a list of new album releases featured in Spotify (shown, for example, on a Spotify player’s “Browse” tab).

**chapters** — Manage chapters

- `spotify-pp-cli chapters get-a` — Get Spotify catalog information for a single audiobook chapter. Chapters are only available within the US, UK,...
- `spotify-pp-cli chapters get-several` — Get Spotify catalog information for several audiobook chapters identified by their Spotify IDs. Chapters are only...

**episodes** — Manage episodes

- `spotify-pp-cli episodes get-an` — Get Spotify catalog information for a single episode identified by its unique Spotify ID.
- `spotify-pp-cli episodes get-multiple` — Get Spotify catalog information for several episodes based on their Spotify IDs.

**markets** — Manage markets

- `spotify-pp-cli markets` — Get the list of markets where Spotify is available.

**me** — Manage me

- `spotify-pp-cli me add-to-queue` — Add an item to be played next in the user's current playback queue. This API only works for users who have Spotify...
- `spotify-pp-cli me check-current-user-follows` — Check to see if the current user is following one or more artists or other Spotify users. **Note:** This endpoint is...
- `spotify-pp-cli me check-library-contains` — Check if one or more items are already saved in the current user's library. Accepts Spotify URIs for tracks, albums,...
- `spotify-pp-cli me check-users-saved-albums` — Check if one or more albums is already saved in the current Spotify user's 'Your Music' library. **Note:** This...
- `spotify-pp-cli me check-users-saved-audiobooks` — Check if one or more audiobooks are already saved in the current Spotify user's library. **Note:** This endpoint is...
- `spotify-pp-cli me check-users-saved-episodes` — Check if one or more episodes is already saved in the current Spotify user's 'Your Episodes' library. **Note:** This...
- `spotify-pp-cli me check-users-saved-shows` — Check if one or more shows is already saved in the current Spotify user's library. **Note:** This endpoint is...
- `spotify-pp-cli me check-users-saved-tracks` — Check if one or more tracks is already saved in the current Spotify user's 'Your Music' library. **Note:** This...
- `spotify-pp-cli me create-playlist` — Create a playlist for the current Spotify user. (The playlist will be empty until you [add...
- `spotify-pp-cli me follow-artists-users` — Add the current user as a follower of one or more artists or other Spotify users. **Note:** This endpoint is...
- `spotify-pp-cli me get-a-list-of-current-users-playlists` — Get a list of the playlists owned or followed by the current Spotify user.
- `spotify-pp-cli me get-a-users-available-devices` — Get information about a user’s available Spotify Connect devices. Some device models are not supported and will...
- `spotify-pp-cli me get-current-users-profile` — Get detailed profile information about the current user (including the current user's username).
- `spotify-pp-cli me get-followed` — Get the current user's followed artists.
- `spotify-pp-cli me get-information-about-the-users-current-playback` — Get information about the user’s current playback state, including track or episode, progress, and active device.
- `spotify-pp-cli me get-queue` — Get the list of objects that make up the user's queue.
- `spotify-pp-cli me get-recently-played` — Get tracks from the current user's recently played tracks. _**Note**: Currently doesn't support podcast episodes._
- `spotify-pp-cli me get-the-users-currently-playing-track` — Get the object currently being played on the user's Spotify account.
- `spotify-pp-cli me get-users-saved-albums` — Get a list of the albums saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-saved-audiobooks` — Get a list of the audiobooks saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-saved-episodes` — Get a list of the episodes saved in the current Spotify user's library.
- `spotify-pp-cli me get-users-saved-shows` — Get a list of shows saved in the current Spotify user's library. Optional parameters can be used to limit the number...
- `spotify-pp-cli me get-users-saved-tracks` — Get a list of the songs saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-top-artists` — Get the current user's top artists based on calculated affinity.
- `spotify-pp-cli me get-users-top-tracks` — Get the current user's top tracks based on calculated affinity.
- `spotify-pp-cli me pause-a-users-playback` — Pause playback on the user's account. This API only works for users who have Spotify Premium. The order of execution...
- `spotify-pp-cli me remove-albums-user` — Remove one or more albums from the current user's 'Your Music' library. **Note:** This endpoint is deprecated. Use...
- `spotify-pp-cli me remove-audiobooks-user` — Remove one or more audiobooks from the Spotify user's library. **Note:** This endpoint is deprecated. Use [Remove...
- `spotify-pp-cli me remove-episodes-user` — Remove one or more episodes from the current user's library. **Note:** This endpoint is deprecated. Use [Remove...
- `spotify-pp-cli me remove-library-items` — Remove one or more items from the current user's library. Accepts Spotify URIs for tracks, albums, episodes, shows,...
- `spotify-pp-cli me remove-shows-user` — Delete one or more shows from current Spotify user's library. **Note:** This endpoint is deprecated. Use [Remove...
- `spotify-pp-cli me remove-tracks-user` — Remove one or more tracks from the current user's 'Your Music' library. **Note:** This endpoint is deprecated. Use...
- `spotify-pp-cli me save-albums-user` — Save one or more albums to the current user's 'Your Music' library. **Note:** This endpoint is deprecated. Use [Save...
- `spotify-pp-cli me save-audiobooks-user` — Save one or more audiobooks to the current Spotify user's library. **Note:** This endpoint is deprecated. Use [Save...
- `spotify-pp-cli me save-episodes-user` — Save one or more episodes to the current user's library. **Note:** This endpoint is deprecated. Use [Save Items to...
- `spotify-pp-cli me save-library-items` — Add one or more items to the current user's library. Accepts Spotify URIs for tracks, albums, episodes, shows,...
- `spotify-pp-cli me save-shows-user` — Save one or more shows to current Spotify user's library. **Note:** This endpoint is deprecated. Use [Save Items to...
- `spotify-pp-cli me save-tracks-user` — Save one or more tracks to the current user's 'Your Music' library. **Note:** This endpoint is deprecated. Use [Save...
- `spotify-pp-cli me seek-to-position-in-currently-playing-track` — Seeks to the given position in the user’s currently playing track. This API only works for users who have Spotify...
- `spotify-pp-cli me set-repeat-mode-on-users-playback` — Set the repeat mode for the user's playback. This API only works for users who have Spotify Premium. The order of...
- `spotify-pp-cli me set-volume-for-users-playback` — Set the volume for the user’s current playback device. This API only works for users who have Spotify Premium. The...
- `spotify-pp-cli me skip-users-playback-to-next-track` — Skips to next track in the user’s queue. This API only works for users who have Spotify Premium. The order of...
- `spotify-pp-cli me skip-users-playback-to-previous-track` — Skips to previous track in the user’s queue. This API only works for users who have Spotify Premium. The order of...
- `spotify-pp-cli me start-a-users-playback` — Start a new context or resume current playback on the user's active device. This API only works for users who have...
- `spotify-pp-cli me toggle-shuffle-for-users-playback` — Toggle shuffle on or off for user’s playback. This API only works for users who have Spotify Premium. The order of...
- `spotify-pp-cli me transfer-a-users-playback` — Transfer playback to a new device and optionally begin playback. This API only works for users who have Spotify...
- `spotify-pp-cli me unfollow-artists-users` — Remove the current user as a follower of one or more artists or other Spotify users. **Note:** This endpoint is...

**playlists** — Manage playlists

- `spotify-pp-cli playlists change-details` — Change a playlist's name and public/private state. (The user must, of course, own the playlist.)
- `spotify-pp-cli playlists get` — Get a playlist owned by a Spotify user.

**recommendations** — Manage recommendations

- `spotify-pp-cli recommendations get` — Recommendations are generated based on the available information for a given seed entity and matched against similar...
- `spotify-pp-cli recommendations get-genres` — Retrieve a list of available genres seed parameter values for...

**shows** — Manage shows

- `spotify-pp-cli shows get-a` — Get Spotify catalog information for a single show identified by its unique Spotify ID.
- `spotify-pp-cli shows get-multiple` — Get Spotify catalog information for several shows based on their Spotify IDs.

**spotify-web-search** — Manage spotify web search

- `spotify-pp-cli spotify-web-search` — Get Spotify catalog information about albums, artists, playlists, tracks, shows, episodes or audiobooks that match a...

**tracks** — Manage tracks

- `spotify-pp-cli tracks get` — Get Spotify catalog information for a single track identified by its unique Spotify ID.
- `spotify-pp-cli tracks get-several` — Get Spotify catalog information for multiple tracks based on their Spotify IDs.

**users** — Manage users

- `spotify-pp-cli users <user_id>` — Get public profile information about a Spotify user.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
spotify-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Top tracks this month, just the names and albums

```bash
spotify-pp-cli me get-users-top-tracks --time-range short_term --limit 25 --agent --select items.name,items.album.name,items.artists.0.name
```

Selects only the three fields an agent typically needs from a deeply-nested top-tracks payload; cuts response size from tens of KB to a flat list.

### List a playlist's tracks with added-at and the primary artist

```bash
spotify-pp-cli me get-a-list-of-current-users-playlists --limit 20 --agent --select items.name,items.tracks.total,items.public
```

Playlist-tracks responses nest the track object two levels deep with an array of artists; --select grabs only the primary artist and the added_at timestamp.

### Search tracks and keep only names and URIs

```bash
spotify-pp-cli search "miles davis kind of blue" --type tracks --limit 10 --agent --select tracks.items.name,tracks.items.uri
```

Search responses are wrapped by entity type; the dotted select walks tracks.items[] and drops the album, artists, and external_urls subtrees that an agent rarely needs for follow-up calls.

### Now-playing as a tight agent payload

```bash
spotify-pp-cli me get-the-users-currently-playing-track --agent --select item.name,item.artists.0.name,is_playing,progress_ms
```

Now-playing returns a 30+ field payload including album art URLs and external IDs; the select narrows to the five fields most agents actually act on.

### Create a private playlist and add tracks in one flow

```bash
spotify-pp-cli me create-playlist --name "agent-curated" --public false --description "built by an agent" --agent
```

End-to-end playlist creation; the first command prints the new playlist id which the agent feeds into add-tracks. Snapshot-aware add-tracks warns if the playlist changed between create and write.

## Auth Setup

This CLI talks to the Spotify Web API on behalf of a real Spotify user, so it needs your own **Spotify Developer app credentials** (Spotify does not ship a public OAuth client). Walk a new user through these steps before running any command that hits the API.

### 1. Create a Spotify Developer app

1. Open <https://developer.spotify.com/dashboard> and sign in with the Spotify account you want the CLI to read/control.
2. Click **Create app**.
3. Fill in:
   - **App name** — anything (e.g. "my spotify-pp-cli").
   - **App description** — anything.
   - **Website** — optional; leave blank.
   - **Which API/SDKs are you planning to use?** — tick **Web API**.

### 2. Register the redirect URI (this is the easy step to get wrong)

The CLI runs an OAuth callback server on your machine at **`http://127.0.0.1:8085/callback`** by default. Spotify will reject the login flow with `INVALID_CLIENT: Invalid redirect URI` unless that exact string is registered on the app:

1. On the app's **Redirect URIs** field, paste exactly:
   ```
   http://127.0.0.1:8085/callback
   ```
2. Click **Add**, then **Save**.

Gotchas to avoid:
- Use **`127.0.0.1`**, not `localhost`. Spotify enforces RFC 8252 for new apps and rejects `localhost` loopback URIs.
- Include `http://`, the port, and `/callback` — no trailing slash, no `https`.
- If you pass `--port <N>` to `auth login`, register `http://127.0.0.1:<N>/callback` instead (and pick a port that isn't in use).

### 3. Copy the Client ID (and Client Secret, if you want Authorization Code with secret)

On the app's **Settings** page:
- Copy the **Client ID** — you always need this.
- Click **View client secret** and copy the **Client Secret** — only needed if you want to use the Authorization Code flow with a secret. If you skip it, the CLI uses PKCE, which is simpler and recommended for desktop use.

### 4. Export the credentials

```bash
export SPOTIFY_CLIENT_ID="<paste client id>"
# Optional — only set if you want to use Authorization Code with secret.
# Either SPOTIFY_CLIENT_SECRET or SPOTIFY_SECRET works.
export SPOTIFY_SECRET="<paste client secret>"
```

Persist them in your shell rc (`~/.zshrc`, `~/.bashrc`) or a project `.env` so future sessions don't need to re-export.

### 5. Run the OAuth login

```bash
spotify-pp-cli auth login
```

The CLI will:
1. Open your default browser to the Spotify consent screen.
2. Listen on `127.0.0.1:8085` for the redirect.
3. Capture the authorization code, exchange it for an access + refresh token, and persist the tokens to `~/.config/spotify-pp-cli/token.json`.

If the browser shows `redirect_uri: Not matching configuration`, the URI registered on the Spotify app does not exactly match the one the CLI used — re-check Step 2.

If the browser shows `INVALID_CLIENT`, the `SPOTIFY_CLIENT_ID` env var is empty or wrong — re-check Step 4.

### 6. Verify

```bash
spotify-pp-cli auth status
spotify-pp-cli me get-current-users-profile --agent
```

`auth status` confirms a valid token is cached; the second command proves the token actually works against the live API.

### Refresh and rotation

Access tokens refresh transparently before each API call; rotating refresh tokens are written back to `~/.config/spotify-pp-cli/token.json` on each refresh. There is no need to re-run `auth login` unless you revoke the app from <https://www.spotify.com/account/apps/> or delete the token file.

### Heads-up: Spotify deprecated some endpoints for new apps on 2024-11-27

Spotify removed access for apps registered after **2024-11-27** to `/audio-features`, `/audio-analysis`, `/recommendations`, `/artists/{id}/related-artists`, `/browse/featured-playlists`, `/browse/categories/{id}/playlists`, and `/browse/new-releases`. Live testing also confirmed `artist.genres` / `artist.popularity` come back null on new apps, and `/artists/{id}/albums` caps `limit` at ~10. The CLI surfaces typed errors with hints when you hit one of these. Discovery features that depend on those endpoints (`discover artists`, `discover new-releases`) will return empty or 403; **`discover artist-gaps` works** because it paginates around the limit cap. Use `spotify-pp-cli doctor` to see the live state.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  spotify-pp-cli audio-analysis mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
spotify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
spotify-pp-cli feedback --stdin < notes.txt
spotify-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.spotify-pp-cli/feedback.jsonl`. They are never POSTed unless `SPOTIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPOTIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
spotify-pp-cli profile save briefing --json
spotify-pp-cli --profile briefing audio-analysis mock-value
spotify-pp-cli profile list --json
spotify-pp-cli profile show briefing
spotify-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `spotify-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add spotify-pp-mcp -- spotify-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which spotify-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   spotify-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `spotify-pp-cli <command> --help`.
