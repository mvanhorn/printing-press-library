---
name: pp-peloton
description: "Workout, ride, and music history from your Peloton account in the terminal. First catalog CLI to harvest the Auth0..."
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

Peloton workout, ride, and music history API. No public spec — reverse-engineered from the members.onepeloton.com Auth0 SPA. Endpoint paths and response shapes can shift unannounced; the auth bearer token is harvested from Auth0 SPA localStorage rather than a documented OAuth flow.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

- **`auth login`** — chromedp-driven Auth0 SPA token harvest. Spawns Chrome at `members.onepeloton.com/login`, lets the user sign in interactively, then extracts the bearer token from `localStorage` under `@@auth0spajs@@::<client_id>::https://api.onepeloton.com/::openid offline_access`. The Auth0 client_id is discovered at runtime so a Peloton client rotation doesn't break the harvester. Pre-fill the form via `PELOTON_USERNAME` / `PELOTON_PASSWORD` env (inline-scope only — see Auth Setup). Profile persists at `~/.config/peloton-pp-cli/chrome/` so subsequent sign-ins reuse cookies.

  ```bash
  peloton-pp-cli auth login --timeout 5m
  ```

- **`discoveries`** — list every in-class song you liked across recent rides, deduped by song id with a `times_played` counter. No Peloton UI surfaces this view; likes are stored Peloton-side per ride playback rather than per-song globally.

  ```bash
  peloton-pp-cli discoveries --limit 30 --agent
  ```

## Command Reference

**identity** — Authenticated user identity

- `peloton-pp-cli identity` — Get the authenticated user's identity (id, username, profile fields). Used implicitly by every workout query to...

**rides** — Ride metadata and playlists

- `peloton-pp-cli rides <rideID>` — Get a ride's metadata and playlist (song order, artists, in-class liked-flag, start-time offsets). The workout's...

**workouts** — Workout history (list, show)

- `peloton-pp-cli workouts get` — Get a single workout by id, with the same ride/instructor join surface as `list`.
- `peloton-pp-cli workouts list` — List the authenticated user's workouts, newest-first. Uses joins=ride,ride.instructor to pull the ride title,...


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
peloton-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Store your access token:

```bash
peloton-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `PELOTON_USERNAME` as an environment variable.

Run `peloton-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  peloton-pp-cli workouts list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

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
peloton-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
peloton-pp-cli feedback --stdin < notes.txt
peloton-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.peloton-pp-cli/feedback.jsonl`. They are never POSTed unless `PELOTON_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PELOTON_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
peloton-pp-cli profile save briefing --json
peloton-pp-cli --profile briefing workouts list mock-value
peloton-pp-cli profile list --json
peloton-pp-cli profile show briefing
peloton-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `peloton-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

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

## Direct Use

1. Check if installed: `which peloton-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   peloton-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `peloton-pp-cli <command> --help`.
