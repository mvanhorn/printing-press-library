---
name: pp-spotify-web
description: "Printing Press CLI for Spotify Web. You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback,..."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata: '{"openclaw":{"requires":{"bins":["spotify-web-pp-cli"]},"install":[{"id":"go","kind":"shell","command":"go install github.com/mvanhorn/printing-press-library/library/other/spotify-web-pp-cli/cmd/spotify-web-pp-cli@latest","bins":["spotify-web-pp-cli"],"label":"Install via go install"}]}}'
---

# Spotify Web — Printing Press CLI

You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback, and much more. Browse our available Web API endpoints using the sidebar at left, or via the navigation bar on top of this page on smaller screens.

In order to make successful Web API requests your app will need a valid access token. One can be obtained through <a href="https://developer.spotify.com/documentation/general/guides/authorization-guide/">OAuth 2.0</a>.

The base URI for all Web API requests is `https://api.spotify.com/v1`.

Need help? See our <a href="https://developer.spotify.com/documentation/web-api/guides/">Web API guides</a> for more information, or visit the <a href="https://community.spotify.com/t5/Spotify-for-Developers/bd-p/Spotify_Developer">Spotify for Developers community forum</a> to ask questions and connect with other developers.

## Command Reference

**albums** — Manage albums

- `spotify-web-pp-cli albums get-an` — Get Album
- `spotify-web-pp-cli albums get-multiple` — Get Several Albums

**artists** — Manage artists

- `spotify-web-pp-cli artists get-an` — Get Artist
- `spotify-web-pp-cli artists get-multiple` — Get Several Artists

**audio-analysis** — Manage audio analysis

- `spotify-web-pp-cli audio-analysis <id>` — Get Track's Audio Analysis

**audio-features** — Manage audio features

- `spotify-web-pp-cli audio-features get` — Get Track's Audio Features
- `spotify-web-pp-cli audio-features get-several` — Get Tracks' Audio Features

**audiobooks** — Manage audiobooks

- `spotify-web-pp-cli audiobooks get-an` — Get an Audiobook
- `spotify-web-pp-cli audiobooks get-multiple` — Get Several Audiobooks

**browse** — Manage browse

- `spotify-web-pp-cli browse get-a-categories-playlists` — Get Category's Playlists
- `spotify-web-pp-cli browse get-a-category` — Get Single Browse Category
- `spotify-web-pp-cli browse get-categories` — Get Several Browse Categories
- `spotify-web-pp-cli browse get-featured-playlists` — Get Featured Playlists
- `spotify-web-pp-cli browse get-new-releases` — Get New Releases

**chapters** — Manage chapters

- `spotify-web-pp-cli chapters get-a` — Get a Chapter
- `spotify-web-pp-cli chapters get-several` — Get Several Chapters

**episodes** — Manage episodes

- `spotify-web-pp-cli episodes get-an` — Get Episode
- `spotify-web-pp-cli episodes get-multiple` — Get Several Episodes

**markets** — Manage markets

- `spotify-web-pp-cli markets` — Get Available Markets

**me** — Manage me

- `spotify-web-pp-cli me add-to-queue` — Add Item to Playback Queue
- `spotify-web-pp-cli me check-current-user-follows` — Check If User Follows Artists or Users
- `spotify-web-pp-cli me check-users-saved-albums` — Check User's Saved Albums
- `spotify-web-pp-cli me check-users-saved-audiobooks` — Check User's Saved Audiobooks
- `spotify-web-pp-cli me check-users-saved-episodes` — Check User's Saved Episodes
- `spotify-web-pp-cli me check-users-saved-shows` — Check User's Saved Shows
- `spotify-web-pp-cli me check-users-saved-tracks` — Check User's Saved Tracks
- `spotify-web-pp-cli me follow-artists-users` — Follow Artists or Users
- `spotify-web-pp-cli me get-a-list-of-current-users-playlists` — Get Current User's Playlists
- `spotify-web-pp-cli me get-a-users-available-devices` — Get Available Devices
- `spotify-web-pp-cli me get-current-users-profile` — Get Current User's Profile
- `spotify-web-pp-cli me get-followed` — Get Followed Artists
- `spotify-web-pp-cli me get-information-about-the-users-current-playback` — Get Playback State
- `spotify-web-pp-cli me get-queue` — Get the User's Queue
- `spotify-web-pp-cli me get-recently-played` — Get Recently Played Tracks
- `spotify-web-pp-cli me get-the-users-currently-playing-track` — Get Currently Playing Track
- `spotify-web-pp-cli me get-users-saved-albums` — Get User's Saved Albums
- `spotify-web-pp-cli me get-users-saved-audiobooks` — Get User's Saved Audiobooks
- `spotify-web-pp-cli me get-users-saved-episodes` — Get User's Saved Episodes
- `spotify-web-pp-cli me get-users-saved-shows` — Get User's Saved Shows
- `spotify-web-pp-cli me get-users-saved-tracks` — Get User's Saved Tracks
- `spotify-web-pp-cli me get-users-top-artists-and-tracks` — Get User's Top Items
- `spotify-web-pp-cli me pause-a-users-playback` — Pause Playback
- `spotify-web-pp-cli me remove-albums-user` — Remove Users' Saved Albums
- `spotify-web-pp-cli me remove-audiobooks-user` — Remove User's Saved Audiobooks
- `spotify-web-pp-cli me remove-episodes-user` — Remove User's Saved Episodes
- `spotify-web-pp-cli me remove-shows-user` — Remove User's Saved Shows
- `spotify-web-pp-cli me remove-tracks-user` — Remove User's Saved Tracks
- `spotify-web-pp-cli me save-albums-user` — Save Albums for Current User
- `spotify-web-pp-cli me save-audiobooks-user` — Save Audiobooks for Current User
- `spotify-web-pp-cli me save-episodes-user` — Save Episodes for Current User
- `spotify-web-pp-cli me save-shows-user` — Save Shows for Current User
- `spotify-web-pp-cli me save-tracks-user` — Save Tracks for Current User
- `spotify-web-pp-cli me seek-to-position-in-currently-playing-track` — Seek To Position
- `spotify-web-pp-cli me set-repeat-mode-on-users-playback` — Set Repeat Mode
- `spotify-web-pp-cli me set-volume-for-users-playback` — Set Playback Volume
- `spotify-web-pp-cli me skip-users-playback-to-next-track` — Skip To Next
- `spotify-web-pp-cli me skip-users-playback-to-previous-track` — Skip To Previous
- `spotify-web-pp-cli me start-a-users-playback` — Start/Resume Playback
- `spotify-web-pp-cli me toggle-shuffle-for-users-playback` — Toggle Playback Shuffle
- `spotify-web-pp-cli me transfer-a-users-playback` — Transfer Playback
- `spotify-web-pp-cli me unfollow-artists-users` — Unfollow Artists or Users

**playlists** — Manage playlists

- `spotify-web-pp-cli playlists change-details` — Change Playlist Details
- `spotify-web-pp-cli playlists get` — Get Playlist

**recommendations** — Manage recommendations

- `spotify-web-pp-cli recommendations get` — Get Recommendations
- `spotify-web-pp-cli recommendations get-genres` — Get Available Genre Seeds

**search** — Manage search

- `spotify-web-pp-cli search search` — Search for Item

**shows** — Manage shows

- `spotify-web-pp-cli shows get-a` — Get Show
- `spotify-web-pp-cli shows get-multiple` — Get Several Shows

**tracks** — Manage tracks

- `spotify-web-pp-cli tracks get` — Get Track
- `spotify-web-pp-cli tracks get-several` — Get Several Tracks

**users** — Manage users

- `spotify-web-pp-cli users <user_id>` — Get User's Profile


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
spotify-web-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
spotify-web-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `SPOTIFY_WEB_TOKEN` as an environment variable.

Run `spotify-web-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  spotify-web-pp-cli audio-analysis 3n3Ppam7vgaVa1iaRUc9jT --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag

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
spotify-web-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
spotify-web-pp-cli feedback --stdin < notes.txt
spotify-web-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.spotify-web-pp-cli/feedback.jsonl`. They are never POSTed unless `SPOTIFY_WEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPOTIFY_WEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
spotify-web-pp-cli profile save briefing --json
spotify-web-pp-cli --profile briefing audio-analysis
spotify-web-pp-cli profile list --json
spotify-web-pp-cli profile show briefing
spotify-web-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `spotify-web-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/spotify-web-pp-cli/cmd/spotify-web-pp-cli@latest
   ```
3. Verify: `spotify-web-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/spotify-web-pp-cli/cmd/spotify-web-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add spotify-web-pp-mcp -- spotify-web-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which spotify-web-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   spotify-web-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `spotify-web-pp-cli <command> --help`.
