---
name: pp-peloton
description: "Workout, ride, and music history from your Peloton account in the terminal. First catalog CLI to harvest the Auth0 SPA bearer token from a browser session, then mirror your history (workouts + ride playlists + liked songs) into a local SQLite store with FTS5 search. Reverse-engineered from members.onepeloton.com SPA traffic — Peloton ships no public API, expect occasional breakage. Trigger phrases: `list my Peloton workouts`, `what did I ride yesterday`, `find songs I liked on Peloton`, `search my Peloton history`, `sync my Peloton workouts`, `what did Cody Rigsby teach me`, `use peloton`, `run peloton`."
author: "Todd Dailey"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - peloton-pp-cli
    install:
      - kind: go
        bins: [peloton-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-cli
---

# Peloton — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `peloton-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install peloton --cli-only
   ```
2. Verify: `peloton-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

## When to Use This CLI

Use this CLI when an agent or user wants structured access to a Peloton account's workout, ride, and music history outside the members.onepeloton.com browser. Good for: pulling recent workouts as JSON, listing the songs that played in a specific ride, finding every liked-in-class song across a window of workouts, or searching the local mirror for an instructor / song / artist by name. Read-only against Peloton — this CLI does not start workouts, mark workouts complete, or change any account state.

## Fragility Notice

Peloton ships **no public API**. Every endpoint and the Auth0 localStorage harvest are reverse-engineered from members.onepeloton.com SPA traffic, which means:

- The bearer-token harvest reads `localStorage` under the Auth0 SPA cache key (`@@auth0spajs@@::<client_id>::<audience>`). Peloton can change the client_id, the cache key format, the audience, or the login form selectors without notice.
- Endpoint paths (`/api/me`, `/api/user/{id}/workouts`, `/api/workout/{id}`, `/api/ride/{id}/details`) and response shapes (`ride.id`, `ride.title`, `playlist.songs[].liked`, etc.) are SPA implementation details, not contract.
- Auth0 tokens last about an hour; re-run `peloton-pp-cli auth login` when one expires.

If a command starts returning HTTP errors or unexpected shapes after a previously-working session, the upstream likely shifted. File an issue or open a patch — this CLI is best-effort against a moving target.

## Command Reference

### Auth

- `peloton-pp-cli auth login` — Spawn Chrome, open `members.onepeloton.com/login`, harvest the Auth0 SPA access token from `localStorage` once you sign in, save to `~/.config/peloton-pp-cli/config.toml`. Pre-fills the form from `PELOTON_USERNAME` / `PELOTON_PASSWORD` if set; otherwise sign in by hand. Profile persists at `~/.config/peloton-pp-cli/chrome/` so subsequent logins are quick.
- `peloton-pp-cli auth status` — Show whether a saved token exists and how old it is. Exits 4 if no token.

### Identity

- `peloton-pp-cli me` — Print the cached identity (user_id, username, token age) without spawning Chrome. Add `--refresh` to re-hit `/api/me` and update the cache.

### Workouts

- `peloton-pp-cli workouts list` — Recent workouts, newest-first. `--limit N` (default 50). Pages `/api/user/{id}/workouts` with the same ride/instructor joins the SPA uses.
- `peloton-pp-cli workouts show <workout-id>` — One workout with the same shape as a list element. Useful for piping into `jq` after grabbing an id from list output.

### Ride detail

- `peloton-pp-cli ride show <ride-id>` — Ride metadata + playlist (song order, artists, liked-flag, start-time offsets). Pair with `workouts show`: the workout's `ride_id` is the input here. Some on-demand rides ship empty playlists (instructor talk-only) — that's a normal `"songs": []`, not an error.

### Discoveries

- `peloton-pp-cli discoveries` — Walks your most-recent workouts (default `--limit 30`), fetches each ride's playlist in parallel, collects songs flagged `liked=true` in-class, dedupes by song id with a `times_played` counter. Likes are stored Peloton-side per ride playback, not per-song globally — this is the closest stand-in for "songs I discovered through Peloton."

### Local store

- `peloton-pp-cli sync` — Mirror new workouts (and their ride playlists) into a local SQLite store at `~/.local/share/peloton-pp-cli/peloton.db`. Incremental: stops paginating once a page is fully contained in already-stored ids. `--full` disables the early-stop. `--no-rides` skips playlist hydration (faster). `--limit N` caps the workout walk (default 200).
- `peloton-pp-cli search <query>` — FTS5 across the synced store: workouts (title + instructor) and songs (title + artists + album), interleaved by bm25. Phrases (`"low impact"`), prefixes (`cure*`), and `NEAR( )` all work. Run `sync` first to populate the store.

### Version

- `peloton-pp-cli version` — Print version. (`--version` works too.)

## Recipes

### First-time setup
```bash
export PELOTON_USERNAME='you@example.com' PELOTON_PASSWORD='…'
peloton-pp-cli auth login           # or just `auth login` and sign in by hand
peloton-pp-cli me                   # confirm identity / token freshness
peloton-pp-cli sync --limit 500     # full backfill of the last 500 workouts + their playlists
```

### Browse recent rides as JSON
```bash
peloton-pp-cli workouts list --limit 10 --compact | jq '.[] | {date: .workout_date, title, instructor: .instructor}'
```

### Liked-songs digest from the last 60 rides
```bash
peloton-pp-cli discoveries --limit 60 --compact | jq '.[] | "\(.title) — \(.artists | join(", "))"'
```

### Find every workout featuring an instructor
```bash
peloton-pp-cli sync                  # ensure local store is fresh
peloton-pp-cli search 'cody rigsby' --json | jq '.[] | select(.kind=="workout")'
```

### Pull the playlist of a specific workout
```bash
RIDE=$(peloton-pp-cli workouts show <workout-id> --compact | jq -r .ride_id)
peloton-pp-cli ride show "$RIDE" --json
```

## Auth Setup

Auth is bearer-token-via-browser-harvest. `peloton-pp-cli auth login` spawns a chromedp-controlled Chrome window pointed at `members.onepeloton.com/login`. If `PELOTON_USERNAME` and `PELOTON_PASSWORD` are set, the form is pre-filled — you still complete sign-in (Auth0 may show a CAPTCHA, 2FA, or a "verify it's you" interstitial; doing it by hand is fine). The CLI reads the Auth0 SPA access token from `localStorage` once it lands and writes it to `~/.config/peloton-pp-cli/config.toml`.

Tokens last about an hour. `peloton-pp-cli auth status` shows the saved token's age. Re-run `auth login` when expired — the persistent Chrome profile at `~/.config/peloton-pp-cli/chrome/` keeps session cookies, so subsequent logins finish in seconds.

`peloton-pp-cli auth login --headless` runs Chrome headless (rare; use only when you've validated headless completes the Auth0 dance — many CAPTCHAs require a visible window).

## Agent Mode

This is a hand-rolled CLI; it does **not** ship the generator-template `--agent` / `--select` / `--deliver` / `--profile` / feedback suite. The agent contract is leaner:

- **Auto-JSON when piped.** Every list/show command emits indented JSON to stdout when stdout is not a TTY. On a TTY, output is human-friendly text.
- **`--json`** forces JSON even on a TTY.
- **`--compact`** projects to high-gravity fields only and emits one-line JSON. Implies `--json`. Typical token reduction is 60–80% on `workouts list` and `ride show`.
- **stderr for progress, stdout for data.** Sync and discoveries print progress on stderr; pipe stdout cleanly into `jq`.
- **Typed exit codes** (see below) so callers branch on `$?` without parsing strings.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found (e.g., unknown workout id, 404 on ride) |
| 4 | Authentication required (no saved token, or upstream 401/403) |
| 5 | API error (upstream issue, non-2xx other than 401/403/404/429) |
| 7 | Rate limited (HTTP 429) |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `peloton-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/peloton/cmd/peloton-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add peloton-pp-mcp -- peloton-pp-mcp
   ```
3. Verify: `claude mcp list`

Note: the MCP server expects `peloton-pp-cli auth login` to have already saved a token to `~/.config/peloton-pp-cli/config.toml`. The MCP cannot itself drive the browser — bootstrap the token from a CLI shell first.

## Direct Use

1. Check if installed: `which peloton-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Bootstrap auth once with `peloton-pp-cli auth login`. Confirm with `peloton-pp-cli me`.
3. Match the user query to the best command from the Command Reference above. Pipe stdout into `jq` for structured downstream consumption.
4. If you need many calls (e.g., walking history for analysis), run `peloton-pp-cli sync` once, then use `peloton-pp-cli search` and the local store rather than re-hitting the upstream every time.
