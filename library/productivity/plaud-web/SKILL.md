---
name: pp-plaud-web
description: "Printing Press CLI for Plaud Web. Unofficial Plaud Web endpoints observed from a user's signed-in Plaud Web session for organizing user-owned recordings."
author: "Stefan Erschwendner"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - plaud-web-pp-cli
    install:
      - kind: go
        bins: [plaud-web-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/plaud-web/cmd/plaud-web-pp-cli
---

# Plaud Web — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `plaud-web-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install plaud-web --cli-only
   ```
2. Verify: `plaud-web-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/plaud-web/cmd/plaud-web-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Unofficial Plaud Web endpoints observed from a user's signed-in Plaud Web session for organizing user-owned recordings.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Knowledge-work organization

- **`batch-rename`** — Rename multiple Plaud recordings from a JSON or CSV mapping after transcription or note synthesis.

  _Lets agents keep Plaud Web aligned with Obsidian, CRM, and meeting-note systems without manual timestamp cleanup._

  ```bash
  plaud-web-pp-cli batch-rename titles.json --dry-run
  ```
- **`move`** — Move one or more recordings into a Plaud folder/tag or back to unfiled.

  _Useful for client calls, event recordings, and presentation-prep batches._

  ```bash
  plaud-web-pp-cli move <recording-id> --folder-id <folder-id>
  ```

### Data portability

- **`export-audio`** — Resolve a short-lived Plaud audio URL and download the recording to a local file with a safe title-based filename.

  _Supports data portability and downstream transcript, archival, or review workflows._

  ```bash
  plaud-web-pp-cli export-audio <recording-id> --output-dir exports
  ```

## Command Reference

**file** — Manage file

- `plaud-web-pp-cli file get-detail` — Fetch detail metadata for one recording.
- `plaud-web-pp-cli file get-temporary-audio-url` — Fetch a short-lived audio download URL.
- `plaud-web-pp-cli file rename` — Rename one recording.
- `plaud-web-pp-cli file update-tags` — Move recordings into a folder/tag or back to unfiled.

**filetag** — Manage filetag

- `plaud-web-pp-cli filetag create-file-tag` — Create a Plaud folder/tag.
- `plaud-web-pp-cli filetag list-file-tags` — List Plaud folders/tags.
- `plaud-web-pp-cli filetag update-file-tag` — Rename or update a Plaud folder/tag.

**gsearch** — Manage gsearch

- `plaud-web-pp-cli gsearch` — Search recordings and generated note content.

**speaker** — Manage speaker

- `plaud-web-pp-cli speaker` — List cloud speaker labels when speaker cloud is enabled.

**user** — Manage user

- `plaud-web-pp-cli user` — Fetch recording statistics.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
plaud-web-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Use a Plaud Web session bearer token from the user's own signed-in browser session. Store it locally:

```bash
plaud-web-pp-cli auth set-token YOUR_PLAUD_WEB_TOKEN
```

Or set `PLAUD_WEB_BEARER_AUTH` as an environment variable. The value may be pasted with or without the leading `bearer ` prefix.

Never print, log, or commit Plaud bearer tokens, cookies, signed audio URLs, or downloaded recordings. Use this CLI only with the user's own Plaud account and recordings.

Run `plaud-web-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  plaud-web-pp-cli speaker --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
plaud-web-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
plaud-web-pp-cli feedback --stdin < notes.txt
plaud-web-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/plaud-web-pp-cli/feedback.jsonl`. They are never POSTed unless `PLAUD_WEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `PLAUD_WEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
plaud-web-pp-cli profile save briefing --json
plaud-web-pp-cli --profile briefing speaker
plaud-web-pp-cli profile list --json
plaud-web-pp-cli profile show briefing
plaud-web-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `plaud-web-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/plaud-web/cmd/plaud-web-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add plaud-web-pp-mcp -- plaud-web-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which plaud-web-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   plaud-web-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `plaud-web-pp-cli <command> --help`.
