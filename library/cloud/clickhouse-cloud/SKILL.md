---
name: pp-clickhouse-cloud
description: "Printing Press CLI for Clickhouse Cloud."
author: "Dhilip Subramanian"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - clickhouse-cloud-pp-cli
    install:
      - kind: go
        bins: [clickhouse-cloud-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/cloud/clickhouse-cloud/cmd/clickhouse-cloud-pp-cli
---

# Clickhouse Cloud — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `clickhouse-cloud-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install clickhouse-cloud --cli-only
   ```
2. Verify: `clickhouse-cloud-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/clickhouse-cloud/cmd/clickhouse-cloud-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.



## Command Reference

**activities** — Manage activities

- `clickhouse-cloud-pp-cli activities activity-get` — Returns a single organization activity by ID.
- `clickhouse-cloud-pp-cli activities activity-get-list` — Returns a list of all organization activities.

**byoc-infrastructure** — Manage byoc infrastructure

- `clickhouse-cloud-pp-cli byoc-infrastructure create` — Create a new BYOC Infrastructure in the organization. Returns the configuration of the newly created infrastructure
- `clickhouse-cloud-pp-cli byoc-infrastructure delete` — Removes a BYOC Infrastructure from the organization
- `clickhouse-cloud-pp-cli byoc-infrastructure update` — Update configuration of the BYOC infrastructure. Returns the modified infrastructure

**invitations** — Manage invitations

- `clickhouse-cloud-pp-cli invitations create` — Creates organization invitation.
- `clickhouse-cloud-pp-cli invitations delete` — Deletes a single organization invitation.
- `clickhouse-cloud-pp-cli invitations get` — Returns details for a single organization invitation.
- `clickhouse-cloud-pp-cli invitations get-list` — Returns list of all organization invitations.

**keys** — Manage keys

- `clickhouse-cloud-pp-cli keys openapi-create` — Creates new API key.
- `clickhouse-cloud-pp-cli keys openapi-delete` — Deletes API key. Only a key not used to authenticate the active request can be deleted.
- `clickhouse-cloud-pp-cli keys openapi-get` — Returns a single key details.
- `clickhouse-cloud-pp-cli keys openapi-get-list` — Returns a list of all keys in the organization.
- `clickhouse-cloud-pp-cli keys openapi-update` — Updates API key properties.

**members** — Manage members

- `clickhouse-cloud-pp-cli members delete` — Removes a user from the organization
- `clickhouse-cloud-pp-cli members get` — Returns a single organization member details.
- `clickhouse-cloud-pp-cli members get-list` — Returns a list of all members in the organization.
- `clickhouse-cloud-pp-cli members update` — Updates organization member role.

**organizations** — Manage organizations

- `clickhouse-cloud-pp-cli organizations get` — Returns details of a single organization. In order to get the details, the auth key must belong to the organization.
- `clickhouse-cloud-pp-cli organizations get-list` — Returns a list with a single organization associated with the API key in the request.
- `clickhouse-cloud-pp-cli organizations update` — Updates organization fields. Requires ADMIN auth key role.

**postgres** — Manage postgres

- `clickhouse-cloud-pp-cli postgres org-prometheus-get` — **Disclaimer:** This beta endpoint is evolving; the API contract may change.
- `clickhouse-cloud-pp-cli postgres service-create` — **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future.
- `clickhouse-cloud-pp-cli postgres service-delete` — **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future.
- `clickhouse-cloud-pp-cli postgres service-get` — **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future.
- `clickhouse-cloud-pp-cli postgres service-get-list` — **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future.
- `clickhouse-cloud-pp-cli postgres service-patch` — **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future.

**private-endpoint-config** — Manage private endpoint config

- `clickhouse-cloud-pp-cli private-endpoint-config <organizationId>` — Deprecated. Please follow [documentation](https://clickhouse.

**prometheus** — Manage prometheus

- `clickhouse-cloud-pp-cli prometheus <organizationId>` — Returns prometheus metrics for all services in an organization.

**roles** — Manage roles

- `clickhouse-cloud-pp-cli roles delete` — Deletes an existing custom role. System roles cannot be deleted.
- `clickhouse-cloud-pp-cli roles get` — Returns details for a specific role.
- `clickhouse-cloud-pp-cli roles get-list` — Returns all available roles (system + custom) for an organization.
- `clickhouse-cloud-pp-cli roles patch` — Updates an existing custom role. System roles cannot be updated.
- `clickhouse-cloud-pp-cli roles post` — Creates a new custom role for an organization with specified policies and actors.

**services** — Manage services

- `clickhouse-cloud-pp-cli services instance-create` — Creates a new service in the organization, and returns the current service state and a password to access the service.
- `clickhouse-cloud-pp-cli services instance-delete` — Deletes the service. The service must be in stopped state and is deleted asynchronously after this method call.
- `clickhouse-cloud-pp-cli services instance-get` — Returns a service that belongs to the organization
- `clickhouse-cloud-pp-cli services instance-get-list` — Returns a list of all services in the organization.
- `clickhouse-cloud-pp-cli services instance-update` — Updates basic service details like service name or IP access list.

**usage-cost** — Manage usage cost

- `clickhouse-cloud-pp-cli usage-cost <organizationId>` — Returns a grand total and a list of daily


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
clickhouse-cloud-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `clickhouse-cloud-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export CLICKHOUSE_CLOUD_USERNAME="<your-key-id>"
export CLICKHOUSE_CLOUD_PASSWORD="<your-key-secret>"
```

To persist credentials, use `clickhouse-cloud-pp-cli auth set-token <key-id> <key-secret>`. Stored secrets live in `credentials.toml` under the data dir, not in `config.toml`.

Run `clickhouse-cloud-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  clickhouse-cloud-pp-cli invitations get mock-value mock-value --agent --select id,name,status
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

## Paths and state

Agents should treat the CLI's path resolver as part of the runtime contract:

- Use `--home <dir>` for one invocation, or set `CLICKHOUSE_CLOUD_HOME=<dir>` to relocate all four path kinds under one root.
- Use per-kind env vars only when a specific kind must diverge: `CLICKHOUSE_CLOUD_CONFIG_DIR`, `CLICKHOUSE_CLOUD_DATA_DIR`, `CLICKHOUSE_CLOUD_STATE_DIR`, `CLICKHOUSE_CLOUD_CACHE_DIR`.
- Resolution order is per-kind env var, `--home`, `CLICKHOUSE_CLOUD_HOME`, XDG (`XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`), then platform defaults.
- `config` contains settings like `config.toml` and profiles. `data` contains `credentials.toml`, `data.db`, cookies, and auth sidecars. `state` contains persisted queries, jobs, and `teach.log`. `cache` contains regenerable HTTP/cache files.
- Stored secrets live in `credentials.toml` under the data dir. Existing legacy `config.toml` secrets are read for compatibility and leave `config.toml` on the first auth write.
- Run `clickhouse-cloud-pp-cli doctor --fail-on warn` to surface path and credential-location warnings. `agent-context` exposes a schema v4 `paths` block for agents that need the resolved dirs.
- For MCP, pass relocation through the MCP host config. The MCP binary does not inherit CLI flags:

  ```json
  {
    "mcpServers": {
      "clickhouse-cloud": {
        "command": "clickhouse-cloud-pp-mcp",
        "env": {
          "CLICKHOUSE_CLOUD_HOME": "/srv/clickhouse-cloud"
        }
      }
    }
  }
  ```

Fleet precedence: an inherited per-kind env var overrides an explicit `--home` for that kind. Use `CLICKHOUSE_CLOUD_HOME` or per-kind vars as durable fleet levers, and use `--home` only for a single invocation. Relocation is not reversible by unsetting env vars; move files manually before clearing `CLICKHOUSE_CLOUD_HOME`, or `doctor` will not find credentials left under the former root.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
clickhouse-cloud-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
clickhouse-cloud-pp-cli feedback --stdin < notes.txt
clickhouse-cloud-pp-cli feedback list --json --limit 10
```

Entries are stored locally as `feedback.jsonl` under the resolved data dir. They are never POSTed unless `CLICKHOUSE_CLOUD_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CLICKHOUSE_CLOUD_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
clickhouse-cloud-pp-cli profile save briefing --json
clickhouse-cloud-pp-cli --profile briefing invitations get mock-value mock-value
clickhouse-cloud-pp-cli profile list --json
clickhouse-cloud-pp-cli profile show briefing
clickhouse-cloud-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Async Jobs

For endpoints that submit long-running work, the generator detects the submit-then-poll pattern (a `job_id`/`task_id`/`operation_id` field in the response plus a sibling status endpoint) and wires up three extra flags on the submitting command:

| Flag | Purpose |
|------|---------|
| `--wait` | Block until the job reaches a terminal status instead of returning the job ID immediately |
| `--wait-timeout` | Maximum wait duration (default 10m, 0 means no timeout) |
| `--wait-interval` | Initial poll interval (default 2s; grows with exponential backoff up to 30s) |

Use async submission without `--wait` when you want to fire-and-forget; use `--wait` when you want one command to return the finished artifact.

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

1. **Empty, `help`, or `--help`** → show `clickhouse-cloud-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/cloud/clickhouse-cloud/cmd/clickhouse-cloud-pp-mcp@latest
   ```
2. Register `clickhouse-cloud-pp-mcp` with your MCP-compatible host using that host's add-server flow.
3. Verify the host can launch `clickhouse-cloud-pp-mcp`.

## Direct Use

1. Check if installed: `which clickhouse-cloud-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   clickhouse-cloud-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `clickhouse-cloud-pp-cli <command> --help`.
