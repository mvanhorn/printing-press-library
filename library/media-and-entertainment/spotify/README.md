# Spotify CLI

**An agent-native Spotify CLI with a SQLite-backed local library that lets you ask listening-drift questions no other Spotify tool can answer.**

Every Spotify Web API endpoint is reachable from one static binary with --json and --select everywhere and a built-in MCP server, so an agent can drive playback, search, and library curation with the same surface a human uses. A local SQLite store captures top-tracks/top-artists snapshots, snapshot-keyed playlist tracks, and extended play history beyond Spotify's 50-event cap, enabling one-shot queries like 'which tracks dropped out of medium-term but stayed in long-term' that no existing CLI can answer. Endpoints Spotify removed for new apps on 2024-11-27 ship as honest deprecation-aware stubs with a --legacy-app opt-in.

Learn more at [Spotify](https://github.com/sonallux/spotify-web-api).

Printed by [@rob-coco](https://github.com/rob-coco) (Rob Zehner).

## Install

The recommended path installs both the `spotify-pp-cli` binary and the `pp-spotify` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install spotify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install spotify --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spotify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-spotify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-spotify --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-spotify skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-spotify. The skill defines how its required CLI can be installed.
```

## Authentication

OAuth flows are auto-detected: PKCE is the default when only SPOTIFY_CLIENT_ID is set, and Authorization Code with secret is used when SPOTIFY_SECRET is also present; access tokens refresh transparently and rotating refresh tokens are persisted to ~/.config/spotify-pp-cli/token.json.

## Quick Start

```bash
# OAuth PKCE by default; opens a browser and captures the token on a loopback redirect.
spotify-pp-cli auth login


# Confirms the token works and shows display_name, product (premium/free), country.
spotify-pp-cli me


# Verifies the read path works without any user-library scope.
spotify-pp-cli search "never gonna give you up" --type tracks --limit 5


# Shows what's currently playing on which device; foundation for every playback command.
spotify-pp-cli now-playing --json


# Lists active devices so you know which --device id to target for play/transfer.
spotify-pp-cli devices list


# Pulls top tracks (last 6 months) and snapshots them into the local store so drift queries become possible.
spotify-pp-cli top tracks --range medium --limit 20


# Lists your playlists; subsequent playlists tracks <id> calls are snapshot-aware.
spotify-pp-cli playlists list --limit 50

```

## Unique Features

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
- **`queue from-saved`** — Pick N tracks from your saved library (optionally filtered by artist or playlist origin) and queue them in one command.

  _Use this when an agent should 'queue something from my chillout saved tracks' or 'queue 10 more from this artist' without resorting to URI manipulation._

  ```bash
  spotify-pp-cli queue from-saved --limit 10 --artist 0OdUWJ0sBjDrqHygGUXeCF
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

## Usage

Run `spotify-pp-cli --help` for the full command reference and flag list.

## Commands

### albums

Manage albums

- **`spotify-pp-cli albums get-an`** - Get Spotify catalog information for a single album.
- **`spotify-pp-cli albums get-multiple`** - Get Spotify catalog information for multiple albums identified by their Spotify IDs.

### artists

Manage artists

- **`spotify-pp-cli artists get-an`** - Get Spotify catalog information for a single artist identified by their unique Spotify ID.
- **`spotify-pp-cli artists get-multiple`** - Get Spotify catalog information for several artists based on their Spotify IDs.

### audio-analysis

Manage audio analysis

- **`spotify-pp-cli audio-analysis get`** - Get a low-level audio analysis for a track in the Spotify catalog. The audio analysis describes the track’s structure and musical content, including rhythm, pitch, and timbre.

### audio-features

Manage audio features

- **`spotify-pp-cli audio-features get`** - Get audio feature information for a single track identified by its unique
Spotify ID.
- **`spotify-pp-cli audio-features get-several`** - Get audio features for multiple tracks based on their Spotify IDs.

### audiobooks

Manage audiobooks

- **`spotify-pp-cli audiobooks get-an`** - Get Spotify catalog information for a single audiobook. Audiobooks are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.
- **`spotify-pp-cli audiobooks get-multiple`** - Get Spotify catalog information for several audiobooks identified by their Spotify IDs. Audiobooks are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.

### browse

Manage browse

- **`spotify-pp-cli browse get-a-categories-playlists`** - Get a list of Spotify playlists tagged with a particular category.
- **`spotify-pp-cli browse get-a-category`** - Get a single category used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- **`spotify-pp-cli browse get-categories`** - Get a list of categories used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- **`spotify-pp-cli browse get-featured-playlists`** - Get a list of Spotify featured playlists (shown, for example, on a Spotify player's 'Browse' tab).
- **`spotify-pp-cli browse get-new-releases`** - Get a list of new album releases featured in Spotify (shown, for example, on a Spotify player’s “Browse” tab).

### chapters

Manage chapters

- **`spotify-pp-cli chapters get-a`** - Get Spotify catalog information for a single audiobook chapter. Chapters are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.
- **`spotify-pp-cli chapters get-several`** - Get Spotify catalog information for several audiobook chapters identified by their Spotify IDs. Chapters are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.

### episodes

Manage episodes

- **`spotify-pp-cli episodes get-an`** - Get Spotify catalog information for a single episode identified by its
unique Spotify ID.
- **`spotify-pp-cli episodes get-multiple`** - Get Spotify catalog information for several episodes based on their Spotify IDs.

### markets

Manage markets

- **`spotify-pp-cli markets get-available`** - Get the list of markets where Spotify is available.

### me

Manage me

- **`spotify-pp-cli me add-to-queue`** - Add an item to be played next in the user's current playback queue. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me check-current-user-follows`** - Check to see if the current user is following one or more artists or other Spotify users.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me check-library-contains`** - Check if one or more items are already saved in the current user's library. Accepts Spotify URIs for tracks, albums, episodes, shows, audiobooks, artists, users, and playlists.
- **`spotify-pp-cli me check-users-saved-albums`** - Check if one or more albums is already saved in the current Spotify user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me check-users-saved-audiobooks`** - Check if one or more audiobooks are already saved in the current Spotify user's library.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me check-users-saved-episodes`** - Check if one or more episodes is already saved in the current Spotify user's 'Your Episodes' library.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me check-users-saved-shows`** - Check if one or more shows is already saved in the current Spotify user's library.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me check-users-saved-tracks`** - Check if one or more tracks is already saved in the current Spotify user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Check User's Saved Items](/documentation/web-api/reference/check-library-contains) instead.
- **`spotify-pp-cli me create-playlist`** - Create a playlist for the current Spotify user. (The playlist will be empty until
you [add tracks](/documentation/web-api/reference/add-tracks-to-playlist).)
Each user is generally limited to a maximum of 11000 playlists.
- **`spotify-pp-cli me follow-artists-users`** - Add the current user as a follower of one or more artists or other Spotify users.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me get-a-list-of-current-users-playlists`** - Get a list of the playlists owned or followed by the current Spotify
user.
- **`spotify-pp-cli me get-a-users-available-devices`** - Get information about a user’s available Spotify Connect devices. Some device models are not supported and will not be listed in the API response.
- **`spotify-pp-cli me get-current-users-profile`** - Get detailed profile information about the current user (including the
current user's username).
- **`spotify-pp-cli me get-followed`** - Get the current user's followed artists.
- **`spotify-pp-cli me get-information-about-the-users-current-playback`** - Get information about the user’s current playback state, including track or episode, progress, and active device.
- **`spotify-pp-cli me get-queue`** - Get the list of objects that make up the user's queue.
- **`spotify-pp-cli me get-recently-played`** - Get tracks from the current user's recently played tracks.
_**Note**: Currently doesn't support podcast episodes._
- **`spotify-pp-cli me get-the-users-currently-playing-track`** - Get the object currently being played on the user's Spotify account.
- **`spotify-pp-cli me get-users-saved-albums`** - Get a list of the albums saved in the current Spotify user's 'Your Music' library.
- **`spotify-pp-cli me get-users-saved-audiobooks`** - Get a list of the audiobooks saved in the current Spotify user's 'Your Music' library.
- **`spotify-pp-cli me get-users-saved-episodes`** - Get a list of the episodes saved in the current Spotify user's library.
- **`spotify-pp-cli me get-users-saved-shows`** - Get a list of shows saved in the current Spotify user's library. Optional parameters can be used to limit the number of shows returned.
- **`spotify-pp-cli me get-users-saved-tracks`** - Get a list of the songs saved in the current Spotify user's 'Your Music' library.
- **`spotify-pp-cli me get-users-top-artists`** - Get the current user's top artists based on calculated affinity.
- **`spotify-pp-cli me get-users-top-tracks`** - Get the current user's top tracks based on calculated affinity.
- **`spotify-pp-cli me pause-a-users-playback`** - Pause playback on the user's account. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me remove-albums-user`** - Remove one or more albums from the current user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.
- **`spotify-pp-cli me remove-audiobooks-user`** - Remove one or more audiobooks from the Spotify user's library.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.
- **`spotify-pp-cli me remove-episodes-user`** - Remove one or more episodes from the current user's library.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.
- **`spotify-pp-cli me remove-library-items`** - Remove one or more items from the current user's library. Accepts Spotify URIs for tracks, albums, episodes, shows, audiobooks, users, and playlists.
- **`spotify-pp-cli me remove-shows-user`** - Delete one or more shows from current Spotify user's library.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.
- **`spotify-pp-cli me remove-tracks-user`** - Remove one or more tracks from the current user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.
- **`spotify-pp-cli me save-albums-user`** - Save one or more albums to the current user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me save-audiobooks-user`** - Save one or more audiobooks to the current Spotify user's library.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me save-episodes-user`** - Save one or more episodes to the current user's library.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me save-library-items`** - Add one or more items to the current user's library. Accepts Spotify URIs for tracks, albums, episodes, shows, audiobooks, users, and playlists.
- **`spotify-pp-cli me save-shows-user`** - Save one or more shows to current Spotify user's library.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me save-tracks-user`** - Save one or more tracks to the current user's 'Your Music' library.

**Note:** This endpoint is deprecated. Use [Save Items to Library](/documentation/web-api/reference/save-library-items) instead.
- **`spotify-pp-cli me seek-to-position-in-currently-playing-track`** - Seeks to the given position in the user’s currently playing track. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me set-repeat-mode-on-users-playback`** - Set the repeat mode for the user's playback. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me set-volume-for-users-playback`** - Set the volume for the user’s current playback device. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me skip-users-playback-to-next-track`** - Skips to next track in the user’s queue. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me skip-users-playback-to-previous-track`** - Skips to previous track in the user’s queue. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me start-a-users-playback`** - Start a new context or resume current playback on the user's active device. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me toggle-shuffle-for-users-playback`** - Toggle shuffle on or off for user’s playback. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me transfer-a-users-playback`** - Transfer playback to a new device and optionally begin playback. This API only works for users who have Spotify Premium. The order of execution is not guaranteed when you use this API with other Player API endpoints.
- **`spotify-pp-cli me unfollow-artists-users`** - Remove the current user as a follower of one or more artists or other Spotify users.

**Note:** This endpoint is deprecated. Use [Remove Items from Library](/documentation/web-api/reference/remove-library-items) instead.

### playlists

Manage playlists

- **`spotify-pp-cli playlists change-details`** - Change a playlist's name and public/private state. (The user must, of
course, own the playlist.)
- **`spotify-pp-cli playlists get`** - Get a playlist owned by a Spotify user.

### recommendations

Manage recommendations

- **`spotify-pp-cli recommendations get`** - Recommendations are generated based on the available information for a given seed entity and matched against similar artists and tracks. If there is sufficient information about the provided seeds, a list of tracks will be returned together with pool size details.

For artists and tracks that are very new or obscure there might not be enough data to generate a list of tracks.
- **`spotify-pp-cli recommendations get-genres`** - Retrieve a list of available genres seed parameter values for [recommendations](/documentation/web-api/reference/get-recommendations).

### shows

Manage shows

- **`spotify-pp-cli shows get-a`** - Get Spotify catalog information for a single show identified by its
unique Spotify ID.
- **`spotify-pp-cli shows get-multiple`** - Get Spotify catalog information for several shows based on their Spotify IDs.

### spotify-web-search

Manage spotify web search

- **`spotify-pp-cli spotify-web-search search`** - Get Spotify catalog information about albums, artists, playlists, tracks, shows, episodes or audiobooks
that match a keyword string. Audiobooks are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.

### tracks

Manage tracks

- **`spotify-pp-cli tracks get`** - Get Spotify catalog information for a single track identified by its
unique Spotify ID.
- **`spotify-pp-cli tracks get-several`** - Get Spotify catalog information for multiple tracks based on their Spotify IDs.

### users

Manage users

- **`spotify-pp-cli users get-profile`** - Get public profile information about a Spotify user.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
spotify-pp-cli audio-analysis mock-value

# JSON for scripting and agents
spotify-pp-cli audio-analysis mock-value --json

# Filter to specific fields
spotify-pp-cli audio-analysis mock-value --json --select id,name,status

# Dry run — show the request without sending
spotify-pp-cli audio-analysis mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
spotify-pp-cli audio-analysis mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-spotify -g
```

Then invoke `/pp-spotify <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add spotify spotify-pp-mcp -e SPOTIFY_WEB_OAUTH_2_0=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spotify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SPOTIFY_WEB_OAUTH_2_0` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "spotify": {
      "command": "spotify-pp-mcp",
      "env": {
        "SPOTIFY_WEB_OAUTH_2_0": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
spotify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/spotify-web-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPOTIFY_WEB_OAUTH_2_0` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `spotify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPOTIFY_WEB_OAUTH_2_0`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on any command** — Run `spotify-pp-cli auth refresh` to rotate the access token, or `spotify-pp-cli auth login` if the refresh token is also expired.
- **403 PREMIUM_REQUIRED on play / pause / next / seek / volume** — Playback writes require Spotify Premium. Read commands (now-playing, search, playlists) work on free accounts.
- **403 or 404 on audio-features, audio-analysis, recommendations, artists related, playlists featured** — Spotify removed these endpoints for apps created after 2024-11-27. The CLI ships these as honest stubs; pass --legacy-app only if your app has grandfathered extended-quota access. See https://developer.spotify.com/blog/2024-11-27-changes-to-the-web-api
- **429 Too Many Requests** — The CLI auto-honors the Retry-After header and backs off with jitter. If you keep hitting it, slow your batch loop or split work across multiple 30-second windows.
- **NO_ACTIVE_DEVICE on play / pause / transfer** — Open Spotify on a phone, desktop, or speaker so a device registers, then retry. Run `spotify-pp-cli devices list` to confirm a device is available and copy its id for --device.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**spotify-tui**](https://github.com/Rigellute/spotify-tui) — Rust (17000 stars)
- [**spotipy**](https://github.com/spotipy-dev/spotipy) — Python (7000 stars)
- [**spotify-web-api-node**](https://github.com/thelinmichael/spotify-web-api-node) — JavaScript (2700 stars)
- [**marcelmarais/spotify-mcp-server**](https://github.com/marcelmarais/spotify-mcp-server) — TypeScript
- [**ncspot**](https://github.com/hrkfdn/ncspot) — Rust

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
