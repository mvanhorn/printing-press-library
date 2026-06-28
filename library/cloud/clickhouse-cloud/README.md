# Clickhouse Cloud CLI



Learn more at [Clickhouse Cloud](https://clickhouse.com/docs/en/cloud/manage/openapi?referrer=openapi-887333).

Created by [@sdhilip200](https://github.com/sdhilip200) (Dhilip Subramanian).

## Install

The recommended path installs both the `clickhouse-cloud-pp-cli` binary and the `pp-clickhouse-cloud` agent skill for supported agent runtimes in one shot:

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --skill-only
```

To constrain the skill install to one or more specific agents, repeat `--agent` with names supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI:

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --agent openclaw
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --agent openclaw --agent hermes
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/clickhouse-cloud/cmd/clickhouse-cloud-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickhouse-cloud-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-clickhouse-cloud --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-clickhouse-cloud --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install clickhouse-cloud --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with MCPB Hosts

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle for hosts that support one-click MCP extension installs.

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/clickhouse-cloud-current).
2. Double-click the `.mcpb` file or import it through your MCP host.
3. Fill in `CLICKHOUSE_CLOUD_USERNAME` and `CLICKHOUSE_CLOUD_PASSWORD` when prompted.

Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/cloud/clickhouse-cloud/cmd/clickhouse-cloud-pp-mcp@latest
```

Add the server to your MCP host configuration:

```json
{
  "mcpServers": {
    "clickhouse-cloud": {
      "command": "clickhouse-cloud-pp-mcp",
      "env": {
        "CLICKHOUSE_CLOUD_USERNAME": "<your-key-id>",
        "CLICKHOUSE_CLOUD_PASSWORD": "<your-key-secret>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Create an API key in ClickHouse Cloud and use the key id as the username and the key secret as the password.

```bash
export CLICKHOUSE_CLOUD_USERNAME="<your-key-id>"
export CLICKHOUSE_CLOUD_PASSWORD="<your-key-secret>"
```

To persist credentials, use `clickhouse-cloud-pp-cli auth set-token <key-id> <key-secret>`. Stored secrets live in `credentials.toml` under the data directory, not in `config.toml`.

### 3. Verify Setup

```bash
clickhouse-cloud-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
clickhouse-cloud-pp-cli invitations get mock-value mock-value
```

## Usage

Run `clickhouse-cloud-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CLICKHOUSE_CLOUD_CONFIG_DIR`, `CLICKHOUSE_CLOUD_DATA_DIR`, `CLICKHOUSE_CLOUD_STATE_DIR`, or `CLICKHOUSE_CLOUD_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CLICKHOUSE_CLOUD_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CLICKHOUSE_CLOUD_HOME=/srv/clickhouse-cloud
clickhouse-cloud-pp-cli doctor
```

Under `CLICKHOUSE_CLOUD_HOME=/srv/clickhouse-cloud`, the four dirs resolve to `/srv/clickhouse-cloud/config`, `/srv/clickhouse-cloud/data`, `/srv/clickhouse-cloud/state`, and `/srv/clickhouse-cloud/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

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

Precedence matters in fleets: an ambient per-kind variable such as `CLICKHOUSE_CLOUD_DATA_DIR` overrides an explicit `--home` for that kind. Use `CLICKHOUSE_CLOUD_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CLICKHOUSE_CLOUD_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `clickhouse-cloud-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activities

Manage activities

- **`clickhouse-cloud-pp-cli activities activity-get`** - Returns a single organization activity by ID.
- **`clickhouse-cloud-pp-cli activities activity-get-list`** - Returns a list of all organization activities.

### byoc-infrastructure

Manage byoc infrastructure

- **`clickhouse-cloud-pp-cli byoc-infrastructure create`** - Create a new BYOC Infrastructure in the organization. Returns the configuration of the newly created infrastructure
- **`clickhouse-cloud-pp-cli byoc-infrastructure delete`** - Removes a BYOC Infrastructure from the organization
- **`clickhouse-cloud-pp-cli byoc-infrastructure update`** - Update configuration of the BYOC infrastructure. Returns the modified infrastructure

### invitations

Manage invitations

- **`clickhouse-cloud-pp-cli invitations create`** - Creates organization invitation.
- **`clickhouse-cloud-pp-cli invitations delete`** - Deletes a single organization invitation.
- **`clickhouse-cloud-pp-cli invitations get`** - Returns details for a single organization invitation.
- **`clickhouse-cloud-pp-cli invitations get-list`** - Returns list of all organization invitations.

### keys

Manage keys

- **`clickhouse-cloud-pp-cli keys openapi-create`** - Creates new API key.
- **`clickhouse-cloud-pp-cli keys openapi-delete`** - Deletes API key. Only a key not used to authenticate the active request can be deleted.
- **`clickhouse-cloud-pp-cli keys openapi-get`** - Returns a single key details.
- **`clickhouse-cloud-pp-cli keys openapi-get-list`** - Returns a list of all keys in the organization.
- **`clickhouse-cloud-pp-cli keys openapi-update`** - Updates API key properties.

### members

Manage members

- **`clickhouse-cloud-pp-cli members delete`** - Removes a user from the organization
- **`clickhouse-cloud-pp-cli members get`** - Returns a single organization member details.
- **`clickhouse-cloud-pp-cli members get-list`** - Returns a list of all members in the organization.
- **`clickhouse-cloud-pp-cli members update`** - Updates organization member role.

### organizations

Manage organizations

- **`clickhouse-cloud-pp-cli organizations get`** - Returns details of a single organization. In order to get the details, the auth key must belong to the organization.
- **`clickhouse-cloud-pp-cli organizations get-list`** - Returns a list with a single organization associated with the API key in the request.
- **`clickhouse-cloud-pp-cli organizations update`** - Updates organization fields. Requires ADMIN auth key role.

### postgres

Manage postgres

- **`clickhouse-cloud-pp-cli postgres org-prometheus-get`** - **Disclaimer:** This beta endpoint is evolving; the API contract may change. <br /><br /> Returns Prometheus metrics for all PostgreSQL services in an organization. Maximum 100 services supported.
- **`clickhouse-cloud-pp-cli postgres service-create`** - **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future. <br /><br /> Creates a new Postgres service in the organization and returns it. The service is started asynchronously.
- **`clickhouse-cloud-pp-cli postgres service-delete`** - **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future. <br /><br /> Deletes a Postgres service that belongs to the organization
- **`clickhouse-cloud-pp-cli postgres service-get`** - **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future. <br /><br /> Returns a Postgres service that belongs to the organization
- **`clickhouse-cloud-pp-cli postgres service-get-list`** - **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future. <br /><br /> Returns a list of all Postgres services in the organization.
- **`clickhouse-cloud-pp-cli postgres service-patch`** - **This endpoint is in beta.** API contract is stable, and no breaking changes are expected in the future. <br /><br /> Update a Postgres service that belongs to the organization. **WARNING:** Changing the name also updates the host name and certificates for the service.

### private-endpoint-config

Manage private endpoint config

- **`clickhouse-cloud-pp-cli private-endpoint-config <organizationId>`** - Deprecated. Please follow [documentation](https://clickhouse.com/docs/manage/security/aws-privatelink#add-endpoint-id-to-services-allow-list) for the updated process.

### prometheus

Manage prometheus

- **`clickhouse-cloud-pp-cli prometheus <organizationId>`** - Returns prometheus metrics for all services in an organization.

### roles

Manage roles

- **`clickhouse-cloud-pp-cli roles delete`** - Deletes an existing custom role. System roles cannot be deleted. This operation will remove the role and all its associated policies.
- **`clickhouse-cloud-pp-cli roles get`** - Returns details for a specific role.
- **`clickhouse-cloud-pp-cli roles get-list`** - Returns all available roles (system + custom) for an organization.
- **`clickhouse-cloud-pp-cli roles patch`** - Updates an existing custom role. System roles cannot be updated. All fields are optional - only provided fields will be updated.
- **`clickhouse-cloud-pp-cli roles post`** - Creates a new custom role for an organization with specified policies and actors.

### services

Manage services

- **`clickhouse-cloud-pp-cli services instance-create`** - Creates a new service in the organization, and returns the current service state and a password to access the service. The service is started asynchronously.
- **`clickhouse-cloud-pp-cli services instance-delete`** - Deletes the service. The service must be in stopped state and is deleted asynchronously after this method call.
- **`clickhouse-cloud-pp-cli services instance-get`** - Returns a service that belongs to the organization
- **`clickhouse-cloud-pp-cli services instance-get-list`** - Returns a list of all services in the organization.
- **`clickhouse-cloud-pp-cli services instance-update`** - Updates basic service details like service name or IP access list.

### usage-cost

Manage usage cost

- **`clickhouse-cloud-pp-cli usage-cost <organizationId>`** - Returns a grand total and a list of daily, per-entity organization usage cost records for the organization in the queried time period (maximum 31 days). All days in both the request and the response are evaluated based on the UTC timezone.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
clickhouse-cloud-pp-cli invitations get mock-value mock-value

# JSON for scripting and agents
clickhouse-cloud-pp-cli invitations get mock-value mock-value --json

# Filter to specific fields
clickhouse-cloud-pp-cli invitations get mock-value mock-value --json --select id,name,status

# Dry run — show the request without sending
clickhouse-cloud-pp-cli invitations get mock-value mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
clickhouse-cloud-pp-cli invitations get mock-value mock-value --agent
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
clickhouse-cloud-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `clickhouse-cloud-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/clickhouse-cloud-pp-cli/config.toml`; `--home`, `CLICKHOUSE_CLOUD_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CLICKHOUSE_CLOUD_USERNAME` | per_call | Yes |  |
| `CLICKHOUSE_CLOUD_PASSWORD` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `clickhouse-cloud-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `clickhouse-cloud-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CLICKHOUSE_CLOUD_USERNAME`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
