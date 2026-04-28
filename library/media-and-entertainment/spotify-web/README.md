# Spotify Web CLI

You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback, and much more. Browse our available Web API endpoints using the sidebar at left, or via the navigation bar on top of this page on smaller screens.

In order to make successful Web API requests your app will need a valid access token. One can be obtained through <a href="https://developer.spotify.com/documentation/general/guides/authorization-guide/">OAuth 2.0</a>.

The base URI for all Web API requests is `https://api.spotify.com/v1`.

Need help? See our <a href="https://developer.spotify.com/documentation/web-api/guides/">Web API guides</a> for more information, or visit the <a href="https://community.spotify.com/t5/Spotify-for-Developers/bd-p/Spotify_Developer">Spotify for Developers community forum</a> to ask questions and connect with other developers.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/spotify-web-pp-cli/cmd/spotify-web-pp-cli@latest
```

### Binary

Download from [Releases](https://github.com/mvanhorn/printing-press-library/releases).

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
spotify-web-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export SPOTIFY_WEB_TOKEN="your-token-here"
```

### 3. Verify Setup

```bash
spotify-web-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
spotify-web-pp-cli audio-analysis
```

## Usage

Run `spotify-web-pp-cli --help` for the full command reference and flag list.

## Commands

### albums

Manage albums

- **`spotify-web-pp-cli albums get-an`** - Get Album
- **`spotify-web-pp-cli albums get-multiple`** - Get Several Albums

### artists

Manage artists

- **`spotify-web-pp-cli artists get-an`** - Get Artist
- **`spotify-web-pp-cli artists get-multiple`** - Get Several Artists

### audio-analysis

Manage audio analysis

- **`spotify-web-pp-cli audio-analysis get`** - Get Track's Audio Analysis

### audio-features

Manage audio features

- **`spotify-web-pp-cli audio-features get`** - Get Track's Audio Features
- **`spotify-web-pp-cli audio-features get-several`** - Get Tracks' Audio Features

### audiobooks

Manage audiobooks

- **`spotify-web-pp-cli audiobooks get-an`** - Get an Audiobook
- **`spotify-web-pp-cli audiobooks get-multiple`** - Get Several Audiobooks

### browse

Manage browse

- **`spotify-web-pp-cli browse get-a-categories-playlists`** - Get Category's Playlists
- **`spotify-web-pp-cli browse get-a-category`** - Get Single Browse Category
- **`spotify-web-pp-cli browse get-categories`** - Get Several Browse Categories
- **`spotify-web-pp-cli browse get-featured-playlists`** - Get Featured Playlists
- **`spotify-web-pp-cli browse get-new-releases`** - Get New Releases

### chapters

Manage chapters

- **`spotify-web-pp-cli chapters get-a`** - Get a Chapter
- **`spotify-web-pp-cli chapters get-several`** - Get Several Chapters

### episodes

Manage episodes

- **`spotify-web-pp-cli episodes get-an`** - Get Episode
- **`spotify-web-pp-cli episodes get-multiple`** - Get Several Episodes

### markets

Manage markets

- **`spotify-web-pp-cli markets get-available`** - Get Available Markets

### me

Manage me

- **`spotify-web-pp-cli me add-to-queue`** - Add Item to Playback Queue
- **`spotify-web-pp-cli me check-current-user-follows`** - Check If User Follows Artists or Users
- **`spotify-web-pp-cli me check-users-saved-albums`** - Check User's Saved Albums
- **`spotify-web-pp-cli me check-users-saved-audiobooks`** - Check User's Saved Audiobooks
- **`spotify-web-pp-cli me check-users-saved-episodes`** - Check User's Saved Episodes
- **`spotify-web-pp-cli me check-users-saved-shows`** - Check User's Saved Shows
- **`spotify-web-pp-cli me check-users-saved-tracks`** - Check User's Saved Tracks
- **`spotify-web-pp-cli me follow-artists-users`** - Follow Artists or Users
- **`spotify-web-pp-cli me get-a-list-of-current-users-playlists`** - Get Current User's Playlists
- **`spotify-web-pp-cli me get-a-users-available-devices`** - Get Available Devices
- **`spotify-web-pp-cli me get-current-users-profile`** - Get Current User's Profile
- **`spotify-web-pp-cli me get-followed`** - Get Followed Artists
- **`spotify-web-pp-cli me get-information-about-the-users-current-playback`** - Get Playback State
- **`spotify-web-pp-cli me get-queue`** - Get the User's Queue
- **`spotify-web-pp-cli me get-recently-played`** - Get Recently Played Tracks
- **`spotify-web-pp-cli me get-the-users-currently-playing-track`** - Get Currently Playing Track
- **`spotify-web-pp-cli me get-users-saved-albums`** - Get User's Saved Albums
- **`spotify-web-pp-cli me get-users-saved-audiobooks`** - Get User's Saved Audiobooks
- **`spotify-web-pp-cli me get-users-saved-episodes`** - Get User's Saved Episodes
- **`spotify-web-pp-cli me get-users-saved-shows`** - Get User's Saved Shows
- **`spotify-web-pp-cli me get-users-saved-tracks`** - Get User's Saved Tracks
- **`spotify-web-pp-cli me get-users-top-artists-and-tracks`** - Get User's Top Items
- **`spotify-web-pp-cli me pause-a-users-playback`** - Pause Playback
- **`spotify-web-pp-cli me remove-albums-user`** - Remove Users' Saved Albums
- **`spotify-web-pp-cli me remove-audiobooks-user`** - Remove User's Saved Audiobooks
- **`spotify-web-pp-cli me remove-episodes-user`** - Remove User's Saved Episodes
- **`spotify-web-pp-cli me remove-shows-user`** - Remove User's Saved Shows
- **`spotify-web-pp-cli me remove-tracks-user`** - Remove User's Saved Tracks
- **`spotify-web-pp-cli me save-albums-user`** - Save Albums for Current User
- **`spotify-web-pp-cli me save-audiobooks-user`** - Save Audiobooks for Current User
- **`spotify-web-pp-cli me save-episodes-user`** - Save Episodes for Current User
- **`spotify-web-pp-cli me save-shows-user`** - Save Shows for Current User
- **`spotify-web-pp-cli me save-tracks-user`** - Save Tracks for Current User
- **`spotify-web-pp-cli me seek-to-position-in-currently-playing-track`** - Seek To Position
- **`spotify-web-pp-cli me set-repeat-mode-on-users-playback`** - Set Repeat Mode
- **`spotify-web-pp-cli me set-volume-for-users-playback`** - Set Playback Volume
- **`spotify-web-pp-cli me skip-users-playback-to-next-track`** - Skip To Next
- **`spotify-web-pp-cli me skip-users-playback-to-previous-track`** - Skip To Previous
- **`spotify-web-pp-cli me start-a-users-playback`** - Start/Resume Playback
- **`spotify-web-pp-cli me toggle-shuffle-for-users-playback`** - Toggle Playback Shuffle
- **`spotify-web-pp-cli me transfer-a-users-playback`** - Transfer Playback
- **`spotify-web-pp-cli me unfollow-artists-users`** - Unfollow Artists or Users

### playlists

Manage playlists

- **`spotify-web-pp-cli playlists change-details`** - Change Playlist Details
- **`spotify-web-pp-cli playlists get`** - Get Playlist

### recommendations

Manage recommendations

- **`spotify-web-pp-cli recommendations get`** - Get Recommendations
- **`spotify-web-pp-cli recommendations get-genres`** - Get Available Genre Seeds

### search

Manage search

- **`spotify-web-pp-cli search search`** - Search for Item

### shows

Manage shows

- **`spotify-web-pp-cli shows get-a`** - Get Show
- **`spotify-web-pp-cli shows get-multiple`** - Get Several Shows

### tracks

Manage tracks

- **`spotify-web-pp-cli tracks get`** - Get Track
- **`spotify-web-pp-cli tracks get-several`** - Get Several Tracks

### users

Manage users

- **`spotify-web-pp-cli users get-profile`** - Get User's Profile


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
spotify-web-pp-cli audio-analysis

# JSON for scripting and agents
spotify-web-pp-cli audio-analysis --json

# Filter to specific fields
spotify-web-pp-cli audio-analysis --json --select id,name,status

# Dry run — show the request without sending
spotify-web-pp-cli audio-analysis --dry-run

# Agent mode — JSON + compact + no prompts in one flag
spotify-web-pp-cli audio-analysis --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add spotify-web spotify-web-pp-mcp -e SPOTIFY_WEB_TOKEN=<your-token>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "spotify-web": {
      "command": "spotify-web-pp-mcp",
      "env": {
        "SPOTIFY_WEB_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Health Check

```bash
spotify-web-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/spotify-web-pp-cli/config.toml`

Environment variables:
- `SPOTIFY_WEB_TOKEN`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `spotify-web-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPOTIFY_WEB_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
