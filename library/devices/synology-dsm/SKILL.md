---
name: pp-synology-dsm
description: "Printing Press CLI for Synology Dsm. REST API for Synology DiskStation Manager (DSM 7.x). All API calls are dispatched through /webapi/entry.cgi with..."
author: "Eric Jung"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - synology-dsm-pp-cli
---

# Synology Dsm — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `synology-dsm-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install synology-dsm --cli-only
   ```
2. Verify: `synology-dsm-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

REST API for Synology DiskStation Manager (DSM 7.x). All API calls are dispatched through /webapi/entry.cgi with api, method, and version query parameters. Authentication uses session-based SID tokens. Use 'auth set-token' to save your SID, or set SYNOLOGY_DSM_SIDCOOKIE env var. Obtain a SID via 'webapi login'.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Command Reference

**webapi** — Manage webapi

- `synology-dsm-pp-cli webapi cancel-backup-task` — Cancel an in-progress Hyper Backup task
- `synology-dsm-pp-cli webapi delete-scheduled-task` — Delete a scheduled task permanently
- `synology-dsm-pp-cli webapi get-backup-task` — Get details for a specific Hyper Backup task
- `synology-dsm-pp-cli webapi get-backup-task-status` — Get current status and progress of a Hyper Backup task
- `synology-dsm-pp-cli webapi get-container` — Get detailed configuration and status for a specific container
- `synology-dsm-pp-cli webapi get-container-logs` — Get recent log output from a container
- `synology-dsm-pp-cli webapi get-dsminfo` — Get DSM system information — model, version, uptime, CPU
- `synology-dsm-pp-cli webapi get-file-info` — Get metadata for specific files or directories
- `synology-dsm-pp-cli webapi get-file-station-info` — Get File Station capabilities and hostname
- `synology-dsm-pp-cli webapi get-scheduled-task` — Get configuration for a specific scheduled task
- `synology-dsm-pp-cli webapi get-storage-disk` — Get detailed information for a specific disk including SMART data
- `synology-dsm-pp-cli webapi get-storage-volume` — Get details for a specific volume
- `synology-dsm-pp-cli webapi get-system-utilization` — Get real-time CPU, memory, network, and disk utilization
- `synology-dsm-pp-cli webapi list-backup-repositories` — List all Hyper Backup repositories (destinations)
- `synology-dsm-pp-cli webapi list-backup-tasks` — List all Hyper Backup tasks with schedule and destination info
- `synology-dsm-pp-cli webapi list-container-images` — List all downloaded Docker images
- `synology-dsm-pp-cli webapi list-containers` — List all Docker containers with running status and image
- `synology-dsm-pp-cli webapi list-files` — List files and directories in a folder path
- `synology-dsm-pp-cli webapi list-scheduled-tasks` — List all Task Scheduler tasks with schedule and enable status
- `synology-dsm-pp-cli webapi list-shared-folders` — List all shared folders visible to the authenticated user
- `synology-dsm-pp-cli webapi list-storage-disks` — List all disks with health status, temperature, and SMART indicators
- `synology-dsm-pp-cli webapi list-storage-pools` — List all storage pools (RAID groups) with health and usage
- `synology-dsm-pp-cli webapi list-storage-volumes` — List all storage volumes with usage and mount point
- `synology-dsm-pp-cli webapi login` — Authenticate with DSM and obtain a session ID (SID). After login, save the returned sid with: auth set-token...
- `synology-dsm-pp-cli webapi logout` — Log out and invalidate the current session
- `synology-dsm-pp-cli webapi restart-container` — Restart a container (stop then start)
- `synology-dsm-pp-cli webapi run-backup-task` — Trigger an immediate Hyper Backup for a task
- `synology-dsm-pp-cli webapi run-scheduled-task` — Run a scheduled task immediately (outside its normal schedule)
- `synology-dsm-pp-cli webapi set-scheduled-task-enabled` — Enable or disable a scheduled task
- `synology-dsm-pp-cli webapi start-container` — Start a stopped container
- `synology-dsm-pp-cli webapi stop-container` — Stop a running container gracefully


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
synology-dsm-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `synology-dsm-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export SYNOLOGY_DSM_SIDCOOKIE="<your-key>"
```

Or persist it in `~/.config/synology-dsm-pp-cli/config.toml`.

Run `synology-dsm-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  synology-dsm-pp-cli webapi cancel-backup-task --agent --select id,name,status
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
synology-dsm-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
synology-dsm-pp-cli feedback --stdin < notes.txt
synology-dsm-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.synology-dsm-pp-cli/feedback.jsonl`. They are never POSTed unless `SYNOLOGY_DSM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SYNOLOGY_DSM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
synology-dsm-pp-cli profile save briefing --json
synology-dsm-pp-cli --profile briefing webapi cancel-backup-task
synology-dsm-pp-cli profile list --json
synology-dsm-pp-cli profile show briefing
synology-dsm-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `synology-dsm-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add synology-dsm-pp-mcp -- synology-dsm-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which synology-dsm-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   synology-dsm-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `synology-dsm-pp-cli <command> --help`.
