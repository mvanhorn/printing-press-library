---
name: pp-tv-time
description: "Every TV Time feature, plus an offline watch-history mirror, bulk episode logging Trigger phrases: `what airs tonight on tv time`, `mark this season watched`, `how many episodes did I watch this year`, `what should I watch next`, `use tv-time`, `run tv-time`."
author: "Jonas Ekerhovd"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tv-time-pp-cli
    install:
      - kind: go
        bins: [tv-time-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tv-time/cmd/tv-time-pp-cli
---

# TV Time — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tv-time-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install tv-time --cli-only
   ```
2. Verify: `tv-time-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tv-time/cmd/tv-time-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Drives 'what to watch tonight' and 'how am I doing this year' surfaces from the same TV Time account you already use on your phone.

## When to Use This CLI

Use this CLI to script or automate TV Time: log episodes in bulk, query your watch history offline, surface today's airings, or feed your show data into other tools. It is the right choice when an agent needs structured TV Time data or needs to mark episodes watched without the mobile app.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to stream or play video; it only tracks what you watch.
- Do not use it for shows you do not follow on TV Time; it operates on your account's data.
- Do not use it as a general TMDB/TVDB metadata source; it surfaces TV Time's own data.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Personal analytics
- **`stats`** — Roll up your watched history into episodes, estimated hours, and your most-watched shows — straight from /user/{user_id}/stats.

  _Pipe your watch totals into a dashboard, a homepage stat widget, or a year-in-review post._

  ```bash
  tv-time-pp-cli stats "$USER_ID" --json
  ```

### Calendar
- **`agenda`** — See what airs today or in the next N days for the shows you follow.

  _Drives a daily 'what's new' email, a Raycast widget, or a homepage 'on tonight' block._

  ```bash
  tv-time-pp-cli agenda "$USER_ID" --days 7
  ```

### Queue
- **`next`** — Pick the single best next episode to watch across all your in-progress shows.

  _Removes the choose-what-to-watch tax. Wire it into a 'one button to start tonight's show' flow._

  ```bash
  tv-time-pp-cli next "$USER_ID"
  ```
- **`backlog`** — Rank the shows you follow by how many unwatched episodes are piling up.

  _Tells you which shows are slipping. Useful for 'shows I should drop' or 'shows to catch up on this weekend'._

  ```bash
  tv-time-pp-cli backlog "$USER_ID" --limit 10
  ```
- **`since`** — (stub) Show what changed in your queue since a point in time. Not yet wired — requires a local sync mirror with historical snapshots that isn't implemented yet.

  _Stubbed for now; intent is to ship once the sync mirror lands._

  ```bash
  tv-time-pp-cli since --help
  ```

### Bulk write
- **`binge`** — (stub) Bulk-mark a whole season or show as watched in one command. Not yet wired — requires a season-episode listing from a sync mirror that isn't implemented yet.

  _Stubbed for now so the shape is visible; intent is to ship once the sync mirror lands._

  ```bash
  tv-time-pp-cli binge --help
  ```

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**episodes** — Your watch queue, history, and episode actions

- `tv-time-pp-cli episodes comment` — Comment on an episode
- `tv-time-pp-cli episodes delete-actor-vote` — Remove your actor vote on an episode
- `tv-time-pp-cli episodes delete-comment` — Delete one of your episode comments
- `tv-time-pp-cli episodes delete-reaction` — Remove your reaction from an episode
- `tv-time-pp-cli episodes mark-unwatched` — Mark an episode as not watched
- `tv-time-pp-cli episodes mark-watched` — Mark an episode as watched
- `tv-time-pp-cli episodes react` — React to an episode (star-meter). emotion_id: great=1 wow=3 ok=6 bad=7 good=8
- `tv-time-pp-cli episodes to-watch` — Episodes in your to-watch queue
- `tv-time-pp-cli episodes vote-actor` — Vote for an actor on an episode
- `tv-time-pp-cli episodes watched` — Episodes you have watched (paged, most recent first)

**session** — Authenticate against TV Time

- `tv-time-pp-cli session` — Sign in (HTTP Basic) and resolve your user id

**shows** — Search and browse TV shows

- `tv-time-pp-cli shows actors` — List actors for a show
- `tv-time-pp-cli shows followed` — Shows you follow
- `tv-time-pp-cli shows for-later` — Shows you saved for later
- `tv-time-pp-cli shows my-shows` — Your shows grouped by category (watching, up_to_date, finished, for_later, ...)
- `tv-time-pp-cli shows search` — Search shows by name

**user** — Your profile, stats, friends, calendar, and notifications

- `tv-time-pp-cli user badges` — Your earned badges
- `tv-time-pp-cli user calendar` — Calendar of upcoming airings for shows you follow
- `tv-time-pp-cli user friends` — Your friends list
- `tv-time-pp-cli user notifications` — Your notifications
- `tv-time-pp-cli user profile` — Your profile
- `tv-time-pp-cli user stats` — Your viewing stats


## Freshness Contract

This printed CLI owns bounded freshness only for registered store-backed read command paths. In `--data-source auto` mode, those paths check `sync_state` and may run a bounded refresh before reading local data. `--data-source local` never refreshes. `--data-source live` reads the API and does not mutate the local store. Set `TV_TIME_NO_AUTO_REFRESH=1` to skip the freshness hook without changing source selection.

When JSON output uses the generated provenance envelope, freshness metadata appears at `meta.freshness`. Treat it as current-cache freshness for the covered command path, not a guarantee of complete historical backfill or API-specific enrichment.

### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tv-time-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Year-in-review widget

```bash
tv-time-pp-cli stats "$USER_ID" --json | jq '.episodes_watched, .hours_watched'
```

Feed your watch totals into a homepage stat widget.

### What airs this week

```bash
tv-time-pp-cli agenda "$USER_ID" --days 7
```

Surface only what airs in the next seven days for the shows you follow.

### Shows I should drop

```bash
tv-time-pp-cli backlog "$USER_ID" --limit 5
```

The top 5 shows piling up unwatched — useful pruning signal.

## Auth Setup

TV Time has no public API. This CLI authenticates against the mobile backend (api2.tozelabs.com) with your account username and password over HTTP Basic, then resolves your user id via a signin handshake. Set TVTIME_USERNAME and TVTIME_PASSWORD, then run 'tv-time-pp-cli doctor' to confirm the handshake.

Run `tv-time-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tv-time-pp-cli shows search --q example-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tv-time-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tv-time-pp-cli feedback --stdin < notes.txt
tv-time-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/tv-time-pp-cli/feedback.jsonl`. They are never POSTed unless `TV_TIME_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TV_TIME_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
tv-time-pp-cli profile save briefing --json
tv-time-pp-cli --profile briefing shows search --q example-value
tv-time-pp-cli profile list --json
tv-time-pp-cli profile show briefing
tv-time-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `tv-time-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/tv-time/cmd/tv-time-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tv-time-pp-mcp -- tv-time-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tv-time-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tv-time-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tv-time-pp-cli <command> --help`.
