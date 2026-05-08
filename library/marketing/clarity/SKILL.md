---
name: pp-clarity
description: "Generate and audit Microsoft Clarity browser instrumentation from the terminal. Trigger phrases: `generate a Clarity snippet`, `audit Clarity instrumentation`, `add Microsoft Clarity identify call`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clarity-pp-cli
    install:
      - kind: go
        bins: [clarity-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/clarity/cmd/clarity-pp-cli
---

# PP Clarity — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clarity-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install clarity --cli-only
   ```
2. Verify: `clarity-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/clarity/cmd/clarity-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Microsoft Clarity's client API is made of JavaScript calls and HTML attributes, so this CLI treats it as an instrumentation assistant instead of a fake REST wrapper. It can fetch the public tag script, render copy-safe snippets, and audit local HTML for the calls Microsoft documents.

## When to Use This CLI

Use this CLI when you need to add, review, or explain Microsoft Clarity browser instrumentation. It is strongest for generating snippets and auditing local HTML; it is not a replacement for the Clarity dashboard.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Instrumentation authoring
- **`snippet install`** — Render a complete Clarity tracking snippet for a project ID, plus focused snippets for every documented client API call.

  _Use this when adding Clarity to a site or handing implementation-ready code to another agent._

  ```bash
  clarity-pp-cli snippet install abc123 --format html
  ```

### Instrumentation review
- **`audit html`** — Inspect an HTML file for a Clarity tag script, masking attributes, and common window.clarity client API calls.

  _Use this before shipping page changes that are supposed to include Clarity instrumentation._

  ```bash
  clarity-pp-cli audit html ./index.html --json --select found_project_id,calls
  ```

## Command Reference

**tag** — Inspect the Microsoft Clarity tracking tag script

- `clarity-pp-cli tag <project_id>` — Fetch the Clarity tracking tag script for a project ID


**Hand-written commands**

- `clarity-pp-cli snippet install` — Render the tracking snippet for a Clarity project ID
- `clarity-pp-cli snippet consent` — Render the Cookie consent client API call
- `clarity-pp-cli snippet identify` — Render the custom identifiers client API call
- `clarity-pp-cli snippet set` — Render the custom tags client API call
- `clarity-pp-cli snippet event` — Render the custom event client API call
- `clarity-pp-cli snippet upgrade` — Render the session-priority client API call
- `clarity-pp-cli snippet mask` — Render Clarity mask or unmask HTML attributes
- `clarity-pp-cli audit html` — Check an HTML file for a Clarity install snippet and client API calls
- `clarity-pp-cli insights live` — Fetch Microsoft Clarity Data Export API live insights


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clarity-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Install Clarity

```bash
clarity-pp-cli snippet install abc123 --format html
```

Produces the tracking snippet that belongs in the page head.

### Add a custom event

```bash
clarity-pp-cli snippet event newsletterSignup
```

Produces the documented event call for a named user action.

### Audit only high-gravity fields

```bash
clarity-pp-cli audit html ./index.html --json --select found_project_id,calls,mask_count,unmask_count
```

Narrows audit output for agent consumption.

## Auth Setup

Client-side snippet commands only need the public Clarity project ID.

For Data Export API reads, set a token in the environment. Do not paste token values into chat or commit them to files:

```bash
export PP_CLARITY_API_TOKEN="..."
clarity-pp-cli insights live --days 1 --dimension OS --json
```

For local agent testing, prefer the local token file:

```bash
mkdir -p ~/.config/clarity-pp-cli
printf '%s' 'YOUR_TOKEN_HERE' > ~/.config/clarity-pp-cli/api-token
chmod 600 ~/.config/clarity-pp-cli/api-token
```

The command also accepts `MICROSOFT_CLARITY_API_TOKEN`, `CLARITY_API_TOKEN`, or `PP_CLARITY_API_TOKEN_FILE`.

Run `clarity-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clarity-pp-cli tag mock-value --agent --select id,name,status
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
clarity-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clarity-pp-cli feedback --stdin < notes.txt
clarity-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.clarity-pp-cli/feedback.jsonl`. They are never POSTed unless `CLARITY_CLIENT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLARITY_CLIENT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clarity-pp-cli profile save briefing --json
clarity-pp-cli --profile briefing tag mock-value
clarity-pp-cli profile list --json
clarity-pp-cli profile show briefing
clarity-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `clarity-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/clarity/cmd/clarity-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add clarity-pp-mcp -- clarity-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which clarity-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clarity-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clarity-pp-cli <command> --help`.
