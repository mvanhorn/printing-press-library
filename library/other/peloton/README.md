# peloton-pp-cli

Workout, ride, and music history from your Peloton account in the terminal.

The first catalog CLI to harvest the Auth0 SPA bearer token from the
`members.onepeloton.com` browser session, then mirror your history
(workouts → rides → liked songs) into a local SQLite store with FTS5 search.

## Fragility notice

Peloton ships **no public API**. Every endpoint and the Auth0 localStorage
harvest are reverse-engineered from members.onepeloton.com SPA traffic.
Endpoint paths, response shapes, the Auth0 client_id, the localStorage key
format, and the login form selectors can change unannounced — expect
occasional breakage and patch as needed. If a previously-working command
starts returning HTTP errors or unexpected JSON shapes, the upstream
likely shifted; open an issue or send a patch.

## Quick start

```bash
# 1. Install (Go 1.26.3 or newer)
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-cli@latest

# 2. Sign in. Spawns a Chrome window; sign in normally — the CLI
#    extracts the Auth0 bearer token from localStorage.
export PELOTON_USERNAME='you@example.com' PELOTON_PASSWORD='…'   # optional, prefills form
peloton-pp-cli auth login

# 3. Confirm
peloton-pp-cli me

# 4. Browse, sync, search
peloton-pp-cli workouts list --limit 10
peloton-pp-cli sync --limit 500          # mirror to ~/.local/share/peloton-pp-cli/peloton.db
peloton-pp-cli search 'cody rigsby'      # FTS5 over the local mirror
```

## Commands

| Command | Purpose |
|---|---|
| `auth login` | Spawn Chrome, sign in, harvest the Auth0 SPA token |
| `auth status` | Show whether a token is saved and how old it is |
| `me [--refresh]` | Print cached identity; `--refresh` re-fetches `/api/me` |
| `workouts list [--limit N]` | Recent workouts, newest-first |
| `workouts show <id>` | One workout's full details |
| `ride show <ride-id>` | Ride metadata + playlist (song order, artists, liked-flag) |
| `discoveries [--limit N]` | Liked-in-class songs across recent rides, deduped with `times_played` |
| `sync [--limit N] [--full] [--no-rides]` | Mirror workouts (and ride playlists) into local SQLite |
| `search <query> [--limit N]` | FTS5 over synced workouts + songs, interleaved by bm25 |
| `version` | Print version |

Every list/show/search command auto-emits JSON when stdout is not a TTY,
or with `--json`. `--compact` projects to high-gravity fields and one-line
JSON (typical 60–80% token reduction). All commands return typed exit
codes (0 / 2 / 3 / 4 / 5 / 7) — see SKILL.md.

## How auth works

`auth login` uses [chromedp](https://github.com/chromedp/chromedp) to spawn
a Chrome window pointed at `members.onepeloton.com/login`. Once you sign
in, Auth0 caches the OAuth response in `localStorage` under a key shaped
like:

```
@@auth0spajs@@::<client_id>::https://api.onepeloton.com/::openid offline_access
```

The CLI scans `localStorage` for that prefix, reads the `body.access_token`,
and writes it to `~/.config/peloton-pp-cli/config.toml` (mode 0600). The
client_id is discovered at runtime from the localStorage key, not
hardcoded — so a Peloton client rotation doesn't break the harvester.

The Chrome profile persists at `~/.config/peloton-pp-cli/chrome/`, so once
you've signed in once, subsequent `auth login` calls reuse session cookies
and finish in seconds (still need a fresh token because Auth0 expires the
access token after about an hour).

## How sync works

`peloton sync` walks `/api/user/{id}/workouts` newest-first, upserts every
workout into the local SQLite store, then by default fetches
`/api/ride/{id}/details` for every ride not already hydrated. Concurrency
is capped at 4 in flight (matches what the SPA does when prefetching
adjacent ride pages).

The walk is **incremental**: pagination stops the first time a page is
fully contained in the already-stored ids — Peloton ships
reverse-chronological, so the rest of the feed is older still. Use
`--full` to disable the early-stop and walk every page up to `--limit`.

The schema (`internal/store/store.go`) is hand-rolled relational, not
the generator's generic `resources(id, type, data JSON)` bag:
`workouts → rides → ride_songs → songs`, with FTS5 virtual tables over
`workouts(title, instructor)` and `songs(title, artists, album)`.
Driver is [`modernc.org/sqlite`](https://gitlab.com/cznic/sqlite) (pure
Go, no CGO) so `go install` works on stock toolchains.

## Files this CLI writes

| Path | What |
|---|---|
| `~/.config/peloton-pp-cli/config.toml` | Bearer token + cached user_id/username (mode 0600) |
| `~/.config/peloton-pp-cli/chrome/` | Persistent Chrome profile for `auth login` |
| `~/.local/share/peloton-pp-cli/peloton.db` | SQLite store populated by `sync` |

Delete any of these to start fresh; nothing is sent off-machine besides
authenticated requests to `api.onepeloton.com`.

## Known gaps

- **No mutation surface.** This CLI is read-only. It cannot start
  workouts, mark workouts complete, like songs, follow members, or
  change any account state. Mutations are out of scope.
- **No music-stream URL.** The ride playlist surfaces title, artists,
  album, and Peloton's in-class liked-flag, but not the actual streaming
  URL — Peloton stores tracks behind their own catalog and the SPA
  fetches a per-session `stream_url` we don't surface.
- **No FTP / power-zone / cycling-segment data.** The trimmed `Workout`
  shape drops 50+ fields the SPA uses for the in-class UI. Add what you
  need to `internal/client/client.go` if a downstream workflow wants more.
- **No multi-account.** Token + db live in fixed config paths; running
  against more than one Peloton account at the same time isn't supported.

## Install

### Via the Printing Press installer (recommended)

```bash
npx -y @mvanhorn/printing-press install peloton --cli-only
```

Adds the CLI to `$GOPATH/bin` and installs the agent skill to your
default skills directory.

### Via `go install`

```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-cli@latest
```

### MCP server

Once the MCP binary lands, register it with Claude Code:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-mcp@latest
claude mcp add peloton-pp-mcp -- peloton-pp-mcp
```

The MCP server expects `auth login` to have already saved a token; it
cannot itself drive the browser.

## License

Apache-2.0. See `LICENSE` and `NOTICE`.
