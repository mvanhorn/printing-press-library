---
name: pp-vibecode
description: "Every Vibecode feature, plus offline sync, cross-project search, and deployment drift detection no other tool has Trigger phrases: `deploy my vibecode project`, `check vibecode deployments`, `search vibecode projects`, `build with vibecode`, `vibecode stale projects`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - vibecode-pp-cli
    install:
      - kind: go
        bins: [vibecode-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/cmd/vibecode-pp-cli
---

# Vibecode — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `vibecode-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press-library install vibecode --cli-only
   ```
2. Verify: `vibecode-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/cmd/vibecode-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A local-first CLI for the vibecode.dev platform that caches your projects, deployments, and commits in SQLite for instant offline access. Adds cross-entity search, since-style delta commands, and deployment drift detection that the official CLI cannot offer.

## When to Use This CLI

Use this CLI when you're building apps on vibecode.dev and want offline access to your project data, cross-project search, or deployment drift detection. Ideal for developers managing multiple Vibecode projects or AI agents that need reliable structured output.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`search`** — Full-text search across all projects, commits, and deployments in one query

  _Find that project or commit without remembering which entity type contains it_

  ```bash
  vibecode-pp-cli search 'landing page' --json
  ```
- **`changes --since`** — See what changed across all projects since a timestamp or last sync

  _Resume work with full context after stepping away_

  ```bash
  vibecode-pp-cli changes --since '2 hours ago' --json
  ```

### Production safety
- **`drift`** — Compare current live deployment against cached configuration to spot unexpected changes

  _Catch environment variable or build setting changes before they cause production issues_

  ```bash
  vibecode-pp-cli drift proj_abc123 --json
  ```
- **`stale --days`** — Find deployments across all projects that haven't been updated in N days

  _Clean up old deployments to reduce costs and attack surface_

  ```bash
  vibecode-pp-cli stale --days 30 --json
  ```

### Developer experience
- **`metrics builds`** — Track build times over history with averages, p95, and trend indicators

  _Identify builds getting slower before they become a bottleneck_

  ```bash
  vibecode-pp-cli metrics builds --project proj_abc123 --json
  ```
- **`batch deploy`** — Deploy multiple projects matching a glob pattern with parallelism control

  _Deploy coordinated changes across microservices in one command_

  ```bash
  vibecode-pp-cli batch deploy --pattern 'frontend-*' --parallel 3 --json
  ```

## Command Reference

**agent** — AI coding agent control

- `vibecode-pp-cli agent send` — Send prompt to coding agent (streams events)
- `vibecode-pp-cli agent stop` — Stop running agent task

**deployment_auth** — HTTP Basic Auth for deployments

- `vibecode-pp-cli deployment_auth disable` — Disable HTTP Basic Auth
- `vibecode-pp-cli deployment_auth get` — Get HTTP Basic Auth configuration
- `vibecode-pp-cli deployment_auth set` — Enable HTTP Basic Auth

**deployment_domain** — Custom domain configuration for deployments

- `vibecode-pp-cli deployment_domain get` — Get custom domain configuration
- `vibecode-pp-cli deployment_domain remove` — Remove custom domain
- `vibecode-pp-cli deployment_domain set` — Set custom domain
- `vibecode-pp-cli deployment_domain verify` — Verify DNS records for custom domain

**deployment_links** — Tunnel links for deployments

- `vibecode-pp-cli deployment_links create` — Create deployment link
- `vibecode-pp-cli deployment_links delete` — Delete deployment link
- `vibecode-pp-cli deployment_links list` — List deployment links

**deployment_subdomain** — Subdomain configuration for deployments

- `vibecode-pp-cli deployment_subdomain check` — Check subdomain availability
- `vibecode-pp-cli deployment_subdomain set` — Set deployment subdomain

**deployments** — Production deployments

- `vibecode-pp-cli deployments deploy` — Deploy project (waits up to 2min)
- `vibecode-pp-cli deployments destroy` — Tear down deployment
- `vibecode-pp-cli deployments get` — Get deployment details
- `vibecode-pp-cli deployments list` — List deployments
- `vibecode-pp-cli deployments ready` — Check if deployment is live

**projects** — Vibecode projects (web apps, mobile apps, openclaw)

- `vibecode-pp-cli projects commits` — List git commits for a project
- `vibecode-pp-cli projects create` — Create a new project
- `vibecode-pp-cli projects delete` — Delete a project permanently
- `vibecode-pp-cli projects get` — Get project details including subdomain and custom domain
- `vibecode-pp-cli projects list` — List all projects
- `vibecode-pp-cli projects rename` — Rename a project

**sandbox_links** — Tunnel links for sandboxes

- `vibecode-pp-cli sandbox_links create` — Create tunnel link
- `vibecode-pp-cli sandbox_links delete` — Delete tunnel link
- `vibecode-pp-cli sandbox_links list` — List tunnel links for a sandbox

**sandboxes** — Cloud VM sandboxes for development

- `vibecode-pp-cli sandboxes acquire` — Start sandbox and ensure tunnel links
- `vibecode-pp-cli sandboxes get` — Get sandbox status for a project
- `vibecode-pp-cli sandboxes kill` — Terminate sandbox
- `vibecode-pp-cli sandboxes list` — List running sandboxes

**user** — Current authenticated user profile

- `vibecode-pp-cli user` — Get current user profile

**yolo** — Combined build and deploy

- `vibecode-pp-cli yolo <project_id>` — Agent send + deploy in one shot


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
vibecode-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes

### Find stale deployments

```bash
vibecode-pp-cli stale --days 14 --json
```

List deployments not updated in 14 days for cleanup

### Check what changed

```bash
vibecode-pp-cli changes --since '1 day ago' --select name,status
```

See projects modified in the last day with only essential fields

### Batch deploy frontend

```bash
vibecode-pp-cli batch deploy --pattern 'frontend-*' --parallel 2
```

Deploy all frontend projects with controlled parallelism

### Detect deployment drift

```bash
vibecode-pp-cli drift proj_abc123 --json
```

Compare live deployment against cached config

### Build metrics

```bash
vibecode-pp-cli metrics builds --project proj_abc123 --agent
```

Get build duration trends with agent-optimized output

## Auth Setup

Set VIBECODE_API_KEY from vibecode.dev/key. The CLI validates on first use and caches your user profile locally.

Run `vibecode-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  vibecode-pp-cli deployment_auth get mock-value --agent --select id,name,status
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
vibecode-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
vibecode-pp-cli feedback --stdin < notes.txt
vibecode-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.vibecode-pp-cli/feedback.jsonl`. They are never POSTed unless `VIBECODE_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `VIBECODE_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
vibecode-pp-cli profile save briefing --json
vibecode-pp-cli --profile briefing deployment_auth get mock-value
vibecode-pp-cli profile list --json
vibecode-pp-cli profile show briefing
vibecode-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `vibecode-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/developer-tools/vibecode/cmd/vibecode-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add vibecode-pp-mcp -- vibecode-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which vibecode-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   vibecode-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `vibecode-pp-cli <command> --help`.
