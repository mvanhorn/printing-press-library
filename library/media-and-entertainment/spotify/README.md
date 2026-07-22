# Spotify CLI

You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback, and much more. Browse our available Web API endpoints using the sidebar at left, or via the navigation bar on top of this page on smaller screens.

In order to make successful Web API requests your app will need a valid access token. One can be obtained through <a href="https://developer.spotify.com/documentation/general/guides/authorization-guide/">OAuth 2.0</a>.

The base URI for all Web API requests is `https://api.spotify.com/v1`.

Need help? See our <a href="https://developer.spotify.com/documentation/web-api/guides/">Web API guides</a> for more information, or visit the <a href="https://community.spotify.com/t5/Spotify-for-Developers/bd-p/Spotify_Developer">Spotify for Developers community forum</a> to ask questions and connect with other developers.

Learn more at [Spotify](https://github.com/sonallux/spotify-web-api).

Created by [@rob-coco](https://github.com/rob-coco) (Rob Zehner).

## Install

The recommended path installs both the `spotify-pp-cli` binary and the `pp-spotify` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install spotify
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install spotify --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install spotify --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install spotify --agent claude-code
npx -y @mvanhorn/printing-press-library install spotify --agent claude-code --agent codex
```

### Without Node

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/cmd/spotify-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spotify-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install spotify --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-spotify --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-spotify --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install spotify --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/spotify-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SPOTIFY_OAUTH_2_0` when Claude Desktop prompts you.

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
        "SPOTIFY_PLAYLIST_ID": "<playlist_id>",
        "SPOTIFY_USER_ID": "<user_id>",
        "SPOTIFY_OAUTH_2_0": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
spotify-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export SPOTIFY_OAUTH_2_0="your-token-here"
```

### 3. Verify Setup

```bash
spotify-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
spotify-pp-cli audio-analysis mock-value
```

## Usage

Run `spotify-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SPOTIFY_CONFIG_DIR`, `SPOTIFY_DATA_DIR`, `SPOTIFY_STATE_DIR`, or `SPOTIFY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SPOTIFY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SPOTIFY_HOME=/srv/spotify
spotify-pp-cli doctor
```

Under `SPOTIFY_HOME=/srv/spotify`, the four dirs resolve to `/srv/spotify/config`, `/srv/spotify/data`, `/srv/spotify/state`, and `/srv/spotify/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "spotify": {
      "command": "spotify-pp-mcp",
      "env": {
        "SPOTIFY_HOME": "/srv/spotify"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SPOTIFY_DATA_DIR` overrides an explicit `--home` for that kind. Use `SPOTIFY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SPOTIFY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `spotify-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

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

- **`spotify-pp-cli audio-analysis <id>`** - Get a low-level audio analysis for a track in the Spotify catalog. The audio analysis describes the track’s structure and musical content, including rhythm, pitch, and timbre.

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

- **`spotify-pp-cli markets`** - Get the list of markets where Spotify is available.

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

### spotify_web_search

Manage spotify web search

- **`spotify-pp-cli spotify-web-search`** - Get Spotify catalog information about albums, artists, playlists, tracks, shows, episodes or audiobooks
that match a keyword string. Audiobooks are only available within the US, UK, Canada, Ireland, New Zealand and Australia markets.

### tracks

Manage tracks

- **`spotify-pp-cli tracks get`** - Get Spotify catalog information for a single track identified by its
unique Spotify ID.
- **`spotify-pp-cli tracks get-several`** - Get Spotify catalog information for multiple tracks based on their Spotify IDs.

### users

Manage users

- **`spotify-pp-cli users <user_id>`** - Get public profile information about a Spotify user.


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
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `SPOTIFY_PLAYLIST_ID` resolves `{playlist_id}`
- `SPOTIFY_USER_ID` resolves `{user_id}`

Base URL: `https://api.spotify.com/v1`

## Health Check

```bash
spotify-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `spotify-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/spotify-pp-cli/config.toml`; `--home`, `SPOTIFY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPOTIFY_PLAYLIST_ID` | endpoint | Yes |  |
| `SPOTIFY_USER_ID` | endpoint | Yes |  |
| `SPOTIFY_OAUTH_2_0` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `spotify-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `spotify-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPOTIFY_OAUTH_2_0`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
