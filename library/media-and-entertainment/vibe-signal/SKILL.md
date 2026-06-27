---
name: pp-vibe-signal
description: "One question, every low-friction source: a recency-aware, cited signal report instead of ten raw source dumps. Trigger phrases: `what are people saying about X`, `vibe check on X`, `trend report for X`, `what's the conversation on X right now`, `use vibe-signal`, `run vibe-signal`."
author: "not0xjarvis"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vibe-signal-pp-cli
    install:
      - kind: go
        bins: [vibe-signal-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/cmd/vibe-signal-pp-cli
---

# Vibe Signal — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vibe-signal-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install vibe-signal --cli-only
   ```
2. Verify: `vibe-signal-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/cmd/vibe-signal-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Vibe Signal composes the catalog's source surfaces into a single editorial research loop. Ask what people are saying now about a topic and get themes backed by raw evidence (report), pull the citable items behind a claim (evidence), and see which sources are covered (sources list). v1 ships the no-auth sources: Hacker News and Techmeme.

## When to Use This CLI

Use Vibe Signal when an agent or writer needs the current cross-source conversation on a topic, product, company, or person, with citable evidence and recency awareness. It is the editorial layer over single-source CLIs.

## Anti-triggers

Do not use this CLI for:
- Do not use it to manage or post to any source — it is read-only.
- Do not use it for a single source's full archive; use that source's own CLI.
- Do not use it for sources requiring credentials in v1 (Product Hunt, YouTube) — those are deferred.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Composed editorial workflow
- **`report`** — Ask one question across Hacker News and Techmeme and get a recency-aware signal report with per-source coverage.

  _Reach for this when you need the current conversation on a topic across sources, not one source's raw dump._

  ```bash
  vibe-signal-pp-cli report "AI browser agents" --window 30d --json
  ```
- **`evidence`** — List the raw, cited items (post URL, author, timestamp, points, comments) backing a topic from a chosen source.

  _Reach for this when you need to quote or link real posts behind a claim, not a paraphrase._

  ```bash
  vibe-signal-pp-cli evidence "AI browser agents" --source hackernews --limit 20 --json
  ```
- **`sources list`** — Show which sources are wired in, their auth needs, and which command syncs them.

  _Reach for this to see coverage and auth requirements before running a report._

  ```bash
  vibe-signal-pp-cli sources list --json
  ```

## Command Reference

**hn** — Hacker News source: search (Algolia) and item lookup (Firebase)

- `vibe-signal-pp-cli hn item` — Get a single HN item (story/comment) with score and comment count
- `vibe-signal-pp-cli hn relevance` — Search HN by relevance (popularity-ranked) for a topic
- `vibe-signal-pp-cli hn stories` — Search HN stories by recency for a topic


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vibe-signal-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Cross-source report as JSON for ranking

```bash
vibe-signal-pp-cli report "local-first software" --window 14d --json --select query,themes,coverage
```

Narrow the report envelope to the fields a downstream ranker needs.

### Pull citable HN evidence

```bash
vibe-signal-pp-cli evidence "local-first software" --source hackernews --limit 15 --json
```

Get raw items (url, author, points, comments) behind the topic.

### Check coverage before reporting

```bash
vibe-signal-pp-cli sources list --json
```

Confirm which sources are wired and free before running a report.

## Auth Setup

No authentication required.

Run `vibe-signal-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vibe-signal-pp-cli hn item mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `VIBE_SIGNAL_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `VIBE_SIGNAL_CONFIG_DIR`, `VIBE_SIGNAL_DATA_DIR`, `VIBE_SIGNAL_STATE_DIR`, `VIBE_SIGNAL_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `VIBE_SIGNAL_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `vibe-signal-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "vibe-signal": {
        "command": "vibe-signal-pp-mcp",
        "env": {
          "VIBE_SIGNAL_HOME": "/srv/vibe-signal"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `VIBE_SIGNAL_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `VIBE_SIGNAL_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
vibe-signal-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vibe-signal-pp-cli feedback --stdin < notes.txt
vibe-signal-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `VIBE_SIGNAL_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VIBE_SIGNAL_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vibe-signal-pp-cli profile save briefing --json
vibe-signal-pp-cli --profile briefing hn item mock-value
vibe-signal-pp-cli profile list --json
vibe-signal-pp-cli profile show briefing
vibe-signal-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `vibe-signal-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/vibe-signal/cmd/vibe-signal-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vibe-signal-pp-mcp -- vibe-signal-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vibe-signal-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vibe-signal-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vibe-signal-pp-cli <command> --help`.
