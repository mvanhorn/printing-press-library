---
name: pp-airbyte-admin
description: "Printing Press CLI for Airbyte Admin. Inspect Airbyte workspaces, sources, destinations, connections, jobs, connector definitions, users, organizations, permissions, and tags."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - airbyte-admin-pp-cli
    install:
      - kind: go
        bins: [airbyte-admin-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/airbyte-admin/cmd/airbyte-admin-pp-cli
---

# Airbyte Admin — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `airbyte-admin-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install airbyte-admin --cli-only
   ```
2. Verify: `airbyte-admin-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/airbyte-admin/cmd/airbyte-admin-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Airbyte is a data integration platform used by data teams to move data from sources into warehouses, lakes, and operational destinations. This CLI exposes Airbyte's official Public API as a read-only terminal and MCP surface for inspecting pipeline inventory, sync jobs, connector metadata, workspaces, users, organizations, permissions, and tags.

Use it when an engineer or agent needs to answer operational questions such as "which sources and destinations are configured?", "what does this connection look like?", "which jobs are failing or running?", or "what workspaces and permissions can this account see?"

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**public** — Manage public

- `airbyte-admin-pp-cli public get-application` — Get an Application detail
- `airbyte-admin-pp-cli public get-connection` — Get Connection details
- `airbyte-admin-pp-cli public get-destination` — Get Destination details
- `airbyte-admin-pp-cli public get-documentation` — Root path, currently returns a redirect to the documentation
- `airbyte-admin-pp-cli public get-health-check` — Health Check
- `airbyte-admin-pp-cli public get-job` — Get Job status and details
- `airbyte-admin-pp-cli public get-source` — Get Source details
- `airbyte-admin-pp-cli public get-stream-properties` — Get stream properties
- `airbyte-admin-pp-cli public list-applications` — List Applications
- `airbyte-admin-pp-cli public list-connector-definitions` — List connector definitions
- `airbyte-admin-pp-cli public list-destinations` — List destinations
- `airbyte-admin-pp-cli public list-jobs` — List Jobs
- `airbyte-admin-pp-cli public list-organizations-for-user` — List all organizations for a user
- `airbyte-admin-pp-cli public list-permissions` — List permissions
- `airbyte-admin-pp-cli public list-sources` — List sources
- `airbyte-admin-pp-cli public list-tags` — Lists all tags
- `airbyte-admin-pp-cli public list-users` — Organization Admin user can list all users within the same organization.
- `airbyte-admin-pp-cli public list-workspaces` — List workspaces
- `airbyte-admin-pp-cli public oauth-callback` — Redirected to by identity providers after authentication.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
airbyte-admin-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

The health endpoint is public, but most Airbyte Cloud and self-managed Public API commands require an Authorization header. Do not store credentials in GitHub or in prompts.

Preferred environment variables:

```bash
export AIRBYTE_ADMIN_BASE_URL="https://cloud.airbyte.com/api"
export AIRBYTE_ADMIN_TOKEN="<airbyte-token>"
```

For local self-managed Airbyte or deployments that need a complete header:

```bash
export AIRBYTE_ADMIN_BASE_URL="http://localhost:8000/api"
export AIRBYTE_ADMIN_AUTH_HEADER="Basic <base64-user-pass>"
```

`AIRBYTE_ADMIN_TOKEN` is converted to `Bearer <token>` unless it already starts with `Bearer `. `AIRBYTE_ADMIN_AUTH_HEADER` wins when both are set.

Run `airbyte-admin-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  airbyte-admin-pp-cli public list-sources --agent --select id,name,status
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

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
airbyte-admin-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
airbyte-admin-pp-cli feedback --stdin < notes.txt
airbyte-admin-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/airbyte-admin-pp-cli/feedback.jsonl`. They are never POSTed unless `AIRBYTE_ADMIN_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `AIRBYTE_ADMIN_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
airbyte-admin-pp-cli profile save briefing --json
airbyte-admin-pp-cli --profile briefing public list-sources
airbyte-admin-pp-cli profile list --json
airbyte-admin-pp-cli profile show briefing
airbyte-admin-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `airbyte-admin-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/airbyte-admin/cmd/airbyte-admin-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add airbyte-admin-pp-mcp -- airbyte-admin-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which airbyte-admin-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   airbyte-admin-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `airbyte-admin-pp-cli <command> --help`.
