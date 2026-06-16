---
name: pp-vapi
description: "Printing Press CLI for Vapi. Voice AI for developers."
author: ""
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vapi-pp-cli
    install:
      - kind: go
        bins: [vapi-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai/vapi/cmd/vapi-pp-cli
---

# Vapi — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vapi-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer into a user bin directory:
   ```bash
   npx -y @mvanhorn/printing-press-library install vapi --cli-only --bin-dir ~/.local/bin
   ```
2. Verify: `vapi-pp-cli --version`
3. Ensure `~/.local/bin` is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/ai/vapi/cmd/vapi-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Voice AI for developers.

## Command Reference

**assistant** — Manage assistant

- `vapi-pp-cli assistant create` — Create Assistant
- `vapi-pp-cli assistant find-all` — List Assistants
- `vapi-pp-cli assistant find-one` — Get Assistant
- `vapi-pp-cli assistant remove` — Delete Assistant
- `vapi-pp-cli assistant update` — Update Assistant

**call** — Manage call

- `vapi-pp-cli call create` — Create Call
- `vapi-pp-cli call delete-data` — Delete Call
- `vapi-pp-cli call find-all` — List Calls
- `vapi-pp-cli call find-one` — Get Call
- `vapi-pp-cli call update` — Update Call

**private outbound calling** — Personal Vapi call workflows

- `vapi-pp-cli dial` — Place or schedule outbound assistant calls with dry-run/yes guardrails. Records by default; pass `--no-record` to opt out.
- `vapi-pp-cli juno call` — Make a general delegated assistant call.
- `vapi-pp-cli juno reservation` — Make or modify a reservation.
- `vapi-pp-cli juno order` — Order items or gather an order-ready quote.
- `vapi-pp-cli juno quote` — Call one or more vendors for comparable quotes.
- `vapi-pp-cli juno followup` — Re-call or leave a message using a previous call context.
- `vapi-pp-cli juno status <call-id>` — Show call outcome, transcript, cost, and download the recording.
- `vapi-pp-cli juno setup` — Verify Juno assistant/phone readiness and save a reusable local profile.
- `vapi-pp-cli juno test-call` — Place a guarded recorded smoke-test call; use `--dry-run` before `--yes`.
- `vapi-pp-cli juno report` — Summarize recent Juno calls or explicit call IDs.
- `vapi-pp-cli juno phone-numbers` — List Vapi phone numbers with outbound readiness hints.
- `vapi-pp-cli juno assistant payload|create|update` — Preview, create, or update the default Juno assistant prompt/settings.

**campaign** — Manage campaign

- `vapi-pp-cli campaign create` — Create Campaign
- `vapi-pp-cli campaign find-all` — List Campaigns
- `vapi-pp-cli campaign find-one` — Get Campaign
- `vapi-pp-cli campaign remove` — Delete Campaign
- `vapi-pp-cli campaign update` — Update Campaign

**chat** — Manage chat

- `vapi-pp-cli chat create` — Creates a new chat with optional SMS delivery via transport field.
- `vapi-pp-cli chat create-open-aichat` — Create Chat (OpenAI Compatible)
- `vapi-pp-cli chat delete` — Delete Chat
- `vapi-pp-cli chat get` — Get Chat
- `vapi-pp-cli chat list` — List Chats

**eval** — Manage eval

- `vapi-pp-cli eval create` — Create Eval
- `vapi-pp-cli eval get` — Get Eval
- `vapi-pp-cli eval get-paginated` — List Evals
- `vapi-pp-cli eval get-run` — Get Eval Run
- `vapi-pp-cli eval get-runs-paginated` — List Eval Runs
- `vapi-pp-cli eval remove` — Delete Eval
- `vapi-pp-cli eval remove-run` — Delete Eval Run
- `vapi-pp-cli eval run` — Create Eval Run
- `vapi-pp-cli eval update` — Update Eval

**file** — Manage file

- `vapi-pp-cli file create` — Upload File
- `vapi-pp-cli file find-all` — List Files
- `vapi-pp-cli file find-one` — Get File
- `vapi-pp-cli file remove` — Delete File
- `vapi-pp-cli file update` — Update File

**observability** — Manage observability

- `vapi-pp-cli observability scorecard-create` — Create Scorecard
- `vapi-pp-cli observability scorecard-get` — Get Scorecard
- `vapi-pp-cli observability scorecard-get-paginated` — List Scorecards
- `vapi-pp-cli observability scorecard-remove` — Delete Scorecard
- `vapi-pp-cli observability scorecard-update` — Update Scorecard

**phone-number** — Manage phone number

- `vapi-pp-cli phone-number create` — Create Phone Number
- `vapi-pp-cli phone-number find-all` — List Phone Numbers
- `vapi-pp-cli phone-number find-all-paginated` — List Phone Numbers
- `vapi-pp-cli phone-number find-one` — Get Phone Number
- `vapi-pp-cli phone-number remove` — Delete Phone Number
- `vapi-pp-cli phone-number update` — Update Phone Number

**provider** — Manage provider

- `vapi-pp-cli provider resource-create-resource` — Create Provider Resource
- `vapi-pp-cli provider resource-delete-resource` — Delete Provider Resource
- `vapi-pp-cli provider resource-get-resource` — Get Provider Resource
- `vapi-pp-cli provider resource-get-resources-paginated` — List Provider Resources
- `vapi-pp-cli provider resource-update-resource` — Update Provider Resource

**reporting** — Manage reporting

- `vapi-pp-cli reporting insight-create` — Create Insight
- `vapi-pp-cli reporting insight-find-all` — Get Insights
- `vapi-pp-cli reporting insight-find-one` — Get Insight
- `vapi-pp-cli reporting insight-preview` — Preview Insight
- `vapi-pp-cli reporting insight-remove` — Delete Insight
- `vapi-pp-cli reporting insight-run` — Run Insight
- `vapi-pp-cli reporting insight-update` — Update Insight

**session** — Manage session

- `vapi-pp-cli session create` — Create Session
- `vapi-pp-cli session find-all-paginated` — List Sessions
- `vapi-pp-cli session find-one` — Get Session
- `vapi-pp-cli session remove` — Delete Session
- `vapi-pp-cli session update` — Update Session

**squad** — Manage squad

- `vapi-pp-cli squad create` — Create Squad
- `vapi-pp-cli squad find-all` — List Squads
- `vapi-pp-cli squad find-one` — Get Squad
- `vapi-pp-cli squad remove` — Delete Squad
- `vapi-pp-cli squad update` — Update Squad

**structured-output** — Manage structured output

- `vapi-pp-cli structured-output create` — Create Structured Output
- `vapi-pp-cli structured-output find-all` — List Structured Outputs
- `vapi-pp-cli structured-output find-one` — Get Structured Output
- `vapi-pp-cli structured-output remove` — Delete Structured Output
- `vapi-pp-cli structured-output run` — Run Structured Output
- `vapi-pp-cli structured-output update` — Update Structured Output

**tool** — Manage tool

- `vapi-pp-cli tool create` — Create Tool
- `vapi-pp-cli tool find-all` — List Tools
- `vapi-pp-cli tool find-one` — Get Tool
- `vapi-pp-cli tool remove` — Delete Tool
- `vapi-pp-cli tool update` — Update Tool

**vapi-analytics** — Manage vapi analytics

- `vapi-pp-cli vapi-analytics` — Create Analytics Queries


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vapi-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `vapi-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
vapi-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `VAPI_TOKEN` as an environment variable.

Run `vapi-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vapi-pp-cli chat list --agent --select id,name,status
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
vapi-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vapi-pp-cli feedback --stdin < notes.txt
vapi-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/vapi-pp-cli/feedback.jsonl`. They are never POSTed unless `VAPI_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VAPI_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vapi-pp-cli profile save briefing --json
vapi-pp-cli --profile briefing chat list
vapi-pp-cli profile list --json
vapi-pp-cli profile show briefing
vapi-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `vapi-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai/vapi/cmd/vapi-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vapi-pp-mcp -- vapi-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vapi-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vapi-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vapi-pp-cli <command> --help`.
