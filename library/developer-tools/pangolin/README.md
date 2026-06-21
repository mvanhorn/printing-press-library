# Pangolin CLI

**The first agent-native CLI for Pangolin — every endpoint, plus offline SQLite, cross-org audits, and one-shot config backup and restore the dashboard cannot do.**

Pangolin is the open-source, self-hosted alternative to Cloudflare Tunnels. This CLI exposes all 157 integration-API endpoints with --json, --select, --csv, and --dry-run, mirrors the full configuration into a local SQLite store, and adds commands that span orgs — audit, cert-watch, access-graph, backup, restore, expose — that no Pangolin tool offers today.

## Install

The recommended path installs both the `pangolin-pp-cli` binary and the `pp-pangolin` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install pangolin
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install pangolin --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install pangolin --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install pangolin --agent claude-code
npx -y @mvanhorn/printing-press-library install pangolin --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/cmd/pangolin-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pangolin-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-pangolin --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-pangolin --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-pangolin skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-pangolin. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/pangolin-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `PANGOLIN_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/pangolin/cmd/pangolin-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "pangolin": {
      "command": "pangolin-pp-mcp",
      "env": {
        "PANGOLIN_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Authentication is a single Bearer integration token. Set PANGOLIN_TOKEN in your environment; the base URL is typically https://<your-dashboard>/api/v1 (note: the OpenAPI spec advertises /v1 but real EE deployments mount the API under /api/v1). Set PANGOLIN_BASE_URL accordingly and run `pangolin-pp-cli doctor` to confirm.

## Quick Start

```bash
# Confirm auth and base URL are correct before doing anything else
pangolin-pp-cli doctor


# Mirror every org, site, resource, target, role, and IdP into local SQLite
pangolin-pp-cli sync --full


# Cross-org health: stale targets, missing roles, orphaned resources
pangolin-pp-cli audit --json


# List certificates expiring in the next 30 days
pangolin-pp-cli cert-watch --days 30 --json


# Snapshot the full configuration for version control or DR
pangolin-pp-cli backup --out pangolin-backup.json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`audit`** — Find stale targets, orphaned resources, and missing role bindings across every org you administer, in one command.

  _When the user asks 'is anything broken in my Pangolin stack?', this is the single command that answers it — no dashboard tab-clicking._

  ```bash
  pangolin-pp-cli audit --json --select issues
  ```
- **`cert-watch`** — List certificates sorted by days-until-expiry across every org, with a configurable warning window.

  _When the user asks 'what certs expire soon?', skip the dashboard and get an actionable list with one call._

  ```bash
  pangolin-pp-cli cert-watch --days 30 --json
  ```
- **`access-graph`** — Answer 'who can reach what' by joining users, roles, resources, and orgs into one queryable view.

  _When the user asks 'what does $person have access to?' or 'who can hit this resource?', this is the answer._

  ```bash
  pangolin-pp-cli access-graph --user $USER_ID --json
  ```

### Disaster recovery
- **`backup`** — Export the complete Pangolin configuration — orgs, sites, resources, targets, roles, IdPs — as version-controllable JSON.

  _When the user wants a disaster-recovery snapshot or a config diff between dates, this is the artifact._

  ```bash
  pangolin-pp-cli backup --out pangolin-backup.json
  ```
- **`restore`** — Re-apply a backup against a fresh Pangolin install, in the correct dependency order, with dry-run preview.

  _When the user rebuilds a Pangolin host or moves between machines, this is the way back up._

  ```bash
  pangolin-pp-cli restore pangolin-backup.json --dry-run
  ```

### Agent-native plumbing
- **`expose`** — Create site (if needed), create resource, attach target, bind a role — in one command, with dry-run preview.

  _When the user (or an agent) wants to expose a homelab service, this is the entire workflow in one line._

  ```bash
  pangolin-pp-cli expose grafana --target 192.168.1.50:3000 --site site_42 --role admins --dry-run
  ```
- **`doctor`** — Validate auth, probe both /v1 and /api/v1 mount paths, and report the working integration-API base URL with environment-variable guidance.

  _When the user sets up the CLI for the first time, this catches the single most common misconfiguration before any other command fails._

  ```bash
  pangolin-pp-cli doctor
  ```

## Usage

Run `pangolin-pp-cli --help` for the full command reference and flag list.

## Commands

### access-token

Manage access token

- **`pangolin-pp-cli access-token <accessTokenId>`** - Delete a access token.

### certificate

Manage certificate

- **`pangolin-pp-cli certificate <certId> <orgId>`** - Restart a certificate by ID.

### client

Manage client

- **`pangolin-pp-cli client create`** - Update a client by its client ID.
- **`pangolin-pp-cli client delete`** - Delete a client by its client ID.
- **`pangolin-pp-cli client get`** - Get a client by its client ID.

### domain

Manage domain

- **`pangolin-pp-cli domain`** - Check if a domain namespace is available based on subdomain

### domains

Manage domains

- **`pangolin-pp-cli domains`** - List all domain namespaces in the system

### idp

Manage idp

- **`pangolin-pp-cli idp delete`** - Delete IDP.
- **`pangolin-pp-cli idp get`** - Get an IDP by its IDP ID.
- **`pangolin-pp-cli idp list`** - List all IDP in the system.
- **`pangolin-pp-cli idp update`** - Create an OIDC IdP.

### maintenance

Manage maintenance

- **`pangolin-pp-cli maintenance`** - Get maintenance information for a resource by domain.

### openapi-json

Manage openapi json

- **`pangolin-pp-cli openapi-json`** - Get OpenAPI specification as JSON

### openapi-yaml

Manage openapi yaml

- **`pangolin-pp-cli openapi-yaml`** - Get OpenAPI specification as YAML

### org

Manage org

- **`pangolin-pp-cli org create`** - Update an organization
- **`pangolin-pp-cli org delete`** - Delete an organization
- **`pangolin-pp-cli org get`** - Get an organization
- **`pangolin-pp-cli org update`** - Create a new organization

### orgs

Manage orgs

- **`pangolin-pp-cli orgs`** - List all organizations in the system.

### resource

Manage resource

- **`pangolin-pp-cli resource create`** - Update a resource.
- **`pangolin-pp-cli resource delete`** - Delete a resource.
- **`pangolin-pp-cli resource get`** - Get a resource by resourceId.

### role

Manage role

- **`pangolin-pp-cli role create`** - Update a role.
- **`pangolin-pp-cli role delete`** - Delete a role.
- **`pangolin-pp-cli role get`** - Get a role.

### site

Manage site

- **`pangolin-pp-cli site create`** - Update a site.
- **`pangolin-pp-cli site delete`** - Delete a site and all its associated data.
- **`pangolin-pp-cli site get`** - Get a site by siteId.

### site-resource

Manage site resource

- **`pangolin-pp-cli site-resource create`** - Update a site resource.
- **`pangolin-pp-cli site-resource delete`** - Delete a site resource.
- **`pangolin-pp-cli site-resource get`** - Get a specific site resource by siteResourceId.

### target

Manage target

- **`pangolin-pp-cli target create`** - Update a target.
- **`pangolin-pp-cli target delete`** - Delete a target.
- **`pangolin-pp-cli target get`** - Get a target.

### user

Manage user

- **`pangolin-pp-cli user <userId>`** - Get a user by ID.

### users

Manage users

- **`pangolin-pp-cli users`** - List non–server-admin users (server admin).


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
pangolin-pp-cli client get mock-value

# JSON for scripting and agents
pangolin-pp-cli client get mock-value --json

# Filter to specific fields
pangolin-pp-cli client get mock-value --json --select id,name,status

# Dry run — show the request without sending
pangolin-pp-cli client get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
pangolin-pp-cli client get mock-value --agent
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

## Health Check

```bash
pangolin-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/pangolin-integration-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `PANGOLIN_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `pangolin-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $PANGOLIN_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **404 on every command** — The OpenAPI spec says /v1 but most EE deployments mount the API at /api/v1. Set PANGOLIN_BASE_URL=https://<your-host>/api/v1 and rerun `pangolin-pp-cli doctor`.
- **401 unauthorized** — Verify PANGOLIN_TOKEN is set to a valid integration token (created in the dashboard under Integration Tokens). Run `pangolin-pp-cli doctor` to confirm.
- **Empty results from a list command** — Local store may be stale. Run `pangolin-pp-cli sync --full` first, then retry.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
