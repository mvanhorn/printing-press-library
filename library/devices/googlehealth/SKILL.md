---
name: pp-googlehealth
description: "Printing Press CLI for Googlehealth. View and manage health and fitness metrics and measurement data from Fitbit, Pixel Watch"
author: "ryanc00per"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - googlehealth-pp-cli
    install:
      - kind: go
        bins: [googlehealth-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/devices/googlehealth/cmd/googlehealth-pp-cli
---

# Googlehealth — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `googlehealth-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install googlehealth --cli-only
   ```
2. Verify: `googlehealth-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/devices/googlehealth/cmd/googlehealth-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

View and manage health and fitness metrics and measurement data from Fitbit, Pixel Watch, and other third-party devices and apps.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.
- **`trends`** — Per-metric trailing rolling-average trend lines with net first-to-last delta, de-noising day-to-day scale-weight and resting-HR whiplash
- **`streaks`** — Current and longest consecutive-calendar-day streaks where a metric met a goal (e.g. 10k steps, resting HR under 60), with calendar gaps breaking the run
- **`correlate`** — Pearson correlation plus best-lag scan between any two daily metrics (steps vs resting HR, sleep vs HRV) over locally synced history
- **`sync`** — Mirror Google Health data points across data types into a local SQLite store for offline analysis
- **`search`** — FTS5 full-text search across locally synced Google Health records

## Recipes

### Step-goal streak

```bash
googlehealth-pp-cli streaks --metric steps --threshold 10000 --op ">=" --json
```

### Resting-HR trend

```bash
googlehealth-pp-cli trends --metric daily-resting-heart-rate --window 14 --json
```

### Live data points for a metric

```bash
googlehealth-pp-cli users data-types list me steps --json
```

## Command Reference

**projects** — Manage projects


**users** — Manage users



### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
googlehealth-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `googlehealth-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
googlehealth-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `GOOGLEHEALTH_OAUTH2C` as an environment variable.

Run `googlehealth-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  googlehealth-pp-cli --help
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
googlehealth-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
googlehealth-pp-cli feedback --stdin < notes.txt
googlehealth-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/googlehealth-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLEHEALTH_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLEHEALTH_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
googlehealth-pp-cli profile save briefing --json
googlehealth-pp-cli --profile briefing doctor
googlehealth-pp-cli profile list --json
googlehealth-pp-cli profile show briefing
googlehealth-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `googlehealth-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/devices/googlehealth/cmd/googlehealth-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add googlehealth-pp-mcp -- googlehealth-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which googlehealth-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   googlehealth-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `googlehealth-pp-cli <command> --help`.
