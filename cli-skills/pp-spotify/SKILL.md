---
name: pp-spotify
description: "Printing Press CLI for Spotify. You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback"
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
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/media-and-entertainment/spotify/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# Spotify — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `spotify-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install spotify --cli-only
   ```
2. Verify: `spotify-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/cmd/spotify-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

You can use Spotify's Web API to discover music and podcasts, manage your Spotify library, control audio playback, and much more. Browse our available Web API endpoints using the sidebar at left, or via the navigation bar on top of this page on smaller screens.

In order to make successful Web API requests your app will need a valid access token. One can be obtained through <a href="https://developer.spotify.com/documentation/general/guides/authorization-guide/">OAuth 2.0</a>.

The base URI for all Web API requests is `https://api.spotify.com/v1`.

Need help? See our <a href="https://developer.spotify.com/documentation/web-api/guides/">Web API guides</a> for more information, or visit the <a href="https://community.spotify.com/t5/Spotify-for-Developers/bd-p/Spotify_Developer">Spotify for Developers community forum</a> to ask questions and connect with other developers.

## Command Reference

**albums** — Manage albums

- `spotify-pp-cli albums get-an` — Get Spotify catalog information for a single album.
- `spotify-pp-cli albums get-multiple` — Get Spotify catalog information for multiple albums identified by their Spotify IDs.

**artists** — Manage artists

- `spotify-pp-cli artists get-an` — Get Spotify catalog information for a single artist identified by their unique Spotify ID.
- `spotify-pp-cli artists get-multiple` — Get Spotify catalog information for several artists based on their Spotify IDs.

**audio-analysis** — Manage audio analysis

- `spotify-pp-cli audio-analysis <id>` — Get a low-level audio analysis for a track in the Spotify catalog.

**audio-features** — Manage audio features

- `spotify-pp-cli audio-features get` — Get audio feature information for a single track identified by its unique Spotify ID.
- `spotify-pp-cli audio-features get-several` — Get audio features for multiple tracks based on their Spotify IDs.

**audiobooks** — Manage audiobooks

- `spotify-pp-cli audiobooks get-an` — Get Spotify catalog information for a single audiobook.
- `spotify-pp-cli audiobooks get-multiple` — Get Spotify catalog information for several audiobooks identified by their Spotify IDs.

**browse** — Manage browse

- `spotify-pp-cli browse get-a-categories-playlists` — Get a list of Spotify playlists tagged with a particular category.
- `spotify-pp-cli browse get-a-category` — Get a single category used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- `spotify-pp-cli browse get-categories` — Get a list of categories used to tag items in Spotify (on, for example, the Spotify player’s “Browse” tab).
- `spotify-pp-cli browse get-featured-playlists` — Get a list of Spotify featured playlists (shown, for example, on a Spotify player's 'Browse' tab).
- `spotify-pp-cli browse get-new-releases` — Get a list of new album releases featured in Spotify (shown, for example, on a Spotify player’s “Browse” tab).

**chapters** — Manage chapters

- `spotify-pp-cli chapters get-a` — Get Spotify catalog information for a single audiobook chapter.
- `spotify-pp-cli chapters get-several` — Get Spotify catalog information for several audiobook chapters identified by their Spotify IDs.

**episodes** — Manage episodes

- `spotify-pp-cli episodes get-an` — Get Spotify catalog information for a single episode identified by its unique Spotify ID.
- `spotify-pp-cli episodes get-multiple` — Get Spotify catalog information for several episodes based on their Spotify IDs.

**markets** — Manage markets

- `spotify-pp-cli markets` — Get the list of markets where Spotify is available.

**me** — Manage me

- `spotify-pp-cli me add-to-queue` — Add an item to be played next in the user's current playback queue.
- `spotify-pp-cli me check-current-user-follows` — Check to see if the current user is following one or more artists or other Spotify users.
- `spotify-pp-cli me check-library-contains` — Check if one or more items are already saved in the current user's library.
- `spotify-pp-cli me check-users-saved-albums` — Check if one or more albums is already saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me check-users-saved-audiobooks` — Check if one or more audiobooks are already saved in the current Spotify user's library.
- `spotify-pp-cli me check-users-saved-episodes` — Check if one or more episodes is already saved in the current Spotify user's 'Your Episodes' library.
- `spotify-pp-cli me check-users-saved-shows` — Check if one or more shows is already saved in the current Spotify user's library.
- `spotify-pp-cli me check-users-saved-tracks` — Check if one or more tracks is already saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me create-playlist` — Create a playlist for the current Spotify user.
- `spotify-pp-cli me follow-artists-users` — Add the current user as a follower of one or more artists or other Spotify users. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me get-a-list-of-current-users-playlists` — Get a list of the playlists owned or followed by the current Spotify user.
- `spotify-pp-cli me get-a-users-available-devices` — Get information about a user’s available Spotify Connect devices.
- `spotify-pp-cli me get-current-users-profile` — Get detailed profile information about the current user (including the current user's username).
- `spotify-pp-cli me get-followed` — Get the current user's followed artists.
- `spotify-pp-cli me get-information-about-the-users-current-playback` — Get information about the user’s current playback state, including track or episode, progress, and active device.
- `spotify-pp-cli me get-queue` — Get the list of objects that make up the user's queue.
- `spotify-pp-cli me get-recently-played` — Get tracks from the current user's recently played tracks. _**Note**: Currently doesn't support podcast episodes._
- `spotify-pp-cli me get-the-users-currently-playing-track` — Get the object currently being played on the user's Spotify account.
- `spotify-pp-cli me get-users-saved-albums` — Get a list of the albums saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-saved-audiobooks` — Get a list of the audiobooks saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-saved-episodes` — Get a list of the episodes saved in the current Spotify user's library.
- `spotify-pp-cli me get-users-saved-shows` — Get a list of shows saved in the current Spotify user's library.
- `spotify-pp-cli me get-users-saved-tracks` — Get a list of the songs saved in the current Spotify user's 'Your Music' library.
- `spotify-pp-cli me get-users-top-artists` — Get the current user's top artists based on calculated affinity.
- `spotify-pp-cli me get-users-top-tracks` — Get the current user's top tracks based on calculated affinity.
- `spotify-pp-cli me pause-a-users-playback` — Pause playback on the user's account. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me remove-albums-user` — Remove one or more albums from the current user's 'Your Music' library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me remove-audiobooks-user` — Remove one or more audiobooks from the Spotify user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me remove-episodes-user` — Remove one or more episodes from the current user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me remove-library-items` — Remove one or more items from the current user's library.
- `spotify-pp-cli me remove-shows-user` — Delete one or more shows from current Spotify user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me remove-tracks-user` — Remove one or more tracks from the current user's 'Your Music' library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me save-albums-user` — Save one or more albums to the current user's 'Your Music' library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me save-audiobooks-user` — Save one or more audiobooks to the current Spotify user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me save-episodes-user` — Save one or more episodes to the current user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me save-library-items` — Add one or more items to the current user's library.
- `spotify-pp-cli me save-shows-user` — Save one or more shows to current Spotify user's library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me save-tracks-user` — Save one or more tracks to the current user's 'Your Music' library. **Note:** This endpoint is deprecated.
- `spotify-pp-cli me seek-to-position-in-currently-playing-track` — Seeks to the given position in the user’s currently playing track.
- `spotify-pp-cli me set-repeat-mode-on-users-playback` — Set the repeat mode for the user's playback. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me set-volume-for-users-playback` — Set the volume for the user’s current playback device. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me skip-users-playback-to-next-track` — Skips to next track in the user’s queue. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me skip-users-playback-to-previous-track` — Skips to previous track in the user’s queue. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me start-a-users-playback` — Start a new context or resume current playback on the user's active device.
- `spotify-pp-cli me toggle-shuffle-for-users-playback` — Toggle shuffle on or off for user’s playback. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me transfer-a-users-playback` — Transfer playback to a new device and optionally begin playback. This API only works for users who have Spotify Premium.
- `spotify-pp-cli me unfollow-artists-users` — Remove the current user as a follower of one or more artists or other Spotify users.

**playlists** — Manage playlists

- `spotify-pp-cli playlists change-details` — Change a playlist's name and public/private state. (The user must, of course, own the playlist.)
- `spotify-pp-cli playlists get` — Get a playlist owned by a Spotify user.

**recommendations** — Manage recommendations

- `spotify-pp-cli recommendations get` — Recommendations are generated based on the available information for a given seed entity and matched against similar
- `spotify-pp-cli recommendations get-genres` — Retrieve a list of available genres seed parameter values for

**shows** — Manage shows

- `spotify-pp-cli shows get-a` — Get Spotify catalog information for a single show identified by its unique Spotify ID.
- `spotify-pp-cli shows get-multiple` — Get Spotify catalog information for several shows based on their Spotify IDs.

**spotify_web_search** — Manage spotify web search

- `spotify-pp-cli spotify-web-search` — Get Spotify catalog information about albums, artists, playlists, tracks, shows

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

## Auth Setup

Run `spotify-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
spotify-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `SPOTIFY_OAUTH_2_0` as an environment variable.

Run `spotify-pp-cli doctor` to verify setup.

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
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and use `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `SPOTIFY_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `SPOTIFY_CONFIG_DIR`, `SPOTIFY_DATA_DIR`, `SPOTIFY_STATE_DIR`, `SPOTIFY_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `SPOTIFY_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `spotify-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

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

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `SPOTIFY_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `SPOTIFY_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
spotify-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
spotify-pp-cli feedback --stdin < notes.txt
spotify-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `SPOTIFY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPOTIFY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled or recurring agent reuses the same saved flags while providing different input each run.

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
