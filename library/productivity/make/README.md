# Make.com CLI

**Every Make (formerly Integromat) Management API operation from the terminal, plus blocking agent runs, git-backed blueprint sync, dev-to-prod promote with ID remap, and a cross-team DLQ inbox no other tool has.**

The official make-cli is a 1:1 REST mirror; the Make Cloud MCP is paid-tier and Anthropic-hosted only. make-pp-cli is a single Go binary with an offline SQLite mirror, agent-native --json/--select/--csv on every read, a `scenarios run --wait` that actually returns the bundle, blueprint sync for git version control, and a dev-to-prod promote that solves the ID-remap problem the community has been complaining about for years.

Learn more at [Make.com](https://www.make.com/).

Created by [@wcarpenter110981](https://github.com/wcarpenter110981) (Wade Carpenter).

## Install

The recommended path installs both the `make-pp-cli` binary and the `pp-make` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install make
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install make --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install make --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install make --agent claude-code
npx -y @mvanhorn/printing-press-library install make --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/make/cmd/make-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/make-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-make --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-make --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-make skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-make. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/make-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `MAKE_API_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/make/cmd/make-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "make": {
      "command": "make-pp-mcp",
      "env": {
        "MAKE_ZONE": "<zone>",
        "MAKE_API_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Auth is a Make API token (`Authorization: Token <token>`) plus a zone (us1, us2, eu1, eu2). Token is created at make.com profile -> API -> Add token. Set `MAKE_API_TOKEN` and `MAKE_ZONE` in your environment; `doctor` verifies both.

## Quick Start

```bash
# confirm token + zone before anything else
make-pp-cli doctor

# find your organization and team IDs
make-pp-cli orgs list --json

# populate the local store so search/audit/dlq work offline
make-pp-cli sync

# one-shot cross-team view
make-pp-cli scenarios list-all --active --json --select rows.id,rows.name,rows.teamId,rows.lastEdit

# the agent-native blocking run
make-pp-cli scenarios run 3041366 --wait --timeout 5m --json

# back up every scenario's blueprint into a git repo
make-pp-cli blueprint sync --repo ./make-blueprints --all-teams

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Agent-native plumbing
- **`scenarios run --wait`** — Trigger a scenario and block until it finishes, returning the execution result as one JSON envelope. Composes /scenarios/{id}/run + executions polling + bundle fetch.

  _When an agent needs a Make scenario's output to decide its next step, this is the only command that returns the bundle inline._

  ```bash
  make-pp-cli scenarios run 3041366 --wait --timeout 5m --json
  ```

### Version control for scenarios
- **`blueprint sync`** — Export every scenario's blueprint into a local repo as canonical JSON, one file per scenario, with a separate sidecar for metadata noise. Commit-ready.

  _Lets an agent or human treat scenarios as code: review diffs, restore prior versions, run CI on changes._

  ```bash
  make-pp-cli blueprint sync --repo ./make-blueprints --all-teams
  ```
- **`blueprint promote`** — Promote a scenario from one team to another, rewriting connectionId/hookId/dataStoreId/folderId via auto-suggested name match or a user-edited YAML map. Dry-run shows the rewrites.

  _Pick this when a Make scenario needs to ship from dev to prod — the only command that solves the ID-remap problem._

  ```bash
  make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --auto-suggest --dry-run
  ```
- **`blueprint diff`** — Diff two blueprint snapshots from the local snapshot table; restore re-PUTs a snapshot via the API. Structural diff ignores the metadata.expect/restore noise.

  _After a blueprint promote went wrong, this is the rollback path — git log + git restore for Make scenarios._

  ```bash
  make-pp-cli blueprint diff 3454192 --from ./make-blueprints/team-588013/3454192.blueprint.json --to current --json
  ```

### Local state that compounds
- **`dlq inbox`** — List incomplete executions across all scenarios in one or many teams, group by error-reason fingerprint, bulk retry or resolve with a regex match.

  _When triaging Make failures, this is the only command that shows everything that broke and groups by why._

  ```bash
  make-pp-cli dlq inbox --all-teams --age 24h --group-by reason --json
  ```
- **`connections audit`** — Find connections that are unused, expiring, or repeatedly errored. Joins local connections with walked blueprint references and execution errors.

  _Run before a prod cutover, or weekly to catch stale OAuth tokens before they 401 your scenarios._

  ```bash
  make-pp-cli connections audit --all-teams --unused --expiring 168h --json
  ```
- **`scenarios list-all`** — Union scenarios across every team the token can see, with last-edit, last-run, error rate, folder path, active flag. Filter by --stale Nh.

  _Monday-morning ritual command: what is active and what is stale across everything you own._

  ```bash
  make-pp-cli scenarios list-all --active --stale 720h --json --select rows.id,rows.name,rows.teamId,rows.lastEdit
  ```
- **`hooks map`** — For each webhook, show which scenarios consume it by walking every blueprint's gateway:CustomWebHook references. Flag orphan hooks (no consumer) and shared hooks (multiple consumers).

  _When a webhook misroutes or you are cleaning up old hooks, this tells you what is actually connected._

  ```bash
  make-pp-cli hooks map --all-teams --orphans --json
  ```

## Recipes


### Agent-blocking scenario run with output bundle

```bash
make-pp-cli scenarios run 3041366 --wait --timeout 5m --json --select status,bundles
```

Triggers the scenario, blocks until terminal, returns the execution status and parsed output bundles inline so the calling agent can act on the result.

### Weekly DLQ triage by error reason

```bash
make-pp-cli dlq inbox --all-teams --age 24h --group-by reason --json
```

Joins incomplete executions across every team and groups by extracted error fingerprint so the on-call sees patterns, not individual failures.

### Promote a scenario dev to prod with auto-remap dry-run

```bash
make-pp-cli blueprint promote --from-team 588013 --to-team 999999 --scenario 3454192 --auto-suggest --dry-run
```

Reads the source blueprint, matches connections/hooks/data-stores by name in the target, prints the rewrite map so you can review before the real promote.

### Find stale OAuth connections before prod cutover

```bash
make-pp-cli connections audit --all-teams --expiring 168h --errored 168h --json --select rows.id,rows.name,rows.expire,rows.issues
```

Surfaces every connection expiring within a week or that has logged errors in the last 7 days, so a stale token does not 401 the production run.

### Sync every blueprint to a local repo nightly

```bash
make-pp-cli blueprint sync --repo ./make-blueprints --all-teams
```

Replaces the community's DIY 'daily-export-to-github' Make scenarios. Run from cron; pipe into `git add -A && git commit` to commit.

## Usage

Run `make-pp-cli --help` for the full command reference and flag list.

## Commands

### connections

Connections — auth grants to external services

- **`make-pp-cli connections delete`** - Delete a connection
- **`make-pp-cli connections get`** - Get a connection
- **`make-pp-cli connections list`** - List connections for a team
- **`make-pp-cli connections test`** - Verify a connection is live

### data_stores

Data stores — Make's key-value persistence

- **`make-pp-cli data-stores create`** - Create a data store
- **`make-pp-cli data-stores delete`** - Delete data stores
- **`make-pp-cli data-stores get`** - Get a data store
- **`make-pp-cli data-stores list`** - List data stores for a team
- **`make-pp-cli data-stores update`** - Update data store

### data_structures

Data structures — typed schemas referenced by data stores and webhooks

- **`make-pp-cli data-structures create`** - Create a data structure
- **`make-pp-cli data-structures delete`** - Delete a data structure
- **`make-pp-cli data-structures get`** - Get a data structure
- **`make-pp-cli data-structures list`** - List data structures for a team

### devices

Mobile devices registered for triggering Make scenarios

- **`make-pp-cli devices delete`** - Delete a device
- **`make-pp-cli devices get`** - Get a device
- **`make-pp-cli devices list`** - List devices for a team

### dlqs

Incomplete executions (Dead Letter Queue) — scenarios that failed

- **`make-pp-cli dlqs get`** - Get one DLQ entry
- **`make-pp-cli dlqs list`** - List incomplete executions for a scenario
- **`make-pp-cli dlqs resolve`** - Resolve (acknowledge) a failed execution
- **`make-pp-cli dlqs retry`** - Retry a failed execution

### executions

Scenario executions — historical runs

- **`make-pp-cli executions <scenarioId> <executionId>`** - Get one execution's detail

### folders

Scenario folders — group scenarios within a team

- **`make-pp-cli folders create`** - Create a folder
- **`make-pp-cli folders delete`** - Delete a folder
- **`make-pp-cli folders list`** - List scenario folders for a team
- **`make-pp-cli folders update`** - Rename a folder

### functions

Custom functions usable inside scenario mappings

- **`make-pp-cli functions delete`** - Delete a custom function
- **`make-pp-cli functions get`** - Get a custom function
- **`make-pp-cli functions list`** - List custom functions for a team

### hooks

Webhooks — entry points that trigger scenarios

- **`make-pp-cli hooks delete`** - Delete a webhook
- **`make-pp-cli hooks disable`** - Disable a webhook
- **`make-pp-cli hooks enable`** - Enable a webhook
- **`make-pp-cli hooks get`** - Get a webhook
- **`make-pp-cli hooks learn-start`** - Start webhook learning mode
- **`make-pp-cli hooks learn-stop`** - Stop webhook learning mode
- **`make-pp-cli hooks list`** - List webhooks for a team
- **`make-pp-cli hooks ping`** - Send a ping to a webhook

### keys

Keys — credentials (API tokens, basic auth, etc.) attached to connections

- **`make-pp-cli keys delete`** - Delete a key
- **`make-pp-cli keys get`** - Get a key
- **`make-pp-cli keys list`** - List keys for a team

### orgs

Organizations — top-level account container

- **`make-pp-cli orgs get`** - Get an organization
- **`make-pp-cli orgs list`** - List organizations visible to this token

### scenarios

Scenarios — Make's workflow automations

- **`make-pp-cli scenarios activate`** - Activate a scenario
- **`make-pp-cli scenarios blueprint`** - Export a scenario's blueprint as JSON
- **`make-pp-cli scenarios clone`** - Clone a scenario
- **`make-pp-cli scenarios create`** - Create a scenario from a blueprint
- **`make-pp-cli scenarios deactivate`** - Deactivate a scenario
- **`make-pp-cli scenarios delete`** - Delete a scenario
- **`make-pp-cli scenarios get`** - Get a scenario
- **`make-pp-cli scenarios list`** - List scenarios for a team
- **`make-pp-cli scenarios logs`** - Get execution log entries for a scenario
- **`make-pp-cli scenarios run`** - Trigger one execution of a scenario (asynchronous unless --wait is used)
- **`make-pp-cli scenarios update`** - Update scenario metadata

### sdk_apps

Custom SDK apps — user-built integrations published to Make

- **`make-pp-cli sdk-apps get`** - Get an SDK app
- **`make-pp-cli sdk-apps list`** - List SDK apps for a team

### teams

Teams within an organization

- **`make-pp-cli teams create`** - Create a team
- **`make-pp-cli teams delete`** - Delete a team
- **`make-pp-cli teams get`** - Get a team
- **`make-pp-cli teams list`** - List teams in an organization

### templates

Public templates — sharable scenario blueprints

- **`make-pp-cli templates get`** - Get a template
- **`make-pp-cli templates list`** - List templates for a team

### users

Users — token owner and team members

- **`make-pp-cli users`** - Get the authenticated user


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
make-pp-cli connections list --team-id 42

# JSON for scripting and agents
make-pp-cli connections list --team-id 42 --json

# Filter to specific fields
make-pp-cli connections list --team-id 42 --json --select id,name,status

# Dry run — show the request without sending
make-pp-cli connections list --team-id 42 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
make-pp-cli connections list --team-id 42 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `MAKE_ZONE` resolves `{zone}`

Base URL: `https://{zone}.make.com/api/v2`

## Health Check

```bash
make-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/make-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `MAKE_ZONE` | endpoint | Yes |  |
| `MAKE_API_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `make-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `make-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $MAKE_API_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every call** — Wrong zone for this token. Run `make-pp-cli doctor`; it probes us1/us2/eu1/eu2 and reports which one your token belongs to.
- **scenarios run --wait times out before the scenario finishes** — Raise --timeout (default 5m) or use --poll-interval 2s for faster turn-around on quick scenarios.
- **blueprint promote shows TODO entries in the remap** — Some source connections/hooks have no name match in the target team. Provide a --map file with manual mappings for the TODO IDs.
- **Permission denied (SC403) on a specific endpoint** — Your token may lack the required scope. Run `make-pp-cli doctor --scopes` to print the full scope list, then create a token with the needed scope in the Make profile UI.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**make-mcp-server**](https://github.com/integromat/make-mcp-server) — TypeScript (159 stars)
- [**make-cli**](https://github.com/integromat/make-cli) — TypeScript (97 stars)
- [**make-typescript-sdk**](https://github.com/integromat/make-typescript-sdk) — TypeScript
- [**makker**](https://github.com/zezutom/makker) — Kotlin

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
