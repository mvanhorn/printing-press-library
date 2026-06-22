---
name: pp-attentive
description: "Printing Press CLI for Attentive. Use this CLI with Attentive API credentials and the public Attentive API surface."
author: "Deb Mukherjee"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - attentive-pp-cli
    install:
      - kind: go
        bins: [attentive-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-cli
---

# Attentive — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `attentive-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install attentive --cli-only
   ```
2. Verify: `attentive-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Use this CLI with Attentive API credentials and the public Attentive API surface.

## Command Reference

**bulk** — Manage bulk

- `attentive-pp-cli bulk get-job-status` — Checks the status of a bulk ingestion job identified by bulkJobId.
- `attentive-pp-cli bulk post-segment-members` — Add members to a segment in bulk. This endpoint accepts 1 to 10,000 members per request.
- `attentive-pp-cli bulk post-user-attributes` — This endpoint allows clients to submit multiple user attribute updates in bulk

**me** — Manage me

- `attentive-pp-cli me` — Make a call to this endpoint to test your unique token that you generate in the Attentive product.

**segments** — Endpoints for submitting bulk segment member additions and removals. Use these endpoints to manage segment memberships in bulk and monitor the processing status asynchronously.

## Processing Times

The Bulk API processes jobs with the following targets:
- **Standard Processing**: The first 10,000 records per day per customer typically complete within 4 hours of request acceptance.
- **High-Volume Processing**: Additional records beyond 10,000 per day typically complete within 12 hours.

**Note**: For jobs with more than 1 million records, processing times may vary.

- `attentive-pp-cli segments create` — Creates a new empty segment with the specified name and optional description.
- `attentive-pp-cli segments delete-by-external-id` — Archives (soft deletes) a segment by external ID.
- `attentive-pp-cli segments get-by-external-id` — Retrieves segment details by external ID.
- `attentive-pp-cli segments list` — Lists segments with optional filtering by name, external ID, or update timestamp.
- `attentive-pp-cli segments patch-by-external-id` — Partially updates an existing segment. Only provided fields will be updated.

**user** — Manage user

- `attentive-pp-cli user` — Creates or updates a single user record, including associated attributes, subscriptions, and identifiers.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
attentive-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `attentive-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
attentive-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `ATTENTIVE_BEARER_AUTH` as an environment variable.

Run `attentive-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  attentive-pp-cli segments list --agent --select id,name,status
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
attentive-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
attentive-pp-cli feedback --stdin < notes.txt
attentive-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/attentive-pp-cli/feedback.jsonl`. They are never POSTed unless `ATTENTIVE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ATTENTIVE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
attentive-pp-cli profile save briefing --json
attentive-pp-cli --profile briefing segments list
attentive-pp-cli profile list --json
attentive-pp-cli profile show briefing
attentive-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `attentive-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/marketing/attentive/cmd/attentive-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add attentive-pp-mcp -- attentive-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which attentive-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   attentive-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `attentive-pp-cli <command> --help`.
