---
name: pp-ticktick
description: "Every TickTick surface — tasks, habits, focus, daily notes — with a corruption-proof daily-note contract and offline analytics no other TickTick tool has. Trigger phrases: `pull up my agenda`, `update my daily note`, `log a focus session`, `check my habit streaks`, `gather my week in review`, `use ticktick`, `run ticktick`."
author: "Harvey The AI Guy"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ticktick-pp-cli
    install:
      - kind: go
        bins: [ticktick-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/ticktick/cmd/ticktick-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/ticktick/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See the repository agent guide, section "Generated artifacts: registry.json, cli-skills/". -->

# TickTick — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ticktick-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install ticktick --cli-only
   ```
2. Verify: `ticktick-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.5 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/ticktick/cmd/ticktick-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Covers both TickTick APIs: the official Open API and the richer internal V2 surface (habits, focus records, completed-task history, batch ops). The 'note edit' command encodes a field-whitelist write contract that cannot corrupt TEXT-kind daily notes, and the local SQLite mirror powers 'agenda', 'review', 'habits streaks', and 'focus stats' — ritual-shaped commands that replace multi-call fan-outs.

## When to Use This CLI

Reach for this CLI whenever an agent task touches TickTick daily notes, habits, focus sessions, or weekly reviews — the surfaces the official API does not expose. It is the safe write path for daily notes and the bounded read path for briefings and reviews.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI to edit TickTick calendar events synced from external calendars — that data is read-only in the V2 API
- Do not use raw 'tasks update' on a daily note; use 'note edit'
- Do not use this CLI for Dida365 (China) accounts without changing the base URL — endpoints differ

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Corruption-proof writes
- **`note edit`** — Edit your daily note's content with a corruption-proof contract — the command can never flip the note's task kind or break its subtasks.

  _Use this instead of a raw task update whenever the target is a daily note; raw updates through generic tools have corrupted notes before._

  ```bash
  ticktick-pp-cli note edit --date today --append "20:15 wrapped the printing-press build" --dry-run
  ```

### Local state that compounds
- **`agenda`** — One command returns today's tasks, habits with checkin state, and focus sessions together.

  _Reach for this at session start instead of fanning out list calls; one bounded response covers the whole briefing._

  ```bash
  ticktick-pp-cli agenda --json --select tasks.title,habits.name
  ```
- **`review`** — Gather a week of completed tasks, daily notes, focus totals, and habit checkins as one structured pack ready for synthesis.

  _Use for weekly-review synthesis instead of paging completed tasks and re-reading seven daily notes by hand._

  ```bash
  ticktick-pp-cli review --since 7d --json
  ```
- **`habits streaks`** — Current streak, longest streak, and at-risk-today flags for every habit.

  _Answers 'which habits are about to break' without hand-rolling date math over raw checkins._

  ```bash
  ticktick-pp-cli habits streaks --json
  ```
- **`focus stats`** — Focus/pomodoro time aggregated per day or per project over any window.

  _Use for weekly focus totals instead of scraping raw focus records._

  ```bash
  ticktick-pp-cli focus stats --since 7d --json
  ```

## Command Reference

**completed** — Completed task history

- `ticktick-pp-cli completed` — Completed tasks in a date window

**focus** — Focus / pomodoro records

- `ticktick-pp-cli focus` — Focus and pomodoro records, most recent first

**habits** — Habits and habit checkins

- `ticktick-pp-cli habits checkin` — Upsert habit checkins (batch add/update)
- `ticktick-pp-cli habits checkins` — Query habit checkins after a date stamp
- `ticktick-pp-cli habits list` — List all habits

**projects** — Projects (lists)

- `ticktick-pp-cli projects` — List all projects

**tags** — Tags

- `ticktick-pp-cli tags` — List all tags

**tasks** — Tasks — all uncompleted tasks via the V2 sync surface, single-task get, and batch mutations

- `ticktick-pp-cli tasks batch` — Batch create/update/delete tasks (arrays of task objects; updates must carry id, projectId
- `ticktick-pp-cli tasks get` — Get a single task by id (full JSON incl. kind, etag, childIds)
- `ticktick-pp-cli tasks list` — List all uncompleted tasks (from the V2 sync check)

**user** — Account profile

- `ticktick-pp-cli user` — Authenticated user profile


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ticktick-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Morning briefing

```bash
ticktick-pp-cli agenda --json --agent
```

One bounded response with today's tasks, habit state, and focus sessions for a session-start briefing.

### Log a focus block to the daily note

```bash
ticktick-pp-cli note edit --date today --append "15:00-15:45 client audit prep"
```

Timestamped append through the corruption-proof write path.

### Friday review pack, narrowed for agents

```bash
ticktick-pp-cli review --since 7d --agent --select completed.title,notes.content,focus_totals.by_day
```

Deeply nested week pack narrowed with --select so synthesis doesn't drown in raw JSON.

### Which habits are about to break

```bash
ticktick-pp-cli habits streaks --json --select name,current_streak,at_risk
```

Streak math over synced checkins with only the decision-relevant fields.

### Weekly focus hours by project

```bash
ticktick-pp-cli focus stats --since 7d --by project --json
```

Aggregated pomodoro/focus durations from the local mirror.

## Auth Setup

Two tiers. The V2 internal API (habits, focus, daily notes, batch) authenticates with a session token: set TICKTICK_USERNAME and TICKTICK_PASSWORD and the CLI signs on automatically, or set TICKTICK_SESSION_TOKEN with the 't' cookie from a logged-in browser session (required for Google-SSO accounts without a password). The optional V1 Open API tier uses TICKTICK_API_TOKEN (OAuth bearer from developer.ticktick.com). Requests send a browser-like User-Agent because the API returns 503 to default clients.

Run `ticktick-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ticktick-pp-cli completed --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

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

- Use `--home <dir>` for one invocation, or set `TICKTICK_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `TICKTICK_CONFIG_DIR`, `TICKTICK_DATA_DIR`, `TICKTICK_STATE_DIR`, `TICKTICK_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `TICKTICK_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `ticktick-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "ticktick": {
        "command": "ticktick-pp-mcp",
        "env": {
          "TICKTICK_HOME": "/srv/ticktick"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `TICKTICK_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `TICKTICK_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ticktick-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ticktick-pp-cli feedback --stdin < notes.txt
ticktick-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `TICKTICK_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TICKTICK_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ticktick-pp-cli profile save briefing --json
ticktick-pp-cli --profile briefing completed
ticktick-pp-cli profile list --json
ticktick-pp-cli profile show briefing
ticktick-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ticktick-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/ticktick/cmd/ticktick-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add ticktick-pp-mcp -- ticktick-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ticktick-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ticktick-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ticktick-pp-cli <command> --help`.
